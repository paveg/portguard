package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/paveg/portguard/internal/process"
)

// Static error variables to satisfy err113 linter
var (
	ErrNoVersionInfo      = errors.New("state file has no version information")
	ErrUnsupportedVersion = errors.New("unsupported state file version")
	ErrProcessIDMismatch  = errors.New("process ID mismatch")
	ErrProcessEmptyCmd    = errors.New("process has empty command")
	ErrProcessZeroTime    = errors.New("process has zero creation time")
)

// StateData represents the complete state stored in JSON
type StateData struct {
	Processes map[string]*process.ManagedProcess `json:"processes"`
	Metadata  *Metadata                          `json:"metadata"`
}

// Metadata contains information about the state file
type Metadata struct {
	Version   string    `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	PIDFile   string    `json:"pid_file"`
}

// JSONStore implements StateStore interface using JSON files
type JSONStore struct {
	filePath string
	data     *StateData
}

// NewJSONStore creates a new JSON-based state store
func NewJSONStore(filePath string) (*JSONStore, error) {
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

	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(filePath), 0o750); err != nil {
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	// Load existing data if file exists
	if _, err := os.Stat(filePath); err == nil {
		if err := store.load(); err != nil {
			return nil, fmt.Errorf("failed to load existing state: %w", err)
		}
	}

	return store, nil
}

// Save persists the processes to JSON file
func (js *JSONStore) Save(processes map[string]*process.ManagedProcess) error {
	js.data.Processes = processes
	js.data.Metadata.UpdatedAt = time.Now()

	// Marshal to JSON with indentation for readability
	data, err := json.MarshalIndent(js.data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state data: %w", err)
	}

	// Write to temporary file first for atomic operation
	tempFile := js.filePath + ".tmp"
	if err := os.WriteFile(tempFile, data, 0o600); err != nil {
		return fmt.Errorf("failed to write temp state file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempFile, js.filePath); err != nil {
		_ = os.Remove(tempFile) //nolint:errcheck // Best effort cleanup of temp file
		return fmt.Errorf("failed to rename state file: %w", err)
	}

	return nil
}

// Load reads the processes from JSON file
func (js *JSONStore) Load() (map[string]*process.ManagedProcess, error) {
	if err := js.load(); err != nil {
		return nil, err
	}

	return js.data.Processes, nil
}

// load is the internal method to load data from file
func (js *JSONStore) load() error {
	data, err := os.ReadFile(js.filePath)
	if err != nil {
		return fmt.Errorf("failed to read state file: %w", err)
	}

	if err := json.Unmarshal(data, js.data); err != nil {
		return fmt.Errorf("failed to unmarshal state data: %w", err)
	}

	return nil
}

// Delete removes a process from the store
func (js *JSONStore) Delete(id string) error {
	if err := js.load(); err != nil {
		return err
	}

	delete(js.data.Processes, id)
	return js.Save(js.data.Processes)
}

// GetFilePath returns the file path being used
func (js *JSONStore) GetFilePath() string {
	return js.filePath
}

// GetMetadata returns the metadata about the state
func (js *JSONStore) GetMetadata() *Metadata {
	return js.data.Metadata
}

// ValidateState performs validation on the loaded state
func (js *JSONStore) ValidateState() error {
	if js.data.Metadata.Version == "" {
		return ErrNoVersionInfo
	}

	// Validate each process
	for id, proc := range js.data.Processes {
		if proc.ID != id {
			return fmt.Errorf("%w: key=%s, proc.ID=%s", ErrProcessIDMismatch, id, proc.ID)
		}

		if proc.Command == "" {
			return fmt.Errorf("%w: %s", ErrProcessEmptyCmd, id)
		}

		if proc.CreatedAt.IsZero() {
			return fmt.Errorf("%w: %s", ErrProcessZeroTime, id)
		}
	}

	return nil
}

// BackupState creates a backup of the current state file
func (js *JSONStore) BackupState() error {
	if _, err := os.Stat(js.filePath); os.IsNotExist(err) {
		return nil // No state file to backup
	}

	backupPath := js.filePath + ".backup." + time.Now().Format("20060102-150405")

	data, err := os.ReadFile(js.filePath)
	if err != nil {
		return fmt.Errorf("failed to read state file for backup: %w", err)
	}

	if err := os.WriteFile(backupPath, data, 0o600); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	return nil
}

// CleanupOldBackups removes backup files older than the specified duration
func (js *JSONStore) CleanupOldBackups(maxAge time.Duration) error {
	dir := filepath.Dir(js.filePath)
	baseFileName := filepath.Base(js.filePath)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read state directory: %w", err)
	}

	cutoff := time.Now().Add(-maxAge)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !filepath.HasPrefix(name, baseFileName+".backup.") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			fullPath := filepath.Join(dir, name)
			_ = os.Remove(fullPath) //nolint:errcheck // Best effort cleanup operation
		}
	}

	return nil
}
