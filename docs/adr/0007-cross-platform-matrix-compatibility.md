# ADR 0007: Cross-Platform Matrix Compatibility and Path Relativization

* Status: accepted
* Date: 2026-09-11

## Context and Problem Statement

Daneel targets macOS (Darwin), Linux, and Windows across both CLI tools and GitHub Actions multi-OS test matrices (`[ubuntu-latest, macos-latest, windows-latest]`).

In CI and multi-platform environments, several platform-specific discrepancies occurred:
1. **Windows Process Detachment**: On Unix, detaching a daemon process from the calling terminal utilizes `syscall.SysProcAttr{ Setsid: true }`. On Windows, `Setsid` is not supported in `syscall.SysProcAttr`, causing compilation failure of the `cmd` package.
2. **Windows Drive Letters & 8.3 Short Paths**: Computing relative paths between absolute paths using `filepath.Rel(vaultRoot, targetAbs)` fails or generates invalid paths on Windows runners when path casing differs (e.g. `c:\` vs `C:\`) or short path aliases (`RUNNER~1`) are present.
3. **macOS Unix Socket Path Length**: Darwin BSD sockets strictly enforce a 104-byte limit on `sockaddr_un.sun_path`. Standard Go temporary directories (`t.TempDir()`) on macOS easily exceed this limit, causing `net.Listen("unix", ...)` to fail with `bind: invalid argument`.

## Decision Drivers

* Strict 100% CI pass rate across Linux, macOS, and Windows matrix targets.
* Idiomatic Go OS build tags (`//go:build !windows` vs `//go:build windows`).
* Vault portability across heterogeneous filesystems and operating systems.
* Fast, robust IPC unit tests without path truncation or bind failures.

## Considered Options

* **Option 1: Drop Windows and macOS testing from CI matrix** (Linux-only).
* **Option 2: Decoupled OS build abstractions and pure Vault-relative path arithmetic**.

## Decision Outcome

Chosen option: **Option 2: Decoupled OS build abstractions and pure Vault-relative path arithmetic**, because:
- Daneel is an open-source tool intended to run across diverse developer workstations.
- Decoupling OS-specific process attributes into `cmd/daemon_unix.go` and `cmd/daemon_windows.go` allows clean platform compilation without runtime compromises.
- Resolving links and wikilinks using relative directories (`filepath.Dir(sourceRelPath)` and `filepath.Rel(sourceDirRel, targetRelPath)`) eliminates dependence on absolute path prefixes, drive letters, and symlinks.
- Using short temporary paths (`os.MkdirTemp("", "dnl*")`) for Unix socket tests guarantees socket path lengths stay well below Darwin's 104-byte boundary.

### Positive Consequences

* Seamless compilation and test suite execution across Ubuntu, macOS, and Windows.
* Enhanced robustness against filesystem path variations in CI environments.
