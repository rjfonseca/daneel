//nolint:revive // package comment not required
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// InboxItem represents an unprocessed file in the inbox directory.
type InboxItem struct {
	Name      string `json:"name"`
	RelPath   string `json:"rel_path"`
	SizeBytes int64  `json:"size_bytes"`
	IsDir     bool   `json:"is_dir"`
}

var inboxCmd = &cobra.Command{
	Use:     "inbox",
	Aliases: []string{"triage"},
	Short:   "Lists and inspects pending items in the inbox/ directory",
	RunE: func(cmd *cobra.Command, args []string) error { //nolint:revive
		inboxDir := filepath.Join(cfg.VaultPath, "inbox")
		if _, err := os.Stat(inboxDir); os.IsNotExist(err) {
			fmt.Println("inbox/ directory not found.")
			return nil
		}

		entries, err := os.ReadDir(inboxDir)
		if err != nil {
			return fmt.Errorf("error reading inbox: %w", err)
		}

		var items []InboxItem
		for _, e := range entries {
			info, err := e.Info()
			if err != nil {
				continue
			}
			relPath := filepath.ToSlash(filepath.Join("inbox", e.Name()))
			items = append(items, InboxItem{
				Name:      e.Name(),
				RelPath:   relPath,
				SizeBytes: info.Size(),
				IsDir:     e.IsDir(),
			})
		}

		if jsonOut {
			bytes, err := json.MarshalIndent(items, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(bytes))
			return nil
		}

		titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
		badgeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

		fmt.Println(titleStyle.Render(fmt.Sprintf("📥 Inbox Items (%d pending):", len(items))))
		if len(items) == 0 {
			fmt.Println("✨ Inbox empty! No notes pending promotion.")
			return nil
		}

		for i, item := range items {
			kind := "📄"
			if item.IsDir {
				kind = "📁"
			}
			fmt.Printf("%2d. %s %s  %s\n",
				i+1,
				kind,
				badgeStyle.Render(item.Name),
				fmt.Sprintf("(%d bytes)", item.SizeBytes),
			)
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(inboxCmd)
}
