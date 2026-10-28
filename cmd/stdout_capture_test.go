//nolint:revive // package comment not required
package cmd

import (
	"bytes"
	"os"
	"testing"
)

// captureStdout runs fn with os.Stdout redirected to a pipe and returns what it wrote.
// The pipe is drained concurrently so large outputs never block on the OS pipe buffer
// (which is small on Windows and deadlocks if read only after fn returns).
func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		done <- buf.String()
	}()

	origStdout := os.Stdout
	os.Stdout = w
	runErr := fn()
	os.Stdout = origStdout
	_ = w.Close()

	out := <-done
	_ = r.Close()
	return out, runErr
}

func TestCaptureStdout_LargeOutputDoesNotBlock(t *testing.T) {
	payload := bytes.Repeat([]byte("x"), 1<<20) // 1 MiB, far above any OS pipe buffer
	out, err := captureStdout(t, func() error {
		_, werr := os.Stdout.Write(payload)
		return werr
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != len(payload) {
		t.Fatalf("expected %d bytes, got %d", len(payload), len(out))
	}
}
