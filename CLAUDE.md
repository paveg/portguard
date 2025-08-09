# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Portguard is an AI-aware process management tool that prevents duplicate server startups when using AI development tools. It integrates with Claude Code through hooks to intercept Bash commands, detect server startup attempts, and prevent port conflicts.

## Essential Development Commands

```bash
# Build and run
make build                    # Build binary to build/portguard
make run                      # Run in development mode
go run cmd/portguard/main.go  # Direct run

# Testing
make test                     # Run all tests
make test-coverage           # Generate coverage reports
make test-race              # Run with race detection
go test ./internal/process   # Test specific package

# Code quality
make lint                    # Run golangci-lint (comprehensive config in .golangci.yml)
make fmt                     # Format code

# Claude Code hooks testing
./hooks/test_hooks.sh        # Test hook integration
PORTGUARD_BIN=./portguard ./hooks/test_hooks.sh  # Test with local binary

# Demos and integration testing
make demo                    # Run demo commands
make ai-test                # Test AI integration features
```

## Architecture Overview

### Core Components

**ProcessManager** (`internal/process/manager.go`)

- Central orchestrator for process lifecycle management
- Implements duplicate detection via `ShouldStartNew()`
- Integrates StateStore, LockManager, and PortScanner
- Thread-safe operations with mutex-protected state

**Claude Code Integration** (`internal/cmd/intercept.go` + `hooks/`)

- `intercept` command: Processes hook requests using official Claude Code format
- PreToolUse hook: Analyzes commands before execution, returns `proceed: true/false`
- PostToolUse hook: Registers successful server startups, returns status info
- Supports event-based JSON communication: `{"event": "preToolUse|postToolUse", ...}`

**State Management**

- `StateStore` (`internal/state/`): JSON persistence for process state
- `LockManager` (`internal/lock/`): File-based locking prevents concurrent access conflicts
- `PortScanner` (`internal/port/`): Cross-platform port availability detection

### CLI Command Flow

1. **Root Command** (`internal/cmd/root.go`): Cobra-based CLI with Viper config
2. **Command Handlers** (`internal/cmd/*.go`): Individual command implementations
3. **Process Operations**: All commands route through ProcessManager for consistency

### Hook Integration Flow

```
Claude Code Bash Command
    ↓ (PreToolUse hook)
pretooluse.sh → portguard intercept
    ↓ (Command analysis + conflict detection)
JSON Response: {proceed: true/false, message: "...", data: {...}}
    ↓ (If allowed, command executes)
Command Output → posttooluse.sh → portguard intercept
    ↓ (Server detection + registration)
JSON Response: {status: "success", message: "...", data: {...}}
```

## Key Implementation Details

**Server Command Detection** (`internal/cmd/intercept.go`):

- Pattern matching for: `npm run dev`, `go run main.go`, `flask run`, etc.
- Port extraction from flags (`--port`, `-p`) or default framework ports
- Output parsing for server startup messages

**Process Conflict Resolution**:

- Command-based matching: exact string comparison
- Port-based matching: detect conflicts on same port
- Health checks: only reuse healthy existing processes
- Fail-open safety: allow commands if error occurs

**State Persistence**:

- JSON format in `~/.portguard/state.json`
- File locking prevents concurrent access
- Atomic updates with proper error handling

## Testing Patterns

- Use `testify` for assertions and mocks
- Test files: `*_test.go` (relaxed linting rules applied)
- Integration tests: `hooks/test_hooks.sh` for end-to-end hook testing
- Mock interfaces defined in `internal/process/types.go`

## Configuration

- YAML configuration support via Viper
- Environment variable overrides (e.g., `PORTGUARD_BIN` for hooks)
- Claude Code settings integration in `examples/claude-code-settings.json`

When working on this codebase, focus on the ProcessManager as the central coordination point, and remember that all Claude Code integration must use the official hooks format with proper JSON request/response structures.
