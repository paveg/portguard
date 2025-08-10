package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/paveg/portguard/internal/process"
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
	RunE: func(_ *cobra.Command, _ []string) error {
		fmt.Println("Listing managed processes...")

		if showAll {
			fmt.Println("Showing all processes (including stopped)")
		}

		// Initialize process manager
		pm, err := initializeProcessManager()
		if err != nil {
			return fmt.Errorf("failed to initialize process manager: %w", err)
		}

		// Get process list options
		options := process.ProcessListOptions{
			IncludeStopped: showAll,
		}

		processes := pm.ListProcesses(options)

		if jsonOutput {
			data := map[string]interface{}{
				"processes": processes,
				"total":     len(processes),
			}

			output, err := json.MarshalIndent(data, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Println(string(output))
			return nil
		}

		// Text output
		if len(processes) == 0 {
			fmt.Println("No processes found")
			return nil
		}

		fmt.Printf("Found %d process(es):\n\n", len(processes))

		// Table header
		fmt.Printf("%-10s %-8s %-10s %-6s %-s\n", "ID", "PID", "STATUS", "PORT", "COMMAND")
		fmt.Println("------------------------------------------------------------------------")

		for _, proc := range processes {
			portStr := "-"
			if proc.Port > 0 {
				portStr = strconv.Itoa(proc.Port)
			}

			fmt.Printf("%-10s %-8d %-10s %-6s %-s\n",
				proc.ID[:8], proc.PID, proc.Status, portStr, proc.Command)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().BoolVar(&jsonOutput, "json", false, "output in JSON format (AI-friendly)")
	listCmd.Flags().BoolVarP(&showAll, "all", "a", false, "show all processes including stopped ones")
}
