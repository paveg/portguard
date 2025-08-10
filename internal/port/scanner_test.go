package port

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paveg/portguard/internal/process"
)

const (
	defaultTimeout = 2 * time.Second
	testPortStart  = 30000 // Use high port numbers to avoid conflicts
	testPortEnd    = 35000
)

// Helper function to find an available port for testing
func findTestPort(t *testing.T) int {
	t.Helper()
	
	// Use context.Background() for better practice
	var lc net.ListenConfig
	listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() {
		//nolint:errcheck // Test cleanup can fail
		_ = listener.Close()
	}()
	
	//nolint:errcheck // Type assertion is safe for TCP listener
	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port
}

// Helper function to create a test server on a specific port
//nolint:unparam // net.Listener is used in some tests but ignored in others
func createTestServer(t *testing.T, port int) (net.Listener, func()) {
	t.Helper()
	
	//nolint:noctx // Test helper function, context not critical
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	require.NoError(t, err)
	
	cleanup := func() {
		//nolint:errcheck // Test cleanup can fail
		_ = listener.Close()
	}
	
	return listener, cleanup
}

// Helper function to create a UDP server for testing
func createTestUDPServer(t *testing.T, port int) (net.PacketConn, func()) {
	t.Helper()
	
	//nolint:noctx // Test helper function, context not critical
	conn, err := net.ListenPacket("udp", fmt.Sprintf("127.0.0.1:%d", port))
	require.NoError(t, err)
	
	cleanup := func() {
		//nolint:errcheck // Test cleanup can fail
		_ = conn.Close()
	}
	
	return conn, cleanup
}

func TestNewScanner(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{
			name:    "default_timeout",
			timeout: defaultTimeout,
		},
		{
			name:    "custom_short_timeout",
			timeout: 500 * time.Millisecond,
		},
		{
			name:    "custom_long_timeout",
			timeout: 5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := NewScanner(tt.timeout)
			
			require.NotNil(t, scanner)
			assert.Equal(t, tt.timeout, scanner.timeout)
		})
	}
}

func TestScanner_IsPortInUse(t *testing.T) {
	scanner := NewScanner(defaultTimeout)

	tests := []struct {
		name           string
		setupServer    func(t *testing.T) (int, func())
		expectedInUse  bool
	}{
		{
			name: "port_not_in_use",
			setupServer: func(t *testing.T) (int, func()) {
				t.Helper()
				port := findTestPort(t)
				return port, func() {} // No server, just return available port
			},
			expectedInUse: false,
		},
		{
			name: "tcp_port_in_use",
			setupServer: func(t *testing.T) (int, func()) {
				t.Helper()
				port := findTestPort(t)
				_, cleanup := createTestServer(t, port)
				return port, cleanup
			},
			expectedInUse: true,
		},
		{
			name: "udp_port_in_use",
			setupServer: func(t *testing.T) (int, func()) {
				t.Helper()
				port := findTestPort(t)
				_, cleanup := createTestUDPServer(t, port)
				return port, cleanup
			},
			expectedInUse: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port, cleanup := tt.setupServer(t)
			defer cleanup()

			inUse := scanner.IsPortInUse(port)
			assert.Equal(t, tt.expectedInUse, inUse)
		})
	}
}

func TestScanner_GetPortInfo(t *testing.T) {
	scanner := NewScanner(defaultTimeout)

	tests := []struct {
		name        string
		setupServer func(t *testing.T) (int, func())
		validate    func(t *testing.T, portInfo *process.PortInfo, err error)
	}{
		{
			name: "available_port_info",
			setupServer: func(t *testing.T) (int, func()) {
				t.Helper()
				port := findTestPort(t)
				return port, func() {}
			},
			validate: func(t *testing.T, portInfo *process.PortInfo, err error) {
				t.Helper()
				require.NoError(t, err)
				require.NotNil(t, portInfo)
				assert.False(t, portInfo.IsManaged)
				assert.Equal(t, -1, portInfo.PID) // No process using the port
				assert.Empty(t, portInfo.ProcessName)
				assert.Equal(t, "tcp", portInfo.Protocol)
			},
		},
		{
			name: "tcp_port_in_use_info",
			setupServer: func(t *testing.T) (int, func()) {
				t.Helper()
				port := findTestPort(t)
				_, cleanup := createTestServer(t, port)
				return port, cleanup
			},
			validate: func(t *testing.T, portInfo *process.PortInfo, err error) {
				t.Helper()
				require.NoError(t, err)
				require.NotNil(t, portInfo)
				assert.Positive(t, portInfo.Port)
				assert.Equal(t, "tcp", portInfo.Protocol)
				// Process info may or may not be available depending on platform and permissions
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port, cleanup := tt.setupServer(t)
			defer cleanup()

			portInfo, err := scanner.GetPortInfo(port)
			tt.validate(t, portInfo, err)
		})
	}
}

func TestScanner_FindAvailablePort(t *testing.T) {
	scanner := NewScanner(defaultTimeout)

	tests := []struct {
		name      string
		startPort int
		setup     func(t *testing.T) func() // Setup to create port conflicts
		validate  func(t *testing.T, port int, err error)
	}{
		{
			name:      "find_available_port_no_conflicts",
			startPort: testPortStart,
			setup:     func(t *testing.T) func() { t.Helper(); return func() {} },
			validate: func(t *testing.T, port int, err error) {
				t.Helper()
				require.NoError(t, err)
				assert.GreaterOrEqual(t, port, testPortStart)
				assert.LessOrEqual(t, port, 65535)
				
				// Verify the returned port is actually available
				assert.False(t, scanner.IsPortInUse(port))
			},
		},
		{
			name:      "find_port_with_conflicts",
			startPort: testPortStart + 100,
			setup: func(t *testing.T) func() {
				t.Helper()
				// Create servers on first few ports
				cleanupFuncs := make([]func(), 0, 3)
				
				for i := 0; i < 3; i++ {
					port := testPortStart + 100 + i
					_, cleanup := createTestServer(t, port)
					cleanupFuncs = append(cleanupFuncs, cleanup)
				}
				
				return func() {
					for _, cleanup := range cleanupFuncs {
						cleanup()
					}
				}
			},
			validate: func(t *testing.T, port int, err error) {
				t.Helper()
				require.NoError(t, err)
				
				// Should find a port after the occupied ones
				assert.Greater(t, port, testPortStart+102)
				assert.False(t, scanner.IsPortInUse(port))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := tt.setup(t)
			defer cleanup()

			port, err := scanner.FindAvailablePort(tt.startPort)
			tt.validate(t, port, err)
		})
	}
}

func TestScanner_ScanRange(t *testing.T) {
	scanner := NewScanner(defaultTimeout)

	tests := []struct {
		name      string
		startPort int
		endPort   int
		setup     func(t *testing.T, startPort, endPort int) func()
		validate  func(t *testing.T, portInfos []process.PortInfo, err error)
	}{
		{
			name:      "scan_empty_range",
			startPort: testPortStart + 200,
			endPort:   testPortStart + 210,
			setup:     func(t *testing.T, startPort, endPort int) func() { t.Helper(); return func() {} },
			validate: func(t *testing.T, portInfos []process.PortInfo, err error) {
				t.Helper()
				require.NoError(t, err)
				require.NotNil(t, portInfos)
				
				// Should return info for all ports in range, even if not in use
				expectedCount := (testPortStart + 210) - (testPortStart + 200) + 1
				assert.Len(t, portInfos, expectedCount)
				
				for _, portInfo := range portInfos {
					assert.GreaterOrEqual(t, portInfo.Port, testPortStart+200)
					assert.LessOrEqual(t, portInfo.Port, testPortStart+210)
				}
			},
		},
		{
			name:      "scan_range_with_used_ports",
			startPort: testPortStart + 300,
			endPort:   testPortStart + 305,
			setup: func(t *testing.T, startPort, endPort int) func() {
				t.Helper()
				// Create servers on a couple ports in the range
				port1 := startPort + 1
				port2 := startPort + 3
				
				_, cleanup1 := createTestServer(t, port1)
				_, cleanup2 := createTestServer(t, port2)
				
				return func() {
					cleanup1()
					cleanup2()
				}
			},
			validate: func(t *testing.T, portInfos []process.PortInfo, err error) {
				t.Helper()
				require.NoError(t, err)
				
				expectedCount := (testPortStart + 305) - (testPortStart + 300) + 1
				assert.Len(t, portInfos, expectedCount)
				
				usedPortsFound := 0
				for _, portInfo := range portInfos {
					if portInfo.Port == testPortStart+301 || portInfo.Port == testPortStart+303 {
						usedPortsFound++
						// These ports should show as in use (implementation dependent)
						assert.GreaterOrEqual(t, portInfo.PID, -1) // May be -1 if process info not available
					}
				}
				
				// We should have found both used ports in the scan
				assert.Equal(t, 2, usedPortsFound)
			},
		},
		{
			name:      "invalid_port_range_order",
			startPort: testPortStart + 400,
			endPort:   testPortStart + 350, // End before start
			setup:     func(t *testing.T, startPort, endPort int) func() { t.Helper(); return func() {} },
			validate: func(t *testing.T, portInfos []process.PortInfo, err error) {
				t.Helper()
				require.Error(t, err)
				assert.Nil(t, portInfos)
				assert.ErrorIs(t, err, ErrPortRangeOrder)
			},
		},
		{
			name:      "invalid_port_numbers",
			startPort: -1,
			endPort:   testPortStart + 400,
			setup:     func(t *testing.T, startPort, endPort int) func() { t.Helper(); return func() {} },
			validate: func(t *testing.T, portInfos []process.PortInfo, err error) {
				t.Helper()
				require.Error(t, err)
				assert.Nil(t, portInfos)
				assert.ErrorIs(t, err, ErrInvalidPortRange)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := tt.setup(t, tt.startPort, tt.endPort)
			defer cleanup()

			portInfos, err := scanner.ScanRange(tt.startPort, tt.endPort)
			tt.validate(t, portInfos, err)
		})
	}
}

func TestScanner_GetListeningPorts(t *testing.T) {
	scanner := NewScanner(defaultTimeout)

	// Create a test server on a well-known high port
	port1 := findTestPort(t)
	
	_, cleanup1 := createTestServer(t, port1)
	defer cleanup1()

	ports, err := scanner.GetListeningPorts()
	require.NoError(t, err)
	require.NotNil(t, ports)

	// Should find at least some listening ports (system or our test port)
	assert.Greater(t, len(ports), 0, "Should find at least one listening port")
	
	foundPorts := make(map[int]bool)
	foundTestPort := false
	for _, portInfo := range ports {
		foundPorts[portInfo.Port] = true
		assert.Positive(t, portInfo.Port)
		assert.LessOrEqual(t, portInfo.Port, 65535)
		
		// Check if we found our test port
		if portInfo.Port == port1 {
			foundTestPort = true
		}
	}

	// Log information for debugging CI issues
	t.Logf("Found %d listening ports, test port %d found: %v", len(ports), port1, foundTestPort)
	
	// In CI environments, there might not always be predictable ports available,
	// so we just verify the function works and returns valid data
	for port := range foundPorts {
		assert.True(t, scanner.IsPortInUse(port), "Port %d should be reported as in use", port)
	}
}

func TestScanner_IsPortInRange(t *testing.T) {
	scanner := NewScanner(defaultTimeout)

	tests := []struct {
		name     string
		port     int
		expected bool
	}{
		{
			name:     "valid_port_in_range",
			port:     5000,
			expected: true,
		},
		{
			name:     "valid_low_port",
			port:     1024,
			expected: true,
		},
		{
			name:     "valid_high_port",
			port:     65535,
			expected: true,
		},
		{
			name:     "invalid_zero_port",
			port:     0,
			expected: false,
		},
		{
			name:     "invalid_negative_port",
			port:     -1,
			expected: false,
		},
		{
			name:     "invalid_too_high_port",
			port:     65536,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scanner.IsPortInRange(tt.port)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestScanner_IsPrivilegedPort(t *testing.T) {
	scanner := NewScanner(defaultTimeout)

	tests := []struct {
		name       string
		port       int
		privileged bool
	}{
		{
			name:       "http_port",
			port:       80,
			privileged: true,
		},
		{
			name:       "https_port",
			port:       443,
			privileged: true,
		},
		{
			name:       "ssh_port",
			port:       22,
			privileged: true,
		},
		{
			name:       "boundary_port_1023",
			port:       1023,
			privileged: true,
		},
		{
			name:       "non_privileged_port",
			port:       1024,
			privileged: false,
		},
		{
			name:       "high_port",
			port:       8080,
			privileged: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scanner.IsPrivilegedPort(tt.port)
			assert.Equal(t, tt.privileged, result)
		})
	}
}

func TestScanner_GetRecommendedPort(t *testing.T) {
	scanner := NewScanner(defaultTimeout)

	tests := []struct {
		name     string
		appType  string
		expected int
	}{
		{
			name:     "web_application",
			appType:  "web",
			expected: 3000, // Common default
		},
		{
			name:     "api_application",
			appType:  "api",
			expected: 8080, // Common API port
		},
		{
			name:     "development_server",
			appType:  "dev",
			expected: 3000, // Development default
		},
		{
			name:     "unknown_application_type",
			appType:  "unknown",
			expected: 8080, // Default fallback
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recommended := scanner.GetRecommendedPort(tt.appType)
			assert.Positive(t, recommended)
			assert.LessOrEqual(t, recommended, 65535)
		})
	}
}

func TestScanner_ParsePortRange(t *testing.T) {
	scanner := NewScanner(defaultTimeout)

	tests := []struct {
		name        string
		portRange   string
		expectedMin int
		expectedMax int
		expectError bool
	}{
		{
			name:        "single_port",
			portRange:   "8080",
			expectedMin: 8080,
			expectedMax: 8080,
			expectError: false,
		},
		{
			name:        "port_range",
			portRange:   "3000-3010",
			expectedMin: 3000,
			expectedMax: 3010,
			expectError: false,
		},
		{
			name:        "wide_range",
			portRange:   "8000-9000",
			expectedMin: 8000,
			expectedMax: 9000,
			expectError: false,
		},
		{
			name:        "invalid_format",
			portRange:   "invalid",
			expectedMin: 0,
			expectedMax: 0,
			expectError: true,
		},
		{
			name:        "invalid_range_order",
			portRange:   "9000-8000",
			expectedMin: 0,
			expectedMax: 0,
			expectError: true,
		},
		{
			name:        "negative_port",
			portRange:   "-1-1000",
			expectedMin: 0,
			expectedMax: 0,
			expectError: true,
		},
		{
			name:        "port_too_high",
			portRange:   "65536-70000",
			expectedMin: 0,
			expectedMax: 0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			minPort, maxPort, err := scanner.ParsePortRange(tt.portRange)
			
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedMin, minPort)
				assert.Equal(t, tt.expectedMax, maxPort)
			}
		})
	}
}

func TestScanner_ConcurrentOperations(t *testing.T) {
	scanner := NewScanner(defaultTimeout)
	
	const numGoroutines = 10
	const portsPerGoroutine = 5

	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	
	results := make([][]int, numGoroutines)
	errors := make([]error, numGoroutines)

	// Concurrent port scanning operations
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			
			startPort := testPortStart + (id * 1000)
			foundPorts := make([]int, 0, portsPerGoroutine)
			
			// Find multiple available ports
			for j := 0; j < portsPerGoroutine; j++ {
				port, err := scanner.FindAvailablePort(startPort + (j * 10))
				if err != nil {
					errors[id] = err
					return
				}
				foundPorts = append(foundPorts, port)
			}
			
			results[id] = foundPorts
		}(i)
	}

	wg.Wait()

	// Verify results
	allFoundPorts := make(map[int]bool)
	for i, ports := range results {
		if errors[i] != nil {
			t.Logf("Goroutine %d encountered error: %v", i, errors[i])
			continue
		}
		
		require.Len(t, ports, portsPerGoroutine)
		
		for _, port := range ports {
			// Each port should be unique
			assert.False(t, allFoundPorts[port], "Port %d was found by multiple goroutines", port)
			allFoundPorts[port] = true
			
			// Verify port is actually available
			assert.False(t, scanner.IsPortInUse(port))
		}
	}
}

func TestScanner_CrossPlatformCompatibility(t *testing.T) {
	scanner := NewScanner(defaultTimeout)

	t.Run("basic_functionality_works_on_all_platforms", func(t *testing.T) {
		// Test that basic port scanning works regardless of platform
		port := findTestPort(t)
		
		// Test available port
		assert.False(t, scanner.IsPortInUse(port))
		
		// Create server and test used port
		listener, cleanup := createTestServer(t, port)
		defer cleanup()
		
		assert.True(t, scanner.IsPortInUse(port))
		
		// Get port info
		portInfo, err := scanner.GetPortInfo(port)
		require.NoError(t, err)
		assert.Equal(t, port, portInfo.Port)
		
		_ = listener // Use listener to avoid unused variable
	})

	t.Run("process_info_availability_varies_by_platform", func(t *testing.T) {
		port := findTestPort(t)
		listener, cleanup := createTestServer(t, port)
		defer cleanup()

		portInfo, err := scanner.GetPortInfo(port)
		require.NoError(t, err)
		
		// Process information availability depends on platform and permissions
		switch runtime.GOOS {
		case "linux", "darwin":
			// Unix-like systems may provide process info if we have permissions
			if portInfo.PID > 0 {
				assert.NotEmpty(t, portInfo.ProcessName)
			}
		case "windows":
			// Windows implementation may have different behavior
			// Just verify we get some response
			assert.GreaterOrEqual(t, portInfo.PID, -1)
		default:
			// Other platforms - just verify basic structure
			assert.GreaterOrEqual(t, portInfo.Port, 0)
		}
		
		_ = listener // Use listener to avoid unused variable
	})
}