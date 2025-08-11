// Package process provides process management and adoption functionality for Portguard.
// This file implements external process adoption capabilities.
package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/paveg/portguard/internal/port"
)

// Static errors for external process adoption
var (
	ErrProcessNotSuitable = errors.New("process not suitable for adoption")
	ErrSystemProcess      = errors.New("cannot adopt system process")
	ErrInsufficientPerms  = errors.New("insufficient permissions to adopt process")
	ErrProcessAlreadyDead = errors.New("process is no longer running")
)

// AdoptionInfo contains information about a process that can be adopted
type AdoptionInfo struct {
	PID         int    `json:"pid"`
	ProcessName string `json:"process_name"`
	Command     string `json:"command"`
	Port        int    `json:"port,omitempty"`
	WorkingDir  string `json:"working_dir,omitempty"`
	IsSuitable  bool   `json:"is_suitable"`
	Reason      string `json:"reason,omitempty"`
}

// ProcessAdopter handles adoption of external processes
type ProcessAdopter struct {
	scanner *port.Scanner
	timeout time.Duration
}

// NewProcessAdopter creates a new process adopter
func NewProcessAdopter(timeout time.Duration) *ProcessAdopter {
	return &ProcessAdopter{
		scanner: port.NewScanner(timeout),
		timeout: timeout,
	}
}

// AdoptProcessByPID adopts an existing process by PID
func (pa *ProcessAdopter) AdoptProcessByPID(pid int) (*ManagedProcess, error) {
	// Validate PID
	if pid <= 0 {
		return nil, fmt.Errorf("invalid PID: %d", pid)
	}

	// Check if process exists and is running
	if !pa.isProcessRunning(pid) {
		return nil, ErrProcessNotFound
	}

	// Get process information
	adoptionInfo, err := pa.GetProcessInfo(pid)
	if err != nil {
		return nil, fmt.Errorf("failed to get process info: %w", err)
	}

	// Check if process is suitable for adoption
	if !adoptionInfo.IsSuitable {
		return nil, fmt.Errorf("%w: %s", ErrProcessNotSuitable, adoptionInfo.Reason)
	}

	// Create ManagedProcess from adoption info
	return pa.createManagedProcessFromAdoption(adoptionInfo)
}

// AdoptProcessByPort adopts a process running on a specific port
func (pa *ProcessAdopter) AdoptProcessByPort(port int) (*ManagedProcess, error) {
	// Get port information
	portInfo, err := pa.scanner.GetPortInfo(port)
	if err != nil {
		return nil, fmt.Errorf("failed to get port info: %w", err)
	}

	// Check if port is in use
	if portInfo.PID <= 0 {
		return nil, fmt.Errorf("port %d is not in use or process not identified", port)
	}

	// Adopt the process by PID
	managedProcess, err := pa.AdoptProcessByPID(portInfo.PID)
	if err != nil {
		return nil, err
	}

	// Update port information
	if managedProcess.Config.Port == 0 {
		managedProcess.Config.Port = port
	}

	return managedProcess, nil
}

// DiscoverAdoptableProcesses finds processes that can be adopted
func (pa *ProcessAdopter) DiscoverAdoptableProcesses(portRange PortRange) ([]*AdoptionInfo, error) {
	// Discover development servers in the port range
	developmentServers, err := pa.scanner.DiscoverDevelopmentServers(portRange.Start, portRange.End)
	if err != nil {
		return nil, fmt.Errorf("failed to discover development servers: %w", err)
	}

	var adoptableProcesses []*AdoptionInfo

	for _, serverInfo := range developmentServers {
		if serverInfo.PID > 0 {
			adoptionInfo, err := pa.GetProcessInfo(serverInfo.PID)
			if err != nil {
				// Log error but continue with other processes
				continue
			}

			// Set port information
			adoptionInfo.Port = serverInfo.Port

			adoptableProcesses = append(adoptableProcesses, adoptionInfo)
		}
	}

	return adoptableProcesses, nil
}

// GetProcessInfo retrieves detailed information about a process for adoption evaluation
func (pa *ProcessAdopter) GetProcessInfo(pid int) (*AdoptionInfo, error) {
	info := &AdoptionInfo{
		PID:        pid,
		IsSuitable: false,
	}

	// Get process name and command
	processName, command, err := pa.scanner.GetProcessInfoByPID(pid)
	if err != nil {
		info.Reason = fmt.Sprintf("failed to get process info: %v", err)
		return info, nil
	}

	info.ProcessName = processName
	info.Command = command

	// Get working directory if possible
	if workingDir, err := pa.getProcessWorkingDir(pid); err == nil {
		info.WorkingDir = workingDir
	}

	// Evaluate if process is suitable for adoption
	info.IsSuitable, info.Reason = pa.evaluateProcessSuitability(info)

	return info, nil
}

// isProcessRunning checks if a process with given PID is running
func (pa *ProcessAdopter) isProcessRunning(pid int) bool {
	// Try to send signal 0 to check if process exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix systems, Signal(0) tests if process exists without sending actual signal
	// On Windows, this behaves differently, so we use platform-specific checks
	if runtime.GOOS == "windows" {
		return pa.isProcessRunningWindows(pid)
	}

	// Unix systems
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// isProcessRunningWindows checks if process is running on Windows
func (pa *ProcessAdopter) isProcessRunningWindows(pid int) bool {
	ctx, cancel := context.WithTimeout(context.Background(), pa.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	return strings.TrimSpace(string(output)) != ""
}

// getProcessWorkingDir attempts to get the working directory of a process
func (pa *ProcessAdopter) getProcessWorkingDir(pid int) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), pa.timeout)
	defer cancel()

	switch runtime.GOOS {
	case "darwin", "linux":
		// Try to read from /proc/<pid>/cwd (Linux) or use lsof (macOS)
		if runtime.GOOS == "linux" {
			link := fmt.Sprintf("/proc/%d/cwd", pid)
			if target, err := os.Readlink(link); err == nil {
				return target, nil
			}
		}

		// Fallback: use lsof to get current working directory
		cmd := exec.CommandContext(ctx, "lsof", "-p", strconv.Itoa(pid), "-d", "cwd", "-Fn")
		output, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "n") {
					return strings.TrimPrefix(line, "n"), nil
				}
			}
		}

	case "windows":
		// Windows doesn't have a direct equivalent, skip for now
		return "", errors.New("working directory detection not supported on Windows")
	}

	return "", errors.New("unable to determine working directory")
}

// evaluateProcessSuitability determines if a process is suitable for adoption
func (pa *ProcessAdopter) evaluateProcessSuitability(info *AdoptionInfo) (bool, string) {
	// Check if it's a system process (PID < 1000 on Unix, < 100 on Windows)
	systemPIDThreshold := 1000
	if runtime.GOOS == "windows" {
		systemPIDThreshold = 100
	}

	if info.PID < systemPIDThreshold {
		return false, "system process (low PID)"
	}

	// Check process name against development server patterns
	devPatterns := []string{
		"node", "npm", "yarn", "pnpm", "webpack", "vite", "next",
		"react-scripts", "vue", "nuxt", "svelte",
		"python", "flask", "django", "fastapi", "uvicorn",
		"go", "air", "gin", "echo", "fiber",
		"ruby", "rails", "sinatra",
		"php", "artisan", "symfony",
		"java", "spring", "tomcat", "jetty",
		"dotnet", "kestrel",
	}

	processNameLower := strings.ToLower(info.ProcessName)
	commandLower := strings.ToLower(info.Command)

	for _, pattern := range devPatterns {
		if strings.Contains(processNameLower, pattern) || strings.Contains(commandLower, pattern) {
			return true, "development server detected"
		}
	}

	// Check for development-related command arguments
	devArgs := []string{"dev", "serve", "start", "run", "watch", "hot"}
	for _, arg := range devArgs {
		if strings.Contains(commandLower, arg) {
			return true, "development command detected"
		}
	}

	return false, "not a recognized development server"
}

// createManagedProcessFromAdoption creates a ManagedProcess from adoption info
func (pa *ProcessAdopter) createManagedProcessFromAdoption(info *AdoptionInfo) (*ManagedProcess, error) {
	// Create process configuration
	config := &ProcessConfig{
		Command:    info.Command,
		Port:       info.Port,
		WorkingDir: info.WorkingDir,
		ID:         fmt.Sprintf("adopted-%d", info.PID),
		// Default health check for adopted processes
		HealthCheck: &HealthCheck{
			Type:     "tcp",
			Target:   fmt.Sprintf("localhost:%d", info.Port),
			Timeout:  30 * time.Second,
			Interval: 10 * time.Second,
			Retries:  3,
		},
	}

	// If no port detected, use process-based health check
	if info.Port == 0 {
		config.HealthCheck = &HealthCheck{
			Type:     "process",
			Target:   strconv.Itoa(info.PID),
			Timeout:  5 * time.Second,
			Interval: 30 * time.Second,
			Retries:  1,
		}
	}

	// Create managed process
	managedProcess := &ManagedProcess{
		Config:     config,
		PID:        info.PID,
		Status:     StatusRunning,
		StartedAt:  time.Now(), // We don't know the actual start time, use adoption time
		IsExternal: true,       // Mark as externally started
	}

	return managedProcess, nil
}
