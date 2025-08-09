package lock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Static error variables to satisfy err113 linter
var (
	ErrLockTimeout       = errors.New("failed to acquire lock within timeout")
	ErrNotOwner          = errors.New("cannot unlock: we don't own the lock")
	ErrInvalidLockFormat = errors.New("invalid lock file format")
)

// FileLock implements LockManager interface using file-based locking
type FileLock struct {
	lockFile    string
	lockTimeout time.Duration
	locked      bool
}

// NewFileLock creates a new file-based lock manager
func NewFileLock(lockFile string, timeout time.Duration) *FileLock {
	return &FileLock{
		lockFile:    lockFile,
		lockTimeout: timeout,
		locked:      false,
	}
}

// Lock acquires the file lock
func (fl *FileLock) Lock() error {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(fl.lockFile), 0o750); err != nil {
		return fmt.Errorf("failed to create lock directory: %w", err)
	}

	// Try to acquire lock with timeout
	deadline := time.Now().Add(fl.lockTimeout)

	for time.Now().Before(deadline) {
		// Try to create lock file exclusively
		file, err := os.OpenFile(fl.lockFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			// Successfully acquired lock
			pid := os.Getpid()
			timestamp := time.Now().Unix()

			// Write PID and timestamp to lock file
			lockData := fmt.Sprintf("%d\n%d\n", pid, timestamp)
			if _, err := file.WriteString(lockData); err != nil {
				file.Close()
				return fmt.Errorf("failed to write lock data: %w", err)
			}
			file.Close()

			fl.locked = true
			return nil
		}

		// Check if existing lock is stale
		if fl.isStale() {
			// Remove stale lock and try again
			_ = os.Remove(fl.lockFile) //nolint:errcheck // Best effort cleanup of stale lock
			continue
		}

		// Wait a bit before retrying
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("%w: %v", ErrLockTimeout, fl.lockTimeout)
}

// Unlock releases the file lock
func (fl *FileLock) Unlock() error {
	if !fl.locked {
		return nil // Not locked by us
	}

	// Verify we own the lock before removing it
	if !fl.ownsLock() {
		return ErrNotOwner
	}

	if err := os.Remove(fl.lockFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove lock file: %w", err)
	}

	fl.locked = false
	return nil
}

// IsLocked checks if the lock is currently held
func (fl *FileLock) IsLocked() bool {
	if fl.locked {
		return true
	}

	// Check if lock file exists
	_, err := os.Stat(fl.lockFile)
	return err == nil
}

// isStale checks if an existing lock is stale (process no longer exists)
func (fl *FileLock) isStale() bool {
	data, err := os.ReadFile(fl.lockFile)
	if err != nil {
		return true // Can't read lock file, consider it stale
	}

	lines := string(data)
	if len(lines) < 2 {
		return true // Invalid lock file format
	}

	// Parse PID from first line
	pidStr := ""
	for i, char := range lines {
		if char == '\n' {
			pidStr = lines[:i]
			break
		}
	}

	if pidStr == "" {
		return true // No PID found
	}

	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return true // Invalid PID
	}

	// Check if process exists
	return !processExists(pid)
}

// ownsLock checks if the current process owns the lock
func (fl *FileLock) ownsLock() bool {
	data, err := os.ReadFile(fl.lockFile)
	if err != nil {
		return false
	}

	lines := string(data)
	pidStr := ""
	for i, char := range lines {
		if char == '\n' {
			pidStr = lines[:i]
			break
		}
	}

	if pidStr == "" {
		return false
	}

	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return false
	}

	return pid == os.Getpid()
}

// GetLockInfo returns information about the current lock holder
func (fl *FileLock) GetLockInfo() (*Info, error) {
	data, err := os.ReadFile(fl.lockFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read lock file: %w", err)
	}

	lines := string(data)
	parts := make([]string, 0, 2)
	current := ""

	for _, char := range lines {
		if char == '\n' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}

	if len(parts) < 2 {
		return nil, ErrInvalidLockFormat
	}

	pid, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid PID in lock file: %w", err)
	}

	timestamp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp in lock file: %w", err)
	}

	return &Info{
		PID:       pid,
		Timestamp: time.Unix(timestamp, 0),
		IsStale:   !processExists(pid),
	}, nil
}

// Info contains information about a lock
type Info struct {
	PID       int       `json:"pid"`
	Timestamp time.Time `json:"timestamp"`
	IsStale   bool      `json:"is_stale"`
}

// ForceClearLock removes the lock file regardless of ownership (use with caution)
func (fl *FileLock) ForceClearLock() error {
	if err := os.Remove(fl.lockFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to force clear lock: %w", err)
	}

	fl.locked = false
	return nil
}
