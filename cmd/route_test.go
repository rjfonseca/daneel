//nolint:revive // package comment not required
package cmd

import (
	"strings"
	"testing"
)

func TestRouteCmd(t *testing.T) {
	out, err := captureStdout(t, func() error {
		return routeCmd.RunE(routeCmd, []string{"briefing", "matinal"})
	})
	if err != nil {
		t.Fatalf("routeCmd failed: %v", err)
	}

	if !strings.Contains(out, "Fast-Path") || !strings.Contains(out, "routine_morning") {
		t.Errorf("expected Fast-Path routine_morning in routeCmd output, got: %s", out)
	}
}

func TestRouteCmd_Toon(t *testing.T) {
	origToon := toonOut
	toonOut = true
	defer func() {
		toonOut = origToon
	}()

	out, err := captureStdout(t, func() error {
		return routeCmd.RunE(routeCmd, []string{"status", "do", "cofre"})
	})
	if err != nil {
		t.Fatalf("routeCmd --toon failed: %v", err)
	}

	if !strings.Contains(out, "intent: vault_health") {
		t.Errorf("expected TOON output with intent: vault_health, got: %s", out)
	}
}
