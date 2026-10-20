package daemon

import (
	"context"
)

// RunOrDelegate tries to execute the task via the daemon.
// If the daemon is offline, it falls back to executing the localRunner.
func RunOrDelegate(ctx context.Context, command string, args []string, localRunner func() error) error {
	client, err := NewClient()
	if err != nil {
		// Daemon offline, fallback to local
		return localRunner()
	}
	defer func() { _ = client.Close() }()

	if err := client.Ping(); err != nil {
		return localRunner()
	}

	// Daemon is online.
	// For simplicity, we just run the localRunner and then notify the daemon.
	// (In a more complex implementation, we might send the command to the daemon to execute)
	if err := localRunner(); err != nil {
		return err
	}

	// Notify daemon to trigger a reload or update
	_, _ = client.sendRequest("exec_task", map[string]interface{}{
		"command": command,
		"args":    args,
	})

	return nil
}
