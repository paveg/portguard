package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paveg/portguard/internal/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartCommand_Structure(t *testing.T) {
	t.Run("command_has_correct_metadata", func(t *testing.T) {
		assert.Equal(t, "start <command|project>", startCmd.Use)
		assert.Equal(t, "Start a new process or reuse existing one", startCmd.Short)
		assert.NotNil(t, startCmd.RunE)
		assert.NotNil(t, startCmd.Args)
	})

	t.Run("command_has_required_flags", func(t *testing.T) {
		// Check for port flag
		portFlag := startCmd.Flags().Lookup("port")
		require.NotNil(t, portFlag)
		assert.Equal(t, "p", portFlag.Shorthand)

		// Check for background flag
		bgFlag := startCmd.Flags().Lookup("background")
		require.NotNil(t, bgFlag)
		assert.Equal(t, "b", bgFlag.Shorthand)

		// Check for health-check flag
		healthFlag := startCmd.Flags().Lookup("health-check")
		require.NotNil(t, healthFlag)
	})
}

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  []string
		expectErr bool
	}{
		{
			name:     "simple_command",
			input:    "echo hello",
			expected: []string{"echo", "hello"},
		},
		{
			name:     "command_with_flags",
			input:    "npm run dev --port 3000",
			expected: []string{"npm", "run", "dev", "--port", "3000"},
		},
		{
			name:     "command_with_extra_spaces",
			input:    "  echo   hello   world  ",
			expected: []string{"echo", "hello", "world"},
		},
		{
			name:     "single_command",
			input:    "ls",
			expected: []string{"ls"},
		},
		{
			name:      "empty_command",
			input:     "",
			expectErr: true,
		},
		{
			name:      "whitespace_only",
			input:     "   ",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseCommand(tt.input)

			if tt.expectErr {
				require.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestParseHealthCheck(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *process.HealthCheck
	}{
		{
			name:  "http_health_check",
			input: "http://localhost:3000/health",
			expected: &process.HealthCheck{
				Type:   process.HealthCheckHTTP,
				Target: "http://localhost:3000/health",
			},
		},
		{
			name:  "https_health_check",
			input: "https://api.example.com/status",
			expected: &process.HealthCheck{
				Type:   process.HealthCheckHTTP,
				Target: "https://api.example.com/status",
			},
		},
		{
			name:  "tcp_health_check",
			input: "localhost:8080",
			expected: &process.HealthCheck{
				Type:   process.HealthCheckTCP,
				Target: "localhost:8080",
			},
		},
		{
			name:  "ip_tcp_health_check",
			input: "127.0.0.1:9000",
			expected: &process.HealthCheck{
				Type:   process.HealthCheckTCP,
				Target: "127.0.0.1:9000",
			},
		},
		{
			name:  "command_health_check",
			input: "curl -f http://localhost:3000/ping",
			expected: &process.HealthCheck{
				Type:   process.HealthCheckCommand,
				Target: "curl -f http://localhost:3000/ping",
			},
		},
		{
			name:  "simple_command_health_check",
			input: "ping localhost",
			expected: &process.HealthCheck{
				Type:   process.HealthCheckCommand,
				Target: "ping localhost",
			},
		},
		{
			name:     "empty_health_check",
			input:    "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseHealthCheck(tt.input)
			require.NoError(t, err)

			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, tt.expected.Type, result.Type)
				assert.Equal(t, tt.expected.Target, result.Target)
			}
		})
	}
}

func TestInitializeProcessManager(t *testing.T) {
	// Create a temporary directory for this test
	tempDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	defer func() { _ = os.Setenv("HOME", oldHome) }() // Restore HOME directory
	_ = os.Setenv("HOME", tempDir)                    // Set temporary HOME for test

	t.Run("creates_process_manager_successfully", func(t *testing.T) {
		pm, err := initializeProcessManager()

		require.NoError(t, err)
		assert.NotNil(t, pm)

		// Check that directories were created
		portguardDir := filepath.Join(tempDir, ".portguard")
		assert.DirExists(t, portguardDir)
	})

	t.Run("handles_existing_directory", func(t *testing.T) {
		// Create the directory first
		portguardDir := filepath.Join(tempDir, ".portguard")
		err := os.MkdirAll(portguardDir, 0o755)
		require.NoError(t, err)

		pm, err := initializeProcessManager()

		require.NoError(t, err)
		assert.NotNil(t, pm)
	})
}

func TestHealthCheckTypes(t *testing.T) {
	t.Run("health_check_type_constants", func(t *testing.T) {
		// Ensure the constants we're using exist
		assert.Equal(t, process.HealthCheckHTTP, process.HealthCheckType("http"))
		assert.Equal(t, process.HealthCheckTCP, process.HealthCheckType("tcp"))
		assert.Equal(t, process.HealthCheckCommand, process.HealthCheckType("command"))
	})
}

func TestStartOptions_Structure(t *testing.T) {
	t.Run("start_options_can_be_created", func(t *testing.T) {
		options := process.StartOptions{
			Port:       3000,
			Background: true,
			HealthCheck: &process.HealthCheck{
				Type:   process.HealthCheckHTTP,
				Target: "http://localhost:3000/health",
			},
		}

		assert.Equal(t, 3000, options.Port)
		assert.True(t, options.Background)
		assert.NotNil(t, options.HealthCheck)
		assert.Equal(t, process.HealthCheckHTTP, options.HealthCheck.Type)
		assert.Equal(t, "http://localhost:3000/health", options.HealthCheck.Target)
	})
}
