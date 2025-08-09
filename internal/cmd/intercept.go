package cmd

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/paveg/portguard/internal/lock"
	portscanner "github.com/paveg/portguard/internal/port"
	"github.com/paveg/portguard/internal/process"
	"github.com/paveg/portguard/internal/state"
	"github.com/spf13/cobra"
)

// Static errors for err113 compliance
var (
	ErrUnknownEvent = errors.New("unknown event type")
)

// InterceptRequest represents the official Claude Code hook request format
type InterceptRequest struct {
	Event      string                 `json:"event"`            // "preToolUse" or "postToolUse"
	ToolName   string                 `json:"tool_name"`        // e.g., "Bash"
	Parameters map[string]interface{} `json:"parameters"`       // Tool parameters
	Result     *ToolResult            `json:"result,omitempty"` // For postToolUse
	SessionID  string                 `json:"session_id,omitempty"`
	WorkingDir string                 `json:"working_dir,omitempty"`
	Timestamp  string                 `json:"timestamp,omitempty"`
}

// ToolResult represents the result from tool execution (postToolUse)
type ToolResult struct {
	Success  bool   `json:"success"`
	Output   string `json:"output,omitempty"`
	Error    string `json:"error,omitempty"`
	ExitCode int    `json:"exit_code,omitempty"`
}

// PreToolUseResponse represents the official PreToolUse hook response
type PreToolUseResponse struct {
	Proceed bool                   `json:"proceed"`
	Message string                 `json:"message,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

// PostToolUseResponse represents the official PostToolUse hook response
type PostToolUseResponse struct {
	Status  string                 `json:"status"` // "success", "warning", "error"
	Message string                 `json:"message,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

var interceptCmd = &cobra.Command{
	Use:   "intercept",
	Short: "Claude Code hooks intercept with official format",
	Long: `Process hook requests from Claude Code using the official JSON format.
Fully compatible with the Claude Code hooks specification.`,
	Run: func(_ *cobra.Command, args []string) {
		var request InterceptRequest

		// Read JSON from stdin
		scanner := bufio.NewScanner(os.Stdin)
		var jsonInput string
		for scanner.Scan() {
			jsonInput += scanner.Text()
		}

		if err := scanner.Err(); err != nil {
			outputErrorResponse(err)
			return
		}

		if err := json.Unmarshal([]byte(jsonInput), &request); err != nil {
			outputErrorResponse(err)
			return
		}

		// Route based on event type
		switch request.Event {
		case "preToolUse":
			handlePreToolUse(&request)
		case "postToolUse":
			handlePostToolUse(&request)
		default:
			outputErrorResponse(fmt.Errorf("%w: %s", ErrUnknownEvent, request.Event))
		}
	},
}

func handlePreToolUse(request *InterceptRequest) {
	response := PreToolUseResponse{
		Proceed: true,
		Message: "Command allowed",
		Data:    make(map[string]interface{}),
	}

	// Only intercept Bash commands
	if request.ToolName != "Bash" {
		response.Message = "Non-Bash tool, allowing"
		outputJSON(response)
		return
	}

	// Extract command from parameters
	command, ok := request.Parameters["command"].(string)
	if !ok || command == "" {
		response.Message = "No command found"
		outputJSON(response)
		return
	}

	// Check if it's a server command
	if !isServerCommand(command) {
		response.Message = "Not a server command"
		outputJSON(response)
		return
	}

	// Check for conflicts
	//nolint:govet // TODO: Rename variable to avoid shadowing (e.g., detectedPort)
	port := extractPort(command)
	pm := createProcessManager()

	if existing := checkForConflict(pm, command, port); existing != nil {
		response.Proceed = false
		response.Message = fmt.Sprintf("Port %d already in use by: %s", existing.Port, existing.Command)
		response.Data["existing_process"] = map[string]interface{}{
			"id":      existing.ID,
			"command": existing.Command,
			"port":    existing.Port,
			"status":  existing.Status,
		}
		response.Data["suggestions"] = []string{
			"Use 'portguard stop' to stop the existing process",
			"Choose a different port",
			"Check 'portguard list' for all processes",
		}
	} else {
		response.Message = "Server command allowed, no conflicts"
		response.Data["detected_port"] = port
	}

	outputJSON(response)
}

func handlePostToolUse(request *InterceptRequest) {
	response := PostToolUseResponse{
		Status:  "success",
		Message: "Command processed",
		Data:    make(map[string]interface{}),
	}

	// Only process successful Bash commands
	if request.ToolName != "Bash" || request.Result == nil || !request.Result.Success {
		outputJSON(response)
		return
	}

	// Extract command
	command, ok := request.Parameters["command"].(string)
	if !ok || !isServerCommand(command) {
		outputJSON(response)
		return
	}

	// Check if server started successfully
	//nolint:govet // TODO: Rename variable to avoid shadowing (e.g., outputPort)
	if port := extractPortFromOutput(request.Result.Output); port > 0 {
		// Register the process (async to not block)
		go func() {
			pm := createProcessManager()
			_, _ = pm.StartProcess(command, []string{}, process.StartOptions{
				Port:       port,
				WorkingDir: request.WorkingDir,
				Background: true,
			})
		}()

		response.Message = fmt.Sprintf("Server registered on port %d", port)
		response.Data["port"] = port
	}

	outputJSON(response)
}

func isServerCommand(command string) bool {
	patterns := []string{
		"npm run dev", "yarn dev", "pnpm dev",
		"next dev", "vite", "webpack-dev-server",
		"python.*app.py", "flask run", "django.*runserver",
		"go run.*main.go", "rails server",
	}

	for _, pattern := range patterns {
		matched, err := regexp.MatchString(pattern, command)
		if err != nil {
			continue // Skip invalid patterns
		}
		if matched {
			return true
		}
	}
	return false
}

func extractPort(command string) int {
	// Port extraction logic
	portRegex := regexp.MustCompile(`--port[=\s]+(\d+)|:(\d+)|-p[=\s]+(\d+)`)
	if matches := portRegex.FindStringSubmatch(command); len(matches) > 0 {
		for i := 1; i < len(matches); i++ {
			if matches[i] != "" {
				//nolint:govet // TODO: Rename variable to avoid shadowing (e.g., extractedPort)
				var port int
				if _, err := fmt.Sscanf(matches[i], "%d", &port); err == nil && port > 0 {
					return port
				}
			}
		}
	}

	// Default ports
	if strings.Contains(command, "next dev") {
		return 3000
	}
	if strings.Contains(command, "vite") {
		//nolint:mnd // TODO: Extract to constant defaultVitePort = 5173
		return 5173
	}
	if strings.Contains(command, "flask") {
		//nolint:mnd // TODO: Extract to constant defaultFlaskPort = 5000
		return 5000
	}

	return 0
}

func extractPortFromOutput(output string) int {
	patterns := []string{
		`listening on port (\d+)`,
		`localhost:(\d+)`,
		`http://[^:]+:(\d+)`,
		`0\.0\.0\.0:(\d+)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(strings.ToLower(output)); len(matches) > 1 {
			//nolint:govet // TODO: Rename variable to avoid shadowing (e.g., parsedPort)
			var port int
			fmt.Sscanf(matches[1], "%d", &port)
			if port > 0 {
				return port
			}
		}
	}
	return 0
}

func createProcessManager() *process.ProcessManager {
	stateStore, _ := state.NewJSONStore("~/.portguard/state.json")
	lockManager := lock.NewFileLock("~/.portguard/portguard.lock", 5*time.Second)
	//nolint:noctx // TODO: Add context support to port scanner for better timeout control
	scanner := portscanner.NewScanner(2 * time.Second)
	return process.NewProcessManager(stateStore, lockManager, scanner)
}

func checkForConflict(pm *process.ProcessManager, command string, port int) *process.ManagedProcess {
	processes := pm.ListProcesses(process.ProcessListOptions{
		IncludeStopped: false,
	})

	for _, proc := range processes {
		if proc.Command == command && proc.IsHealthy() {
			return proc
		}
		if port > 0 && proc.Port == port && proc.IsRunning() {
			return proc
		}
	}
	return nil
}

func outputJSON(v interface{}) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(v)
}

func outputErrorResponse(err error) {
	response := PreToolUseResponse{
		Proceed: true, // Fail open for safety
		Message: fmt.Sprintf("Hook error: %v", err),
	}
	outputJSON(response)
}

func init() {
	rootCmd.AddCommand(interceptCmd)
}
