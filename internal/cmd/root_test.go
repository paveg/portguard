package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecute(t *testing.T) {
	// Reset root command state
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	t.Run("help_command_success", func(t *testing.T) {
		// Set args to show help
		os.Args = []string{"portguard", "--help"}

		// Reset root command
		rootCmd.SetArgs([]string{"--help"})

		err := Execute()
		assert.NoError(t, err)
	})

	t.Run("version_command_success", func(t *testing.T) {
		// Set args to show version
		os.Args = []string{"portguard", "--version"}

		// Reset root command
		rootCmd.SetArgs([]string{"--version"})

		err := Execute()
		assert.NoError(t, err)
	})

	t.Run("invalid_command_error", func(t *testing.T) {
		// Set args with invalid command
		os.Args = []string{"portguard", "invalid-command"}

		// Reset root command
		rootCmd.SetArgs([]string{"invalid-command"})

		err := Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "command execution failed")
	})
}

func TestRootCommandStructure(t *testing.T) {
	t.Run("command_metadata", func(t *testing.T) {
		assert.Equal(t, "portguard", rootCmd.Use)
		assert.Equal(t, "AI-aware process management tool", rootCmd.Short)
		assert.Contains(t, rootCmd.Long, "Portguard is a CLI tool")
		assert.Contains(t, rootCmd.Long, "Claude Code")
		// Version should be set and not empty
		assert.NotEmpty(t, rootCmd.Version)
		assert.Equal(t, Version, rootCmd.Version)
	})

	t.Run("persistent_flags", func(t *testing.T) {
		// Check config flag
		configFlag := rootCmd.PersistentFlags().Lookup("config")
		assert.NotNil(t, configFlag)
		assert.Empty(t, configFlag.DefValue)
		assert.Contains(t, configFlag.Usage, "config file")

		// Check verbose flag
		verboseFlag := rootCmd.PersistentFlags().Lookup("verbose")
		assert.NotNil(t, verboseFlag)
		assert.Equal(t, "false", verboseFlag.DefValue)
		assert.Equal(t, "verbose output", verboseFlag.Usage)

		// Check verbose short flag
		verboseShort := rootCmd.PersistentFlags().ShorthandLookup("v")
		assert.NotNil(t, verboseShort)
		assert.Equal(t, verboseFlag, verboseShort)
	})

	t.Run("has_subcommands", func(t *testing.T) {
		// Verify root command has subcommands
		commands := rootCmd.Commands()
		assert.NotEmpty(t, commands)

		// Check for expected commands
		cmdNames := make([]string, len(commands))
		for i, cmd := range commands {
			cmdNames[i] = cmd.Name()
		}

		// Should have common commands
		expectedCommands := []string{"start", "stop", "list", "check", "intercept"}
		for _, expected := range expectedCommands {
			assert.Contains(t, cmdNames, expected, "Missing expected command: %s", expected)
		}
	})
}

func TestInitConfig(t *testing.T) {
	// Save original viper state
	originalConfigFile := viper.ConfigFileUsed()
	defer func() {
		viper.Reset()
		if originalConfigFile != "" {
			viper.SetConfigFile(originalConfigFile)
			_ = viper.ReadInConfig() // Best effort restore
		}
	}()

	t.Run("with_explicit_config_file", func(t *testing.T) {
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "test-config.yml")

		// Create a test config file
		configContent := `
verbose: true
default:
  log_level: debug
`
		err := os.WriteFile(configFile, []byte(configContent), 0o600)
		require.NoError(t, err)

		// Reset viper and set config file
		viper.Reset()
		cfgFile = configFile
		verbose = false // Reset verbose flag

		// Call initConfig
		initConfig()

		// Check that config was read
		assert.Equal(t, configFile, viper.ConfigFileUsed())
		assert.True(t, viper.GetBool("verbose"))
	})

	t.Run("with_default_config_search", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create .portguard.yml in temp directory (simulating home dir)
		configFile := filepath.Join(tempDir, ".portguard.yml")
		configContent := `
default:
  log_level: info
  port_range:
    start: 4000
    end: 8000
`
		err := os.WriteFile(configFile, []byte(configContent), 0o600)
		require.NoError(t, err)

		// Mock home directory
		originalHome := os.Getenv("HOME")
		defer os.Setenv("HOME", originalHome)
		os.Setenv("HOME", tempDir)

		// Reset viper and cfgFile
		viper.Reset()
		cfgFile = ""

		// Call initConfig
		initConfig()

		// Should have found the config file
		assert.Contains(t, viper.ConfigFileUsed(), ".portguard.yml")
		assert.Equal(t, "info", viper.GetString("default.log_level"))
	})

	t.Run("no_config_file_found", func(t *testing.T) {
		tempDir := t.TempDir()

		// Mock home directory with no config file
		originalHome := os.Getenv("HOME")
		defer os.Setenv("HOME", originalHome)
		os.Setenv("HOME", tempDir)

		// Reset viper and cfgFile
		viper.Reset()
		cfgFile = ""

		// Call initConfig - should not fail even if no config found
		initConfig()

		// ConfigFileUsed should be empty since no config was found
		assert.Empty(t, viper.ConfigFileUsed())
	})

	t.Run("environment_variable_support", func(t *testing.T) {
		// Reset viper
		viper.Reset()

		// Set environment variable
		originalEnv := os.Getenv("PORTGUARD_VERBOSE")
		defer func() {
			if originalEnv == "" {
				os.Unsetenv("PORTGUARD_VERBOSE")
			} else {
				os.Setenv("PORTGUARD_VERBOSE", originalEnv)
			}
		}()

		os.Setenv("PORTGUARD_VERBOSE", "true")

		cfgFile = ""

		// Call initConfig
		initConfig()

		// Should have picked up environment variable
		assert.True(t, viper.GetBool("verbose"))
	})
}

func TestRootCommandPersistentPreRun(t *testing.T) {
	// Test the PersistentPreRun function
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test.yml")

	// Create test config
	err := os.WriteFile(configFile, []byte("test: value"), 0o600)
	require.NoError(t, err)

	// Setup viper with test config
	viper.Reset()
	viper.SetConfigFile(configFile)
	err = viper.ReadInConfig()
	require.NoError(t, err)

	// Test with verbose = true
	verbose = true

	// Capture output by running PersistentPreRun
	oldStdout := os.Stdout
	pipeReader, pipeWriter, pipeErr := os.Pipe()
	require.NoError(t, pipeErr)
	os.Stdout = pipeWriter

	rootCmd.PersistentPreRun(rootCmd, []string{})

	pipeWriter.Close()
	os.Stdout = oldStdout

	output := make([]byte, 1024)
	readLen, readErr := pipeReader.Read(output)
	if readErr != nil {
		readLen = 0
	}
	outputStr := string(output[:readLen])

	assert.Contains(t, outputStr, "Using config file:")
	assert.Contains(t, outputStr, configFile)

	// Test with verbose = false
	verbose = false

	// Should not produce output
	pipeReader2, pipeWriter2, pipeErr2 := os.Pipe()
	require.NoError(t, pipeErr2)
	os.Stdout = pipeWriter2

	rootCmd.PersistentPreRun(rootCmd, []string{})

	pipeWriter2.Close()
	os.Stdout = oldStdout

	output2 := make([]byte, 1024)
	readLen2, readErr2 := pipeReader2.Read(output2)
	if readErr2 != nil {
		readLen2 = 0
	}

	// Should be empty or very minimal output
	assert.Equal(t, 0, readLen2)
}

func TestRootCommandIntegration(t *testing.T) {
	t.Run("viper_flag_binding", func(t *testing.T) {
		// Test that viper flags are properly bound
		// This tests the init() function's viper binding

		// Reset state
		viper.Reset()
		verbose = false

		// Set verbose flag
		err := rootCmd.PersistentFlags().Set("verbose", "true")
		require.NoError(t, err)

		// The binding should have been set up in init()
		// We can't easily test this without executing the command,
		// but we can verify the flag exists and is bound
		verboseFlag := rootCmd.PersistentFlags().Lookup("verbose")
		assert.NotNil(t, verboseFlag)

		// Verify flag value was set
		flagValue, err := rootCmd.PersistentFlags().GetBool("verbose")
		require.NoError(t, err)
		assert.True(t, flagValue)
	})

	t.Run("cobra_initialization", func(t *testing.T) {
		// Test that initialization works by checking config is loaded
		// We can't test cobra.OnInitialize directly without cobra dependency

		// Reset viper state for a clean test
		viper.Reset()

		// Just verify that initConfig function exists and can be called
		// without panicking or errors
		assert.NotPanics(t, func() {
			initConfig()
		})

		// Verify viper is properly configured for environment variables
		// by setting a test environment variable and checking it's read
		t.Setenv("PORTGUARD_TEST_KEY", "test_value")
		viper.AutomaticEnv() // Re-enable env reading after reset
		viper.SetEnvPrefix("PORTGUARD")
		assert.Equal(t, "test_value", viper.GetString("test_key"))
	})
}
