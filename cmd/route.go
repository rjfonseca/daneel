//nolint:revive // package comment not required
package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/rjfonseca/daneel/internal/router"
	"github.com/rjfonseca/daneel/internal/toon"
)

var routeCmd = &cobra.Command{
	Use:   "route <prompt>",
	Short: "Evaluates conversational prompt with CPU Fast-Path Semantic Router (< 2ms)",
	Long: `Evaluates a natural language prompt or command using Daneel's sub-millisecond CPU
Semantic Router. High-confidence matches (>= 0.85) are routed deterministically to native
Go routines, bypassing heavy remote LLM processes.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		input := strings.Join(args, " ")
		r := router.New()
		match := r.Evaluate(input)

		if toonOut {
			out, err := toon.MarshalString(match)
			if err != nil {
				return err
			}
			fmt.Println(out)
			return nil
		}

		if jsonOut {
			data, err := json.MarshalIndent(match, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}

		headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
		passStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))
		warnStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))

		fmt.Println(headerStyle.Render("🚦 CPU Fast-Path Semantic Route Evaluation"))
		fmt.Printf("• Query:       %s\n", input)
		fmt.Printf("• Intent:      %s\n", match.Intent)
		fmt.Printf("• Confidence:  %.2f\n", match.Confidence)

		if len(match.Params) > 0 {
			fmt.Print("• Parameters:  ")
			for k, v := range match.Params {
				fmt.Printf("%s=%s ", k, v)
			}
			fmt.Println()
		}

		if match.Confidence >= 0.85 {
			fmt.Println(passStyle.Render("⚡ Fast-Path: Eligible for instant local execution (bypasses LLM)"))
		} else {
			fmt.Println(warnStyle.Render("🤖 Slow-Path: Ambiguous/complex prompt; delegates to interactive LLM agent"))
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(routeCmd)
}
