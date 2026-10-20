# Getting Started with Development

This guide outlines setup, build procedures, and standard development workflows for contributing to Daneel.

## 1. Prerequisites

- **Go**: Version 1.27 or higher (CI: 1.27.2)
- **Node.js**: Version 26 (CI: 26.11.1) (`npm`)
- **Task**: Taskfile runner (`go-task/task`)
- **mise** (Optional): Polyglot runtime version manager

## 2. Initial Setup

Clone and install dependencies:
```bash
# Clone the repository
git clone https://github.com/rjfonseca/daneel.git
cd daneel

# Install web dependencies
task web-deps

# Install Lefthook git pre-commit hooks
task hooks
```

## 3. Common Build Recipes

```bash
# Build Go binary with embedded Web frontend
task build

# Run Go and Web tests
task test
cd web && npm test

# Regenerate Protobuf and ConnectRPC bindings
task proto-gen

# Install binary to ~/.local/bin/daneel
task install
```

## 4. Frontend & WebFS Synchronization

When working on `web/src`:
```bash
# Live frontend development server (Vite)
cd web && npm run dev

# Production build and WebFS sync (mandatory before commit)
cd web && npm run build && rm -rf ../internal/webfs/dist && cp -r dist ../internal/webfs/dist
```
