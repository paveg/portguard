#!/bin/bash
#
# Claude Code PostToolUse Hook - Official Format Compatible
#
# This hook follows the official Claude Code hooks specification.
#
# Expected Input Format:
# {
#   "event": "postToolUse",
#   "tool_name": "Bash",
#   "parameters": {
#     "command": "npm run dev"
#   },
#   "result": {
#     "success": true,
#     "output": "Server started on port 3000",
#     "exit_code": 0
#   },
#   "session_id": "abc123",
#   "working_dir": "/path/to/project"
# }
#
# Expected Output Format:
# {
#   "status": "success|warning|error",
#   "message": "Description",
#   "data": {}
# }

set -euo pipefail

PORTGUARD_BIN="${PORTGUARD_BIN:-portguard}"

# Function to output JSON response
output_json() {
    local status="$1"
    local message="$2"
    local data="${3:-{}}"
    
    cat << EOF
{
  "status": "$status",
  "message": "$message",
  "data": $data
}
EOF
}

# Function to handle errors gracefully
handle_error() {
    output_json "warning" "Hook warning: $1" "{\"error\": true}"
    exit 0
}

# Check dependencies
if ! command -v jq >/dev/null 2>&1; then
    handle_error "jq is required but not installed"
fi

if ! command -v "$PORTGUARD_BIN" >/dev/null 2>&1; then
    handle_error "portguard not found"
fi

# Read and parse JSON input
json_input=$(cat)

if [ -z "$json_input" ]; then
    output_json "success" "No input provided" "{}"
    exit 0
fi

# Extract event type
event=$(echo "$json_input" | jq -r '.event // "unknown"')

if [ "$event" != "postToolUse" ]; then
    output_json "success" "Not a postToolUse event" "{}"
    exit 0
fi

# Call portguard intercept with the full request
response=$(echo "$json_input" | "$PORTGUARD_BIN" intercept 2>/dev/null || echo '{"status": "success", "message": "Processing complete"}')

# Output the response directly
echo "$response"