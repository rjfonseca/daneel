//nolint:revive // package comment not required
package harvester

import (
	"time"

	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"

	"github.com/rjfonseca/daneel/internal/config"
)

// GDriveHarvester handles downloading documents from Google Drive.
type GDriveHarvester struct {
	cfg *config.Config
}

// NewGDriveHarvester creates a new Google Drive harvester.
func NewGDriveHarvester(cfg *config.Config) *GDriveHarvester {
	return &GDriveHarvester{
		cfg: cfg,
	}
}

// Harvest executes the fetching of documents from Google Drive.
func (h *GDriveHarvester) Harvest(ctx context.Context) (*HarvestStats, error) {
	if len(h.cfg.Harvester.GoogleDrive) == 0 {
		return &HarvestStats{Platform: "gdrive"}, nil
	}
	start := time.Now()

	// Uses Application Default Credentials (ADC)
	// Requires: gcloud auth application-default login
	srv, err := drive.NewService(ctx, option.WithScopes(drive.DriveReadonlyScope))
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve Drive client: %v", err)
	}

	cacheFile := filepath.Join(h.cfg.VaultPath, ".agents", "cache", "gdrive_harvest_cache.json")
	cache := loadCache(cacheFile)

	stats := &HarvestStats{Platform: "gdrive"}
	for _, gCfg := range h.cfg.Harvester.GoogleDrive {
		stats.OutputDir = gCfg.TargetDir
		err := h.syncFolder(srv, gCfg, cache, stats)
		if err != nil {
			return nil, fmt.Errorf("failed to sync folder %s: %v", gCfg.FolderID, err)
		}
	}

	saveCache(cacheFile, cache)
	stats.DurationMs = time.Since(start).Milliseconds()
	return stats, nil
}

func (h *GDriveHarvester) syncFolder(srv *drive.Service, gCfg config.GDriveConfig, cache map[string]string, stats *HarvestStats) error {
	if (gCfg.FolderID == "" && gCfg.Query == "") || gCfg.TargetDir == "" {
		return nil
	}
	stats.TotalProjects++
	targetDir := gCfg.TargetDir
	if !filepath.IsAbs(targetDir) {
		targetDir = filepath.Join(h.cfg.VaultPath, targetDir)
	}

	// Create target directory if it doesn't exist
	if err := os.MkdirAll(targetDir, 0750); err != nil {
		return err
	}

	// Query files within the folder or using custom query.
	var query string
	if gCfg.Query != "" {
		query = gCfg.Query
	} else {
		query = fmt.Sprintf("'%s' in parents and trashed = false", gCfg.FolderID)
	}

	r, err := srv.Files.List().Q(query).Fields("files(id, name, mimeType, modifiedTime)").Do()
	if err == nil {
		stats.TotalDocsFound += len(r.Files)
	}
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	tasks := make(chan *drive.File, 10)
	var mu sync.Mutex

	workerCount := 5
	for w := 0; w < workerCount; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for file := range tasks {
				var err error
				if file.MimeType == "application/vnd.google-apps.document" {
					err = h.exportDocToMarkdown(srv, file, targetDir)
				} else {
					err = h.downloadFile(srv, file, targetDir)
				}

				if err != nil {
					fmt.Printf("Warning: failed to download %s: %v\n", file.Name, err)
				} else {
					mu.Lock()
					cache[file.Id] = file.ModifiedTime
					stats.DownloadedCount++
					mu.Unlock()
				}
			}
		}()
	}

	for _, file := range r.Files {
		mu.Lock()
		cachedTime := cache[file.Id]
		mu.Unlock()

		if cachedTime != "" && cachedTime == file.ModifiedTime {
			// File hasn't changed
			stats.SkippedCount++
			continue
		}

		tasks <- file
	}
	close(tasks)
	wg.Wait()

	return nil
}

func (h *GDriveHarvester) exportDocToMarkdown(srv *drive.Service, file *drive.File, targetDir string) error {
	targetPath := filepath.Join(targetDir, file.Name+".md")

	// Export natively to Markdown via Google Drive API v3
	resp, err := srv.Files.Export(file.Id, "text/markdown").Download()
	if err != nil {
		// Fallback to text/plain if markdown is not enabled in the Google Workspace domain
		resp, err = srv.Files.Export(file.Id, "text/plain").Download()
		if err != nil {
			return err
		}
	}
	defer resp.Body.Close() //nolint:errcheck,gosec // fully read or safe to ignore

	out, err := os.Create(targetPath) //nolint:gosec // intentional
	if err != nil {
		return err
	}
	defer out.Close() //nolint:errcheck,gosec // fully read or safe to ignore

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	fmt.Printf("Harvested Doc -> Markdown: %s\n", targetPath)
	return nil
}

func (h *GDriveHarvester) downloadFile(srv *drive.Service, file *drive.File, targetDir string) error {
	targetPath := filepath.Join(targetDir, file.Name)

	resp, err := srv.Files.Get(file.Id).Download()
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck,gosec // fully read or safe to ignore

	out, err := os.Create(targetPath) //nolint:gosec // intentional
	if err != nil {
		return err
	}
	defer out.Close() //nolint:errcheck,gosec // fully read or safe to ignore

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	fmt.Printf("Harvested File: %s\n", targetPath)
	return nil
}
