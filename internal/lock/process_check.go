package lock

import (
	"os"
)

// processExists checks if a process with the given PID exists
// The actual implementation is platform-specific
func processExists(pid int) bool {
	return checkProcess(pid)
}

// getCurrentPID returns the current process ID
func getCurrentPID() int { //nolint:unused // TODO: Remove if not used in future lock manager features
	return os.Getpid()
}
