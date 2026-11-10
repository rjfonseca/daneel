package ingest

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	graphv1 "github.com/rjfonseca/daneel/api/v1"
	"github.com/rjfonseca/daneel/internal/safefile"
	"github.com/rjfonseca/daneel/internal/search"
	"github.com/rjfonseca/daneel/internal/vault"
)

var (
	fmRegex = regexp.MustCompile(`(?s)^---\s*\r?\n(.*?)\r?\n---\s*\r?\n`)
	h1Regex = regexp.MustCompile(`(?m)^#\s+(.+)$`)
)

// Engine coordinates safe note ingestion, metadata preservation, and collision avoidance.
type Engine struct {
	vaultRoot    string
	searchEngine *search.Engine
	eventHub     EventHub
	mu           sync.Mutex
}

// NewEngine initializes a unified note ingestion engine.
func NewEngine(vaultRoot string, searchEngine *search.Engine, eventHub EventHub) *Engine {
	absRoot, err := filepath.Abs(vaultRoot)
	if err != nil {
		absRoot = vaultRoot
	}
	if evalRoot, err := filepath.EvalSymlinks(absRoot); err == nil {
		absRoot = evalRoot
	}
	return &Engine{
		vaultRoot:    absRoot,
		searchEngine: searchEngine,
		eventHub:     eventHub,
	}
}

func evalCanonical(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = p
	}
	if eval, err := filepath.EvalSymlinks(abs); err == nil {
		return eval
	}
	return abs
}

func isContained(baseDir, targetPath string) (bool, error) {
	evalBase := evalCanonical(baseDir)
	evalTarget := evalCanonical(targetPath)
	rel, err := filepath.Rel(evalBase, evalTarget)
	if err != nil {
		return false, err
	}
	if strings.HasPrefix(rel, "..") || !filepath.IsLocal(rel) {
		return false, nil
	}
	return true, nil
}

// IngestPath ingests a single file or a directory from hostPath into the vault.
func (e *Engine) IngestPath(ctx context.Context, hostPath string, opts IngestOptions) (*IngestReport, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	absHost, err := filepath.Abs(hostPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	lstat, err := os.Lstat(absHost)
	if err != nil {
		return nil, fmt.Errorf("source path does not exist: %w", err)
	}

	targetPath := absHost
	// Verify symlink boundaries
	if lstat.Mode()&os.ModeSymlink != 0 {
		target, err := filepath.EvalSymlinks(absHost)
		if err != nil {
			return nil, fmt.Errorf("resolve symlink: %w", err)
		}
		// Check containment
		inVault, _ := isContained(e.vaultRoot, absHost)
		if inVault {
			targetInVault, err := isContained(e.vaultRoot, target)
			if err != nil || !targetInVault {
				return nil, fmt.Errorf("symlink %q escapes vault root", hostPath)
			}
		} else {
			sourceDir := filepath.Dir(absHost)
			targetInSource, err := isContained(sourceDir, target)
			if err != nil || !targetInSource {
				return nil, fmt.Errorf("symlink %q escapes source tree", hostPath)
			}
		}
		lstat, err = os.Stat(target)
		if err != nil {
			return nil, fmt.Errorf("stat symlink target: %w", err)
		}
		targetPath = target
	}

	if lstat.IsDir() {
		return e.IngestDirectory(ctx, targetPath, opts)
	}

	destDir, err := e.sanitizeDestination(opts.Destination)
	if err != nil {
		return nil, err
	}

	note, err := e.ingestSingleFile(ctx, absHost, destDir, filepath.Base(absHost), opts)
	if err != nil {
		return nil, err
	}

	return &IngestReport{
		Items:          []IngestedNote{*note},
		TotalProcessed: 1,
	}, nil
}

// IngestDirectory traverses hostDir and ingests all supported markdown and text documents.
func (e *Engine) IngestDirectory(ctx context.Context, hostDir string, opts IngestOptions) (*IngestReport, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	absDir, err := filepath.Abs(hostDir)
	if err != nil {
		return nil, fmt.Errorf("invalid directory path: %w", err)
	}

	stat, err := os.Stat(absDir)
	if err != nil {
		return nil, fmt.Errorf("directory not found: %w", err)
	}
	if !stat.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", hostDir)
	}

	destBase, err := e.sanitizeDestination(opts.Destination)
	if err != nil {
		return nil, err
	}

	report := &IngestReport{}

	err = filepath.Walk(absDir, func(currentPath string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			report.Errors = append(report.Errors, walkErr.Error())
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}

		if info.IsDir() {
			if currentPath == absDir {
				return nil
			}
			name := info.Name()
			if strings.HasPrefix(name, ".") || isIgnoredDir(name) {
				return filepath.SkipDir
			}
			if !opts.Recursive {
				return filepath.SkipDir
			}
			return nil
		}

		// Handle symlink verification
		lstat, lerr := os.Lstat(currentPath)
		if lerr == nil && lstat.Mode()&os.ModeSymlink != 0 {
			target, err := filepath.EvalSymlinks(currentPath)
			if err != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("failed to resolve symlink %s: %v", currentPath, err))
				return nil //nolint:nilerr // continue walking despite broken symlink
			}
			contained, err := isContained(absDir, target)
			if err != nil || !contained {
				report.Errors = append(report.Errors, fmt.Sprintf("symlink %s escapes source directory", currentPath))
				return nil //nolint:nilerr // continue walking despite escaping symlink
			}
			targetInfo, statErr := os.Stat(target)
			if statErr != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("stat symlink target %s: %v", target, statErr))
				return nil //nolint:nilerr // continue walking despite unreadable symlink target
			}
			if targetInfo.IsDir() {
				return nil
			}
			info = targetInfo
		}

		ext := strings.ToLower(filepath.Ext(currentPath))
		if ext != ".md" && ext != ".markdown" && ext != ".txt" {
			return nil
		}

		relInHost, err := filepath.Rel(absDir, currentPath)
		if err != nil {
			report.Errors = append(report.Errors, err.Error())
			return nil
		}

		subDir := filepath.ToSlash(filepath.Dir(relInHost))
		var destDir string
		if subDir == "." {
			destDir = destBase
		} else {
			destDir = path.Join(destBase, subDir)
		}

		note, err := e.ingestSingleFile(ctx, currentPath, destDir, filepath.Base(currentPath), opts)
		if err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("%s: %v", currentPath, err))
			return nil
		}

		report.Items = append(report.Items, *note)
		report.TotalProcessed++
		return nil
	})

	if err != nil {
		return nil, err
	}

	return report, nil
}

// IngestContent saves arbitrary in-memory note content directly to the vault.
func (e *Engine) IngestContent(ctx context.Context, title string, content []byte, opts IngestOptions) (*IngestedNote, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	title = strings.TrimSpace(title)
	if title == "" && len(strings.TrimSpace(string(content))) == 0 {
		return nil, fmt.Errorf("title and content cannot both be empty")
	}

	destDir, err := e.sanitizeDestination(opts.Destination)
	if err != nil {
		return nil, err
	}

	data, finalTitle, err := processFrontmatter(content, title, "", opts)
	if err != nil {
		return nil, fmt.Errorf("process frontmatter: %w", err)
	}

	var filename string
	if finalTitle != "" {
		filename = vault.ToKebabCase(finalTitle) + ".md"
	}
	if filename == "" || filename == ".md" {
		filename = "note-" + time.Now().UTC().Format("20060102-150405") + ".md"
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	targetRel := path.Join(destDir, filename)
	resolvedRel, warning := e.resolveCollision(targetRel)

	absTarget := filepath.Join(e.vaultRoot, filepath.FromSlash(resolvedRel))
	if err := safefile.WriteFile(e.vaultRoot, absTarget, data, 0600); err != nil {
		return nil, fmt.Errorf("write note: %w", err)
	}

	if e.searchEngine != nil {
		_ = e.searchEngine.IndexNote(ctx, filepath.ToSlash(resolvedRel))
	}
	if e.eventHub != nil {
		e.eventHub.PublishUIEvent(&graphv1.UIEvent{
			Action:    graphv1.UIActionType_ACTION_GALAXY_RELOAD,
			Timestamp: time.Now().UnixMilli(),
		})
	}

	return &IngestedNote{
		RelPath:   filepath.ToSlash(resolvedRel),
		Title:     finalTitle,
		SizeBytes: int64(len(data)),
		Warning:   warning,
	}, nil
}

func (e *Engine) ingestSingleFile(ctx context.Context, hostPath, destDir, filename string, opts IngestOptions) (*IngestedNote, error) {
	raw, err := os.ReadFile(hostPath)
	if err != nil {
		return nil, fmt.Errorf("read source file: %w", err)
	}

	cleanDest, err := e.sanitizeDestination(destDir)
	if err != nil {
		return nil, err
	}

	baseName := strings.TrimSuffix(filename, filepath.Ext(filename))
	fallbackTitle := strings.Title(strings.ReplaceAll(baseName, "-", " ")) //nolint:staticcheck

	data, finalTitle, err := processFrontmatter(raw, fallbackTitle, hostPath, opts)
	if err != nil {
		return nil, fmt.Errorf("process frontmatter: %w", err)
	}

	outExt := filepath.Ext(filename)
	if outExt == "" {
		filename += ".md"
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	targetRel := path.Join(cleanDest, filename)
	resolvedRel, warning := e.resolveCollision(targetRel)

	absTarget := filepath.Join(e.vaultRoot, filepath.FromSlash(resolvedRel))
	if err := safefile.WriteFile(e.vaultRoot, absTarget, data, 0600); err != nil {
		return nil, fmt.Errorf("write note: %w", err)
	}

	if opts.Move {
		if err := os.Remove(hostPath); err != nil {
			if warning != "" {
				warning += "; failed to remove source file: " + err.Error()
			} else {
				warning = "failed to remove source file: " + err.Error()
			}
		}
	}

	if e.searchEngine != nil {
		_ = e.searchEngine.IndexNote(ctx, filepath.ToSlash(resolvedRel))
	}
	if e.eventHub != nil {
		e.eventHub.PublishUIEvent(&graphv1.UIEvent{
			Action:    graphv1.UIActionType_ACTION_GALAXY_RELOAD,
			Timestamp: time.Now().UnixMilli(),
		})
	}

	return &IngestedNote{
		SourcePath: hostPath,
		RelPath:    filepath.ToSlash(resolvedRel),
		Title:      finalTitle,
		SizeBytes:  int64(len(data)),
		Warning:    warning,
	}, nil
}

func (e *Engine) sanitizeDestination(dest string) (string, error) {
	if dest == "" {
		dest = "inbox"
	}
	clean := path.Clean(strings.ReplaceAll(dest, "\\", "/"))
	if filepath.IsAbs(filepath.FromSlash(clean)) {
		evalVault := evalCanonical(e.vaultRoot)
		evalClean := evalCanonical(clean)
		rel, err := filepath.Rel(evalVault, evalClean)
		if err != nil || strings.HasPrefix(rel, "..") || !filepath.IsLocal(rel) {
			return "", fmt.Errorf("destination path %q escapes vault root", dest)
		}
		clean = filepath.ToSlash(rel)
	}
	if strings.HasPrefix(clean, "..") || !filepath.IsLocal(filepath.FromSlash(clean)) {
		return "", fmt.Errorf("destination path %q escapes vault root", dest)
	}

	// Verify symlink destination directories do not escape vault root
	evalVault := evalCanonical(e.vaultRoot)
	parts := strings.Split(clean, "/")
	curr := e.vaultRoot
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		curr = filepath.Join(curr, filepath.FromSlash(part))
		if evalCurr, err := filepath.EvalSymlinks(curr); err == nil {
			rel, err := filepath.Rel(evalVault, evalCurr)
			if err != nil || strings.HasPrefix(rel, "..") || !filepath.IsLocal(rel) {
				return "", fmt.Errorf("destination directory %q is a symlink escaping vault root", dest)
			}
		} else if os.IsNotExist(err) {
			break
		}
	}

	return clean, nil
}

func (e *Engine) resolveCollision(targetRel string) (string, string) {
	fullPath := filepath.Join(e.vaultRoot, filepath.FromSlash(targetRel))
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return filepath.ToSlash(targetRel), ""
	}

	ext := path.Ext(targetRel)
	base := strings.TrimSuffix(targetRel, ext)
	now := time.Now().UTC()
	candidate := fmt.Sprintf("%s-%s%s", base, now.Format("20060102-150405"), ext)

	if _, err := os.Stat(filepath.Join(e.vaultRoot, filepath.FromSlash(candidate))); os.IsNotExist(err) {
		warning := fmt.Sprintf("file collision detected; saved as %s", filepath.ToSlash(candidate))
		return filepath.ToSlash(candidate), warning
	}

	for i := 1; ; i++ {
		candidateWithCounter := fmt.Sprintf("%s-%s-%d%s", base, now.Format("20060102-150405"), i, ext)
		if _, err := os.Stat(filepath.Join(e.vaultRoot, filepath.FromSlash(candidateWithCounter))); os.IsNotExist(err) {
			warning := fmt.Sprintf("file collision detected; saved as %s", filepath.ToSlash(candidateWithCounter))
			return filepath.ToSlash(candidateWithCounter), warning
		}
	}
}

func processFrontmatter(rawContent []byte, fallbackTitle, sourcePath string, opts IngestOptions) ([]byte, string, error) {
	fmMatch := fmRegex.FindSubmatch(rawContent)
	if fmMatch != nil {
		yamlBlock := fmMatch[1]
		body := rawContent[len(fmMatch[0]):]

		var fm map[string]any
		if err := yaml.Unmarshal(yamlBlock, &fm); err != nil || fm == nil {
			fm = make(map[string]any)
		}

		title := ""
		if titleStr, ok := fm["title"].(string); ok && strings.TrimSpace(titleStr) != "" {
			title = strings.TrimSpace(titleStr)
		} else {
			if h1 := h1Regex.FindSubmatch(body); len(h1) > 1 {
				title = strings.TrimSpace(string(h1[1]))
			} else if fallbackTitle != "" {
				title = fallbackTitle
			}
			if title != "" {
				fm["title"] = title
			}
		}

		fm["ingested_at"] = time.Now().UTC().Format(time.RFC3339)

		if sourcePath != "" && fm["source_path"] == nil {
			fm["source_path"] = sourcePath
		}
		if opts.SourceURL != "" && fm["source_url"] == nil {
			fm["source_url"] = opts.SourceURL
		}

		if len(opts.Tags) > 0 {
			existingTags := extractTagList(fm["tags"])
			seen := make(map[string]bool)
			var merged []string
			for _, t := range existingTags {
				if !seen[t] {
					seen[t] = true
					merged = append(merged, t)
				}
			}
			for _, t := range opts.Tags {
				t = strings.TrimSpace(t)
				if t != "" && !seen[t] {
					seen[t] = true
					merged = append(merged, t)
				}
			}
			fm["tags"] = merged
		}

		newFmBytes, err := yaml.Marshal(fm)
		if err != nil {
			return nil, "", err
		}

		result := append([]byte("---\n"), newFmBytes...)
		result = append(result, []byte("---\n")...)
		result = append(result, body...)
		return result, title, nil
	}

	// No frontmatter present
	title := ""
	if h1 := h1Regex.FindSubmatch(rawContent); len(h1) > 1 {
		title = strings.TrimSpace(string(h1[1]))
	} else if fallbackTitle != "" {
		title = fallbackTitle
	}
	if title == "" {
		title = "Untitled Note"
	}

	fm := map[string]any{
		"title":       title,
		"ingested_at": time.Now().UTC().Format(time.RFC3339),
	}
	if sourcePath != "" {
		fm["source_path"] = sourcePath
	}
	if opts.SourceURL != "" {
		fm["source_url"] = opts.SourceURL
	}
	if len(opts.Tags) > 0 {
		seen := make(map[string]bool)
		var tags []string
		for _, t := range opts.Tags {
			t = strings.TrimSpace(t)
			if t != "" && !seen[t] {
				seen[t] = true
				tags = append(tags, t)
			}
		}
		if len(tags) > 0 {
			fm["tags"] = tags
		}
	}

	newFmBytes, err := yaml.Marshal(fm)
	if err != nil {
		return nil, "", err
	}

	bodyStr := strings.TrimLeft(string(rawContent), "\r\n")
	result := "---\n" + string(newFmBytes) + "---\n\n" + bodyStr
	return []byte(result), title, nil
}

func extractTagList(val any) []string {
	if val == nil {
		return nil
	}
	var res []string
	switch v := val.(type) {
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					res = append(res, s)
				}
			}
		}
	case []string:
		for _, s := range v {
			s = strings.TrimSpace(s)
			if s != "" {
				res = append(res, s)
			}
		}
	case string:
		for _, s := range strings.Split(v, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				res = append(res, s)
			}
		}
	}
	return res
}

func isIgnoredDir(name string) bool {
	switch name {
	case ".git", ".dev", "node_modules", ".obsidian", ".agents", ".daneel", "vendor", "dist", "build":
		return true
	default:
		return strings.HasPrefix(name, ".")
	}
}
