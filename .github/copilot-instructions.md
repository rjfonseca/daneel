# GitHub Copilot & Autonomous Agent Instructions

See [AGENTS.md](../AGENTS.md) for architectural invariants, progressive disclosure documentation, and coding standards.
- Always produce ADRs (MADR format) under `docs/adr/` for architectural changes.
- Ensure all user-facing UI, tooltips, and documentation remain strictly in English.
- **PR Titles**: Always use strict Conventional Commits (`feat(web): ...`, `fix(hud): ...`) for GitHub PR titles and commit subjects. Never prefix PR titles with emojis or persona labels such as `🎨 Palette:`, `⚡ Bolt:`, or `🛡️ Sentinel:`.
- **Frontend & WebFS Sync**: Use `npm` exclusively (`cd web && npm ci`, never `pnpm` or `yarn`). After any change in `web/`, rebuild and sync `internal/webfs/dist` (`task web-build` or `cd web && npm ci && npm run build && cd .. && rm -rf internal/webfs/dist && cp -r web/dist internal/webfs/dist`).
- **No Ephemeral Files**: Never commit `.Jules/`, `pnpm-lock.yaml`, or `yarn.lock`.
