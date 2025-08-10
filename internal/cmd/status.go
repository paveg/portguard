package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	portpkg "github.com/paveg/portguard/internal/port"
	"github.com/paveg/portguard/internal/process"
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
	RunE: func(_ *cobra.Command, args []string) error {
		// Initialize process manager
		pm, err := initializeProcessManager()
		if err != nil {
			return fmt.Errorf("failed to initialize process manager: %w", err)
		}

		// Handle single process status
		if len(args) == 1 {
			return handleSingleProcessStatus(pm, args[0])
		}

		// Handle system-wide status
		return handleSystemStatus(pm)
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)

	statusCmd.Flags().BoolVar(&jsonOutput, "json", false, "output in JSON format")
}

// ProcessStatus represents detailed status information for a process
type ProcessStatus struct {
	ID          string               `json:"id"`
	Command     string               `json:"command"`
	Args        []string             `json:"args"`
	Port        int                  `json:"port"`
	PID         int                  `json:"pid"`
	Status      string               `json:"status"`
	Healthy     bool                 `json:"healthy"`
	CreatedAt   time.Time            `json:"created_at"`
	UpdatedAt   time.Time            `json:"updated_at"`
	LastSeen    time.Time            `json:"last_seen"`
	Uptime      string               `json:"uptime"`
	Environment map[string]string    `json:"environment,omitempty"`
	WorkingDir  string               `json:"working_dir,omitempty"`
	LogFile     string               `json:"log_file,omitempty"`
	HealthCheck *process.HealthCheck `json:"health_check,omitempty"`
	PortInfo    *PortStatusInfo      `json:"port_info,omitempty"`
}

// PortStatusInfo represents port-related status information
type PortStatusInfo struct {
	InUse        bool   `json:"in_use"`
	IsPrivileged bool   `json:"is_privileged"`
	Type         string `json:"type"`
	Available    bool   `json:"available"`
}

// SystemStatus represents overall system status
type SystemStatus struct {
	TotalProcesses     int                    `json:"total_processes"`
	RunningProcesses   int                    `json:"running_processes"`
	StoppedProcesses   int                    `json:"stopped_processes"`
	HealthyProcesses   int                    `json:"healthy_processes"`
	UnhealthyProcesses int                    `json:"unhealthy_processes"`
	PortsInUse         int                    `json:"ports_in_use"`
	SystemUptime       string                 `json:"system_uptime,omitempty"`
	CheckedAt          time.Time              `json:"checked_at"`
	Processes          []ProcessStatus        `json:"processes"`
	PortSummary        map[string]interface{} `json:"port_summary"`
}

// handleSingleProcessStatus shows detailed status for a specific process
func handleSingleProcessStatus(pm *process.ProcessManager, processID string) error {
	proc, exists := pm.GetProcess(processID)
	if !exists {
		return fmt.Errorf("process %s not found", processID)
	}

	fmt.Printf("Getting detailed status for process %s...\n", processID)

	// Create port scanner for additional port information
	scanner := portpkg.NewScanner(2 * time.Second)

	status := convertToProcessStatus(proc, scanner)

	if jsonOutput {
		output, err := json.MarshalIndent(status, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(output))
		return nil
	}

	// Text output
	fmt.Printf("\nProcess Status Details:\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("  ID: %s\n", status.ID)
	fmt.Printf("  Command: %s\n", status.Command)
	if len(status.Args) > 0 {
		fmt.Printf("  Arguments: %v\n", status.Args)
	}
	fmt.Printf("  PID: %d\n", status.PID)
	fmt.Printf("  Status: %s\n", status.Status)
	fmt.Printf("  Healthy: %v\n", status.Healthy)
	if status.Port > 0 {
		fmt.Printf("  Port: %d\n", status.Port)
		if status.PortInfo != nil {
			fmt.Printf("    Port In Use: %v\n", status.PortInfo.InUse)
			fmt.Printf("    Port Type: %s\n", status.PortInfo.Type)
			fmt.Printf("    Available: %v\n", status.PortInfo.Available)
		}
	}
	fmt.Printf("  Created: %s\n", status.CreatedAt.Format(time.RFC3339))
	fmt.Printf("  Updated: %s\n", status.UpdatedAt.Format(time.RFC3339))
	fmt.Printf("  Last Seen: %s\n", status.LastSeen.Format(time.RFC3339))
	fmt.Printf("  Uptime: %s\n", status.Uptime)

	if status.WorkingDir != "" {
		fmt.Printf("  Working Dir: %s\n", status.WorkingDir)
	}
	if status.LogFile != "" {
		fmt.Printf("  Log File: %s\n", status.LogFile)
	}
	if len(status.Environment) > 0 {
		fmt.Printf("  Environment Variables: %d set\n", len(status.Environment))
	}
	if status.HealthCheck != nil {
		fmt.Printf("  Health Check: Configured\n")
	}

	return nil
}

// handleSystemStatus shows overall system status
func handleSystemStatus(pm *process.ProcessManager) error {
	fmt.Println("Getting system-wide status...")

	// Get all processes
	allOptions := process.ProcessListOptions{IncludeStopped: true}
	allProcesses := pm.ListProcesses(allOptions)

	// Get running processes
	runningOptions := process.ProcessListOptions{IncludeStopped: false}
	runningProcesses := pm.ListProcesses(runningOptions)

	// Create port scanner
	scanner := portpkg.NewScanner(2 * time.Second)

	// Calculate statistics
	var healthyCount, unhealthyCount, stoppedCount int
	var portsInUse []int
	processStatuses := make([]ProcessStatus, 0, len(allProcesses))

	for _, proc := range allProcesses {
		status := convertToProcessStatus(proc, scanner)
		processStatuses = append(processStatuses, status)

		if proc.IsRunning() {
			if status.Healthy {
				healthyCount++
			} else {
				unhealthyCount++
			}
		} else {
			stoppedCount++
		}

		if proc.Port > 0 && proc.IsRunning() {
			portsInUse = append(portsInUse, proc.Port)
		}
	}

	// Get port summary
	listeningPorts, _ := scanner.GetListeningPorts()
	portSummary := map[string]interface{}{
		"managed_ports":   len(portsInUse),
		"listening_ports": len(listeningPorts),
		"port_range":      "3000-9000",
	}

	systemStatus := SystemStatus{
		TotalProcesses:     len(allProcesses),
		RunningProcesses:   len(runningProcesses),
		StoppedProcesses:   stoppedCount,
		HealthyProcesses:   healthyCount,
		UnhealthyProcesses: unhealthyCount,
		PortsInUse:         len(portsInUse),
		CheckedAt:          time.Now(),
		Processes:          processStatuses,
		PortSummary:        portSummary,
	}

	if jsonOutput {
		output, err := json.MarshalIndent(systemStatus, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(output))
		return nil
	}

	// Text output
	fmt.Printf("\nSystem Status Summary:\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("  Total Processes: %d\n", systemStatus.TotalProcesses)
	fmt.Printf("  Running: %d | Stopped: %d\n", systemStatus.RunningProcesses, systemStatus.StoppedProcesses)
	fmt.Printf("  Healthy: %d | Unhealthy: %d\n", systemStatus.HealthyProcesses, systemStatus.UnhealthyProcesses)
	fmt.Printf("  Ports In Use: %d\n", systemStatus.PortsInUse)
	fmt.Printf("  Checked At: %s\n", systemStatus.CheckedAt.Format(time.RFC3339))

	// Show process summary if any exist
	if len(processStatuses) > 0 {
		fmt.Printf("\nProcess Summary:\n")
		fmt.Printf("%-12s %-10s %-10s %-6s %-s\n", "PROCESS ID", "STATUS", "HEALTHY", "PORT", "COMMAND")
		fmt.Println("────────────────────────────────────────────────────────────────────")

		for i := range processStatuses {
			status := &processStatuses[i]
			healthyStr := "No"
			if status.Healthy {
				healthyStr = "Yes"
			}

			portStr := "-"
			if status.Port > 0 {
				portStr = strconv.Itoa(status.Port)
			}

			fmt.Printf("%-12s %-10s %-10s %-6s %-s\n",
				status.ID[:8]+"...",
				status.Status,
				healthyStr,
				portStr,
				status.Command)
		}
	} else {
		fmt.Printf("\nNo processes currently managed.\n")
	}

	return nil
}

// convertToProcessStatus converts a ManagedProcess to ProcessStatus with additional information
func convertToProcessStatus(proc *process.ManagedProcess, scanner *portpkg.Scanner) ProcessStatus {
	status := ProcessStatus{
		ID:          proc.ID,
		Command:     proc.Command,
		Args:        proc.Args,
		Port:        proc.Port,
		PID:         proc.PID,
		Status:      string(proc.Status),
		Healthy:     proc.IsHealthy(),
		CreatedAt:   proc.CreatedAt,
		UpdatedAt:   proc.UpdatedAt,
		LastSeen:    proc.LastSeen,
		Uptime:      time.Since(proc.CreatedAt).String(),
		Environment: proc.Environment,
		WorkingDir:  proc.WorkingDir,
		LogFile:     proc.LogFile,
		HealthCheck: proc.HealthCheck,
	}

	// Add port information if port is specified
	if proc.Port > 0 {
		inUse := scanner.IsPortInUse(proc.Port)
		isPrivileged := scanner.IsPrivilegedPort(proc.Port)
		portType := "user"
		if isPrivileged {
			portType = "privileged"
		}

		status.PortInfo = &PortStatusInfo{
			InUse:        inUse,
			IsPrivileged: isPrivileged,
			Type:         portType,
			Available:    !inUse,
		}
	}

	return status
}
