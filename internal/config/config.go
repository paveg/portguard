// Package config provides configuration management for Portguard.
// It handles loading and validation of configuration files, environment variables,
// and command-line parameters for the AI-aware process management tool.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/paveg/portguard/internal/process"
	"github.com/spf13/viper"
)

// Static error variables to satisfy err113 linter
var (
	ErrInvalidPortRange    = errors.New("start port must be less than end port")
	ErrInvalidStartPort    = errors.New("invalid start port")
	ErrInvalidEndPort      = errors.New("invalid end port")
	ErrHealthCheckTimeout  = errors.New("health check timeout must be positive")
	ErrHealthCheckInterval = errors.New("health check interval must be positive")
	ErrHealthCheckRetries  = errors.New("health check retries cannot be negative")
	ErrProjectEmptyCommand = errors.New("project has empty command")
	ErrProjectInvalidPort  = errors.New("project has invalid port")
)

// Config represents the application configuration
type Config struct {
	Default  *DefaultConfig            `mapstructure:"default" yaml:"default"`
	Projects map[string]*ProjectConfig `mapstructure:"projects" yaml:"projects"`
}

// DefaultConfig contains default settings
type DefaultConfig struct {
	HealthCheck *HealthCheckConfig `mapstructure:"health_check" yaml:"health_check"`
	PortRange   *PortRangeConfig   `mapstructure:"port_range" yaml:"port_range"`
	Cleanup     *CleanupConfig     `mapstructure:"cleanup" yaml:"cleanup"`
	StateFile   string             `mapstructure:"state_file" yaml:"state_file"`
	LockFile    string             `mapstructure:"lock_file" yaml:"lock_file"`
	LogLevel    string             `mapstructure:"log_level" yaml:"log_level"`
}

// HealthCheckConfig contains default health check settings
type HealthCheckConfig struct {
	Enabled  bool          `mapstructure:"enabled" yaml:"enabled"`
	Timeout  time.Duration `mapstructure:"timeout" yaml:"timeout"`
	Interval time.Duration `mapstructure:"interval" yaml:"interval"`
	Retries  int           `mapstructure:"retries" yaml:"retries"`
}

// PortRangeConfig defines the default port range to scan
type PortRangeConfig struct {
	Start int `mapstructure:"start" yaml:"start"`
	End   int `mapstructure:"end" yaml:"end"`
}

// CleanupConfig contains cleanup settings
type CleanupConfig struct {
	AutoCleanup     bool          `mapstructure:"auto_cleanup" yaml:"auto_cleanup"`
	MaxIdleTime     time.Duration `mapstructure:"max_idle_time" yaml:"max_idle_time"`
	BackupRetention time.Duration `mapstructure:"backup_retention" yaml:"backup_retention"`
}

// ProjectConfig contains project-specific settings
type ProjectConfig struct {
	Command     string               `mapstructure:"command" yaml:"command"`
	Port        int                  `mapstructure:"port" yaml:"port"`
	HealthCheck *process.HealthCheck `mapstructure:"health_check" yaml:"health_check"`
	Environment map[string]string    `mapstructure:"environment" yaml:"environment"`
	WorkingDir  string               `mapstructure:"working_dir" yaml:"working_dir"`
	LogFile     string               `mapstructure:"log_file" yaml:"log_file"`
}

// Load loads configuration from file and environment
func Load() (*Config, error) {
	// Set defaults
	setDefaults()

	// Try to read config file
	if err := viper.ReadInConfig(); err != nil {
		var configNotFoundError viper.ConfigFileNotFoundError
		if !errors.As(err, &configNotFoundError) {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		// Config file not found is OK, we'll use defaults
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unable to decode config: %w", err)
	}

	// Apply defaults if not set
	if config.Default == nil {
		config.Default = getDefaultConfig()
	}

	if config.Projects == nil {
		config.Projects = make(map[string]*ProjectConfig)
	}

	// Expand paths
	if err := expandPaths(&config); err != nil {
		return nil, fmt.Errorf("failed to expand paths: %w", err)
	}

	return &config, nil
}

// setDefaults sets default configuration values
func setDefaults() {
	// Default health check settings
	viper.SetDefault("default.health_check.enabled", true)
	viper.SetDefault("default.health_check.timeout", "30s")
	viper.SetDefault("default.health_check.interval", "10s")
	viper.SetDefault("default.health_check.retries", 3)

	// Default port range
	viper.SetDefault("default.port_range.start", 3000)
	viper.SetDefault("default.port_range.end", 9000)

	// Default cleanup settings
	viper.SetDefault("default.cleanup.auto_cleanup", true)
	viper.SetDefault("default.cleanup.max_idle_time", "1h")
	viper.SetDefault("default.cleanup.backup_retention", "168h")

	// Default file paths
	homeDir, _ := os.UserHomeDir() //nolint:errcheck // Fallback to current dir if home unavailable
	viper.SetDefault("default.state_file", filepath.Join(homeDir, ".portguard", "state.json"))
	viper.SetDefault("default.lock_file", filepath.Join(homeDir, ".portguard", "portguard.lock"))
	viper.SetDefault("default.log_level", "info")
}

// getDefaultConfig returns the default configuration
func getDefaultConfig() *DefaultConfig {
	homeDir, _ := os.UserHomeDir() //nolint:errcheck // Fallback to current dir if home unavailable

	return &DefaultConfig{
		HealthCheck: &HealthCheckConfig{
			Enabled:  true,
			Timeout:  30 * time.Second,
			Interval: 10 * time.Second,
			Retries:  3,
		},
		PortRange: &PortRangeConfig{
			Start: 3000,
			End:   9000,
		},
		Cleanup: &CleanupConfig{
			AutoCleanup:     true,
			MaxIdleTime:     time.Hour,
			BackupRetention: 7 * 24 * time.Hour,
		},
		StateFile: filepath.Join(homeDir, ".portguard", "state.json"),
		LockFile:  filepath.Join(homeDir, ".portguard", "portguard.lock"),
		LogLevel:  "info",
	}
}

// expandPaths expands relative paths to absolute paths
func expandPaths(config *Config) error {
	if config.Default != nil {
		if config.Default.StateFile != "" {
			expanded, err := expandPath(config.Default.StateFile)
			if err != nil {
				return fmt.Errorf("failed to expand state file path: %w", err)
			}
			config.Default.StateFile = expanded
		}

		if config.Default.LockFile != "" {
			expanded, err := expandPath(config.Default.LockFile)
			if err != nil {
				return fmt.Errorf("failed to expand lock file path: %w", err)
			}
			config.Default.LockFile = expanded
		}
	}

	// Expand paths in project configs
	for _, project := range config.Projects {
		if project.WorkingDir != "" {
			expanded, err := expandPath(project.WorkingDir)
			if err != nil {
				return fmt.Errorf("failed to expand working directory: %w", err)
			}
			project.WorkingDir = expanded
		}

		if project.LogFile != "" {
			expanded, err := expandPath(project.LogFile)
			if err != nil {
				return fmt.Errorf("failed to expand log file path: %w", err)
			}
			project.LogFile = expanded
		}
	}

	return nil
}

// expandPath expands ~ to home directory and resolves relative paths
func expandPath(path string) (string, error) {
	if path == "" {
		return path, nil
	}

	// Expand ~ to home directory
	if path[:1] == "~" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		path = filepath.Join(homeDir, path[1:])
	}

	// Convert to absolute path
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}
	return abs, nil
}

// Save saves the configuration to a file
func (c *Config) Save(filename string) error {
	viper.Set("default", c.Default)
	viper.Set("projects", c.Projects)

	if err := viper.WriteConfigAs(filename); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	return nil
}

// GetProject returns a project configuration by name
func (c *Config) GetProject(name string) (*ProjectConfig, bool) {
	project, exists := c.Projects[name]
	return project, exists
}

// AddProject adds or updates a project configuration
func (c *Config) AddProject(name string, project *ProjectConfig) {
	if c.Projects == nil {
		c.Projects = make(map[string]*ProjectConfig)
	}
	c.Projects[name] = project
}

// RemoveProject removes a project configuration
func (c *Config) RemoveProject(name string) {
	delete(c.Projects, name)
}

// ListProjects returns all project names
func (c *Config) ListProjects() []string {
	var names []string //nolint:prealloc // TODO: Pre-allocate slice with len(c.Projects)
	for name := range c.Projects {
		names = append(names, name)
	}
	return names
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Default != nil {
		// Validate port range
		if c.Default.PortRange != nil {
			if c.Default.PortRange.Start < 1 || c.Default.PortRange.Start > 65535 {
				return fmt.Errorf("%w: %d", ErrInvalidStartPort, c.Default.PortRange.Start)
			}
			if c.Default.PortRange.End < 1 || c.Default.PortRange.End > 65535 {
				return fmt.Errorf("%w: %d", ErrInvalidEndPort, c.Default.PortRange.End)
			}
			if c.Default.PortRange.Start > c.Default.PortRange.End {
				return ErrInvalidPortRange
			}
		}

		// Validate health check settings
		if c.Default.HealthCheck != nil {
			if c.Default.HealthCheck.Timeout <= 0 {
				return ErrHealthCheckTimeout
			}
			if c.Default.HealthCheck.Interval <= 0 {
				return ErrHealthCheckInterval
			}
			if c.Default.HealthCheck.Retries < 0 {
				return ErrHealthCheckRetries
			}
		}
	}

	// Validate project configurations
	for name, project := range c.Projects {
		if project.Command == "" {
			return fmt.Errorf("%w: %s", ErrProjectEmptyCommand, name)
		}
		if project.Port != 0 && (project.Port < 1 || project.Port > 65535) {
			return fmt.Errorf("%w: %s (port: %d)", ErrProjectInvalidPort, name, project.Port)
		}
	}

	return nil
}
