package routine

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"
)

const serviceUnitTemplate = `[Unit]
Description=Daneel Routine: {{.Description}}
After=network.target

[Service]
Type=oneshot
ExecStart={{.Executable}} routine exec {{.Name}} --trigger=systemd
StandardOutput=journal
StandardError=journal
`

const timerUnitTemplate = `[Unit]
Description=Timer for Daneel Routine: {{.Description}}

[Timer]
OnCalendar={{.OnCalendar}}
Persistent=true

[Install]
WantedBy=timers.target
`

// CommandRunner abstracts external command execution for testability.
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) error
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
	CombinedOutput(ctx context.Context, name string, args ...string) ([]byte, error)
}

type osCommandRunner struct{}

func (r *osCommandRunner) Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (r *osCommandRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

func (r *osCommandRunner) CombinedOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

// SystemdAdapter implements SchedulerPort over Linux systemd --user units and timers.
type SystemdAdapter struct {
	unitDir    string
	executable string
	runner     CommandRunner
}

// NewSystemdAdapter creates a new systemd routine adapter.
func NewSystemdAdapter(unitDir string, executable string, runner CommandRunner) *SystemdAdapter {
	if unitDir == "" {
		unitDir = getDefaultUnitDir()
	}
	if executable == "" {
		if exe, err := os.Executable(); err == nil {
			executable = exe
		} else {
			executable = "daneel"
		}
	}
	if runner == nil {
		runner = &osCommandRunner{}
	}
	return &SystemdAdapter{
		unitDir:    unitDir,
		executable: executable,
		runner:     runner,
	}
}

func getDefaultUnitDir() string {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "/tmp"
		}
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "systemd", "user")
}

func (s *SystemdAdapter) Name() string {
	return "systemd"
}

func (s *SystemdAdapter) servicePath(name string) string {
	return filepath.Join(s.unitDir, fmt.Sprintf("daneel-%s.service", name))
}

func (s *SystemdAdapter) timerPath(name string) string {
	return filepath.Join(s.unitDir, fmt.Sprintf("daneel-%s.timer", name))
}

func (s *SystemdAdapter) timerUnitName(name string) string {
	return fmt.Sprintf("daneel-%s.timer", name)
}

func (s *SystemdAdapter) serviceUnitName(name string) string {
	return fmt.Sprintf("daneel-%s.service", name)
}

// Install writes unit and timer files, reloads systemd, and enables the timer if enabled.
func (s *SystemdAdapter) Install(ctx context.Context, r Routine) error {
	if err := os.MkdirAll(s.unitDir, 0750); err != nil {
		return fmt.Errorf("failed to create systemd user directory: %w", err)
	}

	onCalendar := ParseScheduleToOnCalendar(r.Schedule)

	// Render Service Unit
	svcTmpl, err := template.New("service").Parse(serviceUnitTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse service template: %w", err)
	}
	var svcBuf bytes.Buffer
	err = svcTmpl.Execute(&svcBuf, map[string]interface{}{
		"Name":        r.Name,
		"Description": r.Description,
		"Executable":  s.executable,
		"Command":     r.Command,
		"Args":        r.Args,
	})
	if err != nil {
		return fmt.Errorf("failed to render service unit: %w", err)
	}

	// Render Timer Unit
	timerTmpl, err := template.New("timer").Parse(timerUnitTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse timer template: %w", err)
	}
	var timerBuf bytes.Buffer
	err = timerTmpl.Execute(&timerBuf, map[string]interface{}{
		"Description": r.Description,
		"OnCalendar":  onCalendar,
	})
	if err != nil {
		return fmt.Errorf("failed to render timer unit: %w", err)
	}

	// Ensure unit directory exists
	if err := os.MkdirAll(s.unitDir, 0750); err != nil {
		return fmt.Errorf("failed to create systemd unit directory %s: %w", s.unitDir, err)
	}

	// Write files
	if err := os.WriteFile(s.servicePath(r.Name), svcBuf.Bytes(), 0600); err != nil {
		return fmt.Errorf("failed to write service unit %s: %w", s.servicePath(r.Name), err)
	}
	if err := os.WriteFile(s.timerPath(r.Name), timerBuf.Bytes(), 0600); err != nil {
		return fmt.Errorf("failed to write timer unit %s: %w", s.timerPath(r.Name), err)
	}

	// Reload daemon
	_ = s.runner.Run(ctx, "systemctl", "--user", "daemon-reload")

	// Enable if requested
	if r.Enabled {
		_ = s.runner.Run(ctx, "systemctl", "--user", "enable", "--now", s.timerUnitName(r.Name))
	}

	return nil
}

// Uninstall stops, disables, and deletes timer and service units.
func (s *SystemdAdapter) Uninstall(ctx context.Context, name string) error {
	_ = s.runner.Run(ctx, "systemctl", "--user", "stop", s.timerUnitName(name))
	_ = s.runner.Run(ctx, "systemctl", "--user", "disable", s.timerUnitName(name))

	_ = os.Remove(s.timerPath(name))
	_ = os.Remove(s.servicePath(name))

	_ = s.runner.Run(ctx, "systemctl", "--user", "daemon-reload")
	return nil
}

// Enable activates the timer unit.
func (s *SystemdAdapter) Enable(ctx context.Context, name string) error {
	return s.runner.Run(ctx, "systemctl", "--user", "enable", "--now", s.timerUnitName(name))
}

// Disable deactivates the timer unit.
func (s *SystemdAdapter) Disable(ctx context.Context, name string) error {
	return s.runner.Run(ctx, "systemctl", "--user", "disable", "--now", s.timerUnitName(name))
}

// RunNow executes the oneshot service immediately.
func (s *SystemdAdapter) RunNow(ctx context.Context, name string) error {
	return s.runner.Run(ctx, "systemctl", "--user", "start", s.serviceUnitName(name))
}

// Logs streams or prints systemd journal output for the service.
func (s *SystemdAdapter) Logs(ctx context.Context, name string, lines int, follow bool) error {
	args := []string{"--user", "-u", s.serviceUnitName(name)}
	if lines > 0 {
		args = append(args, "-n", strconv.Itoa(lines))
	}
	if follow {
		args = append(args, "-f")
	}
	return s.runner.Run(ctx, "journalctl", args...)
}

// Status inspects the operational state of a routine.
func (s *SystemdAdapter) Status(ctx context.Context, name string) (*RoutineStatus, error) {
	timerPath := s.timerPath(name)
	timerBytes, err := os.ReadFile(timerPath)
	if err != nil {
		return nil, fmt.Errorf("routine %q timer not found: %w", name, err)
	}

	sched := ""
	for _, line := range strings.Split(string(timerBytes), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "OnCalendar=") {
			sched = strings.TrimPrefix(line, "OnCalendar=")
			break
		}
	}

	desc := ""
	svcBytes, _ := os.ReadFile(s.servicePath(name))
	for _, line := range strings.Split(string(svcBytes), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Description=") {
			desc = strings.TrimPrefix(line, "Description=Daneel Routine: ")
			desc = strings.TrimPrefix(desc, "Description=")
			break
		}
	}

	st := &RoutineStatus{
		Name:        name,
		Description: desc,
		Schedule:    sched,
		Enabled:     true,
		Active:      false,
		LastResult:  "unknown",
	}

	// Query systemctl for real runtime details (best-effort)
	out, err := s.runner.CombinedOutput(ctx, "systemctl", "--user", "show", s.timerUnitName(name), "--property=ActiveState,UnitFileState,NextElapseUSecRealtime,LastTriggerUSec")
	if err == nil && len(out) > 0 {
		props := parseKeyValues(string(out))
		if props["ActiveState"] == "active" {
			st.Active = true
		}
		if props["UnitFileState"] == "disabled" {
			st.Enabled = false
		}
		if val, ok := props["NextElapseUSecRealtime"]; ok && val != "" {
			if usec, err := strconv.ParseInt(val, 10, 64); err == nil && usec > 0 {
				st.NextTrigger = time.UnixMicro(usec)
			}
		}
		if val, ok := props["LastTriggerUSecRealtime"]; ok && val != "" {
			if usec, err := strconv.ParseInt(val, 10, 64); err == nil && usec > 0 {
				st.LastTrigger = time.UnixMicro(usec)
			}
		}
	}

	svcOut, err := s.runner.CombinedOutput(ctx, "systemctl", "--user", "show", s.serviceUnitName(name), "--property=ActiveState,Result")
	if err == nil && len(svcOut) > 0 {
		props := parseKeyValues(string(svcOut))
		if props["ActiveState"] == "activating" || props["ActiveState"] == "active" {
			st.LastResult = "running"
		} else if res, ok := props["Result"]; ok && res != "" {
			st.LastResult = res
		}
	}

	return st, nil
}

// List scans installed routine timers in the user unit directory.
func (s *SystemdAdapter) List(ctx context.Context) ([]RoutineStatus, error) {
	entries, err := os.ReadDir(s.unitDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var list []RoutineStatus
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, "daneel-") && strings.HasSuffix(name, ".timer") {
			routineName := strings.TrimSuffix(strings.TrimPrefix(name, "daneel-"), ".timer")
			st, err := s.Status(ctx, routineName)
			if err == nil && st != nil {
				list = append(list, *st)
			}
		}
	}

	return list, nil
}

func parseKeyValues(content string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if idx := strings.Index(line, "="); idx > 0 {
			k := strings.TrimSpace(line[:idx])
			v := strings.TrimSpace(line[idx+1:])
			result[k] = v
		}
	}
	return result
}
