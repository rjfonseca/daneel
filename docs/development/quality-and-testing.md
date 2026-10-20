# Quality Gates & Testing Standards

Daneel enforces strict automated quality gates to protect architectural invariants, visual rendering precision, and security.

## 1. Quality Gates Overview

Every commit is validated via **Lefthook** (`lefthook.yml`) executing parallel checks:

| Gate | Tool | Target | Success Standard |
|---|---|---|---|
| **Go Formatting** | `task fmt` (`gofumpt` / `go fmt`) | Backend Go files | Zero diffs |
| **Go Static Analysis** | `task lint` (`golangci-lint`) | Backend Go files | Zero warnings / errors |
| **Security Audit** | `task vulncheck` (`govulncheck`) | Dependencies | Zero affected symbols |
| **Backend Tests** | `task test` (`go test -v -cover`) | All Go packages | 100% passing tests |
| **Frontend Lint** | `npm run lint` (`eslint --max-warnings=0`) | Web source and tests | Zero errors or warnings |
| **Frontend Dependency Audit** | `npm run audit` (`npm audit --include=dev --audit-level=low`) | Complete npm lockfile, including development dependencies | Zero known advisories |
| **Frontend Types** | `npm run typecheck` (`tsc`) | Application and unit/browser tests | Zero type errors |
| **Frontend Tests** | `npm run test` (`vitest`) | Web test suites | 100% passing tests |
| **WebFS Drift Verification** | `task web-verify-sync` (`git diff`) | Embedded assets (`internal/webfs/dist`) | Zero drift from `web/src` |

## 2. Test Execution Commands

```bash
# Run all Go tests
go test -v -cover ./...

# Run all Web unit tests
cd web && npm test

# Run WebFS drift verification
task web-verify-sync

# Run Playwright E2E visual and camera invariant tests
cd web && npx playwright test tests/e2e/camera-invariants.spec.ts
```

## 3. E2E Invariant Test Specifications

The Playwright suite in [`web/tests/e2e/camera-invariants.spec.ts`](../../web/tests/e2e/camera-invariants.spec.ts) runs a headless Chromium instance to verify:
1. **Satellite Centering**: Target satellite projects within 25px of the open drawer center `(width - 440) / 2`.
2. **Cluster Bounding Box**: Focused nebulae occupy between 65% and 90% of the visible viewport.
3. **English Chrome**: All HUD labels, edge mode tooltips, and toasts are strictly in English.

## 4. WebFS Drift & Anti-Tampering Protection

To prevent direct or manual modifications to `internal/webfs/dist` and ensure all embedded web assets are 100% reproducible:
- **Lefthook Git Hooks** (`pre-commit` & `pre-push`): Runs `task web-verify-sync` whenever frontend files or dist are modified, ensuring dist matches a clean build from `web/src`.
- **CI Workflow** (`.github/workflows/ci.yml`): Rebuilds `web/src` via `npm ci && npm run build` and runs `git diff --exit-code internal/webfs/dist`. Pull requests containing manual edits or stale dist files are rejected.
- **Release Workflow** (`.github/workflows/release.yml`): Rebuilds web assets on the isolated GitHub Actions runner and validates integrity before GoReleaser packages the official binaries.


## Release gates and isolated checks

`task test` and `task bench` clear Git's repository-local environment variables
before invoking nested tests. This prevents inherited hook context from redirecting
temporary-repository commands into the contributor's checkout. The regression
test runs against a disposable parent repository and verifies that its HEAD,
index and configuration remain unchanged. CI exercises it with pinned Task 3.54.0
on each supported operating system.

CI is reusable through `workflow_call`; the release job requires that workflow
on the tagged commit. The OS matrix runs `python scripts/smoke.py` against a
fresh local installation. Before publishing a tagged draft, a separate native
Linux/macOS/Windows matrix downloads the matching uploaded archive, verifies its
checksum and runs the same smoke against its extracted binary. Browser checks run camera invariants and console-error
checks against a temporary neutral vault using Playwright-managed Chromium.
No existing local server is reused. Failure traces are retained as CI artifacts.

See the [release checklist](../releases/checklist.md) for snapshot, archive,
SBOM and provenance verification. Frontend lint rejects warnings. Both application and test TypeScript contracts are checked.

## Production browser checks

After `npm test && npm run build` in `web`, run:

```bash
DANEEL_E2E_PREVIEW=1 npx playwright test
```

The preview serves the production chunks and proxies RPCs to the isolated test
backend. The deferred-panel check verifies that Markdown and operational tools
are absent from initial requests, then exercises lazy search, note rendering and
cockpit state across closing and reopening. CI uses this production profile.
Generated coverage files are excluded from lint; source and test rules remain enabled.

To keep a development server running on port 5173 while testing, build the embedded
assets and run the same suite against the isolated Go server on port 8088:

```bash
task --force web-build
cd web && DANEEL_E2E_EMBEDDED=1 npx playwright test
```

This mode uses a temporary neutral vault and the generated WebFS assets. It does
not reuse the development server or write to the user's vault.
