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

## Coding Standards and Guidelines

### Error Handling Requirements

**ALWAYS check errors**: Never ignore error return values

```go
// ❌ Bad
os.Remove(tempFile)

// ✅ Good  
if err := os.Remove(tempFile); err != nil {
    log.Printf("failed to remove temp file: %v", err)
}

// ✅ Acceptable for cleanup (with explicit ignore)
_ = os.Remove(tempFile) // Cleanup - errors are not critical
```

**Use errors.As for type assertions on errors**:

```go
// ❌ Bad
if configErr, ok := err.(*viper.ConfigFileNotFoundError); ok {

// ✅ Good
var configErr viper.ConfigFileNotFoundError
if errors.As(err, &configErr) {
```

**Wrap external package errors**:

```go
// ❌ Bad
return err

// ✅ Good
return fmt.Errorf("failed to parse config: %w", err)
```

### Test Code Standards

**Import restrictions**: Test files should not import production dependencies unnecessarily

- Avoid importing `cobra` and `viper` in tests unless testing those specific integrations
- Use dependency injection and mocks instead

**Use proper assertion methods**:

```go
// ❌ Bad
assert.Equal(t, "", value)           // Use assert.Empty
assert.Equal(t, nil, err)            // Use assert.NoError
assert.NotNil(t, err)               // Use assert.Error

// ✅ Good  
assert.Empty(t, value)
assert.NoError(t, err)
assert.Error(t, err)
assert.ErrorIs(t, err, expectedErr)
```

**Use require for critical assertions**:

```go
// ❌ Bad - test continues if this fails
assert.NoError(t, err)
result := processResult() // might panic if err != nil

// ✅ Good - test stops if this fails
require.NoError(t, err)
result := processResult()
```

**Use context with exec.Command**:

```go
// ❌ Bad
cmd := exec.Command("git", "status")

// ✅ Good
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
cmd := exec.CommandContext(ctx, "git", "status")
```

**Avoid variable shadowing**:

```go
// ❌ Bad
func TestSomething(t *testing.T) {
    port := 3000
    for _, tt := range tests {
        port, err := strconv.Atoi(tt.port) // shadows outer port
    }
}

// ✅ Good
func TestSomething(t *testing.T) {
    defaultPort := 3000
    for _, tt := range tests {
        port, err := strconv.Atoi(tt.port)
    }
}
```

### Variable Naming

**Use descriptive names for broader scopes**:

```go
// ❌ Bad - too short for scope
func processData() {
    r, w, _ := os.Pipe() // used for 20+ lines
}

// ✅ Good
func processData() {
    reader, writer, _ := os.Pipe()
}
```

### Duration Handling

**Use Go standard durations only**:

```go
// ❌ Bad - "d" is not a standard Go duration unit
BackupRetention: "7d"

// ✅ Good - use hours for days
BackupRetention: "168h" // 7 days = 7 * 24 hours
```

### Configuration Standards  

**Use appropriate types**:

```go
// ❌ Bad - magic numbers without constants
if timeout < 30 {

// ✅ Good - define constants
const DefaultTimeout = 30 * time.Second
if timeout < DefaultTimeout {
```

**Provide sensible defaults**:

```go
// ✅ All config fields should have reasonable defaults
viper.SetDefault("default.health_check.timeout", "30s")
viper.SetDefault("default.cleanup.max_idle_time", "1h")
```

### JSON API Standards

**Use consistent response structures**:

```go
type APIResponse struct {
    Success bool        `json:"success"`
    Message string      `json:"message,omitempty"`
    Data    interface{} `json:"data,omitempty"`
    Error   string      `json:"error,omitempty"`
}
```

**Use snake_case for JSON fields**:

```go
type Config struct {
    MaxIdleTime time.Duration `json:"max_idle_time"`
    AutoCleanup bool          `json:"auto_cleanup"`
}
```

### Security Requirements

**Never log or commit sensitive information**:

- API keys, passwords, tokens should use environment variables
- Sanitize user input in commands and file paths
- Use secure file permissions (0600 for sensitive files, 0755 for directories)

**Safe command execution**:

```go
// Validate and sanitize command arguments
// Use absolute paths when possible
// Set timeouts for external commands
```

### Performance Guidelines

**Pre-allocate slices when size is known**:

```go
// ❌ Bad
var items []string
for range data {
    items = append(items, process(item))
}

// ✅ Good  
items := make([]string, 0, len(data))
for range data {
    items = append(items, process(item))
}
```

## Development Workflow Requirements

### Branch Management

**ALWAYS create a new branch before starting work**:

```bash
# ✅ Required workflow
git switch main
git pull origin main
git switch -c feature/your-feature-name

# Work on changes...

# Push feature branch and create PR
git push -u origin feature/your-feature-name
gh pr create --title "feat: your feature description" --body "..."
```

**NEVER push directly to main branch**:

- ❌ **Prohibited**: `git push origin main`
- ✅ **Required**: Always use pull requests for code review
- All changes must go through the PR process
- Main branch is protected and should only receive merged PRs

### Commit Guidelines

**Make commits in logical, atomic units**:

```bash
# ✅ Good - each commit represents one logical change
git add internal/cmd/health.go
git commit -m "feat: implement HTTP health checks"

git add internal/cmd/health.go internal/process/manager.go  
git commit -m "feat: add TCP health check support"

git add internal/cmd/ports.go
git commit -m "feat: implement port scanning functionality"
```

**Commit message format**:

```
type(scope): description

feat: new feature
fix: bug fix
refactor: code restructuring
docs: documentation changes
test: test additions/changes
ci: CI/CD changes
```

### Release Management

#### Proper Release Process (CRITICAL - DO NOT SKIP STEPS)

**⚠️ WARNING**: Improper release process will cause Go module checksum errors and installation failures!

##### Step 1: Pre-release Preparation

1. **Version is now managed automatically from Git tags**:
   ```bash
   # No manual version file updates needed!
   # Version is automatically derived from git tags via Makefile
   
   # Verify current version detection
   make build
   ./bin/portguard --version  # Shows current git tag
   ```

2. **Update CHANGELOG.md**:
   ```bash
   vim CHANGELOG.md
   # Add new version section with changes
   ```

3. **Run all checks**:
   ```bash
   make test          # All tests must pass
   make lint          # Zero linting issues
   make build         # Build successfully
   ./bin/portguard --version  # Verify correct version displays
   ```

##### Step 2: Commit and Tag (DO THIS CORRECTLY!)

```bash
# 1. Commit only CHANGELOG.md (no version files needed!)
git add CHANGELOG.md
git commit -m "release: prepare vX.Y.Z"

# 2. Create PR to main branch (required due to branch protection)
git push origin feature/release-vX.Y.Z
gh pr create --title "Release vX.Y.Z" --body "..."

# 3. After PR is merged, tag from main branch
git checkout main
git pull origin main
git tag vX.Y.Z
git push origin vX.Y.Z

# 4. Create GitHub release
gh release create vX.Y.Z --title "vX.Y.Z" --notes "..."
```

##### Step 3: Verify Release

```bash
# Wait 1-2 minutes for Go proxy to update
sleep 120

# Test installation (should work WITHOUT any environment variables)
go install github.com/paveg/portguard/cmd/portguard@vX.Y.Z
$(go env GOPATH)/bin/portguard --version  # Should show vX.Y.Z
```

#### Common Mistakes to Avoid

❌ **NEVER do these**:
- Don't delete and recreate tags (causes checksum mismatches)
- Don't create release from unmerged branches  
- Don't skip the PR process for main branch

✅ **ALWAYS do these**:
- Create tags only once (never delete/recreate)
- Test the build locally before releasing
- Use PRs for main branch updates
- Wait for CI checks to pass before releasing

#### Emergency: Fixing a Broken Release

If a release is broken (wrong version displayed, checksum errors, etc.):

1. **Create a NEW version** (never reuse version numbers):
   ```bash
   # If vX.Y.Z is broken, create vX.Y.(Z+1)
   # No version files to update - just follow proper release process from Step 1
   ```

2. **Document the issue**:
   ```bash
   # In CHANGELOG.md, note why the new version was needed
   ## [X.Y.Z+1] - Date
   ### Fixed
   - Fixed version mismatch from vX.Y.Z release
   ```

## Mandatory Pre-Commit Checks

Before any code changes:

1. **Run tests**: `make test` must pass completely
2. **Run linter**: `make lint` must show 0 issues  
3. **Check coverage**: Maintain >70% test coverage
4. **Verify integration**: `./hooks/test_hooks.sh` must pass

When working on this codebase, focus on the ProcessManager as the central coordination point, and remember that all Claude Code integration must use the official hooks format with proper JSON request/response structures.
