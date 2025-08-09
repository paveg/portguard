package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all managed processes",
	Long: `List all managed processes with their status, ports, and health information.
Supports both human-readable table format and JSON output for AI tools.

Examples:
  portguard list
  portguard list --json
  portguard list --all`,
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Println("Listing managed processes...")

		if jsonOutput {
			fmt.Println("JSON output requested")
			// TODO: Implement JSON output format
		}

		if showAll {
			fmt.Println("Showing all processes (including stopped)")
		}

		// TODO: Implement actual process listing logic
		fmt.Println("Process listing not yet implemented")
	},
}

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().BoolVar(&jsonOutput, "json", false, "output in JSON format (AI-friendly)")
	listCmd.Flags().BoolVarP(&showAll, "all", "a", false, "show all processes including stopped ones")
}
