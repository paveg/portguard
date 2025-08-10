package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paveg/portguard/internal/process"
)

// Helper function to create test ManagedProcess
func createTestManagedProcess(id, command string, port int, status process.ProcessStatus) *process.ManagedProcess {
	return &process.ManagedProcess{
		ID:          id,
		Command:     command,
		Args:        []string{"run", "dev"},
		Port:        port,
		PID:         1000 + len(id), // Dynamic PID generation
		Status:      status,
		CreatedAt:   time.Now().Add(-time.Hour),
		UpdatedAt:   time.Now().Add(-time.Minute),
		LastSeen:    time.Now().Add(-time.Second),
		Environment: map[string]string{"NODE_ENV": "test"},
		WorkingDir:  "/tmp/test",
		LogFile:     "/tmp/test.log",
	}
}

// Helper function to setup test JSONStore with temp directory
func setupTestJSONStore(t *testing.T) (*JSONStore, string, func()) {
	t.Helper()

	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test_state.json")

	store, err := NewJSONStore(filePath)
	require.NoError(t, err)

	cleanup := func() {
		// cleanup is handled by t.TempDir()
	}

	return store, filePath, cleanup
}

func TestNewJSONStore(t *testing.T) {
	tests := []struct {
		name        string
		setupPath   func(*testing.T) string
		expectError bool
	}{
		{
			name: "successful_creation_new_file",
			setupPath: func(t *testing.T) string {
				t.Helper()
				return filepath.Join(t.TempDir(), "new_state.json")
			},
			expectError: false,
		},
		{
			name: "successful_creation_with_nested_directories",
			setupPath: func(t *testing.T) string {
				t.Helper()
				return filepath.Join(t.TempDir(), "nested", "deep", "state.json")
			},
			expectError: false,
		},
		{
			name: "load_existing_valid_file",
			setupPath: func(t *testing.T) string {
				t.Helper()
				tempDir := t.TempDir()
				filePath := filepath.Join(tempDir, "existing_state.json")

				// Create a valid existing state file
				validState := StateData{
					Processes: map[string]*process.ManagedProcess{
						"test": createTestManagedProcess("test", "npm start", 3000, process.StatusRunning),
					},
					Metadata: &Metadata{
						Version:   "1.0",
						CreatedAt: time.Now().Add(-time.Hour),
						UpdatedAt: time.Now(),
					},
				}

				data, err := json.MarshalIndent(validState, "", "  ")
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(filePath, data, 0o600))

				return filePath
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := tt.setupPath(t)

			store, err := NewJSONStore(filePath)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, store)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, store)
			assert.Equal(t, filePath, store.GetFilePath())
			assert.NotNil(t, store.GetMetadata())
			assert.Equal(t, "1.0", store.GetMetadata().Version)
		})
	}
}

func TestJSONStore_SaveAndLoad(t *testing.T) {
	store, _, cleanup := setupTestJSONStore(t)
	defer cleanup()

	// Create test processes with different statuses
	testProcesses := map[string]*process.ManagedProcess{
		"proc1": createTestManagedProcess("proc1", "npm run dev", 3000, process.StatusRunning),
		"proc2": createTestManagedProcess("proc2", "go run main.go", 8080, process.StatusStopped),
		"proc3": createTestManagedProcess("proc3", "python app.py", 5000, process.StatusUnhealthy),
	}

	// Test Save
	err := store.Save(testProcesses)
	require.NoError(t, err)

	// Test Load
	loadedProcesses, err := store.Load()
	require.NoError(t, err)
	require.NotNil(t, loadedProcesses)

	// Verify loaded data matches saved data
	assert.Len(t, loadedProcesses, len(testProcesses))

	for id, originalProcess := range testProcesses {
		loadedProcess, exists := loadedProcesses[id]
		require.True(t, exists, "Process %s should exist", id)

		assert.Equal(t, originalProcess.ID, loadedProcess.ID)
		assert.Equal(t, originalProcess.Command, loadedProcess.Command)
		assert.Equal(t, originalProcess.Port, loadedProcess.Port)
		assert.Equal(t, originalProcess.Status, loadedProcess.Status)
		assert.Equal(t, originalProcess.Environment, loadedProcess.Environment)
	}
}

func TestJSONStore_SaveLoadRoundTrip(t *testing.T) {
	store, _, cleanup := setupTestJSONStore(t)
	defer cleanup()

	// Multiple round trips to test data integrity
	for i := 0; i < 3; i++ {
		processes := map[string]*process.ManagedProcess{
			"round_trip": createTestManagedProcess(
				"round_trip",
				"test-command-iteration-"+string(rune('0'+i)),
				3000+i,
				process.StatusRunning,
			),
		}

		// Save
		err := store.Save(processes)
		require.NoError(t, err)

		// Load
		loaded, err := store.Load()
		require.NoError(t, err)
		require.Len(t, loaded, 1)

		// Verify
		proc := loaded["round_trip"]
		assert.Equal(t, processes["round_trip"].Command, proc.Command)
		assert.Equal(t, processes["round_trip"].Port, proc.Port)
	}
}

func TestJSONStore_Delete(t *testing.T) {
	store, _, cleanup := setupTestJSONStore(t)
	defer cleanup()

	// Setup initial processes
	initialProcesses := map[string]*process.ManagedProcess{
		"keep1":   createTestManagedProcess("keep1", "npm start", 3000, process.StatusRunning),
		"delete1": createTestManagedProcess("delete1", "npm build", 3001, process.StatusStopped),
		"keep2":   createTestManagedProcess("keep2", "go run main.go", 8080, process.StatusRunning),
	}

	err := store.Save(initialProcesses)
	require.NoError(t, err)

	// Delete one process
	err = store.Delete("delete1")
	require.NoError(t, err)

	// Verify process was deleted
	loaded, err := store.Load()
	require.NoError(t, err)
	assert.Len(t, loaded, 2)

	_, exists := loaded["delete1"]
	assert.False(t, exists)

	_, exists = loaded["keep1"]
	assert.True(t, exists)

	_, exists = loaded["keep2"]
	assert.True(t, exists)

	// Test deleting non-existent process (should not error)
	err = store.Delete("nonexistent")
	require.NoError(t, err)
}

func TestJSONStore_ValidateState(t *testing.T) {
	tests := []struct {
		name        string
		setupData   func() *StateData
		expectError bool
		errorType   error
	}{
		{
			name: "valid_state",
			setupData: func() *StateData {
				return &StateData{
					Processes: map[string]*process.ManagedProcess{
						"test": createTestManagedProcess("test", "npm start", 3000, process.StatusRunning),
					},
					Metadata: &Metadata{
						Version:   "1.0",
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
				}
			},
			expectError: false,
		},
		{
			name: "missing_version",
			setupData: func() *StateData {
				return &StateData{
					Processes: map[string]*process.ManagedProcess{},
					Metadata: &Metadata{
						Version:   "", // Empty version
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
				}
			},
			expectError: true,
			errorType:   ErrNoVersionInfo,
		},
		{
			name: "process_id_mismatch",
			setupData: func() *StateData {
				proc := createTestManagedProcess("correct_id", "npm start", 3000, process.StatusRunning)
				return &StateData{
					Processes: map[string]*process.ManagedProcess{
						"wrong_key": proc, // Key doesn't match process ID
					},
					Metadata: &Metadata{
						Version:   "1.0",
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
				}
			},
			expectError: true,
			errorType:   ErrProcessIDMismatch,
		},
		{
			name: "empty_command",
			setupData: func() *StateData {
				proc := createTestManagedProcess("test", "", 3000, process.StatusRunning) // Empty command
				return &StateData{
					Processes: map[string]*process.ManagedProcess{
						"test": proc,
					},
					Metadata: &Metadata{
						Version:   "1.0",
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
				}
			},
			expectError: true,
			errorType:   ErrProcessEmptyCmd,
		},
		{
			name: "zero_creation_time",
			setupData: func() *StateData {
				proc := createTestManagedProcess("test", "npm start", 3000, process.StatusRunning)
				proc.CreatedAt = time.Time{} // Zero time
				return &StateData{
					Processes: map[string]*process.ManagedProcess{
						"test": proc,
					},
					Metadata: &Metadata{
						Version:   "1.0",
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
				}
			},
			expectError: true,
			errorType:   ErrProcessZeroTime,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, _, cleanup := setupTestJSONStore(t)
			defer cleanup()

			// Set up test data
			store.data = tt.setupData()

			err := store.ValidateState()

			if tt.expectError {
				require.Error(t, err)
				if tt.errorType != nil {
					require.ErrorIs(t, err, tt.errorType)
				}
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestJSONStore_BackupState(t *testing.T) {
	store, filePath, cleanup := setupTestJSONStore(t)
	defer cleanup()

	// Save some data first
	processes := map[string]*process.ManagedProcess{
		"backup_test": createTestManagedProcess("backup_test", "npm run backup", 3000, process.StatusRunning),
	}
	err := store.Save(processes)
	require.NoError(t, err)

	// Create backup
	err = store.BackupState()
	require.NoError(t, err)

	// Verify backup file was created
	dir := filepath.Dir(filePath)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	backupFound := false
	baseFileName := filepath.Base(filePath)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), baseFileName+".backup.") {
			continue
		}

		backupFound = true

		// Verify backup content is valid JSON
		backupPath := filepath.Join(dir, entry.Name())
		backupData, err := os.ReadFile(backupPath)
		require.NoError(t, err)

		var backupState StateData
		err = json.Unmarshal(backupData, &backupState)
		require.NoError(t, err)
		break
	}
	assert.True(t, backupFound, "Backup file should be created")
}

func TestJSONStore_BackupStateNoFile(t *testing.T) {
	// Create store with non-existent file
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "nonexistent.json")

	store := &JSONStore{
		filePath: filePath,
		data: &StateData{
			Processes: make(map[string]*process.ManagedProcess),
			Metadata: &Metadata{
				Version:   "1.0",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
	}

	// Backup should succeed (no file to backup)
	err := store.BackupState()
	require.NoError(t, err)
}

func TestJSONStore_CleanupOldBackups(t *testing.T) {
	store, filePath, cleanup := setupTestJSONStore(t)
	defer cleanup()

	dir := filepath.Dir(filePath)
	baseFileName := filepath.Base(filePath)

	// Create some fake backup files with different ages
	now := time.Now()
	oldBackupName := baseFileName + ".backup." + now.Add(-2*time.Hour).Format("20060102-150405")
	recentBackupName := baseFileName + ".backup." + now.Add(-30*time.Minute).Format("20060102-150405")

	oldBackupPath := filepath.Join(dir, oldBackupName)
	recentBackupPath := filepath.Join(dir, recentBackupName)

	// Create backup files
	err := os.WriteFile(oldBackupPath, []byte("old backup"), 0o600)
	require.NoError(t, err)
	err = os.WriteFile(recentBackupPath, []byte("recent backup"), 0o600)
	require.NoError(t, err)

	// Set modification times
	oldTime := now.Add(-2 * time.Hour)
	recentTime := now.Add(-30 * time.Minute)

	err = os.Chtimes(oldBackupPath, oldTime, oldTime)
	require.NoError(t, err)
	err = os.Chtimes(recentBackupPath, recentTime, recentTime)
	require.NoError(t, err)

	// Cleanup backups older than 1 hour
	err = store.CleanupOldBackups(time.Hour)
	require.NoError(t, err)

	// Verify old backup was deleted and recent backup remains
	_, err = os.Stat(oldBackupPath)
	assert.True(t, os.IsNotExist(err), "Old backup should be deleted")

	_, err = os.Stat(recentBackupPath)
	assert.NoError(t, err, "Recent backup should remain")
}

func TestJSONStore_CorruptedDataHandling(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "corrupted_state.json")

	// Create corrupted JSON file
	corruptedData := []byte(`{"processes": {"test": {invalid json}`)
	err := os.WriteFile(filePath, corruptedData, 0o600)
	require.NoError(t, err)

	// Creating store should fail due to corrupted data
	_, err = NewJSONStore(filePath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load existing state")
}

func TestJSONStore_AtomicWrite(t *testing.T) {
	store, filePath, cleanup := setupTestJSONStore(t)
	defer cleanup()

	// Verify atomic write by checking temp file handling
	processes := map[string]*process.ManagedProcess{
		"atomic_test": createTestManagedProcess("atomic_test", "npm atomic", 3000, process.StatusRunning),
	}

	err := store.Save(processes)
	require.NoError(t, err)

	// Verify no temp files are left behind
	dir := filepath.Dir(filePath)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	tempFileFound := false
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			tempFileFound = true
			break
		}
	}
	assert.False(t, tempFileFound, "No temporary files should remain after atomic write")

	// Verify final file exists and is readable
	_, err = os.Stat(filePath)
	require.NoError(t, err)

	// Verify data integrity
	loaded, err := store.Load()
	require.NoError(t, err)
	assert.Len(t, loaded, 1)
	assert.Equal(t, "atomic_test", loaded["atomic_test"].ID)
}

func TestJSONStore_GetMetadata(t *testing.T) {
	store, _, cleanup := setupTestJSONStore(t)
	defer cleanup()

	metadata := store.GetMetadata()
	require.NotNil(t, metadata)

	assert.Equal(t, "1.0", metadata.Version)
	assert.False(t, metadata.CreatedAt.IsZero())
	assert.False(t, metadata.UpdatedAt.IsZero())
}

func TestJSONStore_ConcurrentSaveLoad(t *testing.T) {
	tmpDir := t.TempDir()

	const numGoroutines = 3
	const operationsPerGoroutine = 2
	var successCount int32

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Each goroutine uses its own JSONStore instance with separate subdirectory to avoid race conditions
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			// Create separate subdirectory for each goroutine to ensure complete isolation
			goroutineDir := filepath.Join(tmpDir, fmt.Sprintf("goroutine_%d", id))
			err := os.MkdirAll(goroutineDir, 0o755)
			if err != nil {
				t.Errorf("Failed to create directory for goroutine %d: %v", id, err)
				return
			}

			storePath := filepath.Join(goroutineDir, "test_state.json")
			store, err := NewJSONStore(storePath)
			if err != nil {
				t.Errorf("Failed to create store for goroutine %d: %v", id, err)
				return
			}

			for j := 0; j < operationsPerGoroutine; j++ {
				// Create unique process for this goroutine and iteration
				processID := fmt.Sprintf("concurrent_%d_%d", id, j)
				processes := map[string]*process.ManagedProcess{
					processID: createTestManagedProcess(processID, fmt.Sprintf("cmd-%d-%d", id, j), 3000+id*100+j, process.StatusRunning),
				}

				// Save operation
				if err := store.Save(processes); err != nil {
					t.Errorf("Unexpected save error in goroutine %d iteration %d: %v", id, j, err)
					continue
				}

				// Load operation
				if loaded, err := store.Load(); err != nil {
					t.Errorf("Unexpected load error in goroutine %d iteration %d: %v", id, j, err)
				} else if len(loaded) > 0 {
					atomic.AddInt32(&successCount, 1)
				}

				// Small delay to reduce contention
				time.Sleep(5 * time.Millisecond)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Verify that operations succeeded
	assert.Positive(t, successCount, "At least some concurrent operations should succeed")

	// Final verification - create a separate store to ensure functionality
	finalStore, err := NewJSONStore(filepath.Join(tmpDir, "final_test.json"))
	require.NoError(t, err)

	finalProcesses := map[string]*process.ManagedProcess{
		"final_test": createTestManagedProcess("final_test", "final command", 9999, process.StatusRunning),
	}

	err = finalStore.Save(finalProcesses)
	require.NoError(t, err)

	loaded, err := finalStore.Load()
	require.NoError(t, err)
	assert.Len(t, loaded, 1)
	assert.Equal(t, "final_test", loaded["final_test"].ID)
}
