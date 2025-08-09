package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status [id]",
	Short: "Show process status and health information",
	Long: `Show detailed status and health information for a specific process or all processes.
Includes port information, health check results, and resource usage.

Examples:
  portguard status
  portguard status abc123
  portguard status --json`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Println("Showing status for all processes...")
		} else {
			processID := args[0]
			fmt.Printf("Showing status for process: %s\n", processID)
		}

		if jsonOutput {
			fmt.Println("JSON output requested")
		}

		// TODO: Implement actual status checking logic
		fmt.Println("Status checking not yet implemented")
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)

	statusCmd.Flags().BoolVar(&jsonOutput, "json", false, "output in JSON format")
}
