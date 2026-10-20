//nolint:revive // package comment not required
package vault

import (
	"os"
	"path/filepath"
	"strings"
)

// ScanVault traverses the vault directory and parses all markdown files in the specified indexed directories.
func ScanVault(vaultRoot string, indexedDirs []string) ([]*Note, error) {
	var notes []*Note

	for _, dir := range indexedDirs {
		absDir := filepath.Join(vaultRoot, dir)
		if _, err := os.Stat(absDir); os.IsNotExist(err) {
			continue
		}

		err := filepath.Walk(absDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Skip ignored directories
			if info.IsDir() {
				base := info.Name()
				if base == ".git" || base == ".dev" || base == "node_modules" || base == ".obsidian" || base == ".agents" {
					return filepath.SkipDir
				}
				return nil
			}

			if !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
				return nil
			}

			note, err := ParseNote(path, vaultRoot)
			if err == nil && note != nil {
				notes = append(notes, note)
			}
			return nil
		})

		if err != nil {
			return nil, err
		}
	}

	return notes, nil
}

type VaultMetrics struct { //nolint:revive
	TotalNotes   int
	TotalLinks   int
	OrphanNotes  int
	LinksPerNote float64
}

func CalculateVaultMetrics(vaultRoot string) VaultMetrics { //nolint:revive
	// Re-use indexed dirs logic.
	indexedDirs := []string{"notes", "daily", "meta", "inbox"}
	notes, err := ScanVault(vaultRoot, indexedDirs)
	if err != nil || len(notes) == 0 {
		return VaultMetrics{}
	}

	totalLinks := 0
	incomingLinks := make(map[string]int)

	for _, n := range notes {
		totalLinks += len(n.Links)
		for _, l := range n.Links {
			// Basic match by basename or exact path
			target := l.TargetRelPath
			if l.IsWikiLink {
				target = strings.ToLower(strings.ReplaceAll(target, " ", "-")) + ".md"
			} else {
				target = filepath.Base(target)
			}
			incomingLinks[target]++
		}
	}

	orphanNotes := 0
	for _, n := range notes {
		base := filepath.Base(n.RelPath)
		if incomingLinks[base] == 0 && len(n.Links) == 0 && !strings.HasPrefix(n.RelPath, "daily/") && !strings.HasPrefix(n.RelPath, "inbox/") {
			orphanNotes++
		}
	}

	return VaultMetrics{
		TotalNotes:   len(notes),
		TotalLinks:   totalLinks,
		OrphanNotes:  orphanNotes,
		LinksPerNote: float64(totalLinks) / float64(len(notes)),
	}
}
