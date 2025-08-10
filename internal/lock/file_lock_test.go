package lock

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testLockTimeout = 2 * time.Second
	shortTimeout    = 100 * time.Millisecond
)

// Helper function to create a test FileLock with temp file
func setupTestFileLock(t *testing.T, timeout time.Duration) (*FileLock, string, func()) {
	t.Helper()
	
	tempDir := t.TempDir()
	lockFile := filepath.Join(tempDir, "test.lock")
	
	fileLock := NewFileLock(lockFile, timeout)
	
	cleanup := func() {
		// cleanup is handled by t.TempDir()
		// but we should try to unlock if still locked
		if fileLock.IsLocked() {
			//nolint:errcheck // Cleanup can fail, but test should not fail
			_ = fileLock.Unlock()
		}
	}
	
	return fileLock, lockFile, cleanup
}

// Helper function to create lock info for testing
func createTestLockInfo(pid int) Info {
	return Info{
		PID:       pid,
		Timestamp: time.Now(),
		IsStale:   false,
	}
}

func TestNewFileLock(t *testing.T) {
	tests := []struct {
		name        string
		setupPath   func(*testing.T) string
		timeout     time.Duration
		validateFunc func(*testing.T, *FileLock)
	}{
		{
			name: "valid_lock_creation",
			setupPath: func(t *testing.T) string {
				t.Helper()
				return filepath.Join(t.TempDir(), "valid.lock")
			},
			timeout: testLockTimeout,
			validateFunc: func(t *testing.T, fl *FileLock) {
				t.Helper()
				assert.Equal(t, testLockTimeout, fl.lockTimeout)
				assert.False(t, fl.locked)
				assert.NotEmpty(t, fl.lockFile)
			},
		},
		{
			name: "nested_directory_creation",
			setupPath: func(t *testing.T) string {
				t.Helper()
				return filepath.Join(t.TempDir(), "nested", "deep", "path", "test.lock")
			},
			timeout: testLockTimeout,
			validateFunc: func(t *testing.T, fl *FileLock) {
				t.Helper()
				assert.Equal(t, testLockTimeout, fl.lockTimeout)
				assert.False(t, fl.locked)
			},
		},
		{
			name: "short_timeout",
			setupPath: func(t *testing.T) string {
				t.Helper()
				return filepath.Join(t.TempDir(), "short.lock")
			},
			timeout: shortTimeout,
			validateFunc: func(t *testing.T, fl *FileLock) {
				t.Helper()
				assert.Equal(t, shortTimeout, fl.lockTimeout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lockFile := tt.setupPath(t)
			
			fileLock := NewFileLock(lockFile, tt.timeout)
			
			require.NotNil(t, fileLock)
			tt.validateFunc(t, fileLock)
		})
	}
}

func TestFileLock_LockUnlock(t *testing.T) {
	tests := []struct {
		name     string
		timeout  time.Duration
		testFunc func(*testing.T, *FileLock)
	}{
		{
			name:    "successful_lock_unlock",
			timeout: testLockTimeout,
			testFunc: func(t *testing.T, fl *FileLock) {
				t.Helper()
				// Lock should succeed
				err := fl.Lock()
				require.NoError(t, err)
				assert.True(t, fl.IsLocked())

				// Unlock should succeed
				err = fl.Unlock()
				require.NoError(t, err)
				assert.False(t, fl.IsLocked())
			},
		},
		{
			name:    "multiple_locks_same_instance",
			timeout: testLockTimeout,
			testFunc: func(t *testing.T, fl *FileLock) {
				t.Helper()
				// First lock should succeed
				err := fl.Lock()
				require.NoError(t, err)
				
				// Second lock should succeed (same instance)
				err = fl.Lock()
				require.NoError(t, err)
				assert.True(t, fl.IsLocked())

				// Unlock should work
				err = fl.Unlock()
				require.NoError(t, err)
				assert.False(t, fl.IsLocked())
			},
		},
		{
			name:    "unlock_without_lock",
			timeout: testLockTimeout,
			testFunc: func(t *testing.T, fl *FileLock) {
				t.Helper()
				// Unlock without lock should return error
				err := fl.Unlock()
				require.Error(t, err)
				assert.False(t, fl.IsLocked())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileLock, _, cleanup := setupTestFileLock(t, tt.timeout)
			defer cleanup()

			tt.testFunc(t, fileLock)
		})
	}
}

func TestFileLock_ConcurrentLocking(t *testing.T) {
	fileLock1, lockFile, cleanup := setupTestFileLock(t, testLockTimeout)
	defer cleanup()

	// Create second FileLock instance for same file
	fileLock2 := NewFileLock(lockFile, shortTimeout) // Shorter timeout for faster test

	tests := []struct {
		name     string
		testFunc func(*testing.T)
	}{
		{
			name: "second_lock_fails_when_first_is_locked",
			testFunc: func(t *testing.T) {
				t.Helper()
				// First lock should succeed
				err := fileLock1.Lock()
				require.NoError(t, err)
				defer func() {
					//nolint:errcheck // Test cleanup can fail
					//nolint:errcheck // Test cleanup can fail
					_ = fileLock1.Unlock()
				}()

				// Second lock should fail due to timeout
				err = fileLock2.Lock()
				require.Error(t, err)
				require.ErrorIs(t, err, ErrLockTimeout)
				assert.False(t, fileLock2.IsLocked())
			},
		},
		{
			name: "second_lock_succeeds_after_first_unlocks",
			testFunc: func(t *testing.T) {
				t.Helper()
				// First lock
				err := fileLock1.Lock()
				require.NoError(t, err)

				// Create second lock instance with longer timeout to account for timing
				fileLock2Extended := NewFileLock(lockFile, 250*time.Millisecond)

				// Start goroutine to unlock after delay
				go func() {
					time.Sleep(30 * time.Millisecond) // Shorter delay for more reliable timing
					//nolint:errcheck // Test cleanup can fail
					_ = fileLock1.Unlock()
				}()

				// Second lock should succeed after first unlocks
				err = fileLock2Extended.Lock()
				require.NoError(t, err)
				assert.True(t, fileLock2Extended.IsLocked())

				//nolint:errcheck // Test cleanup can fail
				_ = fileLock2Extended.Unlock()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}

func TestFileLock_StaleLockDetection(t *testing.T) {
	fileLock, lockFile, cleanup := setupTestFileLock(t, testLockTimeout)
	defer cleanup()

	t.Run("detect_stale_lock_from_dead_process", func(t *testing.T) {
		// Create a fake stale lock file with non-existent PID
		staleLockInfo := createTestLockInfo(99999) // Very high PID unlikely to exist
		
		lockData, err := json.Marshal(staleLockInfo)
		require.NoError(t, err)
		
		err = os.WriteFile(lockFile, lockData, 0o600)
		require.NoError(t, err)

		// Lock should succeed by detecting and clearing stale lock
		err = fileLock.Lock()
		require.NoError(t, err)
		assert.True(t, fileLock.IsLocked())

		//nolint:errcheck // Test cleanup can fail
		_ = fileLock.Unlock()
	})

	t.Run("detect_stale_lock_from_old_timestamp", func(t *testing.T) {
		// Create a fake lock with old timestamp
		staleLockInfo := createTestLockInfo(os.Getpid())
		staleLockInfo.Timestamp = time.Now().Add(-24 * time.Hour) // Very old timestamp
		
		lockData, err := json.Marshal(staleLockInfo)
		require.NoError(t, err)
		
		err = os.WriteFile(lockFile, lockData, 0o600)
		require.NoError(t, err)

		// Lock should succeed by detecting stale lock with old timestamp
		err = fileLock.Lock()
		require.NoError(t, err)
		assert.True(t, fileLock.IsLocked())

		//nolint:errcheck // Test cleanup can fail
		_ = fileLock.Unlock()
	})
}

func TestFileLock_GetLockInfo(t *testing.T) {
	fileLock, _, cleanup := setupTestFileLock(t, testLockTimeout)
	defer cleanup()

	t.Run("get_lock_info_when_locked", func(t *testing.T) {
		err := fileLock.Lock()
		require.NoError(t, err)
		defer func() {
			//nolint:errcheck // Test cleanup can fail
			_ = fileLock.Unlock()
		}()

		info, err := fileLock.GetLockInfo()
		require.NoError(t, err)
		require.NotNil(t, info)

		assert.Equal(t, os.Getpid(), info.PID)
		assert.False(t, info.Timestamp.IsZero())
		assert.False(t, info.IsStale)
	})

	t.Run("get_lock_info_when_not_locked", func(t *testing.T) {
		info, err := fileLock.GetLockInfo()
		
		// Should return error when no lock exists
		require.Error(t, err)
		assert.Nil(t, info)
	})

	t.Run("get_lock_info_with_external_lock", func(t *testing.T) {
		// Create lock file manually in the correct format (PID\nTIMESTAMP\n)
		testPID := 12345
		testTimestamp := time.Now().Unix()
		lockData := fmt.Sprintf("%d\n%d\n", testPID, testTimestamp)
		
		err := os.WriteFile(fileLock.lockFile, []byte(lockData), 0o600)
		require.NoError(t, err)

		info, err := fileLock.GetLockInfo()
		require.NoError(t, err)
		require.NotNil(t, info)

		assert.Equal(t, 12345, info.PID)
		assert.False(t, info.Timestamp.IsZero())
	})
}

func TestFileLock_ForceClearLock(t *testing.T) {
	fileLock, _, cleanup := setupTestFileLock(t, testLockTimeout)
	defer cleanup()

	t.Run("force_clear_existing_lock", func(t *testing.T) {
		// Create a lock
		err := fileLock.Lock()
		require.NoError(t, err)
		assert.True(t, fileLock.IsLocked())

		// Force clear should work
		err = fileLock.ForceClearLock()
		require.NoError(t, err)
		assert.False(t, fileLock.IsLocked())

		// Lock file should be gone
		_, err = os.Stat(fileLock.lockFile)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("force_clear_no_lock_file", func(t *testing.T) {
		// Force clear when no lock exists should not error
		err := fileLock.ForceClearLock()
		assert.NoError(t, err)
	})

	t.Run("force_clear_external_lock", func(t *testing.T) {
		// Create external lock
		externalInfo := createTestLockInfo(99999)
		lockData, err := json.Marshal(externalInfo)
		require.NoError(t, err)
		
		err = os.WriteFile(fileLock.lockFile, lockData, 0o600)
		require.NoError(t, err)

		// Force clear should remove external lock
		err = fileLock.ForceClearLock()
		require.NoError(t, err)

		// Lock file should be gone
		_, err = os.Stat(fileLock.lockFile)
		assert.True(t, os.IsNotExist(err))
	})
}

func TestFileLock_CorruptedLockFile(t *testing.T) {
	fileLock, _, cleanup := setupTestFileLock(t, testLockTimeout)
	defer cleanup()

	tests := []struct {
		name        string
		corruptData string
		expectError bool
	}{
		{
			name:        "invalid_json",
			corruptData: `{"invalid": json syntax}`,
			expectError: false, // Should treat as stale and clear
		},
		{
			name:        "empty_file",
			corruptData: "",
			expectError: false, // Should treat as stale and clear
		},
		{
			name:        "partial_json",
			corruptData: `{"pid": 123`,
			expectError: false, // Should treat as stale and clear
		},
		{
			name:        "wrong_structure",
			corruptData: `{"wrong": "structure"}`,
			expectError: false, // Should treat as stale and clear
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create corrupted lock file
			err := os.WriteFile(fileLock.lockFile, []byte(tt.corruptData), 0o600)
			require.NoError(t, err)

			// Lock should succeed by clearing corrupted file
			err = fileLock.Lock()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.True(t, fileLock.IsLocked())
				//nolint:errcheck // Test cleanup can fail
				_ = fileLock.Unlock()
			}
		})
	}
}

func TestFileLock_MultipleGoroutinesConcurrent(t *testing.T) {
	lockFile := filepath.Join(t.TempDir(), "concurrent.lock")
	const numGoroutines = 10
	const attemptsPerGoroutine = 5

	results := make([][]bool, numGoroutines)
	errors := make([][]error, numGoroutines)
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Multiple goroutines trying to acquire the same lock
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			
			results[id] = make([]bool, attemptsPerGoroutine)
			errors[id] = make([]error, attemptsPerGoroutine)
			
			for j := 0; j < attemptsPerGoroutine; j++ {
				fileLock := NewFileLock(lockFile, 50*time.Millisecond) // Short timeout
				
				err := fileLock.Lock()
				if err == nil {
					results[id][j] = true
					
					// Hold lock briefly
					time.Sleep(10 * time.Millisecond)
					//nolint:errcheck // Test cleanup can fail
					_ = fileLock.Unlock()
				} else {
					errors[id][j] = err
				}
			}
		}(i)
	}

	wg.Wait()

	// Analyze results
	totalSuccesses := 0
	totalTimeouts := 0
	
	for i := 0; i < numGoroutines; i++ {
		for j := 0; j < attemptsPerGoroutine; j++ {
			if results[i][j] {
				totalSuccesses++
			} else if errors[i][j] != nil && assert.ErrorIs(t, errors[i][j], ErrLockTimeout) {
				totalTimeouts++
			}
		}
	}

	// We should have some successes and some timeouts due to contention
	assert.Positive(t, totalSuccesses, "At least some lock attempts should succeed")
	assert.Positive(t, totalTimeouts, "Some lock attempts should timeout due to contention")
	
	t.Logf("Concurrent test results: %d successes, %d timeouts", totalSuccesses, totalTimeouts)
}

func TestFileLock_TimeoutBehavior(t *testing.T) {
	fileLock1, lockFile, cleanup := setupTestFileLock(t, testLockTimeout)
	defer cleanup()

	tests := []struct {
		name            string
		timeout         time.Duration
		lockDelay       time.Duration
		expectTimeout   bool
		maxWaitTime     time.Duration
	}{
		{
			name:          "timeout_before_lock_release",
			timeout:       100 * time.Millisecond,
			lockDelay:     200 * time.Millisecond, // Hold lock longer than timeout
			expectTimeout: true,
			maxWaitTime:   150 * time.Millisecond,
		},
		{
			name:          "success_after_lock_release",
			timeout:       200 * time.Millisecond,
			lockDelay:     50 * time.Millisecond, // Release before timeout
			expectTimeout: false,
			maxWaitTime:   250 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// First lock takes the lock
			err := fileLock1.Lock()
			require.NoError(t, err)

			// Create second lock instance with specific timeout
			fileLock2 := NewFileLock(lockFile, tt.timeout)

			// Start goroutine to release first lock after delay
			go func() {
				time.Sleep(tt.lockDelay)
				//nolint:errcheck // Test cleanup can fail
				_ = fileLock1.Unlock()
			}()

			// Measure time for second lock attempt
			start := time.Now()
			err = fileLock2.Lock()
			elapsed := time.Since(start)

			if tt.expectTimeout {
				require.Error(t, err)
				require.ErrorIs(t, err, ErrLockTimeout)
				assert.Less(t, elapsed, tt.maxWaitTime)
			} else {
				require.NoError(t, err)
				assert.True(t, fileLock2.IsLocked())
				//nolint:errcheck // Test cleanup can fail
				_ = fileLock2.Unlock()
			}
		})
	}
}

func TestFileLock_IsLocked(t *testing.T) {
	fileLock, _, cleanup := setupTestFileLock(t, testLockTimeout)
	defer cleanup()

	// Initially not locked
	assert.False(t, fileLock.IsLocked())

	// After locking
	err := fileLock.Lock()
	require.NoError(t, err)
	assert.True(t, fileLock.IsLocked())

	// After unlocking
	err = fileLock.Unlock()
	require.NoError(t, err)
	assert.False(t, fileLock.IsLocked())
}

func TestFileLock_OwnershipValidation(t *testing.T) {
	fileLock1, lockFile, cleanup := setupTestFileLock(t, testLockTimeout)
	defer cleanup()

	// Create second FileLock instance
	fileLock2 := NewFileLock(lockFile, testLockTimeout)

	t.Run("cannot_unlock_others_lock", func(t *testing.T) {
		// First instance locks
		err := fileLock1.Lock()
		require.NoError(t, err)

		// Second instance should not be able to unlock
		err = fileLock2.Unlock()
		require.Error(t, err)
		require.ErrorIs(t, err, ErrNotOwner)

		// First instance should still own the lock
		assert.True(t, fileLock1.IsLocked())
		
		// Clean up
		//nolint:errcheck // Test cleanup can fail
		_ = fileLock1.Unlock()
	})

	t.Run("owner_can_unlock", func(t *testing.T) {
		// Lock and unlock with same instance
		err := fileLock1.Lock()
		require.NoError(t, err)
		
		err = fileLock1.Unlock()
		require.NoError(t, err)
		assert.False(t, fileLock1.IsLocked())
	})
}

func TestFileLock_DirectoryCreation(t *testing.T) {
	tempDir := t.TempDir()
	lockFile := filepath.Join(tempDir, "nested", "deep", "directory", "test.lock")
	
	fileLock := NewFileLock(lockFile, testLockTimeout)
	
	// Lock should succeed and create necessary directories
	err := fileLock.Lock()
	require.NoError(t, err)
	assert.True(t, fileLock.IsLocked())
	
	// Verify lock file was created
	_, err = os.Stat(lockFile)
	require.NoError(t, err)
	
	// Verify directory structure was created
	dir := filepath.Dir(lockFile)
	_, err = os.Stat(dir)
	require.NoError(t, err)
	
	//nolint:errcheck // Test cleanup can fail
	_ = fileLock.Unlock()
}