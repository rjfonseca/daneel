package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/rjfonseca/daneel/internal/routine"
	"github.com/rjfonseca/daneel/internal/telemetry"
)

var (
	routineInstallAll   bool
	routineUninstallAll bool
	routineLogLines     int
	routineLogFollow    bool
)

var routineCmd = &cobra.Command{
	Use:   "routine",
	Short: "Manage and inspect scheduled background maintenance routines",
}

var routineListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured and active scheduled routines",
	RunE: func(cmd *cobra.Command, args []string) error {
		adapter := routine.NewSystemdAdapter("", "", nil)
		rCfg := cfg.GetRoutinesConfig()

		var statuses []routine.RoutineStatus
		for name, item := range rCfg.Items {
			st, err := adapter.Status(cmd.Context(), name)
			if err != nil || st == nil {
				statuses = append(statuses, routine.RoutineStatus{
					Name:        name,
					Description: item.Description,
					Schedule:    item.Schedule,
					Enabled:     item.Enabled,
					Active:      false,
					LastResult:  "not installed",
				})
			} else {
				statuses = append(statuses, *st)
			}
		}

		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(statuses)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		_, _ = fmt.Fprintln(w, "NAME\tSCHEDULE\tENABLED\tACTIVE\tLAST TRIGGER\tNEXT TRIGGER\tLAST RESULT")
		for _, s := range statuses {
			enabledStr := "no"
			if s.Enabled {
				enabledStr = "yes"
			}
			activeStr := "no"
			if s.Active {
				activeStr = "yes"
			}
			lastTrig := "-"
			if !s.LastTrigger.IsZero() {
				lastTrig = s.LastTrigger.Format("2006-01-02 15:04:05")
			}
			nextTrig := "-"
			if !s.NextTrigger.IsZero() {
				nextTrig = s.NextTrigger.Format("2006-01-02 15:04:05")
			}
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				s.Name, s.Schedule, enabledStr, activeStr, lastTrig, nextTrig, s.LastResult)
		}
		return w.Flush()
	},
}

var routineInstallCmd = &cobra.Command{
	Use:   "install [name]",
	Short: "Install systemd user timer and service for routine(s)",
	RunE: func(cmd *cobra.Command, args []string) error {
		adapter := routine.NewSystemdAdapter("", "", nil)
		rCfg := cfg.GetRoutinesConfig()

		if len(args) == 0 && !routineInstallAll {
			routineInstallAll = true
		}

		if len(args) > 0 {
			name := args[0]
			item, ok := rCfg.Items[name]
			if !ok {
				return fmt.Errorf("routine %q not found in configuration", name)
			}
			r := routine.Routine{
				Name:        name,
				Command:     item.Command,
				Args:        item.Args,
				Schedule:    item.Schedule,
				Description: item.Description,
				Enabled:     item.Enabled,
			}
			if err := adapter.Install(cmd.Context(), r); err != nil {
				return fmt.Errorf("failed to install routine %s: %w", name, err)
			}
			fmt.Printf("✅ Routine %s timer installed successfully (%s).\n", name, r.Schedule)
			return nil
		}

		count := 0
		for name, item := range rCfg.Items {
			r := routine.Routine{
				Name:        name,
				Command:     item.Command,
				Args:        item.Args,
				Schedule:    item.Schedule,
				Description: item.Description,
				Enabled:     item.Enabled,
			}
			if err := adapter.Install(cmd.Context(), r); err != nil {
				fmt.Printf("⚠️ Failed to install routine %s: %v\n", name, err)
			} else {
				fmt.Printf("✅ Routine %s timer installed (%s)\n", name, r.Schedule)
				count++
			}
		}
		fmt.Printf("Installed %d routine timer(s).\n", count)
		return nil
	},
}

var routineUninstallCmd = &cobra.Command{
	Use:   "uninstall [name]",
	Short: "Uninstall systemd timer and service for routine(s)",
	RunE: func(cmd *cobra.Command, args []string) error {
		adapter := routine.NewSystemdAdapter("", "", nil)
		rCfg := cfg.GetRoutinesConfig()

		if len(args) > 0 {
			name := args[0]
			if err := adapter.Uninstall(cmd.Context(), name); err != nil {
				return fmt.Errorf("failed to uninstall routine %s: %w", name, err)
			}
			fmt.Printf("🗑️ Routine %s uninstalled.\n", name)
			return nil
		}

		if routineUninstallAll || len(args) == 0 {
			for name := range rCfg.Items {
				_ = adapter.Uninstall(cmd.Context(), name)
				fmt.Printf("🗑️ Routine %s uninstalled.\n", name)
			}
		}
		return nil
	},
}

var routineEnableCmd = &cobra.Command{
	Use:   "enable <name>",
	Short: "Enable and start a routine timer",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		adapter := routine.NewSystemdAdapter("", "", nil)
		name := args[0]
		if err := adapter.Enable(cmd.Context(), name); err != nil {
			return fmt.Errorf("failed to enable routine %s: %w", name, err)
		}
		fmt.Printf("✅ Routine %s enabled.\n", name)
		return nil
	},
}

var routineDisableCmd = &cobra.Command{
	Use:   "disable <name>",
	Short: "Disable and stop a routine timer",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		adapter := routine.NewSystemdAdapter("", "", nil)
		name := args[0]
		if err := adapter.Disable(cmd.Context(), name); err != nil {
			return fmt.Errorf("failed to disable routine %s: %w", name, err)
		}
		fmt.Printf("⏸️ Routine %s disabled.\n", name)
		return nil
	},
}

var routineRunCmd = &cobra.Command{
	Use:   "run <name>",
	Short: "Trigger a routine execution immediately via systemd",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		adapter := routine.NewSystemdAdapter("", "", nil)
		name := args[0]
		fmt.Printf("🚀 Starting routine %s...\n", name)
		if err := adapter.RunNow(cmd.Context(), name); err != nil {
			return fmt.Errorf("failed to run routine %s: %w", name, err)
		}
		fmt.Printf("✅ Routine %s triggered.\n", name)
		return nil
	},
}

var routineLogsCmd = &cobra.Command{
	Use:   "logs <name>",
	Short: "View systemd journal logs for a routine",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		adapter := routine.NewSystemdAdapter("", "", nil)
		name := args[0]
		return adapter.Logs(cmd.Context(), name, routineLogLines, routineLogFollow)
	},
}

var routineStatusCmd = &cobra.Command{
	Use:   "status [name]",
	Short: "Inspect technical details and state of a routine",
	RunE: func(cmd *cobra.Command, args []string) error {
		adapter := routine.NewSystemdAdapter("", "", nil)

		names := []string{}
		if len(args) > 0 {
			names = append(names, args[0])
		} else {
			for name := range cfg.GetRoutinesConfig().Items {
				names = append(names, name)
			}
		}

		for _, name := range names {
			st, err := adapter.Status(cmd.Context(), name)
			if err != nil {
				fmt.Printf("Routine %q: %v\n", name, err)
				continue
			}

			if jsonOut {
				b, _ := json.MarshalIndent(st, "", "  ")
				fmt.Println(string(b))
				continue
			}

			fmt.Printf("Routine: %s\n", st.Name)
			fmt.Printf("  Description:  %s\n", st.Description)
			fmt.Printf("  Schedule:     %s\n", st.Schedule)
			fmt.Printf("  Enabled:      %v\n", st.Enabled)
			fmt.Printf("  Active:       %v\n", st.Active)
			if !st.LastTrigger.IsZero() {
				fmt.Printf("  Last Trigger: %s\n", st.LastTrigger.Format(time.RFC3339))
			}
			if !st.NextTrigger.IsZero() {
				fmt.Printf("  Next Trigger: %s\n", st.NextTrigger.Format(time.RFC3339))
			}
			fmt.Printf("  Last Result:  %s\n", st.LastResult)
			fmt.Println()
		}
		return nil
	},
}

var routineTriggerSource string

var routineExecutor = func(ctx context.Context, cmdName string, cmdArgs []string, stdout, stderr io.Writer) (int, error) {
	exe, err := os.Executable()
	if err != nil {
		exe = "daneel"
	}
	execArgs := append([]string{cmdName}, cmdArgs...)
	childCmd := exec.CommandContext(ctx, exe, execArgs...)
	childCmd.Stdout = stdout
	childCmd.Stderr = stderr
	runErr := childCmd.Run()
	exitCode := 0
	if runErr != nil {
		exitCode = 1
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
	}
	return exitCode, runErr
}

var routineExecCmd = &cobra.Command{
	Use:   "exec <name>",
	Short: "Execute a routine with automated duration measurement and execution history logging",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := strings.TrimSpace(args[0])

		var cmdName string
		var cmdArgs []string
		var desc string

		if cfg != nil {
			rCfg := cfg.GetRoutinesConfig()
			if item, ok := rCfg.Items[name]; ok {
				cmdName = item.Command
				cmdArgs = item.Args
				desc = item.Description
			}
		}

		if cmdName == "" {
			// Builtins fallback
			switch name {
			case "dream", "rem", "sleep":
				cmdName = "dream"
				cmdArgs = []string{"--no-agent"}
			case "tidy", "hygiene":
				cmdName = "tidy"
			case "doctor":
				cmdName = "doctor"
			case "workspace":
				cmdName = "workspace"
			case "guard":
				cmdName = "guard"
			case "dossier":
				cmdName = "dossier"
			case "ingest":
				cmdName = "ingest"
			case "index", "reindex":
				cmdName = "index"
			case "session_mining", "sessions":
				cmdName = "audit"
				cmdArgs = []string{"sessions"}
			case "audit_agents", "evals":
				cmdName = "audit"
				cmdArgs = []string{"agents"}
			case "log", "time":
				cmdName = "log"
				cmdArgs = []string{"--recent=7"}
			case "harvest":
				cmdName = "harvest"
			default:
				if strings.HasPrefix(name, "harvest:") {
					platform := strings.TrimPrefix(name, "harvest:")
					cmdName = "harvest"
					cmdArgs = []string{platform}
				} else {
					cmdName = name
				}
			}
		}

		startedAt := time.Now()
		startUnix := startedAt.Unix()

		var combinedBuf bytes.Buffer
		outW := io.MultiWriter(os.Stdout, &combinedBuf)
		errW := io.MultiWriter(os.Stderr, &combinedBuf)

		exitCode, runErr := routineExecutor(cmd.Context(), cmdName, cmdArgs, outW, errW)
		completedAt := time.Now()
		durationMs := completedAt.Sub(startedAt).Milliseconds()

		status := "success"
		var errorMsg string

		if exitCode != 0 || runErr != nil {
			status = "failed"
			if runErr != nil {
				errorMsg = runErr.Error()
			} else {
				errorMsg = fmt.Sprintf("routine exited with status %d", exitCode)
			}
		}

		summary := fmt.Sprintf("Routine '%s' completed with exit code %d in %dms", name, exitCode, durationMs)
		if desc != "" {
			summary = fmt.Sprintf("[%s] %s", desc, summary)
		}

		itemsProcessed := 0
		reItems := regexp.MustCompile(`(?i)(\d+)\s+(?:files|notes|repositories|repos|links|sessions|turns|items)`)
		if matches := reItems.FindStringSubmatch(combinedBuf.String()); len(matches) > 1 {
			if n, err := strconv.Atoi(matches[1]); err == nil {
				itemsProcessed = n
			}
		}

		vaultDir := ""
		if cfg != nil {
			vaultDir = cfg.VaultPath
		}

		store, storeErr := routine.OpenDefaultStore(vaultDir)
		if storeErr == nil && store != nil {
			defer func() { _ = store.Close() }()

			runRecord := routine.RoutineRun{
				RoutineName:    name,
				StartedAt:      startUnix,
				CompletedAt:    completedAt.Unix(),
				DurationMs:     durationMs,
				TriggerSource:  routineTriggerSource,
				ExitCode:       exitCode,
				Status:         status,
				Summary:        summary,
				ErrorMessage:   errorMsg,
				ItemsProcessed: itemsProcessed,
			}

			parserOpts := telemetry.ParserOptions{
				Since:  24 * time.Hour,
				Config: cfg,
			}
			if vaultDir != "" {
				brainDir := filepath.Join(vaultDir, "brain")
				if fi, err := os.Stat(brainDir); err == nil && fi.IsDir() {
					parserOpts.BrainDir = brainDir
				}
			}

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			if summaries, err := telemetry.SyncSessionsToStore(ctx, store, parserOpts); err == nil {
				if matched := telemetry.FindSessionInWindow(summaries, startedAt, completedAt, name); matched != nil {
					runRecord.InputTokens = matched.TotalInputTokens
					runRecord.CacheReadTokens = matched.TotalCacheTokens
					runRecord.OutputTokens = matched.TotalOutputTokens
					runRecord.EstimatedCostUSD = matched.TotalCostUSD
					runRecord.Model = matched.Record.Model
					runRecord.SessionID = matched.Record.SessionID
				}
			}

			_ = store.RecordRun(ctx, runRecord)
		}

		if exitCode != 0 {
			return fmt.Errorf("routine %s failed with exit code %d: %v", name, exitCode, runErr)
		}
		return nil
	},
}

func init() {
	routineInstallCmd.Flags().BoolVar(&routineInstallAll, "all", false, "Install all configured routines")
	routineUninstallCmd.Flags().BoolVar(&routineUninstallAll, "all", false, "Uninstall all configured routines")

	routineLogsCmd.Flags().IntVarP(&routineLogLines, "lines", "n", 50, "Number of journal lines to display")
	routineLogsCmd.Flags().BoolVarP(&routineLogFollow, "follow", "f", false, "Follow journal output")

	routineExecCmd.Flags().StringVar(&routineTriggerSource, "trigger", "cli", "Execution trigger source (systemd, cron, cli, web_cockpit, mcp)")

	routineCmd.AddCommand(routineListCmd)
	routineCmd.AddCommand(routineInstallCmd)
	routineCmd.AddCommand(routineUninstallCmd)
	routineCmd.AddCommand(routineEnableCmd)
	routineCmd.AddCommand(routineDisableCmd)
	routineCmd.AddCommand(routineRunCmd)
	routineCmd.AddCommand(routineLogsCmd)
	routineCmd.AddCommand(routineStatusCmd)
	routineCmd.AddCommand(routineExecCmd)

	RootCmd.AddCommand(routineCmd)
}
