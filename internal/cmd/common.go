package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// Common error definitions
var (
	ErrNotInJSONMode = errors.New("not in JSON mode")
)

// Common variables used across multiple commands
var (
	port        int
	jsonOutput  bool
	startPort   int
	force       bool
	dryRun      bool
	showAll     bool
	healthCheck string
	background  bool
	verbose     bool
	cfgFile     string
)

// OutputHandler provides common output formatting
type OutputHandler struct {
	JSONOutput bool
}

// NewOutputHandler creates a new output handler
func NewOutputHandler(jsonOutput bool) *OutputHandler {
	return &OutputHandler{JSONOutput: jsonOutput}
}

// PrintJSON outputs data as JSON or returns error
func (oh *OutputHandler) PrintJSON(data interface{}) error {
	if oh.JSONOutput {
		output, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %w", err)
		}
		fmt.Println(string(output))
		return nil
	}
	return ErrNotInJSONMode
}

// PrintError prints error message consistently
func (oh *OutputHandler) PrintError(msg string, err error) {
	if oh.JSONOutput {
		errorData := map[string]interface{}{
			"error":   true,
			"message": msg,
		}
		if err != nil {
			errorData["details"] = err.Error()
		}
		_ = oh.PrintJSON(errorData) //nolint:errcheck // JSON marshal error in error handler should not cause panic
	} else {
		if err != nil {
			fmt.Printf("Error: %s: %v\n", msg, err)
		} else {
			fmt.Printf("Error: %s\n", msg)
		}
	}
}

// PrintSuccess prints success message consistently
func (oh *OutputHandler) PrintSuccess(msg string, data ...interface{}) {
	if oh.JSONOutput {
		result := map[string]interface{}{
			"success": true,
			"message": msg,
		}
		if len(data) > 0 {
			result["data"] = data[0]
		}
		_ = oh.PrintJSON(result) //nolint:errcheck // JSON marshal error in success handler should not cause panic
	} else {
		fmt.Println(msg)
		if len(data) > 0 {
			fmt.Printf("Details: %+v\n", data[0])
		}
	}
}

// EnsureDirectory creates directory if it doesn't exist with standard permissions
func EnsureDirectory(path string) error {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil { // More secure permissions
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

// WriteFileAtomic writes file atomically using temp file + rename
func WriteFileAtomic(path string, content []byte) error {
	if err := EnsureDirectory(path); err != nil {
		return err
	}

	tempFile := path + ".tmp"
	if err := os.WriteFile(tempFile, content, 0o600); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tempFile, path); err != nil {
		_ = os.Remove(tempFile) //nolint:errcheck // Best effort cleanup of temp file
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// CommandRunner provides standard cobra command execution pattern
type CommandRunner struct {
	OutputHandler *OutputHandler
	DryRun        bool
}

// NewCommandRunner creates a command runner with common configuration
func NewCommandRunner(jsonOutput, dryRun bool) *CommandRunner {
	return &CommandRunner{
		OutputHandler: NewOutputHandler(jsonOutput),
		DryRun:        dryRun,
	}
}

// ErrInsufficientArgs represents argument validation error
type ErrInsufficientArgs struct {
	Required int
	Got      int
	Usage    string
}

func (e ErrInsufficientArgs) Error() string {
	return fmt.Sprintf("requires at least %d argument(s), got %d\nUsage: %s", e.Required, e.Got, e.Usage)
}

// ValidateArgs validates that required arguments are provided
func ValidateArgs(_ *cobra.Command, args []string, minArgs int, usage string) error {
	if len(args) < minArgs {
		return ErrInsufficientArgs{
			Required: minArgs,
			Got:      len(args),
			Usage:    usage,
		}
	}
	return nil
}

// AddCommonJSONFlag adds the standard JSON output flag to a command
func AddCommonJSONFlag(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output in JSON format")
}

// AddCommonPortFlags adds standard port-related flags
func AddCommonPortFlags(cmd *cobra.Command) {
	cmd.Flags().IntVarP(&port, "port", "p", 0, "target port number")
	cmd.Flags().IntVar(&startPort, "start", 3000, "start port for scanning")
}

// AddCommonForceFlag adds the standard force flag
func AddCommonForceFlag(cmd *cobra.Command, usage string) {
	cmd.Flags().BoolVarP(&force, "force", "f", false, usage)
}
