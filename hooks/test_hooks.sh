#!/bin/bash
#
# Test script for Portguard Claude Code hooks
#
# This script tests both PreToolUse and PostToolUse hooks using the official
# Claude Code hooks format to ensure they work correctly.

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Hook script paths
PRETOOLUSE_HOOK="$SCRIPT_DIR/pretooluse.sh"
POSTTOOLUSE_HOOK="$SCRIPT_DIR/posttooluse.sh"

# Test counter
TESTS_PASSED=0
TESTS_FAILED=0

# Utility functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $*"
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} $*"
    ((TESTS_PASSED++)) || true
}

log_error() {
    echo -e "${RED}[FAIL]${NC} $*"
    ((TESTS_FAILED++)) || true
}

log_warning() {
    echo -e "${YELLOW}[WARN]${NC} $*"
}

# Test function for PreToolUse hook
run_pretooluse_test() {
    local test_name="$1"
    local hook_script="$2"
    local input_json="$3"
    local expected_proceed="${4:-true}"
    
    log_info "Running test: $test_name"
    
    # Check if hook script exists and is executable
    if [[ ! -x "$hook_script" ]]; then
        log_error "Hook script not found or not executable: $hook_script"
        return 1
    fi
    
    # Run the hook with test input
    local output
    if output=$(echo "$input_json" | "$hook_script" 2>/dev/null); then
        if [[ -n "$expected_proceed" ]]; then
            # Check if output contains expected proceed value
            local actual_proceed
            actual_proceed=$(echo "$output" | jq -r '.proceed // "null"' 2>/dev/null)
            
            if [[ "$actual_proceed" == "$expected_proceed" ]]; then
                log_success "$test_name - Proceed: $expected_proceed"
            else
                log_error "$test_name - Expected proceed: $expected_proceed, got: $actual_proceed"
            fi
        else
            # Just check if it runs without error
            log_success "$test_name - Executed without error"
        fi
        
        # Pretty print the output for inspection
        # echo "$output" | jq . 2>/dev/null || echo "$output"
        echo
    else
        log_error "$test_name - Hook execution failed"
        return 1
    fi
}

# Test function for PostToolUse hook
run_posttooluse_test() {
    local test_name="$1"
    local hook_script="$2"
    local input_json="$3"
    local expected_status="${4:-success}"
    
    log_info "Running test: $test_name"
    
    # Check if hook script exists and is executable
    if [[ ! -x "$hook_script" ]]; then
        log_error "Hook script not found or not executable: $hook_script"
        return 1
    fi
    
    # Run the hook with test input
    local output
    if output=$(echo "$input_json" | "$hook_script" 2>/dev/null); then
        if [[ -n "$expected_status" ]]; then
            # Check if output contains expected status
            local actual_status
            actual_status=$(echo "$output" | jq -r '.status // "null"' 2>/dev/null)
            
            if [[ "$actual_status" == "$expected_status" ]]; then
                log_success "$test_name - Status: $expected_status"
            else
                log_error "$test_name - Expected status: $expected_status, got: $actual_status"
            fi
        else
            # Just check if it runs without error
            log_success "$test_name - Executed without error"
        fi
        
        # Pretty print the output for inspection
        # echo "$output" | jq . 2>/dev/null || echo "$output"
        echo
    else
        log_error "$test_name - Hook execution failed"
        return 1
    fi
}

# Main test runner
main() {
    echo "=================================================="
    echo "Portguard Claude Code Hooks Test Suite"
    echo "=================================================="
    echo
    
    # Check prerequisites
    log_info "Checking prerequisites..."
    
    if ! command -v jq >/dev/null 2>&1; then
        log_error "jq is required but not installed"
        exit 1
    fi
    
    if ! command -v portguard >/dev/null 2>&1; then
        log_warning "portguard binary not found in PATH - some tests may fail"
    fi
    
    echo
    
    # PreToolUse Hook Tests
    log_info "Testing PreToolUse Hook..."
    echo "----------------------------------------"
    
    # Test 1: Server command detection (should allow)
    run_pretooluse_test "Server Command Detection" "$PRETOOLUSE_HOOK" '{
        "event": "preToolUse",
        "tool_name": "Bash",
        "parameters": {
            "command": "npm run dev --port 3000"
        },
        "session_id": "test123",
        "working_dir": "/tmp/test"
    }' "true"
    
    # Test 2: Non-server command (should allow)
    run_pretooluse_test "Non-Server Command" "$PRETOOLUSE_HOOK" '{
        "event": "preToolUse",
        "tool_name": "Bash", 
        "parameters": {
            "command": "ls -la"
        },
        "session_id": "test123",
        "working_dir": "/tmp/test"
    }' "true"
    
    # Test 3: Non-Bash tool (should allow)
    run_pretooluse_test "Non-Bash Tool" "$PRETOOLUSE_HOOK" '{
        "event": "preToolUse",
        "tool_name": "Read",
        "parameters": {
            "file_path": "/tmp/test.txt"
        },
        "session_id": "test123",
        "working_dir": "/tmp/test"
    }' "true"
    
    # Test 4: Different server commands
    local server_commands=(
        "yarn dev"
        "next dev"
        "vite"
        "flask run"
        "go run main.go"
        "python app.py --port 5000"
        "rails server -p 3000"
    )
    
    for cmd in "${server_commands[@]}"; do
        run_pretooluse_test "Server Command: $cmd" "$PRETOOLUSE_HOOK" "{
            \"event\": \"preToolUse\",
            \"tool_name\": \"Bash\",
            \"parameters\": {
                \"command\": \"$cmd\"
            },
            \"session_id\": \"test123\",
            \"working_dir\": \"/tmp/test\"
        }" "true"
    done
    
    echo
    
    # PostToolUse Hook Tests
    log_info "Testing PostToolUse Hook..."
    echo "----------------------------------------"
    
    # Test 1: Successful server startup
    run_posttooluse_test "Successful Server Startup" "$POSTTOOLUSE_HOOK" '{
        "event": "postToolUse",
        "tool_name": "Bash",
        "parameters": {
            "command": "npm run dev"
        },
        "result": {
            "success": true,
            "output": "Server listening on port 3000\nWebpack compiled successfully",
            "exit_code": 0
        },
        "session_id": "test123",
        "working_dir": "/tmp/test"
    }' "success"
    
    # Test 2: Failed command
    run_posttooluse_test "Failed Command" "$POSTTOOLUSE_HOOK" '{
        "event": "postToolUse",
        "tool_name": "Bash",
        "parameters": {
            "command": "npm run dev"
        },
        "result": {
            "success": false,
            "output": "Error: Command failed",
            "exit_code": 1
        },
        "session_id": "test123",
        "working_dir": "/tmp/test"
    }' "error"
    
    # Test 3: Non-server command success
    run_posttooluse_test "Non-Server Command Success" "$POSTTOOLUSE_HOOK" '{
        "event": "postToolUse",
        "tool_name": "Bash",
        "parameters": {
            "command": "echo hello world"
        },
        "result": {
            "success": true,
            "output": "hello world",
            "exit_code": 0
        },
        "session_id": "test123",
        "working_dir": "/tmp/test"
    }' "success"
    
    # Test 4: Different server startup outputs
    local server_outputs=(
        "Server running on http://localhost:3000"
        "🚀 Server ready at http://localhost:8080"
        "Flask application running on port 5000"
        "Listening on 0.0.0.0:4000"
    )
    
    for output in "${server_outputs[@]}"; do
        run_posttooluse_test "Server Output: ${output:0:30}..." "$POSTTOOLUSE_HOOK" "{
            \"event\": \"postToolUse\",
            \"tool_name\": \"Bash\",
            \"parameters\": {
                \"command\": \"npm run dev\"
            },
            \"result\": {
                \"success\": true,
                \"output\": \"$output\",
                \"exit_code\": 0
            },
            \"session_id\": \"test123\",
            \"working_dir\": \"/tmp/test\"
        }" "success"
    done
    
    echo
    
    # Error handling tests
    log_info "Testing Error Handling..."
    echo "----------------------------------------"
    
    # Test invalid JSON for PreToolUse
    if echo "invalid json" | "$PRETOOLUSE_HOOK" >/dev/null 2>&1; then
        log_success "PreToolUse handles invalid JSON gracefully"
    else
        log_error "PreToolUse does not handle invalid JSON gracefully"
    fi
    
    # Test empty input for PostToolUse  
    if echo "" | "$POSTTOOLUSE_HOOK" >/dev/null 2>&1; then
        log_success "PostToolUse handles empty input gracefully"
    else
        log_error "PostToolUse does not handle empty input gracefully"
    fi
    
    # Test malformed event type
    run_pretooluse_test "Invalid Event Type" "$PRETOOLUSE_HOOK" '{
        "event": "invalidEvent",
        "tool_name": "Bash",
        "parameters": {
            "command": "npm run dev"
        }
    }' "true"
    
    echo
    
    # Summary
    echo "=================================================="
    echo "Test Summary"
    echo "=================================================="
    echo -e "Tests Passed: ${GREEN}$TESTS_PASSED${NC}"
    echo -e "Tests Failed: ${RED}$TESTS_FAILED${NC}"
    echo -e "Total Tests:  $((TESTS_PASSED + TESTS_FAILED))"
    
    if [[ $TESTS_FAILED -eq 0 ]]; then
        echo -e "${GREEN}All tests passed! 🎉${NC}"
        exit 0
    else
        echo -e "${RED}Some tests failed. Please check the output above.${NC}"
        exit 1
    fi
}

# Run the tests
main "$@"