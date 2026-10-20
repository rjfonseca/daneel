# Changelog

## 1.0.0

This section describes the 1.0 content. Signed tags and GitHub Releases establish
publication status; a changelog heading alone does not announce a release.

### Knowledge navigation and capture

- Add a two-row Command Bridge with responsive navigation, scrollable Recent and Saved menus, search, capture and view controls.
- Browse and filter large knowledge sectors in bounded batches while searching the complete scope.
- Read notes with safe frontmatter handling, tables, internal links, navigation history and reloadable deep links.
- Capture Markdown notes into the local inbox and find them after refreshing the graph.
- Open daily notes through date selection and report missing notes without creating them.
- Review structural suggestions with supporting evidence and persistent local decisions.
- Keep dialogs, menus, focus indicators and reader controls usable with keyboards and short viewports.

### Agent integration

- Bound agent hook execution to the host's timeout budget and terminate descendant processes on timeout.
- Migrate installed Codex hook handlers while preserving unrelated configuration.

### Release and toolchain security

- Upgrade the frontend build and test toolchain to patched dependencies and audit the complete npm lockfile before pushes and releases.
- Preserve shared HUD design tokens and existing utility behavior while migrating to Tailwind CSS 4.
- Isolate test and benchmark Git commands from inherited hook context, preserving the contributor's repository state.

### Reliability and distribution

- Load secondary Web panels and Markdown on demand while retaining modal state.
- Contain deferred panel download failures without hiding the galaxy.
- Enforce zero frontend lint warnings and type-check application and tests.
- Exercise production Web chunks, deferred requests and cockpit state in browser CI.

- Restrict the Web listener and browser requests to explicit local origins and hosts.
- Confine Web and MCP note reads to the vault, including symlink resolution.
- Preserve earlier Dream output with atomic replacements and private recovery copies.
- Keep dry-run cycles free of vault writes and retain all newly added MOC entries.
- Quarantine pruned sources with recovery paths instead of permanently deleting them.
- Require shared CI and isolated installation/browser checks before publishing tags.
- Add pinned release tooling, archive SBOMs and artifact provenance configuration.
- Define stable and experimental interfaces and document recovery and candidate validation.
