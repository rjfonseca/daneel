//nolint:revive // package comment not required
package briefing

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPreflightBriefing(t *testing.T) {
	tempDir := t.TempDir()

	peopleDir := filepath.Join(tempDir, "meta", "pessoas")
	_ = os.MkdirAll(peopleDir, 0750)

	personFile := filepath.Join(peopleDir, "jane-doe.md")
	content := `---
type: person
role: Tech Lead
squad: Squad Core
---
# Jane Doe

**Cargo / Papel:** Tech Lead
**Squad / Tribo:** Squad Core
**Realção:** Par

## 🤝 Histórico de Interações
- **2026-08-10:** Alinhamento de arquitetura de microsserviços.

## 🎯 Tópicos de Interesse
- [ ] Implementar tracing com OpenTelemetry.
`
	_ = os.WriteFile(personFile, []byte(content), 0600)

	svc := NewPreflightService(tempDir)
	profile, err := svc.GenerateBriefing("jane-doe")
	if err != nil {
		t.Fatalf("GenerateBriefing failed: %v", err)
	}

	if profile.Name != "Jane Doe" {
		t.Errorf("expected Name 'Jane Doe', got '%s'", profile.Name)
	}
	if profile.Cargo != "Tech Lead" {
		t.Errorf("expected Cargo 'Tech Lead', got '%s'", profile.Cargo)
	}
	if len(profile.RecentInteractions) != 1 {
		t.Errorf("expected 1 interaction, got %d", len(profile.RecentInteractions))
	}
}

func TestPreflightBriefing_CompoundNameAndAliasResolution(t *testing.T) {
	tempDir := t.TempDir()

	peopleDir := filepath.Join(tempDir, "notes", "pessoas")
	_ = os.MkdirAll(peopleDir, 0750)

	lovelaceFile := filepath.Join(peopleDir, "augusta-ada-byron-king-lovelace.md")
	content := `---
type: person
role: Engineering Manager
squad: Platform Core / Architecture
aliases:
  - "Augusta Ada Byron King Lovelace"
  - "Augusta Ada Byron King Lovelace (lovelace)"
  - "lovelace"
---
# 👤 Augusta Ada Byron King Lovelace

**Cargo / Papel:** Engineering Manager
**Squad / Tribo:** Platform Core
**Realção:** Liderança

## 🤝 Histórico de Interações
- **2026-08-18:** Alinhamento geral em reunião de protótipo.
`
	_ = os.WriteFile(lovelaceFile, []byte(content), 0600)

	svc := NewPreflightService(tempDir)

	// Test A: Compound name "Ada Lovelace"
	profileA, err := svc.GenerateBriefing("Ada Lovelace")
	if err != nil {
		t.Fatalf("expected 'Ada Lovelace' to resolve, got error: %v", err)
	}
	if profileA.Name != "Augusta Ada Byron King Lovelace" {
		t.Errorf("expected 'Augusta Ada Byron King Lovelace', got '%s'", profileA.Name)
	}

	// Test B: Nickname alias "lovelace"
	profileB, err := svc.GenerateBriefing("lovelace")
	if err != nil {
		t.Fatalf("expected 'lovelace' to resolve, got error: %v", err)
	}
	if profileB.Name != "Augusta Ada Byron King Lovelace" {
		t.Errorf("expected 'Augusta Ada Byron King Lovelace', got '%s'", profileB.Name)
	}
}
