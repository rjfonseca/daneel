package dream

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func snapshotVault(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if entry.IsDir() {
			result[rel] = "directory"
			return nil
		}
		data, err := os.ReadFile(path) //nolint:gosec // isolated test snapshot, no concurrent filesystem changes
		if err != nil {
			return err
		}
		result[rel] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestDreamDryRunPreservesVault(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for path, content := range map[string]string{
		".daneel.yaml":               "vault:\n  title: Test Vault\nworkspace:\n  work_dir: ./empty-workspace\n",
		"notes/systems/hello.md":     "# Hello\n[[World]]\n",
		"notes/systems/world.md":     "# World\n",
		"daily/2026-10-01.md":        "# Daily\n[Alan](../notes/pessoas/alan.md) discussed system design.\n",
		"notes/pessoas/alan.md":      "# Alan Turing\n## 🤝 Histórico de Interações\n",
		"meta/dashboard-metricas.md": "# Existing dashboard\n",
	} {
		target := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	before := snapshotVault(t, root)
	if _, err := NewDreamEngine(root).RunFullCycle("2026-10-01", true, true); err != nil {
		t.Fatal(err)
	}
	if after := snapshotVault(t, root); !reflect.DeepEqual(before, after) {
		t.Fatalf("dry-run changed vault: before=%v after=%v", before, after)
	}
}

func TestSyncMOCsKeepsAllNewEntries(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "notes", "systems")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{"moc-systems.md": "# Systems\n", "alpha.md": "# Alpha\n", "beta.md": "# Beta\n"} {
		if err := os.WriteFile(filepath.Join(dir, path), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := SyncMOCs(root, true); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "moc-systems.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"alpha.md", "beta.md"} {
		if !strings.Contains(string(data), name) {
			t.Fatalf("lost MOC entry %s: %s", name, data)
		}
	}
}

func TestDreamKeepsLegacyLocalFeedbackCache(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	legacy := filepath.Join(root, ".agents", "cache")
	if err := os.MkdirAll(legacy, 0700); err != nil {
		t.Fatal(err)
	}
	history := `{"feedback_history":[{"topic":"Keep useful history","note":"Prior correction"}]}`
	if err := os.WriteFile(filepath.Join(legacy, "agent_learnings.json"), []byte(history), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewDreamEngine(root).SetSynthesize(false).RunFullCycle("2026-10-01", false, false); err != nil {
		t.Fatal(err)
	}
	learnings := LoadLearnings(root)
	if len(learnings.FeedbackHistory) != 1 || learnings.FeedbackHistory[0].Topic != "Keep useful history" {
		t.Fatalf("legacy feedback became inaccessible: %+v", learnings.FeedbackHistory)
	}
	if err := RecordFeedback(root, "New entry", "correction", "keep", "New correction"); err != nil {
		t.Fatal(err)
	}
	learnings = LoadLearnings(root)
	if len(learnings.FeedbackHistory) != 2 {
		t.Fatalf("feedback history lost: %+v", learnings.FeedbackHistory)
	}
}

func TestSyncMOCsAutoCreatesMissingMOC(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "notes", "pessoas")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "jane-doe.md"), []byte("# Jane Doe\n"), 0600); err != nil {
		t.Fatal(err)
	}

	missing, err := SyncMOCs(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 1 || missing[0] != "notes/pessoas/moc-pessoas.md" {
		t.Fatalf("expected 1 missing MOC 'notes/pessoas/moc-pessoas.md', got %v", missing)
	}

	if _, err := SyncMOCs(root, true); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "moc-pessoas.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "jane-doe") {
		t.Fatalf("expected created MOC to contain jane-doe: %s", data)
	}
}
