# ADR 0026: Recoverable Vault Maintenance

* Status: accepted
* Date: 2026-10-14

## Context and Problem Statement

Dream replaces user-visible files and pruning removes source trees. Ignored persistence errors and non-atomic writes can lose content or report misleading success. Dry-run must not mutate derived artifacts.

## Decision Drivers

- Preserve user edits before replacement or removal.
- Surface write and scan failures.
- Keep recovery local, inspectable and independent of Git tracking.

## Considered Options

- Keep direct writes and permanent recursive removal.
- Require Git commits before every operation.
- Use rooted atomic replacements with backups and quarantine source trees.

## Decision Outcome

Use rooted atomic replacements with backups and quarantine source trees. Stage and sync files in the destination directory, preserve existing permissions, retain old bytes in .daneel/backups/<unique-id>/<relative-path>, then rename. Plan pruning before mutation, reject symlinked source/platform roots, and move selected directories into .daneel/quarantine/<unique-id>/<original-path>. Report recovery paths and persistence errors. Pruning does not implicitly reconcile catalogs or edit retained repository notes. Resolve cache locations without creating directories, and suppress all Dream output during dry-run.

### Consequences

- Recovery copies remain inside the vault and require an independent external backup for disk failure.
- Operations are atomic per file or rename, not transactional across an entire cycle. Run one maintenance writer per vault.
- Retention is explicit and manual; retained content consumes disk and remains sensitive.
- Read-only configuration resolution no longer creates cache directories. Callers that write must create their own destinations.
