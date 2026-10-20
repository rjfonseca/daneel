# Release verification

## CI repair validation — 2026-10-21

The next candidate includes the short-socket, JSON-escaping, SQLite-cleanup,
portable compiler-error and persisted-timing fixture corrections, plus the
search dialog's asynchronous focus-wrap assertion. CI uses Go 1.27.3 and
Python 3.13 with UTF-8 mode across Linux, macOS and Windows. Workflow syntax,
Git-hook isolation, full npm audit and archive-checksum regressions are gates.

Fresh local Linux checks passed: `npm ci`, all-dependency npm audit (zero
advisories), 309 frontend tests, lint and application/test types, Go tests with
coverage, Go lint, reachable-symbol vulnerability scanning, workflow validation,
GoReleaser configuration, seven Python verifier regressions, source installation
smoke and search benchmarks. Vite 6.4.5 resolves the remaining editor-launch and
development-server advisories; Vitest 4.1.12 and Tailwind CSS 4.3.3 remain in use.
The audit also passes with `npm_config_omit=dev` because the gate explicitly
includes development dependencies.

All seven targeted embedded-production Chromium checks passed: camera
invariants, bundled font, console errors, forced-colors focus, short-viewport
capture, daily-note dates and search/menu keyboard focus. A forced WebFS rebuild
and synchronization check passed without generated-asset drift.

Hosted results must be checked on this candidate's exact commit. These local
checks do not establish native macOS/Windows execution or a published release.

## Local release-readiness validation — 2026-10-20

Validated on Linux amd64 against the readiness working tree based on `ed0cc7b`.
The generated version is `0.0.1-snapshot`: it is a local validation build, not a
published release candidate or v1.0.0. The changelog describes intended 1.0 content;
only signed tags and observed hosted release gates establish publication status.

| Check | Result |
| --- | --- |
| Clean frontend installation | `npm ci --ignore-scripts --no-audit --no-fund` passed from the committed lockfile |
| Full npm dependency audit | Zero advisories, including development dependencies; also passed with `npm_config_omit=dev` |
| Frontend unit tests | 309 tests in 49 files passed with Vitest 4.1.12 |
| Frontend lint and types | Zero lint warnings/errors; application and test TypeScript checks passed |
| Production build | Passed with Tailwind CSS 4.3.3; shared design tokens retained and local fonts bundled |
| Embedded assets | Forced rebuild and `web-verify-sync` passed against a temporary Git index |
| Browser regressions | Bundled font and forced-colors input outline tests passed after observed failures before their fixes; exact search-dialog Tab order and asynchronous focus wrapping passed five repeated embedded-browser runs |
| Full embedded-production Chromium suite | All 30 tests passed in 12.8 minutes against the isolated neutral vault; note framing checks enforce the actual dynamic bounds and drawer centering |
| Go tests and lint | All packages passed, including `task test` with inherited hook context; repository HEAD, index and configuration stayed unchanged; golangci-lint reported zero issues |
| Validation runner regressions | All three Python tests passed, including disposable-repository Git isolation |
| Native fixture regressions | Short socket paths verified under a long temporary-directory prefix; compiler failure preserves raw bytes; workspace paths encoded as JSON; test-owned SQLite stores close before cleanup; RPC timing values match the persisted engine report |
| Fresh-home workspace and Dream Cycle | Missing and empty workspace reports retain valid metadata; a fresh home completes catalog persistence inside the vault; regressions observed failing before the fix and passing afterward |
| Go vulnerability scan | Zero affected symbols or imported packages; one module-only finding remains outside imported code |
| Workflow and release configuration | Actionlint 1.7.7 and GoReleaser 2.17.1 checks passed; referenced action tags resolved |
| Snapshot distribution | Six Linux/macOS/Windows amd64/arm64 archives and six populated SPDX SBOMs produced with Syft 1.54.1 |
| Checksums | All 12 archive and SBOM checksums verified |
| Native archive installation | Init, search, read, MCP, embedded Web and foreign-origin rejection passed |
| Neutral vault upgrade | Index rebuild, doctor and search passed; all 21 Markdown/configuration files and an independent backup retained their original bytes |
| Independent review | Initial outline/audit findings corrected; source, runner isolation, camera assertions, fixture setup budget, repository rulesets and single-root publication policy reviewed with no further actionable issues |

The source installation baseline used for the upgrade fixture was built from
`ed0cc7b`. Both binaries ran with isolated XDG directories and a temporary neutral
vault. The backup was outside the vault. This demonstrates preservation across
the tested upgrade; it does not replace platform-specific candidate testing.

### Hosted repository setup — 2026-10-20

The recreated repository is public. GitHub API verification confirms private
vulnerability reporting and dependency alerts are enabled, with secret scanning
and secret-scanning push protection active. This report captures pre-push
validation; branch and tag controls, native CI, candidate artifacts and provenance
must be verified in the hosted workflow. See the repository Actions and Releases
for publication evidence without adding commits to the initial source history.

### Hosted gates still required

The checked-in [GitHub setup](github-setup.md) and ruleset JSON files describe
intended controls. The API checks above establish only the settings already
observed. Verify branch and release-tag rules, native macOS/Windows jobs, artifact
provenance and the downloaded candidate before stable publication. No candidate or stable tag
was created by the local validation above.

## Historical hardening verification — 2026-10-14

Validated locally on Linux amd64 from the working tree on
`release/1.0-hardening`. No tag or release was published.

| Check | Result |
| --- | --- |
| `go test ./...` | Passed all packages |
| `golangci-lint run ./...` | Zero issues |
| `govulncheck -show verbose ./...` | Zero affected symbols or imported packages; module-only finding described below |
| `cd web && npm test` | 203 tests in 30 files passed |
| `cd web && npm run lint` | Zero errors; 98 existing warnings |
| Camera invariants and no-console-errors Playwright suites | Both passed with the isolated fixture and Playwright Chromium |
| Clean `npm run build` and WebFS directory comparison | Passed; embedded assets unchanged |
| `task web-verify-sync` | Passed |
| `actionlint` 1.7.7 | Passed |
| `goreleaser check` | Passed |
| GoReleaser 2.17.1 snapshot, Syft 1.54.1 | Six Linux/macOS/Windows amd64/arm64 archives, six populated SPDX SBOMs, all checksums verified |
| `python scripts/smoke.py` | Clean install, init, search, read, MCP, embedded Web and origin protection passed |
| `python scripts/smoke.py --archive dist/daneel_0.0.1-snapshot_linux_amd64.tar.gz` | Extracted native archive passed the same workflow |
| Independent review | Initial findings corrected; second review found no remaining Critical or Important issues |

Regression tests cover path traversal and symlink escapes, foreign Host/Origin
requests, backup failure preservation, exact permissions under restrictive umask,
dry-run vault snapshots, MOC entry retention, legacy feedback continuity,
quarantine restoration and preservation of retained repository notes.

## Known validation limits

The vulnerability scanner reports [GO-2026-5932](https://pkg.go.dev/vuln/GO-2026-5932)
for the unmaintained `golang.org/x/crypto/openpgp` package in a required module.
That package is not imported and no affected symbols are reachable in Daneel.
The advisory has no fixed version; this is not a reachable vulnerability finding.

The frontend lint and chunk-size warnings recorded in the original snapshot were
resolved by the Web follow-up below.

macOS and Windows artifacts were cross-compiled, not executed natively here.
The reusable CI matrix runs native platform tests and clean installation smoke
checks when the repository is available. Artifact attestation issuance, draft
promotion, private vulnerability reporting and repository/tag protections require
hosted GitHub verification. See the [release checklist](checklist.md).

## Web follow-up — 2026-10-14

The Web polish was validated on Linux amd64 on `web/release-polish`.

| Check | Result |
| --- | --- |
| Frontend lint | Zero warnings or errors; `--max-warnings=0` is enforced |
| TypeScript | Application and all unit/browser test files pass |
| Vitest | 206 tests in 31 files pass |
| Production Playwright | Camera, console-error and deferred-panel checks all pass |
| Go suite | Passed |
| Build | No large-chunk warning; no warning threshold increase |
| Embedded assets | Generated by `task web-build` and compared with `web/dist` |

| JavaScript measurement | Before (bytes) | After (bytes) | Reduction |
| --- | ---: | ---: | ---: |
| Main entry | 610,411 | 172,131 | 71.8% |
| Initial entry plus static/preload dependencies | 850,967 | 548,698 | 35.5% |
| Initial JavaScript, gzip | 245,326 | 159,212 | 35.1% |

Measurements sum the entry and module-preload JavaScript referenced by production
`index.html`; gzip sizes use the same Python gzip settings for both builds.
They describe transferred asset sizes, not a measured improvement in page latency.
First use of deferred panels still downloads their code.

Browser verification confirms that the cockpit and Markdown are absent from
initial requests, then checks opening tools, retaining the active cockpit tab
across closing/reopening, searching and rendering a note. Unit regressions cover
loading feedback, state retention and failed panel downloads without hiding the
galaxy. Browser checks run against the production preview, not only Vite dev mode.
