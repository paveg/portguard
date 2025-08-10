//go:build windows
// +build windows

package process

import (
	"os"
)

// signalProcess sends a signal to the process (Windows implementation)
func signalProcess(proc *os.Process, sig os.Signal) error {
	// On Windows, we can only kill the process, not send signals
	if sig == os.Kill {
		return proc.Kill()
	}
	// For other signals, try to kill the process
	return proc.Kill()
}

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