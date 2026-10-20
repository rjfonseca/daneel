# ADR 0025: Local HTTP and Vault Read Boundaries

* Status: accepted
* Date: 2026-10-14

## Context and Problem Statement

The Web API can read vault notes and invoke maintenance. An all-interface listener, permissive CORS and path concatenation expose data beyond the intended local trust boundary.

## Decision Drivers

- Keep local workflows available without credential setup.
- Reject browser-origin and DNS rebinding attacks.
- Enforce filesystem containment during symlink resolution.

## Considered Options

- Retain an unauthenticated all-interface listener.
- Add remote authentication and a new network deployment mode.
- Keep HTTP local-only and enforce origin, host and vault boundaries.

## Decision Outcome

Keep HTTP local-only and enforce origin, host and vault boundaries. Bind to 127.0.0.1, accept only the selected local host/port and explicit local development origins, and read requested Web/MCP notes through os.Root. Validate paths before UI dispatch. Remote serving is unsupported.

### Consequences

- Same-user local processes remain trusted; this is not authentication.
- Originless local CLI requests continue to work. Foreign and null browser origins are rejected.
- Absolute symlinks may be rejected even when their targets are inside the vault.
- No new remote credentials or authentication subsystem is introduced.
