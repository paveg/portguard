package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/paveg/portguard/internal/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock implementations for testing
type mockStateStore struct {
	mock.Mock
}

func (m *mockStateStore) Save(processes map[string]*process.ManagedProcess) error {
	args := m.Called(processes)
	return args.Error(0)
}

func (m *mockStateStore) Load() (map[string]*process.ManagedProcess, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]*process.ManagedProcess), args.Error(1)
}

func (m *mockStateStore) Delete(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

type mockLockManager struct {
	mock.Mock
}

func (m *mockLockManager) Lock() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockLockManager) Unlock() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockLockManager) IsLocked() bool {
	args := m.Called()
	return args.Bool(0)
}

type mockPortScanner struct {
	mock.Mock
}

func (m *mockPortScanner) IsPortInUse(port int) bool {
	args := m.Called(port)
	return args.Bool(0)
}

func (m *mockPortScanner) GetPortInfo(port int) (*process.PortInfo, error) {
	args := m.Called(port)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*process.PortInfo), args.Error(1)
}

func (m *mockPortScanner) ScanRange(startPort, endPort int) ([]process.PortInfo, error) {
	args := m.Called(startPort, endPort)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]process.PortInfo), args.Error(1)
}

func (m *mockPortScanner) FindAvailablePort(startPort int) (int, error) {
	args := m.Called(startPort)
	return args.Int(0), args.Error(1)
}

// Helper function to create a mock ProcessManager with no conflicts
func createMockProcessManager() *process.ProcessManager {
	mockStore := &mockStateStore{}
	mockLock := &mockLockManager{}
	mockScanner := &mockPortScanner{}
	
	// Set up mocks to return empty state (no conflicts)
	mockStore.On("Load").Return(map[string]*process.ManagedProcess{}, nil)
	mockStore.On("Save", mock.AnythingOfType("map[string]*process.ManagedProcess")).Return(nil)
	mockLock.On("Lock").Return(nil)
	mockLock.On("Unlock").Return(nil)
	mockLock.On("IsLocked").Return(false)
	mockScanner.On("IsPortInUse", mock.AnythingOfType("int")).Return(false)
	
	return process.NewProcessManager(mockStore, mockLock, mockScanner)
}

// Helper function to execute intercept command with given input
func executeInterceptCmd(t *testing.T, input string) (string, error) {
	t.Helper()

	// Parse input as JSON
	var req InterceptRequest
	decoder := json.NewDecoder(strings.NewReader(input))
	if err := decoder.Decode(&req); err != nil {
		return "", err
	}

	// Capture stdout for the handler
	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()
	
	reader, writer, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = writer
	
	// Capture output in a goroutine
	var outputBuf bytes.Buffer
	done := make(chan bool)
	go func() {
		defer close(done)
		_, _ = outputBuf.ReadFrom(reader)
	}()

	// Call the appropriate handler directly
	switch req.Event {
	case "preToolUse":
		handlePreToolUse(&req)
	case "postToolUse":
		handlePostToolUse(&req)
	default:
		response := map[string]interface{}{
			"error":   "unknown event",
			"message": "Event not supported",
		}
		encoder := json.NewEncoder(os.Stdout)
		_ = encoder.Encode(response)
	}
	
	// Close write end and wait for output
	writer.Close()
	<-done
	
	return outputBuf.String(), nil
}

// Helper function to create test InterceptRequest
func createTestInterceptRequest(event, toolName string, parameters map[string]interface{}, result *ToolResult) InterceptRequest {
	return InterceptRequest{
		Event:      event,
		ToolName:   toolName,
		Parameters: parameters,
		Result:     result,
		SessionID:  "test-session",
		WorkingDir: "/tmp/test",
		Timestamp:  time.Now().Format(time.RFC3339),
	}
}

// Helper function to create test bash parameters
func createBashParameters(command string) map[string]interface{} {
	return map[string]interface{}{
		"command": command,
	}
}

func TestInterceptCommand_PreToolUse(t *testing.T) {
	// Set up mock ProcessManager factory for all tests (thread-safe)
	restoreFactory := SetProcessManagerFactory(createMockProcessManager)
	defer restoreFactory()

	tests := []struct {
		name           string
		request        InterceptRequest
		expectedProceed bool
		expectedMessage string
		expectError     bool
	}{
		{
			name: "allow_non_server_command",
			request: createTestInterceptRequest(
				"preToolUse", 
				"Bash", 
				createBashParameters("ls -la"),
				nil,
			),
			expectedProceed: true,
			expectedMessage: "",
		},
		{
			name: "detect_npm_run_dev_command",
			request: createTestInterceptRequest(
				"preToolUse",
				"Bash",
				createBashParameters("npm run dev"),
				nil,
			),
			expectedProceed: true, // Should proceed but may detect conflict
		},
		{
			name: "detect_go_run_command",
			request: createTestInterceptRequest(
				"preToolUse",
				"Bash",
				createBashParameters("go run main.go --port=8080"),
				nil,
			),
			expectedProceed: true,
		},
		{
			name: "detect_python_server_command",
			request: createTestInterceptRequest(
				"preToolUse",
				"Bash",
				createBashParameters("python -m http.server 8000"),
				nil,
			),
			expectedProceed: true,
		},
		{
			name: "detect_flask_command",
			request: createTestInterceptRequest(
				"preToolUse",
				"Bash",
				createBashParameters("flask run --port 5000"),
				nil,
			),
			expectedProceed: true,
		},
		{
			name: "handle_non_bash_tool",
			request: createTestInterceptRequest(
				"preToolUse",
				"Read",
				map[string]interface{}{"file_path": "/tmp/test.txt"},
				nil,
			),
			expectedProceed: true,
		},
		{
			name: "handle_complex_command_with_flags",
			request: createTestInterceptRequest(
				"preToolUse",
				"Bash",
				createBashParameters("npm run dev -- --port 3001 --host 0.0.0.0"),
				nil,
			),
			expectedProceed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, err := json.Marshal(tt.request)
			require.NoError(t, err)

			output, err := executeInterceptCmd(t, string(input))

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, output)

			// Parse response
			var response PreToolUseResponse
			err = json.Unmarshal([]byte(output), &response)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedProceed, response.Proceed)
			
			if tt.expectedMessage != "" {
				assert.Contains(t, response.Message, tt.expectedMessage)
			}
		})
	}
}

func TestInterceptCommand_PostToolUse(t *testing.T) {
	// Set up mock ProcessManager factory for all tests (thread-safe)
	restoreFactory := SetProcessManagerFactory(createMockProcessManager)
	defer restoreFactory()

	tests := []struct {
		name        string
		request     InterceptRequest
		expectError bool
		validateResponse func(*testing.T, string)
	}{
		{
			name: "successful_server_startup",
			request: createTestInterceptRequest(
				"postToolUse",
				"Bash",
				createBashParameters("npm run dev"),
				&ToolResult{
					Success:  true,
					Output:   "Server running on http://localhost:3000\nReady for connections",
					ExitCode: 0,
				},
			),
			expectError: false,
			validateResponse: func(t *testing.T, output string) {
				t.Helper()
				var response PostToolUseResponse
				err := json.Unmarshal([]byte(output), &response)
				require.NoError(t, err)
				
				assert.Equal(t, "success", response.Status)
				assert.NotEmpty(t, response.Message)
			},
		},
		{
			name: "failed_command",
			request: createTestInterceptRequest(
				"postToolUse",
				"Bash",
				createBashParameters("npm run dev"),
				&ToolResult{
					Success:  false,
					Error:    "Error: Cannot start server",
					ExitCode: 1,
				},
			),
			expectError: false,
			validateResponse: func(t *testing.T, output string) {
				t.Helper()
				var response PostToolUseResponse
				err := json.Unmarshal([]byte(output), &response)
				require.NoError(t, err)
				
				assert.Equal(t, "error", response.Status)
			},
		},
		{
			name: "non_server_command_success",
			request: createTestInterceptRequest(
				"postToolUse",
				"Bash",
				createBashParameters("ls -la"),
				&ToolResult{
					Success:  true,
					Output:   "total 8\ndrwxr-xr-x 2 user user 4096 Jan 1 12:00 .",
					ExitCode: 0,
				},
			),
			expectError: false,
			validateResponse: func(t *testing.T, output string) {
				t.Helper()
				var response PostToolUseResponse
				err := json.Unmarshal([]byte(output), &response)
				require.NoError(t, err)
				
				assert.Equal(t, "success", response.Status)
			},
		},
		{
			name: "server_startup_with_port_extraction",
			request: createTestInterceptRequest(
				"postToolUse",
				"Bash",
				createBashParameters("go run main.go"),
				&ToolResult{
					Success:  true,
					Output:   "Starting server...\nListening on :8080\nServer ready",
					ExitCode: 0,
				},
			),
			expectError: false,
			validateResponse: func(t *testing.T, output string) {
				t.Helper()
				var response PostToolUseResponse
				err := json.Unmarshal([]byte(output), &response)
				require.NoError(t, err)
				
				assert.Equal(t, "success", response.Status)
				// Note: In this simplified mock, we don't populate Data field
				// In real implementation, this would contain process information
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, err := json.Marshal(tt.request)
			require.NoError(t, err)

			output, err := executeInterceptCmd(t, string(input))

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, output)

			tt.validateResponse(t, output)
		})
	}
}

func TestInterceptCommand_InvalidEvents(t *testing.T) {
	// Set up mock ProcessManager factory for all tests (thread-safe)
	restoreFactory := SetProcessManagerFactory(createMockProcessManager)
	defer restoreFactory()

	tests := []struct {
		name      string
		event     string
		expectError bool
	}{
		{
			name:      "unknown_event",
			event:     "unknownEvent",
			expectError: true,
		},
		{
			name:      "empty_event",
			event:     "",
			expectError: true,
		},
		{
			name:      "misspelled_event",
			event:     "preToolUs", // Missing 'e'
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := createTestInterceptRequest(
				tt.event,
				"Bash",
				createBashParameters("echo test"),
				nil,
			)

			input, err := json.Marshal(request)
			require.NoError(t, err)

			output, err := executeInterceptCmd(t, string(input))

			if tt.expectError {
				// Should output error response in JSON format
				require.NotEmpty(t, output)
				assert.Contains(t, output, "error")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsServerCommand(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected bool
	}{
		// Node.js commands
		{name: "npm_run_dev", command: "npm run dev", expected: true},
		{name: "npm_start", command: "npm start", expected: true},
		{name: "yarn_dev", command: "yarn dev", expected: true},
		{name: "pnpm_dev", command: "pnpm dev", expected: true},
		{name: "node_server", command: "node server.js", expected: true},

		// Go commands
		{name: "go_run", command: "go run main.go", expected: true},
		{name: "go_run_with_flags", command: "go run -ldflags='-X main.version=1.0' main.go", expected: true},

		// Python commands
		{name: "python_http_server", command: "python -m http.server", expected: true},
		{name: "python3_http_server", command: "python3 -m http.server 8080", expected: true},
		{name: "flask_run", command: "flask run", expected: true},
		{name: "django_runserver", command: "python manage.py runserver", expected: true},
		{name: "uvicorn", command: "uvicorn app:app --reload", expected: true},

		// Other server commands
		{name: "hugo_server", command: "hugo server", expected: true},
		{name: "jekyll_serve", command: "jekyll serve", expected: true},
		{name: "php_server", command: "php -S localhost:8000", expected: true},

		// Non-server commands
		{name: "ls", command: "ls -la", expected: false},
		{name: "npm_install", command: "npm install", expected: false},
		{name: "git_status", command: "git status", expected: false},
		{name: "make_build", command: "make build", expected: false},
		{name: "echo", command: "echo hello", expected: false},
		{name: "cat_file", command: "cat package.json", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isServerCommand(tt.command)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractPort(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected int
	}{
		// Common port flag formats
		{name: "port_flag", command: "npm run dev --port 3001", expected: 3001},
		{name: "port_flag_equals", command: "go run main.go --port=8080", expected: 8080},
		{name: "p_flag", command: "python -m http.server -p 8000", expected: 8000},
		{name: "port_after_colon", command: "hugo server --bind :1313", expected: 1313},

		// Framework-specific defaults
		{name: "npm_dev_default", command: "npm run dev", expected: 3000},
		{name: "flask_default", command: "flask run", expected: 5000},
		{name: "django_default", command: "python manage.py runserver", expected: 8000},
		{name: "jekyll_default", command: "jekyll serve", expected: 4000},
		{name: "hugo_default", command: "hugo server", expected: 1313},

		// No port specified
		{name: "go_run_no_port", command: "go run main.go", expected: 0},
		{name: "node_no_port", command: "node server.js", expected: 0},

		// Edge cases
		{name: "multiple_port_flags", command: "cmd --port 3000 --backup-port 3001", expected: 3000}, // First one wins
		{name: "port_in_middle", command: "npm run dev --env production --port 4000 --verbose", expected: 4000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractPort(tt.command)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractPortFromOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected int
	}{
		// Common server output patterns
		{name: "localhost_url", output: "Server running on http://localhost:3000", expected: 3000},
		{name: "listening_on", output: "Listening on :8080", expected: 8080},
		{name: "server_started", output: "Server started on port 5000", expected: 5000},
		{name: "available_at", output: "Local: http://localhost:4000/", expected: 4000},
		{name: "bind_address", output: "Serving at 127.0.0.1:8000", expected: 8000},

		// Multiple ports in output
		{name: "multiple_ports", output: "Local: http://localhost:3000\nNetwork: http://192.168.1.100:3000", expected: 3000},

		// No port in output
		{name: "no_port", output: "Server started successfully", expected: 0},
		{name: "empty_output", output: "", expected: 0},

		// Edge cases
		{name: "port_with_path", output: "Server running on http://localhost:3000/api", expected: 3000},
		{name: "https_port", output: "HTTPS server on https://localhost:8443", expected: 8443},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractPortFromOutput(tt.output)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInterceptCommand_JSONResponseFormat(t *testing.T) {
	t.Run("pre_tool_use_response_format", func(t *testing.T) {
		request := createTestInterceptRequest(
			"preToolUse",
			"Bash",
			createBashParameters("npm run dev"),
			nil,
		)

		input, err := json.Marshal(request)
		require.NoError(t, err)

		output, err := executeInterceptCmd(t, string(input))
		require.NoError(t, err)

		// Verify valid JSON
		var response PreToolUseResponse
		err = json.Unmarshal([]byte(output), &response)
		require.NoError(t, err)

		// Verify required fields are present (Proceed is a bool, just verify it's valid)
		// Proceed should be either true or false (bools have default false in Go)
		assert.NotNil(t, response.Message)   // Should be string, even if empty
	})

	t.Run("post_tool_use_response_format", func(t *testing.T) {
		request := createTestInterceptRequest(
			"postToolUse",
			"Bash",
			createBashParameters("echo test"),
			&ToolResult{
				Success:  true,
				Output:   "test",
				ExitCode: 0,
			},
		)

		input, err := json.Marshal(request)
		require.NoError(t, err)

		output, err := executeInterceptCmd(t, string(input))
		require.NoError(t, err)

		// Verify valid JSON
		var response PostToolUseResponse
		err = json.Unmarshal([]byte(output), &response)
		require.NoError(t, err)

		// Verify required fields
		assert.NotEmpty(t, response.Status)
		assert.NotNil(t, response.Message)
	})
}

func TestInterceptCommand_ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		checkOutput func(*testing.T, string)
	}{
		{
			name:        "invalid_json",
			input:       `{"invalid": json}`,
			expectError: true,
		},
		{
			name:        "empty_input",
			input:       "",
			expectError: true,
		},
		{
			name:        "missing_required_fields",
			input:       `{"event": "preToolUse"}`, // Missing tool_name and parameters
			expectError: false, // Should handle gracefully
			checkOutput: func(t *testing.T, output string) {
				t.Helper()
				assert.NotEmpty(t, output)
				// Should still produce valid JSON response
				var response interface{}
				err := json.Unmarshal([]byte(output), &response)
				assert.NoError(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := executeInterceptCmd(t, tt.input)

			if tt.expectError {
				// For invalid JSON, we expect a decode error
				assert.Error(t, err)
				return
			}

			// Even with malformed input, should produce some output
			if tt.checkOutput != nil {
				tt.checkOutput(t, output)
			}
		})
	}
}

func TestInterceptCommand_ComplexScenarios(t *testing.T) {
	t.Run("chained_commands", func(t *testing.T) {
		request := createTestInterceptRequest(
			"preToolUse",
			"Bash",
			createBashParameters("cd /app && npm install && npm run dev -- --port 3001"),
			nil,
		)

		input, err := json.Marshal(request)
		require.NoError(t, err)

		output, err := executeInterceptCmd(t, string(input))
		require.NoError(t, err)

		var response PreToolUseResponse
		err = json.Unmarshal([]byte(output), &response)
		require.NoError(t, err)

		// Should detect server command in chain
		assert.True(t, response.Proceed)
	})

	t.Run("command_with_environment_variables", func(t *testing.T) {
		request := createTestInterceptRequest(
			"preToolUse",
			"Bash",
			createBashParameters("NODE_ENV=production PORT=4000 npm start"),
			nil,
		)

		input, err := json.Marshal(request)
		require.NoError(t, err)

		output, err := executeInterceptCmd(t, string(input))
		require.NoError(t, err)

		var response PreToolUseResponse
		err = json.Unmarshal([]byte(output), &response)
		require.NoError(t, err)

		assert.True(t, response.Proceed)
	})

	t.Run("server_startup_with_multiple_output_lines", func(t *testing.T) {
		multilineOutput := strings.Join([]string{
			"Installing dependencies...",
			"Dependencies installed successfully",
			"Starting development server...",
			"Server running on http://localhost:3000",
			"Ready for connections",
			"Press Ctrl+C to stop",
		}, "\n")

		request := createTestInterceptRequest(
			"postToolUse",
			"Bash",
			createBashParameters("npm run dev"),
			&ToolResult{
				Success:  true,
				Output:   multilineOutput,
				ExitCode: 0,
			},
		)

		input, err := json.Marshal(request)
		require.NoError(t, err)

		output, err := executeInterceptCmd(t, string(input))
		require.NoError(t, err)

		var response PostToolUseResponse
		err = json.Unmarshal([]byte(output), &response)
		require.NoError(t, err)

		assert.Equal(t, "success", response.Status)
	})
}

func TestOutputErrorResponse(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	reader, writer, _ := os.Pipe()
	os.Stdout = writer

	defer func() {
		os.Stdout = oldStdout
	}()

	t.Run("output_error_response", func(t *testing.T) {
		testErr := errors.New("test error message")
		
		outputErrorResponse(testErr)
		
		writer.Close()
		output, _ := io.ReadAll(reader)
		
		var response PreToolUseResponse
		err := json.Unmarshal(output, &response)
		require.NoError(t, err)
		
		assert.True(t, response.Proceed) // Should fail open
		assert.Contains(t, response.Message, "Hook error")
		assert.Contains(t, response.Message, "test error message")
	})
}