package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paveg/portguard/internal/process"
)

func TestConfigErrors(t *testing.T) {
	// Test all error variables exist and have meaningful messages
	tests := []struct {
		name string
		err  error
	}{
		{"ErrInvalidPortRange", ErrInvalidPortRange},
		{"ErrInvalidStartPort", ErrInvalidStartPort},
		{"ErrInvalidEndPort", ErrInvalidEndPort},
		{"ErrHealthCheckTimeout", ErrHealthCheckTimeout},
		{"ErrHealthCheckInterval", ErrHealthCheckInterval},
		{"ErrHealthCheckRetries", ErrHealthCheckRetries},
		{"ErrProjectEmptyCommand", ErrProjectEmptyCommand},
		{"ErrProjectInvalidPort", ErrProjectInvalidPort},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Error(t, tt.err)
			assert.NotEmpty(t, tt.err.Error())
		})
	}
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name        string
		setupConfig func(*testing.T) func()
		expectError bool
		validate    func(*testing.T, *Config)
	}{
		{
			name: "load_default_config",
			setupConfig: func(t *testing.T) func() {
				t.Helper()
				// Reset viper before test
				viper.Reset()
				return func() { viper.Reset() }
			},
			expectError: false,
			validate: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.NotNil(t, cfg)
				assert.NotNil(t, cfg.Default)
				assert.NotNil(t, cfg.Projects)
			},
		},
		{
			name: "load_valid_yaml_config",
			setupConfig: func(t *testing.T) func() {
				t.Helper()
				tempDir := t.TempDir()
				configPath := filepath.Join(tempDir, "test-config.yml")

				configContent := `
default:
  health_check:
    enabled: true
    timeout: 10s
    interval: 5s
    retries: 3
  port_range:
    start: 3000
    end: 9000
  cleanup:
    auto_cleanup: true
    max_idle_time: 24h
  state_file: "/tmp/portguard-test.json"
  lock_file: "/tmp/portguard-test.lock"
  log_level: "info"
projects:
  webapp:
    command: "npm run dev"
    port: 3000
    environment:
      NODE_ENV: "development"
    working_dir: "/app"
`
				err := os.WriteFile(configPath, []byte(configContent), 0o600)
				require.NoError(t, err)

				// Reset viper and set config file
				viper.Reset()
				viper.SetConfigFile(configPath)

				return func() { viper.Reset() }
			},
			expectError: false,
			validate: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.NotNil(t, cfg)
				assert.NotNil(t, cfg.Default)

				// Validate default settings
				assert.True(t, cfg.Default.HealthCheck.Enabled)
				assert.Equal(t, 10*time.Second, cfg.Default.HealthCheck.Timeout)
				assert.Equal(t, 5*time.Second, cfg.Default.HealthCheck.Interval)
				assert.Equal(t, 3, cfg.Default.HealthCheck.Retries)
				assert.Equal(t, 3000, cfg.Default.PortRange.Start)
				assert.Equal(t, 9000, cfg.Default.PortRange.End)
				assert.Equal(t, "/tmp/portguard-test.json", cfg.Default.StateFile)
				assert.Equal(t, "info", cfg.Default.LogLevel)

				// Validate project
				assert.Len(t, cfg.Projects, 1)
				webapp, exists := cfg.Projects["webapp"]
				assert.True(t, exists)
				assert.Equal(t, "npm run dev", webapp.Command)
				assert.Equal(t, 3000, webapp.Port)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := tt.setupConfig(t)
			defer cleanup()

			cfg, err := Load()

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			if tt.validate != nil {
				tt.validate(t, cfg)
			}
		})
	}
}

func TestGetDefaultConfig(t *testing.T) {
	config := getDefaultConfig()

	assert.NotNil(t, config)
	assert.NotNil(t, config.HealthCheck)
	assert.NotNil(t, config.PortRange)
	assert.NotNil(t, config.Cleanup)

	// Verify default values
	assert.True(t, config.HealthCheck.Enabled)
	assert.Equal(t, 30*time.Second, config.HealthCheck.Timeout)
	assert.Equal(t, 10*time.Second, config.HealthCheck.Interval)
	assert.Equal(t, 3, config.HealthCheck.Retries)

	assert.Equal(t, 3000, config.PortRange.Start)
	assert.Equal(t, 9000, config.PortRange.End)

	assert.True(t, config.Cleanup.AutoCleanup)
	assert.Equal(t, 1*time.Hour, config.Cleanup.MaxIdleTime)

	assert.Equal(t, "info", config.LogLevel)
}

func TestExpandPaths(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		validate func(*testing.T, *Config)
	}{
		{
			name: "expand_tilde_in_paths",
			config: &Config{
				Default: &DefaultConfig{
					StateFile: "~/portguard/state.json",
					LockFile:  "~/portguard/lock.file",
				},
			},
			validate: func(t *testing.T, cfg *Config) {
				t.Helper()
				homeDir, err := os.UserHomeDir()
				require.NoError(t, err)
				expectedStateFile := filepath.Join(homeDir, "portguard", "state.json")
				expectedLockFile := filepath.Join(homeDir, "portguard", "lock.file")

				assert.Equal(t, expectedStateFile, cfg.Default.StateFile)
				assert.Equal(t, expectedLockFile, cfg.Default.LockFile)
			},
		},
		{
			name: "expand_relative_paths",
			config: &Config{
				Default: &DefaultConfig{
					StateFile: "./data/state.json",
					LockFile:  "./data/lock.file",
				},
			},
			validate: func(t *testing.T, cfg *Config) {
				t.Helper()
				wd, err := os.Getwd()
				require.NoError(t, err)
				expectedStateFile := filepath.Join(wd, "data", "state.json")
				expectedLockFile := filepath.Join(wd, "data", "lock.file")

				assert.Equal(t, expectedStateFile, cfg.Default.StateFile)
				assert.Equal(t, expectedLockFile, cfg.Default.LockFile)
			},
		},
		{
			name: "absolute_paths_unchanged",
			config: &Config{
				Default: &DefaultConfig{
					StateFile: "/tmp/portguard/state.json",
					LockFile:  "/tmp/portguard/lock.file",
				},
			},
			validate: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, "/tmp/portguard/state.json", cfg.Default.StateFile)
				assert.Equal(t, "/tmp/portguard/lock.file", cfg.Default.LockFile)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := expandPaths(tt.config)
			require.NoError(t, err)
			tt.validate(t, tt.config)
		})
	}
}

func TestExpandPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected func() string
	}{
		{
			name: "expand_tilde",
			path: "~/test/path",
			expected: func() string {
				home, err := os.UserHomeDir()
				if err != nil {
					return ""
				}
				return filepath.Join(home, "test", "path")
			},
		},
		{
			name: "expand_relative_path",
			path: "./test/path",
			expected: func() string {
				wd, err := os.Getwd()
				if err != nil {
					return ""
				}
				return filepath.Join(wd, "test", "path")
			},
		},
		{
			name:     "absolute_path_unchanged",
			path:     "/absolute/test/path",
			expected: func() string { return "/absolute/test/path" },
		},
		{
			name:     "empty_path",
			path:     "",
			expected: func() string { return "" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := expandPath(tt.path)
			require.NoError(t, err)
			expected := tt.expected()
			assert.Equal(t, expected, result)
		})
	}
}

func TestConfigSave(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-save-config.yml")

	config := &Config{
		Default: &DefaultConfig{
			HealthCheck: &HealthCheckConfig{
				Enabled:  true,
				Timeout:  15 * time.Second,
				Interval: 5 * time.Second,
				Retries:  2,
			},
			PortRange: &PortRangeConfig{
				Start: 4000,
				End:   8000,
			},
			Cleanup: &CleanupConfig{
				AutoCleanup: true,
				MaxIdleTime: 12 * time.Hour,
			},
			StateFile: "/tmp/test-state.json",
			LockFile:  "/tmp/test-lock.file",
			LogLevel:  "debug",
		},
		Projects: map[string]*ProjectConfig{
			"testapp": {
				Command: "npm start",
				Port:    4000,
				Environment: map[string]string{
					"NODE_ENV": "test",
				},
				WorkingDir: "/app/test",
				HealthCheck: &process.HealthCheck{
					Type:    process.HealthCheckHTTP,
					Enabled: true,
					Timeout: 20 * time.Second,
				},
			},
		},
	}

	// Test save
	err := config.Save(configPath)
	require.NoError(t, err)

	// Verify file was created
	_, err = os.Stat(configPath)
	require.NoError(t, err)

	// Test load back and verify content
	viper.Reset()
	viper.SetConfigFile(configPath)
	loadedConfig, err := Load()
	require.NoError(t, err)

	assert.Equal(t, config.Default.LogLevel, loadedConfig.Default.LogLevel)
	assert.Equal(t, config.Default.PortRange.Start, loadedConfig.Default.PortRange.Start)

	assert.Len(t, loadedConfig.Projects, 1)
	testapp, exists := loadedConfig.Projects["testapp"]
	assert.True(t, exists)
	assert.Equal(t, "npm start", testapp.Command)
	assert.Equal(t, 4000, testapp.Port)
}

func TestConfigProjectMethods(t *testing.T) {
	config := &Config{
		Default:  getDefaultConfig(),
		Projects: make(map[string]*ProjectConfig),
	}

	// Test AddProject
	project := &ProjectConfig{
		Command:    "npm run test",
		Port:       5000,
		WorkingDir: "/test/app",
	}

	config.AddProject("testproj", project)
	assert.Len(t, config.Projects, 1)

	// Test GetProject
	retrieved, exists := config.GetProject("testproj")
	assert.True(t, exists)
	assert.NotNil(t, retrieved)
	assert.Equal(t, "npm run test", retrieved.Command)

	// Test GetProject non-existent
	notFound, exists := config.GetProject("nonexistent")
	assert.False(t, exists)
	assert.Nil(t, notFound)

	// Test ListProjects
	projects := config.ListProjects()
	assert.Len(t, projects, 1)
	assert.Contains(t, projects, "testproj")

	// Add another project
	project2 := &ProjectConfig{
		Command: "go run main.go",
		Port:    8080,
	}
	config.AddProject("goproj", project2)

	projects = config.ListProjects()
	assert.Len(t, projects, 2)
	assert.Contains(t, projects, "testproj")
	assert.Contains(t, projects, "goproj")

	// Test RemoveProject
	config.RemoveProject("testproj")
	assert.Len(t, config.Projects, 1)

	testproj, exists := config.GetProject("testproj")
	assert.False(t, exists)
	assert.Nil(t, testproj)

	goproj, exists := config.GetProject("goproj")
	assert.True(t, exists)
	assert.NotNil(t, goproj)

	// Test RemoveProject non-existent (should not panic)
	config.RemoveProject("nonexistent")
	assert.Len(t, config.Projects, 1)
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorType   error
	}{
		{
			name: "valid_config",
			config: &Config{
				Default: &DefaultConfig{
					HealthCheck: &HealthCheckConfig{
						Enabled:  true,
						Timeout:  10 * time.Second,
						Interval: 5 * time.Second,
						Retries:  3,
					},
					PortRange: &PortRangeConfig{
						Start: 3000,
						End:   9000,
					},
				},
				Projects: map[string]*ProjectConfig{
					"valid": {
						Command: "npm start",
						Port:    3000,
					},
				},
			},
			expectError: false,
		},
		{
			name: "invalid_port_range",
			config: &Config{
				Default: &DefaultConfig{
					HealthCheck: &HealthCheckConfig{
						Enabled:  true,
						Timeout:  10 * time.Second,
						Interval: 5 * time.Second,
						Retries:  3,
					},
					PortRange: &PortRangeConfig{
						Start: 9000,
						End:   3000, // Invalid: end < start
					},
				},
			},
			expectError: true,
			errorType:   ErrInvalidPortRange,
		},
		{
			name: "invalid_health_check_timeout",
			config: &Config{
				Default: &DefaultConfig{
					HealthCheck: &HealthCheckConfig{
						Enabled:  true,
						Timeout:  0, // Invalid: zero timeout
						Interval: 5 * time.Second,
						Retries:  3,
					},
					PortRange: &PortRangeConfig{
						Start: 3000,
						End:   9000,
					},
				},
			},
			expectError: true,
			errorType:   ErrHealthCheckTimeout,
		},
		{
			name: "invalid_health_check_interval",
			config: &Config{
				Default: &DefaultConfig{
					HealthCheck: &HealthCheckConfig{
						Enabled:  true,
						Timeout:  10 * time.Second,
						Interval: -5 * time.Second, // Invalid: negative interval
						Retries:  3,
					},
					PortRange: &PortRangeConfig{
						Start: 3000,
						End:   9000,
					},
				},
			},
			expectError: true,
			errorType:   ErrHealthCheckInterval,
		},
		{
			name: "invalid_health_check_retries",
			config: &Config{
				Default: &DefaultConfig{
					HealthCheck: &HealthCheckConfig{
						Enabled:  true,
						Timeout:  10 * time.Second,
						Interval: 5 * time.Second,
						Retries:  -1, // Invalid: negative retries
					},
					PortRange: &PortRangeConfig{
						Start: 3000,
						End:   9000,
					},
				},
			},
			expectError: true,
			errorType:   ErrHealthCheckRetries,
		},
		{
			name: "project_empty_command",
			config: &Config{
				Default: getDefaultConfig(),
				Projects: map[string]*ProjectConfig{
					"invalid": {
						Command: "", // Invalid: empty command
						Port:    3000,
					},
				},
			},
			expectError: true,
			errorType:   ErrProjectEmptyCommand,
		},
		{
			name: "project_invalid_port",
			config: &Config{
				Default: getDefaultConfig(),
				Projects: map[string]*ProjectConfig{
					"invalid": {
						Command: "npm start",
						Port:    -1, // Invalid: negative port
					},
				},
			},
			expectError: true,
			errorType:   ErrProjectInvalidPort,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				require.Error(t, err)
				if tt.errorType != nil {
					require.ErrorIs(t, err, tt.errorType)
				}
				return
			}

			require.NoError(t, err)
		})
	}
}
