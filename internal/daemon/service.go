package daemon

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

const systemdUnitTemplate = `[Unit]
Description=Daneel Background Daemon
After=network.target

[Service]
Type=simple
ExecStart={{.Executable}} daemon run{{if .WithServe}} --serve --port {{.Port}}{{end}}
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`

func getUnitPath() string {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "/tmp"
		}
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "systemd", "user", "daneel.service")
}

// InstallUserService generates and installs the systemd --user unit.
func InstallUserService(withServe bool, port int) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	unitPath := getUnitPath()
	if err := os.MkdirAll(filepath.Dir(unitPath), 0750); err != nil {
		return fmt.Errorf("failed to create systemd user directory: %w", err)
	}

	tmpl, err := template.New("unit").Parse(systemdUnitTemplate)
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"Executable": exe,
		"WithServe":  withServe,
		"Port":       port,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return err
	}

	if err := os.WriteFile(unitPath, buf.Bytes(), 0600); err != nil {
		return fmt.Errorf("failed to write systemd unit: %w", err)
	}

	if err := exec.Command("systemctl", "--user", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("failed to reload systemd daemon: %w", err)
	}

	if err := exec.Command("systemctl", "--user", "enable", "--now", "daneel.service").Run(); err != nil {
		return fmt.Errorf("failed to enable and start daneel service: %w", err)
	}

	return nil
}

// UninstallUserService stops and removes the systemd --user unit.
func UninstallUserService() error {
	unitPath := getUnitPath()

	// Best effort stop and disable
	_ = exec.Command("systemctl", "--user", "stop", "daneel.service").Run()
	_ = exec.Command("systemctl", "--user", "disable", "daneel.service").Run()

	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove systemd unit: %w", err)
	}

	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	return nil
}

// UserServiceStatus returns the status of the systemd --user unit.
func UserServiceStatus() (string, error) {
	cmd := exec.Command("systemctl", "--user", "status", "daneel.service")
	out, err := cmd.CombinedOutput()
	// systemctl status exits with non-zero if stopped/failed, so don't treat err as fatal immediately
	if err != nil && len(out) == 0 {
		return "", err
	}
	return string(out), nil
}
