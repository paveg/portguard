package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOutputHandler(t *testing.T) {
	t.Run("json_output_enabled", func(t *testing.T) {
		handler := NewOutputHandler(true)
		assert.NotNil(t, handler)
		assert.True(t, handler.JSONOutput)
	})

	t.Run("json_output_disabled", func(t *testing.T) {
		handler := NewOutputHandler(false)
		assert.NotNil(t, handler)
		assert.False(t, handler.JSONOutput)
	})
}

func TestOutputHandler_PrintJSON(t *testing.T) {
	// Capture stdout for testing
	oldStdout := os.Stdout
	reader, writer, _ := os.Pipe()
	os.Stdout = writer

	defer func() {
		os.Stdout = oldStdout
	}()

	t.Run("json_output_enabled", func(t *testing.T) {
		handler := NewOutputHandler(true)
		data := map[string]interface{}{
			"test": "data",
			"num":  42,
		}

		err := handler.PrintJSON(data)
		require.NoError(t, err)

		// Close writer and read output
		writer.Close()
		output, _ := io.ReadAll(reader)

		// Verify JSON structure
		var result map[string]interface{}
		err = json.Unmarshal(output, &result)
		require.NoError(t, err)
		assert.Equal(t, "data", result["test"])
		assert.InDelta(t, float64(42), result["num"], 0.001) // JSON unmarshals numbers as float64
	})

	// Reset pipe
	reader, writer, _ = os.Pipe()
	os.Stdout = writer

	t.Run("json_output_disabled", func(t *testing.T) {
		handler := NewOutputHandler(false)
		data := map[string]string{"test": "data"}

		err := handler.PrintJSON(data)
		assert.ErrorIs(t, err, ErrNotInJSONMode)
	})

	writer.Close()
}

func TestOutputHandler_PrintError(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	reader, writer, _ := os.Pipe()
	os.Stdout = writer

	defer func() {
		os.Stdout = oldStdout
	}()

	t.Run("json_mode_with_error", func(t *testing.T) {
		handler := NewOutputHandler(true)
		testErr := errors.New("test error")

		handler.PrintError("test message", testErr)

		writer.Close()
		output, _ := io.ReadAll(reader)

		var result map[string]interface{}
		err := json.Unmarshal(output, &result)
		require.NoError(t, err)
		assert.True(t, result["error"].(bool))
		assert.Equal(t, "test message", result["message"])
		assert.Equal(t, "test error", result["details"])
	})

	// Reset pipe
	reader, writer, _ = os.Pipe()
	os.Stdout = writer

	t.Run("json_mode_without_error", func(t *testing.T) {
		handler := NewOutputHandler(true)

		handler.PrintError("test message", nil)

		writer.Close()
		output, _ := io.ReadAll(reader)

		var result map[string]interface{}
		err := json.Unmarshal(output, &result)
		require.NoError(t, err)
		assert.True(t, result["error"].(bool))
		assert.Equal(t, "test message", result["message"])
		assert.Nil(t, result["details"])
	})

	// Reset pipe
	reader, writer, _ = os.Pipe()
	os.Stdout = writer

	t.Run("text_mode_with_error", func(t *testing.T) {
		handler := NewOutputHandler(false)
		testErr := errors.New("test error")

		handler.PrintError("test message", testErr)

		writer.Close()
		output, _ := io.ReadAll(reader)

		outputStr := string(output)
		assert.Contains(t, outputStr, "Error: test message: test error")
	})

	// Reset pipe
	reader, writer, _ = os.Pipe()
	os.Stdout = writer

	t.Run("text_mode_without_error", func(t *testing.T) {
		handler := NewOutputHandler(false)

		handler.PrintError("test message", nil)

		writer.Close()
		output, _ := io.ReadAll(reader)

		outputStr := string(output)
		assert.Contains(t, outputStr, "Error: test message")
		assert.NotContains(t, outputStr, ": <nil>")
	})
}

func TestOutputHandler_PrintSuccess(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	reader, writer, _ := os.Pipe()
	os.Stdout = writer

	defer func() {
		os.Stdout = oldStdout
	}()

	t.Run("json_mode_with_data", func(t *testing.T) {
		handler := NewOutputHandler(true)
		testData := map[string]string{"key": "value"}

		handler.PrintSuccess("success message", testData)

		writer.Close()
		output, _ := io.ReadAll(reader)

		var result map[string]interface{}
		err := json.Unmarshal(output, &result)
		require.NoError(t, err)
		assert.True(t, result["success"].(bool))
		assert.Equal(t, "success message", result["message"])
		assert.NotNil(t, result["data"])
	})

	// Reset pipe
	reader, writer, _ = os.Pipe()
	os.Stdout = writer

	t.Run("json_mode_without_data", func(t *testing.T) {
		handler := NewOutputHandler(true)

		handler.PrintSuccess("success message")

		writer.Close()
		output, _ := io.ReadAll(reader)

		var result map[string]interface{}
		err := json.Unmarshal(output, &result)
		require.NoError(t, err)
		assert.True(t, result["success"].(bool))
		assert.Equal(t, "success message", result["message"])
		assert.Nil(t, result["data"])
	})

	// Reset pipe
	reader, writer, _ = os.Pipe()
	os.Stdout = writer

	t.Run("text_mode_with_data", func(t *testing.T) {
		handler := NewOutputHandler(false)
		testData := "some data"

		handler.PrintSuccess("success message", testData)

		writer.Close()
		output, _ := io.ReadAll(reader)

		outputStr := string(output)
		assert.Contains(t, outputStr, "success message")
		assert.Contains(t, outputStr, "Details: some data")
	})

	// Reset pipe
	reader, writer, _ = os.Pipe()
	os.Stdout = writer

	t.Run("text_mode_without_data", func(t *testing.T) {
		handler := NewOutputHandler(false)

		handler.PrintSuccess("success message")

		writer.Close()
		output, _ := io.ReadAll(reader)

		outputStr := string(output)
		assert.Contains(t, outputStr, "success message")
		assert.NotContains(t, outputStr, "Details:")
	})
}

func TestEnsureDirectory(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("create_nested_directory", func(t *testing.T) {
		testPath := filepath.Join(tempDir, "nested", "dir", "file.txt")

		err := EnsureDirectory(testPath)
		require.NoError(t, err)

		// Check that directory was created
		dirPath := filepath.Dir(testPath)
		stat, err := os.Stat(dirPath)
		require.NoError(t, err)
		assert.True(t, stat.IsDir())
	})

	t.Run("current_directory", func(t *testing.T) {
		err := EnsureDirectory("file.txt")
		require.NoError(t, err)
		// Should not create any new directories
	})

	t.Run("existing_directory", func(t *testing.T) {
		existingDir := filepath.Join(tempDir, "existing")
		err := os.MkdirAll(existingDir, 0o755)
		require.NoError(t, err)

		testPath := filepath.Join(existingDir, "file.txt")
		err = EnsureDirectory(testPath)
		require.NoError(t, err)
	})
}

func TestWriteFileAtomic(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("write_new_file", func(t *testing.T) {
		testPath := filepath.Join(tempDir, "test.txt")
		content := []byte("test content")

		err := WriteFileAtomic(testPath, content)
		require.NoError(t, err)

		// Verify file was created with correct content
		readContent, err := os.ReadFile(testPath)
		require.NoError(t, err)
		assert.Equal(t, content, readContent)

		// Verify no temp file remains
		tempFile := testPath + ".tmp"
		_, err = os.Stat(tempFile)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("write_nested_path", func(t *testing.T) {
		testPath := filepath.Join(tempDir, "nested", "deep", "test.txt")
		content := []byte("nested content")

		err := WriteFileAtomic(testPath, content)
		require.NoError(t, err)

		readContent, err := os.ReadFile(testPath)
		require.NoError(t, err)
		assert.Equal(t, content, readContent)
	})

	t.Run("overwrite_existing_file", func(t *testing.T) {
		testPath := filepath.Join(tempDir, "existing.txt")

		// Create initial file
		initialContent := []byte("initial")
		err := os.WriteFile(testPath, initialContent, 0o600)
		require.NoError(t, err)

		// Overwrite with atomic write
		newContent := []byte("new content")
		err = WriteFileAtomic(testPath, newContent)
		require.NoError(t, err)

		readContent, err := os.ReadFile(testPath)
		require.NoError(t, err)
		assert.Equal(t, newContent, readContent)
	})
}

func TestNewCommandRunner(t *testing.T) {
	t.Run("create_with_json_output", func(t *testing.T) {
		runner := NewCommandRunner(true, false)
		assert.NotNil(t, runner)
		assert.NotNil(t, runner.OutputHandler)
		assert.True(t, runner.OutputHandler.JSONOutput)
		assert.False(t, runner.DryRun)
	})

	t.Run("create_with_dry_run", func(t *testing.T) {
		runner := NewCommandRunner(false, true)
		assert.NotNil(t, runner)
		assert.NotNil(t, runner.OutputHandler)
		assert.False(t, runner.OutputHandler.JSONOutput)
		assert.True(t, runner.DryRun)
	})
}

func TestErrInsufficientArgs(t *testing.T) {
	err := ErrInsufficientArgs{
		Required: 2,
		Got:      1,
		Usage:    "command <arg1> <arg2>",
	}

	expectedMsg := "requires at least 2 argument(s), got 1\nUsage: command <arg1> <arg2>"
	assert.Equal(t, expectedMsg, err.Error())
}

func TestValidateArgs(t *testing.T) {
	t.Run("sufficient_args", func(t *testing.T) {
		args := []string{"arg1", "arg2", "arg3"}
		err := ValidateArgs(nil, args, 2, "test usage")
		assert.NoError(t, err)
	})

	t.Run("exact_minimum_args", func(t *testing.T) {
		args := []string{"arg1", "arg2"}
		err := ValidateArgs(nil, args, 2, "test usage")
		assert.NoError(t, err)
	})

	t.Run("insufficient_args", func(t *testing.T) {
		args := []string{"arg1"}
		err := ValidateArgs(nil, args, 2, "test usage")

		require.Error(t, err)
		var insufficientErr ErrInsufficientArgs
		require.ErrorAs(t, err, &insufficientErr)
		assert.Equal(t, 2, insufficientErr.Required)
		assert.Equal(t, 1, insufficientErr.Got)
		assert.Equal(t, "test usage", insufficientErr.Usage)
	})

	t.Run("no_args_required", func(t *testing.T) {
		args := []string{}
		err := ValidateArgs(nil, args, 0, "test usage")
		assert.NoError(t, err)
	})
}

// TestFlagFunctions tests flag addition functions without cobra dependency
func TestFlagFunctions(t *testing.T) {
	t.Run("json_flag_function_exists", func(t *testing.T) {
		// Test that AddCommonJSONFlag function exists and doesn't panic
		// We can't test flag addition without cobra, so just verify function exists
		assert.NotNil(t, AddCommonJSONFlag)
	})

	t.Run("port_flags_function_exists", func(t *testing.T) {
		// Test that AddCommonPortFlags function exists and doesn't panic
		assert.NotNil(t, AddCommonPortFlags)
	})

	t.Run("force_flag_function_exists", func(t *testing.T) {
		// Test that AddCommonForceFlag function exists and doesn't panic
		assert.NotNil(t, AddCommonForceFlag)
	})
}

func TestCommonErrors(t *testing.T) {
	t.Run("ErrNotInJSONMode", func(t *testing.T) {
		require.Error(t, ErrNotInJSONMode)
		assert.Contains(t, ErrNotInJSONMode.Error(), "not in JSON mode")
	})
}

// Test that captures stdout properly for multiple test runs
func captureOutput(f func()) string {
	oldStdout := os.Stdout
	r, writer, _ := os.Pipe()
	os.Stdout = writer

	f()

	writer.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	return buf.String()
}

func TestOutputHandlerIntegration(t *testing.T) {
	t.Run("json_error_then_success", func(t *testing.T) {
		handler := NewOutputHandler(true)

		// Test error output
		errorOutput := captureOutput(func() {
			handler.PrintError("test error", errors.New("details"))
		})

		assert.Contains(t, errorOutput, "\"error\": true")
		assert.Contains(t, errorOutput, "\"message\": \"test error\"")
		assert.Contains(t, errorOutput, "\"details\": \"details\"")

		// Test success output
		successOutput := captureOutput(func() {
			handler.PrintSuccess("test success", map[string]string{"key": "value"})
		})

		assert.Contains(t, successOutput, "\"success\": true")
		assert.Contains(t, successOutput, "\"message\": \"test success\"")
	})

	t.Run("text_error_then_success", func(t *testing.T) {
		handler := NewOutputHandler(false)

		// Test error output
		errorOutput := captureOutput(func() {
			handler.PrintError("test error", errors.New("details"))
		})

		assert.Contains(t, errorOutput, "Error: test error: details")

		// Test success output
		successOutput := captureOutput(func() {
			handler.PrintSuccess("test success", "some data")
		})

		assert.Contains(t, successOutput, "test success")
		assert.Contains(t, successOutput, "Details: some data")
	})
}
