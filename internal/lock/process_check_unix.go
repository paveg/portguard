//go:build !windows
// +build !windows

package lock

import (
	"errors"
	"syscall"
)

// checkProcess checks if a process exists on Unix-like systems
func checkProcess(pid int) bool {
	// Try to send signal 0 to the process
	// If the process exists, we get nil or permission error
	// If the process doesn't exist, we get ESRCH
	err := syscall.Kill(pid, 0)

	if err == nil {
		return true // Process exists and we can signal it
	}

	// Check the specific error
	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno { //nolint:exhaustive // We only care about specific errno values
		case syscall.ESRCH:
			return false // Process doesn't exist
		case syscall.EPERM:
			return true // Process exists but we don't have permission
		default:
			return false // Other error, assume process doesn't exist
		}
	}

	return false
}
