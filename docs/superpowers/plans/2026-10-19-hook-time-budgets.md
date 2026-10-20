# Bounded Agent Hooks Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking. This document authorizes no implementation or local deployment by itself.

**Goal:** Make valid native hooks finish within Codex's execution window while preserving captured events.

**Architecture:** Add a context-aware IPC path used by hooks without changing ordinary daemon defaults. Give automatic compilation its own cancellable budget, retaining raw sessions on deferral. Update only the Daneel Codex SessionEnd handler and validate the built/installed executable through real subprocess and Codex parser checks.

**Tech Stack:** Existing Go context/net/filesystem APIs, Cobra, Go tests, Codex app-server JSON-RPC. No new dependencies.

**Spec:** [Bounded Native Agent Hooks](../specs/2026-10-19-hook-time-budgets-design.md)

## Global Constraints

- Preserve hook commands, JSON event payloads, existing IPC methods, XDG storage, and existing manual compilation commands.
- Limit hook IPC to one 200ms budget covering connect, write, and reply; retain normal IPC timeouts for unrelated operations.
- Limit automatic SessionEnd compilation to a separate 500ms budget after successful event persistence.
- Configure the Daneel Codex SessionEnd handler with timeout 3 seconds; do not modify unrelated handlers or trust hashes.
- Never report capture success when neither daemon acknowledgement nor local append succeeded.
- Add no dependencies, new queue, background child process, or detached goroutine in the hook CLI.
- Use English documentation, diagnostics, and neutral synthetic fixtures; tests must not access the user's vault or live daemon.

## Review Focus

- A silent daemon accepting connections must not delay disk fallback for five seconds: Task 1 socket test and Task 4 process test.
- A late acknowledgement after a daemon append must not lose the event: Task 1 preservation test; duplicates are allowed by the existing at-least-once contract.
- Cancellation during reading/rendering must not archive a partial session: Task 2 cancelled/deadline/scan-error tests and retry.
- Reinstalling alongside third-party handlers must preserve their timeouts and commands: Task 3 migration/idempotence fixtures.
- Local configuration changes can reset trust, and an old executable can recreate old behavior: Task 4 parser and executable-identity verification, with rollback.

## Task 1: Give hooks a bounded IPC path

**Files:** `internal/daemon/socket.go`, `internal/daemon/daemon_test.go`, `cmd/hook.go`, `cmd/hook_test.go`.

**Interfaces:** Add `NewClientContext(ctx context.Context) (*Client, error)`, `(*Client).CaptureHookEventContext(ctx context.Context, payload HookEventPayload) (string, error)`, and `(*Client).GetHookHandoffContext(ctx context.Context, cwd string) (string, error)`. Existing methods wrap the unchanged default behavior. A private context-aware request helper respects caller cancellation/deadline during writes and reads, including cancellation without a deadline. Hook ingress uses one 200ms context for dial and request.

- [x] Add `TestHookCapture_UnresponsiveDaemonFallsBackWithinBudget`: a fixture socket accepts and withholds replies; assert completion below 750ms and a persisted payload/session ID in the isolated spool. Demonstrate current code fails the bound.
- [x] Add cases for absent daemon, daemon rejection, delayed acknowledgement after persistence, and a non-writable spool. Assert event preservation on fallback, allow duplicate events after ambiguous acknowledgement, and require failure when all persistence paths fail. Fixture goroutines and sockets must be joined/closed by test cleanup.
- [x] Add daemon context tests for cancellation during blocked reads, deadline expiry, and successful responses. Verify unrelated default clients keep the original deadlines. Add handoff timeout/local-fallback coverage.
- [x] Run `go test ./cmd ./internal/daemon -run 'Hook|ClientContext' -count=1`; inspect expected timing failures before implementation.
- [x] Implement context-aware dial/request, retaining one absolute 200ms hook deadline. Remove IsDaemonRunning from capture/handoff's request path, use direct bounded connection, close connections, and preserve error-triggered fallback. Do not create a background goroutine that continues writing after the CLI returns.
- [x] Rerun the focused command; require passing results and no fixture leaks.

## Task 2: Bound automatic compilation and preserve retryable raw data

**Files:** `internal/spooler/compiler.go`, `internal/spooler/compiler_test.go`, `cmd/hook.go`, `cmd/hook_test.go`, `cmd/hook_e2e_test.go`.

**Interfaces:** Add `(*Compiler).CompileSessionContext(ctx context.Context, sessionID string) (string, error)`. Existing CompileSession calls it with context.Background. Automatic SessionEnd calls it with a fresh 500ms context after capture persistence; manual compile stays unrestricted. Context cancellation uses errors.Is with context.Canceled/context.DeadlineExceeded. Preserve existing executeHookCapture signature; emit a concise deferred-compilation diagnostic from the hook path after capture success.

- [x] Add `TestCompileSessionContext_CancelledPreservesSpool` and an expired-deadline equivalent: no final/partial note, no .compiled archive, raw spool retained, unrestricted retry succeeds. Add a deterministic cancellation-during-scan/render fixture using an internal helper boundary rather than a huge timing-sensitive allocation.
- [x] Add scanner-failure and temporary-output-write failure cases; no success or archive on partial input/output. Keep current valid payload sizes supported and check scanner.Err explicitly. Cover success publication and archive-failure observability without deleting raw data.
- [x] Run `go test ./internal/spooler ./cmd -run 'CompileSession|HookCapture|HookE2E' -count=1`; verify new cases expose missing cancellation/error handling.
- [x] Implement cancellation checks at scan, aggregation, rendering, and pre-publication boundaries. Write the complete note through a temporary file in the destination directory and atomically rename it. Archive only after successful publication. On cancellation clean up only this attempt's temporary files; retain raw input and preserve any preexisting final file. Do not add cancellation checks between committed publication and its short archive bookkeeping.
- [x] Wire the 500ms automatic budget; compilation deferral/failure after capture returns capture success with the retry command on stderr. A failed initial append remains an error. Keep the existing small-session automatic-compilation test passing; add a test proving cancellation never changes successful capture into failure.
- [x] Rerun focused tests and `go test -race ./internal/spooler ./internal/daemon ./cmd`; require no races or leaked workers. Limit package parallelism if container resources require it, recording the actual command.

## Task 3: Make Codex installer timeouts explicit and safe to regenerate

**Files:** `cmd/hook_install.go`, `cmd/hook_install_test.go`, `docs/ambient-intelligence/agent-hooks.md`, `docs/adr/0032-bounded-agent-hook-execution.md`, `docs/adr/README.md`.

**Interfaces:** Keep `installCodexHooks(path string, dryRun bool) (string, error)`. After the existing matcher-group migration, set `timeout: 3` on only the exact owned SessionEnd command. Do not alter shared adapter semantics or add an async flag: Codex SessionEnd is synchronous even with async enabled.

- [x] Add installer fixtures for a new file, current installed groups without timeout, a legacy Daneel string, and mixed third-party handlers with their own timeout. Assert exactly one Daneel SessionEnd handler with timeout 3 after repeated installs, third-party data unchanged, and dry-run byte-for-byte unchanged.
- [x] Run `go test ./cmd -run 'HookInstall.*Codex' -count=1`; verify missing-timeout assertions fail before the implementation.
- [x] Implement the targeted migration and rerun installer tests. Preserve existing fields on owned handlers other than the timeout being corrected.
- [x] Update the manual Codex snippet to show timeout 3. Document bounded IPC, bounded compilation, deferred-session recovery, and that errors must not be hidden with shell `|| true`. Add ADR 0032 to the ADR log; retain proposed status until implementation is approved.

## Task 4: Verify end-to-end and deploy the local correction

**Files:** `scripts/check_codex_hooks.py` (new, Python standard library only), plus completed docs/status updates. Local binary/hooks backups live outside the repository and never enter fixtures or commits.

**Interfaces:** The verification script accepts `--binary PATH` and `--codex PATH`. Its default mode uses temporary vault/XDG directories only. A separate `--inspect-installed` mode performs read-only Codex hooks/list requests against current configuration. No mode rewrites hook trust state or calls a model.

- [x] Build a candidate with `go build -o <private-temp>/daneel .`; verify normal and silent-daemon hook subprocesses using neutral Codex-shaped stdin. Enforce a three-second subprocess ceiling and an under-two-second capture target. Verify disk content, zero capture exit status, and no raw payload output. Ensure sockets/children/temp files are cleaned up.
- [x] Run `go test ./...`, the focused race command from Task 2, and `git diff --check`. Require passing output. Run the new script against the candidate; capture actual timings rather than documenting theoretical limits.
- [x] Use Codex app-server initialize and hooks/list with transient `-c` inline hook overrides carrying a fixture-only statusMessage. Identify fixture handlers by that marker so inherited user hooks are not confused with fixtures. Assert no parsing errors and timeoutSec 3. Codex hides an untrusted temporary project's hooks, so that project file cannot serve as the parser fixture without changing trust; do not start a thread or change trust to bypass this. Validate actual generated user JSON after deployment below. Keep model/MCP execution out of this check and leave the real Codex home unchanged during fixture tests.
- [ ] Before local deployment, capture executable path/hash/mode and privately back up the binary and hooks.json outside the repository. Resolve the executable actually used by the hook shell. Copy candidate to a temporary file beside the installed executable and atomically replace it, preserving executable permissions. Then run that installed binary's `hook install --agent codex`. Do not restart the daemon: the hook client/installer change does not require replacing the running daemon.
- [ ] Run read-only hooks/list for the real workspace and a temporary directory. Verify exactly the intended commands, no configuration errors, timeoutSec 3, and record trustStatus. If Codex requires reapproval, report the concrete changed handler and let the user approve in Codex; never fabricate a trusted hash.
- [ ] Verify installed binary hash matches the candidate and rerun isolated subprocess checks using the installed path. Do not send synthetic SessionEnd to the user's real daemon/vault. Record any deferred-compilation warning as a success-with-pending-summary, not lost capture.
- [ ] If installation verification fails, restore both binary and hooks file from the matching backups and revalidate parsing. Leave source changes reviewable. On success, mark design/ADR implemented/accepted and report test results, timings, and pending trust requirements.

## Execution boundary

Only documents are created during planning. Implement and deploy locally after the user approves this plan. No commit, push, merge, daemon restart, service installation, or new automatic compilation worker is included. Existing manual `hook compile` recovery is an explicit tradeoff, not an implicit future task.

## Execution status

Source implementation and reviews passed, including full Go tests, package race tests, isolated candidate captures, Codex inline parser checks, and process-cleanup regressions. Local installation stopped before mutation because the executable directory is mounted read-only. The installed binary and hooks configuration still match their backups; deployment steps remain unchecked. See the plan workspace deployment report and prepared host-side installer.
