package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMainFunction(t *testing.T) {
	// Test main function by running it as a separate process
	if os.Getenv("BE_MAIN") == "1" {
		main()
		return
	}

	tests := []struct {
		name           string
		args           []string
		expectExitCode int
		expectOutput   string
	}{
		{
			name:           "help_command",
			args:           []string{"--help"},
			expectExitCode: 0,
			expectOutput:   "Portguard",
		},
		{
			name:           "version_command",
			args:           []string{"--version"},
			expectExitCode: 0,
			expectOutput:   "",
		},
		{
			name:           "invalid_command",
			args:           []string{"invalid-command"},
			expectExitCode: 1,
			expectOutput:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build the binary first
			buildCmd := exec.CommandContext(context.Background(), "go", "build", "-o", "portguard_test", "main.go")
			buildCmd.Dir = "."
			err := buildCmd.Run()
			require.NoError(t, err)
			defer func() {
				if removeErr := os.Remove("portguard_test"); removeErr != nil {
					t.Logf("Failed to remove test binary: %v", removeErr)
				}
			}()

			// Run the binary with test arguments
			cmd := exec.CommandContext(context.Background(), "./portguard_test", tt.args...)
			output, err := cmd.CombinedOutput()

			if tt.expectExitCode == 0 {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}

			if tt.expectOutput != "" {
				assert.Contains(t, string(output), tt.expectOutput)
			}
		})
	}
}

func TestMainExecution(t *testing.T) {
	// Test that main function calls cmd.Execute() and handles errors correctly
	t.Run("main_execution_integration", func(t *testing.T) {
		// This tests the integration between main and cmd.Execute()
		// We test this by examining the behavior through external process execution

		cmd := exec.CommandContext(context.Background(), "go", "run", "main.go", "help")
		output, err := cmd.CombinedOutput()

		// Should succeed with help command
		require.NoError(t, err)
		assert.Contains(t, strings.ToLower(string(output)), "portguard")
	})

	t.Run("main_error_handling", func(t *testing.T) {
		// Test error handling by providing invalid arguments
		cmd := exec.CommandContext(context.Background(), "go", "run", "main.go", "nonexistent-command")
		_, err := cmd.CombinedOutput()

		// Should exit with error for invalid command
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			assert.NotEqual(t, 0, exitError.ExitCode())
		}
	})
}

func TestMainPackageStructure(t *testing.T) {
	// Test that main package structure is correct
	t.Run("main_package_imports", func(t *testing.T) {
		// Verify that main.go imports the expected packages
		content, err := os.ReadFile("main.go")
		require.NoError(t, err)

		contentStr := string(content)
		assert.Contains(t, contentStr, `"os"`)
		assert.Contains(t, contentStr, `"github.com/paveg/portguard/internal/cmd"`)
	})

	t.Run("main_function_exists", func(t *testing.T) {
		// Verify main function structure
		content, err := os.ReadFile("main.go")
		require.NoError(t, err)

		contentStr := string(content)
		assert.Contains(t, contentStr, "func main()")
		assert.Contains(t, contentStr, "cmd.Execute()")
		assert.Contains(t, contentStr, "os.Exit(1)")
	})
}
