// Package port provides port availability scanning functionality for Portguard.
// It implements cross-platform port detection and health checking for both TCP and UDP protocols.
package port

import (
	"errors"
	"fmt"
	"net"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/paveg/portguard/internal/process"
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
func (s *Scanner) GetPortInfo(port int) (*process.PortInfo, error) {
	portInfo := &process.PortInfo{
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
func (s *Scanner) ScanRange(startPort, endPort int) ([]process.PortInfo, error) {
	// Validate port range
	if startPort > endPort {
		return nil, fmt.Errorf("%w: start port must be less than end port", ErrPortRangeOrder)
	}
	if startPort <= 0 || endPort <= 0 || startPort > 65535 || endPort > 65535 {
		return nil, fmt.Errorf("%w: invalid port range format", ErrInvalidPortRange)
	}

	var result []process.PortInfo

	for port := startPort; port <= endPort; port++ {
		if s.IsPortInUse(port) {
			if portInfo, err := s.GetPortInfo(port); err == nil {
				result = append(result, *portInfo)
			}
		} else {
			// Add info for unused ports as well
			result = append(result, process.PortInfo{
				Port:        port,
				PID:         -1,
				ProcessName: "",
				IsManaged:   false,
				Protocol:    "tcp", // Default to tcp
			})
		}
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
	// This is a simplified implementation
	// In production, you might want to use system calls or parse /proc/net/tcp

	// Try to connect to get basic info
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), s.timeout) //nolint:noctx // TODO: Migrate to (*net.Dialer).DialContext
	if err != nil {
		return -1, "", fmt.Errorf("failed to dial port %d: %w", port, err)
	}
	defer func() { _ = conn.Close() }() //nolint:errcheck // Defer close always completes

	// For now, return placeholder values
	// Real implementation would parse netstat or /proc/net/tcp
	return -1, "unknown", fmt.Errorf("%w for Unix", ErrProcessInfoNotImpl)
}

// getProcessInfoWindows gets process info on Windows
func (s *Scanner) getProcessInfoWindows(_ int) (int, string, error) {
	// Windows-specific implementation would use netstat or WinAPI
	return -1, "unknown", fmt.Errorf("%w for Windows", ErrProcessInfoNotImpl)
}

// GetListeningPorts returns all ports currently being listened on
func (s *Scanner) GetListeningPorts() ([]process.PortInfo, error) {
	// Initialize result slice (never return nil)
	result := make([]process.PortInfo, 0)

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
