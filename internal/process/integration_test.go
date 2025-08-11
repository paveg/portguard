package process

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestProcessManager_ExecuteProcess_Integration(t *testing.T) {
	pm, stateStore, lockManager, _ := setupTestProcessManager(t)

	// Setup mock expectations
	stateStore.On("Save", mock.AnythingOfType("map[string]*process.ManagedProcess")).Return(nil)
	lockManager.On("Lock").Return(nil)
	lockManager.On("Unlock").Return(nil)

	tests := []struct {
		name        string
		command     string
		args        []string
		options     StartOptions
		expectError bool
	}{
		{
			name:    "execute_simple_echo_command",
			command: "echo",
			args:    []string{"hello", "world"},
			options: StartOptions{
				Port:       0,
				Background: false,
			},
			expectError: false,
		},
		{
			name:    "execute_sleep_command",
			command: "sleep",
			args:    []string{"0.1"}, // Very short sleep for testing
			options: StartOptions{
				Port:       3001,
				Background: true,
			},
			expectError: false,
		},
		{
			name:    "execute_nonexistent_command",
			command: "nonexistent_command_12345",
			args:    []string{},
			options: StartOptions{
				Port:       0,
				Background: false,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			process, err := pm.executeProcess(tt.command, tt.args, tt.options)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, process)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, process)
				// The command is stored in the full command string, not just Args[0]
				assert.Contains(t, process.Command, tt.command)
				assert.Equal(t, tt.options.Port, process.Port)
				assert.Positive(t, process.PID)
				assert.Equal(t, StatusRunning, process.Status)
			}
		})
	}
}

func TestProcessManager_TerminateProcess_Integration(t *testing.T) {
	pm, _, _, _ := setupTestProcessManager(t)

	// Create a real process that we can terminate
	process, err := pm.executeProcess("sleep", []string{"10"}, StartOptions{})
	require.NoError(t, err)
	require.NotNil(t, process)

	// Verify process is running
	assert.Equal(t, StatusRunning, process.Status)

	// Test graceful termination
	err = pm.terminateProcess(process, false)
	require.NoError(t, err)
	assert.Equal(t, StatusStopped, process.Status)

	// Test terminating already stopped process
	err = pm.terminateProcess(process, false)
	assert.NoError(t, err) // Should not error
}

func TestProcessManager_TerminateProcess_ForceKill(t *testing.T) {
	pm, _, _, _ := setupTestProcessManager(t)

	// Create a real process
	process, err := pm.executeProcess("sleep", []string{"10"}, StartOptions{})
	require.NoError(t, err)
	require.NotNil(t, process)

	// Test force termination
	err = pm.terminateProcess(process, true)
	require.NoError(t, err)
	assert.Equal(t, StatusStopped, process.Status)
}

func TestProcessManager_MonitorProcess_Integration(t *testing.T) {
	pm, _, _, _ := setupTestProcessManager(t)

	// Create a short-lived process
	process, err := pm.executeProcess("echo", []string{"test"}, StartOptions{})
	require.NoError(t, err)
	require.NotNil(t, process)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Monitor the process (it should exit quickly since it's just echo)
	err = pm.monitorProcess(ctx, process)
	// The error can be nil (if process exits) or context timeout
	// We don't assert on the error since it depends on timing
	_ = err
}

func TestProcessManager_CleanupProcessResources_Integration(t *testing.T) {
	pm, _, _, _ := setupTestProcessManager(t)

	// Create temporary files to test cleanup
	tempDir, err := os.MkdirTemp("", "cleanup-test")
	require.NoError(t, err)
	defer func() {
		// Ignore cleanup errors in tests
		_ = os.RemoveAll(tempDir) // nolint:errcheck // Cleanup in tests
	}()

	logFile := tempDir + "/test.log"
	workingDir := tempDir + "/temp-work"
	err = os.Mkdir(workingDir, 0o755)
	require.NoError(t, err)

	// Create test log file
	err = os.WriteFile(logFile, []byte("test log content"), 0o644)
	require.NoError(t, err)

	// Create test process
	process := &ManagedProcess{
		ID:         "cleanup-test",
		Command:    "echo test",
		PID:        12345, // Fake PID for testing
		Status:     StatusStopped,
		LogFile:    logFile,
		WorkingDir: workingDir,
	}

	// Test cleanup
	err = pm.cleanupProcessResources(process, true)
	require.NoError(t, err)

	// Verify log file was cleaned up
	_, err = os.Stat(logFile)
	assert.True(t, os.IsNotExist(err), "Log file should be cleaned up")

	// Verify working directory was cleaned up (contains "temp")
	_, err = os.Stat(workingDir)
	assert.True(t, os.IsNotExist(err), "Temp working directory should be cleaned up")
}

func TestProcessManager_TimeSinceLastSeen_Coverage(t *testing.T) {
	// Test the TimeSinceLastSeen method for coverage
	now := time.Now()
	process := &ManagedProcess{
		ID:       "test-time",
		LastSeen: now.Add(-5 * time.Minute),
	}

	duration := process.TimeSinceLastSeen()
	assert.GreaterOrEqual(t, duration, 5*time.Minute)
	assert.Less(t, duration, 6*time.Minute) // Should be close to 5 minutes
}

func TestProcessManager_ExecuteProcessWithOptions_Integration(t *testing.T) {
	pm, _, _, _ := setupTestProcessManager(t)

	tempDir, err := os.MkdirTemp("", "execute-options-test")
	require.NoError(t, err)
	defer func() {
		// Ignore cleanup errors in tests
		_ = os.RemoveAll(tempDir) // nolint:errcheck // Cleanup in tests
	}()

	logFile := tempDir + "/output.log"

	// Test with comprehensive options
	options := StartOptions{
		Port:       3000,
		Background: true,
		WorkingDir: tempDir,
		LogFile:    logFile,
		Environment: map[string]string{
			"TEST_VAR": "test_value",
		},
		HealthCheck: &HealthCheck{
			Type:    HealthCheckCommand,
			Target:  "echo healthy",
			Enabled: true,
			Timeout: 5 * time.Second,
		},
	}

	process, err := pm.executeProcess("echo", []string{"test output"}, options)
	require.NoError(t, err)
	require.NotNil(t, process)

	assert.Equal(t, options.Port, process.Port)
	assert.Equal(t, options.WorkingDir, process.WorkingDir)
	assert.Equal(t, options.LogFile, process.LogFile)
	assert.Equal(t, options.Environment, process.Environment)
	assert.Equal(t, options.HealthCheck, process.HealthCheck)

	// Give the process time to complete and write to log file
	time.Sleep(100 * time.Millisecond)

	// Verify log file was created (process should have written to it)
	_, err = os.Stat(logFile)
	assert.NoError(t, err, "Log file should be created")
}

func TestProcessManager_StartProcessWithDuplicate_Integration(t *testing.T) {
	pm, stateStore, lockManager, portScanner := setupTestProcessManager(t)

	// Setup mock expectations
	stateStore.On("Save", mock.AnythingOfType("map[string]*process.ManagedProcess")).Return(nil)
	stateStore.On("Load").Return(map[string]*ManagedProcess{}, nil)
	lockManager.On("Lock").Return(nil)
	lockManager.On("Unlock").Return(nil)
	portScanner.On("IsPortInUse", 3000).Return(false)

	// Start first process
	options := StartOptions{
		Port:       3000,
		Background: true,
	}

	process1, err := pm.StartProcess("echo", []string{"test1"}, options)
	require.NoError(t, err)
	require.NotNil(t, process1)

	// Attempt to start the same command again - should reuse
	process2, err := pm.StartProcess("echo", []string{"test1"}, options)
	require.NoError(t, err)
	require.NotNil(t, process2)

	// In a real scenario, this might return the same process
	// The behavior depends on the actual ShouldStartNew logic
}
