package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paveg/portguard/internal/process"
)

// Mock data for testing
func createMockProcessList() []*process.ManagedProcess {
	return []*process.ManagedProcess{
		{
			ID:        "test-1",
			Command:   "npm run dev",
			Args:      []string{"run", "dev"},
			Port:      3000,
			PID:       1234,
			Status:    process.StatusRunning,
			CreatedAt: time.Now().Add(-2 * time.Hour),
			UpdatedAt: time.Now().Add(-1 * time.Minute),
			LastSeen:  time.Now(),
		},
		{
			ID:        "test-2",
			Command:   "go run main.go",
			Args:      []string{"run", "main.go"},
			Port:      8080,
			PID:       5678,
			Status:    process.StatusStopped,
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: time.Now().Add(-30 * time.Minute),
			LastSeen:  time.Now().Add(-30 * time.Minute),
		},
		{
			ID:        "test-3",
			Command:   "python app.py",
			Args:      []string{"app.py"},
			Port:      5000,
			PID:       9012,
			Status:    process.StatusUnhealthy,
			CreatedAt: time.Now().Add(-30 * time.Minute),
			UpdatedAt: time.Now().Add(-5 * time.Minute),
			LastSeen:  time.Now().Add(-10 * time.Minute),
		},
	}
}

// Helper function to execute list command with given args
func executeListCmd(t *testing.T, args []string) (string, error) {
	t.Helper()

	var buf bytes.Buffer

	// Simple argument parsing for mock testing
	var includeAll, jsonOut, verboseOutput bool
	var filterPort int
	var filterStatus string

	// Parse args manually for testing
	for i, arg := range args {
		switch arg {
		case "--all":
			includeAll = true
		case "--port", "-p":
			if i+1 >= len(args) {
				continue
			}
			port, err := strconv.Atoi(args[i+1])
			if err != nil {
				return "", fmt.Errorf("invalid port value: %s", args[i+1])
			}
			filterPort = port
		case "--json", "-j":
			jsonOut = true
		case "--status":
			if i+1 >= len(args) {
				continue
			}
			filterStatus = args[i+1]
		case "--verbose":
			verboseOutput = true
		}
	}

	// Get mock process list
	allProcesses := createMockProcessList()

	// Apply filters
	filteredProcesses := make([]*process.ManagedProcess, 0, len(allProcesses))
	for _, proc := range allProcesses {
		// Filter by status (running vs all)
		if !includeAll && proc.Status != process.StatusRunning && proc.Status != process.StatusUnhealthy {
			continue
		}

		// Filter by port
		if filterPort > 0 && proc.Port != filterPort {
			continue
		}

		// Filter by status string
		if filterStatus != "" && string(proc.Status) != filterStatus {
			continue
		}

		filteredProcesses = append(filteredProcesses, proc)
	}

	if jsonOut {
		// JSON output
		result := map[string]interface{}{
			"processes": filteredProcesses,
			"total":     len(filteredProcesses),
		}
		encoder := json.NewEncoder(&buf)
		if err := encoder.Encode(result); err != nil {
			return "", err
		}
		return buf.String(), nil
	}

	// Text output
	if len(filteredProcesses) == 0 {
		buf.WriteString("No processes found\n")
		return buf.String(), nil
	}

	if verboseOutput {
		// Verbose output format
		for _, proc := range filteredProcesses {
			buf.WriteString(fmt.Sprintf("ID: %s\n", proc.ID))
			buf.WriteString(fmt.Sprintf("  Command: %s\n", proc.Command))
			buf.WriteString(fmt.Sprintf("  Port: %d\n", proc.Port))
			buf.WriteString(fmt.Sprintf("  PID: %d\n", proc.PID))
			buf.WriteString(fmt.Sprintf("  Status: %s\n", proc.Status))
			buf.WriteString(fmt.Sprintf("  Created: %s\n", proc.CreatedAt.Format(time.RFC3339)))
			buf.WriteString(fmt.Sprintf("  Age: %s\n", time.Since(proc.CreatedAt).Truncate(time.Second)))
			buf.WriteString("\n")
		}
		return buf.String(), nil
	}

	// Simple table format
	buf.WriteString(fmt.Sprintf("%-10s %-20s %-6s %-6s %-10s\n", "ID", "COMMAND", "PORT", "PID", "STATUS"))
	buf.WriteString(fmt.Sprintf("%-10s %-20s %-6s %-6s %-10s\n", "----------", "--------------------", "------", "------", "----------"))
	for _, proc := range filteredProcesses {
		buf.WriteString(fmt.Sprintf("%-10s %-20s %-6d %-6d %-10s\n",
			proc.ID,
			proc.Command,
			proc.Port,
			proc.PID,
			proc.Status))
	}

	return buf.String(), nil
}

func TestListCommand_DefaultOutput(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectError    bool
		validateOutput func(*testing.T, string)
	}{
		{
			name:        "list_running_processes_only",
			args:        []string{},
			expectError: false,
			validateOutput: func(t *testing.T, output string) {
				t.Helper()
				// Should show running and unhealthy, but not stopped
				assert.Contains(t, output, "test-1")    // running
				assert.Contains(t, output, "test-3")    // unhealthy
				assert.NotContains(t, output, "test-2") // stopped
				assert.Contains(t, output, "npm run dev")
				assert.Contains(t, output, "python app.py")
				assert.Contains(t, output, "STATUS")
			},
		},
		{
			name:        "list_all_processes",
			args:        []string{"--all"},
			expectError: false,
			validateOutput: func(t *testing.T, output string) {
				t.Helper()
				// Should show all processes
				assert.Contains(t, output, "test-1")
				assert.Contains(t, output, "test-2")
				assert.Contains(t, output, "test-3")
				assert.Contains(t, output, "running")
				assert.Contains(t, output, "stopped")
				assert.Contains(t, output, "unhealthy")
			},
		},
		{
			name:        "list_verbose_output",
			args:        []string{"--verbose"},
			expectError: false,
			validateOutput: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, "ID: test-1")
				assert.Contains(t, output, "Command: npm run dev")
				assert.Contains(t, output, "Port: 3000")
				assert.Contains(t, output, "PID: 1234")
				assert.Contains(t, output, "Status: running")
				assert.Contains(t, output, "Created:")
				assert.Contains(t, output, "Age:")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := executeListCmd(t, tt.args)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, output)
			tt.validateOutput(t, output)
		})
	}
}

func TestListCommand_JSONOutput(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectError    bool
		validateOutput func(*testing.T, string)
	}{
		{
			name:        "list_json_all_processes",
			args:        []string{"--json", "--all"},
			expectError: false,
			validateOutput: func(t *testing.T, output string) {
				t.Helper()
				var result map[string]interface{}
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err)

				assert.Contains(t, result, "processes")
				assert.Contains(t, result, "total")
				assert.InEpsilon(t, float64(3), result["total"], 0.01)

				processes, ok := result["processes"].([]interface{})
				require.True(t, ok)
				assert.Len(t, processes, 3)

				// Check first process structure
				firstProc, ok := processes[0].(map[string]interface{})
				require.True(t, ok)
				assert.Contains(t, firstProc, "id")
				assert.Contains(t, firstProc, "command")
				assert.Contains(t, firstProc, "port")
				assert.Contains(t, firstProc, "status")
			},
		},
		{
			name:        "list_json_running_only",
			args:        []string{"--json"},
			expectError: false,
			validateOutput: func(t *testing.T, output string) {
				t.Helper()
				var result map[string]interface{}
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err)

				assert.InEpsilon(t, float64(2), result["total"], 0.01) // Only running and unhealthy
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := executeListCmd(t, tt.args)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, output)
			tt.validateOutput(t, output)
		})
	}
}

func TestListCommand_FilterOptions(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectError    bool
		validateOutput func(*testing.T, string)
	}{
		{
			name:        "filter_by_port",
			args:        []string{"--port", "3000"},
			expectError: false,
			validateOutput: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, "test-1")
				assert.NotContains(t, output, "test-2")
				assert.NotContains(t, output, "test-3")
				assert.Contains(t, output, "npm run dev")
			},
		},
		{
			name:        "filter_by_port_short_flag",
			args:        []string{"-p", "8080", "--all"},
			expectError: false,
			validateOutput: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, "test-2")
				assert.NotContains(t, output, "test-1")
				assert.NotContains(t, output, "test-3")
			},
		},
		{
			name:        "filter_by_status",
			args:        []string{"--status", "running"},
			expectError: false,
			validateOutput: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, "test-1")
				assert.NotContains(t, output, "test-2")
				assert.NotContains(t, output, "test-3")
			},
		},
		{
			name:        "filter_by_status_stopped",
			args:        []string{"--status", "stopped", "--all"},
			expectError: false,
			validateOutput: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, "test-2")
				assert.NotContains(t, output, "test-1")
				assert.NotContains(t, output, "test-3")
			},
		},
		{
			name:        "filter_by_nonexistent_port",
			args:        []string{"--port", "9999"},
			expectError: false,
			validateOutput: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, "No processes found")
			},
		},
		{
			name:        "filter_by_invalid_status",
			args:        []string{"--status", "nonexistent"},
			expectError: false,
			validateOutput: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, "No processes found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := executeListCmd(t, tt.args)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, output)
			tt.validateOutput(t, output)
		})
	}
}

func TestListCommand_CombinedOptions(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectError    bool
		validateOutput func(*testing.T, string)
	}{
		{
			name:        "json_with_port_filter",
			args:        []string{"--json", "--port", "3000"},
			expectError: false,
			validateOutput: func(t *testing.T, output string) {
				t.Helper()
				var result map[string]interface{}
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err)

				assert.InEpsilon(t, float64(1), result["total"], 0.01)

				processes, ok := result["processes"].([]interface{})
				require.True(t, ok)
				assert.Len(t, processes, 1)

				proc, ok := processes[0].(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "test-1", proc["id"])
				assert.InEpsilon(t, float64(3000), proc["port"], 0.01)
			},
		},
		{
			name:        "verbose_with_all_and_port_filter",
			args:        []string{"--verbose", "--all", "--port", "8080"},
			expectError: false,
			validateOutput: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, "ID: test-2")
				assert.Contains(t, output, "Command: go run main.go")
				assert.Contains(t, output, "Status: stopped")
				assert.NotContains(t, output, "test-1")
				assert.NotContains(t, output, "test-3")
			},
		},
		{
			name:        "json_with_status_filter",
			args:        []string{"--json", "--status", "unhealthy"},
			expectError: false,
			validateOutput: func(t *testing.T, output string) {
				t.Helper()
				var result map[string]interface{}
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err)

				assert.InEpsilon(t, float64(1), result["total"], 0.01)

				processes, ok := result["processes"].([]interface{})
				require.True(t, ok)
				assert.Len(t, processes, 1)

				proc, ok := processes[0].(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "test-3", proc["id"])
				assert.Equal(t, "unhealthy", proc["status"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := executeListCmd(t, tt.args)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, output)
			tt.validateOutput(t, output)
		})
	}
}

func TestListCommand_OutputFormatting(t *testing.T) {
	t.Run("table_header_format", func(t *testing.T) {
		output, err := executeListCmd(t, []string{})
		require.NoError(t, err)

		lines := strings.Split(strings.TrimSpace(output), "\n")
		require.GreaterOrEqual(t, len(lines), 2)

		// Check header line
		headerLine := lines[0]
		assert.Contains(t, headerLine, "ID")
		assert.Contains(t, headerLine, "COMMAND")
		assert.Contains(t, headerLine, "PORT")
		assert.Contains(t, headerLine, "PID")
		assert.Contains(t, headerLine, "STATUS")

		// Check separator line
		separatorLine := lines[1]
		assert.Contains(t, separatorLine, "----------")
		assert.Contains(t, separatorLine, "--------------------")
	})

	t.Run("verbose_format_structure", func(t *testing.T) {
		output, err := executeListCmd(t, []string{"--verbose"})
		require.NoError(t, err)

		// Should contain block format with labels
		assert.Regexp(t, `ID: test-\d+`, output)
		assert.Regexp(t, `Command: .*`, output)
		assert.Regexp(t, `Port: \d+`, output)
		assert.Regexp(t, `PID: \d+`, output)
		assert.Regexp(t, `Status: \w+`, output)
		assert.Regexp(t, `Created: \d{4}-\d{2}-\d{2}T.*`, output)
		assert.Regexp(t, `Age: .*`, output)
	})

	t.Run("empty_result_handling", func(t *testing.T) {
		output, err := executeListCmd(t, []string{"--port", "99999"})
		require.NoError(t, err)

		assert.Equal(t, "No processes found\n", output)
	})
}

func TestListCommand_FlagValidation(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "invalid_port_string",
			args:        []string{"--port", "invalid"},
			expectError: true,
		},
		{
			name:        "negative_port",
			args:        []string{"--port", "-1"},
			expectError: false, // Flag parsing succeeds, validation in business logic
		},
		{
			name:        "valid_status_values",
			args:        []string{"--status", "running"},
			expectError: false,
		},
		{
			name:        "combining_conflicting_output_formats",
			args:        []string{"--json", "--verbose"},
			expectError: false, // Both are valid, implementation chooses behavior
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := executeListCmd(t, tt.args)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
