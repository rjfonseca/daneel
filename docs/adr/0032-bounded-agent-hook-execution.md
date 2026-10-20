# ADR 0032: Bounded Agent Hook Execution

* Status: accepted
* Date: 2026-10-19

## Context and Problem Statement

Native hook capture falls back to disk only after a daemon request with a five-second deadline. Codex SessionEnd defaults to one second and permits at most three seconds. SessionEnd also compiles a complete session synchronously. Valid hook configuration therefore does not guarantee execution finishes before the agent kills it.

## Decision Drivers

- Preserve captured events when the daemon is slow or unavailable.
- Keep ordinary daemon operations and other agents compatible.
- Avoid new workers, external processes, and falsely successful capture.
- Separate successful capture from optional compilation.

## Considered Options

- Only raise the Codex timeout: insufficient because its three-second maximum is below the existing five-second request deadline.
- Suppress hook errors: hides persistence failures.
- Add an asynchronous compilation worker: viable future work, but introduces lifecycle, recovery, and multi-vault routing contracts beyond this correction.
- Bound hook-specific IPC and automatic compilation, preserving raw sessions for explicit retries.

## Decision Outcome

Use a 200ms hook IPC context covering connection and request, a separate 500ms best-effort automatic compilation context after event persistence, and an explicit three-second Codex SessionEnd timeout. Retain existing daemon client defaults for unrelated callers. Compiler cancellation must preserve raw spool and avoid partial final notes through atomic output publication.

### Consequences

- Oversized/slow sessions can require `daneel hook compile --session <id>` or `--all` after capture succeeds.
- No new background processor or delivery guarantee is implied. Existing ambiguous acknowledgements retain at-least-once behavior.
- Context deadlines do not make arbitrary filesystem/kernel stalls hard real-time operations.
- Existing hooks require a targeted local installer update, and Codex may require approving the changed handler hash.

See [the design](../superpowers/specs/2026-10-19-hook-time-budgets-design.md).
