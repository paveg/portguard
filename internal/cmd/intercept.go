package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"time"

	"github.com/spf13/cobra"
	"github.com/paveg/portguard/internal/process"
	"github.com/paveg/portguard/internal/state"
	"github.com/paveg/portguard/internal/lock"
	portscanner "github.com/paveg/portguard/internal/port"
)

// InterceptRequest represents the hook request payload
type InterceptRequest struct {
	Command     string            `json:"command"`
	Args        []string          `json:"args,omitempty"`
	Context     map[string]string `json:"context,omitempty"`
	ToolName    string            `json:"tool_name,omitempty"`
	SessionID   string            `json:"session_id,omitempty"`
	WorkingDir  string            `json:"working_dir,omitempty"`
}

// InterceptResponse represents the hook response
type InterceptResponse struct {
	Action          string                 `json:"action"`           // "allow", "block", "modify"
	Reason          string                 `json:"reason"`           
	ModifiedCommand string                 `json:"modified_command,omitempty"`
	ModifiedArgs    []string               `json:"modified_args,omitempty"`
	ProcessInfo     map[string]interface{} `json:"process_info,omitempty"`
	Suggestions     []string               `json:"suggestions,omitempty"`
}

// Server command patterns for detection
var serverCommandPatterns = []*regexp.Regexp{
	// Node.js development servers
	regexp.MustCompile(`npm\s+run\s+(dev|start|serve)`),
	regexp.MustCompile(`yarn\s+(dev|start|serve)`),
	regexp.MustCompile(`pnpm\s+(dev|start|serve)`),
	regexp.MustCompile(`node.*server|server.*node`),
	
	// Go development servers
	regexp.MustCompile(`go\s+run.*main\.go`),
	regexp.MustCompile(`go\s+run.*server`),
	
	// Python development servers
	regexp.MustCompile(`python.*app\.py|app\.py.*python`),
	regexp.MustCompile(`python.*server|server.*python`),
	regexp.MustCompile(`flask\s+run`),
	regexp.MustCompile(`django.*runserver`),
	regexp.MustCompile(`uvicorn|gunicorn|hypercorn`),
	
	// Other common development servers
	regexp.MustCompile(`serve|http-server|live-server`),
	regexp.MustCompile(`--port\s+\d+|\s+-p\s+\d+`),
	regexp.MustCompile(`localhost:\d+|127\.0\.0\.1:\d+|0\.0\.0\.0:\d+`),
	
	// Framework-specific
	regexp.MustCompile(`next\s+(dev|start)`),
	regexp.MustCompile(`vite|webpack-dev-server`),
	regexp.MustCompile(`rails\s+server`),
}

var interceptCmd = &cobra.Command{
	Use:   "intercept",
	Short: "Intercept and analyze commands for Claude Code hooks",
	Long: `Analyze incoming commands from Claude Code hooks to determine if they would
create duplicate processes or port conflicts. Returns JSON response with action
recommendation.

This command is designed to be called by Claude Code PreToolUse hooks to provide
intelligent process management during AI-assisted development.

Input: JSON payload with command information
Output: JSON response with recommended action (allow/block/modify)`,
	Run: func(_ *cobra.Command, args []string) {
		runner := NewCommandRunner(true, false) // Always JSON output for hooks
		
		var request InterceptRequest
		
		// Read from stdin for hook integration (primary mode)
		if len(args) == 0 {
			scanner := bufio.NewScanner(os.Stdin)
			var jsonInput string
			
			// Read all input from stdin
			for scanner.Scan() {
				jsonInput += scanner.Text()
			}
			
			if err := scanner.Err(); err != nil {
				runner.OutputHandler.PrintError("Failed to read stdin", err)
				return
			}
			
			if jsonInput == "" {
				runner.OutputHandler.PrintError("No JSON input provided", nil)
				return
			}
			
			if err := json.Unmarshal([]byte(jsonInput), &request); err != nil {
				runner.OutputHandler.PrintError("Invalid JSON input", err)
				return
			}
		} else {
			// Fallback: parse from command line args for testing
			request.Command = args[0]
			if len(args) > 1 {
				request.Args = args[1:]
			}
		}
		
		response := analyzeCommand(&request)
		
		if err := runner.OutputHandler.PrintJSON(response); err != nil {
			runner.OutputHandler.PrintError("Failed to output response", err)
		}
	},
}

// analyzeCommand determines the appropriate action for a given command
func analyzeCommand(request *InterceptRequest) *InterceptResponse {
	response := &InterceptResponse{
		Action: "allow",
		Reason: "Command does not appear to be a server startup",
	}
	
	// Construct full command string for analysis
	fullCommand := request.Command
	if len(request.Args) > 0 {
		fullCommand += " " + strings.Join(request.Args, " ")
	}
	
	// Check if this looks like a server command
	if !isServerCommand(fullCommand) {
		return response
	}
	
	// Extract port information if available
	port := extractPort(request.Command, request.Args)
	
	// Initialize ProcessManager for real conflict detection
	processManager := createProcessManager()
	
	// Check for existing processes with this command or port
	if existingProcess := checkForExistingProcess(processManager, fullCommand, port); existingProcess != nil {
		response.Action = "block"
		response.Reason = fmt.Sprintf("Similar process already running: %s (ID: %s)", existingProcess.Command, existingProcess.ID)
		response.ProcessInfo = map[string]interface{}{
			"existing_process": map[string]interface{}{
				"id":          existingProcess.ID,
				"command":     existingProcess.Command,
				"port":        existingProcess.Port,
				"status":      existingProcess.Status,
				"created_at":  existingProcess.CreatedAt,
			},
			"detected_port": port,
			"command_type":  "server",
		}
		response.Suggestions = []string{
			fmt.Sprintf("Check existing processes: portguard list"),
			fmt.Sprintf("Stop existing process: portguard stop %s", existingProcess.ID),
			"Use a different port",
			"Verify the process is needed",
		}
		return response
	}
	
	// Update response for server commands that don't conflict
	response.Reason = "Server command detected, no conflicts found"
	response.ProcessInfo = map[string]interface{}{
		"detected_port": port,
		"command_type":  "server",
		"full_command":  fullCommand,
	}
	
	return response
}

// isServerCommand checks if a command looks like a server startup command
func isServerCommand(command string) bool {
	for _, pattern := range serverCommandPatterns {
		if pattern.MatchString(command) {
			return true
		}
	}
	return false
}

// extractPort attempts to extract port number from command and args
func extractPort(command string, args []string) int {
	// Look for --port flag patterns
	portPattern := regexp.MustCompile(`--port[=\s]+(\d+)|-p[=\s]+(\d+)`)
	
	// Check command string
	if matches := portPattern.FindStringSubmatch(command); len(matches) > 1 {
		for i := 1; i < len(matches); i++ {
			if matches[i] != "" {
				var port int
				if _, err := fmt.Sscanf(matches[i], "%d", &port); err == nil {
					return port
				}
			}
		}
	}
	
	// Check arguments
	fullCommand := strings.Join(append([]string{command}, args...), " ")
	if matches := portPattern.FindStringSubmatch(fullCommand); len(matches) > 1 {
		for i := 1; i < len(matches); i++ {
			if matches[i] != "" {
				var port int
				if _, err := fmt.Sscanf(matches[i], "%d", &port); err == nil {
					return port
				}
			}
		}
	}
	
	// Look for common default ports based on command type
	if strings.Contains(command, "npm run dev") || strings.Contains(command, "next dev") {
		return 3000 // Next.js default
	}
	if strings.Contains(command, "vite") {
		return 5173 // Vite default
	}
	if strings.Contains(command, "flask run") {
		return 5000 // Flask default
	}
	
	return 0 // No port detected
}

// createProcessManager creates a ProcessManager instance for process checking
func createProcessManager() *process.ProcessManager {
	// Initialize storage components
	stateStore, err := state.NewJSONStore("~/.portguard/state.json")
	if err != nil {
		// Fallback to empty state if we can't load
		stateStore = nil
	}
	
	lockManager := lock.NewFileLock("~/.portguard/portguard.lock", 5*time.Second)
	scanner := portscanner.NewScanner(2*time.Second)
	
	return process.NewProcessManager(stateStore, lockManager, scanner)
}

// checkForExistingProcess checks if there's an existing process that conflicts
func checkForExistingProcess(pm *process.ProcessManager, command string, port int) *process.ManagedProcess {
	// List all running processes
	processes := pm.ListProcesses(process.ProcessListOptions{
		IncludeStopped: false, // Only check running processes
	})
	
	// Check for exact command match
	for _, proc := range processes {
		if proc.Command == command && proc.IsHealthy() {
			return proc
		}
	}
	
	// Check for port conflicts if port is specified
	if port > 0 {
		for _, proc := range processes {
			if proc.Port == port && proc.IsRunning() {
				return proc
			}
		}
	}
	
	return nil
}

func init() {
	rootCmd.AddCommand(interceptCmd)
	
	interceptCmd.Flags().BoolVar(&jsonOutput, "json", true, "output in JSON format (default for hooks)")
	interceptCmd.Flags().StringVar(&cfgFile, "config", "", "config file path")
}