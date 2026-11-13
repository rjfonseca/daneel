//nolint:revive // package comment not required
package codeact

import (
	"context"
	"strings"
	"testing"
	"time"
)

// Minimal valid Wasm binary (8 bytes header)
var minimalWasm = []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}

func TestRunner_MinimalWasm(t *testing.T) {
	ctx := context.Background()
	runner := NewRunner()

	res, err := runner.ExecuteBytes(ctx, minimalWasm, "")
	if err != nil {
		t.Fatalf("ExecuteBytes failed on minimal wasm: %v", err)
	}

	if res.Duration <= 0 {
		t.Errorf("expected positive duration, got %v", res.Duration)
	}
}

func TestRunner_Timeout(t *testing.T) {
	runner := NewRunner(Config{
		MaxMemoryMB: 16,
		Timeout:     50 * time.Millisecond,
	})

	// Use an already-expired deadline to avoid OS timer granularity flakiness (e.g. ~15.6ms on Windows).
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	_, err := runner.ExecuteBytes(ctx, minimalWasm, "")
	if err == nil || !strings.Contains(err.Error(), "context") {
		t.Errorf("expected context timeout error, got %v", err)
	}
}

func TestRunner_InvalidBytes(t *testing.T) {
	ctx := context.Background()
	runner := NewRunner()

	_, err := runner.ExecuteBytes(ctx, []byte("invalid-wasm-bytes"), "")
	if err == nil {
		t.Fatal("expected error on invalid wasm binary, got nil")
	}
}
