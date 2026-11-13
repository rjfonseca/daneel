//nolint:revive // package comment not required
package codeact

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/tetratelabs/wazero/sys"
)

// Config defines execution constraints for the CodeAct sandbox.
type Config struct {
	MaxMemoryMB int           `json:"max_memory_mb"`
	Timeout     time.Duration `json:"timeout"`
}

// DefaultConfig returns safe production defaults.
func DefaultConfig() Config {
	return Config{
		MaxMemoryMB: 16,
		Timeout:     5 * time.Second,
	}
}

// Result captures sandbox execution stdout, stderr, and performance metrics.
type Result struct {
	Stdout     string        `json:"stdout"`
	Stderr     string        `json:"stderr"`
	ExitCode   uint32        `json:"exit_code"`
	Duration   time.Duration `json:"duration"`
	DurationMs int64         `json:"duration_ms"`
}

// Runner provides secure, ephemeral WebAssembly execution via pure Go wazero.
type Runner struct {
	cfg Config
}

// NewRunner creates a new CodeAct Runner with configuration.
func NewRunner(cfg ...Config) *Runner {
	c := DefaultConfig()
	if len(cfg) > 0 {
		if cfg[0].MaxMemoryMB > 0 {
			c.MaxMemoryMB = cfg[0].MaxMemoryMB
		}
		if cfg[0].Timeout > 0 {
			c.Timeout = cfg[0].Timeout
		}
	}
	return &Runner{cfg: c}
}

// ExecuteBytes runs raw Wasm bytecode inside the bounded sandbox.
func (r *Runner) ExecuteBytes(ctx context.Context, wasmBytes []byte, stdin string) (*Result, error) {
	if len(wasmBytes) == 0 {
		return nil, fmt.Errorf("wasm bytecode is empty")
	}

	timeout := r.cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := execCtx.Err(); err != nil {
		return nil, err
	}

	// Memory limit: 1 page = 64KB (16MB = 256 pages)
	memMB := r.cfg.MaxMemoryMB
	if memMB <= 0 || memMB > 1024 {
		memMB = 16
	}
	maxPages := uint32(memMB) * 16

	rConfig := wazero.NewRuntimeConfig().WithMemoryLimitPages(maxPages)
	runtime := wazero.NewRuntimeWithConfig(execCtx, rConfig)
	defer runtime.Close(execCtx) //nolint:errcheck

	wasi_snapshot_preview1.MustInstantiate(execCtx, runtime)

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer

	modConfig := wazero.NewModuleConfig().
		WithStdin(strings.NewReader(stdin)).
		WithStdout(&stdoutBuf).
		WithStderr(&stderrBuf).
		WithArgs("codeact")

	start := time.Now()
	mod, err := runtime.InstantiateWithConfig(execCtx, wasmBytes, modConfig)

	duration := time.Since(start)
	if duration <= 0 {
		duration = 1 * time.Microsecond
	}

	exitCode := uint32(0)
	if err != nil {
		if sysErr, ok := err.(*sys.ExitError); ok {
			exitCode = sysErr.ExitCode()
		} else {
			return nil, fmt.Errorf("wasm execution failed: %w", err)
		}
	}
	if mod != nil {
		_ = mod.Close(execCtx)
	}

	return &Result{
		Stdout:     stdoutBuf.String(),
		Stderr:     stderrBuf.String(),
		ExitCode:   exitCode,
		Duration:   duration,
		DurationMs: duration.Milliseconds(),
	}, nil
}

// ExecuteBase64 decodes base64-encoded Wasm bytes and executes them.
func (r *Runner) ExecuteBase64(ctx context.Context, b64 string, stdin string) (*Result, error) {
	bytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 wasm bytecode: %w", err)
	}
	return r.ExecuteBytes(ctx, bytes, stdin)
}

// ExecuteFile reads a Wasm file from the filesystem and executes it.
func (r *Runner) ExecuteFile(ctx context.Context, path string, stdin string) (*Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read wasm file: %w", err)
	}
	return r.ExecuteBytes(ctx, data, stdin)
}
