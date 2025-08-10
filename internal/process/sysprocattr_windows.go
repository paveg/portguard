//go:build windows
// +build windows

package process

import "syscall"

// setSysProcAttr sets the system process attributes for Windows
func setSysProcAttr(attr *syscall.SysProcAttr) *syscall.SysProcAttr {
	// Windows doesn't support Setpgid
	// Return the attribute as-is or create a new one
	if attr == nil {
		attr = &syscall.SysProcAttr{}
	}
	return attr
}
