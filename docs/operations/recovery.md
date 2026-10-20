# Vault recovery

## Before maintenance or upgrades

Stop the daemon and scheduled routines that write to the vault. Keep a separate
backup of the complete vault, including `.daneel.yaml`, notes, source documents
and the hidden `.daneel` directory. Version control is useful but does not retain
untracked or ignored files. Backups stored inside the vault are not protection
against loss of the disk or vault itself.

## Dream replacements

Before replacing existing content, Dream stores the old bytes at:

```text
.daneel/backups/<unique-id>/<original-relative-path>
```

Unchanged files do not create backups. Original file permissions are retained on
replacement; recovery copies are private. If creating the backup or writing the
replacement fails, the operation reports an error. A cycle updates files
individually, so earlier successful writes can remain after a later error.
Staging is in the destination directory and data is synced before rename; this
is not a transaction across files or a guarantee against every power-loss case.

To recover, stop writers, compare the retained copy with the current note, save
the current note elsewhere, then copy the selected earlier bytes back to the
original relative path. Reindex afterward. Do not replace a newer edited note
without comparing it first. Backups have no automatic expiration; review their
disk usage and remove copies only after retaining an independent backup.

## Pruned sources

Start with `daneel harvest prune --dry-run`. Executing prune moves selected
directories into:

```text
.daneel/quarantine/<unique-id>/sources/<platform>/<namespace>/<repository>
```

Each result prints its recovery copy; JSON includes `restore_path`. Local edits
and untracked files inside the moved directory are retained. No recursive source
deletion is performed by prune. Symlinked source or platform roots are rejected.
A failure can leave earlier items quarantined; their paths remain under the batch
directory. Inspect those locations before retrying.

To restore, stop writers, adjust the exclusion rules that selected the repository,
and move the quarantined directory back to its original location under `sources/`.
If that location exists, compare and merge instead of overwriting it. Then run:

```bash
daneel index --force
daneel doctor
```

Quarantine has no automatic expiration. It can contain sensitive data from the
original sources and should be protected like the vault itself.
