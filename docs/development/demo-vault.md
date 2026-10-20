# Demonstration vault and clean installation

The demonstration data uses fictional content and does not depend on a private
vault, credentials, external agents or user services.

```bash
daneel init --local --yes ./demo-vault
cd demo-vault
mkdir -p notes/architecture
printf '# Cache design\n\nA cache reduces repeated work.\n' > notes/architecture/cache.md
daneel search cache
daneel serve
```

Open the local URL printed by `serve`. Connect an MCP client with command
`daneel` and arguments `["--vault-path", "<absolute-demo-vault-path>", "mcp"]`.
Use `tools/list`, then `get_note_content` with `rel_path` set to
`notes/architecture/cache.md`.

For automated verification from the checkout:

```bash
python scripts/smoke.py
# Or test an extracted release binary:
python scripts/smoke.py --binary ./daneel
cd web
npx playwright install chromium
npx playwright test tests/e2e/camera-invariants.spec.ts tests/e2e/no-console-errors.spec.ts
```

The smoke script installs into a temporary directory, isolates XDG and Git
configuration, and exercises init, search, read, MCP and the embedded Web server.
Playwright starts a separate temporary demonstration vault and cleans it up when
the backend exits. It never reuses an existing application server.
