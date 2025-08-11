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

func TestHandleSingleProcessStatus(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "portguard-status-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Setup process manager
	pm := createStatusTestProcessManager(t, tempDir)

	// Test with non-existent process
	err = handleSingleProcessStatus(pm, "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestHandleSystemStatus(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "portguard-system-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Setup process manager
	pm := createStatusTestProcessManager(t, tempDir)

	// Test handleSystemStatus - should work even with no processes
	err = handleSystemStatus(pm)
	assert.NoError(t, err)
}

func TestConvertToProcessStatus(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "portguard-convert-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create scanner
	scanner := portpkg.NewScanner(5 * time.Second)

	tests := []struct {
		name    string
		process *process.ManagedProcess
	}{
		{
			name: "running_process",
			process: &process.ManagedProcess{
				ID:        "test-running",
				Command:   "echo test",
				PID:       12345,
				Port:      8080,
				Status:    process.StatusRunning,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
				LastSeen:  time.Now(),
			},
		},
		{
			name: "stopped_process",
			process: &process.ManagedProcess{
				ID:        "test-stopped",
				Command:   "echo test",
				PID:       12346,
				Port:      8081,
				Status:    process.StatusStopped,
				CreatedAt: time.Now().Add(-1 * time.Hour),
				UpdatedAt: time.Now().Add(-30 * time.Minute),
				LastSeen:  time.Now().Add(-30 * time.Minute),
			},
		},
		{
			name: "failed_process",
			process: &process.ManagedProcess{
				ID:        "test-failed",
				Command:   "echo test",
				PID:       12347,
				Port:      8082,
				Status:    process.StatusFailed,
				CreatedAt: time.Now().Add(-2 * time.Hour),
				UpdatedAt: time.Now().Add(-1 * time.Hour),
				LastSeen:  time.Now().Add(-1 * time.Hour),
			},
		},
		{
			name: "process_with_health_check",
			process: &process.ManagedProcess{
				ID:      "test-health",
				Command: "echo test",
				PID:     12348,
				Port:    8083,
				Status:  process.StatusRunning,
				HealthCheck: &process.HealthCheck{
					Type:    process.HealthCheckHTTP,
					Target:  "http://localhost:8083/health",
					Enabled: true,
					Timeout: 5 * time.Second,
				},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
				LastSeen:  time.Now(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := convertToProcessStatus(tt.process, scanner)

			assert.Equal(t, tt.process.ID, status.ID)
			assert.Equal(t, tt.process.Command, status.Command)
			assert.Equal(t, tt.process.PID, status.PID)
			assert.Equal(t, tt.process.Port, status.Port)
			assert.Equal(t, string(tt.process.Status), status.Status)
			assert.Equal(t, tt.process.CreatedAt, status.CreatedAt)
			assert.Equal(t, tt.process.UpdatedAt, status.UpdatedAt)
			assert.NotEmpty(t, status.Uptime)

			if tt.process.HealthCheck != nil {
				assert.NotNil(t, status.HealthCheck)
				assert.Equal(t, tt.process.HealthCheck.Type, status.HealthCheck.Type)
				assert.Equal(t, tt.process.HealthCheck.Target, status.HealthCheck.Target)
				assert.Equal(t, tt.process.HealthCheck.Enabled, status.HealthCheck.Enabled)
			}
		})
	}
}

func TestStatusCommandWithJSONOutput(t *testing.T) {
	t.Run("single_process_json", func(t *testing.T) {
		// Create temporary directory for test
		tempDir, err := os.MkdirTemp("", "portguard-json-test")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		// Setup process manager
		pm := createStatusTestProcessManager(t, tempDir)

		// Set JSON output flag
		jsonOutput = true
		defer func() { jsonOutput = false }()

		// Test with non-existent process (will error but exercises the JSON path)
		err = handleSingleProcessStatus(pm, "test-process")
		require.Error(t, err)
	})

	t.Run("system_status_json", func(t *testing.T) {
		// Create temporary directory for test
		tempDir, err := os.MkdirTemp("", "portguard-system-json-test")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		// Setup process manager
		pm := createStatusTestProcessManager(t, tempDir)

		// Set JSON output flag
		jsonOutput = true
		defer func() { jsonOutput = false }()

		// Test system status
		err = handleSystemStatus(pm)
		assert.NoError(t, err)
	})
}

func TestStatusCommandEdgeCases(t *testing.T) {
	t.Run("status_with_nil_health_check", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "portguard-nil-health-test")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		scanner := portpkg.NewScanner(5 * time.Second)

		managedProcess := &process.ManagedProcess{
			ID:          "test-nil-health",
			Command:     "echo test",
			PID:         12349,
			Status:      process.StatusRunning,
			HealthCheck: nil,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			LastSeen:    time.Now(),
		}

		status := convertToProcessStatus(managedProcess, scanner)
		assert.Nil(t, status.HealthCheck)
		assert.Nil(t, status.PortInfo) // No port specified, so no PortInfo
	})

	t.Run("status_with_port_info", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "portguard-port-info-test")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		scanner := portpkg.NewScanner(5 * time.Second)

		managedProcess := &process.ManagedProcess{
			ID:        "test-port-info",
			Command:   "echo test",
			PID:       12350,
			Port:      8084,
			Status:    process.StatusRunning,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			LastSeen:  time.Now(),
		}

		status := convertToProcessStatus(managedProcess, scanner)
		assert.NotNil(t, status.PortInfo)
		// Port info will be populated based on scanner results
	})
}

// Helper function to create a test process manager for status commands
func createStatusTestProcessManager(t *testing.T, tempDir string) *process.ProcessManager {
	t.Helper()

	// Create state file path
	stateFile := filepath.Join(tempDir, "state.json")

	// Create real components
	stateStore, err := state.NewJSONStore(stateFile)
	require.NoError(t, err)

	lockManager := &testStatusLockManager{}
	portScanner := &testStatusPortScanner{}

	pm := process.NewProcessManager(stateStore, lockManager, portScanner)

	return pm
}

// Simple test implementations for status tests
type testStatusLockManager struct{}

func (t *testStatusLockManager) Lock() error    { return nil }
func (t *testStatusLockManager) Unlock() error  { return nil }
func (t *testStatusLockManager) IsLocked() bool { return false }

type testStatusPortScanner struct{}

func (t *testStatusPortScanner) IsPortInUse(port int) bool { return false }
func (t *testStatusPortScanner) GetPortInfo(port int) (*portpkg.PortInfo, error) {
	return &portpkg.PortInfo{
		Port:        port,
		PID:         0,
		ProcessName: "",
		IsManaged:   false,
		Protocol:    "tcp",
	}, nil
}
func (t *testStatusPortScanner) ScanRange(startPort, endPort int) ([]portpkg.PortInfo, error) {
	return []portpkg.PortInfo{}, nil
}
func (t *testStatusPortScanner) FindAvailablePort(startPort int) (int, error) {
	return startPort, nil
}
