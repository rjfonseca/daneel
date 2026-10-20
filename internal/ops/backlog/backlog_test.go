//nolint:revive // package comment not required
package backlog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestActiveBacklog(t *testing.T) {
	tempDir := t.TempDir()

	carreiraDir := filepath.Join(tempDir, "notes", "carreira")
	_ = os.MkdirAll(carreiraDir, 0750)

	sumarioFile := filepath.Join(carreiraDir, "sumario-semanal-atividades-impactos-2026.md")
	content := `# Sumário Semanal

## 📌 Foco Atual & Pendências (Backlog Ativo)

### 🔴 P0 - Urgente
- [ ] **Rollout do Gateway API:** Finalizar virada de chave para 100% da base.
- [x] **Fix de Ingress:** Concluído com sucesso.

### 🟡 P1 - Estratégico
- [ ] **Pesquisa de Agentes:** Benchmark de ferramentas de mercado.

## 📅 Detalhamento Semanal
Histórico antigo...
`
	_ = os.WriteFile(sumarioFile, []byte(content), 0600)

	rep, err := GetActiveBacklog(tempDir)
	if err != nil {
		t.Fatalf("GetActiveBacklog failed: %v", err)
	}

	if len(rep.Items) != 3 {
		t.Fatalf("expected 3 backlog items, got %d", len(rep.Items))
	}

	if rep.Items[0].Priority != "P0" || rep.Items[0].Completed != false {
		t.Errorf("unexpected item 0: %+v", rep.Items[0])
	}
	if rep.Items[1].Priority != "P0" || rep.Items[1].Completed != true {
		t.Errorf("unexpected item 1: %+v", rep.Items[1])
	}
	if rep.Items[2].Priority != "P1" || rep.Items[2].Completed != false {
		t.Errorf("unexpected item 2: %+v", rep.Items[2])
	}
}
