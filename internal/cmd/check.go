// Package cmd provides command-line interface functionality for Portguard.
// It implements all CLI commands using Cobra framework for the AI-aware process management tool.
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Quick status check (AI-friendly)",
	Long: `Perform a quick status check optimized for AI tools and automated scripts.
Returns concise information about process and port status.

This command is designed to be easily parsable by AI development tools
and provides the most commonly needed information in a simple format.

Examples:
  portguard check --port 3000
  portguard check --json
  portguard check --available --start 3000`,
	Run: func(_ *cobra.Command, _ []string) {
		runner := NewCommandRunner(jsonOutput, false)

		result := map[string]interface{}{
			"portguard_running": true,
			"timestamp":         "2024-01-15T10:30:00Z", // TODO: Use actual timestamp
			"managed_processes": 0,                      // TODO: Get actual count
			"running_processes": 0,                      // TODO: Get actual count
		}

		// Port status if requested
		if port > 0 {
			result["port"] = port
			result["port_in_use"] = checkPortInUse(port)
			result["managed_by_portguard"] = false // TODO: Check if managed
		}

		// Available port if requested
		if availablePort {
			if startPort == 0 {
				startPort = 3000
			}
			result["available_port"] = findAvailablePort(startPort)
		}

		if runner.OutputHandler.JSONOutput {
			if err := runner.OutputHandler.PrintJSON(result); err != nil {
				runner.OutputHandler.PrintError("Failed to marshal JSON", err)
				return
			}
		} else {
			// Human-readable output
			fmt.Println("Portguard Status:")
			if port > 0 {
				portInUse, ok := result["port_in_use"].(bool)
				if !ok {
					fmt.Printf("  Port %d: STATUS UNKNOWN\n", port)
				} else if portInUse {
					fmt.Printf("  Port %d: IN USE\n", port)
				} else {
					fmt.Printf("  Port %d: AVAILABLE\n", port)
				}
			}
			if availablePort {
				fmt.Printf("  Next available port: %d\n", result["available_port"])
			}
			fmt.Printf("  Managed processes: %d\n", result["managed_processes"])
		}
	},
}

var (
	availablePort bool
)

func init() {
	rootCmd.AddCommand(checkCmd)

	AddCommonPortFlags(checkCmd)
	AddCommonJSONFlag(checkCmd)
	checkCmd.Flags().BoolVar(&availablePort, "available", false, "find next available port")
}

// Helper functions (these would typically use the real port scanner)
func checkPortInUse(_ int) bool {
	// TODO: Use actual port scanner
	return false
}

func findAvailablePort(start int) int {
	// TODO: Use actual port scanner
	return start
}
