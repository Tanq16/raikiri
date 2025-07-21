//go:build windows
// +build windows

package main

import (
	"os/exec"
	"syscall"
)

// hideWindow sets the CREATION_FLAGS to CREATE_NO_WINDOW on Windows
// to prevent the ffmpeg console window from appearing.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}
