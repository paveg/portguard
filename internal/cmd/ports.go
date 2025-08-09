package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	endPort int
)

var portsCmd = &cobra.Command{
	Use:   "ports",
	Short: "Show port usage information",
	Long: `Display port usage information including which ports are in use,
available ports, and which processes are using them.

Examples:
  portguard ports
  portguard ports --json
  portguard ports --start 3000 --end 4000`,
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Println("Scanning port usage...")

		if startPort > 0 && endPort > 0 {
			fmt.Printf("Scanning port range: %d-%d\n", startPort, endPort)
		}

		if jsonOutput {
			fmt.Println("JSON output requested")
		}

		// TODO: Implement actual port scanning logic
		fmt.Println("Port scanning not yet implemented")
	},
}

func init() {
	rootCmd.AddCommand(portsCmd)

	portsCmd.Flags().BoolVar(&jsonOutput, "json", false, "output in JSON format")
	portsCmd.Flags().IntVar(&startPort, "start", 3000, "start of port range to scan")
	portsCmd.Flags().IntVar(&endPort, "end", 9000, "end of port range to scan")
}
