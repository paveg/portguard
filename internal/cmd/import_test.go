package cmd

import (
	"os"
	"testing"

	"github.com/paveg/portguard/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImportCommand(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	defer func() { _ = os.Setenv("HOME", oldHome) }()
	_ = os.Setenv("HOME", tempDir)

	t.Run("import_by_port_not_found", func(t *testing.T) {
		err := importProcessByPort(99999) // Use a very high port that should be available
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "port")
	})

	t.Run("import_by_invalid_pid", func(t *testing.T) {
		err := importProcessByPID(-1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid PID")
	})

	t.Run("import_by_nonexistent_pid", func(t *testing.T) {
		err := importProcessByPID(99999) // Use a very high PID that should not exist
		assert.Error(t, err)
	})
}

func TestCreateManagementComponents(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	defer func() { _ = os.Setenv("HOME", oldHome) }()
	_ = os.Setenv("HOME", tempDir)

	// Load default config
	cfg, err := config.Load()
	require.NoError(t, err)

	// Test creating management components
	stateStore, lockManager, portScanner, err := createManagementComponents(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, stateStore)
	assert.NotNil(t, lockManager)
	assert.NotNil(t, portScanner)
}

func TestGetPortguardDir(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	defer func() { _ = os.Setenv("HOME", oldHome) }()
	_ = os.Setenv("HOME", tempDir)

	dir, err := getPortguardDir()
	assert.NoError(t, err)
	assert.Contains(t, dir, ".portguard")

	// Verify directory exists
	_, err = os.Stat(dir)
	assert.NoError(t, err)
}
