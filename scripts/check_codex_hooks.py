#!/usr/bin/env python3
"""Verify bounded Daneel hook capture and inspect Codex hook parsing.

The default mode runs only the supplied Daneel executable with synthetic input,
temporary vault/XDG directories, and transient app-server parser overrides. The
optional installed mode sends read-only ``hooks/list`` requests to Codex and
never writes hook configuration or trust state.
"""

from __future__ import annotations

import argparse
import json
import os
import queue
import signal
import socket
import subprocess
import sys
import tempfile
import threading
import time
from pathlib import Path
from typing import Any, Iterable


PROCESS_CEILING_SECONDS = 3.0
CAPTURE_TARGET_SECONDS = 2.0
APP_SERVER_REQUEST_SECONDS = 20.0
SESSION_END_COMMAND = "daneel hook capture SessionEnd --agent codex"
PARSER_FIXTURE_STATUS = "Daneel isolated parser fixture"
SYNTHETIC_MARKER = "synthetic-codex-hook-check-8d71f2"


class CheckError(RuntimeError):
    """A verification failure safe to show without exposing process output."""


def _checked_executable(path: str, name: str) -> str:
    candidate = Path(path).expanduser().resolve()
    if not candidate.is_file() or not os.access(candidate, os.X_OK):
        raise CheckError(f"{name} must name an executable file")
    return str(candidate)


def _parser_overrides() -> list[str]:
    """Provide an in-memory parser fixture without changing Codex trust state."""
    status = json.dumps(PARSER_FIXTURE_STATUS)
    return [
        'hooks.SessionStart=[{hooks=[{type="command",command="daneel hook handoff",statusMessage='
        + status
        + "}]}]",
        'hooks.PostToolUse=[{matcher="*",hooks=[{type="command",command="daneel hook capture PostToolUse --agent codex",statusMessage='
        + status
        + "}]}]",
        'hooks.SessionEnd=[{hooks=[{type="command",command="'
        + SESSION_END_COMMAND
        + '",timeout=3,statusMessage='
        + status
        + "}]}]",
    ]


def _make_temp_environment(root: Path, vault: Path) -> dict[str, str]:
    """Isolate Daneel state without changing HOME or CODEX_HOME."""
    xdg = root / "x"
    env = os.environ.copy()
    for name, leaf in (
        ("XDG_CACHE_HOME", "c"),
        ("XDG_CONFIG_HOME", "g"),
        ("XDG_DATA_HOME", "d"),
        ("XDG_STATE_HOME", "s"),
        ("XDG_RUNTIME_DIR", "r"),
    ):
        path = xdg / leaf
        path.mkdir(parents=True, exist_ok=True, mode=0o700)
        if name == "XDG_RUNTIME_DIR":
            path.chmod(0o700)
        env[name] = str(path)
    env["DANEEL_VAULT_PATH"] = str(vault)
    return env


def _spool_file(cache_home: Path, session_id: str) -> Path:
    spool_dir = cache_home / "daneel" / "spool"
    active = spool_dir / f"{session_id}.jsonl"
    archived = spool_dir / f"{session_id}.jsonl.compiled"
    if active.is_file():
        return active
    if archived.is_file():
        return archived
    raise CheckError("captured session was not present in the isolated spool")


def _verify_spool(cache_home: Path, session_id: str) -> str:
    path = _spool_file(cache_home, session_id)
    found = False
    try:
        with path.open(encoding="utf-8") as spool:
            for line in spool:
                entry = json.loads(line)
                if entry.get("session_id") != session_id:
                    continue
                payload = entry.get("payload")
                if isinstance(payload, dict) and SYNTHETIC_MARKER in json.dumps(payload):
                    found = True
                    break
    except (OSError, UnicodeError, json.JSONDecodeError) as exc:
        raise CheckError("could not parse the isolated hook spool") from exc
    if not found:
        raise CheckError("the isolated spool did not retain the synthetic payload")
    return "archived" if path.name.endswith(".compiled") else "raw"


def _signal_process_group(proc: subprocess.Popen[bytes], sig: int) -> None:
    try:
        # The leader may have exited while a descendant still holds its pipes.
        # The process group remains addressable by the leader's PID in that case.
        os.killpg(proc.pid, sig)
    except (ProcessLookupError, PermissionError):
        if proc.poll() is None:
            try:
                proc.send_signal(sig)
            except ProcessLookupError:
                pass


def _kill_and_drain(proc: subprocess.Popen[bytes]) -> None:
    """Stop the whole subprocess group and bound pipe/reap cleanup."""
    _signal_process_group(proc, signal.SIGKILL)
    try:
        proc.communicate(timeout=0.75)
    except subprocess.TimeoutExpired:
        # A detached child may retain a pipe. Do not let cleanup defeat the
        # three-second execution ceiling; close our descriptors and reap leader.
        _signal_process_group(proc, signal.SIGKILL)
        for stream in (proc.stdin, proc.stdout, proc.stderr):
            if stream is not None:
                try:
                    stream.close()
                except OSError:
                    pass
        try:
            proc.wait(timeout=0.5)
        except subprocess.TimeoutExpired:
            proc.kill()
            proc.wait(timeout=0.5)


def _run_capture(
    binary: str,
    env: dict[str, str],
    cwd: Path,
    session_id: str,
    silent_socket: Path | None,
) -> tuple[float, str]:
    payload = {
        "session_id": session_id,
        "cwd": str(cwd),
        "hook_event_name": "SessionEnd",
        "reason": "completed",
        "transcript_path": str(cwd / "session.jsonl"),
        "prompt": SYNTHETIC_MARKER,
        "agent": "codex",
    }
    argv = [
        binary,
        "hook",
        "capture",
        "SessionEnd",
        "--agent",
        "codex",
        "--cwd",
        str(cwd),
    ]
    proc: subprocess.Popen[bytes] | None = None
    started = time.monotonic()
    try:
        proc = subprocess.Popen(
            argv,
            cwd=cwd,
            env=env,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            start_new_session=True,
        )
        try:
            stdout, stderr = proc.communicate(
                json.dumps(payload, separators=(",", ":")).encode("utf-8"),
                timeout=PROCESS_CEILING_SECONDS,
            )
        except subprocess.TimeoutExpired:
            _kill_and_drain(proc)
            raise CheckError("hook subprocess exceeded the three-second ceiling")
    except OSError as exc:
        raise CheckError("could not start the Daneel hook subprocess") from exc
    finally:
        if proc is not None and proc.poll() is None:
            _signal_process_group(proc, signal.SIGTERM)
            try:
                proc.wait(timeout=0.5)
            except subprocess.TimeoutExpired:
                _signal_process_group(proc, signal.SIGKILL)
                proc.wait()
        if silent_socket is not None:
            # The listener is owned by the context manager around this call.
            pass

    elapsed = time.monotonic() - started
    if proc.returncode != 0:
        raise CheckError(f"hook subprocess exited with status {proc.returncode}")
    if elapsed >= CAPTURE_TARGET_SECONDS:
        raise CheckError("hook capture missed the two-second target")
    combined = stdout + b"\n" + stderr
    if SYNTHETIC_MARKER.encode("utf-8") in combined:
        raise CheckError("hook subprocess printed raw synthetic payload data")
    status = "silent-daemon" if silent_socket is not None else "offline"
    return elapsed, status


class SilentUnixDaemon:
    """Accept one Unix socket connection and withhold the IPC response."""

    def __init__(self, socket_path: Path):
        self.socket_path = socket_path
        self.listener: socket.socket | None = None
        self.connections: list[socket.socket] = []
        self.stop = threading.Event()
        self.ready = threading.Event()
        self.thread: threading.Thread | None = None
        self.failure: BaseException | None = None

    def __enter__(self) -> "SilentUnixDaemon":
        self.listener = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        self.listener.bind(str(self.socket_path))
        self.listener.listen(4)
        self.listener.settimeout(0.1)
        self.thread = threading.Thread(target=self._accept, name="silent-daneel-fixture")
        self.thread.start()
        if not self.ready.wait(timeout=1.0):
            self.close()
            raise CheckError("silent daemon fixture failed to start")
        if self.failure is not None:
            self.close()
            raise CheckError("silent daemon fixture failed to start") from self.failure
        return self

    def _accept(self) -> None:
        assert self.listener is not None
        self.ready.set()
        while not self.stop.is_set():
            try:
                conn, _ = self.listener.accept()
            except socket.timeout:
                continue
            except OSError:
                if not self.stop.is_set():
                    self.failure = RuntimeError("listener stopped unexpectedly")
                return
            conn.settimeout(0.1)
            self.connections.append(conn)

    def close(self) -> None:
        self.stop.set()
        if self.listener is not None:
            try:
                self.listener.close()
            except OSError:
                pass
        for conn in self.connections:
            try:
                conn.close()
            except OSError:
                pass
        if self.thread is not None:
            self.thread.join(timeout=1.0)
            if self.thread.is_alive():
                raise CheckError("silent daemon fixture did not stop")
        try:
            self.socket_path.unlink()
        except FileNotFoundError:
            pass

    def __exit__(self, exc_type: Any, exc: Any, tb: Any) -> None:
        self.close()


class AppServer:
    """Minimal JSON-RPC client that drains and discards unsolicited output."""

    def __init__(self, codex: str, cwd: Path, config_overrides: Iterable[str] = ()):
        self.codex = codex
        self.cwd = cwd
        self.config_overrides = tuple(config_overrides)
        self.proc: subprocess.Popen[bytes] | None = None
        self.lines: queue.Queue[bytes | None] = queue.Queue()
        self.reader: threading.Thread | None = None
        self.stderr_reader: threading.Thread | None = None
        self._next_id = 1

    def __enter__(self) -> "AppServer":
        try:
            argv = [self.codex, "app-server", "--stdio", "--enable", "hooks"]
            for override in self.config_overrides:
                argv.extend(("-c", override))
            self.proc = subprocess.Popen(
                argv,
                cwd=self.cwd,
                stdin=subprocess.PIPE,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                start_new_session=True,
            )
        except OSError as exc:
            raise CheckError("could not start Codex app-server") from exc
        assert self.proc.stdout is not None and self.proc.stderr is not None
        self.reader = threading.Thread(
            target=self._read_stdout, args=(self.proc.stdout,), name="codex-appserver-stdout"
        )
        self.stderr_reader = threading.Thread(
            target=self._drain_stderr, args=(self.proc.stderr,), name="codex-appserver-stderr"
        )
        self.reader.start()
        self.stderr_reader.start()
        try:
            init = self.request(
                "initialize",
                {"clientInfo": {"name": "daneel-hook-check", "version": "1.0.0"}},
            )
            if not isinstance(init, dict):
                raise CheckError("Codex app-server returned an invalid initialize response")
            self.notify("initialized", {})
            return self
        except BaseException:
            self.close()
            raise

    def _read_stdout(self, stream: Any) -> None:
        try:
            for line in iter(stream.readline, b""):
                self.lines.put(line)
        finally:
            self.lines.put(None)

    @staticmethod
    def _drain_stderr(stream: Any) -> None:
        while stream.read(4096):
            pass

    def _send(self, message: dict[str, Any]) -> None:
        assert self.proc is not None and self.proc.stdin is not None
        if self.proc.poll() is not None:
            raise CheckError("Codex app-server exited before replying")
        self.proc.stdin.write(json.dumps(message, separators=(",", ":")).encode() + b"\n")
        self.proc.stdin.flush()

    def notify(self, method: str, params: dict[str, Any]) -> None:
        self._send({"jsonrpc": "2.0", "method": method, "params": params})

    def request(self, method: str, params: dict[str, Any]) -> dict[str, Any]:
        request_id = self._next_id
        self._next_id += 1
        self._send({"jsonrpc": "2.0", "id": request_id, "method": method, "params": params})
        deadline = time.monotonic() + APP_SERVER_REQUEST_SECONDS
        while True:
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise CheckError("Codex app-server request timed out")
            try:
                line = self.lines.get(timeout=remaining)
            except queue.Empty as exc:
                raise CheckError("Codex app-server request timed out") from exc
            if line is None:
                raise CheckError("Codex app-server closed its protocol stream")
            try:
                message = json.loads(line)
            except (UnicodeError, json.JSONDecodeError) as exc:
                raise CheckError("Codex app-server emitted malformed protocol output") from exc
            if message.get("id") != request_id:
                # Notifications and server requests are not executed or surfaced.
                continue
            if "error" in message:
                raise CheckError(f"Codex app-server rejected the {method} request")
            result = message.get("result")
            if not isinstance(result, dict):
                raise CheckError(f"Codex app-server returned an invalid {method} response")
            return result

    def hooks_list(self, cwd: Path) -> dict[str, Any]:
        return self.request("hooks/list", {"cwds": [str(cwd)]})

    def close(self) -> None:
        proc = self.proc
        if proc is None:
            return
        if proc.stdin is not None:
            try:
                proc.stdin.close()
            except OSError:
                pass
        # Close descendants too when the app-server leader has already exited
        # but a child still owns one of the captured output pipes.
        _signal_process_group(proc, signal.SIGTERM)
        try:
            proc.wait(timeout=0.5)
        except subprocess.TimeoutExpired:
            pass
        _signal_process_group(proc, signal.SIGKILL)
        if proc.poll() is None:
            try:
                proc.wait(timeout=0.5)
            except subprocess.TimeoutExpired:
                proc.kill()
                proc.wait(timeout=0.5)
        if proc.stdout is not None:
            proc.stdout.close()
        if proc.stderr is not None:
            proc.stderr.close()
        for thread in (self.reader, self.stderr_reader):
            if thread is not None:
                thread.join(timeout=1.0)
                if thread.is_alive():
                    raise CheckError("Codex app-server output reader did not stop")

    def __exit__(self, exc_type: Any, exc: Any, tb: Any) -> None:
        self.close()


def _entry_for_cwd(response: dict[str, Any], cwd: Path) -> dict[str, Any]:
    entries = response.get("data")
    if not isinstance(entries, list):
        raise CheckError("Codex app-server hooks/list response had no data array")
    cwd_text = str(cwd.resolve())
    for entry in entries:
        if isinstance(entry, dict) and str(Path(str(entry.get("cwd", ""))).resolve()) == cwd_text:
            return entry
    raise CheckError("Codex app-server omitted the requested working directory")


def _validate_fixture_hooks(entry: dict[str, Any]) -> tuple[int, str]:
    errors = entry.get("errors", [])
    if not isinstance(errors, list) or errors:
        raise CheckError("Codex reported hook configuration errors")
    hooks = entry.get("hooks")
    if not isinstance(hooks, list):
        raise CheckError("Codex app-server returned an invalid hooks list")
    fixture_hooks = [
        hook
        for hook in hooks
        if isinstance(hook, dict) and hook.get("statusMessage") == PARSER_FIXTURE_STATUS
    ]
    if not fixture_hooks:
        raise CheckError("Codex did not return the transient parser fixture handlers")
    commands = {
        hook.get("command")
        for hook in fixture_hooks
        if hook.get("handlerType") == "command" and isinstance(hook.get("command"), str)
    }
    expected = {
        "daneel hook handoff",
        "daneel hook capture PostToolUse --agent codex",
        SESSION_END_COMMAND,
    }
    if not expected.issubset(commands):
        raise CheckError("Codex did not parse all transient Daneel hook handlers")
    if len(fixture_hooks) != len(expected):
        raise CheckError("Codex returned duplicate transient parser fixture handlers")
    session_end = next(h for h in fixture_hooks if h.get("command") == SESSION_END_COMMAND)
    if session_end.get("timeoutSec") != 3:
        raise CheckError("Codex did not parse the SessionEnd timeout as three seconds")
    trust = session_end.get("trustStatus")
    return len(fixture_hooks), str(trust) if trust is not None else "unspecified"


def _check_captures(binary: str, root: Path) -> list[str]:
    project = root / "p"
    project.mkdir()
    vault = root / "v"
    vault.mkdir()
    cache_home = root / "x" / "c"
    env = _make_temp_environment(root, vault)
    reports: list[str] = []
    for label in ("offline", "silent-daemon"):
        session_id = f"daneel-check-{label}"
        socket_path: Path | None = None
        daemon: SilentUnixDaemon | None = None
        if label == "silent-daemon":
            socket_path = root / "x" / "s" / "daneel" / "daneel.sock"
            socket_path.parent.mkdir(parents=True, mode=0o700, exist_ok=True)
            daemon = SilentUnixDaemon(socket_path)
            daemon.__enter__()
        try:
            elapsed, _ = _run_capture(binary, env, project, session_id, socket_path)
            storage = _verify_spool(cache_home, session_id)
            reports.append(
                f"PASS {label} SessionEnd capture in {elapsed * 1000:.0f} ms; spool={storage}"
            )
        finally:
            if daemon is not None:
                daemon.close()
    return reports


def _run_fixture_parser(codex: str, root: Path) -> str:
    project = root / "p"
    project.mkdir(exist_ok=True)
    with AppServer(codex, project, _parser_overrides()) as app_server:
        entry = _entry_for_cwd(app_server.hooks_list(project), project)
        count, trust = _validate_fixture_hooks(entry)
    return f"PASS Codex parsed {count} fixture handlers; SessionEnd timeoutSec=3; trustStatus={trust}"


def _inspect_installed(codex: str, cwds: Iterable[Path]) -> list[str]:
    reports: list[str] = []
    for cwd in cwds:
        resolved = cwd.resolve()
        if not resolved.is_dir():
            raise CheckError("--cwd must name an existing directory")
        with AppServer(codex, resolved) as app_server:
            entry = _entry_for_cwd(app_server.hooks_list(resolved), resolved)
        errors = entry.get("errors", [])
        if not isinstance(errors, list):
            raise CheckError("Codex returned an invalid hook error list")
        if errors:
            raise CheckError(f"Codex reported {len(errors)} hook configuration error(s)")
        hooks = entry.get("hooks", [])
        if not isinstance(hooks, list):
            raise CheckError("Codex returned an invalid hooks list")
        daneel_hooks = [
            hook
            for hook in hooks
            if isinstance(hook, dict)
            and hook.get("handlerType") == "command"
            and str(hook.get("command", "")).startswith("daneel hook ")
        ]
        expected_commands = {
            "daneel hook handoff",
            "daneel hook capture PostToolUse --agent codex",
            SESSION_END_COMMAND,
        }
        command_counts: dict[str, int] = {}
        for hook in daneel_hooks:
            command = hook.get("command")
            if isinstance(command, str):
                command_counts[command] = command_counts.get(command, 0) + 1
        if set(command_counts) != expected_commands or any(
            command_counts.get(command) != 1 for command in expected_commands
        ):
            raise CheckError("installed Codex configuration does not contain exactly the three Daneel handlers")
        session_end = next(h for h in daneel_hooks if h.get("command") == SESSION_END_COMMAND)
        if session_end.get("timeoutSec") != 3:
            raise CheckError("installed Daneel SessionEnd hook does not have timeoutSec 3")
        lines = [f"PASS hooks/list cwd={resolved} configuration_errors=0 Daneel_handlers={len(daneel_hooks)}"]
        for hook in daneel_hooks:
            event = str(hook.get("eventName", "unknown"))
            timeout = hook.get("timeoutSec", "unknown")
            trust = hook.get("trustStatus", "unspecified")
            source = _source_path(hook.get("sourcePath")) or "unknown"
            lines.append(
                f"  {event} timeoutSec={timeout} trustStatus={trust} sourcePath={source}"
            )
        reports.extend(lines)
    return reports


def _parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", help="candidate or installed Daneel executable")
    parser.add_argument("--codex", required=True, help="Codex CLI executable")
    parser.add_argument(
        "--inspect-installed",
        action="store_true",
        help="perform read-only hooks/list requests for installed configuration",
    )
    parser.add_argument(
        "--cwd",
        action="append",
        type=Path,
        help="working directory to inspect; repeatable (default: current directory)",
    )
    args = parser.parse_args(argv)
    if not args.inspect_installed and not args.binary:
        parser.error("--binary is required unless --inspect-installed is selected")
    if args.inspect_installed and args.binary:
        parser.error("--binary cannot be combined with --inspect-installed")
    if args.cwd and not args.inspect_installed:
        parser.error("--cwd is only valid with --inspect-installed")
    return args


def main(argv: list[str] | None = None) -> int:
    args = _parse_args(argv)
    try:
        codex = _checked_executable(args.codex, "--codex")
        if args.inspect_installed:
            cwds = args.cwd or [Path.cwd()]
            for line in _inspect_installed(codex, cwds):
                print(line)
            return 0

        binary = _checked_executable(args.binary, "--binary")
        with tempfile.TemporaryDirectory(prefix="dch-") as temp:
            root = Path(temp)
            lines = _check_captures(binary, root)
            for line in lines:
                print(line)
            print(_run_fixture_parser(codex, root))
        return 0
    except CheckError as exc:
        print(f"FAIL: {exc}", file=sys.stderr)
        return 1
    except OSError:
        print("FAIL: unable to prepare isolated hook verification", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
