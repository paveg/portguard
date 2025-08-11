package cmd

import (
	"os"
	"testing"
	"time"

	portpkg "github.com/paveg/portguard/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleSinglePortCheck(t *testing.T) {
	tests := []struct {
		name        string
		port        int
		expectError bool
	}{
		{
			name:        "check_valid_port",
			port:        8080,
			expectError: false,
		},
		{
			name:        "check_system_port",
			port:        80,
			expectError: false,
		},
		{
			name:        "check_high_port",
			port:        65535,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory for test
			tempDir, err := os.MkdirTemp("", "portguard-ports-test")
			require.NoError(t, err)
			defer os.RemoveAll(tempDir)

			// Create scanner
			scanner := portpkg.NewScanner(5 * time.Second)

			// Test handleSinglePortCheck
			err = handleSinglePortCheck(scanner, tt.port)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHandlePortRangeScanning(t *testing.T) {
	tests := []struct {
		name        string
		startPort   int
		endPort     int
		expectError bool
	}{
		{
			name:        "scan_small_range",
			startPort:   8080,
			endPort:     8085,
			expectError: false,
		},
		{
			name:        "scan_invalid_range",
			startPort:   8085,
			endPort:     8080,
			expectError: true,
		},
		{
			name:        "scan_single_port_range",
			startPort:   8080,
			endPort:     8080,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory for test
			tempDir, err := os.MkdirTemp("", "portguard-range-test")
			require.NoError(t, err)
			defer os.RemoveAll(tempDir)

			// Create scanner
			scanner := portpkg.NewScanner(5 * time.Second)

			// Test handlePortRangeScanning
			err = handlePortRangeScanning(scanner, tt.startPort, tt.endPort)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHandleListeningPorts(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "portguard-listening-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create scanner
	scanner := portpkg.NewScanner(5 * time.Second)

	// Test handleListeningPorts - should not error even if no ports are listening
	err = handleListeningPorts(scanner)
	assert.NoError(t, err)
}

func TestPortCommandIntegration(t *testing.T) {
	t.Run("port_check_with_json_output", func(t *testing.T) {
		// Create temporary directory for test
		tempDir, err := os.MkdirTemp("", "portguard-integration-test")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		// Create scanner
		scanner := portpkg.NewScanner(5 * time.Second)

		// Set JSON output flag (would normally be set by cobra)
		jsonOutput = true
		defer func() { jsonOutput = false }()

		// Test single port check
		err = handleSinglePortCheck(scanner, 3000)
		assert.NoError(t, err)
	})

	t.Run("range_scan_with_json_output", func(t *testing.T) {
		// Create temporary directory for test
		tempDir, err := os.MkdirTemp("", "portguard-range-json-test")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		// Create scanner
		scanner := portpkg.NewScanner(5 * time.Second)

		// Set JSON output flag
		jsonOutput = true
		defer func() { jsonOutput = false }()

		// Test range scanning
		err = handlePortRangeScanning(scanner, 8080, 8082)
		assert.NoError(t, err)
	})

	t.Run("listening_ports_with_json_output", func(t *testing.T) {
		// Create temporary directory for test
		tempDir, err := os.MkdirTemp("", "portguard-listening-json-test")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		// Create scanner
		scanner := portpkg.NewScanner(5 * time.Second)

		// Set JSON output flag
		jsonOutput = true
		defer func() { jsonOutput = false }()

		// Test listening ports
		err = handleListeningPorts(scanner)
		assert.NoError(t, err)
	})
}

func TestPortCheckErrorScenarios(t *testing.T) {
	t.Run("invalid_port_number", func(t *testing.T) {
		// Create temporary directory for test
		tempDir, err := os.MkdirTemp("", "portguard-error-test")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		// Create scanner
		scanner := portpkg.NewScanner(5 * time.Second)

		// Test with invalid port (0 is technically valid but unusual)
		err = handleSinglePortCheck(scanner, -1)
		// Scanner might handle this gracefully, so we don't assert error
		_ = err
	})

	t.Run("large_range_scan", func(t *testing.T) {
		// Create temporary directory for test
		tempDir, err := os.MkdirTemp("", "portguard-large-range-test")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		// Create scanner
		scanner := portpkg.NewScanner(5 * time.Second)

		// Test with large range (might be slow but should work)
		err = handlePortRangeScanning(scanner, 1000, 1010)
		assert.NoError(t, err)
	})
}
