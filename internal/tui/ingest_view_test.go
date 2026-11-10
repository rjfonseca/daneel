package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/rjfonseca/daneel/internal/config"
)

func TestIngestView_Initialization(t *testing.T) {
	tempVault := t.TempDir()
	styles := NewStyles()

	iv := NewIngestView(tempVault, nil, styles)
	if iv == nil {
		t.Fatal("NewIngestView returned nil")
	}

	if iv.Destination() != "inbox" {
		t.Errorf("expected default destination 'inbox', got '%s'", iv.Destination())
	}

	if len(iv.History()) != 0 {
		t.Errorf("expected empty initial history, got %d items", len(iv.History()))
	}

	if iv.IsInputMode() {
		t.Errorf("expected initial inputMode to be false")
	}

	iv.SetSize(120, 40)
	rendered := iv.View()
	if !strings.Contains(rendered, "File Browser") {
		t.Errorf("expected rendered view to contain 'File Browser', got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Target Vault Destination") {
		t.Errorf("expected rendered view to contain 'Target Vault Destination', got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "inbox") {
		t.Errorf("expected rendered view to contain 'inbox', got:\n%s", rendered)
	}
}

func TestIngestView_DestinationInput(t *testing.T) {
	tempVault := t.TempDir()
	styles := NewStyles()
	iv := NewIngestView(tempVault, nil, styles)

	// 1. Press 't' to enter destination input mode
	iv, _ = iv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	if !iv.IsInputMode() {
		t.Errorf("expected IsInputMode() to be true after pressing 't'")
	}

	// 2. Type "sources/external"
	iv.destInput.SetValue("sources/external")

	// 3. Press Enter to commit
	iv, _ = iv.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if iv.IsInputMode() {
		t.Errorf("expected IsInputMode() to be false after pressing Enter")
	}
	if iv.Destination() != "sources/external" {
		t.Errorf("expected destination 'sources/external', got '%s'", iv.Destination())
	}

	// 4. Press 't', then 'Esc' to cancel
	iv, _ = iv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	iv.destInput.SetValue("canceled/path")
	iv, _ = iv.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if iv.IsInputMode() {
		t.Errorf("expected IsInputMode() to be false after pressing Esc")
	}
	if iv.Destination() != "sources/external" {
		t.Errorf("expected destination to remain 'sources/external', got '%s'", iv.Destination())
	}
}

func TestIngestView_PaneFocusSwitch(t *testing.T) {
	tempVault := t.TempDir()
	styles := NewStyles()
	iv := NewIngestView(tempVault, nil, styles)

	if iv.focusPane != 0 {
		t.Errorf("expected initial focusPane to be 0, got %d", iv.focusPane)
	}

	// Press tab to switch focus
	iv, _ = iv.Update(tea.KeyMsg{Type: tea.KeyTab})
	if iv.focusPane != 1 {
		t.Errorf("expected focusPane to be 1 after Tab, got %d", iv.focusPane)
	}

	// Press tab again to switch back
	iv, _ = iv.Update(tea.KeyMsg{Type: tea.KeyTab})
	if iv.focusPane != 0 {
		t.Errorf("expected focusPane to be 0 after second Tab, got %d", iv.focusPane)
	}
}

func TestIngestView_IngestPath_SingleFile(t *testing.T) {
	tempVault := t.TempDir()
	tempSourceDir := t.TempDir()
	sampleNote := filepath.Join(tempSourceDir, "architectural-notes.md")
	noteContent := "# Architecture Decisions\n\nPreserve Zeroth Law invariants.\n"
	if err := os.WriteFile(sampleNote, []byte(noteContent), 0o600); err != nil {
		t.Fatalf("failed to write sample note: %v", err)
	}

	styles := NewStyles()
	iv := NewIngestView(tempVault, nil, styles)

	report, err := iv.IngestPath(sampleNote)
	if err != nil {
		t.Fatalf("IngestPath failed: %v", err)
	}
	if report == nil || len(report.Items) != 1 {
		t.Fatalf("expected 1 ingested item, got: %+v", report)
	}

	expectedVaultFile := filepath.Join(tempVault, "inbox", "architectural-notes.md")
	if _, err := os.Stat(expectedVaultFile); err != nil {
		t.Fatalf("expected ingested file at %s: %v", expectedVaultFile, err)
	}

	hist := iv.History()
	if len(hist) != 1 {
		t.Fatalf("expected 1 history item, got %d", len(hist))
	}
	if hist[0].RelPath != filepath.ToSlash(filepath.Join("inbox", "architectural-notes.md")) {
		t.Errorf("expected history RelPath 'inbox/architectural-notes.md', got '%s'", hist[0].RelPath)
	}
	if !strings.Contains(hist[0].Status, "Ingested") {
		t.Errorf("expected history status to contain 'Ingested', got '%s'", hist[0].Status)
	}

	iv.SetSize(120, 40)
	rendered := iv.View()
	if !strings.Contains(rendered, "architectural-notes.md") {
		t.Errorf("expected rendered view to show ingested file in history or preview, got:\n%s", rendered)
	}
}

func TestIngestView_IngestDirectory_Command(t *testing.T) {
	tempVault := t.TempDir()
	tempSourceDir := t.TempDir()
	note1 := filepath.Join(tempSourceDir, "doc1.md")
	note2 := filepath.Join(tempSourceDir, "doc2.md")
	_ = os.WriteFile(note1, []byte("# Note 1\nContent 1"), 0o600)
	_ = os.WriteFile(note2, []byte("# Note 2\nContent 2"), 0o600)

	styles := NewStyles()
	iv := NewIngestView(tempVault, nil, styles)
	iv.SetCurrentDirectory(tempSourceDir)

	// Press 'd' to ingest current directory
	iv, _ = iv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})

	hist := iv.History()
	if len(hist) != 2 {
		t.Fatalf("expected 2 history items after directory ingestion, got %d", len(hist))
	}

	expected1 := filepath.Join(tempVault, "inbox", "doc1.md")
	expected2 := filepath.Join(tempVault, "inbox", "doc2.md")
	if _, err := os.Stat(expected1); err != nil {
		t.Errorf("expected file %s to exist", expected1)
	}
	if _, err := os.Stat(expected2); err != nil {
		t.Errorf("expected file %s to exist", expected2)
	}
}

func TestAppModel_IngestTab(t *testing.T) {
	tempVault := t.TempDir()
	app := NewAppModel(tempVault, nil, config.TuiConfig{})
	if app == nil {
		t.Fatal("NewAppModel returned nil")
	}

	// 1. Verify "9: 📥 Ingest" is in app.tabs
	foundIngest := false
	for _, tab := range app.tabs {
		if strings.Contains(tab, "Ingest") {
			foundIngest = true
			break
		}
	}
	if !foundIngest {
		t.Errorf("expected Ingest tab in app.tabs, got: %v", app.tabs)
	}

	// 2. Press '9' to switch to ViewIngest
	newModel, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("9")})
	app = newModel.(*AppModel)
	if app.activeTab != ViewIngest {
		t.Errorf("expected activeTab to be ViewIngest after pressing '9', got %v", app.activeTab)
	}

	// 3. Render Ingest View via AppModel
	rendered := app.View()
	if !strings.Contains(rendered, "File Browser") || !strings.Contains(rendered, "Target Vault Destination") {
		t.Errorf("expected rendered app view to contain Ingest components, got:\n%s", rendered)
	}

	// 4. Switch back to search with '1'
	newModel, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})
	app = newModel.(*AppModel)
	if app.activeTab != ViewSearch {
		t.Fatalf("expected ViewSearch, got %v", app.activeTab)
	}

	// 5. Press 'i' to jump to ViewIngest
	newModel, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	app = newModel.(*AppModel)
	if app.activeTab != ViewIngest {
		t.Errorf("expected activeTab to be ViewIngest after pressing 'i', got %v", app.activeTab)
	}

	// 6. Test tui_switch_view "ingest"
	newModel, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})
	app = newModel.(*AppModel)

	newModel, _ = app.Update(TUICommandMsg{
		Command: "tui_switch_view",
		Args:    map[string]interface{}{"view_name": "ingest"},
	})
	app = newModel.(*AppModel)
	if app.activeTab != ViewIngest {
		t.Errorf("expected activeTab to be ViewIngest after tui_switch_view 'ingest', got %v", app.activeTab)
	}
}
