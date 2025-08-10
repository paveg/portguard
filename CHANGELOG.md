# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2025-08-10

### Added

- **Core Process Management**: Complete process lifecycle management with real process spawning using `os/exec`
- **Process Monitoring**: PID tracking, health monitoring, and background status updates
- **Duplicate Detection**: Intelligent command signature matching and port conflict resolution
- **Process Reuse**: Smart reuse of healthy existing processes to prevent duplicates
- **Signal Handling**: Graceful SIGTERM and force SIGKILL termination support
- **Claude Code Integration**: Full hooks support with `portguard intercept` command
- **CLI Commands**: Complete command set including start, stop, list, check, config management
- **Cross-platform Support**: Windows, macOS, and Linux compatibility
- **Configuration Management**: YAML-based configuration with environment variable support
- **Port Management**: Advanced port scanning and conflict detection
- **State Persistence**: JSON-based state storage with file locking for concurrency
- **Health Checks**: HTTP, TCP, and command-based health checking
- **AI-Friendly Output**: JSON output mode for AI tool integration
- **Comprehensive Testing**: 80.4% test coverage with unit and integration tests

### Technical Features

- **ProcessManager**: Central orchestrator for all process operations
- **StateStore**: Atomic JSON persistence with concurrent access protection
- **LockManager**: File-based locking mechanism
- **PortScanner**: Cross-platform port availability detection
- **Hook System**: Official Claude Code hooks specification compliance
- **Command Detection**: Pattern matching for common development servers
- **Background Monitoring**: Non-blocking process health monitoring
- **Automatic Cleanup**: Stale process removal and resource management

### CLI Usage

```bash
# Basic usage
portguard start "npm run dev" --port 3000
portguard list
portguard stop <process-id>

# AI integration
portguard check --port 3000 --json
portguard intercept  # For Claude Code hooks

# Configuration
portguard config init
portguard config show
```

### Claude Code Integration

Configure in `~/.claude/settings.json`:

```json
{
  "hooks": {
    "preToolUse": {
      "enabled": true,
      "command": "portguard intercept"
    },
    "postToolUse": {
      "enabled": true,
      "command": "portguard intercept"
    }
  }
}
```

### Server Detection Patterns

Automatically detects and manages:

- `npm run dev`, `npm start`, `yarn dev`
- `go run main.go`, `go run server.go`
- `python manage.py runserver`, `flask run`
- `node server.js`, `nodemon`, `vite`
- And many more development server patterns

[0.1.0]: https://github.com/paveg/portguard/releases/tag/v0.1.0
