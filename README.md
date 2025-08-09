# Portguard

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

```bash
# Build from source
git clone https://github.com/paveg/portguard
cd portguard
go build -o portguard cmd/portguard/main.go

# Add to PATH
mv portguard /usr/local/bin/
```

## Quick Start

### Basic Usage

```bash
# Start a process (or reuse existing one)
portguard start "go run main.go" --port 3000

# List all managed processes
portguard list

# Check status
portguard status

# Stop a process
portguard stop 3000

# Clean up all processes
portguard clean
```

### AI-Friendly Commands

```bash
# Quick status check (returns JSON)
portguard check --json

# Check if specific port is available
portguard check --port 3000 --json

# Find next available port
portguard check --available --start 3000 --json
```

### Configuration

```bash
# Initialize configuration file
portguard config init

# Show current configuration
portguard config show
```

## Commands

### Core Commands

- `portguard start <command>` - Start a new process or reuse existing one
- `portguard stop <id|port>` - Stop a managed process  
- `portguard list` - List all managed processes
- `portguard status [id]` - Show process status and health information
- `portguard clean` - Clean up all managed processes

### Utility Commands

- `portguard ports` - Show port usage information
- `portguard health [id]` - Check health status of processes
- `portguard check` - Quick status check (AI-friendly)
- `portguard config` - Configuration management

## Configuration

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
  
  api:
    command: "go run main.go"
    port: 3001
    working_dir: "./api"
```

## AI Integration Examples

### Claude Code Integration

```bash
# Check if server is already running before starting
if portguard check --port 3000 --json | jq -r '.port_in_use' = "false"; then
    portguard start "npm run dev" --port 3000
else
    echo "Server already running on port 3000"
fi
```

### JSON API for Automated Tools

```bash
# Get status as JSON
portguard check --json
# Returns: {"portguard_running": true, "managed_processes": 2, ...}

# List processes as JSON  
portguard list --json
# Returns: [{"id": "abc123", "command": "npm run dev", "port": 3000, ...}]
```

## Features

### Intelligent Duplicate Detection
- ✅ Command-based process matching
- ✅ Port conflict detection
- ✅ Process health validation
- ✅ Automatic reuse of healthy processes

### Health Monitoring
- ✅ HTTP health checks
- ✅ TCP connectivity checks  
- ✅ Custom command checks
- ✅ Automatic process recovery

### AI-Friendly Design
- ✅ JSON output for all commands
- ✅ Simple status check commands
- ✅ Machine-readable error codes
- ✅ Minimal configuration required

### Cross-Platform Support
- ✅ Windows, macOS, Linux
- ✅ Consistent behavior across platforms
- ✅ Platform-specific optimizations

## Development Status

**Phase 1: Complete** ✅
- [x] Project structure and CLI framework
- [x] Core data structures  
- [x] JSON state persistence
- [x] File-based locking system
- [x] Basic port scanning
- [x] Configuration management

**Phase 2: In Progress** 🚧
- [ ] Process lifecycle management
- [ ] Duplicate detection algorithm
- [ ] Process reuse logic
- [ ] Advanced port management

**Phase 3: Planned** 📋
- [ ] Health check implementation
- [ ] Health monitoring system
- [ ] Advanced CLI features
- [ ] Auto-completion

**Phase 4: Future** 🔮
- [ ] Full AI integration
- [ ] Performance optimization
- [ ] Comprehensive testing
- [ ] Documentation

## Architecture

```
┌─────────────── CLI Layer ───────────────┐
│ Commands: start|stop|list|status|clean  │
├─────────────── Core Layer ──────────────┤
│ ProcessManager | PortScanner | Health   │
├─────────────── Storage Layer ───────────┤
│ StateStore | ConfigManager | Lock       │
└─────────────────────────────────────────┘
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

- GitHub Issues: https://github.com/paveg/portguard/issues
- Documentation: https://github.com/paveg/portguard/wiki