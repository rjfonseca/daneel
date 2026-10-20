# ADR 0030: Local Inbox Capture

* Status: accepted
* Date: 2026-10-14

## Context and Problem Statement

Knowledge exploration needs a direct way to capture an idea without leaving the interface. The existing local Web API reads notes but has no narrow note-creation operation.

## Decision Drivers

- Require an explicit user save action and provide actionable errors.
- Confine writes to inbox and prevent overwrites, traversal and symlink escapes.
- Reuse the local HTTP origin and host boundary and existing indexing engine.

## Considered Options

- Download a Markdown file and require manual import.
- Expose a generic vault write API.
- Provide a dedicated bounded inbox-capture RPC.

## Decision Outcome

Provide CaptureNote with title, Markdown content and optional source URL. The server chooses a unique filename under inbox, validates bounded inputs, and creates the file exclusively through a rooted filesystem handle. Clients cannot provide a filesystem path. Reject an inbox symlink and enforce rooted containment. Allow only HTTP(S) source URLs; store metadata without raw YAML interpolation. Never overwrite an existing note.

The response returns the relative path. Index the saved note using existing parsing/indexing contracts. If indexing fails after durable save, return the saved path with an explicit warning rather than misleading the user into duplicating the capture. The UI keeps unsaved form content on errors, announces success, refreshes the graph and allows opening the new note.

### Consequences

- This adds one explicit write operation, not general editing or deletion.
- Existing local-only HTTP protections remain mandatory.
- Tests use temporary vaults, including symlink and collision cases; verification never writes to a real user's vault.
