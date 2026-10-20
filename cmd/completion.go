//nolint:revive // package comment not required
package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generates shell autocomplete scripts for Daneel",
	Long: `Generates autocomplete scripts for the selected shell.

Usage examples:

  # Bash:
  $ source <(daneel completion bash)
  $ daneel completion bash > /etc/bash_completion.d/daneel

  # Zsh:
  $ daneel completion zsh > "${fpath[1]}/_daneel"

  # Fish:
  $ daneel completion fish | source
  $ daneel completion fish > ~/.config/fish/completions/daneel.fish

  # PowerShell:
  PS> daneel completion powershell | Out-String | Invoke-Expression
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error { //nolint:revive
		switch args[0] {
		case "bash":
			return RootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			return RootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return RootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return RootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(completionCmd)
}
