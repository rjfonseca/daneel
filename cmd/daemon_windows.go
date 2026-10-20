//go:build windows

package cmd

import (
	"os/exec"
)

func detachProcess(cmd *exec.Cmd) {
	// On Windows, background detachment is handled without Unix Setsid.
}
