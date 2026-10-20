//nolint:revive // package comment not required
package meetings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMeetingsProcessor(t *testing.T) {
	tempDir := t.TempDir()

	inboxDir := filepath.Join(tempDir, "inbox")
	sourcesDir := filepath.Join(tempDir, "sources", "reunioes")
	peopleDir := filepath.Join(tempDir, "meta", "pessoas")

	_ = os.MkdirAll(inboxDir, 0750)
	_ = os.MkdirAll(sourcesDir, 0750)
	_ = os.MkdirAll(peopleDir, 0750)

	// Create a person profile
	personFile := filepath.Join(peopleDir, "john-doe.md")
	personContent := `---
type: person
---
# 👤 John Doe

**Cargo / Papel:** Staff Engineer
**Squad / Tribo:** SRE

## 🤝 Histórico de Interações
`
	_ = os.WriteFile(personFile, []byte(personContent), 0600)

	// Create test meeting note in inbox
	meetFile := filepath.Join(inboxDir, "Alinhamento de Arquitetura - 2026-08-15 - Anotações do Gemini.md")
	meetContent := `# Alinhamento de Arquitetura

convidados John Doe, Jane Smith

### **Resumo**
Discutimos a migração do Ingress para Gateway API no GKE e o rollout do cluster de staging.

### **Próximas etapas**
- Finalizar RFC de Gateway API
- Testar deploy no staging
`
	_ = os.WriteFile(meetFile, []byte(meetContent), 0600)

	proc := NewProcessor(tempDir)
	rep, err := proc.ProcessInbox(true)
	if err != nil {
		t.Fatalf("ProcessInbox failed: %v", err)
	}

	if rep.TotalProcessed != 1 {
		t.Errorf("expected 1 meeting processed, got %d", rep.TotalProcessed)
	}

	m := rep.Meetings[0]
	if m.Title != "Alinhamento de Arquitetura" {
		t.Errorf("expected title 'Alinhamento de Arquitetura', got '%s'", m.Title)
	}
	if m.Date != "2026-08-15" {
		t.Errorf("expected date '2026-08-15', got '%s'", m.Date)
	}
	if len(m.ActionItems) != 2 {
		t.Errorf("expected 2 action items, got %d", len(m.ActionItems))
	}

	// Verify person interaction was synced
	updatedPersonData, _ := os.ReadFile(personFile) //nolint:gosec // intentional
	if !strings.Contains(string(updatedPersonData), "2026-08-15") {
		t.Errorf("expected person profile to contain interaction date 2026-08-15")
	}

	// Verify original was cleaned from inbox
	if _, err := os.Stat(meetFile); !os.IsNotExist(err) {
		t.Errorf("expected original inbox file to be removed")
	}
}
