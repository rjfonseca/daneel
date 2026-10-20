//nolint:revive // package comment not required
package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RepoInfo represents metadata for a single Git repository.
type RepoInfo struct {
	Name              string   `json:"name"`
	RelPath           string   `json:"rel_path"`
	Path              string   `json:"path"`
	Category          string   `json:"category"`
	ComponentType     string   `json:"component_type"`
	Stack             string   `json:"stack"`
	Description       string   `json:"description"`
	Commands          string   `json:"commands"`
	Branch            string   `json:"branch"`
	LastCommitDate    string   `json:"last_commit_date"`
	LastCommitMsg     string   `json:"last_commit_msg"`
	DaysSinceActivity int      `json:"days_since_activity"`
	IsRecent          bool     `json:"is_recent"`
	Dirty             bool     `json:"dirty"`
	Synced            bool     `json:"synced,omitempty"`
	PrunedBranches    []string `json:"pruned_branches,omitempty"`
	ModifiedCount     int      `json:"modified_count"`
	UntrackedCount    int      `json:"untracked_count"`
	FilesPreview      []string `json:"files_preview"`
	VaultLink         string   `json:"vault_link,omitempty"`
}

// WorkspaceReport summarizes the workspace scanning results.
type WorkspaceReport struct { //nolint:revive
	TotalRepos          int                   `json:"total_repos"`
	CleanCount          int                   `json:"clean_count"`
	DirtyCount          int                   `json:"dirty_count"`
	SyncedCount         int                   `json:"synced_count"`
	PrunedBranchesCount int                   `json:"pruned_branches_count,omitempty"`
	Categories          map[string][]RepoInfo `json:"categories"`
	DirtyRepos          []RepoInfo            `json:"dirty_repos"`
	ScannedAt           string                `json:"scanned_at"`
	MOCFilePath         string                `json:"moc_file_path,omitempty"`
}

var ignoredDirs = map[string]bool{
	"pkg": true, "node_modules": true, ".cache": true, "vendor": true,
	".terraform": true, "dist": true, "build": true, ".trash": true,
	".system_generated": true, "__pycache__": true, ".git": true,
}

// Scanner handles concurrent scanning of the ~/Work directory.
type Scanner struct {
	workRoot            string
	vaultRoot           string
	inactivityThreshold time.Duration
}

// NewScanner creates a new workspace scanner.
func NewScanner(workRoot string, vaultRoot string) *Scanner {
	if workRoot == "" {
		home, _ := os.UserHomeDir()
		workRoot = filepath.Join(home, "Work")
	}
	return &Scanner{
		workRoot:            workRoot,
		vaultRoot:           vaultRoot,
		inactivityThreshold: 48 * time.Hour,
	}
}

// SetInactivityThreshold configures the duration before resetting a clean feature branch.
func (s *Scanner) SetInactivityThreshold(d time.Duration) {
	if d > 0 {
		s.inactivityThreshold = d
	}
}

// Scan processes all git repositories in the workspace concurrently.
func (s *Scanner) Scan(syncClean bool) (*WorkspaceReport, error) {
	report := &WorkspaceReport{
		Categories:  make(map[string][]RepoInfo),
		DirtyRepos:  []RepoInfo{},
		ScannedAt:   time.Now().Format(time.RFC3339),
		MOCFilePath: filepath.Join(s.vaultRoot, "notes", "organizacao", "moc-workspace-work.md"),
	}
	if _, err := os.Stat(s.workRoot); os.IsNotExist(err) {
		return report, nil
	}

	repoDirs := s.discoverRepoDirs()
	vaultMap := s.buildVaultMapping()

	resultsChan := make(chan RepoInfo, len(repoDirs))
	var wg sync.WaitGroup

	numWorkers := 16
	if len(repoDirs) < numWorkers {
		numWorkers = len(repoDirs)
	}
	if numWorkers == 0 {
		numWorkers = 1
	}

	jobsChan := make(chan string, len(repoDirs))

	for w := 0; w < numWorkers; w++ {
		go func() {
			for repoPath := range jobsChan {
				info := s.inspectRepo(repoPath, vaultMap, syncClean)
				resultsChan <- info
				wg.Done()
			}
		}()
	}

	for _, d := range repoDirs {
		wg.Add(1)
		jobsChan <- d
	}
	close(jobsChan)

	wg.Wait()
	close(resultsChan)

	var allRepos []RepoInfo
	for info := range resultsChan {
		allRepos = append(allRepos, info)
	}

	sort.Slice(allRepos, func(i, j int) bool {
		return allRepos[i].RelPath < allRepos[j].RelPath
	})

	report.TotalRepos = len(allRepos)

	for _, r := range allRepos {
		report.Categories[r.Category] = append(report.Categories[r.Category], r)
		if r.Dirty {
			report.DirtyCount++
			report.DirtyRepos = append(report.DirtyRepos, r)
		} else {
			report.CleanCount++
		}
		if r.Synced {
			report.SyncedCount++
		}
		report.PrunedBranchesCount += len(r.PrunedBranches)
	}

	return report, nil
}

func (s *Scanner) discoverRepoDirs() []string {
	var repoDirs []string

	_ = filepath.Walk(s.workRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			base := info.Name()
			if ignoredDirs[base] {
				return filepath.SkipDir
			}

			// Check if this directory is a git repository
			gitDir := filepath.Join(path, ".git")
			if fi, err := os.Stat(gitDir); err == nil && (fi.IsDir() || !fi.IsDir()) {
				repoDirs = append(repoDirs, path)
				return filepath.SkipDir // Do not look for nested git repositories inside
			}
		}
		return nil
	})

	return repoDirs
}

func (s *Scanner) inspectRepo(repoPath string, vaultMap map[string]string, syncClean bool) RepoInfo {
	relPath, _ := filepath.Rel(s.workRoot, repoPath)
	relPath = filepath.ToSlash(relPath)
	name := filepath.Base(repoPath)

	category := "Outros"
	parts := strings.Split(relPath, "/")
	if len(parts) > 1 {
		category = parts[0]
		if category == "gitlab.com" && len(parts) > 2 {
			category = parts[1]
		} else if category == "github.com" && len(parts) > 2 {
			category = parts[1]
		}
	}

	branch := s.getGitBranch(repoPath)
	dirty, modCount, untrackedCount, filesPreview := s.getGitStatus(repoPath)
	lastDate, lastMsg, daysSince, lastCommitTime := s.getGitLastCommit(repoPath)

	threshold := s.inactivityThreshold
	if threshold <= 0 {
		threshold = 48 * time.Hour
	}

	var synced bool
	var prunedBranches []string

	// Sync clean repo if requested
	if syncClean && !dirty {
		if branch == "main" || branch == "master" {
			_ = exec.Command("git", "-C", repoPath, "pull", "--quiet").Run() //nolint:gosec // intentional
			synced = true
			prunedBranches = s.pruneMergedBranches(repoPath, branch, branch)
		} else if branch != "unknown" && branch != "" {
			// Check if the current branch has been inactive for longer than the threshold
			inactive := false
			if !lastCommitTime.IsZero() {
				inactive = time.Since(lastCommitTime) >= threshold
			}

			if inactive {
				// Try checkout main first, which works if local main or origin/main exists
				if err := exec.Command("git", "-C", repoPath, "checkout", "main").Run(); err == nil { //nolint:gosec // intentional
					_ = exec.Command("git", "-C", repoPath, "pull", "--quiet").Run() //nolint:gosec // intentional
					prunedBranches = s.pruneMergedBranches(repoPath, "main", "main")
					branch = "main"
					synced = true
				} else if err := exec.Command("git", "-C", repoPath, "checkout", "master").Run(); err == nil { //nolint:gosec // intentional
					_ = exec.Command("git", "-C", repoPath, "pull", "--quiet").Run() //nolint:gosec // intentional
					prunedBranches = s.pruneMergedBranches(repoPath, "master", "master")
					branch = "master"
					synced = true
				}
			}
		}
	}

	stack := s.detectStack(repoPath)
	compType := s.inferComponentType(name, relPath, stack)
	description := s.extractDescription(repoPath)
	commands := s.detectCommands(repoPath, stack)

	vaultLink := ""
	cleanName := strings.ToLower(name)
	if link, ok := vaultMap[cleanName]; ok {
		vaultLink = link
	}

	return RepoInfo{
		Name:              name,
		RelPath:           relPath,
		Path:              repoPath,
		Category:          category,
		ComponentType:     compType,
		Stack:             stack,
		Description:       description,
		Commands:          commands,
		Branch:            branch,
		LastCommitDate:    lastDate,
		LastCommitMsg:     lastMsg,
		DaysSinceActivity: daysSince,
		IsRecent:          daysSince <= 30,
		Dirty:             dirty,
		Synced:            synced,
		PrunedBranches:    prunedBranches,
		ModifiedCount:     modCount,
		UntrackedCount:    untrackedCount,
		FilesPreview:      filesPreview,
		VaultLink:         vaultLink,
	}
}

func (s *Scanner) getGitBranch(repoPath string) string {
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "--abbrev-ref", "HEAD") //nolint:gosec // intentional
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func (s *Scanner) getGitStatus(repoPath string) (bool, int, int, []string) {
	cmd := exec.Command("git", "-C", repoPath, "status", "--porcelain") //nolint:gosec // intentional
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return false, 0, 0, nil
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	modCount := 0
	untrackedCount := 0
	var preview []string

	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if len(trimmed) < 3 {
			continue
		}
		status := trimmed[:2]
		filePath := strings.TrimSpace(trimmed[2:])

		if strings.Contains(status, "?") {
			untrackedCount++
		} else {
			modCount++
		}

		if len(preview) < 3 {
			preview = append(preview, filePath)
		}
	}

	return (modCount > 0 || untrackedCount > 0), modCount, untrackedCount, preview
}

func (s *Scanner) getGitLastCommit(repoPath string) (string, string, int, time.Time) {
	cmd := exec.Command("git", "-C", repoPath, "log", "-1", "--format=%ct|||%ci|||%s") //nolint:gosec // intentional
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return "—", "—", 999, time.Time{}
	}

	parts := strings.SplitN(strings.TrimSpace(string(out)), "|||", 3)
	if len(parts) == 0 {
		return "—", "—", 999, time.Time{}
	}

	var commitTime time.Time
	if ts, err := strconv.ParseInt(parts[0], 10, 64); err == nil && ts > 0 {
		commitTime = time.Unix(ts, 0)
	}

	dateStr := "—"
	if len(parts) > 1 {
		dateStr = parts[1]
	}
	msg := ""
	if len(parts) > 2 {
		msg = parts[2]
	}

	days := 999
	if !commitTime.IsZero() {
		days = int(time.Since(commitTime).Hours() / 24)
	} else if len(dateStr) >= 10 {
		if t, err := time.Parse("2006-01-02", dateStr[:10]); err == nil {
			days = int(time.Since(t).Hours() / 24)
		}
	}
	if len(dateStr) >= 10 {
		dateStr = dateStr[:10]
	}

	return dateStr, msg, days, commitTime
}

func (s *Scanner) pruneMergedBranches(repoPath string, targetBranch string, currentBranch string) []string {
	cmd := exec.Command("git", "-C", repoPath, "branch", "--format=%(refname:short)", "--merged", targetBranch) //nolint:gosec // intentional
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var pruned []string
	for _, line := range lines {
		b := strings.TrimSpace(line)
		if b == "" || b == "main" || b == "master" || b == "develop" || b == targetBranch || b == currentBranch {
			continue
		}
		// Attempt safe delete
		if err := exec.Command("git", "-C", repoPath, "branch", "-d", b).Run(); err == nil { //nolint:gosec // intentional
			pruned = append(pruned, b)
		}
	}
	return pruned
}

func (s *Scanner) detectStack(repoPath string) string {
	var stacks []string

	if fileExists(filepath.Join(repoPath, "go.mod")) {
		stacks = append(stacks, "Go")
	}
	if fileExists(filepath.Join(repoPath, "Cargo.toml")) {
		stacks = append(stacks, "Rust")
	}
	if fileExists(filepath.Join(repoPath, "package.json")) {
		if fileExists(filepath.Join(repoPath, "tsconfig.json")) {
			stacks = append(stacks, "TypeScript/Node")
		} else {
			stacks = append(stacks, "JavaScript/Node")
		}
	}
	if fileExists(filepath.Join(repoPath, "composer.json")) {
		stacks = append(stacks, "PHP")
	}
	if fileExists(filepath.Join(repoPath, "pom.xml")) || fileExists(filepath.Join(repoPath, "build.gradle")) {
		stacks = append(stacks, "Java")
	}
	if fileExists(filepath.Join(repoPath, "build.sbt")) {
		stacks = append(stacks, "Scala")
	}
	if fileExists(filepath.Join(repoPath, "pyproject.toml")) || fileExists(filepath.Join(repoPath, "requirements.txt")) {
		stacks = append(stacks, "Python")
	}
	if fileExists(filepath.Join(repoPath, "Chart.yaml")) {
		stacks = append(stacks, "Helm")
	}

	if len(stacks) == 0 {
		return "Other"
	}
	return strings.Join(stacks, " + ")
}

func (s *Scanner) inferComponentType(name string, relPath string, stack string) string { //nolint:revive
	n := strings.ToLower(name)
	p := strings.ToLower(relPath)

	if strings.Contains(p, "terraform") || strings.Contains(p, "ansible") || strings.Contains(p, "helm") || strings.Contains(p, "iam") {
		return "🏗️ Infra / IaC"
	}
	if strings.Contains(p, "talks") || strings.Contains(p, "training") || strings.Contains(p, "treinamento") || strings.Contains(p, "katas") {
		return "🎓 Talk / Training"
	}
	if strings.Contains(p, "pocs") || strings.Contains(p, "playground") || strings.Contains(p, "dark-factory") {
		return "⚡ PoC / Research"
	}
	if strings.HasSuffix(n, "-api") || strings.HasPrefix(n, "api-") || strings.Contains(n, "gateway") || strings.Contains(n, "server") {
		return "🌐 API / Gateway"
	}
	if strings.Contains(n, "worker") || strings.Contains(n, "daemon") || strings.Contains(n, "processor") || strings.Contains(n, "router") {
		return "⚙️ Worker / Async"
	}
	if strings.Contains(n, "sdk") || strings.Contains(n, "kit") || strings.Contains(n, "client") {
		return "📦 Lib / SDK"
	}
	if strings.Contains(n, "frontend") || strings.Contains(n, "layout") || strings.Contains(n, "ui") {
		return "🖥️ Frontend / UI"
	}
	if strings.Contains(p, "tool") || strings.Contains(n, "cli") || strings.Contains(n, "dotfiles") {
		return "🛠️ Tool / CLI"
	}

	return "📦 Service"
}

func (s *Scanner) extractDescription(repoPath string) string {
	// 1. Try README
	for _, rname := range []string{"README.md", "readme.md", "README.org"} {
		rf := filepath.Join(repoPath, rname)
		if data, err := os.ReadFile(rf); err == nil { //nolint:gosec // intentional
			lines := strings.Split(string(data), "\n")
			for _, l := range lines {
				lTrim := strings.TrimSpace(l)
				if lTrim == "" || strings.HasPrefix(lTrim, "#") || strings.HasPrefix(lTrim, "[!") ||
					strings.HasPrefix(lTrim, "![") || strings.HasPrefix(lTrim, "```") {
					continue
				}
				if len(lTrim) > 15 {
					clean := regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`).ReplaceAllString(lTrim, "$1")
					clean = regexp.MustCompile(`[#*`+"`"+`_!<>]`).ReplaceAllString(clean, "")
					clean = strings.TrimSpace(clean)
					if len(clean) > 15 {
						if len(clean) > 120 {
							return clean[:120] + "..."
						}
						return clean
					}
				}
			}
		}
	}

	// 2. Try package.json
	pkgJSON := filepath.Join(repoPath, "package.json")
	if data, err := os.ReadFile(pkgJSON); err == nil { //nolint:gosec // intentional
		var m map[string]interface{}
		if err := json.Unmarshal(data, &m); err == nil {
			if desc, ok := m["description"].(string); ok && len(desc) > 5 {
				if len(desc) > 120 {
					return desc[:120] + "..."
				}
				return desc
			}
		}
	}

	return "—"
}

func (s *Scanner) detectCommands(repoPath string, stack string) string {
	var cmds []string

	mkFile := filepath.Join(repoPath, "Makefile")
	if data, err := os.ReadFile(mkFile); err == nil { //nolint:gosec // intentional
		mk := string(data)
		if strings.Contains(mk, "test:") || strings.Contains(mk, "test ") {
			cmds = append(cmds, "`make test`")
		}
		if strings.Contains(mk, "build:") || strings.Contains(mk, "build ") {
			cmds = append(cmds, "`make build`")
		}
	}

	if len(cmds) == 0 {
		if strings.Contains(stack, "Go") {
			cmds = append(cmds, "`go test ./...`")
		} else if strings.Contains(stack, "Rust") {
			cmds = append(cmds, "`cargo test`")
		} else if strings.Contains(stack, "Node") {
			cmds = append(cmds, "`npm test`")
		}
	}

	if len(cmds) == 0 {
		return "—"
	}
	return strings.Join(cmds, ", ")
}

func (s *Scanner) buildVaultMapping() map[string]string {
	mapping := make(map[string]string)
	notesDir := filepath.Join(s.vaultRoot, "notes")

	_ = filepath.Walk(notesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			return err
		}
		slug := strings.TrimSuffix(info.Name(), ".md")
		rel, _ := filepath.Rel(s.vaultRoot, path)
		rel = filepath.ToSlash(rel)

		clean := strings.ToLower(slug)
		mapping[clean] = rel
		return nil
	})

	return mapping
}

// GenerateMOCContent renders the full markdown MOC for all workspace repositories.
func (s *Scanner) GenerateMOCContent(rep *WorkspaceReport) string {
	var sb strings.Builder

	sb.WriteString("---\n")
	sb.WriteString("type: organization\n")
	sb.WriteString("description: Map of Content (MOC) of all Git repositories cloned in ~/Work with tech stack, status, and Vault connections.\n")
	sb.WriteString("tags:\n")
	sb.WriteString("  - meta/workspace\n")
	sb.WriteString("  - organization/work\n")
	sb.WriteString("---\n\n")

	sb.WriteString("# 🗺️ MOC: Workspace (~/Work)\n\n")
	fmt.Fprintf(&sb, "Automated catalog of **%d local Git repositories** in `~/Work`, mapping technology stacks, architectural roles, active branches, and links to Second Brain notes.\n\n", rep.TotalRepos)
	statusParts := []string{
		fmt.Sprintf("%d clean repositories", rep.CleanCount),
	}
	if rep.SyncedCount > 0 {
		statusParts = append(statusParts, fmt.Sprintf("%d synced", rep.SyncedCount))
	}
	if rep.PrunedBranchesCount > 0 {
		statusParts = append(statusParts, fmt.Sprintf("%d merged branches pruned", rep.PrunedBranchesCount))
	}
	fmt.Fprintf(&sb, "> 💡 **Current Status:** %s | %d repositories with local changes.\n\n", strings.Join(statusParts, " • "), rep.DirtyCount)
	sb.WriteString("---\n\n")

	var sortedCats []string //nolint:prealloc
	for c := range rep.Categories {
		sortedCats = append(sortedCats, c)
	}
	sort.Strings(sortedCats)

	for _, cat := range sortedCats {
		repos := rep.Categories[cat]
		fmt.Fprintf(&sb, "## 📁 %s (%d repositories)\n\n", strings.Title(cat), len(repos)) //nolint:staticcheck
		sb.WriteString("| Repository | Type | Stack | Branch | Status | Test / Build | Vault Note |\n")
		sb.WriteString("| :--- | :---: | :---: | :---: | :---: | :---: | :--- |\n")

		for _, r := range repos {
			statusIcon := "🟢 Clean"
			if r.Dirty {
				statusIcon = fmt.Sprintf("🟡 Dirty (%dM/%d?)", r.ModifiedCount, r.UntrackedCount)
			} else if len(r.PrunedBranches) > 0 {
				statusIcon = fmt.Sprintf("🟢 Clean (✂️ %d pruned)", len(r.PrunedBranches))
			} else if r.Synced {
				statusIcon = "🟢 Clean (Synced)"
			}

			vaultCol := "—"
			if r.VaultLink != "" {
				vaultCol = fmt.Sprintf("[%s](../../%s)", r.Name, r.VaultLink)
			}

			fmt.Fprintf(&sb, "| **`%s`** | %s | %s | `%s` | %s | %s | %s |\n",
				r.Name, r.ComponentType, r.Stack, r.Branch, statusIcon, r.Commands, vaultCol)
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
