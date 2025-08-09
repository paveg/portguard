package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management commands",
	Long: `Manage portguard configuration including project settings,
default values, and initialization of configuration files.`,
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize default configuration",
	Long: `Initialize a default configuration file in the current directory
or specified location. This creates a .portguard.yml file with sensible defaults.`,
	Run: func(_ *cobra.Command, _ []string) {
		configPath := ".portguard.yml"
		if configFile != "" {
			configPath = configFile
		}

		// Check if config already exists
		if _, err := os.Stat(configPath); err == nil {
			if !force {
				fmt.Printf("Configuration file %s already exists. Use --force to overwrite.\n", configPath)
				return
			}
		}

		// Create default configuration content
		defaultConfig := `# Portguard Configuration
# AI-aware process management tool

default:
  health_check:
    enabled: true
    timeout: 5s
    interval: 30s
    retries: 3
  
  port_range:
    start: 3000
    end: 9000
  
  cleanup:
    auto_cleanup: true
    max_idle_time: 1h
    backup_retention: 7d
  
  log_level: info

# Project-specific configurations
projects:
  # Example web application
  web:
    command: "npm run dev"
    port: 3000
    health_check:
      type: http
      target: "http://localhost:3000/health"
    environment:
      NODE_ENV: "development"
  
  # Example API server
  api:
    command: "go run main.go"
    port: 3001
    health_check:
      type: http
      target: "http://localhost:3001/api/health"
    working_dir: "./api"
  
  # Example background service
  worker:
    command: "python worker.py"
    health_check:
      type: command
      target: "curl -f http://localhost:3002/status"
    log_file: "./logs/worker.log"
`

		// Write configuration file atomically
		if err := WriteFileAtomic(configPath, []byte(defaultConfig)); err != nil {
			fmt.Printf("Error creating configuration file: %v\n", err)
			return
		}

		fmt.Printf("Configuration file created: %s\n", configPath)
		fmt.Println("Edit this file to customize your project settings.")
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Long:  `Display the current configuration values including defaults and project settings.`,
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Println("Current Configuration:")
		fmt.Printf("Config file: %s\n", viper.ConfigFileUsed())

		if jsonOutput {
			// Output all configuration as JSON
			allSettings := viper.AllSettings()
			if data, err := jsonMarshalIndent(allSettings); err == nil {
				fmt.Println(string(data))
			} else {
				fmt.Printf("Error marshaling config: %v\n", err)
			}
		} else {
			// Human-readable output
			fmt.Println("\nDefault Settings:")
			fmt.Printf("  State file: %s\n", viper.GetString("default.state_file"))
			fmt.Printf("  Lock file: %s\n", viper.GetString("default.lock_file"))
			fmt.Printf("  Port range: %d-%d\n",
				viper.GetInt("default.port_range.start"),
				viper.GetInt("default.port_range.end"))
			fmt.Printf("  Health check enabled: %v\n", viper.GetBool("default.health_check.enabled"))
			fmt.Printf("  Auto cleanup: %v\n", viper.GetBool("default.cleanup.auto_cleanup"))

			// Show project configurations
			projects := viper.GetStringMap("projects")
			if len(projects) > 0 {
				fmt.Println("\nProject Configurations:")
				for name := range projects {
					fmt.Printf("  %s:\n", name)
					fmt.Printf("    Command: %s\n", viper.GetString(fmt.Sprintf("projects.%s.command", name)))
					//nolint:govet // TODO: Rename variable to avoid shadowing (e.g., projectPort)
					if port := viper.GetInt(fmt.Sprintf("projects.%s.port", name)); port > 0 {
						fmt.Printf("    Port: %d\n", port)
					}
				}
			}
		}
	},
}

var (
	configFile string
)

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configShowCmd)

	configInitCmd.Flags().StringVar(&configFile, "file", "", "configuration file path")
	configInitCmd.Flags().BoolVarP(&force, "force", "f", false, "overwrite existing configuration")

	configShowCmd.Flags().BoolVar(&jsonOutput, "json", false, "output in JSON format")
}

// Helper function for JSON marshaling with indentation
func jsonMarshalIndent(v interface{}) ([]byte, error) {
	return fmt.Appendf(nil, "%+v", v), nil // Simplified for now
}
