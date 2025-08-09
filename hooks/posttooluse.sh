#!/bin/bash
#
# Claude Code PostToolUse Hook - Portguard Process Registration
#
# This hook is called after Claude Code executes any tool. It analyzes
# the completed command to register new server processes and update
# the process state if a server was successfully started.
#
# Environment Variables Available:
# - CLAUDE_TOOL_NAME: Name of the tool that was called
# - CLAUDE_TOOL_PARAMS: JSON parameters passed to the tool
# - CLAUDE_TOOL_RESULT: Result from the tool execution
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
#   "result": {
#     "success": true,
#     "output": "Server started on port 3000...",
#     "exit_code": 0
#   },
#   "session_id": "session_abc123",
#   "working_dir": "/path/to/project"
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
    
    # For PostToolUse, we don't output JSON - just log and exit
    echo "Warning: Portguard PostToolUse hook error: $error_msg" >&2
    exit 0
}

# Function to extract port from command output
extract_port_from_output() {
    local output="$1"
    
    # Common patterns for server startup messages
    local port_patterns=(
        "listening on port ([0-9]+)"
        "server.*running.*:([0-9]+)"
        "started.*port ([0-9]+)"
        "listening.*:([0-9]+)"
        "http://[^:]+:([0-9]+)"
        "localhost:([0-9]+)"
        "0\.0\.0\.0:([0-9]+)"
        "127\.0\.0\.1:([0-9]+)"
    )
    
    for pattern in "${port_patterns[@]}"; do
        if [[ "$output" =~ $pattern ]]; then
            echo "${BASH_REMATCH[1]}"
            return 0
        fi
    done
    
    return 1
}

# Function to check if command appears to have started a server successfully
is_server_startup_successful() {
    local command="$1"
    local output="$2"
    local exit_code="$3"
    
    # Must have successful exit code
    if [[ "$exit_code" != "0" ]]; then
        return 1
    fi
    
    # Check for server startup indicators in output
    local success_indicators=(
        "listening"
        "started"
        "ready"
        "server.*running"
        "development server"
        "webpack.*compiled"
        "vite.*ready"
        "next.*ready"
    )
    
    local output_lower
    output_lower=$(echo "$output" | tr '[:upper:]' '[:lower:]')
    
    for indicator in "${success_indicators[@]}"; do
        if [[ "$output_lower" =~ $indicator ]]; then
            debug "Found server startup indicator: $indicator"
            return 0
        fi
    done
    
    return 1
}

# Main processing function
main() {
    debug "PostToolUse hook starting"
    
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
    
    # Only process Bash commands
    if [[ "$tool_name" != "Bash" ]]; then
        debug "Non-Bash tool, skipping"
        return
    fi
    
    # Extract the bash command
    local bash_command
    bash_command=$(echo "$json_input" | jq -r '.parameters.command // ""')
    
    if [[ -z "$bash_command" ]]; then
        debug "No command found, skipping"
        return
    fi
    
    # Extract result information
    local success
    success=$(echo "$json_input" | jq -r '.result.success // false')
    
    local output
    output=$(echo "$json_input" | jq -r '.result.output // ""')
    
    local exit_code
    exit_code=$(echo "$json_input" | jq -r '.result.exit_code // -1')
    
    debug "Command: $bash_command"
    debug "Success: $success"
    debug "Exit code: $exit_code"
    debug "Output length: ${#output}"
    
    # Check if this appears to be a successful server startup
    if ! is_server_startup_successful "$bash_command" "$output" "$exit_code"; then
        debug "Command does not appear to be a successful server startup"
        return
    fi
    
    debug "Detected successful server startup, registering process"
    
    # Extract port from output or command
    local detected_port
    if detected_port=$(extract_port_from_output "$output"); then
        debug "Extracted port from output: $detected_port"
    else
        # Fallback to extracting from command (reuse logic from intercept)
        if [[ "$bash_command" =~ --port[[:space:]]*([0-9]+) ]]; then
            detected_port="${BASH_REMATCH[1]}"
            debug "Extracted port from command: $detected_port"
        elif [[ "$bash_command" =~ -p[[:space:]]*([0-9]+) ]]; then
            detected_port="${BASH_REMATCH[1]}"
            debug "Extracted port from command (-p): $detected_port"
        else
            # Use common defaults based on command type
            if [[ "$bash_command" =~ npm.*run.*dev|next.*dev ]]; then
                detected_port="3000"
                debug "Using Next.js default port: $detected_port"
            elif [[ "$bash_command" =~ vite ]]; then
                detected_port="5173"
                debug "Using Vite default port: $detected_port"
            elif [[ "$bash_command" =~ flask.*run ]]; then
                detected_port="5000"
                debug "Using Flask default port: $detected_port"
            else
                debug "No port detected, skipping registration"
                return
            fi
        fi
    fi
    
    # Register the process with portguard
    debug "Registering process with port $detected_port"
    
    # Create a simplified start command to register the process
    # We use a background process to avoid blocking the hook
    (
        # Use timeout to prevent hanging
        timeout 10s "$PORTGUARD_BIN" start \
            --command "$bash_command" \
            --port "$detected_port" \
            --working-dir "${CLAUDE_WORKING_DIR:-$(pwd)}" \
            --background \
            --json >/dev/null 2>&1 || true
    ) &
    
    debug "Process registration initiated in background"
}

# Check if jq is available (required for JSON parsing)
if ! command -v jq >/dev/null 2>&1; then
    handle_error "jq is required but not installed"
    exit 1
fi

# Run main function
main