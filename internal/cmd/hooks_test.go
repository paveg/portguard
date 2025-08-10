package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/paveg/portguard/internal/hooks"
)

// captureStdout captures stdout output from a function
func captureStdout(f func()) string {
	oldStdout := os.Stdout
	reader, writer, _ := os.Pipe()
	os.Stdout = writer

	f()

	writer.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(reader)
	return buf.String()
}

func TestPrintHooksInfo(t *testing.T) {
	t.Run("print_list_result_with_templates_and_hooks", func(t *testing.T) {
		listResult := &hooks.ListResult{
			Templates: []hooks.Template{
				{
					Name:        "claude-integration",
					Description: "Basic Claude Code integration template",
					Version:     "1.0.0",
				},
				{
					Name:        "advanced-hooks",
					Description: "Advanced hook configuration",
					Version:     "2.1.0",
				},
			},
			Installed: []hooks.InstalledHook{
				{
					Name:        "portguard-claude",
					Version:     "1.0.0",
					Status:      "active",
					InstalledAt: time.Now(),
				},
				{
					Name:        "test-hook",
					Version:     "0.5.0",
					Status:      "inactive",
					InstalledAt: time.Now(),
				},
			},
		}

		output := captureStdout(func() {
			printHooksInfo(listResult)
		})

		assert.Contains(t, output, "Available Templates:")
		assert.Contains(t, output, "==================")
		assert.Contains(t, output, "claude-integration - Basic Claude Code integration template")
		assert.Contains(t, output, "advanced-hooks - Advanced hook configuration")
		
		assert.Contains(t, output, "Installed Hooks:")
		assert.Contains(t, output, "================")
		assert.Contains(t, output, "portguard-claude (1.0.0) - active")
		assert.Contains(t, output, "test-hook (0.5.0) - inactive")
	})

	t.Run("print_list_result_with_templates_no_installed", func(t *testing.T) {
		listResult := &hooks.ListResult{
			Templates: []hooks.Template{
				{
					Name:        "template1",
					Description: "Test template",
					Version:     "1.0.0",
				},
			},
			Installed: []hooks.InstalledHook{}, // Empty installed hooks
		}

		output := captureStdout(func() {
			printHooksInfo(listResult)
		})

		assert.Contains(t, output, "Available Templates:")
		assert.Contains(t, output, "template1 - Test template")
		assert.Contains(t, output, "Installed Hooks:")
		assert.Contains(t, output, "No hooks currently installed")
	})

	t.Run("print_list_result_empty_templates", func(t *testing.T) {
		listResult := &hooks.ListResult{
			Templates: []hooks.Template{}, // Empty templates
			Installed: []hooks.InstalledHook{
				{
					Name:    "installed-hook",
					Version: "2.0.0",
					Status:  "active",
				},
			},
		}

		output := captureStdout(func() {
			printHooksInfo(listResult)
		})

		assert.Contains(t, output, "Available Templates:")
		assert.Contains(t, output, "Installed Hooks:")
		assert.Contains(t, output, "installed-hook (2.0.0) - active")
	})

	t.Run("print_non_list_result_data", func(t *testing.T) {
		customData := map[string]interface{}{
			"test": "value",
			"num":  42,
		}

		output := captureStdout(func() {
			printHooksInfo(customData)
		})

		assert.Contains(t, output, "Hooks Information:")
		assert.Contains(t, output, "test")
		assert.Contains(t, output, "value")
		assert.Contains(t, output, "42")
	})

	t.Run("print_string_data", func(t *testing.T) {
		stringData := "test string data"

		output := captureStdout(func() {
			printHooksInfo(stringData)
		})

		assert.Contains(t, output, "Hooks Information:")
		assert.Contains(t, output, "test string data")
	})

	t.Run("print_nil_data", func(t *testing.T) {
		output := captureStdout(func() {
			printHooksInfo(nil)
		})

		assert.Contains(t, output, "Hooks Information:")
		assert.Contains(t, output, "<nil>")
	})
}

func TestPrintHooksStatus(t *testing.T) {
	t.Run("status_installed_with_dependencies_ok", func(t *testing.T) {
		status := &hooks.StatusResult{
			Installed:      true,
			Version:        "1.2.3",
			Template:       "claude-integration",
			ConfigPath:     "/home/user/.claude/settings.json",
			DependenciesOK: true,
			HooksActive:    []string{"preToolUse", "postToolUse"},
			LastChecked:    time.Now(),
		}

		output := captureStdout(func() {
			printHooksStatus(status)
		})

		assert.Contains(t, output, "Claude Code Hooks Status")
		assert.Contains(t, output, "========================")
		assert.Contains(t, output, "Status:       ✓ Installed")
		assert.Contains(t, output, "Version:      1.2.3")
		assert.Contains(t, output, "Template:     claude-integration")
		assert.Contains(t, output, "Config Path:  /home/user/.claude/settings.json")
		assert.Contains(t, output, "Dependencies: ✓ OK")
	})

	t.Run("status_not_installed", func(t *testing.T) {
		status := &hooks.StatusResult{
			Installed:      false,
			DependenciesOK: true,
			LastChecked:    time.Now(),
		}

		output := captureStdout(func() {
			printHooksStatus(status)
		})

		assert.Contains(t, output, "Claude Code Hooks Status")
		assert.Contains(t, output, "Status:       ✗ Not Installed")
		assert.Contains(t, output, "Dependencies: ✓ OK")
		// Should not contain version/template info when not installed
		assert.NotContains(t, output, "Version:")
		assert.NotContains(t, output, "Template:")
	})

	t.Run("status_installed_with_missing_dependencies", func(t *testing.T) {
		status := &hooks.StatusResult{
			Installed:      true,
			Version:        "2.0.0",
			Template:       "advanced-template",
			ConfigPath:     "/path/to/config",
			DependenciesOK: false,
			MissingDeps:    []string{"bash", "jq", "curl"},
			LastChecked:    time.Now(),
		}

		output := captureStdout(func() {
			printHooksStatus(status)
		})

		assert.Contains(t, output, "Status:       ✓ Installed")
		assert.Contains(t, output, "Version:      2.0.0")
		assert.Contains(t, output, "Template:     advanced-template")
		assert.Contains(t, output, "Dependencies: ✗ Missing")
		assert.Contains(t, output, "- bash")
		assert.Contains(t, output, "- jq")
		assert.Contains(t, output, "- curl")
	})

	t.Run("status_not_installed_with_missing_dependencies", func(t *testing.T) {
		status := &hooks.StatusResult{
			Installed:      false,
			DependenciesOK: false,
			MissingDeps:    []string{"node", "npm"},
			LastChecked:    time.Now(),
		}

		output := captureStdout(func() {
			printHooksStatus(status)
		})

		assert.Contains(t, output, "Status:       ✗ Not Installed")
		assert.Contains(t, output, "Dependencies: ✗ Missing")
		assert.Contains(t, output, "- node")
		assert.Contains(t, output, "- npm")
	})

	t.Run("status_empty_missing_deps", func(t *testing.T) {
		status := &hooks.StatusResult{
			Installed:      true,
			Version:        "1.0.0",
			Template:       "test-template",
			ConfigPath:     "/test/config",
			DependenciesOK: false,
			MissingDeps:    []string{}, // Empty slice
			LastChecked:    time.Now(),
		}

		output := captureStdout(func() {
			printHooksStatus(status)
		})

		assert.Contains(t, output, "Status:       ✓ Installed")
		assert.Contains(t, output, "Dependencies: ✗ Missing")
		// Should not have any dependency lines with empty MissingDeps
		lines := strings.Split(output, "\n")
		found := false
		for _, line := range lines {
			if strings.Contains(line, "- ") {
				found = true
				break
			}
		}
		assert.False(t, found, "Should not have any '- ' lines with empty MissingDeps")
	})

	t.Run("status_with_empty_strings", func(t *testing.T) {
		status := &hooks.StatusResult{
			Installed:      true,
			Version:        "",
			Template:       "",
			ConfigPath:     "",
			DependenciesOK: true,
			LastChecked:    time.Now(),
		}

		output := captureStdout(func() {
			printHooksStatus(status)
		})

		assert.Contains(t, output, "Status:       ✓ Installed")
		assert.Contains(t, output, "Version:      ") // Empty version
		assert.Contains(t, output, "Template:     ") // Empty template
		assert.Contains(t, output, "Config Path:  ") // Empty config path
		assert.Contains(t, output, "Dependencies: ✓ OK")
	})
}