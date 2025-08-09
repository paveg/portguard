package cmd

import (
	"fmt"

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
	Run: func(cmd *cobra.Command, args []string) {
		if dryRun {
			fmt.Println("Dry run mode - showing what would be cleaned:")
		} else {
			fmt.Println("Cleaning up all managed processes...")
		}

		if force {
			fmt.Println("Force cleanup enabled")
		}

		// TODO: Implement actual cleanup logic
		fmt.Println("Cleanup not yet implemented")
	},
}

func init() {
	rootCmd.AddCommand(cleanCmd)

	cleanCmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be cleaned without actually doing it")
	cleanCmd.Flags().BoolVarP(&force, "force", "f", false, "force cleanup without confirmation")
}
