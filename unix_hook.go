//go:build !windows
// +build !windows

package main

import "os/exec"

// hideWindow is a no-op on non-Windows systems.
func hideWindow(cmd *exec.Cmd) {
	// No action needed
}
