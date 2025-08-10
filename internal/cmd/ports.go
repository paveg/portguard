package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	portpkg "github.com/paveg/portguard/internal/port"
	"github.com/spf13/cobra"
)

const unknownProcessName = "unknown"

var (
	checkPort int
	endPort   int
)

var portsCmd = &cobra.Command{
	Use:   "ports",
	Short: "Show port usage information",
	Long: `Display port usage information including which ports are in use,
available ports, and which processes are using them.

Examples:
  portguard ports
  portguard ports --json
  portguard ports --start 3000 --end 4000
  portguard ports --check 3000`,
	RunE: func(_ *cobra.Command, _ []string) error {
		// Initialize port scanner
		scanner := portpkg.NewScanner(5 * time.Second)

		// Handle single port check
		if checkPort > 0 {
			return handleSinglePortCheck(scanner, checkPort)
		}

		// Handle port range scanning
		if startPort > 0 && endPort > 0 {
			return handlePortRangeScanning(scanner, startPort, endPort)
		}

		// Default: show listening ports
		return handleListeningPorts(scanner)
	},
}

func init() {
	rootCmd.AddCommand(portsCmd)

	portsCmd.Flags().BoolVar(&jsonOutput, "json", false, "output in JSON format")
	portsCmd.Flags().IntVar(&checkPort, "check", 0, "check if specific port is in use")
	portsCmd.Flags().IntVar(&startPort, "start", 3000, "start of port range to scan")
	portsCmd.Flags().IntVar(&endPort, "end", 9000, "end of port range to scan")
}

// handleSinglePortCheck checks if a specific port is in use
func handleSinglePortCheck(scanner *portpkg.Scanner, port int) error {
	inUse := scanner.IsPortInUse(port)

	if jsonOutput {
		result := map[string]interface{}{
			"port":    port,
			"in_use":  inUse,
			"checked": time.Now().Format(time.RFC3339),
		}

		if inUse {
			if portInfo, err := scanner.GetPortInfo(port); err == nil {
				result["process_id"] = portInfo.PID
				result["process_name"] = portInfo.ProcessName
			}
		}

		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(output))
		return nil
	}

	// Text output
	if inUse {
		fmt.Printf("Port %d is IN USE", port)
		if portInfo, err := scanner.GetPortInfo(port); err == nil {
			fmt.Printf(" by process %s (PID: %d)", portInfo.ProcessName, portInfo.PID)
		}
		fmt.Println()
	} else {
		fmt.Printf("Port %d is AVAILABLE\n", port)
	}

	return nil
}

// handlePortRangeScanning scans a range of ports
func handlePortRangeScanning(scanner *portpkg.Scanner, start, end int) error {
	if start > end {
		return fmt.Errorf("start port (%d) cannot be greater than end port (%d)", start, end)
	}

	fmt.Printf("Scanning ports %d-%d...\n", start, end)

	portInfos, err := scanner.ScanRange(start, end)
	if err != nil {
		return fmt.Errorf("failed to scan port range: %w", err)
	}

	if jsonOutput {
		result := map[string]interface{}{
			"range":        fmt.Sprintf("%d-%d", start, end),
			"scanned_at":   time.Now().Format(time.RFC3339),
			"total_ports":  len(portInfos),
			"ports_in_use": portInfos,
		}

		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(output))
		return nil
	}

	// Text output
	if len(portInfos) == 0 {
		fmt.Printf("No ports in use in range %d-%d\n", start, end)
		return nil
	}

	fmt.Printf("Found %d ports in use:\n\n", len(portInfos))
	fmt.Printf("%-6s %-8s %-s\n", "PORT", "PID", "PROCESS")
	fmt.Println("--------------------------------")

	for _, port := range portInfos {
		pidStr := "-"
		if port.PID > 0 {
			pidStr = strconv.Itoa(port.PID)
		}
		processName := port.ProcessName
		if processName == "" {
			processName = unknownProcessName
		}
		fmt.Printf("%-6d %-8s %-s\n", port.Port, pidStr, processName)
	}

	return nil
}

// handleListeningPorts shows all listening ports on the system
func handleListeningPorts(scanner *portpkg.Scanner) error {
	fmt.Println("Scanning for listening ports...")

	ports, err := scanner.GetListeningPorts()
	if err != nil {
		return fmt.Errorf("failed to get listening ports: %w", err)
	}

	if jsonOutput {
		result := map[string]interface{}{
			"scanned_at":      time.Now().Format(time.RFC3339),
			"total_ports":     len(ports),
			"listening_ports": ports,
		}

		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(output))
		return nil
	}

	// Text output
	if len(ports) == 0 {
		fmt.Println("No listening ports found")
		return nil
	}

	fmt.Printf("Found %d listening ports:\n\n", len(ports))
	fmt.Printf("%-6s %-8s %-15s %-s\n", "PORT", "PID", "TYPE", "PROCESS")
	fmt.Println("------------------------------------------------")

	for _, port := range ports {
		pidStr := "-"
		if port.PID > 0 {
			pidStr = strconv.Itoa(port.PID)
		}

		portType := "user"
		if scanner.IsPrivilegedPort(port.Port) {
			portType = "privileged"
		}

		processName := port.ProcessName
		if processName == "" {
			processName = unknownProcessName
		}

		fmt.Printf("%-6d %-8s %-15s %-s\n", port.Port, pidStr, portType, processName)
	}

	return nil
}
