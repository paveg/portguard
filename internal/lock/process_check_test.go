package lock

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcessExists(t *testing.T) {
	t.Run("current_process_exists", func(t *testing.T) {
		// Current process should exist
		currentPID := os.Getpid()
		exists := processExists(currentPID)
		assert.True(t, exists, "Current process should exist")
	})

	t.Run("invalid_negative_pid", func(t *testing.T) {
		// Negative PID handling is OS-specific
		// On Unix systems, negative PIDs might return permission errors
		// Just ensure it doesn't panic and returns consistently
		exists := processExists(-1)
		_ = exists // Don't assert specific result as behavior varies by OS
	})

	t.Run("zero_pid", func(t *testing.T) {
		// PID 0 handling depends on OS, but should be handled gracefully
		exists := processExists(0)
		// Don't assert true/false as PID 0 behavior is OS-specific
		// Just ensure it doesn't panic
		_ = exists
	})

	t.Run("very_high_pid_unlikely_to_exist", func(t *testing.T) {
		// Very high PID that's unlikely to exist
		exists := processExists(999999)
		assert.False(t, exists, "Very high PID should not exist")
	})
}

func TestGetCurrentPID(t *testing.T) {
	t.Run("returns_valid_pid", func(t *testing.T) {
		pid := getCurrentPID()
		assert.Greater(t, pid, 0, "Current PID should be positive")

		// Should match os.Getpid()
		expectedPID := os.Getpid()
		assert.Equal(t, expectedPID, pid, "Should return same PID as os.Getpid()")
	})
}
