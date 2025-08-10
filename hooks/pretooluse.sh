#!/bin/bash
#
# Claude Code PreToolUse Hook - Official Format Compatible
#
# This hook follows the official Claude Code hooks specification.
# It uses the correct JSON request/response format as documented.
#
# Expected Input Format:
# {
#   "event": "preToolUse",
#   "tool_name": "Bash",
#   "parameters": {
#     "command": "npm run dev"
#   },
#   "session_id": "abc123",
#   "working_dir": "/path/to/project"
# }
#
# Expected Output Format:
# {
#   "proceed": true|false,
#   "message": "Explanation",
#   "data": {}
# }

set -euo pipefail

PORTGUARD_BIN="${PORTGUARD_BIN:-portguard}"

# Function to output JSON response
output_json() {
    local proceed="$1"
    local message="$2"
    local data="${3:-{}}"
    
    cat << EOF
{
  "proceed": $proceed,
  "message": "$message",
  "data": $data
}
EOF
}

# Function to handle errors gracefully (fail open)
handle_error() {
    output_json "true" "Hook error: $1" "{\"error\": true}"
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
    output_json "true" "No input provided" "{}"
    exit 0
fi

# Extract event type (handle invalid JSON gracefully)
event=$(echo "$json_input" | jq -r '.event // "unknown"' 2>/dev/null || echo "unknown")

if [ "$event" != "preToolUse" ]; then
    echo '{"proceed": true, "message": "Not a preToolUse event", "data": {}}'
    exit 0
fi

# Extract tool name
tool_name=$(echo "$json_input" | jq -r '.tool_name // ""' 2>/dev/null || echo "")

if [ "$tool_name" != "Bash" ]; then
    echo '{"proceed": true, "message": "Non-Bash tool", "data": {}}'
    exit 0
fi

# Extract command
command=$(echo "$json_input" | jq -r '.parameters.command // ""' 2>/dev/null || echo "")

if [ -z "$command" ]; then
    echo '{"proceed": true, "message": "No command found", "data": {}}'
    exit 0
fi

# Call portguard intercept with the full request
response=$(echo "$json_input" | "$PORTGUARD_BIN" intercept 2>/dev/null || echo '{"proceed": true, "message": "Intercept failed"}')

# Output the response directly
echo "$response"