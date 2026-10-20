//nolint:revive // package comment not required
package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/rjfonseca/daneel/internal/daemon"
	"github.com/rjfonseca/daneel/internal/spooler"
)

var (
	hookAgent          string
	hookSession        string
	hookCWD            string
	hookGitRoot        string
	hookCompileAll     bool
	hookCompileSession string
)

const hookIPCBudget = 200 * time.Millisecond
const hookCompilationBudget = 500 * time.Millisecond

var hookCmd = &cobra.Command{
	Use:   "hook",
	Short: "Manage passive agent lifecycle hooks and ambient context capture",
	Long: `Captures passive agent lifecycle events (tool use, session start/end, edits)
from AI agent runners (Claude Code, Antigravity CLI, Cursor) and spools them locally
for context synthesis, ambient HUD awareness, and Dream Cycle triage.`,
}

var hookCaptureCmd = &cobra.Command{
	Use:   "capture [event]",
	Short: "Ingest a raw agent lifecycle hook event from stdin",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		event := "HookEvent"
		if len(args) > 0 {
			event = args[0]
		}
		sid, err := executeHookCapture(event, os.Stdin, hookAgent, hookSession, hookCWD, hookGitRoot)
		if err != nil {
			return err
		}
		if verbose {
			fmt.Printf("Captured event %q for session %q\n", event, sid)
		}
		return nil
	},
}

var hookHandoffCmd = &cobra.Command{
	Use:   "handoff",
	Short: "Emit cross-agent continuity handoff context for new sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd := hookCWD
		if cwd == "" {
			cwd, _ = os.Getwd()
		}
		handoff, err := executeHookHandoff(cwd)
		if err != nil {
			return err
		}
		if handoff != "" {
			fmt.Print(handoff)
		}
		return nil
	},
}

var hookStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Display spool buffer metrics and active agent sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := executeHookStatus()
		if err != nil {
			return err
		}
		fmt.Print(out)
		return nil
	},
}

var hookCompileCmd = &cobra.Command{
	Use:   "compile [session-id]",
	Short: "Synthesize raw spool session logs into vault inbox notes",
	RunE: func(cmd *cobra.Command, args []string) error {
		vault := resolveVaultRoot()
		sid := hookCompileSession
		if len(args) > 0 && sid == "" {
			sid = args[0]
		}
		paths, err := executeHookCompile(sid, hookCompileAll, vault)
		if err != nil {
			return err
		}
		if len(paths) == 0 {
			fmt.Println("No pending sessions to compile.")
			return nil
		}
		fmt.Printf("Compiled %d session(s) into vault inbox:\n", len(paths))
		for _, p := range paths {
			fmt.Printf("  • %s\n", p)
		}
		return nil
	},
}

func init() {
	hookCaptureCmd.Flags().StringVar(&hookAgent, "agent", "", "Agent identifier (e.g., claude-code, antigravity)")
	hookCaptureCmd.Flags().StringVar(&hookSession, "session", "", "Explicit session ID")
	hookCaptureCmd.Flags().StringVar(&hookCWD, "cwd", "", "Working directory of the session")
	hookCaptureCmd.Flags().StringVar(&hookGitRoot, "git-root", "", "Git repository root directory")

	hookHandoffCmd.Flags().StringVar(&hookCWD, "cwd", "", "Working directory to resolve context for")

	hookCompileCmd.Flags().BoolVarP(&hookCompileAll, "all", "a", false, "Compile all pending sessions in the spool")
	hookCompileCmd.Flags().StringVarP(&hookCompileSession, "session", "s", "", "Specific session ID to compile")

	hookCmd.AddCommand(hookCaptureCmd)
	hookCmd.AddCommand(hookHandoffCmd)
	hookCmd.AddCommand(hookStatusCmd)
	hookCmd.AddCommand(hookCompileCmd)

	RootCmd.AddCommand(hookCmd)
}

// executeHookCapture orchestrates sending the hook payload to the daemon,
// automatically falling back to direct disk spooling when the daemon is offline.
func executeHookCapture(event string, r io.Reader, agent, session, cwd, gitRoot string) (string, error) {
	var payload []byte
	if r != nil {
		// Limit to 10MB to protect memory
		data, err := io.ReadAll(io.LimitReader(r, 10*1024*1024))
		if err == nil && len(data) > 0 {
			payload = data
		}
	}

	var sid string
	var err error

	// 1. Try sending via daemon IPC within one shared hook budget.
	ctx, cancel := context.WithTimeout(context.Background(), hookIPCBudget)
	client, clientErr := daemon.NewClientContext(ctx)
	if clientErr == nil {
		sid, err = client.CaptureHookEventContext(ctx, daemon.HookEventPayload{
			SessionID: session,
			Event:     event,
			Agent:     agent,
			CWD:       cwd,
			GitRoot:   gitRoot,
			Payload:   string(payload),
		})
		_ = client.Close()
	}
	cancel()

	// 2. Offline fallback: write directly to spool on disk
	if sid == "" || err != nil {
		sp := spooler.New("")
		sid, err = sp.AppendEvent(spooler.Event{
			SessionID: session,
			Event:     event,
			Agent:     agent,
			CWD:       cwd,
			GitRoot:   gitRoot,
			Raw:       payload,
			Timestamp: time.Now(),
		})
		if err != nil {
			return "", err
		}
	}

	// 3. Auto-compile on SessionEnd
	if strings.EqualFold(event, "SessionEnd") && sid != "" {
		vault := resolveVaultRoot()
		c := spooler.NewCompiler(vault, nil)
		ctx, cancel := context.WithTimeout(context.Background(), hookCompilationBudget)
		_, compileErr := c.CompileSessionContext(ctx, sid)
		cancel()
		if compileErr != nil {
			fmt.Fprintf(os.Stderr, "Session captured; compilation deferred. Run daneel hook compile --session %s to retry.\n", sid)
		}
	}

	return sid, nil
}

// executeHookCompile compiles a session or all pending sessions into markdown.
func executeHookCompile(sessionID string, compileAll bool, vaultRoot string) ([]string, error) {
	if vaultRoot == "" {
		vaultRoot = resolveVaultRoot()
	}
	c := spooler.NewCompiler(vaultRoot, nil)
	if compileAll || sessionID == "" {
		return c.CompileAll()
	}
	mdPath, err := c.CompileSession(sessionID)
	if err != nil {
		return nil, err
	}
	return []string{mdPath}, nil
}

func resolveVaultRoot() string {
	if vaultPath != "" {
		return vaultPath
	}
	if cfg != nil && cfg.VaultPath != "" {
		return cfg.VaultPath
	}
	cwd, err := os.Getwd()
	if err == nil {
		return cwd
	}
	return "."
}

// executeHookHandoff queries handoff context from the daemon or scans recent inbox/sessions.
func executeHookHandoff(cwd string) (string, error) {
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	ctx, cancel := context.WithTimeout(context.Background(), hookIPCBudget)
	client, err := daemon.NewClientContext(ctx)
	if err == nil {
		handoff, hErr := client.GetHookHandoffContext(ctx, cwd)
		_ = client.Close()
		if hErr == nil && handoff != "" {
			cancel()
			return handoff, nil
		}
	}
	cancel()

	vault := resolveVaultRoot()
	sp := spooler.New("")
	return sp.ResolveHandoffContext(vault, cwd)
}

// executeHookStatus gathers metrics on active spool files and daemon state.
func executeHookStatus() (string, error) {
	sp := spooler.New("")
	sessions, err := sp.ListSessions()
	if err != nil {
		return "", err
	}

	daemonState := "🔴 offline (direct spool active)"
	if daemon.IsDaemonRunning() {
		daemonState = "🟢 online (ipc & ambient hub active)"
	}

	var sb strings.Builder
	sb.WriteString("📦 Daneel Hooks & Spool Status\n")
	fmt.Fprintf(&sb, "Daemon Status: %s\n", daemonState)
	fmt.Fprintf(&sb, "Spool Directory: %s\n", sp.BaseDir())
	fmt.Fprintf(&sb, "Active Sessions: %d\n", len(sessions))

	totalEvents := 0
	var totalBytes int64
	for _, s := range sessions {
		totalEvents += s.EventCount
		totalBytes += s.SizeBytes
	}
	fmt.Fprintf(&sb, "Total Events: %d (%d bytes)\n", totalEvents, totalBytes)

	if len(sessions) > 0 {
		sb.WriteString("\nRecent Sessions:\n")
		for _, s := range sessions {
			fmt.Fprintf(&sb, "  • %-32s (%d events, updated %s)\n",
				s.SessionID, s.EventCount, s.LastUpdated.Format("2006-01-02 15:04:05"))
		}
	}

	return sb.String(), nil
}
