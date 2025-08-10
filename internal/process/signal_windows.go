//go:build windows
// +build windows

package process

import (
	"os"
)

// isProcessAlive checks if the process is still alive (Windows implementation)
func isProcessAlive(proc *os.Process) bool {
	// On Windows, we try to get the process state
	// If Wait returns an error, the process is still running
	_, err := proc.Wait()
	if err != nil {
		// Process is still running
		return true
	}
	return false
}

// terminateProcess terminates the process (Windows implementation)
func terminateProcess(proc *os.Process) error {
	// On Windows, we can only kill the process
	return proc.Kill()
}