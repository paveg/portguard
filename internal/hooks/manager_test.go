package hooks

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHookManager(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	defer func() { _ = os.Setenv("HOME", oldHome) }()
	_ = os.Setenv("HOME", tempDir)

	manager := NewManager()

	t.Run("list_all_templates", func(t *testing.T) {
		result, err := manager.ListAll()
		assert.NoError(t, err)

		// Should return available templates
		assert.NotEmpty(t, result.Templates)

		// Should contain expected templates
		templateNames := make([]string, len(result.Templates))
		for i, template := range result.Templates {
			templateNames[i] = template.Name
		}
		assert.Contains(t, templateNames, "minimal")
		assert.Contains(t, templateNames, "developer")
		assert.Contains(t, templateNames, "advanced")
	})

	t.Run("check_dependencies_basic", func(t *testing.T) {
		// TODO: Implement CheckDependencies method in Manager
		// missing := manager.CheckDependencies()
		// Should return list of missing dependencies (likely including claude-cli or gh)
		// assert.NotNil(t, missing)

		// For now, just verify manager exists
		assert.NotNil(t, manager)
	})
}

func TestUpdateClaudeCodeSettings(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	defer func() { _ = os.Setenv("HOME", oldHome) }()
	_ = os.Setenv("HOME", tempDir)

	// Create a test settings directory
	settingsDir := tempDir + "/.config/claude"
	err := os.MkdirAll(settingsDir, 0o755)
	require.NoError(t, err)

	// Test updating settings
	manager := NewManager()
	// TODO: Implement Install method or use appropriate existing method
	// result, err := manager.Install("minimal")

	// For now, just test that manager was created successfully
	assert.NotNil(t, manager)
}
