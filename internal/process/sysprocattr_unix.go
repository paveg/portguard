//go:build !windows
// +build !windows

package process

import "syscall"

// setSysProcAttr sets the system process attributes for Unix-like systems
func setSysProcAttr(attr *syscall.SysProcAttr) *syscall.SysProcAttr {
	if attr == nil {
		attr = &syscall.SysProcAttr{}
	}
	attr.Setpgid = true
	return attr
}