# ADR 0005: Four Laws of Robotics Security Guardrails for Markdown Notes

* Status: accepted
* Date: 2026-09-10

## Context and Problem Statement

Second Brain vaults often contain a mix of private engineering notes, internal corporate references, architecture sketches, credentials/tokens pasted during debugging, and notes intended for public distribution or blogging.

Exporting notes without automated sanitization creates severe risks of credential leakage (API keys, JWTs, cloud provider secrets), corporate privacy violations, and broken graph links.

How should Daneel audit notes prior to export and enforce privacy and security guardrails?

## Decision Drivers

* Strict prevention of credential and secret leaks.
* Extensible rules for user-configured corporate domains and sensitive keywords.
* Validation of markdown graph integrity (broken links, missing targets).
* Clear hierarchical categorization aligned with lore and mental models.

## Considered Options

* **Option 1: Rely on generic third-party secret scanners (gitleaks, trufflehog)**
* **Option 2: Native Asimov Four Laws rule engine integrated directly into Daneel CLI and MCP**
* **Option 3: Manual review checklists**

## Decision Outcome

Chosen option: **Option 2: Native Asimov Four Laws rule engine (`daneel guard`)**, because:
- Integrates directly with Vault frontmatter parsing, Wikilinks resolvers, and relative markdown links.
- Employs a strict hierarchy:
  - **Zeroth Law**: Zero credential and token leakage (JWT, AWS, GCP, GitHub/GitLab PATs, Private Keys).
  - **First Law**: Privacy protection against local paths, corporate domains, and un-sanitized telemetry.
  - **Second Law**: Synaptic graph integrity (broken links, non-existent target files).
  - **Third Law**: Schema standards and valid frontmatter.
- Usable in CI/CD pipelines via `daneel guard --check --json`.

### Positive Consequences

* Deterministic audit prevents sensitive leaks before public publishing.
* Zero external tool dependencies required for vault verification.
