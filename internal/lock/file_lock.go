// Package lock provides file-based locking mechanisms for Portguard.
// It implements cross-process synchronization to prevent concurrent access conflicts
// during process management operations.
package lock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Static error variables to satisfy err113 linter
var (
	ErrLockTimeout       = errors.New("failed to acquire lock within timeout")
	ErrNotOwner          = errors.New("cannot unlock: we don't own the lock")
	ErrInvalidLockFormat = errors.New("invalid lock file format")
)

// Global counter to ensure unique instance IDs
var instanceCounter uint64

// FileLock implements LockManager interface using file-based locking
type FileLock struct {
	lockFile    string
	lockTimeout time.Duration
	locked      bool
	instanceID  uint64        // Unique identifier for this instance
	mu          sync.Mutex    // Protects locked field
}

// NewFileLock creates a new file-based lock manager
func NewFileLock(lockFile string, timeout time.Duration) *FileLock {
	// Generate unique instance ID combining timestamp with atomic counter
	// This prevents collisions when multiple instances are created rapidly
	counter := atomic.AddUint64(&instanceCounter, 1)
	
	// UnixNano() is always non-negative since Unix epoch, but cast carefully
	now := time.Now().UnixNano()
	//nolint:gosec // UnixNano() is always positive since 1970, safe to cast
	instanceID := (uint64(now) << 16) | (counter & 0xFFFF)
	
	return &FileLock{
		lockFile:    lockFile,
		lockTimeout: timeout,
		locked:      false,
		instanceID:  instanceID,
	}
}

// Lock acquires the file lock
func (fl *FileLock) Lock() error {
	// Check if this instance already holds the lock (re-entrant locking)
	fl.mu.Lock()
	if fl.locked {
		fl.mu.Unlock()
		return nil
	}
	fl.mu.Unlock()

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

			// Write PID, timestamp, and instance ID to lock file
			lockData := fmt.Sprintf("%d\n%d\n%d\n", pid, timestamp, fl.instanceID)
			if _, err := file.WriteString(lockData); err != nil {
				file.Close()
				return fmt.Errorf("failed to write lock data: %w", err)
			}
			file.Close()

			// Set locked flag under mutex protection
			fl.mu.Lock()
			fl.locked = true
			fl.mu.Unlock()
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
	fl.mu.Lock()
	defer fl.mu.Unlock()

	// Check if lock file exists
	_, err := os.Stat(fl.lockFile)
	if os.IsNotExist(err) {
		if fl.locked {
			// Our state is out of sync, reset it
			fl.locked = false
		}
		return errors.New("cannot unlock: lock is not held")
	}

	// Check ownership first - if we don't own the lock, return ErrNotOwner regardless of internal state
	if !fl.ownsLock() {
		return ErrNotOwner
	}

	// If we own the lock but our internal state says we're not locked, this is a state inconsistency
	if !fl.locked {
		return errors.New("cannot unlock: lock is not held")
	}

	// Remove the lock file
	if err := os.Remove(fl.lockFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove lock file: %w", err)
	}

	fl.locked = false
	return nil
}

// IsLocked checks if the lock is currently held
func (fl *FileLock) IsLocked() bool {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	return fl.locked
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

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 3 {
		return false // Invalid format, missing instance ID
	}

	// Parse PID
	pid, err := strconv.Atoi(lines[0])
	if err != nil {
		return false
	}

	// Parse instance ID
	instanceID, err := strconv.ParseUint(lines[2], 10, 64)
	if err != nil {
		return false
	}

	// Check both PID and instance ID
	return pid == os.Getpid() && instanceID == fl.instanceID
}

// GetLockInfo returns information about the current lock holder
func (fl *FileLock) GetLockInfo() (*Info, error) {
	data, err := os.ReadFile(fl.lockFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read lock file: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 2 {
		return nil, ErrInvalidLockFormat
	}

	pid, err := strconv.Atoi(lines[0])
	if err != nil {
		return nil, fmt.Errorf("invalid PID in lock file: %w", err)
	}

	timestamp, err := strconv.ParseInt(lines[1], 10, 64)
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
	fl.mu.Lock()
	defer fl.mu.Unlock()

	if err := os.Remove(fl.lockFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to force clear lock: %w", err)
	}

	fl.locked = false
	return nil
}
