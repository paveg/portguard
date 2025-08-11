package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	portpkg "github.com/paveg/portguard/internal/port"
	"github.com/paveg/portguard/internal/process"
	"github.com/paveg/portguard/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthCommand_SingleProcess(t *testing.T) {
	// Create temporary directory for test state
	tempDir, err := os.MkdirTemp("", "portguard-health-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }() // Best effort cleanup during test

	// Setup process manager
	pm := createTestProcessManager(t, tempDir)

	// Test with non-existent process (this will exercise the error path)
	err = handleSingleProcessHealth(pm, "nonexistent")

	// Should handle error gracefully
	assert.Error(t, err)
}

func TestHealthCommand_AllProcesses(t *testing.T) {
	tests := []struct {
		name           string
		setupProcesses func() []*process.ManagedProcess
		jsonOutput     bool
		expectedCount  int
	}{
		{
			name: "multiple_healthy_processes",
			setupProcesses: func() []*process.ManagedProcess {
				return []*process.ManagedProcess{
					{
						ID:      "proc1",
						Command: "echo test1",
						PID:     11111,
						Status:  process.StatusRunning,
						HealthCheck: &process.HealthCheck{
							Type:    process.HealthCheckCommand,
							Target:  "echo ok",
							Enabled: true,
							Timeout: 5 * time.Second,
						},
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
						LastSeen:  time.Now(),
					},
					{
						ID:        "proc2",
						Command:   "echo test2",
						PID:       22222,
						Status:    process.StatusRunning,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
						LastSeen:  time.Now(),
					},
				}
			},
			jsonOutput:    false,
			expectedCount: 2,
		},
		{
			name: "json_output_format",
			setupProcesses: func() []*process.ManagedProcess {
				return []*process.ManagedProcess{
					{
						ID:        "proc3",
						Command:   "echo test3",
						PID:       33333,
						Status:    process.StatusRunning,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
						LastSeen:  time.Now(),
					},
				}
			},
			jsonOutput:    true,
			expectedCount: 1,
		},
		{
			name: "no_processes",
			setupProcesses: func() []*process.ManagedProcess {
				return []*process.ManagedProcess{}
			},
			jsonOutput:    false,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory for test state
			tempDir, err := os.MkdirTemp("", "portguard-health-all-test")
			require.NoError(t, err)
			defer func() { _ = os.RemoveAll(tempDir) }() // Best effort cleanup during test

			// Setup mock process manager
			pm := createTestProcessManager(t, tempDir)

			// Note: In the real implementation, we can't directly set pm.processes
			// This is a limitation of testing the current design
			// For now, we'll just test that the function doesn't crash

			// Test handleAllProcessesHealth - just verify it doesn't crash
			err = handleAllProcessesHealth(pm)
			require.NoError(t, err)
		})
	}
}

func TestHealthCommand_Integration(t *testing.T) {
	// Create temporary directory for test state
	tempDir, err := os.MkdirTemp("", "portguard-health-integration-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }() // Best effort cleanup during test

	// Test basic functionality without cobra dependency
	pm := createTestProcessManager(t, tempDir)

	// Test both handle functions directly
	err = handleAllProcessesHealth(pm)
	require.NoError(t, err)

	err = handleSingleProcessHealth(pm, "nonexistent")
	assert.Error(t, err)
}

func TestPerformHealthCheck_Coverage(t *testing.T) {
	tests := []struct {
		name        string
		process     *process.ManagedProcess
		expectError bool
	}{
		{
			name: "process_with_command_health_check",
			process: &process.ManagedProcess{
				ID:      "test-cmd",
				Command: "echo test",
				PID:     12345,
				Status:  process.StatusRunning,
				HealthCheck: &process.HealthCheck{
					Type:    process.HealthCheckCommand,
					Target:  "echo healthy",
					Enabled: true,
					Timeout: 2 * time.Second,
				},
			},
			expectError: false,
		},
		{
			name: "process_with_disabled_health_check",
			process: &process.ManagedProcess{
				ID:      "test-disabled",
				Command: "echo test",
				PID:     12345,
				Status:  process.StatusRunning,
				HealthCheck: &process.HealthCheck{
					Type:    process.HealthCheckCommand,
					Target:  "echo healthy",
					Enabled: false,
					Timeout: 2 * time.Second,
				},
			},
			expectError: false,
		},
		{
			name: "process_without_health_check",
			process: &process.ManagedProcess{
				ID:      "test-no-hc",
				Command: "echo test",
				PID:     12345,
				Status:  process.StatusRunning,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir, err := os.MkdirTemp("", "portguard-health-check-test")
			require.NoError(t, err)
			defer func() { _ = os.RemoveAll(tempDir) }() // Best effort cleanup during test

			pm := createTestProcessManager(t, tempDir)

			result, err := performHealthCheck(pm, tt.process)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.process.ID, result.ProcessID)
				assert.Equal(t, tt.process.Command, result.Command)
				assert.NotZero(t, result.CheckedAt)
			}
		})
	}
}

// Helper function to create a test process manager with minimal setup
func createTestProcessManager(t *testing.T, tempDir string) *process.ProcessManager {
	t.Helper()

	// Create state file path
	stateFile := filepath.Join(tempDir, "state.json")

	// Create real components for integration testing
	stateStore, err := createTestStateStore(stateFile)
	require.NoError(t, err)

	lockFile := filepath.Join(tempDir, "test.lock")
	lockManager := createTestLockManager(lockFile)
	portScanner := createTestPortScanner()

	pm := process.NewProcessManager(stateStore, lockManager, portScanner)

	return pm
}

// Helper to create test state store
func createTestStateStore(stateFile string) (*state.JSONStore, error) {
	// Use the actual JSON store for more realistic testing
	return state.NewJSONStore(stateFile)
}

// Helper to create test lock manager
func createTestLockManager(lockFile string) *testLockManager {
	// Create a simple mock that always succeeds
	return &testLockManager{}
}

// Helper to create test port scanner
func createTestPortScanner() *testPortScanner {
	return &testPortScanner{}
}

// Simple test implementations
type testLockManager struct{}

func (t *testLockManager) Lock() error    { return nil }
func (t *testLockManager) Unlock() error  { return nil }
func (t *testLockManager) IsLocked() bool { return false }

type testPortScanner struct{}

func (t *testPortScanner) IsPortInUse(port int) bool { return false }
func (t *testPortScanner) GetPortInfo(port int) (*portpkg.PortInfo, error) {
	return &portpkg.PortInfo{Port: port, PID: 0, ProcessName: "", IsManaged: false, Protocol: "tcp"}, nil
}
func (t *testPortScanner) ScanRange(startPort, endPort int) ([]portpkg.PortInfo, error) {
	return []portpkg.PortInfo{}, nil
}
func (t *testPortScanner) FindAvailablePort(startPort int) (int, error) {
	return startPort, nil
}
