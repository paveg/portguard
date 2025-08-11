package process

import (
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessAdopter(t *testing.T) {
	adopter := NewProcessAdopter(5 * time.Second)
	require.NotNil(t, adopter)

	t.Run("adopt_invalid_pid", func(t *testing.T) {
		// Test with invalid PID
		_, err := adopter.AdoptProcessByPID(-1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid PID")
	})

	t.Run("adopt_nonexistent_pid", func(t *testing.T) {
		// Test with very high PID that should not exist
		_, err := adopter.AdoptProcessByPID(999999)
		assert.Error(t, err)
	})

	t.Run("adopt_by_unused_port", func(t *testing.T) {
		// Test adopting by port that's not in use
		_, err := adopter.AdoptProcessByPort(65534) // Use high port unlikely to be in use
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not in use")
	})

	t.Run("get_process_info_invalid_pid", func(t *testing.T) {
		info, err := adopter.GetProcessInfo(-1)
		require.NoError(t, err) // Should not error, but info should indicate not suitable
		assert.False(t, info.IsSuitable)
		assert.NotEmpty(t, info.Reason)
	})

	t.Run("discover_processes_empty_range", func(t *testing.T) {
		processes, err := adopter.DiscoverAdoptableProcesses(PortRange{
			Start: 65000,
			End:   65001,
		})
		assert.NoError(t, err)
		assert.Empty(t, processes) // Should return empty slice for no processes found
	})
}

func TestProcessAdopterHelpers(t *testing.T) {
	adopter := NewProcessAdopter(5 * time.Second)

	t.Run("is_process_running", func(t *testing.T) {
		// Test with current process (should be running)
		currentPID := os.Getpid()
		running := adopter.isProcessRunning(currentPID)
		assert.True(t, running)

		// Test with invalid PID
		running = adopter.isProcessRunning(-1)
		assert.False(t, running)

		// Test with very high PID (should not exist)
		running = adopter.isProcessRunning(999999)
		assert.False(t, running)
	})

	t.Run("get_process_working_dir", func(t *testing.T) {
		currentPID := os.Getpid()

		workingDir, err := adopter.getProcessWorkingDir(currentPID)

		// On Windows, this might not be supported
		if runtime.GOOS == "windows" {
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "not supported on Windows")
		} else {
			// On Unix systems, it should either succeed or fail gracefully
			if err != nil {
				assert.Contains(t, err.Error(), "unable to determine")
			} else {
				assert.NotEmpty(t, workingDir)
			}
		}
	})

	t.Run("evaluate_process_suitability", func(t *testing.T) {
		// Test system process (low PID)
		systemInfo := &AdoptionInfo{
			PID:         1,
			ProcessName: "init",
			Command:     "/sbin/init",
		}
		suitable, reason := adopter.evaluateProcessSuitability(systemInfo)
		assert.False(t, suitable)
		assert.Contains(t, reason, "system process")

		// Test development process
		devInfo := &AdoptionInfo{
			PID:         12345,
			ProcessName: "node",
			Command:     "npm run dev",
		}
		suitable, reason = adopter.evaluateProcessSuitability(devInfo)
		assert.True(t, suitable)
		assert.Contains(t, reason, "development")

		// Test unrecognized process
		unknownInfo := &AdoptionInfo{
			PID:         12345,
			ProcessName: "unknown-process",
			Command:     "/bin/unknown-process",
		}
		suitable, reason = adopter.evaluateProcessSuitability(unknownInfo)
		assert.False(t, suitable)
		assert.Contains(t, reason, "not a recognized")
	})
}

func TestCreateManagedProcessFromAdoption(t *testing.T) {
	adopter := NewProcessAdopter(5 * time.Second)

	t.Run("create_with_port", func(t *testing.T) {
		info := &AdoptionInfo{
			PID:         12345,
			ProcessName: "node",
			Command:     "npm run dev",
			Port:        3000,
			WorkingDir:  "/path/to/project",
			IsSuitable:  true,
		}

		managedProcess, err := adopter.createManagedProcessFromAdoption(info)
		require.NoError(t, err)
		assert.NotNil(t, managedProcess)
		assert.Equal(t, info.PID, managedProcess.PID)
		assert.Equal(t, info.Command, managedProcess.Config.Command)
		assert.Equal(t, info.Port, managedProcess.Config.Port)
		assert.Equal(t, StatusRunning, managedProcess.Status)
		assert.True(t, managedProcess.IsExternal)

		// Should have TCP health check for processes with ports
		assert.NotNil(t, managedProcess.Config.HealthCheck)
		assert.Equal(t, HealthCheckTCP, managedProcess.Config.HealthCheck.Type)
	})

	t.Run("create_without_port", func(t *testing.T) {
		info := &AdoptionInfo{
			PID:         12345,
			ProcessName: "background-service",
			Command:     "/usr/bin/background-service",
			Port:        0, // No port
			IsSuitable:  true,
		}

		managedProcess, err := adopter.createManagedProcessFromAdoption(info)
		require.NoError(t, err)
		assert.NotNil(t, managedProcess)

		// Should have process-based health check for processes without ports
		assert.NotNil(t, managedProcess.Config.HealthCheck)
		assert.Equal(t, HealthCheckProcess, managedProcess.Config.HealthCheck.Type)
	})
}
