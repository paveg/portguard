package cmd

import (
	"fmt"

	"github.com/paveg/portguard/internal/process"
	"github.com/spf13/cobra"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean up all managed processes",
	Long: `Stop all managed processes and clean up resources.
Use with caution as this will terminate all processes managed by portguard.

Examples:
  portguard clean --dry-run
  portguard clean --force`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Initialize process manager
		pm, err := initializeProcessManager()
		if err != nil {
			return fmt.Errorf("failed to initialize process manager: %w", err)
		}

		if dryRun {
			fmt.Println("Dry run mode - showing what would be cleaned:")

			// Show what would be cleaned
			options := process.ProcessListOptions{
				IncludeStopped: true,
			}

			processes := pm.ListProcesses(options)
			stoppedCount := 0

			for _, proc := range processes {
				if proc.Status == "stopped" || proc.Status == "failed" {
					stoppedCount++
					fmt.Printf("  - Process %s (%s): %s\n", proc.ID[:8], proc.Status, proc.Command)
				}
			}

			fmt.Printf("\nWould clean up %d stopped/failed process(es)\n", stoppedCount)
			return nil
		}

		fmt.Println("Cleaning up all managed processes...")

		if force {
			fmt.Println("Force cleanup enabled")
		}

		// Perform cleanup
		if err := pm.CleanupProcesses(force); err != nil {
			return fmt.Errorf("cleanup failed: %w", err)
		}

		fmt.Println("✅ Cleanup completed successfully")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(cleanCmd)

	cleanCmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be cleaned without actually doing it")
	cleanCmd.Flags().BoolVarP(&force, "force", "f", false, "force cleanup without confirmation")
}
