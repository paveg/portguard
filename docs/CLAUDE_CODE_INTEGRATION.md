# Claude Code Integration Guide

Portguard integrates seamlessly with Claude Code to provide intelligent process management during AI-assisted development sessions. This prevents duplicate server startups and port conflicts when working with AI development tools.

## Overview

The integration uses Claude Code's hooks system to:

1. **PreToolUse Hook**: Intercepts commands before execution to detect potential server startups and prevent conflicts
2. **PostToolUse Hook**: Registers successfully started processes in portguard's state management

## Prerequisites

1. **Portguard**: Install portguard and ensure it's available in your PATH
   ```bash
   go install github.com/paveg/portguard/cmd/portguard@latest
   ```

2. **jq**: Required for JSON parsing in hook scripts
   ```bash
   # macOS
   brew install jq
   
   # Ubuntu/Debian
   sudo apt-get install jq
   
   # Other platforms: https://jqlang.github.io/jq/download/
   ```

3. **Claude Code**: Ensure you have Claude Code installed and configured

## Installation

### 1. Copy Hook Scripts

Copy the hook scripts to a directory accessible by Claude Code:

```bash
# Create hooks directory (choose a location that suits your setup)
mkdir -p ~/.config/claude-code/hooks

# Copy the hook scripts
cp hooks/pretooluse.sh ~/.config/claude-code/hooks/
cp hooks/posttooluse.sh ~/.config/claude-code/hooks/

# Ensure scripts are executable
chmod +x ~/.config/claude-code/hooks/*.sh
```

### 2. Configure Claude Code Hooks

Add the following to your Claude Code settings (`.claude/settings.json` or `.claude/settings.local.json`):

```json
{
  "hooks": {
    "preToolUse": {
      "enabled": true,
      "command": "~/.config/claude-code/hooks/pretooluse.sh",
      "timeout": 10000,
      "failureHandling": "allow"
    },
    "postToolUse": {
      "enabled": true, 
      "command": "~/.config/claude-code/hooks/posttooluse.sh",
      "timeout": 5000,
      "failureHandling": "ignore"
    }
  },
  "tools": {
    "bash": {
      "enabled": true
    }
  }
}
```

### 3. Environment Configuration

Set up environment variables for enhanced functionality:

```bash
# Add to your ~/.bashrc, ~/.zshrc, or similar
export PORTGUARD_BIN="portguard"  # Path to portguard binary
export PORTGUARD_DEBUG="1"       # Enable debug logging (optional)
```

## How It Works

### PreToolUse Hook Flow

1. **Command Interception**: When Claude Code attempts to run a bash command, the PreToolUse hook intercepts it
2. **Server Detection**: The hook analyzes the command to determine if it's a server startup command
3. **Conflict Check**: If it's a server command, portguard checks for existing processes or port conflicts
4. **Decision**: The hook returns a decision:
   - `allow`: Command can proceed normally
   - `block`: Command is blocked due to conflicts
   - `modify`: Command is modified to resolve conflicts (future feature)

### PostToolUse Hook Flow

1. **Result Analysis**: After a command executes, the hook analyzes the output and exit code
2. **Success Detection**: If the command appears to have successfully started a server, it's registered
3. **Process Registration**: The process is added to portguard's state management with:
   - Command that was executed
   - Detected port number
   - Working directory
   - Timestamp information

## Supported Server Commands

Portguard automatically detects these common development server patterns:

### Node.js/JavaScript
- `npm run dev`, `npm run start`, `npm run serve`
- `yarn dev`, `yarn start`, `yarn serve`
- `pnpm dev`, `pnpm start`, `pnpm serve`
- `node server.js`, `node index.js`
- `next dev`, `next start`
- `vite`, `webpack-dev-server`

### Python
- `python app.py`, `python server.py`
- `flask run`
- `django-admin runserver`, `python manage.py runserver`
- `uvicorn`, `gunicorn`, `hypercorn`

### Go
- `go run main.go`, `go run server.go`
- `go run .`

### Ruby
- `rails server`

### Generic
- `serve`, `http-server`, `live-server`
- Commands with `--port` or `-p` flags
- Commands containing `localhost:PORT` patterns

## Configuration Options

### Hook Script Configuration

You can customize the hook behavior using environment variables:

```bash
# Portguard binary location
export PORTGUARD_BIN="/usr/local/bin/portguard"

# Enable debug logging
export PORTGUARD_DEBUG="1"

# Custom timeout for portguard operations (in seconds)
export PORTGUARD_TIMEOUT="10"
```

### Claude Code Settings

Customize hook behavior in your Claude Code settings:

```json
{
  "hooks": {
    "preToolUse": {
      "enabled": true,
      "command": "~/.config/claude-code/hooks/pretooluse.sh",
      "timeout": 10000,
      "failureHandling": "allow",  // "allow" | "block" | "warn"
      "environment": {
        "PORTGUARD_DEBUG": "1"
      }
    },
    "postToolUse": {
      "enabled": true,
      "command": "~/.config/claude-code/hooks/posttooluse.sh", 
      "timeout": 5000,
      "failureHandling": "ignore"  // "ignore" | "warn" | "error"
    }
  }
}
```

## Testing the Integration

### 1. Test PreToolUse Hook

Test that the PreToolUse hook correctly intercepts commands:

```bash
# Test with a sample server command
echo '{
  "tool_name": "Bash",
  "parameters": {
    "command": "npm run dev --port 3000"
  },
  "session_id": "test123",
  "working_dir": "/path/to/project"
}' | ~/.config/claude-code/hooks/pretooluse.sh
```

Expected output:
```json
{
  "action": "allow",
  "reason": "Server command detected, no conflicts found",
  "process_info": {
    "command_type": "server",
    "detected_port": 3000,
    "full_command": "npm run dev --port 3000"
  }
}
```

### 2. Test PostToolUse Hook

Test that the PostToolUse hook registers successful server starts:

```bash
# Test with a sample successful server startup
echo '{
  "tool_name": "Bash",
  "parameters": {
    "command": "npm run dev"
  },
  "result": {
    "success": true,
    "output": "Server listening on port 3000",
    "exit_code": 0
  },
  "session_id": "test123",
  "working_dir": "/path/to/project"
}' | ~/.config/claude-code/hooks/posttooluse.sh
```

### 3. Verify Process Registration

Check that the process was registered with portguard:

```bash
portguard list --json
```

## Troubleshooting

### Common Issues

1. **Hook not executing**
   - Verify hook scripts are executable: `chmod +x ~/.config/claude-code/hooks/*.sh`
   - Check Claude Code settings.json configuration
   - Ensure hooks are in the correct path

2. **jq command not found**
   - Install jq: `brew install jq` (macOS) or `sudo apt-get install jq` (Ubuntu)

3. **Portguard binary not found**
   - Ensure portguard is in PATH: `which portguard`
   - Set PORTGUARD_BIN environment variable to full path

4. **Hook timeouts**
   - Increase timeout values in Claude Code settings
   - Check system performance and disk I/O

### Debug Mode

Enable debug logging to troubleshoot issues:

```bash
export PORTGUARD_DEBUG="1"
```

Debug information will be written to stderr and won't interfere with hook JSON output.

### Manual Testing

Test individual components:

```bash
# Test portguard intercept directly
echo '{"command":"npm run dev","args":["--port","3000"]}' | portguard intercept

# Test hook scripts directly
echo '{"tool_name":"Bash","parameters":{"command":"npm run dev"}}' | ~/.config/claude-code/hooks/pretooluse.sh
```

## Security Considerations

1. **Hook Scripts**: Store hook scripts in a secure location with appropriate permissions
2. **Input Validation**: Hooks validate JSON input to prevent injection attacks  
3. **Timeout Limits**: Hooks use timeouts to prevent hanging processes
4. **Fail-Safe Behavior**: PreToolUse hook fails open (allows commands) if errors occur

## Advanced Configuration

### Custom Port Detection

Extend port detection patterns by modifying the hook scripts or contributing to the portguard project.

### Integration with CI/CD

The hooks can be configured to work in CI/CD environments:

```json
{
  "hooks": {
    "preToolUse": {
      "enabled": true,
      "environment": {
        "PORTGUARD_CI_MODE": "1"
      }
    }
  }
}
```

## Contributing

To contribute improvements to the Claude Code integration:

1. Fork the repository
2. Create a feature branch
3. Make your changes to hook scripts or documentation
4. Test thoroughly with various server types
5. Submit a pull request

For issues and feature requests, please use the [GitHub Issues](https://github.com/paveg/portguard/issues) page.