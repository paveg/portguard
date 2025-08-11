// Package port provides port availability scanning functionality for Portguard.
// It implements cross-platform port detection and health checking for both TCP and UDP protocols.
package port

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Static error variables to satisfy err113 linter
var (
	ErrNoAvailablePort     = errors.New("no available port found")
	ErrUnsupportedPlatform = errors.New("unsupported platform")
	ErrProcessInfoNotImpl  = errors.New("process info not implemented")
	ErrInvalidPortRange    = errors.New("invalid port range format")
	ErrPortRangeOrder      = errors.New("start port must be less than end port")
)

// Scanner implements PortScanner interface for cross-platform port scanning
type Scanner struct {
	timeout time.Duration
}

// PortInfo represents information about a port
type PortInfo struct {
	Port        int    `json:"port"`         // Port number
	PID         int    `json:"pid"`          // Process ID using this port
	ProcessName string `json:"process_name"` // Name of the process
	IsManaged   bool   `json:"is_managed"`   // Whether this port is managed by portguard
	Protocol    string `json:"protocol"`     // TCP or UDP
}

// NewScanner creates a new port scanner
func NewScanner(timeout time.Duration) *Scanner {
	return &Scanner{
		timeout: timeout,
	}
}

// IsPortInUse checks if a specific port is currently in use
func (s *Scanner) IsPortInUse(port int) bool {
	// Try to bind to the port - if we can't, it's in use
	// Use localhost to match common development server binding
	address := fmt.Sprintf("127.0.0.1:%d", port)

	// Check TCP
	if listener, err := net.Listen("tcp", address); err == nil { //nolint:noctx // TODO: Add context support for port scanning operations
		_ = listener.Close() //nolint:errcheck // Best effort cleanup during port scan
	} else {
		return true // Port is in use
	}

	// Check UDP
	if conn, err := net.ListenPacket("udp", address); err == nil { //nolint:noctx // TODO: Add context support for port scanning operations
		_ = conn.Close() //nolint:errcheck // Best effort cleanup during port scan
	} else {
		return true // Port is in use
	}

	return false
}

// GetPortInfo retrieves detailed information about a specific port
func (s *Scanner) GetPortInfo(port int) (*PortInfo, error) {
	portInfo := &PortInfo{
		Port:        port,
		PID:         -1,
		ProcessName: "",
		IsManaged:   false,
		Protocol:    "tcp",
	}

	// Check if port is in use
	if !s.IsPortInUse(port) {
		return portInfo, nil // Port is available
	}

	// Try to get process information using platform-specific methods
	if pid, processName, err := s.getProcessInfoForPort(port); err == nil {
		portInfo.PID = pid
		portInfo.ProcessName = processName
	}

	return portInfo, nil
}

// ScanRange scans a range of ports and returns information about ports in use
func (s *Scanner) ScanRange(startPort, endPort int) ([]PortInfo, error) {
	// Validate port range
	if startPort > endPort {
		return nil, fmt.Errorf("%w: start port must be less than end port", ErrPortRangeOrder)
	}
	if startPort <= 0 || endPort <= 0 || startPort > 65535 || endPort > 65535 {
		return nil, fmt.Errorf("%w: invalid port range format", ErrInvalidPortRange)
	}

	var result []PortInfo

	for port := startPort; port <= endPort; port++ {
		if s.IsPortInUse(port) {
			if portInfo, err := s.GetPortInfo(port); err == nil {
				result = append(result, *portInfo)
			}
		}
		// FIXED: Only add ports that are actually in use
		// Removed the else block that was adding unused ports
	}

	return result, nil
}

// FindAvailablePort finds the first available port starting from the given port
func (s *Scanner) FindAvailablePort(startPort int) (int, error) {
	maxAttempts := 1000 // Prevent infinite loops

	for i := 0; i < maxAttempts; i++ {
		port := startPort + i
		if port > 65535 { //nolint:mnd // TODO: Extract max valid port number to const
			break // Exceeded valid port range
		}

		if !s.IsPortInUse(port) {
			return port, nil
		}
	}

	return 0, fmt.Errorf("%w starting from %d", ErrNoAvailablePort, startPort)
}

// getProcessInfoForPort attempts to get process information for a port
// This is platform-specific and may not work on all systems
func (s *Scanner) getProcessInfoForPort(port int) (int, string, error) {
	switch runtime.GOOS {
	case "darwin", "linux":
		return s.getProcessInfoUnix(port)
	case "windows":
		return s.getProcessInfoWindows(port)
	default:
		return -1, "", fmt.Errorf("%w: %s", ErrUnsupportedPlatform, runtime.GOOS)
	}
}

// getProcessInfoUnix gets process info on Unix-like systems using lsof-like approach
func (s *Scanner) getProcessInfoUnix(port int) (int, string, error) {
	// Use lsof to get process information for the port
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	// Try lsof first (more reliable for process info)
	cmd := exec.CommandContext(ctx, "lsof", "-ti", fmt.Sprintf(":%d", port))
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		// Parse PID from lsof output
		pidStr := strings.TrimSpace(string(output))
		if pid, parseErr := strconv.Atoi(strings.Fields(pidStr)[0]); parseErr == nil {
			// Get process name using ps
			psCmd := exec.CommandContext(ctx, "ps", "-p", strconv.Itoa(pid), "-o", "comm=")
			psOutput, psErr := psCmd.Output()
			if psErr == nil {
				processName := strings.TrimSpace(string(psOutput))
				return pid, processName, nil
			}
			// If ps fails, return PID without name
			return pid, "unknown", nil
		}
	}

	// Fallback to netstat if lsof is not available
	netstatCmd := exec.CommandContext(ctx, "netstat", "-tlnp")
	netstatOutput, netstatErr := netstatCmd.Output()
	if netstatErr == nil {
		return s.parseNetstatOutput(string(netstatOutput), port)
	}

	// If both methods fail, check if port is actually in use
	if s.IsPortInUse(port) {
		return -1, "unknown", nil // Port in use but can't identify process
	}

	return -1, "", fmt.Errorf("port %d not in use or process info unavailable", port)
}

// parseNetstatOutput parses netstat output to extract process information for a specific port
func (s *Scanner) parseNetstatOutput(output string, targetPort int) (int, string, error) {
	lines := strings.Split(output, "\n")
	targetPortStr := fmt.Sprintf(":%d ", targetPort)

	for _, line := range lines {
		// Look for lines containing our target port
		if strings.Contains(line, targetPortStr) && strings.Contains(line, "LISTEN") {
			// Parse netstat line format: tcp 0 0 0.0.0.0:3000 0.0.0.0:* LISTEN 12345/node
			fields := strings.Fields(line)
			if len(fields) >= 7 {
				// Last field typically contains PID/process_name
				processInfo := fields[len(fields)-1]
				if strings.Contains(processInfo, "/") {
					parts := strings.Split(processInfo, "/")
					if len(parts) >= 2 {
						if pid, err := strconv.Atoi(parts[0]); err == nil {
							processName := parts[1]
							return pid, processName, nil
						}
					}
				}
			}
		}
	}

	return -1, "", fmt.Errorf("process info not found for port %d", targetPort)
}

// getProcessInfoWindows gets process info on Windows
func (s *Scanner) getProcessInfoWindows(port int) (int, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	// Use netstat to get process information on Windows
	// -a: show all connections, -n: numerical addresses, -o: show process ID
	cmd := exec.CommandContext(ctx, "netstat", "-ano")
	output, err := cmd.Output()
	if err != nil {
		return -1, "", fmt.Errorf("failed to run netstat: %w", err)
	}

	// Parse netstat output for Windows
	return s.parseNetstatOutputWindows(string(output), port)
}

// parseNetstatOutputWindows parses Windows netstat output to extract process information
func (s *Scanner) parseNetstatOutputWindows(output string, targetPort int) (int, string, error) {
	lines := strings.Split(output, "\n")
	targetPortStr := fmt.Sprintf(":%d ", targetPort)

	for _, line := range lines {
		// Windows netstat format: TCP    127.0.0.1:3000    0.0.0.0:0    LISTENING    12345
		if strings.Contains(line, targetPortStr) && strings.Contains(line, "LISTENING") {
			fields := strings.Fields(line)
			if len(fields) >= 5 {
				// Last field contains PID
				if pid, err := strconv.Atoi(fields[len(fields)-1]); err == nil {
					// Try to get process name using tasklist
					ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
					defer cancel()

					tasklistCmd := exec.CommandContext(ctx, "tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH")
					tasklistOutput, tasklistErr := tasklistCmd.Output()
					if tasklistErr == nil {
						// Parse CSV output to get process name
						if processName := s.parseTasklistOutput(string(tasklistOutput)); processName != "" {
							return pid, processName, nil
						}
					}
					// If tasklist fails, return PID without name
					return pid, "unknown", nil
				}
			}
		}
	}

	return -1, "", fmt.Errorf("process info not found for port %d", targetPort)
}

// parseTasklistOutput parses Windows tasklist CSV output to get process name
func (s *Scanner) parseTasklistOutput(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 {
		return ""
	}

	// CSV format: "process_name.exe","12345","Console","1","1,234 K"
	firstLine := strings.Trim(lines[0], `"`)
	if firstLine != "" {
		// Extract just the process name without quotes
		parts := strings.Split(firstLine, `","`)
		if len(parts) > 0 {
			processName := strings.Trim(parts[0], `"`)
			// Remove .exe extension if present
			if strings.HasSuffix(processName, ".exe") {
				processName = strings.TrimSuffix(processName, ".exe")
			}
			return processName
		}
	}

	return ""
}

// GetListeningPorts returns all ports currently being listened on
func (s *Scanner) GetListeningPorts() ([]PortInfo, error) {
	// Initialize result slice (never return nil)
	result := make([]PortInfo, 0)

	// Scan common development ports
	commonPorts := []int{3000, 3001, 3002, 3003, 4000, 4001, 5000, 5001, 8000, 8001, 8080, 8081, 9000, 9001}

	// Check common ports
	for _, port := range commonPorts {
		if s.IsPortInUse(port) {
			if portInfo, err := s.GetPortInfo(port); err == nil {
				result = append(result, *portInfo)
			}
		}
	}

	// Scan ephemeral port range (system-assigned ports) - common range is 49152-65535
	// For efficiency, scan a smaller range where most dynamic ports are assigned
	for port := 60000; port <= 65535; port++ {
		if s.IsPortInUse(port) {
			if portInfo, err := s.GetPortInfo(port); err == nil {
				result = append(result, *portInfo)
			}
		}
	}

	return result, nil
}

// IsPortInRange checks if a port is within a valid range
func (s *Scanner) IsPortInRange(port int) bool {
	return port > 0 && port <= 65535
}

// IsPrivilegedPort checks if a port requires elevated privileges (ports 1-1023)
func (s *Scanner) IsPrivilegedPort(port int) bool {
	return port > 0 && port < 1024
}

// GetRecommendedPort suggests a good port based on the application type
func (s *Scanner) GetRecommendedPort(appType string) int {
	recommendations := map[string]int{
		"web":        3000,
		"api":        3001,
		"websocket":  3002,
		"database":   5432,
		"cache":      6379,
		"monitoring": 9090,
		"metrics":    9091,
	}

	if port, exists := recommendations[strings.ToLower(appType)]; exists {
		// Try to find available port starting from recommendation
		if available, err := s.FindAvailablePort(port); err == nil {
			return available
		}
	}

	// Default to finding available port from 3000
	if available, err := s.FindAvailablePort(3000); err == nil {
		return available
	}

	return 0
}

// ParsePortRange parses a port range string like "3000-3010"
func (s *Scanner) ParsePortRange(rangeStr string) (int, int, error) {
	if !strings.Contains(rangeStr, "-") {
		// Single port
		port, err := strconv.Atoi(rangeStr)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to parse port %q: %w", rangeStr, err)
		}
		if port <= 0 || port > 65535 {
			return 0, 0, errors.New("invalid port range format")
		}
		return port, port, nil
	}

	parts := strings.Split(rangeStr, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("%w: %s", ErrInvalidPortRange, rangeStr)
	}

	start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid start port: %w", err)
	}

	end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid end port: %w", err)
	}

	if start > end {
		return 0, 0, ErrPortRangeOrder
	}

	// Validate port range
	if start <= 0 || end <= 0 || start > 65535 || end > 65535 {
		return 0, 0, errors.New("invalid port range format")
	}

	return start, end, nil
}

// DiscoverDevelopmentServers scans for and identifies development servers
func (s *Scanner) DiscoverDevelopmentServers(startPort, endPort int) ([]PortInfo, error) {
	portsInUse, err := s.ScanRange(startPort, endPort)
	if err != nil {
		return nil, fmt.Errorf("failed to scan port range: %w", err)
	}

	var developmentServers []PortInfo

	// Development server patterns
	devPatterns := []string{
		"node", "npm", "yarn", "pnpm", "webpack", "vite", "next",
		"react-scripts", "vue", "nuxt", "svelte",
		"python", "flask", "django", "fastapi", "uvicorn",
		"go", "air", "gin", "echo", "fiber",
		"ruby", "rails", "sinatra",
		"php", "artisan", "symfony",
		"java", "spring", "tomcat", "jetty",
		"dotnet", "kestrel",
	}

	for _, portInfo := range portsInUse {
		if portInfo.PID > 0 && portInfo.ProcessName != "" && portInfo.ProcessName != "unknown" {
			// Check if process name matches development server patterns
			processNameLower := strings.ToLower(portInfo.ProcessName)
			for _, pattern := range devPatterns {
				if strings.Contains(processNameLower, pattern) {
					developmentServers = append(developmentServers, portInfo)
					break
				}
			}
		}
	}

	return developmentServers, nil
}

// GetProcessInfoByPID retrieves process information by PID
func (s *Scanner) GetProcessInfoByPID(pid int) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	switch runtime.GOOS {
	case "darwin", "linux":
		// Use ps to get process info
		cmd := exec.CommandContext(ctx, "ps", "-p", strconv.Itoa(pid), "-o", "comm=,args=")
		output, err := cmd.Output()
		if err != nil {
			return "", "", fmt.Errorf("failed to get process info for PID %d: %w", pid, err)
		}

		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(lines) > 0 {
			fields := strings.Fields(lines[0])
			if len(fields) >= 1 {
				processName := fields[0]
				command := strings.Join(fields, " ")
				return processName, command, nil
			}
		}

	case "windows":
		// Use tasklist for Windows
		cmd := exec.CommandContext(ctx, "tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH")
		output, err := cmd.Output()
		if err != nil {
			return "", "", fmt.Errorf("failed to get process info for PID %d: %w", pid, err)
		}

		if processName := s.parseTasklistOutput(string(output)); processName != "" {
			return processName, processName, nil
		}
	}

	return "", "", fmt.Errorf("could not retrieve process info for PID %d", pid)
}
