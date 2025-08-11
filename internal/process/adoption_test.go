package process

import (
	"encoding/json"
	"os"
	"runtime"
	"strings"
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

	t.Run("create_with_working_dir", func(t *testing.T) {
		info := &AdoptionInfo{
			PID:         67890,
			ProcessName: "python",
			Command:     "flask run --host=0.0.0.0 --port=5000",
			Port:        5000,
			WorkingDir:  "/app",
			IsSuitable:  true,
		}

		managedProcess, err := adopter.createManagedProcessFromAdoption(info)
		require.NoError(t, err)
		assert.NotNil(t, managedProcess)
		assert.Equal(t, info.WorkingDir, managedProcess.Config.WorkingDir)
		assert.Contains(t, managedProcess.Config.ID, "adopted-67890")
		assert.NotZero(t, managedProcess.StartedAt)
	})
}

func TestAdoptProcessByPortWithRealScenarios(t *testing.T) {
	adopter := NewProcessAdopter(5 * time.Second)

	t.Run("adopt_by_port_missing_process", func(t *testing.T) {
		// Test adopting by port when no process info is available
		_, err := adopter.AdoptProcessByPort(65535) // Use very high port unlikely to be in use
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not in use")
	})

	t.Run("adopt_by_port_scanner_error", func(t *testing.T) {
		// Test with invalid port to trigger scanner error
		_, err := adopter.AdoptProcessByPort(-1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not in use")
	})
}

func TestDiscoverAdoptableProcessesExtended(t *testing.T) {
	adopter := NewProcessAdopter(5 * time.Second)

	t.Run("discover_with_valid_range", func(t *testing.T) {
		// Test discovery in a range where we don't expect processes
		portRange := PortRange{Start: 65000, End: 65010}
		processes, err := adopter.DiscoverAdoptableProcesses(portRange)
		assert.NoError(t, err)
		assert.Empty(t, processes)
		// In most cases, should be empty since high port range unlikely to have dev servers
	})

	t.Run("discover_with_invalid_range", func(t *testing.T) {
		// Test with invalid port range
		portRange := PortRange{Start: -1, End: 1000}
		_, err := adopter.DiscoverAdoptableProcesses(portRange)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to discover development servers")
	})
}

func TestEvaluateProcessSuitabilityComprehensive(t *testing.T) {
	adopter := NewProcessAdopter(5 * time.Second)

	testCases := []struct {
		name           string
		info           *AdoptionInfo
		expectedSuitable bool
		expectedReason   string
	}{
		{
			name: "node_development_server",
			info: &AdoptionInfo{
				PID:         5000,
				ProcessName: "node",
				Command:     "npm run dev",
			},
			expectedSuitable: true,
			expectedReason:   "development server detected",
		},
		{
			name: "python_flask_server",
			info: &AdoptionInfo{
				PID:         5001,
				ProcessName: "python",
				Command:     "flask run --host=0.0.0.0",
			},
			expectedSuitable: true,
			expectedReason:   "development server detected",
		},
		{
			name: "go_development_with_air",
			info: &AdoptionInfo{
				PID:         5002,
				ProcessName: "air",
				Command:     "air -c .air.toml",
			},
			expectedSuitable: true,
			expectedReason:   "development server detected",
		},
		{
			name: "development_command_by_args",
			info: &AdoptionInfo{
				PID:         5003,
				ProcessName: "custom-server",
				Command:     "custom-server serve --watch",
			},
			expectedSuitable: true,
			expectedReason:   "development command detected",
		},
		{
			name: "yarn_dev_server",
			info: &AdoptionInfo{
				PID:         5004,
				ProcessName: "yarn",
				Command:     "yarn start",
			},
			expectedSuitable: true,
			expectedReason:   "development server detected",
		},
		{
			name: "system_process_low_pid_unix",
			info: &AdoptionInfo{
				PID:         100,
				ProcessName: "systemd",
				Command:     "/lib/systemd/systemd",
			},
			expectedSuitable: false,
			expectedReason:   "system process",
		},
		{
			name: "unrecognized_process",
			info: &AdoptionInfo{
				PID:         5005,
				ProcessName: "unknown-app",
				Command:     "/bin/unknown-app --config=/etc/app.conf",
			},
			expectedSuitable: false,
			expectedReason:   "not a recognized",
		},
		{
			name: "java_spring_boot",
			info: &AdoptionInfo{
				PID:         5006,
				ProcessName: "java",
				Command:     "java -jar spring-boot-app.jar",
			},
			expectedSuitable: true,
			expectedReason:   "development server detected",
		},
		{
			name: "dotnet_kestrel",
			info: &AdoptionInfo{
				PID:         5007,
				ProcessName: "dotnet",
				Command:     "dotnet run --urls http://localhost:5000",
			},
			expectedSuitable: true,
			expectedReason:   "development server detected",
		},
		{
			name: "ruby_rails_server",
			info: &AdoptionInfo{
				PID:         5008,
				ProcessName: "ruby",
				Command:     "rails server",
			},
			expectedSuitable: true,
			expectedReason:   "development server detected",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			suitable, reason := adopter.evaluateProcessSuitability(tc.info)
			assert.Equal(t, tc.expectedSuitable, suitable, "Suitability mismatch for %s", tc.name)
			assert.Contains(t, reason, tc.expectedReason, "Reason mismatch for %s", tc.name)
		})
	}
}

func TestIsProcessRunningWindowsSpecific(t *testing.T) {
	adopter := NewProcessAdopter(5 * time.Second)

	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	t.Run("windows_process_running_check", func(t *testing.T) {
		// Test current process (should be running)
		currentPID := os.Getpid()
		running := adopter.isProcessRunningWindows(currentPID)
		assert.True(t, running)

		// Test with very high PID (should not exist)
		running = adopter.isProcessRunningWindows(999999)
		assert.False(t, running)
	})
}

func TestGetProcessInfoEdgeCases(t *testing.T) {
	adopter := NewProcessAdopter(2 * time.Second)

	t.Run("get_info_for_current_process", func(t *testing.T) {
		// Test getting info for current process (should work)
		currentPID := os.Getpid()
		info, err := adopter.GetProcessInfo(currentPID)
		assert.NoError(t, err)
		assert.NotNil(t, info)
		assert.Equal(t, currentPID, info.PID)
		// The result will depend on the actual process name but should not be empty
		assert.NotEmpty(t, info.ProcessName)
	})

	t.Run("get_info_for_invalid_pid", func(t *testing.T) {
		// Test with invalid PID
		info, err := adopter.GetProcessInfo(-5)
		assert.NoError(t, err) // Should not error, but process should be unsuitable
		assert.NotNil(t, info)
		assert.False(t, info.IsSuitable)
		assert.NotEmpty(t, info.Reason)
		assert.Contains(t, info.Reason, "failed to get process info")
	})

	t.Run("get_info_zero_pid", func(t *testing.T) {
		// Test with PID 0 (special case)
		info, err := adopter.GetProcessInfo(0)
		assert.NoError(t, err)
		assert.NotNil(t, info)
		assert.False(t, info.IsSuitable)
	})
}

func TestAdoptProcessByPIDErrorConditions(t *testing.T) {
	adopter := NewProcessAdopter(2 * time.Second)

	t.Run("adopt_zero_pid", func(t *testing.T) {
		_, err := adopter.AdoptProcessByPID(0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid PID")
	})

	t.Run("adopt_negative_pid", func(t *testing.T) {
		_, err := adopter.AdoptProcessByPID(-10)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid PID")
	})

	t.Run("adopt_nonexistent_pid_large", func(t *testing.T) {
		// Test with very large PID that should not exist
		_, err := adopter.AdoptProcessByPID(999999)
		assert.Error(t, err)
		// Could be process not found or process not running
		assert.True(t, 
			strings.Contains(err.Error(), "not found") || 
			strings.Contains(err.Error(), "no longer running"),
			"Expected 'not found' or 'no longer running' error, got: %v", err)
	})
}

func TestProcessAdopterErrorVariables(t *testing.T) {
	t.Run("error_variables_defined", func(t *testing.T) {
		// Test that error variables are properly defined
		assert.NotNil(t, ErrProcessNotSuitable)
		assert.NotNil(t, ErrSystemProcess)
		assert.NotNil(t, ErrInsufficientPerms)
		assert.NotNil(t, ErrProcessAlreadyDead)

		// Test error messages
		assert.Contains(t, ErrProcessNotSuitable.Error(), "not suitable")
		assert.Contains(t, ErrSystemProcess.Error(), "system process")
		assert.Contains(t, ErrInsufficientPerms.Error(), "permissions")
		assert.Contains(t, ErrProcessAlreadyDead.Error(), "no longer running")
	})
}

func TestAdoptionInfoStructure(t *testing.T) {
	t.Run("adoption_info_json_tags", func(t *testing.T) {
		// Test that AdoptionInfo struct can be marshaled to JSON
		info := &AdoptionInfo{
			PID:         1234,
			ProcessName: "test-process",
			Command:     "test-command --arg",
			Port:        8080,
			WorkingDir:  "/test/dir",
			IsSuitable:  true,
			Reason:      "test reason",
		}

		// Should be able to marshal to JSON without issues
		jsonData, err := json.Marshal(info)
		assert.NoError(t, err)
		assert.Contains(t, string(jsonData), "test-process")
		assert.Contains(t, string(jsonData), "8080")

		// Should be able to unmarshal back
		var unmarshaled AdoptionInfo
		err = json.Unmarshal(jsonData, &unmarshaled)
		assert.NoError(t, err)
		assert.Equal(t, info.PID, unmarshaled.PID)
		assert.Equal(t, info.ProcessName, unmarshaled.ProcessName)
		assert.Equal(t, info.IsSuitable, unmarshaled.IsSuitable)
	})

	t.Run("adoption_info_omitempty", func(t *testing.T) {
		// Test omitempty behavior for optional fields
		info := &AdoptionInfo{
			PID:         1234,
			ProcessName: "test-process",
			Command:     "test-command",
			// Port: 0 - should be omitted
			// WorkingDir: "" - should be omitted
			IsSuitable: false,
			// Reason: "" - should be omitted
		}

		jsonData, err := json.Marshal(info)
		assert.NoError(t, err)

		jsonString := string(jsonData)
		// Port should be omitted when 0
		assert.NotContains(t, jsonString, "port")
		// WorkingDir should be omitted when empty
		assert.NotContains(t, jsonString, "working_dir")
		// Reason should be omitted when empty
		assert.NotContains(t, jsonString, "reason")
	})
}
