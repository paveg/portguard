package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/paveg/portguard/internal/process"
)

var stopCmd = &cobra.Command{
	Use:   "stop <id|port>",
	Short: "Stop a managed process",
	Long: `Stop a managed process by ID or port number.
Gracefully shuts down the process and cleans up resources.

Examples:
  portguard stop abc123
  portguard stop 3000
  portguard stop 3001 --force`,
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		target := args[0]

		// Initialize process manager
		pm, err := initializeProcessManager()
		if err != nil {
			return fmt.Errorf("failed to initialize process manager: %w", err)
		}

		// Check if target is a port number
		if port, err := strconv.Atoi(target); err == nil {
			fmt.Printf("Stopping process on port: %d\n", port)
			
			// Find processes by port
			options := process.ProcessListOptions{
				FilterByPort: port,
				IncludeStopped: false,
			}
			
			processes := pm.ListProcesses(options)
			if len(processes) == 0 {
				fmt.Printf("No running processes found on port %d\n", port)
				return nil
			}
			
			// Stop all processes on this port
			for _, proc := range processes {
				if err := pm.StopProcess(proc.ID, force); err != nil {
					fmt.Printf("Failed to stop process %s: %v\n", proc.ID, err)
				} else {
					fmt.Printf("✅ Process %s stopped successfully\n", proc.ID)
				}
			}
		} else {
			fmt.Printf("Stopping process with ID: %s\n", target)
			
			if force {
				fmt.Println("Force stop enabled")
			}
			
			// Stop by ID
			if err := pm.StopProcess(target, force); err != nil {
				return fmt.Errorf("failed to stop process %s: %w", target, err)
			}
			
			fmt.Printf("✅ Process %s stopped successfully\n", target)
		}
		
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)

	stopCmd.Flags().BoolVarP(&force, "force", "f", false, "force stop the process")
}
