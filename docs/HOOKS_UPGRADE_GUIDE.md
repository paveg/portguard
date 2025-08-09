# Portguard Hooks Upgrade Guide

This document explains the new simplified approach to Claude Code hooks installation and compares it with the previous manual method.

## 🚀 New Simplified Approach (Recommended)

With Portguard v1.1+, hook installation is now a **single command**:

```bash
# One-command installation
portguard hooks install

# Or with a specific template
portguard hooks install advanced

# Or with custom configuration path
portguard hooks install --claude-config ~/.config/claude-code
```

### Benefits of the New Approach

✅ **Zero Manual Setup**: No shell script copying  
✅ **Automatic Updates**: Built-in update mechanism  
✅ **Template System**: Choose from predefined configurations  
✅ **Easy Management**: List, update, and remove hooks easily  
✅ **Better Maintenance**: Hooks stay current with portguard releases  

### Available Templates

| Template | Use Case | Dependencies |
|----------|----------|-------------|
| **basic** | Simple server conflict prevention | `jq`, `portguard` |
| **advanced** | Health monitoring & lifecycle tracking | `jq`, `portguard`, `ps`, `lsof` |
| **developer** | Full workflow optimization | `jq`, `portguard`, `ps`, `lsof`, `netstat` |

## 📋 Quick Commands

```bash
# List available templates
portguard hooks list --templates

# Check installation status  
portguard hooks status

# Update installed hooks
portguard hooks update

# Remove hooks (if needed)
portguard hooks remove
```

## 🔧 Migration from Manual Installation

If you previously installed hooks manually, here's how to upgrade:

### Step 1: Check Current Installation
```bash
# Check what's currently installed
ls ~/.config/claude-code/hooks/
cat ~/.config/claude-code/settings.json | jq '.hooks'
```

### Step 2: Backup (Optional)
```bash
# Backup current hooks directory
cp -r ~/.config/claude-code/hooks ~/.config/claude-code/hooks.backup
cp ~/.config/claude-code/settings.json ~/.config/claude-code/settings.json.backup
```

### Step 3: Clean Install
```bash
# Remove old hooks (optional - new install will overwrite)
rm -f ~/.config/claude-code/hooks/pretooluse.sh
rm -f ~/.config/claude-code/hooks/posttooluse.sh

# Install with new system
portguard hooks install basic
```

### Step 4: Verify Installation
```bash
# Verify everything is working
portguard hooks status

# Test the hooks
echo '{"tool_name":"Bash","parameters":{"command":"npm run dev"}}' | ~/.config/claude-code/hooks/preToolUse.sh
```

## 📖 Previous Manual Method (Legacy)

**⚠️ This method is still supported but no longer recommended**

<details>
<summary>Click to view legacy manual installation steps</summary>

### Legacy Installation Steps

1. **Copy Scripts Manually**
   ```bash
   mkdir -p ~/.config/claude-code/hooks
   cp hooks/pretooluse.sh ~/.config/claude-code/hooks/
   cp hooks/posttooluse.sh ~/.config/claude-code/hooks/
   chmod +x ~/.config/claude-code/hooks/*.sh
   ```

2. **Edit settings.json Manually**
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
     }
   }
   ```

### Why Manual Method is Problematic

❌ **Tedious Setup**: Multiple manual steps prone to errors  
❌ **No Updates**: Scripts become outdated  
❌ **Hard to Maintain**: Configuration scattered across files  
❌ **No Templates**: One-size-fits-all approach  
❌ **Error Prone**: Easy to misconfigure paths and permissions  

</details>

## 🎯 Feature Comparison

| Feature | Manual Method | New Hooks System |
|---------|---------------|------------------|
| **Installation** | Multi-step manual process | Single command |
| **Updates** | Manual file replacement | `portguard hooks update` |
| **Templates** | None | 3 built-in templates |
| **Configuration** | Manual JSON editing | Automatic config management |
| **Validation** | None | Built-in dependency checking |
| **Status Check** | Manual inspection | `portguard hooks status` |
| **Removal** | Manual cleanup | `portguard hooks remove` |
| **Version Tracking** | None | Automatic version management |

## 🔍 Under the Hood

The new hooks system:

1. **Generates optimized scripts** based on your chosen template
2. **Automatically configures** Claude Code settings.json
3. **Tracks installation metadata** in `.portguard-hooks.json`
4. **Validates dependencies** before installation
5. **Provides rollback capability** if something goes wrong

### Generated Files

```
~/.config/claude-code/
├── hooks/
│   ├── preToolUse.sh      # Generated hook script
│   └── postToolUse.sh     # Generated hook script
├── settings.json          # Updated automatically
└── .portguard-hooks.json  # Installation metadata
```

## 🚨 Troubleshooting

### Common Issues and Solutions

**Issue: "portguard not found"**
```bash
# Ensure portguard is in PATH
go install github.com/paveg/portguard/cmd/portguard@latest
```

**Issue: "jq not found"**
```bash
# Install jq
brew install jq  # macOS
sudo apt-get install jq  # Ubuntu
```

**Issue: "Claude Code config not found"**
```bash
# Specify path explicitly
portguard hooks install --claude-config ~/.config/claude-code
```

**Issue: "Permission denied"**
```bash
# Fix permissions
chmod +x ~/.config/claude-code/hooks/*.sh
```

### Debug Mode

Enable debug logging to troubleshoot issues:
```bash
export PORTGUARD_DEBUG=1
portguard hooks install --dry-run
```

## 📚 Additional Resources

- [Claude Code Integration Guide](./CLAUDE_CODE_INTEGRATION.md) - Complete setup documentation
- [Hook Templates Reference](../examples/) - Example configurations
- [Portguard Commands](../README.md#commands) - Full command reference

## 💡 Pro Tips

1. **Start with Basic**: Use the `basic` template first, upgrade later if needed
2. **Dry Run First**: Always test with `--dry-run` before actual installation
3. **Check Status Regularly**: Use `portguard hooks status` to monitor health
4. **Keep Updated**: Run `portguard hooks update` periodically
5. **Backup Settings**: Keep backups of your Claude Code configuration

---

**Ready to upgrade?** Run `portguard hooks install` and experience the difference! 🎉