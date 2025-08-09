#!/bin/bash
#
# Claude Code PreToolUse Hook - Portguard Process Interception
#
# This hook is called before Claude Code executes any tool. It analyzes
# the incoming command to detect potential server startups and prevent
# duplicate processes or port conflicts.
#
# Environment Variables Available:
# - CLAUDE_TOOL_NAME: Name of the tool being called (e.g., "Bash")
# - CLAUDE_TOOL_PARAMS: JSON parameters passed to the tool
# - CLAUDE_SESSION_ID: Current session ID
# - CLAUDE_WORKING_DIR: Current working directory
#
# Expected JSON Input (stdin):
# {
#   "tool_name": "Bash",
#   "parameters": {
#     "command": "npm run dev --port 3000",
#     "description": "Start development server"
#   },
#   "session_id": "session_abc123",
#   "working_dir": "/path/to/project"
# }
#
# Expected JSON Output (stdout):
# {
#   "action": "allow|block|modify",
#   "reason": "Explanation of the decision",
#   "modified_command": "Alternative command if action=modify",
#   "process_info": {},
#   "suggestions": []
# }

set -euo pipefail

# Default portguard binary location - can be overridden with PORTGUARD_BIN
PORTGUARD_BIN="${PORTGUARD_BIN:-portguard}"

# Function to log debug information (to stderr to avoid polluting JSON output)
debug() {
    if [[ "${PORTGUARD_DEBUG:-}" == "1" ]]; then
        echo "[DEBUG] $*" >&2
    fi
}

# Function to handle errors and exit gracefully
handle_error() {
    local error_msg="$1"
    debug "Error: $error_msg"
    
    # Return JSON that allows the command (fail-open for safety)
    cat << EOF
{
  "action": "allow",
  "reason": "Portguard hook encountered an error: $error_msg",
  "process_info": {
    "hook_error": true,
    "error_message": "$error_msg"
  },
  "suggestions": ["Check portguard installation and configuration"]
}
EOF
    exit 0
}

# Main processing function
main() {
    debug "PreToolUse hook starting"
    
    # Read JSON input from stdin
    local json_input
    json_input=$(cat)
    
    debug "Received input: $json_input"
    
    # Check if portguard is available
    if ! command -v "$PORTGUARD_BIN" >/dev/null 2>&1; then
        handle_error "portguard binary not found in PATH"
        return
    fi
    
    # Extract tool name and command from JSON input
    local tool_name
    tool_name=$(echo "$json_input" | jq -r '.tool_name // "unknown"')
    
    debug "Tool name: $tool_name"
    
    # Only intercept Bash commands (where server processes are typically started)
    if [[ "$tool_name" != "Bash" ]]; then
        debug "Non-Bash tool, allowing through"
        cat << EOF
{
  "action": "allow",
  "reason": "Tool '$tool_name' is not a command execution tool"
}
EOF
        return
    fi
    
    # Extract the actual bash command
    local bash_command
    bash_command=$(echo "$json_input" | jq -r '.parameters.command // ""')
    
    if [[ -z "$bash_command" ]]; then
        debug "No command found, allowing through"
        cat << EOF
{
  "action": "allow",
  "reason": "No command to analyze"
}
EOF
        return
    fi
    
    debug "Analyzing command: $bash_command"
    
    # Create intercept request JSON
    local intercept_request
    intercept_request=$(cat << EOF
{
  "command": "$bash_command",
  "tool_name": "$tool_name",
  "session_id": "${CLAUDE_SESSION_ID:-unknown}",
  "working_dir": "${CLAUDE_WORKING_DIR:-unknown}",
  "context": {
    "hook": "pretooluse",
    "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  }
}
EOF
    )
    
    debug "Intercept request: $intercept_request"
    
    # Call portguard intercept and capture output
    local intercept_response
    if ! intercept_response=$(echo "$intercept_request" | "$PORTGUARD_BIN" intercept 2>/dev/null); then
        handle_error "portguard intercept command failed"
        return
    fi
    
    debug "Intercept response: $intercept_response"
    
    # Output the response from portguard
    echo "$intercept_response"
}

# Check if jq is available (required for JSON parsing)
if ! command -v jq >/dev/null 2>&1; then
    handle_error "jq is required but not installed"
    exit 1
fi

# Run main function
main