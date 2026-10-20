package routine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type mockRunner struct {
	mu       sync.Mutex
	calls    [][]string
	outputs  map[string][]byte
	errToRet error
}

func (m *mockRunner) Run(ctx context.Context, name string, args ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	call := append([]string{name}, args...)
	m.calls = append(m.calls, call)
	return m.errToRet
}

func (m *mockRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	call := append([]string{name}, args...)
	m.calls = append(m.calls, call)
	key := strings.Join(call, " ")
	if out, ok := m.outputs[key]; ok {
		return out, nil
	}
	return nil, m.errToRet
}

func (m *mockRunner) CombinedOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	return m.Output(ctx, name, args...)
}

func (m *mockRunner) hasCallWithPrefix(prefix ...string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, call := range m.calls {
		if len(call) >= len(prefix) {
			match := true
			for i, p := range prefix {
				if call[i] != p {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}
	return false
}

func TestSystemdAdapter_Install_And_Uninstall(t *testing.T) {
	unitDir := t.TempDir()
	runner := &mockRunner{outputs: make(map[string][]byte)}
	adapter := NewSystemdAdapter(unitDir, "/usr/local/bin/daneel", runner)

	r := Routine{
		Name:        "tidy",
		Command:     "tidy",
		Args:        []string{"--dry-run"},
		Schedule:    "hourly",
		Description: "Link self-healing",
		Enabled:     true,
	}

	ctx := context.Background()

	// 1. Install
	if err := adapter.Install(ctx, r); err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	servicePath := filepath.Join(unitDir, "daneel-tidy.service")
	timerPath := filepath.Join(unitDir, "daneel-tidy.timer")

	svcBytes, err := os.ReadFile(servicePath)
	if err != nil {
		t.Fatalf("failed to read service file: %v", err)
	}
	svcContent := string(svcBytes)
	if !strings.Contains(svcContent, "Type=oneshot") {
		t.Errorf("expected Type=oneshot in service, got:\n%s", svcContent)
	}
	if !strings.Contains(svcContent, "ExecStart=/usr/local/bin/daneel routine exec tidy --trigger=systemd") {
		t.Errorf("expected ExecStart in service, got:\n%s", svcContent)
	}
	if !strings.Contains(svcContent, "Description=Daneel Routine: Link self-healing") {
		t.Errorf("expected Description in service, got:\n%s", svcContent)
	}

	timerBytes, err := os.ReadFile(timerPath)
	if err != nil {
		t.Fatalf("failed to read timer file: %v", err)
	}
	timerContent := string(timerBytes)
	if !strings.Contains(timerContent, "OnCalendar=hourly") {
		t.Errorf("expected OnCalendar=hourly in timer, got:\n%s", timerContent)
	}
	if !strings.Contains(timerContent, "Persistent=true") {
		t.Errorf("expected Persistent=true in timer, got:\n%s", timerContent)
	}

	// Verify systemctl commands called
	if !runner.hasCallWithPrefix("systemctl", "--user", "daemon-reload") {
		t.Errorf("expected systemctl --user daemon-reload call")
	}
	if !runner.hasCallWithPrefix("systemctl", "--user", "enable", "--now", "daneel-tidy.timer") {
		t.Errorf("expected systemctl --user enable --now daneel-tidy.timer call")
	}

	// 2. Enable / Disable / RunNow
	if err := adapter.Disable(ctx, "tidy"); err != nil {
		t.Errorf("Disable failed: %v", err)
	}
	if !runner.hasCallWithPrefix("systemctl", "--user", "disable", "--now", "daneel-tidy.timer") {
		t.Errorf("expected disable call")
	}

	if err := adapter.Enable(ctx, "tidy"); err != nil {
		t.Errorf("Enable failed: %v", err)
	}
	if !runner.hasCallWithPrefix("systemctl", "--user", "enable", "--now", "daneel-tidy.timer") {
		t.Errorf("expected enable call")
	}

	if err := adapter.RunNow(ctx, "tidy"); err != nil {
		t.Errorf("RunNow failed: %v", err)
	}
	if !runner.hasCallWithPrefix("systemctl", "--user", "start", "daneel-tidy.service") {
		t.Errorf("expected start call")
	}

	// 3. Status and List
	status, err := adapter.Status(ctx, "tidy")
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if status.Name != "tidy" || status.Schedule != "hourly" || status.Description != "Link self-healing" {
		t.Errorf("unexpected status: %+v", status)
	}

	list, err := adapter.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 1 || list[0].Name != "tidy" {
		t.Errorf("unexpected list: %+v", list)
	}

	// 4. Uninstall
	if err := adapter.Uninstall(ctx, "tidy"); err != nil {
		t.Fatalf("Uninstall failed: %v", err)
	}
	if _, err := os.Stat(servicePath); !os.IsNotExist(err) {
		t.Errorf("service file still exists after uninstall")
	}
	if _, err := os.Stat(timerPath); !os.IsNotExist(err) {
		t.Errorf("timer file still exists after uninstall")
	}
}
