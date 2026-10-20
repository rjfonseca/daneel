//nolint:revive // package comment not required
package harvester

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// RepositoryMetadata contains literal OKF frontmatter attributes for a repository MOC.
type RepositoryMetadata struct {
	Type        string   `yaml:"type"`
	Title       string   `yaml:"title,omitempty"`
	Description string   `yaml:"description,omitempty"`
	Repository  string   `yaml:"repository"`
	Group       string   `yaml:"group,omitempty"`
	Platform    string   `yaml:"platform,omitempty"`
	URL         string   `yaml:"url,omitempty"`
	Tags        []string `yaml:"tags,omitempty"`
}

// GroupMetadata contains literal OKF frontmatter attributes for an organizational group MOC.
type GroupMetadata struct {
	Type        string   `yaml:"type"`
	Title       string   `yaml:"title,omitempty"`
	Description string   `yaml:"description,omitempty"`
	Group       string   `yaml:"group"`
	Platform    string   `yaml:"platform,omitempty"`
	Tags        []string `yaml:"tags,omitempty"`
}

// EnsureRepositoryIndex creates or updates the index.md MOC at the root of a repository.
// If index.md already exists, it preserves existing content while injecting/updating
// standard OKF frontmatter and ensuring catalog links are present.
func EnsureRepositoryIndex(repoDir, repoName, groupPath, platform, repoURL, repoDesc string) error {
	if err := os.MkdirAll(repoDir, 0750); err != nil {
		return err
	}

	indexPath := filepath.Join(repoDir, "index.md")

	// 1. Gather all markdown files in repoDir (excluding index.md)
	var docFiles []string
	_ = filepath.Walk(repoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		// Only skip the repository root index.md, so legitimate subfolder docs (e.g. docs/index.md) are cataloged!
		if path == filepath.Join(repoDir, "index.md") || strings.EqualFold(info.Name(), "_index.md") {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".md" || ext == ".markdown" || ext == ".org" {
			rel, relErr := filepath.Rel(repoDir, path)
			if relErr == nil {
				cleanRel := filepath.ToSlash(rel)
				parts := strings.Split(cleanRel, "/")
				isToxic := false
				for _, p := range parts {
					pLow := strings.ToLower(p)
					if pLow == "node_modules" || pLow == "vendor" || pLow == ".git" || pLow == "dist" || pLow == "build" {
						isToxic = true
						break
					}
				}
				if !isToxic {
					docFiles = append(docFiles, cleanRel)
				}
			}
		}
		return nil
	})
	sort.Strings(docFiles)

	title := repoName
	if title == "" {
		title = filepath.Base(repoDir)
	}

	tags := []string{"repository"}
	if platform != "" {
		tags = append(tags, strings.ToLower(platform))
	}

	// 2. Read existing content if present
	var existingContent string
	var existingBody string
	var existingMeta map[string]interface{}

	if data, err := os.ReadFile(indexPath); err == nil {
		existingContent = string(data)
		existingMeta, existingBody = parseFrontmatterMap(existingContent)
	}

	if existingMeta == nil {
		existingMeta = make(map[string]interface{})
	}

	// Set standard OKF fields
	existingMeta["type"] = "repository"
	existingMeta["repository"] = repoName
	if groupPath != "" {
		existingMeta["group"] = groupPath
	}
	if platform != "" {
		existingMeta["platform"] = strings.ToLower(platform)
	}
	if repoURL != "" {
		existingMeta["url"] = repoURL
	}
	if existingMeta["title"] == nil || existingMeta["title"] == "" {
		existingMeta["title"] = title
	}
	if (existingMeta["description"] == nil || existingMeta["description"] == "") && repoDesc != "" {
		existingMeta["description"] = repoDesc
	}
	if existingMeta["tags"] == nil {
		existingMeta["tags"] = tags
	}

	fmBytes, err := yaml.Marshal(existingMeta)
	if err != nil {
		return fmt.Errorf("failed to marshal repository frontmatter: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.Write(fmBytes)
	sb.WriteString("---\n\n")

	if existingBody != "" {
		sb.WriteString(strings.TrimSpace(existingBody))
		sb.WriteString("\n\n")
	} else {
		fmt.Fprintf(&sb, "# 📦 %s\n\n", title)
		if repoDesc != "" {
			fmt.Fprintf(&sb, "> %s\n\n", repoDesc)
		}
		if platform != "" {
			fmt.Fprintf(&sb, "**Platform:** `%s`  \n", strings.Title(platform)) //nolint:staticcheck
		}
		if groupPath != "" {
			fmt.Fprintf(&sb, "**Group:** `%s`  \n", groupPath)
		}
		fmt.Fprintf(&sb, "**Repository:** `%s`  \n\n", repoName)
		sb.WriteString("---\n\n")
	}

	// Append catalog section if not already present and there are doc files
	if len(docFiles) > 0 && !strings.Contains(existingBody, "Documentação Catalogada") && !strings.Contains(existingBody, "Cataloged Documentation") {
		sb.WriteString("## 📚 Cataloged Documentation\n")
		for _, f := range docFiles {
			cleanName := strings.TrimSuffix(filepath.Base(f), filepath.Ext(f))
			if strings.EqualFold(cleanName, "README") {
				cleanName = "README"
			}
			fmt.Fprintf(&sb, "- [%s](%s)\n", cleanName, f)
		}
		sb.WriteString("\n")
	}

	return os.WriteFile(indexPath, []byte(sb.String()), 0600)
}

// EnsureGroupIndex creates or updates the index.md MOC at the root of a group or subgroup directory.
func EnsureGroupIndex(groupDir, groupName, groupPath, platform string, subdirs, repos []string) error {
	if err := os.MkdirAll(groupDir, 0750); err != nil {
		return err
	}

	indexPath := filepath.Join(groupDir, "index.md")

	// If index.md exists and is already a repository, do not overwrite it as a group
	if data, err := os.ReadFile(indexPath); err == nil {
		if strings.Contains(string(data), "type: repository") {
			return nil
		}
	}

	title := groupName
	if title == "" {
		title = filepath.Base(groupDir)
	}

	tags := []string{"group"}
	if platform != "" {
		tags = append(tags, strings.ToLower(platform))
	}

	meta := GroupMetadata{
		Type:        "group",
		Title:       title,
		Description: fmt.Sprintf("Organizational group %s", groupPath),
		Group:       groupPath,
		Platform:    strings.ToLower(platform),
		Tags:        tags,
	}

	fmBytes, err := yaml.Marshal(meta)
	if err != nil {
		return fmt.Errorf("failed to marshal group frontmatter: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.Write(fmBytes)
	sb.WriteString("---\n\n")

	fmt.Fprintf(&sb, "# 🌌 Group: %s\n\n", title)
	if platform != "" {
		fmt.Fprintf(&sb, "**Platform:** `%s`  \n", strings.Title(platform)) //nolint:staticcheck
	}
	fmt.Fprintf(&sb, "**Group:** `%s`  \n\n", groupPath)
	sb.WriteString("---\n\n")

	sort.Strings(subdirs)
	sort.Strings(repos)

	if len(subdirs) > 0 {
		sb.WriteString("## 📁 Subgroups\n")
		for _, sub := range subdirs {
			fmt.Fprintf(&sb, "- [📁 %s](%s/index.md)\n", sub, sub)
		}
		sb.WriteString("\n")
	}

	if len(repos) > 0 {
		sb.WriteString("## 📦 Repositories\n")
		for _, r := range repos {
			fmt.Fprintf(&sb, "- [📦 %s](%s/index.md)\n", r, r)
		}
		sb.WriteString("\n")
	}

	return os.WriteFile(indexPath, []byte(sb.String()), 0600)
}

// CatalogReconcileStats aggregates the result of reconciling sources MOCs.
type CatalogReconcileStats struct {
	TotalReposIndexed  int
	TotalGroupsIndexed int
}

// ReconcileSourcesCatalog walks the sources/ directory and generates/repairs
// MOC index.md files for all repositories and groups.
func ReconcileSourcesCatalog(vaultRoot string) (*CatalogReconcileStats, error) {
	stats := &CatalogReconcileStats{}
	sourcesRoot := filepath.Join(vaultRoot, "sources")
	stat, err := os.Stat(sourcesRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return stats, nil
		}
		return nil, err
	}
	if !stat.IsDir() {
		return stats, nil
	}

	// 1. Reconcile GitLab: sources/gitlab/<group>/.../<repo>
	glRoot := filepath.Join(sourcesRoot, "gitlab")
	if stat, err := os.Stat(glRoot); err == nil && stat.IsDir() {
		glStats, err := reconcilePlatformDir(glRoot, "gitlab")
		if err == nil && glStats != nil {
			stats.TotalReposIndexed += glStats.TotalReposIndexed
			stats.TotalGroupsIndexed += glStats.TotalGroupsIndexed
		}
	}

	// 2. Reconcile GitHub: sources/github/<owner>/<repo>
	ghRoot := filepath.Join(sourcesRoot, "github")
	if stat, err := os.Stat(ghRoot); err == nil && stat.IsDir() {
		ghStats, err := reconcilePlatformDir(ghRoot, "github")
		if err == nil && ghStats != nil {
			stats.TotalReposIndexed += ghStats.TotalReposIndexed
			stats.TotalGroupsIndexed += ghStats.TotalGroupsIndexed
		}
	}

	return stats, nil
}

var repoInternalDirs = map[string]bool{
	"docs": true, "doc": true, "documentation": true,
	"specs": true, "spec": true, "plans": true, "conductor": true,
	"runbooks": true, "runbook": true, "adrs": true, "adr": true,
	"tracks": true, "archive": true, "superpowers": true,
	"rfcs": true, "rfc": true, "packages": true, "examples": true,
	"vendor": true, "node_modules": true, "dist": true, "build": true,
	"src": true, "internal": true, "pkg": true, "cmd": true,
}

func reconcilePlatformDir(platformDir, platform string) (*CatalogReconcileStats, error) {
	stats := &CatalogReconcileStats{}

	// Map of dir -> isRepo
	repoDirs := make(map[string]bool)
	allDirs := make(map[string]bool)

	// 1. Detect repository directories top-down:
	// Once a repository root is detected, skip descending into its children!
	_ = filepath.Walk(platformDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr // continue walking on unreadable paths
		}
		if !info.IsDir() || path == platformDir {
			return nil
		}

		allDirs[path] = true

		isRepo := false
		if platform == "github" {
			rel, relErr := filepath.Rel(platformDir, path)
			if relErr == nil {
				parts := strings.Split(filepath.ToSlash(rel), "/")
				if len(parts) == 2 {
					isRepo = true
				}
			}
		}

		if !isRepo && isRepositoryDir(path) {
			isRepo = true
		}

		if isRepo {
			repoDirs[path] = true
			return filepath.SkipDir
		}
		return nil
	})

	// 2. Sanitize or purge spurious index.md / _index.md in subdirectories of every repository
	var subfolderIndices []string
	for rDir := range repoDirs {
		_ = filepath.Walk(rDir, func(subPath string, subInfo os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return nil //nolint:nilerr // continue walking on unreadable paths
			}
			if subInfo.IsDir() {
				return nil
			}
			if subPath == filepath.Join(rDir, "index.md") {
				return nil // Keep the repository root index.md!
			}
			name := strings.ToLower(subInfo.Name())
			if name == "index.md" || name == "_index.md" {
				subfolderIndices = append(subfolderIndices, subPath)
			}
			return nil
		})
	}
	for _, p := range subfolderIndices {
		sanitizeOrRemoveSubfolderIndex(p)
	}

	// 3. Ensure single canonical repository MOCs
	for rDir := range repoDirs {
		relFromPlatform, _ := filepath.Rel(platformDir, rDir)
		relFromPlatform = filepath.ToSlash(relFromPlatform)
		parts := strings.Split(relFromPlatform, "/")
		repoName := parts[len(parts)-1]
		groupPath := strings.Join(parts[:len(parts)-1], "/")

		err := EnsureRepositoryIndex(rDir, repoName, groupPath, platform, "", "")
		if err == nil {
			stats.TotalReposIndexed++
		}
	}

	// 4. Build parent-to-children directory map to build group MOCs
	// Collect groups (ancestors of repositories, excluding repoDirs and platformDir)
	groupDirs := make(map[string]bool)
	for rDir := range repoDirs {
		parent := filepath.Dir(rDir)
		for parent != platformDir && parent != "." && parent != "/" {
			groupDirs[parent] = true
			parent = filepath.Dir(parent)
		}
	}

	for gDir := range groupDirs {
		relFromPlatform, _ := filepath.Rel(platformDir, gDir)
		groupPath := filepath.ToSlash(relFromPlatform)
		groupName := filepath.Base(gDir)

		entries, err := os.ReadDir(gDir)
		if err != nil {
			continue
		}

		var subdirs []string
		var repos []string
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			childPath := filepath.Join(gDir, e.Name())
			if repoDirs[childPath] {
				repos = append(repos, e.Name())
			} else if groupDirs[childPath] {
				subdirs = append(subdirs, e.Name())
			}
		}

		err = EnsureGroupIndex(gDir, groupName, groupPath, platform, subdirs, repos)
		if err == nil {
			stats.TotalGroupsIndexed++
		}
	}

	return stats, nil
}

// isRepositoryDir checks if a directory represents a repository root.
func isRepositoryDir(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}

	hasGit := false
	hasCatalogInfo := false
	hasReadme := false
	hasDirectDocs := false
	hasInternalDocDir := false

	for _, e := range entries {
		nameLower := strings.ToLower(e.Name())
		if e.IsDir() {
			if nameLower == ".git" {
				hasGit = true
			} else if repoInternalDirs[nameLower] {
				hasInternalDocDir = true
			}
		} else {
			if nameLower == "catalog-info.yaml" || nameLower == "catalog-info.yml" {
				hasCatalogInfo = true
			} else if nameLower == "readme.md" || nameLower == "readme" || nameLower == "readme.markdown" {
				hasReadme = true
			} else if strings.HasSuffix(nameLower, ".md") || strings.HasSuffix(nameLower, ".markdown") || strings.HasSuffix(nameLower, ".org") {
				if nameLower != "index.md" && nameLower != "_index.md" {
					hasDirectDocs = true
				}
			}
		}
	}

	if hasGit || hasCatalogInfo || hasReadme || hasInternalDocDir || hasDirectDocs {
		return true
	}

	// Also check if index.md explicitly claims "type: repository"
	indexPath := filepath.Join(dir, "index.md")
	if data, err := os.ReadFile(indexPath); err == nil {
		if strings.Contains(string(data), "type: repository") {
			return true
		}
	}

	return false
}

func parseFrontmatterMap(content string) (map[string]interface{}, string) {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "---") {
		return nil, content
	}

	parts := strings.SplitN(trimmed[3:], "---", 2)
	if len(parts) < 2 {
		return nil, content
	}

	fmRaw := parts[0]
	body := parts[1]

	var meta map[string]interface{}
	if err := yaml.Unmarshal([]byte(fmRaw), &meta); err != nil {
		return nil, content
	}

	return meta, body
}

// sanitizeOrRemoveSubfolderIndex inspects an index.md found in a repository subfolder.
// If it was purely generated by Daneel's cataloger (zero human content), it is deleted.
// If it contains legitimate user documentation (committed by developers), it is preserved,
// and only the erroneous "type: repository" or "type: group" is stripped to prevent phantom stars in Galaxy.
func sanitizeOrRemoveSubfolderIndex(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	content := string(data)

	meta, body := parseFrontmatterMap(content)
	if meta == nil {
		// No YAML frontmatter: this is 100% human-authored markdown! Never delete!
		return
	}

	t, _ := meta["type"].(string)
	isMocType := strings.EqualFold(t, "repository") || strings.EqualFold(t, "group")
	if !isMocType {
		// Legitimate project documentation (e.g. type: docs, or no type): Never delete!
		return
	}

	// Check if body contains any human text outside Daneel MOC boilerplate
	hasHumanContent := false
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "---") {
			continue
		}
		// Boilerplate headers and lines generated by EnsureRepositoryIndex or EnsureGroupIndex
		if strings.HasPrefix(trimmed, "# 📦") || strings.HasPrefix(trimmed, "# 🌌") ||
			strings.HasPrefix(trimmed, "**Platform:**") || strings.HasPrefix(trimmed, "**Group:**") ||
			strings.HasPrefix(trimmed, "**Repository:**") ||
			strings.HasPrefix(trimmed, "## 📚 Cataloged Documentation") ||
			strings.HasPrefix(trimmed, "## Documentação Catalogada") ||
			strings.HasPrefix(trimmed, "## 📁 Subgroups") ||
			strings.HasPrefix(trimmed, "## 📦 Repositories") ||
			strings.HasPrefix(trimmed, "- [📁") || strings.HasPrefix(trimmed, "- [📦") ||
			(strings.HasPrefix(trimmed, "- [") && strings.Contains(trimmed, "](")) {
			continue
		}
		// Any other text indicates human-authored content!
		hasHumanContent = true
		break
	}

	if !hasHumanContent {
		// Purely Daneel-generated artifact with zero human content: safe to remove
		_ = os.Remove(path)
		return
	}

	// Legitimate documentation that had MOC type injected: preserve file, sanitize frontmatter
	delete(meta, "type")
	delete(meta, "repository")
	delete(meta, "group")

	fmBytes, err := yaml.Marshal(meta)
	if err != nil {
		return
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.Write(fmBytes)
	sb.WriteString("---\n\n")
	sb.WriteString(strings.TrimSpace(body))
	sb.WriteString("\n")

	_ = os.WriteFile(path, []byte(sb.String()), 0600)
}
