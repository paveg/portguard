package process

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
	"time"
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
	GetPortInfo(port int) (*PortInfo, error)
	ScanRange(startPort, endPort int) ([]PortInfo, error)
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
	return fmt.Sprintf("%x", hash)[:8]
}

// ShouldStartNew determines if a new process should be started or an existing one reused
func (pm *ProcessManager) ShouldStartNew(command string, port int) (bool, *ManagedProcess) {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	// 1. Check if exact command is already running
	for _, process := range pm.processes {
		if process.Command == command && process.IsHealthy() {
			return false, process // Reuse existing healthy process
		}
	}

	// 2. Check port availability if specified
	if port > 0 {
		if pm.portScanner.IsPortInUse(port) {
			// Check if the port is occupied by one of our managed processes
			for _, process := range pm.processes {
				if process.Port == port && process.IsRunning() {
					return false, process // Port occupied by managed process
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

	// Create new managed process
	process := &ManagedProcess{
		ID:          pm.generateID(command),
		Command:     command,
		Args:        args,
		Port:        options.Port,
		Status:      StatusPending,
		HealthCheck: options.HealthCheck,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		LastSeen:    time.Now(),
		Environment: options.Environment,
		WorkingDir:  options.WorkingDir,
		LogFile:     options.LogFile,
	}

	// TODO: Actually start the process here
	// For now, just mark it as running
	process.Status = StatusRunning
	process.PID = 12345 // Placeholder

	// Store the process
	pm.mutex.Lock()
	pm.processes[process.ID] = process
	pm.mutex.Unlock()

	// Persist to storage
	if err := pm.stateStore.Save(pm.processes); err != nil {
		return nil, fmt.Errorf("failed to save state: %w", err)
	}

	return process, nil
}

// StopProcess stops a managed process
func (pm *ProcessManager) StopProcess(id string, _ bool) error {
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

	// TODO: Actually stop the process here
	process.Status = StatusStopped
	process.UpdatedAt = time.Now()
	pm.mutex.Unlock()

	// Persist to storage
	if err := pm.stateStore.Save(pm.processes); err != nil {
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

	var result []*ManagedProcess
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
	for id, process := range pm.processes {
		if force || process.Status == StatusStopped || process.Status == StatusFailed {
			// TODO: Actually clean up process resources
			toRemove = append(toRemove, id)
		}
	}

	for _, id := range toRemove {
		delete(pm.processes, id)
	}

	if err := pm.stateStore.Save(pm.processes); err != nil {
		return fmt.Errorf("failed to save process state: %w", err)
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
