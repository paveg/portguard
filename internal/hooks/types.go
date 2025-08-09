package hooks

import (
	"time"
)

// Template represents a hook template configuration
type Template struct {
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Version      string            `json:"version"`
	Hooks        []HookDefinition  `json:"hooks"`
	Dependencies []string          `json:"dependencies"`
	Config       map[string]string `json:"config,omitempty"`
	Examples     []Example         `json:"examples,omitempty"`
}

// HookDefinition defines a specific hook (PreToolUse, PostToolUse, etc.)
type HookDefinition struct {
	Name        string            `json:"name"`         // e.g., "preToolUse"
	Type        HookType          `json:"type"`         // PreToolUse, PostToolUse, etc.
	Script      string            `json:"script"`       // Embedded script content
	Timeout     time.Duration     `json:"timeout"`      // Execution timeout
	FailureMode FailureMode       `json:"failure_mode"` // How to handle failures
	Environment map[string]string `json:"environment"`  // Environment variables
	Enabled     bool              `json:"enabled"`      // Whether hook is enabled
	Description string            `json:"description"`  // Hook description
}

// HookType represents the type of hook event
type HookType string

const (
	PreToolUse  HookType = "preToolUse"
	PostToolUse HookType = "postToolUse"
	PreSession  HookType = "preSession"
	PostSession HookType = "postSession"
)

// FailureMode defines how to handle hook failures
type FailureMode string

const (
	FailureAllow  FailureMode = "allow"  // Allow operation to proceed
	FailureBlock  FailureMode = "block"  // Block operation
	FailureWarn   FailureMode = "warn"   // Show warning but proceed
	FailureIgnore FailureMode = "ignore" // Silently ignore failures
)

// Example provides usage examples for the template
type Example struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Command     string `json:"command"`
	Expected    string `json:"expected"`
}

// InstallConfig configures hook installation
type InstallConfig struct {
	Template     string `json:"template"`      // Template name to install
	ClaudeConfig string `json:"claude_config"` // Path to Claude Code config directory
	DryRun       bool   `json:"dry_run"`       // Don't make actual changes
	Force        bool   `json:"force"`         // Force overwrite existing hooks
}

// InstallResult contains the result of hook installation
type InstallResult struct {
	Success       bool      `json:"success"`
	Template      string    `json:"template"`
	InstalledAt   time.Time `json:"installed_at"`
	ConfigPath    string    `json:"config_path"`
	HooksCreated  []string  `json:"hooks_created"`
	ConfigUpdated bool      `json:"config_updated"`
	Messages      []string  `json:"messages,omitempty"`
	Warnings      []string  `json:"warnings,omitempty"`
}

// UpdateConfig configures hook updates
type UpdateConfig struct {
	DryRun bool `json:"dry_run"` // Don't make actual changes
	Force  bool `json:"force"`   // Force update even if no changes
}

// UpdateResult contains the result of hook updates
type UpdateResult struct {
	Success         bool      `json:"success"`
	UpdatedAt       time.Time `json:"updated_at"`
	PreviousVersion string    `json:"previous_version"`
	NewVersion      string    `json:"new_version"`
	HooksUpdated    []string  `json:"hooks_updated"`
	Messages        []string  `json:"messages,omitempty"`
}

// RemoveConfig configures hook removal
type RemoveConfig struct {
	DryRun         bool `json:"dry_run"`         // Don't make actual changes
	Force          bool `json:"force"`           // Skip confirmation
	PreserveConfig bool `json:"preserve_config"` // Keep user customizations
}

// RemoveResult contains the result of hook removal
type RemoveResult struct {
	Success       bool      `json:"success"`
	RemovedAt     time.Time `json:"removed_at"`
	HooksRemoved  []string  `json:"hooks_removed"`
	ConfigCleaned bool      `json:"config_cleaned"`
	Messages      []string  `json:"messages,omitempty"`
}

// ListResult contains the result of listing templates/hooks
type ListResult struct {
	Templates []Template      `json:"templates,omitempty"`
	Installed []InstalledHook `json:"installed,omitempty"`
}

// InstalledHook represents an installed hook
type InstalledHook struct {
	Name        string    `json:"name"`
	Template    string    `json:"template"`
	Version     string    `json:"version"`
	Status      string    `json:"status"`
	InstalledAt time.Time `json:"installed_at"`
	ConfigPath  string    `json:"config_path"`
}

// StatusResult contains hook installation status
type StatusResult struct {
	Installed      bool      `json:"installed"`
	Version        string    `json:"version,omitempty"`
	Template       string    `json:"template,omitempty"`
	ConfigPath     string    `json:"config_path,omitempty"`
	HooksActive    []string  `json:"hooks_active,omitempty"`
	DependenciesOK bool      `json:"dependencies_ok"`
	MissingDeps    []string  `json:"missing_deps,omitempty"`
	LastChecked    time.Time `json:"last_checked"`
	Messages       []string  `json:"messages,omitempty"`
}

// ClaudeCodeSettings represents Claude Code configuration structure
type ClaudeCodeSettings struct {
	Hooks    map[string]ClaudeCodeHook `json:"hooks,omitempty"`
	Tools    map[string]interface{}    `json:"tools,omitempty"`
	Security map[string]interface{}    `json:"security,omitempty"`
	Other    map[string]interface{}    `json:"-"` // For preserving unknown fields
}

// ClaudeCodeHook represents a hook configuration in Claude Code settings
type ClaudeCodeHook struct {
	Enabled         bool              `json:"enabled"`
	Command         string            `json:"command"`
	Timeout         int               `json:"timeout,omitempty"`         // in milliseconds
	FailureHandling string            `json:"failureHandling,omitempty"` // "allow", "block", "warn", "ignore"
	Environment     map[string]string `json:"environment,omitempty"`
	Description     string            `json:"description,omitempty"`
}

// PortguardConfig represents portguard-specific hook configuration
type PortguardConfig struct {
	Version   string                 `json:"version"`
	Template  string                 `json:"template"`
	Installed time.Time              `json:"installed"`
	Hooks     map[string]HookConfig  `json:"hooks"`
	Settings  map[string]interface{} `json:"settings,omitempty"`
}

// HookConfig represents configuration for a specific hook
type HookConfig struct {
	Enabled     bool              `json:"enabled"`
	Version     string            `json:"version"`
	Customized  bool              `json:"customized"` // Whether user has modified the hook
	Environment map[string]string `json:"environment,omitempty"`
}
