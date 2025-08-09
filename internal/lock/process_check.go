package lock

import (
	"errors"
	"os"
	"runtime"
	"strconv"
	"syscall"
)

// processExists checks if a process with the given PID exists
// Cross-platform implementation
func processExists(pid int) bool {
	switch runtime.GOOS {
	case "windows":
		return processExistsWindows(pid)
	default:
		return processExistsUnix(pid)
	}
}

// processExistsUnix checks if a process exists on Unix-like systems
func processExistsUnix(pid int) bool {
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

// processExistsWindows checks if a process exists on Windows
func processExistsWindows(pid int) bool {
	// On Windows, we check if /proc/PID exists (if available)
	// or try to read process information
	procDir := "/proc/" + strconv.Itoa(pid)
	if _, err := os.Stat(procDir); err == nil {
		return true
	}

	// Fallback: try to open the process (Windows-specific would need syscalls)
	// For now, we'll use a simple heuristic
	return pid > 0 && pid < 65536 // Basic PID range check
}

// getCurrentPID returns the current process ID
func getCurrentPID() int {
	return os.Getpid()
}
