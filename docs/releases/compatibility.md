# Compatibility policy for 1.0

Daneel follows [Semantic Versioning](https://semver.org/). This policy takes effect
with v1.0.0; release candidates remain prereleases.

## Stable interfaces

| Interface | Guarantee in 1.x |
| --- | --- |
| Markdown vault | User notes remain the source of truth; caches can be rebuilt. No mandatory proprietary note format. |
| Core CLI | Documented invocation and flags for `init`, `index`, `search`, `read`, `backlinks`, `doctor`, `guard`, `version`, `mcp`, `serve` remain compatible. |
| Core configuration | Existing documented `vault`, `workspace`, `publishing` and `chat` keys retain meaning. New optional keys may be added. |
| Search JSON | Existing fields from `search --json` retain names and types; consumers must ignore additional fields. Empty results may be null or an empty array. |
| MCP retrieval | `search_vault`, `resolve_note`, `get_note_content`, `get_backlinks`, `vault_health`, `validate_public_export` retain names and existing input parameter types. Text rendering can change. |
| Exit status | Core commands return zero on success and nonzero on failure. Exact nonzero values are not separately classified. MCP tool failures use the MCP result/error envelope. |
| HTTP listener | Local IPv4 loopback only, with explicit browser origins. Remote access is unsupported. |

All other agent automation, Dream synthesis heuristics, telemetry, UI actions,
ConnectRPC schemas, layout caches, daemon IPC internals and generated file layouts
are experimental. They may evolve in minor releases. They must still preserve
user content and report failed mutations.

## Deprecation and upgrades

Stable interfaces are deprecated in documentation and release notes before
removal in a future major release. Security fixes may reject previously accepted
unsafe inputs. Additive JSON fields and MCP tools are compatible additions.

The SQLite search index and handoff caches are derived artifacts, not public
storage schemas. Stop background activity before upgrading. Keep a backup of the
vault and configuration, install the replacement binary, then run `daneel index
--force` and `daneel doctor`. Do not delete a SQLite database while another process
has it open. A downgrade may require rebuilding the index with the older binary;
newer experimental configuration keys may need to be removed.

Linux and macOS support the full local workflow; Linux systemd integration is
Linux-only. Windows binaries support core CLI, search, MCP and Web; Unix socket,
signal and systemd lifecycle parity is not guaranteed. The OS test matrix and
installation smoke checks are required before distributing these binaries.

The Web interface requires Chrome 111+, Firefox 128+ or Safari 16.4+.
