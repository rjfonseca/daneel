//nolint:revive // package comment not required
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

const (
	llmModeNone     = "none"
	llmModeOptional = "optional"
	llmModeUses     = "uses"

	llmAnnotationKey     = "llm"
	llmNoteAnnotationKey = "llm_note"
)

type llmAnnotationSpec struct {
	cmd  *cobra.Command
	mode string
	note string
}

const llmUsageTemplate = `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if llmDetail .}}

LLM Consumption:
  {{llmDetail .}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Available Commands:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{llmBadge .}}{{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{llmBadge .}}{{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

Additional Commands:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{llmBadge .}}{{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`

func setLLMMode(cmd *cobra.Command, mode, note string) {
	if cmd == nil {
		return
	}
	if cmd.Annotations == nil {
		cmd.Annotations = make(map[string]string)
	}
	cmd.Annotations[llmAnnotationKey] = mode
	if note != "" {
		cmd.Annotations[llmNoteAnnotationKey] = note
	}
}

func llmBadge(cmd *cobra.Command) string {
	if cmd == nil {
		return ""
	}
	switch cmd.Annotations[llmAnnotationKey] {
	case llmModeNone:
		return fmt.Sprintf("%-15s", "[No LLM]")
	case llmModeOptional:
		return fmt.Sprintf("%-15s", "[Optional LLM]")
	case llmModeUses:
		return fmt.Sprintf("%-15s", "[Uses LLM]")
	default:
		return ""
	}
}

func llmDetail(cmd *cobra.Command) string {
	if cmd == nil {
		return ""
	}
	mode := cmd.Annotations[llmAnnotationKey]
	if mode == "" {
		return ""
	}
	note := cmd.Annotations[llmNoteAnnotationKey]
	switch mode {
	case llmModeNone:
		if note != "" {
			return "No LLM — " + note
		}
		return "No LLM (100% deterministic local Go execution)"
	case llmModeOptional:
		if note != "" {
			return "Optional LLM — " + note
		}
		return "Optional LLM (invokes external AI agent only when enabled via flags or UI)"
	case llmModeUses:
		if note != "" {
			return "Uses LLM — " + note
		}
		return "Uses LLM (delegates to the configured external AI agent)"
	default:
		return ""
	}
}

func init() {
	specs := []llmAnnotationSpec{
		// Uses LLM
		{chatCmd, llmModeUses, "Launches the configured external AI agent (deterministic queries may be intercepted by the CPU Fast-Path Router)"},

		// Optional LLM
		{dreamCmd, llmModeOptional, "Invokes the configured AI agent by default; pass --no-agent, --stats, --metacognition, or --prompt for No LLM execution"},
		{cloneCmd, llmModeOptional, "Pure Go by default; pass --dream to run the Dream Cycle with an AI agent"},
		{serveCmd, llmModeOptional, "Pure Go server by default; invokes an AI agent only when triggered from the Web Cockpit Action Deck"},
		{routineCmd, llmModeOptional, "Pure Go wrapper by default; invokes an AI agent only when executing a routine that uses one (e.g. dream without --no-agent)"},

		// No LLM
		{auditCmd, llmModeNone, ""},
		{backlinksCmd, llmModeNone, ""},
		{completionCmd, llmModeNone, ""},
		{configCmd, llmModeNone, ""},
		{daemonCmd, llmModeNone, ""},
		{doctorCmd, llmModeNone, ""},
		{dossierCmd, llmModeNone, ""},
		{eveningCmd, llmModeNone, ""},
		{guardCmd, llmModeNone, ""},
		{harvestCmd, llmModeNone, ""},
		{hookCmd, llmModeNone, ""},
		{inboxCmd, llmModeNone, ""},
		{indexCmd, llmModeNone, ""},
		{ingestCmd, llmModeNone, ""},
		{initCmd, llmModeNone, ""},
		{logCmd, llmModeNone, ""},
		{mcpCmd, llmModeNone, "Pure Go stdio MCP server; exposes tools consumed by an external AI agent"},
		{morningCmd, llmModeNone, ""},
		{peopleCmd, llmModeNone, ""},
		{prepCmd, llmModeNone, ""},
		{projectsCmd, llmModeNone, ""},
		{readCmd, llmModeNone, ""},
		{resolveCmd, llmModeNone, ""},
		{routeCmd, llmModeNone, ""},
		{searchCmd, llmModeNone, ""},
		{telemetryCmd, llmModeNone, ""},
		{tidyCmd, llmModeNone, ""},
		{tuiCmd, llmModeNone, ""},
		{versionCmd, llmModeNone, ""},
		{workspaceCmd, llmModeNone, ""},
	}

	for _, spec := range specs {
		setLLMMode(spec.cmd, spec.mode, spec.note)
	}

	cobra.AddTemplateFunc("llmBadge", llmBadge)
	cobra.AddTemplateFunc("llmDetail", llmDetail)
	RootCmd.SetUsageTemplate(llmUsageTemplate)
}
