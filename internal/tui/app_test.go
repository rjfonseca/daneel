//nolint:revive // package comment not required
package tui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/rjfonseca/daneel/internal/config"

	tea "github.com/charmbracelet/bubbletea"
)

func TestAppModelLifecycle(t *testing.T) {
	tempDir := t.TempDir()

	// Create minimal folders
	_ = filepath.Join(tempDir, "notes")
	_ = filepath.Join(tempDir, "meta", "pessoas")

	app := NewAppModel(tempDir, nil, config.TuiConfig{})
	if app == nil {
		t.Fatal("NewAppModel returned nil")
	}

	// Test window resize message
	newModel, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = newModel.(*AppModel)

	if app.width != 120 || app.height != 40 {
		t.Errorf("expected dimensions 120x40, got %dx%d", app.width, app.height)
	}

	// Test Tab Switching: Tab 1 -> Tab 2 -> Tab 3 -> Tab 4 -> Tab 5
	tabs := []ViewType{ViewDashboard, ViewWorkspace, ViewPeople, ViewRoutines, ViewSearch}
	keys := []string{"2", "3", "4", "5", "1"}

	for i, k := range keys {
		newModel, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)})
		app = newModel.(*AppModel)
		if app.activeTab != tabs[i] {
			t.Errorf("expected tab %v after pressing %s, got %v", tabs[i], k, app.activeTab)
		}
	}

	// Test View rendering (must not panic)
	rendered := app.View()
	if len(rendered) == 0 {
		t.Errorf("expected non-empty rendered view")
	}

	// Test Help overlay
	newModel, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	app = newModel.(*AppModel)
	if !app.showHelp {
		t.Errorf("expected showHelp to be true")
	}
	helpRendered := app.View()
	if len(helpRendered) == 0 {
		t.Errorf("expected non-empty help view")
	}

	// Close help
	newModel, _ = app.Update(tea.KeyMsg{Type: tea.KeyEscape})
	app = newModel.(*AppModel)
	if app.showHelp {
		t.Errorf("expected showHelp to be false after escape")
	}

	// Test Vim-style Search Modal Interaction
	// 1. Enter Search mode with '/'
	newModel, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	app = newModel.(*AppModel)
	if !app.isChildInInputMode() {
		t.Errorf("expected app to be in search input mode after pressing '/'")
	}

	// 2. In Search Mode, typing '2' must NOT switch to tab 2
	newModel, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	app = newModel.(*AppModel)
	if app.activeTab != ViewSearch {
		t.Errorf("expected active tab to remain ViewSearch when typing in search mode, got %v", app.activeTab)
	}
	if app.searchView.searchInput.Value() != "2" {
		t.Errorf("expected searchInput value to be '2', got '%s'", app.searchView.searchInput.Value())
	}

	// 3. Press Esc to exit Search Mode back to Normal Mode
	newModel, _ = app.Update(tea.KeyMsg{Type: tea.KeyEscape})
	app = newModel.(*AppModel)
	if app.isChildInInputMode() {
		t.Errorf("expected search mode to be deactivated after pressing escape")
	}

	// 4. In Normal Mode, pressing '2' MUST now switch to Tab 2
	newModel, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	app = newModel.(*AppModel)
	if app.activeTab != ViewDashboard {
		t.Errorf("expected tab to switch to ViewDashboard after pressing '2' in normal mode, got %v", app.activeTab)
	}

	// 5. Test 'q' in Normal Mode quits the application
	newModel, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	app = newModel.(*AppModel)
	if !app.quitting {
		t.Errorf("expected app.quitting to be true after pressing 'q' in normal mode")
	}
}

func TestAppModelTelemetryTab(t *testing.T) {
	tempDir := t.TempDir()

	app := NewAppModel(tempDir, nil, config.TuiConfig{})
	if app == nil {
		t.Fatal("NewAppModel returned nil")
	}

	// Verify Telemetry tab exists in tabs
	foundTelemetry := false
	for _, tab := range app.tabs {
		if strings.Contains(tab, "Telemetry") {
			foundTelemetry = true
			break
		}
	}
	if !foundTelemetry {
		t.Errorf("expected Telemetry tab in app.tabs, got: %v", app.tabs)
	}

	// Test pressing '8' switches to ViewTelemetry
	newModel, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("8")})
	app = newModel.(*AppModel)
	if app.activeTab != ViewTelemetry {
		t.Errorf("expected activeTab to be ViewTelemetry after pressing '8', got %v", app.activeTab)
	}

	// Test rendering ViewTelemetry
	rendered := app.View()
	if !strings.Contains(rendered, "AI FinOps") && !strings.Contains(rendered, "Telemetry") {
		t.Errorf("expected rendered app view to contain Telemetry headers, got:\n%s", rendered)
	}

	// Switch back to search
	newModel, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})
	app = newModel.(*AppModel)
	if app.activeTab != ViewSearch {
		t.Fatalf("expected ViewSearch, got %v", app.activeTab)
	}

	// Test pressing 'T' switches directly to ViewTelemetry
	newModel, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("T")})
	app = newModel.(*AppModel)
	if app.activeTab != ViewTelemetry {
		t.Errorf("expected activeTab to be ViewTelemetry after pressing 'T', got %v", app.activeTab)
	}

	// Test TUICommandMsg switch view to telemetry and finops
	newModel, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})
	app = newModel.(*AppModel)

	newModel, _ = app.Update(TUICommandMsg{
		Command: "tui_switch_view",
		Args:    map[string]interface{}{"view_name": "telemetry"},
	})
	app = newModel.(*AppModel)
	if app.activeTab != ViewTelemetry {
		t.Errorf("expected activeTab to be ViewTelemetry after tui_switch_view 'telemetry', got %v", app.activeTab)
	}

	newModel, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})
	app = newModel.(*AppModel)

	newModel, _ = app.Update(TUICommandMsg{
		Command: "tui_switch_view",
		Args:    map[string]interface{}{"view_name": "finops"},
	})
	app = newModel.(*AppModel)
	if app.activeTab != ViewTelemetry {
		t.Errorf("expected activeTab to be ViewTelemetry after tui_switch_view 'finops', got %v", app.activeTab)
	}
}
