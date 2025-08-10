package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/paveg/portguard/internal/lock"
	portpkg "github.com/paveg/portguard/internal/port"
	"github.com/paveg/portguard/internal/process"
	"github.com/paveg/portguard/internal/state"
)

var startCmd = &cobra.Command{
	Use:   "start <command>",
	Short: "Start a new process or reuse existing one",
	Long: `Start a new process or reuse an existing one if the same command is already running.
Includes intelligent duplicate detection and port conflict resolution.

Examples:
  portguard start "go run main.go" --port 3000
  portguard start "npm run dev" --port 3001 --health-check http://localhost:3001/health
  portguard start "python app.py" --background`,
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		command := args[0]

		fmt.Printf("Starting command: %s\n", command)
		if port > 0 {
			fmt.Printf("Target port: %d\n", port)
		}
		if healthCheck != "" {
			fmt.Printf("Health check: %s\n", healthCheck)
		}
		if background {
			fmt.Println("Running in background mode")
		}

		// Initialize process manager
		pm, err := initializeProcessManager()
		if err != nil {
			return fmt.Errorf("failed to initialize process manager: %w", err)
		}

		// Parse command and arguments
		commandParts, err := parseCommand(command)
		if err != nil {
			return fmt.Errorf("failed to parse command: %w", err)
		}

		var cmd string
		var cmdArgs []string
		if len(commandParts) > 0 {
			cmd = commandParts[0]
			if len(commandParts) > 1 {
				cmdArgs = commandParts[1:]
			}
		}

		// Setup start options
		options := process.StartOptions{
			Port:       port,
			Background: background,
		}

		// Parse health check if provided
		if healthCheck != "" {
			healthCheckObj, parseErr := parseHealthCheck(healthCheck)
			if parseErr != nil {
				return fmt.Errorf("failed to parse health check: %w", parseErr)
			}
			options.HealthCheck = healthCheckObj
		}

		// Start the process
		process, err := pm.StartProcess(cmd, cmdArgs, options)
		if err != nil {
			return fmt.Errorf("failed to start process: %w", err)
		}

		fmt.Printf("✅ Process started successfully:\n")
		fmt.Printf("   ID: %s\n", process.ID)
		fmt.Printf("   PID: %d\n", process.PID)
		fmt.Printf("   Command: %s\n", process.Command)
		fmt.Printf("   Status: %s\n", process.Status)
		if process.Port > 0 {
			fmt.Printf("   Port: %d\n", process.Port)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(startCmd)

	startCmd.Flags().IntVarP(&port, "port", "p", 0, "target port for the process")
	startCmd.Flags().StringVar(&healthCheck, "health-check", "", "health check URL or command")
	startCmd.Flags().BoolVarP(&background, "background", "b", false, "run process in background")
}

// initializeProcessManager creates a new ProcessManager with default configurations
func initializeProcessManager() (*process.ProcessManager, error) {
	// Get home directory for state file
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}
	
	// Create .portguard directory if it doesn't exist
	portguardDir := filepath.Join(homeDir, ".portguard")
	if mkdirErr := os.MkdirAll(portguardDir, 0o755); mkdirErr != nil {
		return nil, fmt.Errorf("failed to create portguard directory: %w", mkdirErr)
	}

	// Initialize state store
	stateFile := filepath.Join(portguardDir, "state.json")
	stateStore, err := state.NewJSONStore(stateFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create state store: %w", err)
	}

	// Initialize lock manager
	lockFile := filepath.Join(portguardDir, "portguard.lock")
	lockManager := lock.NewFileLock(lockFile, 5*time.Second)

	// Initialize port scanner
	portScanner := portpkg.NewScanner(5 * time.Second)

	// Create and return process manager
	pm := process.NewProcessManager(stateStore, lockManager, portScanner)
	return pm, nil
}

// parseCommand parses a command string into command and arguments
func parseCommand(command string) ([]string, error) {
	// Simple parsing by splitting on whitespace
	// For more complex parsing with quotes, we'd need a proper shell parser
	parts := strings.Fields(strings.TrimSpace(command))
	if len(parts) == 0 {
		return nil, errors.New("empty command")
	}
	return parts, nil
}

// parseHealthCheck parses health check configuration
func parseHealthCheck(healthCheckStr string) (*process.HealthCheck, error) {
	if healthCheckStr == "" {
		return nil, nil
	}

	// Simple parsing - if it starts with http, it's an HTTP check
	if strings.HasPrefix(healthCheckStr, "http://") || strings.HasPrefix(healthCheckStr, "https://") {
		return &process.HealthCheck{
			Type:   process.HealthCheckHTTP,
			Target: healthCheckStr,
		}, nil
	}

	// If it contains a colon and looks like host:port, it's a TCP check
	if strings.Contains(healthCheckStr, ":") && !strings.Contains(healthCheckStr, " ") {
		return &process.HealthCheck{
			Type:   process.HealthCheckTCP,
			Target: healthCheckStr,
		}, nil
	}

	// Otherwise, treat it as a command
	return &process.HealthCheck{
		Type:   process.HealthCheckCommand,
		Target: healthCheckStr,
	}, nil
}
