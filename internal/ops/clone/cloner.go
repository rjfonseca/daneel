//nolint:revive // package comment not required
package clone

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rjfonseca/daneel/internal/dream"
	"github.com/rjfonseca/daneel/internal/search"
)

// CloneOptions configures the behavior of the clone operation.
type CloneOptions struct { //nolint:revive
	WorkRoot   string
	VaultRoot  string
	DBPath     string
	ForceFetch bool
	RunDream   bool
	SkipNote   bool
}

// CloneResult contains the outcome of a repository clone and vault integration.
type CloneResult struct { //nolint:revive
	Target         GitTarget `json:"target"`
	LocalPath      string    `json:"local_path"`
	AlreadyExisted bool      `json:"already_existed"`
	Branch         string    `json:"branch"`
	Stack          string    `json:"stack"`
	Description    string    `json:"description"`
	VaultNotePath  string    `json:"vault_note_path,omitempty"`
	MOCUpdated     bool      `json:"moc_updated"`
	Indexed        bool      `json:"indexed"`
	DreamExecuted  bool      `json:"dream_executed"`
}

// Cloner orchestrates git cloning, static code inspection, and Second Brain integration.
type Cloner struct{}

// NewCloner returns a new Cloner instance.
func NewCloner() *Cloner {
	return &Cloner{}
}

// Clone executes the complete cognitive clone and vault cataloging pipeline.
func (c *Cloner) Clone(ctx context.Context, rawInput string, opts CloneOptions) (*CloneResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	target, err := ParseGitTarget(rawInput)
	if err != nil {
		return nil, err
	}

	workRoot := opts.WorkRoot
	if workRoot == "" {
		home, _ := os.UserHomeDir()
		workRoot = filepath.Join(home, "Work")
	}

	localPath := filepath.Join(workRoot, target.RelPath)
	alreadyExisted := false

	// 1. Check if already exists
	if stat, err := os.Stat(filepath.Join(localPath, ".git")); err == nil && stat.IsDir() {
		alreadyExisted = true
		slog.Debug("repository already exists in local work directory", "path", localPath)
		if opts.ForceFetch {
			slog.Debug("fetching updates for existing repository", "path", localPath)
			cmd := exec.CommandContext(ctx, "git", "-C", localPath, "fetch", "--quiet") //nolint:gosec // intentional
			_ = cmd.Run()
		}
	} else {
		// Ensure parent directories exist
		if err := os.MkdirAll(filepath.Dir(localPath), 0750); err != nil {
			return nil, fmt.Errorf("failed to create parent directory: %w", err)
		}

		slog.Info("cloning repository", "clone_url", target.CloneURL, "dest", localPath)
		cmd := exec.CommandContext(ctx, "git", "clone", target.CloneURL, localPath) //nolint:gosec // intentional
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("failed to execute git clone '%s': %w", target.CloneURL, err)
		}
	}

	// 2. Static Code & Metadata Inspection
	stack, desc, branch, commands, tags := c.inspectRepo(localPath)

	result := &CloneResult{
		Target:         *target,
		LocalPath:      localPath,
		AlreadyExisted: alreadyExisted,
		Branch:         branch,
		Stack:          stack,
		Description:    desc,
		Indexed:        false,
	}

	// 3. Generate or Link Second Brain Note
	if !opts.SkipNote && opts.VaultRoot != "" {
		notePath, err := c.generateSystemNote(target, localPath, opts.VaultRoot, stack, desc, commands, tags)
		if err != nil {
			slog.Warn("failed to generate system note in vault", "error", err)
		} else {
			result.VaultNotePath = notePath
			if updated, err := c.updateSystemsMOC(opts.VaultRoot, notePath, target.Repo, desc); err == nil && updated {
				result.MOCUpdated = true
			}
		}
	}

	// 4. Quick Index in SQLite FTS5 (< 5ms)
	if opts.DBPath != "" && opts.VaultRoot != "" && result.VaultNotePath != "" {
		if sEngine, err := search.Open(opts.DBPath, opts.VaultRoot); err == nil {
			if err := sEngine.IndexNote(ctx, result.VaultNotePath); err == nil {
				result.Indexed = true
			}
			sEngine.Close() //nolint:errcheck,gosec // fully read or safe to ignore
		}
	}

	// 5. Optional Dream Cycle Execution
	if opts.RunDream && opts.VaultRoot != "" {
		dreamEng := dream.NewDreamEngine(opts.VaultRoot)
		if _, err := dreamEng.RunFullCycle(time.Now().Format("2006-01-02"), true, false); err == nil {
			result.DreamExecuted = true
		}
	}

	return result, nil
}

func (c *Cloner) inspectRepo(repoPath string) (stack string, desc string, branch string, commands []string, techTags []string) {
	// Extract branch
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "--abbrev-ref", "HEAD") //nolint:gosec // intentional
	if out, err := cmd.Output(); err == nil {
		branch = strings.TrimSpace(string(out))
	}
	if branch == "" {
		branch = "main"
	}

	var detected []string

	if fileExists(filepath.Join(repoPath, "go.mod")) {
		detected = append(detected, "Go")
		techTags = append(techTags, "go")
	}
	if fileExists(filepath.Join(repoPath, "Cargo.toml")) {
		detected = append(detected, "Rust")
		techTags = append(techTags, "rust")
	}
	if fileExists(filepath.Join(repoPath, "package.json")) {
		detected = append(detected, "TypeScript/JavaScript")
		techTags = append(techTags, "typescript", "javascript")
	}
	if fileExists(filepath.Join(repoPath, "composer.json")) {
		detected = append(detected, "PHP")
		techTags = append(techTags, "php")
	}
	if fileExists(filepath.Join(repoPath, "pom.xml")) || fileExists(filepath.Join(repoPath, "build.gradle")) {
		detected = append(detected, "Java")
		techTags = append(techTags, "java")
	}
	if fileExists(filepath.Join(repoPath, "pyproject.toml")) || fileExists(filepath.Join(repoPath, "requirements.txt")) {
		detected = append(detected, "Python")
		techTags = append(techTags, "python")
	}
	if fileExists(filepath.Join(repoPath, "Dockerfile")) || fileExists(filepath.Join(repoPath, "compose.yaml")) || fileExists(filepath.Join(repoPath, "docker-compose.yml")) {
		detected = append(detected, "Docker")
		techTags = append(techTags, "docker")
	}
	if fileExists(filepath.Join(repoPath, "Chart.yaml")) {
		detected = append(detected, "Kubernetes/Helm")
		techTags = append(techTags, "kubernetes", "helm")
	}

	if len(detected) == 0 {
		stack = "Software / Multi-Stack"
	} else {
		stack = strings.Join(detected, " + ")
	}

	// Detect commands
	if fileExists(filepath.Join(repoPath, "Taskfile.yml")) || fileExists(filepath.Join(repoPath, "Taskfile.yaml")) {
		commands = append(commands, "`task test` - Runs the test suite", "`task build` - Compiles the project binaries")
	} else if fileExists(filepath.Join(repoPath, "Makefile")) {
		commands = append(commands, "`make test` - Runs the test suite", "`make build` - Compiles the project")
	}
	if fileExists(filepath.Join(repoPath, "package.json")) {
		commands = append(commands, "`npm test` / `pnpm test` - Runs tests", "`npm run dev` - Starts development server")
	}
	if fileExists(filepath.Join(repoPath, "compose.yaml")) || fileExists(filepath.Join(repoPath, "docker-compose.yml")) {
		commands = append(commands, "`docker compose up -d` - Starts local dependencies in background")
	}

	// Extract description from README
	readmeCandidates := []string{"README.md", "readme.md", "README", "readme"}
	for _, cand := range readmeCandidates {
		p := filepath.Join(repoPath, cand)
		if data, err := os.ReadFile(p); err == nil { //nolint:gosec // intentional
			desc = extractSummaryFromMarkdown(string(data))
			break
		}
	}
	if desc == "" {
		desc = fmt.Sprintf("Source code repository for %s (%s).", filepath.Base(repoPath), stack)
	}

	return stack, desc, branch, commands, techTags
}

func extractSummaryFromMarkdown(content string) string {
	lines := strings.Split(content, "\n")
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "[![") || strings.HasPrefix(trimmed, "<") {
			continue
		}
		// Clean bold/italics
		cleaned := strings.ReplaceAll(trimmed, "**", "")
		cleaned = strings.ReplaceAll(cleaned, "*", "")
		cleaned = strings.ReplaceAll(cleaned, "`", "")
		if len(cleaned) > 20 {
			if len(cleaned) > 200 {
				return cleaned[:197] + "..."
			}
			return cleaned
		}
	}
	return ""
}

func (c *Cloner) generateSystemNote(target *GitTarget, localPath string, vaultRoot string, stack string, desc string, commands []string, tags []string) (string, error) {
	sistemasDir := filepath.Join(vaultRoot, "notes", "sistemas")
	if err := os.MkdirAll(sistemasDir, 0750); err != nil {
		return "", err
	}

	slug := strings.ToLower(target.Repo)
	noteFile := filepath.Join(sistemasDir, slug+".md")
	relPath := filepath.Join("notes", "sistemas", slug+".md")

	// If note already exists, we preserve user edits and return the rel path
	if fileExists(noteFile) {
		return relPath, nil
	}

	// Generate smart cross-links to technologies in vault
	var techLinks []string
	if strings.Contains(strings.ToLower(stack), "go") {
		techLinks = append(techLinks, "[Golang](../go/index.md)")
	}
	if strings.Contains(strings.ToLower(stack), "rust") {
		techLinks = append(techLinks, "[Rust](../linguagens/rust.md)")
	}
	if strings.Contains(strings.ToLower(stack), "docker") {
		techLinks = append(techLinks, "[Docker](../infra/docker.md)")
	}
	if strings.Contains(strings.ToLower(stack), "kubernetes") || strings.Contains(strings.ToLower(stack), "helm") {
		techLinks = append(techLinks, "[Kubernetes](../infra/kubernetes.md)")
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString("type: sistema\n")
	fmt.Fprintf(&sb, "description: %s\n", desc)
	sb.WriteString("tags:\n")
	sb.WriteString("  - sistema\n")
	for _, t := range tags {
		fmt.Fprintf(&sb, "  - %s\n", t)
	}
	fmt.Fprintf(&sb, "local_path: %s\n", localPath)
	sb.WriteString("---\n\n")

	fmt.Fprintf(&sb, "# %s (`%s`)\n\n", target.Repo, slug)
	fmt.Fprintf(&sb, "> **Local Path:** `%s`  \n", localPath)
	sb.WriteString("> **Systems Map:** [MOC Sistemas](moc-sistemas.md)\n\n")

	sb.WriteString("## 📌 Overview\n\n")
	sb.WriteString(desc + "\n\n")

	sb.WriteString("## 🛠️ Technical Stack & Architecture\n\n")
	fmt.Fprintf(&sb, "- **Primary Stack:** %s\n", stack)
	if len(techLinks) > 0 {
		fmt.Fprintf(&sb, "- **Related Technologies:** %s\n", strings.Join(techLinks, ", "))
	}
	sb.WriteString("\n")

	if len(commands) > 0 {
		sb.WriteString("## ⚡ Useful Commands & Local Execution\n\n")
		for _, cmd := range commands {
			fmt.Fprintf(&sb, "- %s\n", cmd)
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## 🔗 References & Navigation\n\n")
	sb.WriteString("- [MOC Sistemas](moc-sistemas.md)\n")
	sb.WriteString("- [Master MOC](../moc-indice-geral.md)\n")

	if err := os.WriteFile(noteFile, []byte(sb.String()), 0600); err != nil {
		return "", err
	}

	slog.Info("created system note in vault", "file", noteFile)
	return relPath, nil
}

func (c *Cloner) updateSystemsMOC(vaultRoot string, noteRelPath string, repoName string, desc string) (bool, error) {
	sistemasDir := filepath.Join(vaultRoot, "notes", "sistemas")
	mocPath := filepath.Join(sistemasDir, "moc-sistemas.md")

	if !fileExists(mocPath) {
		// Check if there is another moc file in this directory
		entries, err := os.ReadDir(sistemasDir)
		if err == nil {
			for _, e := range entries {
				if strings.HasPrefix(e.Name(), "moc-") && strings.HasSuffix(e.Name(), ".md") {
					mocPath = filepath.Join(sistemasDir, e.Name())
					break
				}
			}
		}
	}

	if !fileExists(mocPath) {
		initialContent := fmt.Sprintf("# 📦 MOC: Systems & Services\n\n> Content Map of systems, services, and local repositories.\n\n## 📋 Service Catalog\n\n- [%s](%s) - %s\n",
			repoName, filepath.Base(noteRelPath), desc)
		if err := os.WriteFile(mocPath, []byte(initialContent), 0600); err != nil {
			return false, err
		}
		slog.Info("created MOC sistemas", "moc", mocPath)
		return true, nil
	}

	data, err := os.ReadFile(mocPath) //nolint:gosec // intentional
	if err != nil {
		return false, err
	}

	content := string(data)
	noteBase := filepath.Base(noteRelPath)

	// Check if already present in MOC
	if strings.Contains(content, noteBase) {
		return false, nil
	}

	entry := fmt.Sprintf("- [%s](%s) - %s\n", repoName, noteBase, desc)

	// Append under the first markdown heading or at bottom
	var newContent string
	if strings.Contains(content, "## ") {
		lines := strings.Split(content, "\n")
		var updatedLines []string
		inserted := false
		for _, l := range lines {
			updatedLines = append(updatedLines, l)
			if !inserted && strings.HasPrefix(l, "## ") {
				updatedLines = append(updatedLines, "", entry)
				inserted = true
			}
		}
		newContent = strings.Join(updatedLines, "\n")
	} else {
		newContent = content + "\n" + entry
	}

	if err := os.WriteFile(mocPath, []byte(newContent), 0600); err != nil { //nolint:gosec // intentional
		return false, err
	}

	slog.Info("updated MOC sistemas", "moc", mocPath)
	return true, nil
}

func fileExists(p string) bool {
	stat, err := os.Stat(p)
	return err == nil && !stat.IsDir()
}
