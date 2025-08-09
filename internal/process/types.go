package process

import (
	"time"
)

// ProcessStatus represents the current status of a managed process
type ProcessStatus string

// Process status constants
const (
	StatusPending   ProcessStatus = "pending"   // Process is being started
	StatusRunning   ProcessStatus = "running"   // Process is running normally
	StatusStopped   ProcessStatus = "stopped"   // Process has been stopped
	StatusFailed    ProcessStatus = "failed"    // Process failed to start or crashed
	StatusUnhealthy ProcessStatus = "unhealthy" // Process is running but failing health checks
)

// HealthCheckType represents the type of health check to perform
type HealthCheckType string

// Health check type constants
const (
	HealthCheckHTTP    HealthCheckType = "http"    // HTTP endpoint health check
	HealthCheckTCP     HealthCheckType = "tcp"     // TCP connection health check
	HealthCheckCommand HealthCheckType = "command" // Custom command health check
	HealthCheckNone    HealthCheckType = "none"    // No health check
)

// HealthCheck defines how to check if a process is healthy
type HealthCheck struct {
	Type     HealthCheckType `json:"type"`
	Target   string          `json:"target"`   // URL for HTTP, address for TCP, command for command
	Interval time.Duration   `json:"interval"` // How often to check
	Timeout  time.Duration   `json:"timeout"`  // Timeout for each check
	Retries  int             `json:"retries"`  // Number of retries before marking unhealthy
	Enabled  bool            `json:"enabled"`  // Whether health checking is enabled
}

// ManagedProcess represents a process managed by portguard
type ManagedProcess struct {
	ID          string            `json:"id"`           // Unique identifier
	Command     string            `json:"command"`      // Command that was executed
	Args        []string          `json:"args"`         // Command arguments
	Port        int               `json:"port"`         // Primary port the process is using
	PID         int               `json:"pid"`          // Process ID
	Status      ProcessStatus     `json:"status"`       // Current status
	HealthCheck *HealthCheck      `json:"health_check"` // Health check configuration
	CreatedAt   time.Time         `json:"created_at"`   // When the process was started
	UpdatedAt   time.Time         `json:"updated_at"`   // Last status update
	LastSeen    time.Time         `json:"last_seen"`    // Last time process was confirmed running
	Environment map[string]string `json:"environment"`  // Environment variables
	WorkingDir  string            `json:"working_dir"`  // Working directory
	LogFile     string            `json:"log_file"`     // Path to log file
}

// IsHealthy checks if the process is considered healthy
func (p *ManagedProcess) IsHealthy() bool {
	return p.Status == StatusRunning
}

// IsRunning checks if the process is currently running
func (p *ManagedProcess) IsRunning() bool {
	return p.Status == StatusRunning || p.Status == StatusUnhealthy
}

// Age returns how long the process has been running
func (p *ManagedProcess) Age() time.Duration {
	return time.Since(p.CreatedAt)
}

// TimeSinceLastSeen returns how long since the process was last confirmed running
func (p *ManagedProcess) TimeSinceLastSeen() time.Duration {
	return time.Since(p.LastSeen)
}

// PortInfo represents information about a port
type PortInfo struct {
	Port        int    `json:"port"`         // Port number
	PID         int    `json:"pid"`          // Process ID using this port
	ProcessName string `json:"process_name"` // Name of the process
	IsManaged   bool   `json:"is_managed"`   // Whether this port is managed by portguard
	Protocol    string `json:"protocol"`     // TCP or UDP
}

// ProcessListOptions defines options for listing processes
type ProcessListOptions struct {
	IncludeStopped bool `json:"include_stopped"` // Include stopped processes
	JSONOutput     bool `json:"json_output"`     // Output in JSON format
	FilterByPort   int  `json:"filter_by_port"`  // Filter by specific port
}

// PortScanOptions defines options for port scanning
type PortScanOptions struct {
	StartPort int  `json:"start_port"` // Start of port range
	EndPort   int  `json:"end_port"`   // End of port range
	TCPOnly   bool `json:"tcp_only"`   // Only scan TCP ports
	UDPOnly   bool `json:"udp_only"`   // Only scan UDP ports
}
