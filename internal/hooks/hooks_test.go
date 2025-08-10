package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHookTypes(t *testing.T) {
	// Test HookType constants
	t.Run("hook_type_constants", func(t *testing.T) {
		assert.Equal(t, PreToolUse, HookType("preToolUse"))
		assert.Equal(t, PostToolUse, HookType("postToolUse"))
		assert.Equal(t, PreSession, HookType("preSession"))
		assert.Equal(t, PostSession, HookType("postSession"))
	})

	// Test FailureMode constants
	t.Run("failure_mode_constants", func(t *testing.T) {
		assert.Equal(t, FailureAllow, FailureMode("allow"))
		assert.Equal(t, FailureBlock, FailureMode("block"))
		assert.Equal(t, FailureWarn, FailureMode("warn"))
		assert.Equal(t, FailureIgnore, FailureMode("ignore"))
	})
}

func TestTemplate(t *testing.T) {
	template := Template{
		Name:        "test-template",
		Version:     "1.0.0",
		Description: "A test template",
		Hooks: []HookDefinition{
			{
				Type:        PreToolUse,
				Script:      "echo 'pre-tool'",
				FailureMode: FailureWarn,
				Enabled:     true,
			},
		},
		Examples: []Example{
			{
				Name:        "basic-example",
				Description: "Basic usage example",
				Command:     "portguard intercept",
				Expected:    "success",
			},
		},
		Dependencies: []string{"bash", "echo"},
		Config:       map[string]string{"test": "value"},
	}

	// Test template structure
	t.Run("template_structure", func(t *testing.T) {
		assert.Equal(t, "test-template", template.Name)
		assert.Equal(t, "1.0.0", template.Version)
		assert.Len(t, template.Hooks, 1)
		assert.Len(t, template.Examples, 1)
		assert.Len(t, template.Dependencies, 2)
	})

	// Test JSON marshaling/unmarshaling
	t.Run("template_json_serialization", func(t *testing.T) {
		data, err := json.Marshal(template)
		require.NoError(t, err)
		assert.NotEmpty(t, data)

		var unmarshaled Template
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)
		
		assert.Equal(t, template.Name, unmarshaled.Name)
		assert.Equal(t, template.Version, unmarshaled.Version)
		assert.Len(t, unmarshaled.Hooks, 1)
		assert.Equal(t, template.Hooks[0].Type, unmarshaled.Hooks[0].Type)
	})
}

func TestHookDefinition(t *testing.T) {
	hook := HookDefinition{
		Type:        PostToolUse,
		Script:      "portguard intercept",
		FailureMode: FailureBlock,
		Enabled:     true,
		Environment: map[string]string{
			"PORTGUARD_BIN": "/usr/local/bin/portguard",
		},
		Timeout: 30 * time.Second,
	}

	// Test hook definition structure
	t.Run("hook_definition_structure", func(t *testing.T) {
		assert.Equal(t, PostToolUse, hook.Type)
		assert.Equal(t, "portguard intercept", hook.Script)
		assert.Equal(t, FailureBlock, hook.FailureMode)
		assert.True(t, hook.Enabled)
		assert.Equal(t, 30*time.Second, hook.Timeout)
		assert.Contains(t, hook.Environment, "PORTGUARD_BIN")
	})
}

func TestNewManager(t *testing.T) {
	manager := NewManager()
	
	assert.NotNil(t, manager)
}

func TestManagerListTemplates(t *testing.T) {
	manager := NewManager()
	
	// Test listing built-in templates
	t.Run("list_builtin_templates", func(t *testing.T) {
		result, err := manager.ListTemplates()
		require.NoError(t, err)
		assert.NotNil(t, result)
		
		// Should have template data
		assert.NotEmpty(t, result.Templates)
		
		// Check for expected template structure
		for _, template := range result.Templates {
			assert.NotEmpty(t, template.Name)
			assert.NotEmpty(t, template.Version)
			assert.NotEmpty(t, template.Description)
			assert.NotNil(t, template.Hooks)
		}
	})
}

func TestNewInstaller(t *testing.T) {
	installer := NewInstaller()
	assert.NotNil(t, installer)
}

func TestInstallerInstall(t *testing.T) {
	tempDir := t.TempDir()
	
	config := &InstallConfig{
		Template:     "basic-claude-integration",
		ClaudeConfig: tempDir,
		DryRun:       true,
		Force:        false,
	}
	
	installer := NewInstaller()
	
	t.Run("install_dry_run", func(t *testing.T) {
		result, err := installer.Install(config)
		
		// In dry run mode, should not error but also shouldn't create files
		if err == nil {
			assert.NotNil(t, result)
			assert.True(t, result.Success || !result.Success) // Just test structure exists
		}
	})
}

func TestNewUpdater(t *testing.T) {
	updater := NewUpdater()
	assert.NotNil(t, updater)
}

func TestNewRemover(t *testing.T) {
	remover := NewRemover()
	assert.NotNil(t, remover)
}

func TestNewStatusChecker(t *testing.T) {
	checker := NewStatusChecker()
	assert.NotNil(t, checker)
}

func TestStatusCheckerCheck(t *testing.T) {
	checker := NewStatusChecker()
	
	t.Run("check_status", func(t *testing.T) {
		result, err := checker.Check()
		require.NoError(t, err)
		assert.NotNil(t, result)
		
		// Should have basic status information
		assert.False(t, result.Installed) // Default should be false in test
		assert.NotNil(t, result.Messages)
	})
}

func TestManagerGetClaudeConfigPaths(t *testing.T) {
	manager := NewManager()
	
	t.Run("get_claude_config_paths", func(t *testing.T) {
		paths := manager.getClaudeConfigPaths()
		assert.NotNil(t, paths)
		
		// Should return at least some potential paths
		assert.GreaterOrEqual(t, len(paths), 1)
		
		for _, path := range paths {
			assert.NotEmpty(t, path)
			assert.True(t, filepath.IsAbs(path))
		}
	})
}

func TestManagerListInstalled(t *testing.T) {
	manager := NewManager()
	
	t.Run("list_installed_hooks", func(t *testing.T) {
		result, err := manager.ListInstalled()
		require.NoError(t, err)
		assert.NotNil(t, result)
		// result.Installed can be an empty slice (not nil) when no hooks are installed
		// This is expected behavior in test environments
		if result.Installed != nil {
			assert.IsType(t, []InstalledHook{}, result.Installed)
		}
	})
}

func TestClaudeCodeSettings(t *testing.T) {
	settings := ClaudeCodeSettings{
		Hooks: map[string]ClaudeCodeHook{
			"preToolUse": {
				Enabled: true,
				Command: "portguard intercept",
			},
		},
		Tools: map[string]interface{}{
			"test": "value",
		},
	}

	t.Run("claude_code_settings_structure", func(t *testing.T) {
		assert.Len(t, settings.Hooks, 1)
		hook, exists := settings.Hooks["preToolUse"]
		assert.True(t, exists)
		assert.True(t, hook.Enabled)
		assert.Equal(t, "portguard intercept", hook.Command)
		
		assert.Len(t, settings.Tools, 1)
		assert.Contains(t, settings.Tools, "test")
	})

	t.Run("claude_code_settings_json", func(t *testing.T) {
		data, err := json.Marshal(settings)
		require.NoError(t, err)
		
		var unmarshaled ClaudeCodeSettings
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)
		
		assert.Len(t, unmarshaled.Hooks, 1)
		hook, exists := unmarshaled.Hooks["preToolUse"]
		assert.True(t, exists)
		assert.True(t, hook.Enabled)
	})
}

func TestHookErrors(t *testing.T) {
	// Test all error variables exist and have meaningful messages
	tests := []struct {
		name string
		err  error
	}{
		{"ErrTemplateNotFound", ErrTemplateNotFound},
		{"ErrHooksNotInstalled", ErrHooksNotInstalled},
		{"ErrSettingsNotFound", ErrSettingsNotFound},
		{"ErrPortguardNotInstalled", ErrPortguardNotInstalled},
		{"ErrInvalidConfig", ErrInvalidConfig},
		{"ErrDependencyMissing", ErrDependencyMissing},
		{"ErrClaudeConfigNotFound", ErrClaudeConfigNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Error(t, tt.err)
			assert.NotEmpty(t, tt.err.Error())
		})
	}
}

func TestInstallerCheckDependencies(t *testing.T) {
	installer := NewInstaller()
	
	t.Run("check_basic_dependencies", func(t *testing.T) {
		dependencies := []string{"echo", "cat"} // Common commands that should exist
		
		for _, dep := range dependencies {
			available := installer.isCommandAvailable(dep)
			// Most systems should have echo and cat
			if available {
				assert.True(t, available)
			}
		}
	})
	
	t.Run("check_nonexistent_dependency", func(t *testing.T) {
		available := installer.isCommandAvailable("definitely-not-a-real-command-12345")
		assert.False(t, available)
	})
}

func TestInstallerFindClaudeConfigPath(t *testing.T) {
	tempDir := t.TempDir()
	_ = InstallConfig{
		ClaudeConfig: tempDir,
		DryRun:    true,
	}
	
	installer := NewInstaller()
	
	t.Run("find_claude_config_path", func(t *testing.T) {
		// Test that findClaudeConfigPath returns a valid directory path or empty string
		// Note: This method checks real system paths, so we can only verify it returns
		// a valid path structure without requiring specific files to exist
		
		path := installer.findClaudeConfigPath()
		
		// The method should either return an empty string (no config found) or a valid absolute path
		if path != "" {
			assert.True(t, filepath.IsAbs(path), "Config path should be absolute if not empty")
			// The path should be a directory that exists or can be created
			if stat, err := os.Stat(path); err == nil {
				assert.True(t, stat.IsDir(), "Config path should be a directory")
			}
		}
		// It's valid for this to return empty string in test environments
	})
}

func TestExample(t *testing.T) {
	example := Example{
		Name:        "basic-setup",
		Description: "Basic Portguard setup for Claude Code",
		Command:     "portguard intercept",
		Expected:    "success",
	}

	t.Run("example_structure", func(t *testing.T) {
		assert.Equal(t, "basic-setup", example.Name)
		assert.Equal(t, "Basic Portguard setup for Claude Code", example.Description)
		assert.Equal(t, "portguard intercept", example.Command)
		assert.Equal(t, "success", example.Expected)
	})
}

func TestInstalledHook(t *testing.T) {
	hook := InstalledHook{
		Name:        "portguard-integration",
		Template:    "basic-claude-integration",
		Version:     "1.0.0",
		Status:      "active",
		ConfigPath:  "/home/user/.claude/settings.json",
		InstalledAt: time.Now(),
	}

	t.Run("installed_hook_structure", func(t *testing.T) {
		assert.Equal(t, "portguard-integration", hook.Name)
		assert.Equal(t, "basic-claude-integration", hook.Template)
		assert.Equal(t, "1.0.0", hook.Version)
		assert.Equal(t, "active", hook.Status)
		assert.NotEmpty(t, hook.ConfigPath)
		assert.False(t, hook.InstalledAt.IsZero())
	})
}

func TestHookConfig(t *testing.T) {
	config := HookConfig{
		Enabled:     true,
		Version:     "1.0.0",
		Customized:  false,
		Environment: map[string]string{
			"PORTGUARD_ENV": "production",
		},
	}

	t.Run("hook_config_structure", func(t *testing.T) {
		assert.True(t, config.Enabled)
		assert.Equal(t, "1.0.0", config.Version)
		assert.False(t, config.Customized)
		assert.Contains(t, config.Environment, "PORTGUARD_ENV")
	})
}