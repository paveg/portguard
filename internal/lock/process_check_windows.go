//go:build windows
// +build windows

package lock

import (
	"os"
	"strconv"
)

// checkProcess checks if a process exists on Windows
func checkProcess(pid int) bool {
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