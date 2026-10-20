//nolint:revive // package comment not required
package dream

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecordFeedback_HistoricLogRegexVariants(t *testing.T) {
	tests := []struct {
		name        string
		heading     string
		fileRelPath string
	}{
		{
			name:        "Section 4 legacy heading",
			heading:     "## 📝 4. Log Histórico de Feedbacks\n\n",
			fileRelPath: filepath.Join("meta", "guias", "aprendizados-agente.md"),
		},
		{
			name:        "Section 5 expanded heading",
			heading:     "## 📝 5. Log Histórico de Feedbacks & Auto-Evoluções\n\n",
			fileRelPath: filepath.Join("meta", "guias", "aprendizados-agente.md"),
		},
		{
			name:        "Section 5 in agente-evolucao.md fallback",
			heading:     "## 📝 5. Log Histórico de Feedbacks & Auto-Evoluções\n\n",
			fileRelPath: filepath.Join("meta", "guias", "agente-evolucao.md"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			_ = os.MkdirAll(filepath.Join(tempDir, ".daneel", "cache"), 0750)
			docPath := filepath.Join(tempDir, tt.fileRelPath)
			_ = os.MkdirAll(filepath.Dir(docPath), 0750)

			initialContent := "# Guia do Agente\n\nAlgum conteúdo anterior.\n\n" + tt.heading + "- **2026-09-01:** [MISSED_OPPORTUNITY] Antigo feedback.\n"
			if err := os.WriteFile(docPath, []byte(initialContent), 0600); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			err := RecordFeedback(tempDir, "Payment Gateway", "missed_opportunity", "search_vault", "Agent missed opportunity")
			if err != nil {
				t.Fatalf("RecordFeedback returned error: %v", err)
			}

			updatedData, err := os.ReadFile(docPath)
			if err != nil {
				t.Fatalf("failed to read updated doc: %v", err)
			}
			content := string(updatedData)

			if !strings.Contains(content, "[MISSED_OPPORTUNITY] Payment Gateway ➔ search_vault") {
				t.Errorf("expected updated doc to contain newly recorded feedback, got:\n%s", content)
			}
		})
	}
}
