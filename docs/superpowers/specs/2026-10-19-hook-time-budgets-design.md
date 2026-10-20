# Bounded Native Agent Hooks

Status: implemented and candidate-validated; local deployment pending because the installed executable directory is mounted read-only in the execution environment.

## Problem and evidence

Codex 0.163.0 accepts the installed SessionStart, PostToolUse, and SessionEnd handlers without configuration errors. Isolated invocations of the installed Daneel binary completed in approximately 20ms with no daemon. An accepting Unix socket that never replies made SessionEnd take 5.09 seconds before disk fallback succeeded. The default Codex SessionEnd limit is one second; three seconds is the supported maximum.

This reproduces a failure mechanism, not proof that every historical warning has this cause. The previous invalid JSON hook structure is not currently present.

References: `cmd/hook.go`, `internal/daemon/socket.go`, `cmd/hook_install.go`, and the [official Codex hooks guide](https://learn.chatgpt.com/docs/hooks).

## Goals and constraints

- Preserve hook commands, JSON event payloads, existing IPC methods, XDG storage, and existing manual compilation commands.
- Limit hook IPC to one 200ms budget covering connect, write, and reply; retain normal IPC timeouts for unrelated operations.
- Limit automatic SessionEnd compilation to a separate 500ms budget after successful event persistence.
- Configure the Daneel Codex SessionEnd handler with timeout 3 seconds; do not modify unrelated handlers or trust hashes.
- Never report capture success when neither daemon acknowledgement nor local append succeeded.
- Add no dependencies, new queue, background child process, or detached goroutine in the hook CLI.
- Use English documentation, diagnostics, and neutral synthetic fixtures; tests must not access the user's vault or live daemon.

Budgets bound waiting and cooperative CPU/file-processing work on ordinary local filesystems. They do not provide hard real-time guarantees against arbitrary kernel I/O stalls, slow configuration files, or an incomplete stdin producer. Three seconds provides startup and filesystem margin; an end-to-end process test still enforces the supported Codex limit.

## Proposed behavior

### Hook IPC

Use a context-aware daemon connection and request path. Establish one context with a 200ms deadline for the whole hook IPC attempt, avoiding the preliminary IsDaemonRunning connection probe. Request handling must respect the earlier of its normal timeout and the caller deadline. Capture falls back to local spool storage on connection failure, malformed/negative reply, or deadline expiry. Handoff uses the same short daemon attempt and its existing local fallback.

Default daemon clients retain their existing one-second connect and five-second request behavior. Connection closure and cancellation release resources. Do not implement timeout by abandoning an active goroutine or by shortening the global timeout for routine execution and other clients.

Existing capture fallback is at least once: a daemon can persist an event before its acknowledgement is lost, then the CLI can append the same event locally. This plan does not claim exactly-once delivery or introduce event deduplication. Regression tests must assert preservation rather than a single append in this ambiguous-ack case.

### Automatic compilation

Persist the SessionEnd event before starting compilation. Reuse the existing compiler with an optional context, preserving the current no-time-limit CompileSession entry point for explicit CLI use. Automatic compilation uses a fresh 500ms context. Check cancellation while reading entries, processing payloads, and building output, and immediately before committing output. Check scanner errors explicitly; never archive a partially scanned session.

Build output into a temporary file beside the destination and rename it atomically only when the entire note is ready. A cancelled/failed attempt leaves the source JSONL available and must not create a partial final note, remove the raw spool, or mark it compiled. Once the final note is committed, finish the short archive bookkeeping without treating a later cancellation as an uncommitted output. Archive errors remain observable; never delete the raw spool on error. Remove only temporary files created by the attempt.

When compilation is deferred or fails after capture succeeds, return capture success and a concise stderr diagnostic such as `Session captured; compilation deferred. Run daneel hook compile --session <id> to retry.` Existing synchronous compilation remains automatic when it fits the budget. No raw payloads are logged.

Tradeoff: an oversized or slow session may stay as raw JSONL until an explicit `daneel hook compile --session <id>` or existing `--all` run. This change does not silently promise a new automatic background processor. Preserve current handoff behavior and document that a deferred session's compiled summary is unavailable until compilation completes.

### Installer and local migration

Set timeout 3 on the owned Codex SessionEnd command handler, including one already installed. Match only the exact Daneel capture command; preserve non-Daneel hooks, their fields, and other settings. Installation is idempotent and dry-run does not write. Preserve already supported migration from legacy Daneel strings/matcher groups.

Do not blindly replace the shared appendMatcherHookIfMissing behavior for Claude Code or Antigravity. Do not change the active local hooks until code tests pass and a new binary is built. Back up the executable and hooks file in a private directory outside the repository. Update the binary atomically while preserving executable mode, then run the new installer for Codex only. Never edit Codex trusted_hash values manually: report the approval state exposed by hooks/list; if reapproval is required, it must be granted through Codex.

## Acceptance

1. Absent, dead, accepting-but-silent, and late-reply daemon fixtures do not cause a five-second hook wait. Use 200ms production deadlines; allow a 750ms test wall-time ceiling to accommodate CI scheduling.
2. Normal daemon notification and local capture continue working. A filesystem write failure remains a nonzero capture error. Late acknowledgement may yield duplicates but never an empty spool.
3. Cancelled/expired/failed compilation preserves raw data and leaves no partial final note. A subsequent unrestricted CompileSession succeeds. Existing small-session auto-compilation and untrusted-content envelope remain intact.
4. A built CLI processes SessionEnd against a silent daemon within two seconds under a three-second harness deadline; the raw event is saved and a session ID can be resolved from the payload. Budget exhaustion does not require a real running daemon.
5. Installed Codex hooks parse with no errors; the owned SessionEnd handler reports timeoutSec 3. Trust status is recorded rather than fabricated.
6. Focused tests, race checks for touched packages, and full Go tests pass. No frontend code/assets changes are needed.

## Scope boundary

Do not fix unrelated plugin manifest warnings or claim to identify historical failures without matching logs. Do not add services, restart the user's daemon, alter the vault, or commit/merge/push as part of planning. An implementation review is required before deployment. ADR 0032 records the revised timeout and compilation contract relative to ADR 0024.
