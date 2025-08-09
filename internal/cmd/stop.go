package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
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
	Run: func(_ *cobra.Command, args []string) {
		target := args[0]

		if port, err := strconv.Atoi(target); err == nil {
			fmt.Printf("Stopping process on port: %d\n", port)
		} else {
			fmt.Printf("Stopping process with ID: %s\n", target)
		}

		if force {
			fmt.Println("Force stop enabled")
		}

		// TODO: Implement actual process stopping logic
		fmt.Println("Process stopping not yet implemented")
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)

	stopCmd.Flags().BoolVarP(&force, "force", "f", false, "force stop the process")
}
