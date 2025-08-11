package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/paveg/portguard/internal/config"
	"github.com/paveg/portguard/internal/lock"
	portpkg "github.com/paveg/portguard/internal/port"
	"github.com/paveg/portguard/internal/process"
	"github.com/paveg/portguard/internal/state"
	"github.com/spf13/cobra"
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import existing processes into portguard management",
	Long: `Import allows you to take control of existing processes and manage them with portguard.
This is useful for processes that were started outside of portguard but you want to monitor and manage.

Examples:
  portguard import --port 8080          # Import process running on port 8080
  portguard import --pid 12345          # Import process with PID 12345
  portguard import --port 3000 --name my-app  # Import with custom name`,
}

var importPortCmd = &cobra.Command{
	Use:   "port <port>",
	Short: "Import a process by port number",
	Long: `Import a process by specifying the port it's running on.
Portguard will detect the process using that port and add it to management.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		portNum, err := strconv.Atoi(args[0])
		if err != nil {
			fmt.Printf("Invalid port number: %s\n", args[0])
			return
		}

		if err := importProcessByPort(portNum); err != nil {
			fmt.Printf("Failed to import process on port %d: %v\n", portNum, err)
			return
		}

		fmt.Printf("Successfully imported process on port %d\n", portNum)
	},
}

var importPidCmd = &cobra.Command{
	Use:   "pid <pid>",
	Short: "Import a process by PID",
	Long: `Import a process by specifying its Process ID (PID).
Portguard will adopt the process and add it to management.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pid, err := strconv.Atoi(args[0])
		if err != nil {
			fmt.Printf("Invalid PID: %s\n", args[0])
			return
		}

		if err := importProcessByPID(pid); err != nil {
			fmt.Printf("Failed to import process with PID %d: %v\n", pid, err)
			return
		}

		fmt.Printf("Successfully imported process with PID %d\n", pid)
	},
}

func importProcessByPort(port int) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create process adopter
	adopter := process.NewProcessAdopter(30 * time.Second)

	// Adopt process by port
	managedProcess, err := adopter.AdoptProcessByPort(port)
	if err != nil {
		return fmt.Errorf("failed to adopt process: %w", err)
	}

	// Create process manager to save the adopted process
	stateStore, lockManager, portScanner, err := createManagementComponents(cfg)
	if err != nil {
		return fmt.Errorf("failed to create management components: %w", err)
	}

	processManager := process.NewProcessManager(stateStore, lockManager, portScanner)

	// Add the adopted process to management
	if err := addAdoptedProcess(processManager, managedProcess); err != nil {
		return fmt.Errorf("failed to add adopted process to management: %w", err)
	}

	return nil
}

func importProcessByPID(pid int) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create process adopter
	adopter := process.NewProcessAdopter(30 * time.Second)

	// Adopt process by PID
	managedProcess, err := adopter.AdoptProcessByPID(pid)
	if err != nil {
		return fmt.Errorf("failed to adopt process: %w", err)
	}

	// Create process manager to save the adopted process
	stateStore, lockManager, portScanner, err := createManagementComponents(cfg)
	if err != nil {
		return fmt.Errorf("failed to create management components: %w", err)
	}

	processManager := process.NewProcessManager(stateStore, lockManager, portScanner)

	// Add the adopted process to management
	if err := addAdoptedProcess(processManager, managedProcess); err != nil {
		return fmt.Errorf("failed to add adopted process to management: %w", err)
	}

	return nil
}

func addAdoptedProcess(processManager *process.ProcessManager, managedProcess *process.ManagedProcess) error {
	// Use the new AdoptProcess method
	return processManager.AdoptProcess(managedProcess)
}

// createManagementComponents creates the necessary components for process management
func createManagementComponents(cfg *config.Config) (process.StateStore, process.LockManager, process.PortScanner, error) {
	// Get or create portguard directory
	portguardDir, err := getPortguardDir()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get portguard directory: %w", err)
	}

	// Initialize state store
	stateFile := filepath.Join(portguardDir, "state.json")
	stateStore, err := state.NewJSONStore(stateFile)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create state store: %w", err)
	}

	// Initialize lock manager
	lockFile := filepath.Join(portguardDir, "portguard.lock")
	lockManager := lock.NewFileLock(lockFile, 5*time.Second)

	// Initialize port scanner
	portScanner := portpkg.NewScanner(5 * time.Second)

	return stateStore, lockManager, portScanner, nil
}

// getPortguardDir gets or creates the portguard directory
func getPortguardDir() (string, error) {
	// Get home directory for state file
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	// Create .portguard directory if it doesn't exist
	portguardDir := filepath.Join(homeDir, ".portguard")
	if mkdirErr := os.MkdirAll(portguardDir, 0o755); mkdirErr != nil {
		return "", fmt.Errorf("failed to create portguard directory: %w", mkdirErr)
	}

	return portguardDir, nil
}

func init() {
	rootCmd.AddCommand(importCmd)
	importCmd.AddCommand(importPortCmd)
	importCmd.AddCommand(importPidCmd)

	// Add flags
	importCmd.PersistentFlags().StringVar(&processName, "name", "", "custom name for the imported process")
	importCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output in JSON format")
}

var processName string
