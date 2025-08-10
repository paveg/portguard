//go:build !windows
// +build !windows

package process

import (
	"os"
	"syscall"
)

// isProcessAlive checks if the process is still alive
func isProcessAlive(proc *os.Process) bool {
	err := proc.Signal(syscall.Signal(0))
	return err == nil
}

// terminateProcess sends SIGTERM to the process
func terminateProcess(proc *os.Process) error {
	return proc.Signal(syscall.SIGTERM)
}