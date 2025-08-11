package cmd

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
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

// ProcessManagerFactory can be overridden in tests
// Ensure thread-safe access for concurrent test execution
var (
	processManagerFactoryMu sync.RWMutex
	processManagerFactory   = createDefaultProcessManager
)

// ProcessManagerFactory returns the current factory function thread-safely
func ProcessManagerFactory() *process.ProcessManager {
	processManagerFactoryMu.RLock()
	factory := processManagerFactory
	processManagerFactoryMu.RUnlock()
	return factory()
}

// SetProcessManagerFactory sets the factory function thread-safely (for tests)
func SetProcessManagerFactory(factory func() *process.ProcessManager) func() {
	processManagerFactoryMu.Lock()
	original := processManagerFactory
	processManagerFactory = factory
	processManagerFactoryMu.Unlock()

	// Return restore function
	return func() {
		processManagerFactoryMu.Lock()
		processManagerFactory = original
		processManagerFactoryMu.Unlock()
	}
}

// InterceptRequest represents the official Claude Code hook request format
type InterceptRequest struct {
	Event      string                 `json:"event"`            // "preToolUse" or "postToolUse"
	ToolName   string                 `json:"tool_name"`        // e.g., "Bash" (official field)
	Tool       string                 `json:"tool"`             // e.g., "Bash" (alternative field)
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

	// Get tool name from either field
	toolName := request.ToolName
	if toolName == "" {
		toolName = request.Tool
	}

	// Only intercept Bash commands
	if toolName != "Bash" && toolName != "bash" {
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

	// Extract port and create process manager
	//nolint:govet // TODO: Rename variable to avoid shadowing (e.g., detectedPort)
	port := extractPort(command)
	pm := ProcessManagerFactory()

	// Check for conflicts with managed processes
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
		// Check for existing unmanaged processes that could be imported
		if port > 0 {
			if adoptableInfo := checkForAdoptableProcess(port); adoptableInfo != nil {
				response.Data["adoptable_process"] = map[string]interface{}{
					"pid":          adoptableInfo.PID,
					"process_name": adoptableInfo.ProcessName,
					"command":      adoptableInfo.Command,
					"port":         adoptableInfo.Port,
					"suitable":     adoptableInfo.IsSuitable,
					"reason":       adoptableInfo.Reason,
				}

				if adoptableInfo.IsSuitable {
					response.Message = fmt.Sprintf("Found existing process on port %d that could be imported", port)
					response.Data["suggestions"] = []string{
						fmt.Sprintf("Use 'portguard import port %d' to import the existing process", port),
						"Or proceed to start a new process (may cause conflicts)",
					}
				} else {
					response.Message = fmt.Sprintf("Found process on port %d, but not suitable for import: %s", port, adoptableInfo.Reason)
				}
			} else {
				response.Message = "Server command allowed, no conflicts detected"
				response.Data["detected_port"] = port
			}
		} else {
			response.Message = "Server command allowed, no port detected"
		}
	}

	outputJSON(response)
}

func handlePostToolUse(request *InterceptRequest) {
	response := PostToolUseResponse{
		Status:  "success",
		Message: "Command processed",
		Data:    make(map[string]interface{}),
	}

	// Only process Bash commands
	if request.ToolName != "Bash" || request.Result == nil {
		outputJSON(response)
		return
	}

	// If command failed, return error status
	if !request.Result.Success {
		response.Status = "error"
		response.Message = "Command failed"
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
			pm := ProcessManagerFactory()
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
		// Node.js patterns
		"npm run dev", "npm start", "yarn dev", "pnpm dev", "pnpm run dev",
		"node .*\\.js", "next dev", "vite", "webpack-dev-server",

		// Modern JavaScript tooling
		"turbo run dev", "turbo dev", "nx serve", "nx dev",
		"bun run dev", "bun dev", "deno run.*dev",

		// Go patterns
		"go run.*\\.go", "air", "gin", "realize start",
		"go run main\\.go", "go run \\./cmd/.*",

		// Python patterns
		"python.*-m http\\.server", "python3.*-m http\\.server",
		"flask run", "python.*manage\\.py runserver", "uvicorn",
		"gunicorn", "fastapi dev", "python.*-m flask run",

		// Rust patterns
		"cargo run", "cargo watch -x run", "trunk serve",

		// Docker/Container patterns
		"docker run.*-p \\d+", "docker-compose up", "podman run.*-p \\d+",

		// Other server patterns
		"hugo server", "jekyll serve", "php.*-S", "rails server",
		"serve", "http-server", "live-server", "browser-sync start",

		// Database servers
		"mongodb", "postgres", "mysql", "redis-server",

		// Development proxy/tunneling
		"ngrok http", "lt --port", "localtunnel",

		// Static site generators
		"gatsby develop", "nuxt dev", "gridsome develop",
		"eleventy --serve", "astro dev",
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
	// First try to extract explicitly specified port
	if explicitPort := extractExplicitPort(command); explicitPort > 0 {
		return explicitPort
	}

	// Then try framework-specific default ports
	return extractDefaultPort(command)
}

func extractExplicitPort(command string) int {
	portRegex := regexp.MustCompile(`--port[=\s]+(\d+)|:(\d+)|-p[=\s]+(\d+)|--addr[=\s]+.*:(\d+)`)
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
	return 0
}

func extractDefaultPort(command string) int {
	// JavaScript/Node.js frameworks
	if jsPort := extractJavaScriptFrameworkPort(command); jsPort > 0 {
		return jsPort
	}

	// Go frameworks
	if goPort := extractGoFrameworkPort(command); goPort > 0 {
		return goPort
	}

	// Python frameworks
	if pyPort := extractPythonFrameworkPort(command); pyPort > 0 {
		return pyPort
	}

	// Other frameworks and services
	if otherPort := extractOtherFrameworkPort(command); otherPort > 0 {
		return otherPort
	}

	return 0
}

func extractJavaScriptFrameworkPort(command string) int {
	if strings.Contains(command, "npm run dev") || strings.Contains(command, "pnpm dev") || strings.Contains(command, "pnpm run dev") {
		return 3000
	}
	if strings.Contains(command, "next dev") {
		return 3000
	}
	if strings.Contains(command, "vite") {
		//nolint:mnd // TODO: Extract to constant defaultVitePort = 5173
		return 5173
	}
	if strings.Contains(command, "turbo run dev") || strings.Contains(command, "turbo dev") {
		return 3000
	}
	if strings.Contains(command, "nx serve") || strings.Contains(command, "nx dev") {
		//nolint:mnd // TODO: Extract to constant defaultNxPort = 4200
		return 4200
	}
	if strings.Contains(command, "gatsby develop") {
		//nolint:mnd // TODO: Extract to constant defaultGatsbyPort = 8000
		return 8000
	}
	if strings.Contains(command, "nuxt dev") || strings.Contains(command, "astro dev") {
		return 3000
	}
	return 0
}

func extractGoFrameworkPort(command string) int {
	if strings.Contains(command, "air") {
		return 3000
	}
	if strings.Contains(command, "gin") {
		//nolint:mnd // TODO: Extract to constant defaultGinPort = 8080
		return 8080
	}
	return 0
}

func extractPythonFrameworkPort(command string) int {
	if strings.Contains(command, "flask run") {
		//nolint:mnd // TODO: Extract to constant defaultFlaskPort = 5000
		return 5000
	}
	if strings.Contains(command, "manage.py runserver") || strings.Contains(command, "uvicorn") ||
		strings.Contains(command, "gunicorn") || strings.Contains(command, "fastapi dev") {
		//nolint:mnd // TODO: Extract to constant defaultPythonWebPort = 8000
		return 8000
	}
	return 0
}

func extractOtherFrameworkPort(command string) int {
	// Rust frameworks
	if strings.Contains(command, "cargo run") {
		//nolint:mnd // TODO: Extract to constant defaultCargoPort = 3000
		return 3000
	}
	if strings.Contains(command, "trunk serve") {
		//nolint:mnd // TODO: Extract to constant defaultTrunkPort = 8080
		return 8080
	}

	// Static site generators
	if strings.Contains(command, "jekyll serve") {
		//nolint:mnd // TODO: Extract to constant defaultJekyllPort = 4000
		return 4000
	}
	if strings.Contains(command, "hugo server") {
		//nolint:mnd // TODO: Extract to constant defaultHugoPort = 1313
		return 1313
	}
	if strings.Contains(command, "eleventy --serve") || strings.Contains(command, "http-server") || strings.Contains(command, "live-server") {
		//nolint:mnd // TODO: Extract to constant defaultGenericWebPort = 8080
		return 8080
	}

	// Development servers (be more specific to avoid false matches)
	if strings.Contains(command, "trunk serve") || strings.Contains(command, "jekyll serve") ||
		strings.HasPrefix(command, "serve ") || command == "serve" {
		//nolint:mnd // TODO: Extract to constant defaultServePort = 5000
		return 5000
	}

	// Database servers
	if strings.Contains(command, "mongodb") {
		//nolint:mnd // TODO: Extract to constant defaultMongoPort = 27017
		return 27017
	}
	if strings.Contains(command, "postgres") {
		//nolint:mnd // TODO: Extract to constant defaultPostgresPort = 5432
		return 5432
	}
	if strings.Contains(command, "mysql") {
		//nolint:mnd // TODO: Extract to constant defaultMySQLPort = 3306
		return 3306
	}
	if strings.Contains(command, "redis-server") {
		//nolint:mnd // TODO: Extract to constant defaultRedisPort = 6379
		return 6379
	}

	return 0
}

func extractPortFromOutput(output string) int {
	patterns := []string{
		// Common server output patterns
		`localhost:(\d+)`,
		`127\.0\.0\.1:(\d+)`,
		`0\.0\.0\.0:(\d+)`,
		`listening on :(\d+)`,
		`listening on port (\d+)`,
		`port (\d+)`,
		`https?://[^:]+:(\d+)`,
		`serving at [^:]+:(\d+)`,
		`server running on [^:]+:(\d+)`,

		// Framework-specific patterns
		`Local:.*:(\d+)`,                // Vite, Webpack Dev Server
		`Network:.*:(\d+)`,              // Vite, Webpack Dev Server
		`ready on [^:]*:(\d+)`,          // Next.js
		`started server on [^:]*:(\d+)`, // Next.js
		`local:.*localhost:(\d+)`,       // Gatsby
		`on your network:.*:(\d+)`,      // Gatsby
		`listening at [^:]+:(\d+)`,      // Express.js
		`server started at [^:]+:(\d+)`, // Various frameworks

		// Rust patterns
		`listening on [^:]+:(\d+)`, // Actix, Warp
		`serving on [^:]+:(\d+)`,   // Trunk

		// Go patterns
		`gin running on [^:]+:(\d+)`,           // Gin
		`listening and serving on [^:]+:(\d+)`, // Go HTTP servers

		// Python patterns
		`running on [^:]+:(\d+)`,            // Flask
		`development server at [^:]+:(\d+)`, // Django
		`uvicorn running on [^:]+:(\d+)`,    // Uvicorn
		`application startup complete`,      // FastAPI (followed by address)

		// Database patterns
		`listening on port (\d+)`,                   // PostgreSQL, MySQL
		`server is ready on port (\d+)`,             // MongoDB
		`ready to accept connections on port (\d+)`, // Redis

		// Development tools
		`proxy server listening on [^:]+:(\d+)`, // Browser Sync
		`live reload enabled on port (\d+)`,     // Live Server
		`forwarding [^:]+:(\d+)`,                // ngrok

		// Container patterns
		`exposed on.*:(\d+)`, // Docker
		`mapped to.*:(\d+)`,  // Docker port mapping

		// Generic patterns (should be last to avoid false positives)
		`\*:(\d+)`,             // Wildcard binding
		`bound to [^:]*:(\d+)`, // Generic binding message
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile("(?i)" + pattern) // Case insensitive
		if matches := re.FindStringSubmatch(output); len(matches) > 1 {
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

func createDefaultProcessManager() *process.ProcessManager {
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

// checkForAdoptableProcess checks if there's an existing process on the given port that could be adopted
func checkForAdoptableProcess(port int) *process.AdoptionInfo {
	// Create a process adopter to check for adoptable processes
	adopter := process.NewProcessAdopter(5 * time.Second)

	// Get the PID of the process using this port
	pid := getProcessByPort(port)
	if pid <= 0 {
		return nil // No process found on this port
	}

	// Try to get process info for the PID
	if adoptionInfo, err := adopter.GetProcessInfo(pid); err == nil {
		return adoptionInfo
	}

	return nil
}

// getProcessByPort gets the PID of the process using the specified port
func getProcessByPort(port int) int {
	scanner := portscanner.NewScanner(2 * time.Second)
	if portInfo, err := scanner.GetPortInfo(port); err == nil && portInfo.PID > 0 {
		return portInfo.PID
	}
	return 0
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
