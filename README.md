# Portguard

[![CI](https://github.com/paveg/portguard/workflows/CI/badge.svg)](https://github.com/paveg/portguard/actions)
[![codecov](https://codecov.io/gh/paveg/portguard/graph/badge.svg?token=20P31OII5Q)](https://codecov.io/gh/paveg/portguard)
[![Go Report Card](https://goreportcard.com/badge/github.com/paveg/portguard)](https://goreportcard.com/report/github.com/paveg/portguard)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

AI-aware process management tool designed to prevent duplicate server startups when using AI development tools like Claude Code, GitHub Copilot, and Cursor.

## Problem

AI development tools often start servers without checking if they're already running, leading to:

- Port conflicts and errors
- Resource waste (multiple identical processes)
- Development environment confusion
- Debugging difficulties

## Solution

Portguard provides:

- **Intelligent Duplicate Detection**: Automatically detects if the same command is already running
- **Port Management**: Prevents port conflicts and suggests alternatives
- **Process Reuse**: Reuses healthy existing processes instead of starting duplicates
- **Health Monitoring**: Monitors process health and provides status information
- **AI-Friendly Interface**: JSON output and simple commands for easy AI integration

## Installation

### Option 1: Go Install (Recommended)

```bash
go install github.com/paveg/portguard/cmd/portguard@latest
```

### Option 2: Download Binary

Download the latest binary from the [releases page](https://github.com/paveg/portguard/releases/latest):

```bash
# Example for Linux AMD64
wget https://github.com/paveg/portguard/releases/latest/download/portguard-linux-amd64
chmod +x portguard-linux-amd64
sudo mv portguard-linux-amd64 /usr/local/bin/portguard
```

### Option 3: Build from Source

```bash
git clone https://github.com/paveg/portguard
cd portguard
make build

# Add to PATH
sudo cp bin/portguard /usr/local/bin/
```

## Quick Start

### 🚀 Claude Code Integration (Recommended)

The fastest way to get started is with our one-command Claude Code integration:

```bash
# Install Claude Code hooks (prevents duplicate servers automatically)
portguard hooks install

# That's it! Your AI development workflow is now protected
```

Choose from different templates:

- **basic**: Simple server conflict prevention
- **advanced**: Health monitoring & lifecycle tracking
- **developer**: Full workflow optimization

```bash
portguard hooks install advanced  # For more features
portguard hooks status            # Check installation
portguard hooks list --templates  # See all options
```

### Manual Process Management

```bash
# Start a process (or reuse existing one)
portguard start "go run main.go" --port 3000

# Start using project configuration
portguard start api --config .portguard.yml

# List all managed processes
portguard list

# Check status
portguard status

# Stop a process
portguard stop 3000

# Clean up all processes
portguard clean
```

## Commands

### Core Commands

- `portguard start <command|project>` - Start a new process or reuse existing one
- `portguard stop <id|port>` - Stop a managed process  
- `portguard list` - List all managed processes
- `portguard status [id]` - Show process status and health information
- `portguard clean` - Clean up all managed processes

### Hook Commands

- `portguard hooks install [template]` - Install Claude Code hooks
- `portguard hooks status` - Check hook installation status
- `portguard hooks list` - List available templates and installed hooks
- `portguard hooks update` - Update installed hooks
- `portguard hooks remove` - Remove installed hooks

### Utility Commands

- `portguard ports` - Show port usage information
- `portguard health [id]` - Check health status of processes
- `portguard check` - Quick status check (AI-friendly)
- `portguard config` - Configuration management

### AI-Friendly Commands

```bash
# Quick status check (returns JSON)
portguard check --json

# Check if specific port is available
portguard check --port 3000 --json

# Find next available port
portguard check --available --start 3000 --json
```

## Claude Code Integration

Portguard seamlessly integrates with Claude Code using the official hooks specification:

### How It Works

1. **PreToolUse Hook**: Intercepts Bash commands before execution
   - Detects 40+ server startup commands (`npm run dev`, `pnpm dev`, `air`, `turbo run dev`, etc.)
   - Checks for existing processes on the same port
   - Blocks duplicate servers or suggests alternatives

2. **PostToolUse Hook**: Registers successful server startups
   - Monitors command output for server startup messages
   - Extracts port information from output
   - Registers the process in Portguard for future conflict detection

### Manual Installation

If you prefer manual setup:

1. Copy hook scripts:

```bash
mkdir -p ~/.config/claude-code/hooks
cp hooks/*.sh ~/.config/claude-code/hooks/
chmod +x ~/.config/claude-code/hooks/*.sh
```

2. Update Claude Code settings:

```json
{
  "hooks": [
    {
      "matchers": [
        {"tool": "Bash"}
      ],
      "hooks": [
        {
          "event": "preToolUse",
          "command": "~/.config/claude-code/hooks/pretooluse.sh",
          "timeout": 10000,
          "failureHandling": "allow",
          "environment": {
            "PORTGUARD_BIN": "portguard"
          }
        },
        {
          "event": "postToolUse",
          "command": "~/.config/claude-code/hooks/posttooluse.sh",
          "timeout": 5000,
          "failureHandling": "ignore",
          "environment": {
            "PORTGUARD_BIN": "portguard"
          }
        }
      ]
    }
  ]
}
```

## Configuration

```bash
# Initialize configuration file
portguard config init

# Show current configuration
portguard config show
```

Portguard uses a YAML configuration file (`.portguard.yml`) for project-specific settings:

```yaml
default:
  health_check:
    enabled: true
    timeout: 5s
    interval: 30s
  port_range:
    start: 3000
    end: 9000

projects:
  web:
    command: "npm run dev"
    port: 3000
    health_check:
      type: http
      target: "http://localhost:3000/health"
    environment:
      NODE_ENV: "development"
  
  api:
    command: "go run main.go"
    port: 3001
    working_dir: "./api"
    health_check:
      type: http
      target: "http://localhost:3001/api/health"
  
  # Modern development tools
  monorepo:
    command: "turbo run dev"
    port: 3000
    
  rust-app:
    command: "cargo run"
    port: 8080
    working_dir: "./rust-backend"
```

### Project-Based Commands

You can now start processes using project names defined in your configuration:

```bash
# Start web project (uses npm run dev on port 3000)
portguard start web

# Start API project (uses go run main.go on port 3001)  
portguard start api

# Start with custom config file
portguard start web --config ./custom-config.yml
```

## AI Integration Examples

```bash
# Check if server is already running before starting
if portguard check --port 3000 --json | jq -r '.port_in_use' = "false"; then
    portguard start "npm run dev" --port 3000
else
    echo "Server already running on port 3000"
fi

# Get status as JSON
portguard check --json
# Returns: {"portguard_running": true, "managed_processes": 2, ...}

# List processes as JSON  
portguard list --json
# Returns: [{"id": "abc123", "command": "npm run dev", "port": 3000, ...}]
```

## Features

### Comprehensive Framework Support

Portguard automatically recognizes and handles 40+ development frameworks and tools:

**JavaScript/Node.js**
- `npm run dev`, `npm start`, `yarn dev`
- `pnpm dev`, `pnpm run dev` 
- `turbo run dev`, `turbo dev` (Turbo Repo)
- `nx serve`, `nx dev` (Nx Monorepo)
- `bun run dev`, `bun dev` (Bun)
- `next dev`, `vite`, `gatsby develop`
- `nuxt dev`, `astro dev`

**Go**
- `go run main.go`, `go run ./cmd/...`
- `air` (hot reload)
- `gin` (web framework)

**Python**  
- `flask run`, `uvicorn`, `gunicorn`
- `python manage.py runserver` (Django)
- `fastapi dev`

**Rust**
- `cargo run`, `trunk serve`

**Static Site Generators**
- `hugo server`, `jekyll serve`
- `eleventy --serve`

**Databases**
- `mongodb`, `postgres`, `mysql`, `redis-server`

**Development Tools**
- `docker run -p`, `docker-compose up`
- `serve`, `http-server`, `live-server`

And many more! Each framework includes intelligent port detection and startup message parsing.

### Intelligent Duplicate Detection

- ✅ Command-based process matching
- ✅ Port conflict detection
- ✅ Process health validation
- ✅ Automatic reuse of healthy processes

### Health Monitoring

- ✅ HTTP health checks
- ✅ TCP connectivity checks  
- ✅ Custom command checks
- ✅ Process-based health checks (PID monitoring)
- ✅ Automatic process recovery

### AI-Friendly Design

- ✅ JSON output for all commands
- ✅ Simple status check commands
- ✅ Machine-readable error codes
- ✅ Minimal configuration required
- ✅ Official Claude Code hooks support

### Cross-Platform Support

- ✅ Windows, macOS, Linux
- ✅ Consistent behavior across platforms
- ✅ Platform-specific optimizations

## Architecture

```text
┌─────────────── CLI Layer ───────────────┐
│ Commands: start|stop|list|status|hooks  │
├─────────────── Core Layer ──────────────┤
│ ProcessManager | PortScanner | Health   │
├─────────────── Storage Layer ───────────┤
│ StateStore | ConfigManager | Lock       │
└─────────────────────────────────────────┘
```

### Hook Integration Architecture

```text
Claude Code
     ↓
PreToolUse Hook (pretooluse.sh)
     ↓
portguard intercept
     ↓
Process Conflict Detection
     ↓
JSON Response (proceed: true/false)
```

## Testing

### Test Hook Integration

```bash
# Test PreToolUse hook with different frameworks
echo '{"event":"preToolUse","tool_name":"Bash","parameters":{"command":"npm run dev"}}' | \
  portguard intercept

echo '{"event":"preToolUse","tool_name":"Bash","parameters":{"command":"air"}}' | \
  portguard intercept

echo '{"event":"preToolUse","tool_name":"Bash","parameters":{"command":"turbo run dev"}}' | \
  portguard intercept

# Test PostToolUse hook
echo '{"event":"postToolUse","tool_name":"Bash","parameters":{"command":"npm run dev"},"result":{"success":true,"output":"Server running on port 3000"}}' | \
  portguard intercept

# Test project-based commands
portguard start api --config test-config.yml
```

### Run Test Suite

```bash
# Run hook tests
./hooks/test_hooks.sh

# Run Go tests
go test ./...

# Run linting
golangci-lint run
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

MIT License - see LICENSE file for details.

## Support

- [GitHub Issues](https://github.com/paveg/portguard/issues)
- [Documentation](https://github.com/paveg/portguard/wiki)
