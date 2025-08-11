package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/paveg/portguard/internal/config"
	"github.com/paveg/portguard/internal/lock"
	portpkg "github.com/paveg/portguard/internal/port"
	"github.com/paveg/portguard/internal/process"
	"github.com/paveg/portguard/internal/state"
	"github.com/spf13/cobra"
)

var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover running development servers",
	Long: `Discover automatically scans for running development servers that could be managed by portguard.
It identifies processes that match common development server patterns and shows information about them.

This command is useful for finding servers that you might want to import into portguard management.

Examples:
  portguard discover                    # Scan default port range (3000-9000)
  portguard discover --range 8000-8100 # Scan specific range
  portguard discover --auto-import     # Discover and automatically import suitable processes`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runDiscoverCommand(); err != nil {
			fmt.Printf("Discovery failed: %v\n", err)
			return
		}
	},
}

var (
	portRange  string
	autoImport bool
)

func runDiscoverCommand() error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create process adopter for discovery
	adopter := process.NewProcessAdopter(30 * time.Second)

	// Parse port range or use default
	var startPort, endPort int
	if portRange != "" {
		scanner := portpkg.NewScanner(5 * time.Second)
		startPort, endPort, err = scanner.ParsePortRange(portRange)
		if err != nil {
			return fmt.Errorf("invalid port range %s: %w", portRange, err)
		}
	} else {
		// Use default range from config or fallback
		if cfg.Default != nil && cfg.Default.PortRange != nil {
			startPort = cfg.Default.PortRange.Start
			endPort = cfg.Default.PortRange.End
		} else {
			startPort = 3000
			endPort = 9000
		}
	}

	fmt.Printf("Discovering development servers in port range %d-%d...\n", startPort, endPort)

	// Discover adoptable processes
	adoptableProcesses, err := adopter.DiscoverAdoptableProcesses(process.PortRange{
		Start: startPort,
		End:   endPort,
	})
	if err != nil {
		return fmt.Errorf("failed to discover processes: %w", err)
	}

	if len(adoptableProcesses) == 0 {
		fmt.Printf("No development servers found in port range %d-%d\n", startPort, endPort)
		return nil
	}

	fmt.Printf("Found %d development server(s):\n\n", len(adoptableProcesses))

	if jsonOutput {
		return outputDiscoveryResultsJSON(adoptableProcesses)
	}

	return outputDiscoveryResults(adoptableProcesses, autoImport)
}

func outputDiscoveryResults(processes []*process.AdoptionInfo, shouldAutoImport bool) error {
	var processManager *process.ProcessManager

	// Initialize process manager if auto-import is enabled
	if shouldAutoImport {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config for auto-import: %w", err)
		}

		stateStore, lockManager, portScanner, err := createDiscoveryManagementComponents(cfg)
		if err != nil {
			return fmt.Errorf("failed to create management components: %w", err)
		}

		processManager = process.NewProcessManager(stateStore, lockManager, portScanner)
	}

	for i, proc := range processes {
		fmt.Printf("[%d] Process: %s (PID: %d)\n", i+1, proc.ProcessName, proc.PID)
		fmt.Printf("    Command: %s\n", proc.Command)
		if proc.Port > 0 {
			fmt.Printf("    Port: %d\n", proc.Port)
		}
		if proc.WorkingDir != "" {
			fmt.Printf("    Working Directory: %s\n", proc.WorkingDir)
		}
		fmt.Printf("    Suitable for adoption: %v\n", proc.IsSuitable)
		if !proc.IsSuitable {
			fmt.Printf("    Reason: %s\n", proc.Reason)
		}

		// Auto-import if requested and process is suitable
		if shouldAutoImport && proc.IsSuitable {
			fmt.Print("    Auto-importing... ")
			if err := autoImportProcess(processManager, proc); err != nil {
				fmt.Printf("Failed: %v\n", err)
			} else {
				fmt.Println("Success ✓")
			}
		}

		fmt.Println()
	}

	if !shouldAutoImport && hasSuitableProcesses(processes) {
		fmt.Println("To import any of these processes, use:")
		for i, proc := range processes {
			if proc.IsSuitable {
				if proc.Port > 0 {
					fmt.Printf("  portguard import port %d  # Import process [%d]\n", proc.Port, i+1)
				} else {
					fmt.Printf("  portguard import pid %d   # Import process [%d]\n", proc.PID, i+1)
				}
			}
		}
	}

	return nil
}

func outputDiscoveryResultsJSON(processes []*process.AdoptionInfo) error {
	data, err := jsonMarshalIndent(map[string]interface{}{
		"discovered_processes": processes,
		"count":                len(processes),
		"suitable_count":       countSuitableProcesses(processes),
	})
	if err != nil {
		return fmt.Errorf("failed to marshal discovery results: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

func autoImportProcess(processManager *process.ProcessManager, adoptionInfo *process.AdoptionInfo) error {
	// Create process adopter
	adopter := process.NewProcessAdopter(30 * time.Second)

	// Adopt the process by PID
	managedProcess, err := adopter.AdoptProcessByPID(adoptionInfo.PID)
	if err != nil {
		return fmt.Errorf("failed to adopt process: %w", err)
	}

	// Add to process management
	if err := processManager.AdoptProcess(managedProcess); err != nil {
		return fmt.Errorf("failed to add to management: %w", err)
	}

	return nil
}

func hasSuitableProcesses(processes []*process.AdoptionInfo) bool {
	for _, proc := range processes {
		if proc.IsSuitable {
			return true
		}
	}
	return false
}

func countSuitableProcesses(processes []*process.AdoptionInfo) int {
	count := 0
	for _, proc := range processes {
		if proc.IsSuitable {
			count++
		}
	}
	return count
}

// createDiscoveryManagementComponents creates management components for discovery operations
func createDiscoveryManagementComponents(cfg *config.Config) (process.StateStore, process.LockManager, process.PortScanner, error) {
	// Get or create portguard directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	portguardDir := filepath.Join(homeDir, ".portguard")
	if mkdirErr := os.MkdirAll(portguardDir, 0o755); mkdirErr != nil {
		return nil, nil, nil, fmt.Errorf("failed to create portguard directory: %w", mkdirErr)
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

func init() {
	rootCmd.AddCommand(discoverCmd)

	// Add flags
	discoverCmd.Flags().StringVar(&portRange, "range", "", "port range to scan (e.g., '3000-4000')")
	discoverCmd.Flags().BoolVar(&autoImport, "auto-import", false, "automatically import suitable processes")
	discoverCmd.Flags().BoolVar(&jsonOutput, "json", false, "output results in JSON format")
}
