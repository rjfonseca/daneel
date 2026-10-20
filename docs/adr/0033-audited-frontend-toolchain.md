# ADR 0033: Audited Frontend Toolchain

* Status: accepted
* Date: 2026-10-19

## Context and Problem Statement

The release audit found known advisories in frontend development dependencies. Backend vulnerability scanning does not cover the npm build and test toolchain. Remaining on Tailwind CSS 3 retains dependencies without patched releases, while the existing Vitest 3 tree includes affected worker-pool packages.

## Decision Drivers

- Require an auditable build and test environment for release contributors.
- Preserve the cosmic HUD, keyboard focus, responsive navigation and note reader.
- Keep shared design tokens as the source of truth.
- Enforce the same dependency policy before pushes and tagged releases.

## Considered Options

- Ignore development-only findings or suppress individual advisories.
- Override vulnerable dependencies with incompatible replacements.
- Upgrade the affected toolchain and validate its compatibility.

## Decision Outcome

Upgrade to patched Vitest 4 and Tailwind CSS 4 releases. Use the dedicated Tailwind PostCSS plugin, retain the JavaScript theme configuration backed by shared TypeScript tokens, and migrate renamed utility classes to preserve their previous behavior. Keep explicit compatibility styles for changed framework defaults.

Run `npm run audit` against the complete lockfile with `--include=dev --audit-level=low`, even when npm is configured to omit development dependencies. Enforce it in the shared CI workflow and pre-push checks. Do not suppress advisories or use forced incompatible dependency overrides. Production-only audit results remain useful context but do not replace the full audit.

The typography plugin pins an affected selector parser. Scope an override to that plugin for patched `postcss-selector-parser` 7.1.6 or newer. Its major change concerns safe iteration during insertion; the plugin's selector traversal and serialization API remains available. Validate the override through actual typography compilation and note-reader browser tests, and remove it when the plugin updates its dependency.

### Consequences

- Dependency updates can block a release until new findings are resolved.
- Framework upgrades require unit, type, lint, generated-asset and real browser validation.
- The Web interface requires Chrome 111+, Firefox 128+ or Safari 16.4+, matching the CSS framework's browser requirements.
- Framework style defaults can change; existing interaction and camera invariants must remain intact.
