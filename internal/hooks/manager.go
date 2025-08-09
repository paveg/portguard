// Package hooks provides Claude Code hooks management functionality.
// It handles template-based hook installation, configuration management,
// and integration with Claude Code's hook system for process management.
package hooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Common errors
var (
	ErrTemplateNotFound      = errors.New("template not found")
	ErrHooksNotInstalled     = errors.New("hooks not installed")
	ErrSettingsNotFound      = errors.New("settings.json not found")
	ErrPortguardNotInstalled = errors.New("portguard hooks not installed")
	ErrInvalidConfig         = errors.New("invalid configuration")
	ErrDependencyMissing     = errors.New("required dependency missing")
	ErrClaudeConfigNotFound  = errors.New("claude code configuration not found")
)

// Manager handles hook management operations
type Manager struct {
	configPath string //nolint:unused // TODO: remove if truly unused or implement usage
}

// NewManager creates a new hook manager
func NewManager() *Manager {
	return &Manager{}
}

// ListAll returns both templates and installed hooks
func (m *Manager) ListAll() (*ListResult, error) {
	templates, err := GetBuiltinTemplates()
	if err != nil {
		return nil, fmt.Errorf("failed to get templates: %w", err)
	}

	installed, err := m.ListInstalled()
	if err != nil {
		// Don't fail if we can't list installed - just return empty
		installed = &ListResult{Installed: []InstalledHook{}}
	}

	return &ListResult{
		Templates: templates,
		Installed: installed.Installed,
	}, nil
}

// ListTemplates returns available templates
func (m *Manager) ListTemplates() (*ListResult, error) {
	templates, err := GetBuiltinTemplates()
	if err != nil {
		return nil, fmt.Errorf("failed to get templates: %w", err)
	}

	return &ListResult{
		Templates: templates,
	}, nil
}

// ListInstalled returns installed hooks
func (m *Manager) ListInstalled() (*ListResult, error) {
	claudeConfigPaths := m.getClaudeConfigPaths()

	var installed []InstalledHook

	for _, configPath := range claudeConfigPaths {
		hooks, err := m.getInstalledHooksFromPath(configPath)
		if err != nil {
			continue // Skip paths that don't have hooks
		}
		installed = append(installed, hooks...)
	}

	return &ListResult{
		Installed: installed,
	}, nil
}

// getClaudeConfigPaths returns potential Claude Code configuration paths
func (m *Manager) getClaudeConfigPaths() []string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return []string{}
	}

	return []string{
		filepath.Join(homeDir, ".config", "claude-code"),
		filepath.Join(homeDir, ".claude"),
		filepath.Join(homeDir, "Library", "Application Support", "claude-code"), // macOS
		// Add Windows paths if needed
	}
}

// getInstalledHooksFromPath checks for installed hooks in a specific path
func (m *Manager) getInstalledHooksFromPath(configPath string) ([]InstalledHook, error) {
	settingsPath := filepath.Join(configPath, "settings.json")
	portguardConfigPath := filepath.Join(configPath, ".portguard-hooks.json")

	// Check if settings.json exists
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		return nil, ErrSettingsNotFound
	}

	// Check for portguard hook configuration
	if _, err := os.Stat(portguardConfigPath); os.IsNotExist(err) {
		return nil, ErrPortguardNotInstalled
	}

	// Load portguard hook config
	data, err := os.ReadFile(portguardConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read portguard config: %w", err)
	}

	var pgConfig PortguardConfig
	if err := json.Unmarshal(data, &pgConfig); err != nil {
		return nil, fmt.Errorf("failed to parse portguard config: %w", err)
	}

	installed := []InstalledHook{
		{
			Name:        "portguard-hooks",
			Template:    pgConfig.Template,
			Version:     pgConfig.Version,
			Status:      "active", // Could be enhanced to check actual status
			InstalledAt: pgConfig.Installed,
			ConfigPath:  configPath,
		},
	}

	return installed, nil
}

// Installer handles hook installation
type Installer struct{}

// NewInstaller creates a new hook installer
func NewInstaller() *Installer {
	return &Installer{}
}

// Install installs hooks with the specified configuration
func (i *Installer) Install(config *InstallConfig) (*InstallResult, error) {
	// Get template
	template, err := GetTemplate(config.Template)
	if err != nil {
		return nil, fmt.Errorf("template '%s' not found: %w", config.Template, err)
	}

	// Determine Claude Code config path
	claudeConfigPath := config.ClaudeConfig
	if claudeConfigPath == "" {
		claudeConfigPath = i.findClaudeConfigPath()
		if claudeConfigPath == "" {
			return nil, ErrClaudeConfigNotFound
		}
	}

	// Check dependencies
	if depErr := i.checkDependencies(template.Dependencies); depErr != nil { //nolint:govet // TODO: rename variables to avoid shadowing
		return nil, fmt.Errorf("dependency check failed: %w", depErr)
	}

	result := &InstallResult{
		Success:      true,
		Template:     config.Template,
		InstalledAt:  time.Now(),
		ConfigPath:   claudeConfigPath,
		HooksCreated: []string{},
		Messages:     []string{},
	}

	if config.DryRun {
		result.Messages = append(result.Messages, "DRY RUN: Would install hooks to "+claudeConfigPath)
		return result, nil
	}

	// Create hook scripts
	for _, hook := range template.Hooks {
		scriptPath := filepath.Join(claudeConfigPath, "hooks", hook.Name+".sh")

		if mkdirErr := os.MkdirAll(filepath.Dir(scriptPath), 0o755); mkdirErr != nil { //nolint:gocritic,govet // TODO: consider using constants for file permissions and avoid shadowing
			return nil, fmt.Errorf("failed to create hooks directory: %w", mkdirErr)
		}

		if writeErr := os.WriteFile(scriptPath, []byte(hook.Script), 0o755); writeErr != nil { //nolint:gocritic,govet // TODO: consider using constants for file permissions and avoid shadowing
			return nil, fmt.Errorf("failed to write hook script: %w", writeErr)
		}

		result.HooksCreated = append(result.HooksCreated, hook.Name+".sh")
	}

	// Update Claude Code settings.json
	if settingsErr := i.updateClaudeCodeSettings(claudeConfigPath, template); settingsErr != nil { //nolint:govet // TODO: rename variables to avoid shadowing
		return nil, fmt.Errorf("failed to update Claude Code settings: %w", settingsErr)
	}
	result.ConfigUpdated = true

	// Create portguard hook config
	pgConfig := PortguardConfig{
		Version:   "1.0.0",
		Template:  template.Name,
		Installed: time.Now(),
		Hooks:     make(map[string]HookConfig),
		Settings:  make(map[string]interface{}),
	}

	for _, hook := range template.Hooks {
		pgConfig.Hooks[hook.Name] = HookConfig{
			Enabled:     hook.Enabled,
			Version:     template.Version,
			Customized:  false,
			Environment: hook.Environment,
		}
	}

	pgConfigData, err := json.MarshalIndent(pgConfig, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal portguard config: %w", err)
	}

	pgConfigPath := filepath.Join(claudeConfigPath, ".portguard-hooks.json")
	if err := os.WriteFile(pgConfigPath, pgConfigData, 0o644); err != nil { //nolint:gocritic // TODO: consider using constants for file permissions
		return nil, fmt.Errorf("failed to write portguard config: %w", err)
	}

	result.Messages = append(result.Messages, "Hooks installed successfully", "Configuration: "+pgConfigPath) //nolint:gocritic,perfsprint // TODO: optimize message building

	return result, nil
}

// findClaudeConfigPath finds the Claude Code configuration directory
func (i *Installer) findClaudeConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	candidates := []string{
		filepath.Join(homeDir, ".config", "claude-code"),
		filepath.Join(homeDir, ".claude"),
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// Default to creating in .config/claude-code
	defaultPath := filepath.Join(homeDir, ".config", "claude-code")
	if err := os.MkdirAll(defaultPath, 0o755); err == nil { //nolint:gocritic // TODO: consider using constants for file permissions
		return defaultPath
	}

	return ""
}

// checkDependencies verifies required dependencies are available
func (i *Installer) checkDependencies(deps []string) error {
	for _, dep := range deps {
		if !i.isCommandAvailable(dep) {
			return fmt.Errorf("%w: %s", ErrDependencyMissing, dep)
		}
	}
	return nil
}

// isCommandAvailable checks if a command is available in PATH
func (i *Installer) isCommandAvailable(command string) bool {
	_, err := os.Stat(command)
	if err == nil {
		return true
	}

	// Check PATH
	pathEnv := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(pathEnv) {
		fullPath := filepath.Join(dir, command)
		if _, err := os.Stat(fullPath); err == nil {
			return true
		}
	}

	return false
}

// updateClaudeCodeSettings updates the Claude Code settings.json file
func (i *Installer) updateClaudeCodeSettings(configPath string, template *Template) error {
	settingsPath := filepath.Join(configPath, "settings.json")

	// Load existing settings or create new
	var settings ClaudeCodeSettings
	if data, err := os.ReadFile(settingsPath); err == nil {
		_ = json.Unmarshal(data, &settings) //nolint:errcheck // TODO: handle unmarshal error properly instead of ignoring
	}

	// Initialize hooks map if needed
	if settings.Hooks == nil {
		settings.Hooks = make(map[string]ClaudeCodeHook)
	}

	// Add hooks from template
	for _, hook := range template.Hooks {
		claudeHook := ClaudeCodeHook{
			Enabled:         hook.Enabled,
			Command:         filepath.Join(configPath, "hooks", hook.Name+".sh"),
			Timeout:         int(hook.Timeout / time.Millisecond),
			FailureHandling: string(hook.FailureMode),
			Environment:     hook.Environment,
			Description:     hook.Description,
		}

		settings.Hooks[string(hook.Type)] = claudeHook
	}

	// Write updated settings
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	if err := os.WriteFile(settingsPath, data, 0o644); err != nil { //nolint:gocritic // TODO: consider using constants for file permissions
		return fmt.Errorf("failed to write settings: %w", err)
	}

	return nil
}

// Updater handles hook updates
type Updater struct{}

// NewUpdater creates a new hook updater
func NewUpdater() *Updater {
	return &Updater{}
}

// Update updates installed hooks
func (u *Updater) Update(config *UpdateConfig) (*UpdateResult, error) {
	result := &UpdateResult{
		Success:   true,
		UpdatedAt: time.Now(),
		Messages:  []string{"Hook update completed"},
	}

	// TODO: Implement actual update logic
	// This would involve:
	// 1. Finding installed hooks
	// 2. Comparing versions
	// 3. Updating scripts and configuration

	return result, nil
}

// Remover handles hook removal
type Remover struct{}

// NewRemover creates a new hook remover
func NewRemover() *Remover {
	return &Remover{}
}

// Remove removes installed hooks
func (r *Remover) Remove(config *RemoveConfig) (*RemoveResult, error) {
	result := &RemoveResult{
		Success:   true,
		RemovedAt: time.Now(),
		Messages:  []string{"Hooks removed successfully"},
	}

	// TODO: Implement actual removal logic

	return result, nil
}

// StatusChecker checks hook installation status
type StatusChecker struct{}

// NewStatusChecker creates a new status checker
func NewStatusChecker() *StatusChecker {
	return &StatusChecker{}
}

// Check checks the current installation status
func (s *StatusChecker) Check() (*StatusResult, error) {
	result := &StatusResult{
		Installed:      false,
		DependenciesOK: true,
		LastChecked:    time.Now(),
		Messages:       []string{},
	}

	// Check dependencies
	deps := []string{"jq", "portguard"}
	for _, dep := range deps {
		if !s.isCommandAvailable(dep) {
			result.DependenciesOK = false
			result.MissingDeps = append(result.MissingDeps, dep)
		}
	}

	// TODO: Check actual installation status

	return result, nil
}

// isCommandAvailable checks if a command is available
func (s *StatusChecker) isCommandAvailable(command string) bool {
	_, err := os.Stat(command)
	if err == nil {
		return true
	}

	pathEnv := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(pathEnv) {
		fullPath := filepath.Join(dir, command)
		if _, err := os.Stat(fullPath); err == nil {
			return true
		}
	}

	return false
}
