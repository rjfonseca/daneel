package dream

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func snapshotVault(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == ".daneel" {
			return filepath.SkipDir
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

func TestSyncMOCsRequiresExactLinkTarget(t *testing.T) {
	for _, existing := range []string{"- [Foobar](foobar.md)", "A plain mention of foo", "- [[foo]]", "- [[foo.md#Overview]]", "- [[foo#Overview|Label]]", "- [Foo](foo)", "- [Foo](foo.md)"} {
		t.Run(existing, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "notes", "topics")
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "foo.md"), []byte("# Foo\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "moc-topics.md"), []byte(existing), 0600); err != nil {
				t.Fatal(err)
			}
			missing, err := SyncMOCs(root, false)
			if err != nil {
				t.Fatal(err)
			}
			wantMissing := strings.Contains(existing, "Foobar") || strings.Contains(existing, "plain mention")
			if (len(missing) == 1) != wantMissing {
				t.Fatalf("missing = %v, want missing %v", missing, wantMissing)
			}
		})
	}
}

func TestSyncMOCsPreservesUnicodeCategoryTitle(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "notes", "árvore")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "foo.md"), []byte("# Foo\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncMOCs(root, true); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "moc-árvore.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !utf8.Valid(data) || !strings.Contains(string(data), "MOC: Árvore") {
		t.Fatalf("invalid category title: %q", data)
	}
}

func TestArchiveFeedbackHistoryKeepsHotCacheSmallAndClearsAlert(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	cacheDir := filepath.Join(root, ".daneel", "cache")
	if err := os.MkdirAll(cacheDir, 0750); err != nil {
		t.Fatal(err)
	}

	learnings := DefaultLearnings()
	for i := 0; i < 7; i++ {
		month := "2026-10"
		if i >= 4 {
			month = "2026-11"
		}
		learnings.FeedbackHistory = append(learnings.FeedbackHistory, FeedbackEntry{
			Date:      fmt.Sprintf("%s-%02d", month, i+1),
			Timestamp: fmt.Sprintf("%s-%02dT10:00:00Z", month, i+1),
			Topic:     fmt.Sprintf("Calibration topic %d", i+1),
			Type:      "correction",
			Action:    "keep",
			Note:      "Sample calibration note",
		})
	}
	if err := SaveLearnings(root, learnings); err != nil {
		t.Fatal(err)
	}

	// Before archiving, > 5 entries triggers skill_refactoring insight.
	beforeRep := RunMetacognitivePass(root, &FullDreamReport{}, nil)
	foundBefore := false
	for _, ins := range beforeRep.Insights {
		if ins.Category == "skill_refactoring" && strings.Contains(ins.Observation, "calibrations") {
			foundBefore = true
		}
	}
	if !foundBefore {
		t.Fatalf("expected skill_refactoring insight before archiving, got %+v", beforeRep.Insights)
	}

	// Dry-run leaves hot cache untouched.
	dryRes, err := ArchiveFeedbackHistory(root, true)
	if err != nil {
		t.Fatal(err)
	}
	if dryRes.ArchivedCount != 7 || dryRes.RemainingPending != 7 {
		t.Fatalf("unexpected dry-run result: %+v", dryRes)
	}
	if len(LoadLearnings(root).FeedbackHistory) != 7 {
		t.Fatalf("dry-run modified hot cache")
	}

	// Real archive moves entries to monthly files and empties hot cache.
	res, err := ArchiveFeedbackHistory(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.ArchivedCount != 7 || res.RemainingPending != 0 || len(res.ArchiveFiles) != 2 {
		t.Fatalf("unexpected archive result: %+v", res)
	}
	if len(LoadLearnings(root).FeedbackHistory) != 0 {
		t.Fatalf("expected hot cache feedback_history to be empty after archiving")
	}

	// Idempotent second run archives 0.
	res2, err := ArchiveFeedbackHistory(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if res2.ArchivedCount != 0 {
		t.Fatalf("expected 0 archived on second run, got %+v", res2)
	}

	// Archived entries remain loadable and metacognition alert is cleared.
	archived := LoadArchivedFeedbackEntries(root)
	if len(archived) != 7 {
		t.Fatalf("expected 7 archived entries, got %d", len(archived))
	}
	afterRep := RunMetacognitivePass(root, &FullDreamReport{}, nil)
	for _, ins := range afterRep.Insights {
		if ins.Category == "skill_refactoring" && strings.Contains(ins.Observation, "calibrations") {
			t.Fatalf("unexpected skill_refactoring insight after archiving: %+v", ins)
		}
	}
}
