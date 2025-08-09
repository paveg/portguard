package process

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestProcessStatus(t *testing.T) {
	// Test valid statuses
	validStatuses := []ProcessStatus{
		StatusPending,
		StatusRunning,
		StatusStopped,
		StatusFailed,
		StatusUnhealthy,
	}

	for _, status := range validStatuses {
		assert.NotEmpty(t, string(status))
	}
}

func TestManagedProcess_IsHealthy(t *testing.T) {
	process := &ManagedProcess{
		ID:     "test",
		Status: StatusRunning,
	}

	assert.True(t, process.IsHealthy())

	process.Status = StatusStopped
	assert.False(t, process.IsHealthy())
}

func TestManagedProcess_IsRunning(t *testing.T) {
	process := &ManagedProcess{
		ID:     "test",
		Status: StatusRunning,
	}

	assert.True(t, process.IsRunning())

	process.Status = StatusUnhealthy
	assert.True(t, process.IsRunning())

	process.Status = StatusStopped
	assert.False(t, process.IsRunning())
}

func TestManagedProcess_Age(t *testing.T) {
	now := time.Now()
	process := &ManagedProcess{
		ID:        "test",
		CreatedAt: now.Add(-time.Hour),
	}

	age := process.Age()
	assert.True(t, age > 50*time.Minute)
	assert.True(t, age < 2*time.Hour)
}

func TestHealthCheckType(t *testing.T) {
	// Test valid health check types
	validTypes := []HealthCheckType{
		HealthCheckHTTP,
		HealthCheckTCP,
		HealthCheckCommand,
		HealthCheckNone,
	}

	for _, hcType := range validTypes {
		assert.NotEmpty(t, string(hcType))
	}
}

func TestHealthCheck(t *testing.T) {
	hc := &HealthCheck{
		Type:     HealthCheckHTTP,
		Target:   "http://localhost:3000/health",
		Interval: 30 * time.Second,
		Timeout:  5 * time.Second,
		Retries:  3,
	}

	assert.Equal(t, HealthCheckHTTP, hc.Type)
	assert.Equal(t, "http://localhost:3000/health", hc.Target)
	assert.Equal(t, 30*time.Second, hc.Interval)
	assert.Equal(t, 5*time.Second, hc.Timeout)
	assert.Equal(t, 3, hc.Retries)
}