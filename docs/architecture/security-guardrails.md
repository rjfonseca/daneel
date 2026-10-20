# Security Guardrails & The Four Laws of Robotics

Daneel implements a zero-trust audit engine for validating public markdown notes, preventing confidential data leaks, and ensuring graph integrity prior to publishing or static site export.

---

## 1. The Four Laws of Robotics Architecture

Inspired by Isaac Asimov's robotics lore, validation rules are organized hierarchically:

```
┌─────────────────────────────────────────────────────────────┐
│  Zeroth Law: Zero Secret & Credential Leakage               │
│  (API keys, JWTs, AWS/GCP secrets, PATs, Private Keys)     │
├─────────────────────────────────────────────────────────────┤
│  First Law: Privacy & Corporate Identity Protection         │
│  (Restricted corporate domains, private tracker tickets)    │
├─────────────────────────────────────────────────────────────┤
│  Second Law: Synaptic Graph Integrity                       │
│  (Broken markdown links, non-existent target files)         │
├─────────────────────────────────────────────────────────────┤
│  Third Law: Schema & Metadata Standards                     │
│  (Valid YAML frontmatter, OKF taxonomy, required tags)      │
└─────────────────────────────────────────────────────────────┘
```

---

## 2. Rule Hierarchy and Violations

### Zeroth Law: Zero Credential Leakage (`ZEROTH_LAW_ZERO_LEAKAGE`)
High-severity secret detection powered by regex token matching:
- **Authentication Tokens**: JWT patterns (`eyJ...`), `Authorization: Bearer` headers.
- **Cloud Provider Keys**: AWS Access Key IDs (`AKIA...`), Google Cloud API keys (`AIza...`).
- **Private Keys**: RSA, SSH, and TLS PEM blocks (`-----BEGIN ... PRIVATE KEY-----`).
- **Personal Access Tokens (PATs)**: GitHub tokens (`ghp_`, `github_pat_`), GitLab tokens (`glpat-`), Slack tokens (`xox[baprs]-`).

### First Law: Identity & Privacy Protection (`FIRST_LAW_PARTNER_PRIVACY`)
Prevents leaking local environment data or private corporate trackers:
- **Absolute Local Paths**: Flags paths matching `/home/<user>/(dev|work|github|projects)`.
- **Private Telemetry**: Flags un-sanitized time tracking markers.
- **Configured Restricted Domains**: Audits against user-defined domains in `.daneel.yaml` under `publishing.block.domains`.
- **Restricted Keywords**: Audits against user-defined sensitive keywords under `publishing.block.keywords`.

### Second Law: Graph Integrity (`SECOND_LAW_GRAPH_INTEGRITY`)
Ensures public notes do not expose dead links or private paths:
- **Unconverted WikiLinks**: Validates that notes in `public/` do not contain un-rendered `[[wiki-links]]`.
- **Broken Relative Links**: Verifies that any markdown link `[text](target.md)` points to an existing file in the public export path.
- **Leak Links Outside Public**: Prevents public notes from referencing non-public directories (e.g. `../notes/internal/`).

### Third Law: Metadata Standards (`THIRD_LAW_SCHEMA_STANDARDS`)
Validates that notes conform to Open Knowledge Framework (OKF) schema standards:
- Verifies presence and syntax of YAML frontmatter blocks (`---`).

---

## 3. CLI Auditing

The guard audit runs via `daneel guard`:

```bash
# Audit all notes in the public content directory
daneel guard --check

# Audit a specific note file
daneel guard --file public/content/architecture-overview.md

# Format report as JSON for CI/CD gates
daneel guard --check --json
```

### Exit Codes
- `0`: All audited files passed without violations.
- `1`: One or more security or integrity violations detected.
