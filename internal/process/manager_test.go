package process

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock implementations for testing
type mockStateStore struct {
	mock.Mock
}

func (m *mockStateStore) Save(processes map[string]*ManagedProcess) error {
	args := m.Called(processes)
	return args.Error(0)
}

func (m *mockStateStore) Load() (map[string]*ManagedProcess, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	//nolint:errcheck // Mock args.Get is safe in testify
	return args.Get(0).(map[string]*ManagedProcess), args.Error(1)
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

func (m *mockPortScanner) GetPortInfo(port int) (*PortInfo, error) {
	args := m.Called(port)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	//nolint:errcheck // Mock args.Get is safe in testify
	return args.Get(0).(*PortInfo), args.Error(1)
}

func (m *mockPortScanner) ScanRange(startPort, endPort int) ([]PortInfo, error) {
	args := m.Called(startPort, endPort)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	//nolint:errcheck // Mock args.Get is safe in testify
	return args.Get(0).([]PortInfo), args.Error(1)
}

func (m *mockPortScanner) FindAvailablePort(startPort int) (int, error) {
	args := m.Called(startPort)
	return args.Int(0), args.Error(1)
}

// Helper function to create a test ProcessManager with mocks
func setupTestProcessManager(t *testing.T) (*ProcessManager, *mockStateStore, *mockLockManager, *mockPortScanner) {
	t.Helper()
	
	stateStore := &mockStateStore{}
	lockManager := &mockLockManager{}
	portScanner := &mockPortScanner{}
	
	pm := &ProcessManager{
		processes:   make(map[string]*ManagedProcess),
		mutex:       sync.RWMutex{},
		stateStore:  stateStore,
		lockManager: lockManager,
		portScanner: portScanner,
	}
	
	return pm, stateStore, lockManager, portScanner
}

// Helper function to create test ManagedProcess
func createTestProcess(id, command string, port int, status ProcessStatus) *ManagedProcess {
	return &ManagedProcess{
		ID:        id,
		Command:   command,
		Port:      port,
		PID:       1000 + len(id), // Dynamic PID based on ID length
		Status:    status,
		CreatedAt: time.Now().Add(-time.Hour),
		UpdatedAt: time.Now().Add(-time.Minute),
		LastSeen:  time.Now().Add(-time.Second),
	}
}

func TestProcessManager_ShouldStartNew(t *testing.T) {
	tests := []struct {
		name            string
		command         string
		port            int
		existingProcess *ManagedProcess
		portInUse       bool
		expectStart     bool
		expectProcess   bool
	}{
		{
			name:        "should_start_new_process_no_conflicts",
			command:     "npm run dev",
			port:        3000,
			portInUse:   false,
			expectStart: true,
		},
		{
			name:            "should_reuse_healthy_existing_process",
			command:         "npm run dev",
			port:            3000,
			existingProcess: createTestProcess("test1", "npm run dev", 3000, StatusRunning),
			expectStart:     false,
			expectProcess:   true,
			// Note: IsPortInUse not called because command match is found first
		},
		{
			name:            "should_start_new_when_existing_unhealthy",
			command:         "npm run dev",
			port:            3000,
			existingProcess: createTestProcess("test1", "npm run dev", 3000, StatusStopped),
			portInUse:       false,
			expectStart:     true,
		},
		{
			name:        "should_not_start_when_port_used_externally",
			command:     "npm run dev",
			port:        3000,
			portInUse:   true,
			expectStart: false,
		},
		{
			name:            "should_reuse_process_on_same_port",
			command:         "flask run",
			port:            8080,
			existingProcess: createTestProcess("test2", "flask run", 8080, StatusRunning),
			expectStart:     false,
			expectProcess:   true,
			// Note: IsPortInUse not called because command match is found first
		},
		{
			name:        "should_start_new_process_no_port_specified",
			command:     "python app.py",
			port:        0, // No port specified
			expectStart: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm, _, _, mockPortScanner := setupTestProcessManager(t)

			// Setup existing process if provided
			if tt.existingProcess != nil {
				pm.processes[tt.existingProcess.ID] = tt.existingProcess
			}

			// Setup port scanner mock only if IsPortInUse will be called
			// This happens when port > 0 AND there's no existing healthy process with same command
			needsPortCheck := tt.port > 0
			if tt.existingProcess != nil && tt.existingProcess.Command == tt.command && tt.existingProcess.IsHealthy() {
				needsPortCheck = false // Command match found first, port check skipped
			}
			
			if needsPortCheck {
				mockPortScanner.On("IsPortInUse", tt.port).Return(tt.portInUse)
			}

			shouldStart, returnedProcess := pm.ShouldStartNew(tt.command, tt.port)

			assert.Equal(t, tt.expectStart, shouldStart)
			if tt.expectProcess {
				assert.NotNil(t, returnedProcess)
			} else if !tt.expectStart {
				// If we shouldn't start and don't expect a process, it should be nil
				if tt.existingProcess == nil {
					assert.Nil(t, returnedProcess)
				}
			}

			mockPortScanner.AssertExpectations(t)
		})
	}
}

func TestProcessManager_StartProcess(t *testing.T) {
	tests := []struct {
		name          string
		command       string
		args          []string
		options       StartOptions
		mockSetup     func(*mockStateStore, *mockLockManager, *mockPortScanner)
		expectError   bool
		validateResult func(*testing.T, *ManagedProcess)
	}{
		{
			name:    "successful_process_start",
			command: "npm run dev",
			args:    []string{"run", "dev"},
			options: StartOptions{
				Port:        3000,
				Environment: map[string]string{"NODE_ENV": "development"},
				WorkingDir:  "/tmp/test",
			},
			mockSetup: func(stateStore *mockStateStore, lockManager *mockLockManager, portScanner *mockPortScanner) {
				portScanner.On("IsPortInUse", 3000).Return(false)
				lockManager.On("Lock").Return(nil)
				lockManager.On("Unlock").Return(nil)
				stateStore.On("Save", mock.AnythingOfType("map[string]*process.ManagedProcess")).Return(nil)
			},
			expectError: false,
			validateResult: func(t *testing.T, proc *ManagedProcess) {
				t.Helper()
				assert.Equal(t, "npm run dev", proc.Command)
				assert.Equal(t, 3000, proc.Port)
				assert.Equal(t, StatusRunning, proc.Status)
				assert.NotEmpty(t, proc.ID)
				assert.False(t, proc.CreatedAt.IsZero())
			},
		},
		{
			name:    "lock_acquisition_failure",
			command: "go run main.go",
			args:    []string{"run", "main.go"},
			options: StartOptions{Port: 8080},
			mockSetup: func(stateStore *mockStateStore, lockManager *mockLockManager, portScanner *mockPortScanner) {
				// Note: IsPortInUse not needed because Lock fails before ShouldStartNew is called
				lockManager.On("Lock").Return(assert.AnError)
			},
			expectError: true,
		},
		{
			name:    "state_save_failure",
			command: "python app.py",
			args:    []string{"app.py"},
			options: StartOptions{Port: 5000},
			mockSetup: func(stateStore *mockStateStore, lockManager *mockLockManager, portScanner *mockPortScanner) {
				portScanner.On("IsPortInUse", 5000).Return(false)
				lockManager.On("Lock").Return(nil)
				lockManager.On("Unlock").Return(nil)
				stateStore.On("Save", mock.AnythingOfType("map[string]*process.ManagedProcess")).Return(assert.AnError)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm, mockStateStore, mockLockManager, mockPortScanner := setupTestProcessManager(t)

			tt.mockSetup(mockStateStore, mockLockManager, mockPortScanner)

			process, err := pm.StartProcess(tt.command, tt.args, tt.options)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, process)
			} else {
				require.NoError(t, err)
				require.NotNil(t, process)
				if tt.validateResult != nil {
					tt.validateResult(t, process)
				}
			}

			mockStateStore.AssertExpectations(t)
			mockLockManager.AssertExpectations(t)
			mockPortScanner.AssertExpectations(t)
		})
	}
}

func TestProcessManager_StopProcess(t *testing.T) {
	tests := []struct {
		name            string
		processID       string
		existingProcess *ManagedProcess
		mockSetup       func(*mockStateStore, *mockLockManager, *mockPortScanner)
		expectError     bool
	}{
		{
			name:            "successful_process_stop",
			processID:       "test-process-1",
			existingProcess: createTestProcess("test-process-1", "npm run dev", 3000, StatusRunning),
			mockSetup: func(stateStore *mockStateStore, lockManager *mockLockManager, portScanner *mockPortScanner) {
				lockManager.On("Lock").Return(nil)
				lockManager.On("Unlock").Return(nil)
				stateStore.On("Save", mock.AnythingOfType("map[string]*process.ManagedProcess")).Return(nil)
			},
			expectError: false,
		},
		{
			name:      "process_not_found",
			processID: "nonexistent-process",
			mockSetup: func(stateStore *mockStateStore, lockManager *mockLockManager, portScanner *mockPortScanner) {
				lockManager.On("Lock").Return(nil)
				lockManager.On("Unlock").Return(nil)
			},
			expectError: true,
		},
		{
			name:            "lock_failure",
			processID:       "test-process-2",
			existingProcess: createTestProcess("test-process-2", "go run main.go", 8080, StatusRunning),
			mockSetup: func(stateStore *mockStateStore, lockManager *mockLockManager, portScanner *mockPortScanner) {
				lockManager.On("Lock").Return(assert.AnError)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm, mockStateStore, mockLockManager, mockPortScanner := setupTestProcessManager(t)

			// Setup existing process if provided
			if tt.existingProcess != nil {
				pm.processes[tt.existingProcess.ID] = tt.existingProcess
			}

			tt.mockSetup(mockStateStore, mockLockManager, mockPortScanner)

			err := pm.StopProcess(tt.processID, false)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				// Verify process was marked as stopped
				if tt.existingProcess != nil {
					assert.Equal(t, StatusStopped, pm.processes[tt.processID].Status)
				}
			}

			mockStateStore.AssertExpectations(t)
			mockLockManager.AssertExpectations(t)
			mockPortScanner.AssertExpectations(t)
		})
	}
}

func TestProcessManager_GetProcess(t *testing.T) {
	pm, _, _, _ := setupTestProcessManager(t)
	
	testProcess := createTestProcess("test-get", "test command", 9000, StatusRunning)
	pm.processes[testProcess.ID] = testProcess

	// Test getting existing process
	process, exists := pm.GetProcess("test-get")
	assert.True(t, exists)
	assert.Equal(t, testProcess, process)

	// Test getting non-existent process
	process, exists = pm.GetProcess("nonexistent")
	assert.False(t, exists)
	assert.Nil(t, process)
}

func TestProcessManager_ListProcesses(t *testing.T) {
	pm, _, _, _ := setupTestProcessManager(t)

	// Add test processes
	runningProcess := createTestProcess("running", "npm start", 3000, StatusRunning)
	stoppedProcess := createTestProcess("stopped", "npm build", 3001, StatusStopped)
	unhealthyProcess := createTestProcess("unhealthy", "go run main.go", 8080, StatusUnhealthy)

	pm.processes["running"] = runningProcess
	pm.processes["stopped"] = stoppedProcess
	pm.processes["unhealthy"] = unhealthyProcess

	tests := []struct {
		name           string
		options        ProcessListOptions
		expectedCount  int
		expectedIDs    []string
	}{
		{
			name:          "list_all_processes",
			options:       ProcessListOptions{IncludeStopped: true},
			expectedCount: 3,
			expectedIDs:   []string{"running", "stopped", "unhealthy"},
		},
		{
			name:          "list_only_running",
			options:       ProcessListOptions{IncludeStopped: false},
			expectedCount: 2,
			expectedIDs:   []string{"running", "unhealthy"},
		},
		{
			name:          "filter_by_port",
			options:       ProcessListOptions{IncludeStopped: true, FilterByPort: 3000},
			expectedCount: 1,
			expectedIDs:   []string{"running"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processes := pm.ListProcesses(tt.options)
			assert.Len(t, processes, tt.expectedCount)

			actualIDs := make([]string, 0, len(processes))
			for _, p := range processes {
				actualIDs = append(actualIDs, p.ID)
			}

			for _, expectedID := range tt.expectedIDs {
				assert.Contains(t, actualIDs, expectedID)
			}
		})
	}
}

func TestProcessManager_CleanupProcesses(t *testing.T) {
	tests := []struct {
		name            string
		force           bool
		processes       map[string]*ManagedProcess
		mockSetup       func(*mockStateStore, *mockLockManager, *mockPortScanner)
		expectError     bool
		expectedCleanup int
	}{
		{
			name:  "cleanup_stopped_processes",
			force: false,
			processes: map[string]*ManagedProcess{
				"running": createTestProcess("running", "npm start", 3000, StatusRunning),
				"stopped": createTestProcess("stopped", "npm build", 3001, StatusStopped),
				"failed":  createTestProcess("failed", "broken command", 3002, StatusFailed),
			},
			mockSetup: func(stateStore *mockStateStore, lockManager *mockLockManager, portScanner *mockPortScanner) {
				lockManager.On("Lock").Return(nil)
				lockManager.On("Unlock").Return(nil)
				stateStore.On("Save", mock.AnythingOfType("map[string]*process.ManagedProcess")).Return(nil)
			},
			expectError:     false,
			expectedCleanup: 2, // stopped and failed
		},
		{
			name:  "force_cleanup_all",
			force: true,
			processes: map[string]*ManagedProcess{
				"running": createTestProcess("running", "npm start", 3000, StatusRunning),
				"stopped": createTestProcess("stopped", "npm build", 3001, StatusStopped),
			},
			mockSetup: func(stateStore *mockStateStore, lockManager *mockLockManager, portScanner *mockPortScanner) {
				lockManager.On("Lock").Return(nil)
				lockManager.On("Unlock").Return(nil)
				stateStore.On("Save", mock.AnythingOfType("map[string]*process.ManagedProcess")).Return(nil)
			},
			expectError:     false,
			expectedCleanup: 2, // all processes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm, mockStateStore, mockLockManager, mockPortScanner := setupTestProcessManager(t)

			// Set up processes
			for id, process := range tt.processes {
				pm.processes[id] = process
			}
			initialCount := len(pm.processes)

			tt.mockSetup(mockStateStore, mockLockManager, mockPortScanner)

			err := pm.CleanupProcesses(tt.force)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				expectedRemaining := initialCount - tt.expectedCleanup
				assert.Len(t, pm.processes, expectedRemaining)
			}

			mockStateStore.AssertExpectations(t)
			mockLockManager.AssertExpectations(t)
			mockPortScanner.AssertExpectations(t)
		})
	}
}

func TestProcessManager_ConcurrentOperations(t *testing.T) {
	pm, mockStateStore, mockLockManager, mockPortScanner := setupTestProcessManager(t)

	// Setup mocks for concurrent operations
	mockLockManager.On("Lock").Return(nil).Maybe()
	mockLockManager.On("Unlock").Return(nil).Maybe()
	mockStateStore.On("Save", mock.AnythingOfType("map[string]*process.ManagedProcess")).Return(nil).Maybe()
	mockPortScanner.On("IsPortInUse", mock.AnythingOfType("int")).Return(false).Maybe()

	const numGoroutines = 10
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Concurrent operations
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			// Start a process
			command := fmt.Sprintf("test-command-%d", id)
			port := 3000 + id
			options := StartOptions{Port: port}

			process, err := pm.StartProcess(command, []string{}, options)
			if err == nil {
				// Try to get the process
				retrieved, exists := pm.GetProcess(process.ID)
				assert.True(t, exists)
				assert.NotNil(t, retrieved)

				// Try to stop the process
				//nolint:errcheck // Test cleanup can fail
				_ = pm.StopProcess(process.ID, false)
			}
		}(i)
	}

	wg.Wait()

	// Verify no race conditions occurred and state is consistent
	processes := pm.ListProcesses(ProcessListOptions{IncludeStopped: true})
	assert.LessOrEqual(t, len(processes), numGoroutines)

	// All processes should have valid IDs and timestamps
	for _, p := range processes {
		assert.NotEmpty(t, p.ID)
		assert.False(t, p.CreatedAt.IsZero())
	}
}

func TestProcessManager_generateID(t *testing.T) {
	pm, _, _, _ := setupTestProcessManager(t)

	tests := []struct {
		name    string
		command string
	}{
		{
			name:    "simple_command",
			command: "npm run dev",
		},
		{
			name:    "complex_command_with_args",
			command: "go run -ldflags='-X main.version=1.0' main.go --port=8080",
		},
		{
			name:    "command_with_special_chars",
			command: "python3 -m http.server --bind 127.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id1 := pm.generateID(tt.command)
			
			// Add small delay to ensure different timestamp
			time.Sleep(time.Microsecond)
			id2 := pm.generateID(tt.command)

			// IDs should be non-empty and have expected length (8 hex chars)
			assert.NotEmpty(t, id1)
			assert.NotEmpty(t, id2)
			assert.Len(t, id1, 8)
			assert.Len(t, id2, 8)

			// IDs should be unique due to timestamp difference
			assert.NotEqual(t, id1, id2, "Each generateID call should create unique ID due to timestamp")

			// ID should be different for different commands
			if tt.command != "npm run dev" {
				differentID := pm.generateID("npm run dev")
				assert.NotEqual(t, id1, differentID, "Different commands should generate different IDs")
			}
		})
	}
}