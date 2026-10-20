//nolint:revive // package comment not required
package harvester

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rjfonseca/daneel/internal/config"
)

// PruneOptions encapsulates parameters for pruning noisy, archived, or excluded projects and directories.
type PruneOptions struct {
	VaultRoot        string
	Config           config.HarvesterConfig
	ActiveByPlatform map[string][]string // platform -> slice of lowercased full paths (e.g. "gitlab" -> ["group/project"])
	DryRun           bool
}

// PrunedItem details a specific removed artifact.
type PrunedItem struct {
	Path        string `json:"path"`
	Platform    string `json:"platform,omitempty"`
	Type        string `json:"type"` // "toxic_dir" or "project"
	Reason      string `json:"reason"`
	RestorePath string `json:"restore_path,omitempty"`
}

// PruneStats reports the summary of the pruning execution.
type PruneStats struct {
	ToxicDirsCount      int          `json:"toxic_dirs_count"`
	ProjectsPrunedCount int          `json:"projects_pruned_count"`
	TotalRemoved        int          `json:"total_removed"`
	Items               []PrunedItem `json:"items"`
	DurationMs          int64        `json:"duration_ms"`
}

// PruneSources scans sources/ across platforms, purging toxic directories, excluded groups/projects, and remote orphans.
func PruneSources(opts PruneOptions) (*PruneStats, error) {
	start := time.Now()
	sourcesDir := filepath.Join(opts.VaultRoot, "sources")
	info, err := os.Lstat(sourcesDir)
	if os.IsNotExist(err) {
		return &PruneStats{}, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("sources must be a real directory inside the vault")
	}
	root, err := os.OpenRoot(opts.VaultRoot)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()

	stats := &PruneStats{
		Items: []PrunedItem{},
	}

	ignoredDirs := opts.Config.GetIgnoredDirectories()
	toxicLookup := make(map[string]bool, len(ignoredDirs))
	for _, ign := range ignoredDirs {
		toxicLookup[strings.ToLower(ign)] = true
	}

	// 1. Purge toxic directories anywhere under sources/
	err = filepath.Walk(sourcesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		dirBase := strings.ToLower(info.Name())
		if toxicLookup[dirBase] {
			rel, _ := filepath.Rel(opts.VaultRoot, path)
			stats.ToxicDirsCount++
			stats.Items = append(stats.Items, PrunedItem{
				Path:   filepath.ToSlash(rel),
				Type:   "toxic_dir",
				Reason: fmt.Sprintf("ignored toxic directory (%s)", dirBase),
			})
			return filepath.SkipDir
		}
		return nil
	})

	if err != nil {
		return stats, err
	}

	// 2. Scan platform repositories (gitlab, github)
	platforms := []string{"gitlab", "github"}
	for _, platform := range platforms {
		platformDir := filepath.Join(sourcesDir, platform)
		if _, err := os.Stat(platformDir); os.IsNotExist(err) {
			continue
		}

		var activeSet map[string]bool
		if opts.ActiveByPlatform != nil {
			if list, ok := opts.ActiveByPlatform[platform]; ok && len(list) > 0 {
				activeSet = make(map[string]bool, len(list))
				for _, it := range list {
					norm := strings.Trim(strings.ToLower(filepath.ToSlash(it)), "/")
					activeSet[norm] = true
				}
			}
		}

		info, err := os.Lstat(platformDir)
		if err != nil {
			return stats, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return stats, fmt.Errorf("platform directory must not be a symlink")
		}
		err = filepath.Walk(platformDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() || path == platformDir {
				return nil
			}

			if isRepositoryDir(path) {
				relPath, relErr := filepath.Rel(platformDir, path)
				if relErr != nil {
					return relErr
				}
				if relPath == "." {
					return nil
				}
				relNorm := strings.Trim(strings.ToLower(filepath.ToSlash(relPath)), "/")
				repoName := filepath.Base(relNorm)
				groupPath := filepath.Dir(relNorm)
				if groupPath == "." {
					groupPath = ""
				}

				reason := ""
				if opts.Config.IsGroupExcluded(groupPath) || opts.Config.IsGroupExcluded(relNorm) {
					reason = "excluded group"
				} else if opts.Config.IsProjectExcluded(repoName, relNorm) {
					reason = "excluded project"
				} else if activeSet != nil && !activeSet[relNorm] {
					reason = "remote orphan or archived"
				}

				if reason != "" {
					vaultRel, _ := filepath.Rel(opts.VaultRoot, path)
					stats.ProjectsPrunedCount++
					stats.Items = append(stats.Items, PrunedItem{
						Path:     filepath.ToSlash(vaultRel),
						Platform: platform,
						Type:     "project",
						Reason:   reason,
					})
				}
				return filepath.SkipDir
			}
			return nil
		})

		if err != nil {
			return stats, err
		}
	}

	// Select parent paths first so a directory is never moved twice.
	sort.Slice(stats.Items, func(i, j int) bool { return stats.Items[i].Path < stats.Items[j].Path })
	selected := make([]PrunedItem, 0, len(stats.Items))
	for _, item := range stats.Items {
		nested := false
		for _, parent := range selected {
			if strings.HasPrefix(item.Path, parent.Path+"/") {
				nested = true
				break
			}
		}
		if !nested {
			selected = append(selected, item)
		}
	}
	stats.Items = selected
	stats.ToxicDirsCount = 0
	stats.ProjectsPrunedCount = 0
	for _, item := range selected {
		if item.Type == "project" {
			stats.ProjectsPrunedCount++
		} else {
			stats.ToxicDirsCount++
		}
	}
	if !opts.DryRun {
		batch := filepath.Join(".daneel", "quarantine", rand.Text())
		for i := range stats.Items {
			item := &stats.Items[i]
			destination := filepath.Join(batch, filepath.FromSlash(item.Path))
			if err := root.MkdirAll(filepath.Dir(destination), 0700); err != nil {
				return stats, fmt.Errorf("prepare quarantine: %w", err)
			}
			if err := root.Rename(filepath.FromSlash(item.Path), destination); err != nil {
				return stats, fmt.Errorf("quarantine %s: %w", item.Path, err)
			}
			item.RestorePath = filepath.ToSlash(destination)
		}
	}

	stats.TotalRemoved = stats.ToxicDirsCount + stats.ProjectsPrunedCount
	stats.DurationMs = time.Since(start).Milliseconds()

	return stats, nil
}

// LoadActiveRemoteProjects queries remote targets to gather the current set of active, non-archived projects.
func LoadActiveRemoteProjects(vaultRoot string, cfg config.HarvesterConfig, token string) (map[string][]string, error) {
	active := make(map[string][]string)

	for _, target := range cfg.Targets {
		platform := strings.ToLower(target.Platform)
		switch platform {
		case "gitlab":
			gl := NewGitLabHarvester(vaultRoot, token, cfg)
			var projects []glProject
			var err error
			if target.TargetType() == "user" {
				projects, err = gl.getUserProjects(target.TargetName(), 0)
			} else {
				projects, err = gl.getGroupProjects(target.TargetName(), 0)
			}
			if err != nil {
				return nil, fmt.Errorf("list GitLab projects for pruning: %w", err)
			}
			{
				for _, p := range projects {
					if cfg.ShouldSkipArchived() && p.Archived {
						continue
					}
					if cfg.IsGroupExcluded(p.PathWithNamespace) || cfg.IsProjectExcluded(p.Name, p.PathWithNamespace) {
						continue
					}
					active["gitlab"] = append(active["gitlab"], strings.ToLower(p.PathWithNamespace))
				}
			}
		case "github":
			gh := NewGitHubHarvester(vaultRoot, token, cfg)
			var repos []ghRepo
			var err error
			if target.TargetType() == "user" {
				repos, err = gh.getUserRepos(target.TargetName(), 0)
			} else {
				repos, err = gh.getOrgRepos(target.TargetName(), 0)
			}
			if err != nil {
				return nil, fmt.Errorf("list GitHub projects for pruning: %w", err)
			}
			{
				for _, r := range repos {
					if cfg.ShouldSkipArchived() && r.Archived {
						continue
					}
					fullName := r.FullName
					if fullName == "" {
						fullName = fmt.Sprintf("%s/%s", r.Owner.Login, r.Name)
					}
					if cfg.IsGroupExcluded(r.Owner.Login) || cfg.IsGroupExcluded(fullName) || cfg.IsProjectExcluded(r.Name, fullName) {
						continue
					}
					active["github"] = append(active["github"], strings.ToLower(fullName))
				}
			}
		}
	}

	return active, nil
}
