// Package process provides core process management functionality for Portguard.
// It implements the ProcessManager for handling server process lifecycle,
// conflict detection, and integration with state management and port scanning.
package process

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/paveg/portguard/internal/port"
)

// Static error variables to satisfy err113 linter
var (
	ErrPortAlreadyInUse = errors.New("cannot start process: port is already in use")
	ErrProcessNotFound  = errors.New("process not found")
)

// ProcessManager manages all processes for portguard
type ProcessManager struct {
	processes   map[string]*ManagedProcess
	mutex       sync.RWMutex
	stateStore  StateStore
	lockManager LockManager
	portScanner PortScanner
}

// StateStore interface for persisting process state
type StateStore interface {
	Save(processes map[string]*ManagedProcess) error
	Load() (map[string]*ManagedProcess, error)
	Delete(id string) error
}

// LockManager interface for managing concurrent access
type LockManager interface {
	Lock() error
	Unlock() error
	IsLocked() bool
}

// PortScanner interface for scanning port usage
type PortScanner interface {
	IsPortInUse(port int) bool
	GetPortInfo(port int) (*port.PortInfo, error)
	ScanRange(startPort, endPort int) ([]port.PortInfo, error)
	FindAvailablePort(startPort int) (int, error)
}

// NewProcessManager creates a new ProcessManager instance
func NewProcessManager(stateStore StateStore, lockManager LockManager, portScanner PortScanner) *ProcessManager {
	pm := &ProcessManager{
		processes:   make(map[string]*ManagedProcess),
		stateStore:  stateStore,
		lockManager: lockManager,
		portScanner: portScanner,
	}

	// Load existing processes from storage
	if loadedProcesses, err := stateStore.Load(); err == nil {
		pm.processes = loadedProcesses
	}

	return pm
}

// generateID generates a unique ID for a process based on command and timestamp
func (pm *ProcessManager) generateID(command string) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s-%d", command, time.Now().UnixNano())))
	return fmt.Sprintf("%x", hash)[:8] //nolint:perfsprint // TODO: Use hex.EncodeToString for better performance
}

// ShouldStartNew determines if a new process should be started or an existing one reused
func (pm *ProcessManager) ShouldStartNew(command string, portNum int) (bool, *ManagedProcess) {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	// 1. Check if exact command is already running
	for _, process := range pm.processes {
		if process.Command == command && process.IsHealthy() {
			return false, process // Reuse existing healthy process
		}
	}

	// 2. Check port availability if specified
	//nolint:nestif // Complex port conflict logic is necessary for correctness
	if portNum > 0 {
		if pm.portScanner.IsPortInUse(portNum) {
			// Check if the port is occupied by one of our managed processes
			for _, process := range pm.processes {
				if process.Port == portNum && process.IsRunning() {
					// Only return the process if it's the same command
					if process.Command == command {
						return false, process // Same command, reuse process
					}
					// Different command using same port - this is a conflict
					return false, nil // Port occupied by different command
				}
			}
			return false, nil // Port occupied by external process
		}
	}

	// 3. Safe to start new process
	return true, nil
}

// StartProcess starts a new process or returns an existing one
func (pm *ProcessManager) StartProcess(command string, args []string, options StartOptions) (*ManagedProcess, error) {
	if err := pm.lockManager.Lock(); err != nil {
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer func() { _ = pm.lockManager.Unlock() }() //nolint:errcheck // Defer unlock completes regardless

	// Check if we should start a new process
	shouldStart, existing := pm.ShouldStartNew(command, options.Port)
	if !shouldStart {
		if existing != nil {
			return existing, nil // Reuse existing process
		}
		return nil, fmt.Errorf("%w: %d", ErrPortAlreadyInUse, options.Port)
	}

	// Actually start the process using the new executeProcess method
	actualProcess, err := pm.executeProcess(command, args, options)
	if err != nil {
		return nil, fmt.Errorf("failed to execute process: %w", err)
	}

	// Set the process ID for state management
	actualProcess.ID = pm.generateID(actualProcess.Command)

	// Store the process and create a copy for safe concurrent access
	pm.mutex.Lock()
	pm.processes[actualProcess.ID] = actualProcess
	// Create a copy of the processes map for safe concurrent access to stateStore
	processesCopy := make(map[string]*ManagedProcess)
	for k, v := range pm.processes {
		processesCopy[k] = v
	}
	pm.mutex.Unlock()

	// Persist to storage using the copy to avoid race conditions
	if err := pm.stateStore.Save(processesCopy); err != nil {
		return nil, fmt.Errorf("failed to save state: %w", err)
	}

	// Start background monitoring for the process
	go pm.monitorProcessInBackground(actualProcess)

	return actualProcess, nil
}

// AdoptProcess adopts an existing external process into management
func (pm *ProcessManager) AdoptProcess(managedProcess *ManagedProcess) error {
	if err := pm.lockManager.Lock(); err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer func() { _ = pm.lockManager.Unlock() }()

	// Validate the process
	if managedProcess == nil {
		return errors.New("cannot adopt nil process")
	}

	if managedProcess.PID <= 0 {
		return fmt.Errorf("invalid PID: %d", managedProcess.PID)
	}

	// Generate ID if not set
	if managedProcess.ID == "" {
		managedProcess.ID = pm.generateID(managedProcess.Command)
	}

	// Set adoption timestamp
	managedProcess.CreatedAt = time.Now()
	managedProcess.StartedAt = time.Now()
	managedProcess.UpdatedAt = time.Now()
	managedProcess.LastSeen = time.Now()

	// Store the process
	pm.mutex.Lock()
	pm.processes[managedProcess.ID] = managedProcess
	// Create a copy of the processes map for safe concurrent access to stateStore
	processesCopy := make(map[string]*ManagedProcess)
	for k, v := range pm.processes {
		processesCopy[k] = v
	}
	pm.mutex.Unlock()

	// Persist to storage
	if err := pm.stateStore.Save(processesCopy); err != nil {
		// Remove from memory if save failed
		pm.mutex.Lock()
		delete(pm.processes, managedProcess.ID)
		pm.mutex.Unlock()
		return fmt.Errorf("failed to save state: %w", err)
	}

	// Start background monitoring for the adopted process
	go pm.monitorProcessInBackground(managedProcess)

	return nil
}

// StopProcess stops a managed process
func (pm *ProcessManager) StopProcess(id string, forceKill bool) error {
	if err := pm.lockManager.Lock(); err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer func() { _ = pm.lockManager.Unlock() }() //nolint:errcheck // Defer unlock completes regardless

	pm.mutex.Lock()
	process, exists := pm.processes[id]
	if !exists {
		pm.mutex.Unlock()
		return fmt.Errorf("%w: %s", ErrProcessNotFound, id)
	}
	pm.mutex.Unlock()

	// Actually terminate the process using the new method
	if err := pm.terminateProcess(process, forceKill); err != nil {
		return fmt.Errorf("failed to terminate process: %w", err)
	}

	// Update state in storage
	pm.mutex.Lock()
	processesCopy := make(map[string]*ManagedProcess)
	for k, v := range pm.processes {
		processesCopy[k] = v
	}
	pm.mutex.Unlock()

	// Persist to storage using the copy to avoid race conditions
	if err := pm.stateStore.Save(processesCopy); err != nil {
		return fmt.Errorf("failed to save process state: %w", err)
	}
	return nil
}

// GetProcess retrieves a process by ID
func (pm *ProcessManager) GetProcess(id string) (*ManagedProcess, bool) {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	process, exists := pm.processes[id]
	return process, exists
}

// ListProcesses returns all managed processes
func (pm *ProcessManager) ListProcesses(options ProcessListOptions) []*ManagedProcess {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	var result []*ManagedProcess //nolint:prealloc // TODO: Pre-allocate slice based on filter criteria
	for _, process := range pm.processes {
		// Apply filters
		if !options.IncludeStopped && !process.IsRunning() {
			continue
		}

		if options.FilterByPort > 0 && process.Port != options.FilterByPort {
			continue
		}

		result = append(result, process)
	}

	return result
}

// CleanupProcesses removes stopped processes and cleans up resources
func (pm *ProcessManager) CleanupProcesses(force bool) error {
	if err := pm.lockManager.Lock(); err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer func() { _ = pm.lockManager.Unlock() }() //nolint:errcheck // Defer unlock completes regardless

	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	var toRemove []string
	var cleanupErrors []error

	for id, process := range pm.processes {
		if force || process.Status == StatusStopped || process.Status == StatusFailed {
			// Actually clean up process resources
			if err := pm.cleanupProcessResources(process, force); err != nil {
				cleanupErrors = append(cleanupErrors, fmt.Errorf("failed to cleanup process %s: %w", id, err))
				// Continue with other processes even if one fails
			}
			toRemove = append(toRemove, id)
		}
	}

	// Remove processes from memory
	for _, id := range toRemove {
		delete(pm.processes, id)
	}

	// Create a copy of the processes map for safe concurrent access to stateStore
	processesCopy := make(map[string]*ManagedProcess)
	for k, v := range pm.processes {
		processesCopy[k] = v
	}

	if err := pm.stateStore.Save(processesCopy); err != nil {
		return fmt.Errorf("failed to save process state: %w", err)
	}

	// Return first cleanup error if any occurred
	if len(cleanupErrors) > 0 {
		return cleanupErrors[0]
	}

	return nil
}

// cleanupProcessResources performs actual cleanup of process resources
func (pm *ProcessManager) cleanupProcessResources(process *ManagedProcess, force bool) error {
	var cleanupErrors []error

	// 1. Terminate the process if it's still running
	if process.IsRunning() {
		if err := pm.terminateProcess(process, force); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("failed to terminate process: %w", err))
		}
	}

	// 2. Clean up log files if they exist and are managed by us
	if process.LogFile != "" {
		if err := cleanupLogFile(process.LogFile); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("failed to cleanup log file: %w", err))
		}
	}

	// 3. Clean up any temporary files or directories if specified
	// This could be extended to handle process-specific cleanup
	if process.WorkingDir != "" && strings.Contains(process.WorkingDir, "temp") {
		// Only clean up directories that look like temp directories to avoid accidents
		if err := cleanupTempDirectory(process.WorkingDir); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("failed to cleanup working directory: %w", err))
		}
	}

	// Return first error if any occurred
	if len(cleanupErrors) > 0 {
		return cleanupErrors[0]
	}

	return nil
}

// cleanupLogFile removes log files if they exist
func cleanupLogFile(logFile string) error {
	if logFile == "" {
		return nil
	}

	// Check if file exists
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		return nil // File doesn't exist, nothing to clean up
	}

	// Remove the log file
	if err := os.Remove(logFile); err != nil {
		return fmt.Errorf("failed to remove log file %s: %w", logFile, err)
	}

	return nil
}

// cleanupTempDirectory removes temporary directories if they're safe to remove
func cleanupTempDirectory(workingDir string) error {
	if workingDir == "" {
		return nil
	}

	// Safety check: only clean up directories that contain "temp" in the path
	if !strings.Contains(strings.ToLower(workingDir), "temp") {
		return nil // Skip cleanup for safety
	}

	// Check if directory exists
	if _, err := os.Stat(workingDir); os.IsNotExist(err) {
		return nil // Directory doesn't exist, nothing to clean up
	}

	// Remove the directory and its contents
	if err := os.RemoveAll(workingDir); err != nil {
		return fmt.Errorf("failed to remove temp directory %s: %w", workingDir, err)
	}

	return nil
}

// StartOptions defines options for starting a process
type StartOptions struct {
	Port        int               `json:"port"`
	HealthCheck *HealthCheck      `json:"health_check"`
	Environment map[string]string `json:"environment"`
	WorkingDir  string            `json:"working_dir"`
	LogFile     string            `json:"log_file"`
	Background  bool              `json:"background"`
}

// executeProcess executes a process with the given command and options
func (pm *ProcessManager) executeProcess(command string, args []string, options StartOptions) (*ManagedProcess, error) {
	// Parse command if args are empty (for backward compatibility with shell commands)
	if len(args) == 0 {
		parts := strings.Fields(command)
		if len(parts) == 0 {
			return nil, errors.New("empty command")
		}
		command = parts[0]
		if len(parts) > 1 {
			args = parts[1:]
		}
	}

	// Create command with context
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, command, args...)

	// Set working directory if specified
	if options.WorkingDir != "" {
		cmd.Dir = options.WorkingDir
	}

	// Set environment variables
	if len(options.Environment) > 0 {
		cmd.Env = os.Environ()
		for key, value := range options.Environment {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
		}
	}

	// Set up process group for signal management (platform-specific)
	cmd.SysProcAttr = setSysProcAttr(nil)

	// Set up log file if specified
	if options.LogFile != "" {
		logFile, err := os.OpenFile(options.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file %s: %w", options.LogFile, err)
		}
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command '%s': %w", command, err)
	}

	// Create managed process with actual PID
	process := &ManagedProcess{
		Command:     strings.Join(append([]string{command}, args...), " "),
		Args:        args,
		Port:        options.Port,
		PID:         cmd.Process.Pid,
		Status:      StatusRunning,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		LastSeen:    time.Now(),
		Environment: options.Environment,
		WorkingDir:  options.WorkingDir,
		LogFile:     options.LogFile,
		HealthCheck: options.HealthCheck,
	}

	return process, nil
}

// monitorProcessInBackground monitors a process in the background
func (pm *ProcessManager) monitorProcessInBackground(process *ManagedProcess) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Monitor the process
	if err := pm.monitorProcess(ctx, process); err != nil {
		// Log error but don't fail - this is a background operation
		//nolint:errcheck // Background operation, error logged elsewhere
		_ = pm.updateProcessStatus(process.ID, StatusFailed)
	}
}

// monitorProcess monitors a process and updates its status
func (pm *ProcessManager) monitorProcess(ctx context.Context, process *ManagedProcess) error {
	if process.PID <= 0 {
		return fmt.Errorf("invalid PID: %d", process.PID)
	}

	// Use shorter intervals for testing or configurable intervals
	checkInterval := 500 * time.Millisecond // More frequent checks for testing
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	// Do an immediate check first
	osProcess, err := os.FindProcess(process.PID)
	if err != nil {
		//nolint:errcheck // Background monitoring, error logged elsewhere
		_ = pm.updateProcessStatus(process.ID, StatusStopped)
		return fmt.Errorf("process not found: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Send signal 0 to check if process exists
			if !isProcessAlive(osProcess) {
				// Process has stopped
				//nolint:errcheck // Background monitoring, error logged elsewhere
				_ = pm.updateProcessStatus(process.ID, StatusStopped)
				return nil
			}

			// Update last seen timestamp
			pm.mutex.Lock()
			if proc, exists := pm.processes[process.ID]; exists {
				proc.LastSeen = time.Now()
			}
			pm.mutex.Unlock()

			// Run health check if configured
			if process.HealthCheck != nil {
				if err := pm.runHealthCheck(ctx, process); err != nil {
					//nolint:errcheck // Background monitoring, error logged elsewhere
					_ = pm.updateProcessStatus(process.ID, StatusUnhealthy)
				} else {
					//nolint:errcheck // Background monitoring, error logged elsewhere
					_ = pm.updateProcessStatus(process.ID, StatusRunning)
				}
			}
		}
	}
}

// terminateProcess terminates a process
func (pm *ProcessManager) terminateProcess(process *ManagedProcess, forceKill bool) error {
	if process.PID <= 0 {
		return fmt.Errorf("invalid PID: %d", process.PID)
	}

	osProcess, err := os.FindProcess(process.PID)
	if err != nil {
		// Process not found - update status and return success since the goal is achieved
		process.Status = StatusStopped
		process.UpdatedAt = time.Now()
		//nolint:nilerr // Process not existing is the desired outcome for termination
		return nil
	}

	// Check if process is still running before trying to terminate
	if !isProcessAlive(osProcess) {
		// Process is already dead - update status and return success since goal is achieved
		process.Status = StatusStopped
		process.UpdatedAt = time.Now()
		//nolint:nilerr // Process being dead is the desired outcome for termination
		return nil
	}

	// Try graceful termination first
	//nolint:nestif // Complex termination logic with graceful fallback is necessary
	if !forceKill {
		if err := terminateProcess(osProcess); err != nil {
			// If SIGTERM fails, the process might already be gone
			if err.Error() == "os: process already finished" {
				process.Status = StatusStopped
				process.UpdatedAt = time.Now()
				return nil
			}
			// For other errors, fall back to SIGKILL
			forceKill = true
		} else {
			// Wait a bit for graceful shutdown
			time.Sleep(2 * time.Second)

			// Check if process still exists
			if isProcessAlive(osProcess) {
				// Process still running, force kill
				forceKill = true
			}
		}
	}

	// Force kill if requested or graceful termination failed
	if forceKill {
		if err := osProcess.Kill(); err != nil {
			// Process might have exited between checks
			if err.Error() == "os: process already finished" {
				process.Status = StatusStopped
				process.UpdatedAt = time.Now()
				return nil
			}
			return fmt.Errorf("failed to kill process %d: %w", process.PID, err)
		}
	}

	// Update process status
	process.Status = StatusStopped
	process.UpdatedAt = time.Now()

	return nil
}

// findSimilarProcess finds a similar process that could be reused
func (pm *ProcessManager) findSimilarProcess(command string) (*ManagedProcess, bool) {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	signature := pm.generateCommandSignature(command, []string{})

	var candidates []*ManagedProcess

	// Find processes with matching command signature
	for _, process := range pm.processes {
		processSignature := pm.generateCommandSignature(process.Command, process.Args)
		if processSignature == signature && process.IsHealthy() {
			candidates = append(candidates, process)
		}
	}

	if len(candidates) == 0 {
		return nil, false
	}

	// Return the most recently created healthy process
	var newest *ManagedProcess
	for _, candidate := range candidates {
		if newest == nil || candidate.CreatedAt.After(newest.CreatedAt) {
			newest = candidate
		}
	}

	return newest, true
}

// generateCommandSignature generates a normalized signature for a command
func (pm *ProcessManager) generateCommandSignature(command string, args []string) string {
	// Normalize command by joining with args and removing extra whitespace
	fullCommand := strings.TrimSpace(command)
	if len(args) > 0 {
		fullCommand = strings.TrimSpace(strings.Join(append([]string{command}, args...), " "))
	}

	// Normalize whitespace
	parts := strings.Fields(fullCommand)
	return strings.Join(parts, " ")
}

// updateProcessStatus updates the status of a process
func (pm *ProcessManager) updateProcessStatus(processID string, status ProcessStatus) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	process, exists := pm.processes[processID]
	if !exists {
		return fmt.Errorf("%w: %s", ErrProcessNotFound, processID)
	}

	process.Status = status
	process.UpdatedAt = time.Now()

	// Create a copy of the processes map for safe concurrent access to stateStore
	processesCopy := make(map[string]*ManagedProcess)
	for k, v := range pm.processes {
		processesCopy[k] = v
	}

	// Save to persistent storage
	if err := pm.stateStore.Save(processesCopy); err != nil {
		return fmt.Errorf("failed to save process state: %w", err)
	}

	return nil
}

// runHealthCheck runs a health check for a process
func (pm *ProcessManager) runHealthCheck(ctx context.Context, process *ManagedProcess) error {
	if process.HealthCheck == nil {
		return nil // No health check configured
	}

	if !process.HealthCheck.Enabled {
		return nil // Health checking disabled
	}

	// Set up timeout context
	healthCtx, cancel := context.WithTimeout(ctx, process.HealthCheck.Timeout)
	defer cancel()

	// Perform health check based on type
	switch process.HealthCheck.Type {
	case HealthCheckHTTP:
		return pm.performHTTPHealthCheck(healthCtx, process)
	case HealthCheckTCP:
		return pm.performTCPHealthCheck(healthCtx, process)
	case HealthCheckCommand:
		return pm.performCommandHealthCheck(healthCtx, process)
	case HealthCheckNone:
		return nil // No health check
	default:
		// Fallback to basic process alive check
		if process.PID > 0 {
			if osProcess, err := os.FindProcess(process.PID); err == nil {
				if isProcessAlive(osProcess) {
					return nil // Process is running, consider it healthy
				}
			}
		}
		return fmt.Errorf("process %s failed basic health check", process.ID)
	}
}

// cleanupStaleProcesses removes processes that haven't been seen for a while
func (pm *ProcessManager) cleanupStaleProcesses(maxAge time.Duration) (int, error) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	var toRemove []string
	cutoffTime := time.Now().Add(-maxAge)

	for id, process := range pm.processes {
		// Remove processes that haven't been seen recently (stale)
		// This includes both running and non-running processes
		if process.LastSeen.Before(cutoffTime) {
			toRemove = append(toRemove, id)
		}
	}

	for _, id := range toRemove {
		delete(pm.processes, id)
	}

	if len(toRemove) > 0 {
		// Create a copy of the processes map for safe concurrent access to stateStore
		processesCopy := make(map[string]*ManagedProcess)
		for k, v := range pm.processes {
			processesCopy[k] = v
		}

		if err := pm.stateStore.Save(processesCopy); err != nil {
			return 0, fmt.Errorf("failed to save process state: %w", err)
		}
	}

	return len(toRemove), nil
}

// performHTTPHealthCheck performs an HTTP health check
func (pm *ProcessManager) performHTTPHealthCheck(ctx context.Context, process *ManagedProcess) error {
	if process.HealthCheck.Target == "" {
		return errors.New("HTTP health check target URL not specified")
	}

	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, "GET", process.HealthCheck.Target, http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Perform HTTP request with timeout
	httpClient := &http.Client{
		Timeout: process.HealthCheck.Timeout,
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP health check failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // Cleanup operation

	// Check HTTP status code
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP health check failed with status %d", resp.StatusCode)
	}

	return nil
}

// performTCPHealthCheck performs a TCP connection health check
func (pm *ProcessManager) performTCPHealthCheck(ctx context.Context, process *ManagedProcess) error {
	if process.HealthCheck.Target == "" {
		return errors.New("TCP health check target address not specified")
	}

	// Create TCP connection with context
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", process.HealthCheck.Target)
	if err != nil {
		return fmt.Errorf("TCP health check failed: %w", err)
	}
	defer func() { _ = conn.Close() }() //nolint:errcheck // Cleanup operation

	return nil
}

// performCommandHealthCheck performs a command-based health check
func (pm *ProcessManager) performCommandHealthCheck(ctx context.Context, process *ManagedProcess) error {
	if process.HealthCheck.Target == "" {
		return errors.New("command health check target not specified")
	}

	// Parse command and arguments
	parts := strings.Fields(process.HealthCheck.Target)
	if len(parts) == 0 {
		return errors.New("empty health check command")
	}

	command := parts[0]
	args := parts[1:]

	// Execute command with context
	cmd := exec.CommandContext(ctx, command, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command health check failed: %w (output: %s)", err, string(output))
	}

	return nil
}
