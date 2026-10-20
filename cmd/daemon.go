package cmd

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/rjfonseca/daneel/internal/daemon"
	"github.com/rjfonseca/daneel/internal/routine"
	"github.com/rjfonseca/daneel/internal/search"
	"github.com/rjfonseca/daneel/internal/spooler"
)

var (
	daemonRunServe        bool
	daemonRunPort         int
	serviceWithServe      bool
	servicePort           int
	serviceWithRoutines   bool
	uninstallWithRoutines bool
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the Daneel background daemon",
}

var daemonRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the daemon socket server in the foreground",
	RunE: func(cmd *cobra.Command, args []string) error {
		hub := daemon.GetDefaultHub()
		server := daemon.NewServer(hub)
		server.SetSpooler(spooler.New(""))
		if cfg != nil {
			server.SetVaultDir(cfg.VaultPath)
		}

		var httpServer *http.Server
		if daemonRunServe {
			if cfg == nil {
				return fmt.Errorf("configuration not loaded")
			}
			engine, err := search.Open(cfg.DBPath, cfg.VaultPath)
			if err != nil {
				return fmt.Errorf("failed to open search database: %w", err)
			}
			defer func() { _ = engine.Close() }()

			// Incrementally index vault on startup
			_, _, _ = engine.IndexVault(cmd.Context(), cfg.GetIndexedDirs(), false)

			httpServer, err = NewGalaxyHTTPServer(cfg, engine, daemonRunPort, nil)
			if err != nil {
				return fmt.Errorf("failed to initialize HTTP server: %w", err)
			}
			server.SetHTTPServer(httpServer)
		}

		pid := os.Getpid()
		if err := os.MkdirAll(filepath.Dir(daemon.PIDPath()), 0700); err != nil {
			return err
		}
		if err := os.WriteFile(daemon.PIDPath(), []byte(strconv.Itoa(pid)), 0600); err != nil {
			return err
		}
		defer func() { _ = os.Remove(daemon.PIDPath()) }()

		errCh := make(chan error, 2)
		go func() {
			fmt.Printf("Daemon running on %s\n", daemon.SocketPath())
			errCh <- server.ListenAndServe()
		}()

		if daemonRunServe && httpServer != nil {
			go func() {
				fmt.Printf("🚀 Galaxy Visualizer running at http://localhost:%d\n", daemonRunPort)
				if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					errCh <- fmt.Errorf("HTTP server error: %w", err)
				}
			}()
		}

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		select {
		case err := <-errCh:
			return err
		case <-sigCh:
			fmt.Println("Shutting down daemon...")
			server.Stop()
			return nil
		}
	},
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Launch the daemon detached in the background",
	RunE: func(cmd *cobra.Command, args []string) error {
		if daemon.IsDaemonRunning() {
			fmt.Println("Daemon is already running.")
			return nil
		}

		exe, err := os.Executable()
		if err != nil {
			return err
		}

		c := exec.Command(exe, "daemon", "run")
		detachProcess(c)

		if err := c.Start(); err != nil {
			return err
		}

		fmt.Printf("Daemon started with PID %d\n", c.Process.Pid)
		return nil
	},
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the background daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		b, err := os.ReadFile(daemon.PIDPath())
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("Daemon is not running (no PID file).")
				return nil
			}
			return err
		}

		pid, err := strconv.Atoi(string(b))
		if err != nil {
			return err
		}

		process, err := os.FindProcess(pid)
		if err != nil {
			return err
		}

		if err := process.Signal(syscall.SIGTERM); err != nil {
			return err
		}

		fmt.Println("Daemon stopped.")
		return nil
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Report daemon health and status",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !daemon.IsDaemonRunning() {
			fmt.Println("Daemon is offline.")
			return nil
		}

		client, err := daemon.NewClient()
		if err != nil {
			return fmt.Errorf("daemon is running but could not connect: %w", err)
		}
		defer func() { _ = client.Close() }()

		ctx, err := client.GetActiveContext()
		if err != nil {
			return err
		}

		b, err := os.ReadFile(daemon.PIDPath())
		pid := "unknown"
		if err == nil {
			pid = string(b)
		}

		fmt.Printf("Status: Online\n")
		fmt.Printf("PID: %s\n", pid)
		fmt.Printf("Socket: %s\n", daemon.SocketPath())
		fmt.Printf("Active Surface: %s\n", ctx.Surface)
		if !ctx.UpdatedAt.IsZero() {
			fmt.Printf("Last Active: %s\n", ctx.UpdatedAt.Format(time.RFC3339))
		}
		return nil
	},
}

var daemonServiceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage systemd --user service for the daemon",
}

var daemonServiceInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install and start systemd --user unit",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := daemon.InstallUserService(serviceWithServe, servicePort); err != nil {
			return err
		}
		fmt.Println("Daemon service installed and started.")

		if serviceWithRoutines {
			adapter := routine.NewSystemdAdapter("", "", nil)
			rCfg := cfg.GetRoutinesConfig()
			installedCount := 0
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
					installedCount++
				}
			}
			fmt.Printf("Installed %d routine timer(s).\n", installedCount)
		}
		return nil
	},
}

var daemonServiceUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Stop and remove systemd --user unit",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := daemon.UninstallUserService(); err != nil {
			return err
		}
		fmt.Println("Daemon service stopped and uninstalled.")

		if uninstallWithRoutines {
			adapter := routine.NewSystemdAdapter("", "", nil)
			list, err := adapter.List(cmd.Context())
			if err == nil {
				for _, r := range list {
					_ = adapter.Uninstall(cmd.Context(), r.Name)
					fmt.Printf("🗑️ Routine %s uninstalled.\n", r.Name)
				}
			}
		}
		return nil
	},
}

var daemonServiceStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Print systemctl --user status daneel",
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := daemon.UserServiceStatus()
		if out != "" {
			fmt.Print(out)
		}
		if err != nil {
			return err
		}
		return nil
	},
}

func init() {
	daemonRunCmd.Flags().BoolVar(&daemonRunServe, "serve", false, "Enable the HTTP Galaxy Visualizer alongside the Unix socket")
	daemonRunCmd.Flags().IntVarP(&daemonRunPort, "port", "p", 4321, "HTTP port for the Galaxy Visualizer")

	daemonServiceInstallCmd.Flags().BoolVar(&serviceWithServe, "with-serve", false, "Configure the systemd service to run with the HTTP Galaxy Visualizer")
	daemonServiceInstallCmd.Flags().IntVarP(&servicePort, "port", "p", 4321, "HTTP port when --with-serve is enabled")
	daemonServiceInstallCmd.Flags().BoolVar(&serviceWithRoutines, "with-routines", false, "Also install scheduled routine timers")

	daemonServiceUninstallCmd.Flags().BoolVar(&uninstallWithRoutines, "with-routines", false, "Also uninstall all scheduled routine timers")

	daemonServiceCmd.AddCommand(daemonServiceInstallCmd)
	daemonServiceCmd.AddCommand(daemonServiceUninstallCmd)
	daemonServiceCmd.AddCommand(daemonServiceStatusCmd)

	daemonCmd.AddCommand(daemonRunCmd)
	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStopCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
	daemonCmd.AddCommand(daemonServiceCmd)

	RootCmd.AddCommand(daemonCmd)
}
