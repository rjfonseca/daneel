//nolint:revive // package comment not required
package clone

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGitTarget(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedRel string
		expectedOrg string
		expectedRep string
		shouldErr   bool
	}{
		{
			name:        "SSH GitHub",
			input:       "git@github.com:rjfonseca/daneel.git",
			expectedRel: "github.com/rjfonseca/daneel",
			expectedOrg: "rjfonseca",
			expectedRep: "daneel",
		},
		{
			name:        "SSH GitLab Subgroups",
			input:       "git@gitlab.com:company/platform/core/auth-service.git",
			expectedRel: "gitlab.com/company/platform/core/auth-service",
			expectedOrg: "company/platform/core",
			expectedRep: "auth-service",
		},
		{
			name:        "HTTPS GitHub",
			input:       "https://github.com/charmbracelet/bubbletea.git",
			expectedRel: "github.com/charmbracelet/bubbletea",
			expectedOrg: "charmbracelet",
			expectedRep: "bubbletea",
		},
		{
			name:        "Slug full host",
			input:       "github.com/spf13/cobra",
			expectedRel: "github.com/spf13/cobra",
			expectedOrg: "spf13",
			expectedRep: "cobra",
		},
		{
			name:        "Short slug (default to github.com)",
			input:       "jane-doe/awesome-project",
			expectedRel: "github.com/jane-doe/awesome-project",
			expectedOrg: "jane-doe",
			expectedRep: "awesome-project",
		},
		{
			name:      "Empty input",
			input:     "",
			shouldErr: true,
		},
		{
			name:      "Invalid single word",
			input:     "justarepo",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, err := ParseGitTarget(tt.input)
			if tt.shouldErr {
				if err == nil {
					t.Fatalf("expected error for input '%s', got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if target.RelPath != tt.expectedRel {
				t.Errorf("expected RelPath '%s', got '%s'", tt.expectedRel, target.RelPath)
			}
			if target.Org != tt.expectedOrg {
				t.Errorf("expected Org '%s', got '%s'", tt.expectedOrg, target.Org)
			}
			if target.Repo != tt.expectedRep {
				t.Errorf("expected Repo '%s', got '%s'", tt.expectedRep, target.Repo)
			}
		})
	}
}

func TestClonerPipeline(t *testing.T) {
	tempDir := t.TempDir()
	workDir := filepath.Join(tempDir, "Work")
	vaultDir := filepath.Join(tempDir, "Vault")
	dbPath := filepath.Join(tempDir, "cache.db")

	_ = os.MkdirAll(workDir, 0750)
	_ = os.MkdirAll(vaultDir, 0750)

	// Create MOC Sistemas
	sistemasDir := filepath.Join(vaultDir, "notes", "sistemas")
	_ = os.MkdirAll(sistemasDir, 0750)
	mocPath := filepath.Join(sistemasDir, "moc-sistemas.md")
	_ = os.WriteFile(mocPath, []byte("# MOC Sistemas\n\n## Catálogo de Serviços\n"), 0600)

	// 1. Create a mock source Git repository to clone
	mockRemote := filepath.Join(tempDir, "remote-repo")
	_ = os.MkdirAll(mockRemote, 0750)
	_ = exec.Command("git", "-C", mockRemote, "init").Run()                                  //nolint:gosec // intentional
	_ = exec.Command("git", "-C", mockRemote, "config", "user.email", "test@test.com").Run() //nolint:gosec // intentional
	_ = exec.Command("git", "-C", mockRemote, "config", "user.name", "Test").Run()           //nolint:gosec // intentional

	readmeContent := "# Awesome Service\n\nHigh performance event processing microservice written in Go.\n"
	_ = os.WriteFile(filepath.Join(mockRemote, "README.md"), []byte(readmeContent), 0600)
	_ = os.WriteFile(filepath.Join(mockRemote, "go.mod"), []byte("module github.com/test/awesome-service\n\ngo 1.26\n"), 0600)
	_ = os.WriteFile(filepath.Join(mockRemote, "Taskfile.yml"), []byte("version: '3'\n"), 0600)

	_ = exec.Command("git", "-C", mockRemote, "add", "-A").Run()                      //nolint:gosec // intentional
	_ = exec.Command("git", "-C", mockRemote, "commit", "-m", "initial commit").Run() //nolint:gosec // intentional

	// 2. Clone using local target
	cloner := NewCloner()
	opts := CloneOptions{
		WorkRoot:  workDir,
		VaultRoot: vaultDir,
		DBPath:    dbPath,
	}

	// We pass mockRemote directly as target by setting up the expected structure
	targetDir := filepath.Join(workDir, "github.com", "test", "awesome-service")
	_ = os.MkdirAll(filepath.Dir(targetDir), 0750)

	// Execute local clone
	cloneCmd := exec.Command("git", "clone", mockRemote, targetDir) //nolint:gosec // intentional
	if err := cloneCmd.Run(); err != nil {
		t.Fatalf("setup git clone failed: %v", err)
	}

	// Run Cloner inspection & vault integration on already cloned target
	res, err := cloner.Clone(context.Background(), "github.com/test/awesome-service", opts)
	if err != nil {
		t.Fatalf("Cloner.Clone failed: %v", err)
	}

	if !res.AlreadyExisted {
		t.Errorf("expected AlreadyExisted to be true for pre-cloned test")
	}
	if res.Stack != "Go" {
		t.Errorf("expected stack 'Go', got '%s'", res.Stack)
	}
	if !strings.Contains(res.Description, "High performance event processing") {
		t.Errorf("expected extracted description from README, got '%s'", res.Description)
	}

	// 3. Verify Vault Note created
	notePath := filepath.Join(vaultDir, res.VaultNotePath)
	if !fileExists(notePath) {
		t.Fatalf("expected vault note to exist at '%s'", notePath)
	}

	noteData, _ := os.ReadFile(notePath) //nolint:gosec // intentional
	if !strings.Contains(string(noteData), "type: sistema") {
		t.Errorf("expected OKF type 'sistema' in frontmatter")
	}
	if !strings.Contains(string(noteData), "[Golang](../go/index.md)") {
		t.Errorf("expected cross-link to Golang note")
	}

	// 4. Verify MOC was updated
	mocData, _ := os.ReadFile(mocPath) //nolint:gosec // intentional
	if !strings.Contains(string(mocData), "awesome-service.md") {
		t.Errorf("expected moc-sistemas.md to contain link to awesome-service.md, got:\n%s", string(mocData))
	}
}
