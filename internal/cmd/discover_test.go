package cmd

import (
	"bytes"
	"os"
	"testing"

	"github.com/paveg/portguard/internal/config"
	"github.com/paveg/portguard/internal/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoverCommand(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	defer func() { _ = os.Setenv("HOME", oldHome) }()
	_ = os.Setenv("HOME", tempDir)

	t.Run("discover_with_no_processes", func(t *testing.T) {
		// Capture output
		var buf bytes.Buffer
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w
		defer func() { os.Stdout = oldStdout }()

		go func() {
			defer func() { _ = w.Close() }()
			err := runDiscoverCommand()
			assert.NoError(t, err)
		}()

		// Read output
		output := make([]byte, 1024)
		n, _ := r.Read(output)
		_ = r.Close()
		buf.Write(output[:n])

		// Should find no development servers in default range
		assert.Contains(t, buf.String(), "No development servers found")
	})

	t.Run("discover_with_custom_range", func(t *testing.T) {
		// Set custom port range
		portRange = "8000-8010"
		defer func() { portRange = "" }()

		err := runDiscoverCommand()
		assert.NoError(t, err)
	})

	t.Run("discover_with_invalid_range", func(t *testing.T) {
		// Set invalid port range
		portRange = "invalid-range"
		defer func() { portRange = "" }()

		err := runDiscoverCommand()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid port range")
	})
}

func TestDiscoverHelperFunctions(t *testing.T) {
	t.Run("hasSuitableProcesses", func(t *testing.T) {
		// Test with no processes
		emptyProcesses := []*process.AdoptionInfo{}
		assert.False(t, hasSuitableProcesses(emptyProcesses))

		// Test with no suitable processes
		unsuitableProcesses := []*process.AdoptionInfo{
			{PID: 1, ProcessName: "test", IsSuitable: false},
		}
		assert.False(t, hasSuitableProcesses(unsuitableProcesses))

		// Test with suitable processes
		suitableProcesses := []*process.AdoptionInfo{
			{PID: 1, ProcessName: "test", IsSuitable: false},
			{PID: 2, ProcessName: "node", IsSuitable: true},
		}
		assert.True(t, hasSuitableProcesses(suitableProcesses))
	})

	t.Run("countSuitableProcesses", func(t *testing.T) {
		// Test with no processes
		emptyProcesses := []*process.AdoptionInfo{}
		assert.Equal(t, 0, countSuitableProcesses(emptyProcesses))

		// Test with mixed processes
		mixedProcesses := []*process.AdoptionInfo{
			{PID: 1, ProcessName: "test", IsSuitable: false},
			{PID: 2, ProcessName: "node", IsSuitable: true},
			{PID: 3, ProcessName: "npm", IsSuitable: true},
		}
		assert.Equal(t, 2, countSuitableProcesses(mixedProcesses))
	})
}

func TestOutputDiscoveryResultsJSON(t *testing.T) {
	// Set JSON output
	jsonOutput = true
	defer func() { jsonOutput = false }()

	// Capture stdout
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	processes := []*process.AdoptionInfo{
		{
			PID:         1234,
			ProcessName: "node",
			Command:     "npm run dev",
			Port:        3000,
			IsSuitable:  true,
		},
	}

	go func() {
		defer func() { _ = w.Close() }()
		err := outputDiscoveryResultsJSON(processes)
		assert.NoError(t, err)
	}()

	// Read output
	output := make([]byte, 2048)
	n, _ := r.Read(output)
	_ = r.Close()
	buf.Write(output[:n])

	// Verify JSON structure
	result := buf.String()
	assert.Contains(t, result, "discovered_processes")
	assert.Contains(t, result, "count")
	assert.Contains(t, result, "suitable_count")
	assert.Contains(t, result, "node")
}

func TestCreateDiscoveryManagementComponents(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	defer func() { _ = os.Setenv("HOME", oldHome) }()
	_ = os.Setenv("HOME", tempDir)

	// Load default config
	cfg, err := config.Load()
	require.NoError(t, err)

	// Test creating management components
	stateStore, lockManager, portScanner, err := createDiscoveryManagementComponents(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, stateStore)
	assert.NotNil(t, lockManager)
	assert.NotNil(t, portScanner)
}
