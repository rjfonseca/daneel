"""Exercise a clean local installation without touching the user's vault or services."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import tempfile
import time
import socket
import tarfile
import zipfile
import urllib.error
import urllib.request


def smoke(binary, root):
    env = os.environ.copy()
    for key in ("DANEEL_VAULT_PATH", "DANEEL_CONFIG"):
        env.pop(key, None)
    for key, directory in (("XDG_CONFIG_HOME", "config"), ("XDG_CACHE_HOME", "cache"),
                           ("XDG_DATA_HOME", "data"), ("XDG_STATE_HOME", "state")):
        env[key] = str(root / directory)
    env.update(GIT_CONFIG_GLOBAL=os.devnull, GIT_CONFIG_NOSYSTEM="1", NO_COLOR="1")
    vault = root / "vault"

    def run(*args, input=None):
        return subprocess.run([str(binary), *args], cwd=root, env=env, input=input,
                              text=True, capture_output=True, check=True, timeout=90).stdout

    assert "Version:" in run("version")
    run("init", "--local", "--yes", str(vault))
    note = vault / "notes" / "architecture" / "release-smoke.md"
    note.parent.mkdir(parents=True, exist_ok=True)
    note.write_text("# Release smoke\n\nQuasar persistence example.\n", encoding="utf-8")
    results = json.loads(run("--vault-path", str(vault), "search", "Quasar", "--json"))
    assert any(item["rel_path"] == "notes/architecture/release-smoke.md" for item in results), results
    assert "Quasar" in run("--vault-path", str(vault), "read", "notes/architecture/release-smoke.md")
    messages = [
        {"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {}},
        {"jsonrpc": "2.0", "method": "notifications/initialized"},
        {"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": {}},
        {"jsonrpc": "2.0", "id": 3, "method": "tools/call", "params": {
            "name": "get_note_content", "arguments": {"rel_path": "notes/architecture/release-smoke.md"}}},
    ]
    output = run("--vault-path", str(vault), "mcp", input="".join(json.dumps(m) + "\n" for m in messages))
    responses = {r["id"]: r for r in map(json.loads, output.splitlines()) if "id" in r}
    assert "error" not in responses[1], responses[1]
    assert any(t["name"] == "search_vault" for t in responses[2]["result"]["tools"])
    assert "Quasar" in responses[3]["result"]["content"][0]["text"], responses[3]
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 0))
        port = listener.getsockname()[1]
    process = subprocess.Popen([str(binary), "--vault-path", str(vault), "serve", "--port", str(port)],
                               cwd=root, env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    try:
        url = f"http://127.0.0.1:{port}/"
        for _ in range(150):
            if process.poll() is not None:
                raise RuntimeError("Web server exited before readiness")
            try:
                with urllib.request.urlopen(url, timeout=2) as response:
                    assert b"<html" in response.read(), "missing embedded frontend"
                break
            except urllib.error.URLError:
                time.sleep(0.2)
        else:
            raise TimeoutError("Web server did not start")
        try:
            urllib.request.urlopen(urllib.request.Request(url, headers={"Origin": "https://example.com"}), timeout=2)
            raise AssertionError("foreign browser origin was accepted")
        except urllib.error.HTTPError as error:
            assert error.code == 403, error
    finally:
        process.terminate()
        try:
            process.wait(timeout=10)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait()
    print("Installation smoke passed: init, search, read, MCP, embedded Web and origin protection")


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    group = parser.add_mutually_exclusive_group()
    group.add_argument("--binary", type=Path, help="Test an extracted release binary")
    group.add_argument("--archive", type=Path, help="Extract and test a locally built native release archive")
    args = parser.parse_args()
    repo = Path(__file__).resolve().parent.parent
    with tempfile.TemporaryDirectory(prefix="dnl-") as directory:
        root = Path(directory)
        if args.archive:
            extract = root / "extracted"
            extract.mkdir()
            archive = args.archive.resolve()
            if archive.suffix == ".zip":
                with zipfile.ZipFile(archive) as package:
                    package.extractall(extract)
            else:
                with tarfile.open(archive) as package:
                    package.extractall(extract, filter="data")
            binary = extract / ("daneel.exe" if os.name == "nt" else "daneel")
        elif args.binary:
            binary = args.binary.resolve()
        else:
            install_env = os.environ.copy()
            install_env.update(GOBIN=str(root / "bin"), CGO_ENABLED="0")
            subprocess.run(["go", "install", "-trimpath", "."], cwd=repo, env=install_env, check=True, timeout=300)
            binary = root / "bin" / ("daneel.exe" if os.name == "nt" else "daneel")
        smoke(binary, root)
