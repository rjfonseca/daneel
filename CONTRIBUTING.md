# Contributing to Daneel

First off, thank you for considering contributing to **Daneel**! 🧠  
Whether you are fixing a bug, improving documentation, or adding a new feature for AI agent integration, your contributions are welcome.

---

## 🛠️ Development Setup

### Prerequisites

* **Go 1.27+** (Recommended: Go 1.27)
* **[mise](https://mise.jdx.dev/)** (recommended tool manager) or **[Task](https://taskfile.dev/)**
* **Git**

### Getting Started

1. **Fork and clone the repository:**
   ```bash
   git clone https://github.com/rjfonseca/daneel.git
   cd daneel
   ```

2. **Install developer tools via `mise`:**
   ```bash
   mise install
   ```

3. **Install Git hooks via `lefthook`:**
   ```bash
   task hooks
   # or
   lefthook install
   ```

---

## 🏗️ Project Architecture

Daneel follows the standard Go project layout with strict package encapsulation:

```text
daneel/
├── cmd/               # CLI presentation layer with Cobra
│   ├── root.go
│   ├── search.go
│   ├── mcp.go
│   └── ...
├── internal/          # Private domain logic & services
│   ├── config/        # XDG resolution & multi-vault configuration
│   ├── dream/         # Cognitive sleep consolidation & link self-healing
│   ├── guard/         # Public export privacy & leak auditor
│   ├── harvester/     # Multi-repository metadata ingestion
│   ├── mcp/           # Native Model Context Protocol (MCP) JSON-RPC server
│   ├── ops/           # Engineering leadership operations (AVD, 1:1 prep, backlog)
│   ├── search/        # SQLite FTS5 / BM25 pure Go search engine
│   ├── tui/           # Terminal UI with Charm Bubble Tea & Glamour
│   └── vault/         # Note parser & OKF specification model
├── main.go            # Minimal entrypoint
├── Taskfile.yml       # Task runner definitions
├── lefthook.yml       # Pre-commit / pre-push hooks
└── .golangci.yml      # Static analysis configuration
```

---

## 📋 Development Workflow

We use `Taskfile` to centralize all tasks:

```bash
# Format code (gofmt, goimports, gofumpt)
task fmt

# Run static analysis (golangci-lint)
task lint

# Run all unit and integration tests with coverage
task test

# Run performance benchmarks (search engine & indexer)
task bench

# Build static binary
task build
```

---

## 📐 Coding & Git Standards

### 1. Pure Go / Zero CGO
Daneel must compile as a static binary without external C dependencies (`modernc.org/sqlite`). Never introduce CGO dependencies into `internal/`.

### 2. Context Propagation
Always propagate `context.Context` from Cobra commands down to I/O and DB-bound operations (`engine.Search(ctx, ...)`, `engine.IndexVault(ctx, ...)`).

### 3. Structured Logging
Use Go's standard `log/slog` for internal debugging, always outputting to `os.Stderr`. **Never write unformatted debug text to `os.Stdout`**, as it corrupts the JSON-RPC stream used by AI agent MCP connections.

### 4. Conventional Commits
Please follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

* `feat: add new MCP tool for graph visualization`
* `fix: handle edge case in relative link resolution`
* `docs: update quickstart guide for Cursor IDE`
* `refactor: optimize SQLite FTS5 batch inserts`
* `test: add table-driven tests for vault parser`

---

## 🚀 Submitting a Pull Request

1. Create a feature branch (`git checkout -b feat/my-awesome-feature`).
2. Ensure all tests and linters pass (`task fmt && task lint && task test`).
3. Commit your changes with descriptive commit messages.
4. Push to your fork and submit a Pull Request targeting `main`.
5. Provide a clear summary of changes and test results in the PR description.

Release compatibility and required validation are documented in [the release checklist](docs/releases/checklist.md).
