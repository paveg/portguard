package hooks

import (
	"embed"
	"time"
)

//go:embed templates/*
var templatesFS embed.FS

// GetBuiltinTemplates returns all built-in hook templates
func GetBuiltinTemplates() ([]Template, error) {
	templates := []Template{
		getBasicTemplate(),
		getAdvancedTemplate(),
		getDeveloperTemplate(),
	}

	return templates, nil
}

// GetTemplate returns a specific template by name
func GetTemplate(name string) (*Template, error) {
	templates, err := GetBuiltinTemplates()
	if err != nil {
		return nil, err
	}

	for _, template := range templates {
		if template.Name == name {
			return &template, nil
		}
	}

	return nil, ErrTemplateNotFound
}

// getBasicTemplate returns the basic hook template
func getBasicTemplate() Template {
	return Template{
		Name:         "basic",
		Description:  "Basic server conflict prevention for development workflows",
		Version:      "1.0.0",
		Dependencies: []string{"jq", "portguard"},
		Config: map[string]string{
			"PORTGUARD_DEBUG":   "0",
			"PORTGUARD_TIMEOUT": "10",
		},
		Hooks: []HookDefinition{
			{
				Name:        "preToolUse",
				Type:        PreToolUse,
				Script:      getBasicPreToolUseScript(),
				Timeout:     10 * time.Second,
				FailureMode: FailureAllow,
				Environment: map[string]string{
					"PORTGUARD_BIN": "portguard",
				},
				Enabled:     true,
				Description: "Prevents duplicate server startups by analyzing commands before execution",
			},
			{
				Name:        "postToolUse",
				Type:        PostToolUse,
				Script:      getBasicPostToolUseScript(),
				Timeout:     5 * time.Second,
				FailureMode: FailureIgnore,
				Environment: map[string]string{
					"PORTGUARD_BIN": "portguard",
				},
				Enabled:     true,
				Description: "Registers successfully started server processes for tracking",
			},
		},
		Examples: []Example{
			{
				Name:        "Prevent npm duplicate",
				Description: "Prevents starting npm dev server when one is already running",
				Command:     "echo '{\"tool_name\":\"Bash\",\"parameters\":{\"command\":\"npm run dev\"}}' | portguard hooks exec preToolUse",
				Expected:    "{\"action\":\"block\",\"reason\":\"npm dev server already running on port 3000\"}",
			},
		},
	}
}

// getAdvancedTemplate returns the advanced hook template
func getAdvancedTemplate() Template {
	basic := getBasicTemplate()
	advanced := basic
	advanced.Name = "advanced"
	advanced.Description = "Advanced process management with lifecycle tracking and health monitoring"
	advanced.Dependencies = []string{"jq", "portguard", "ps", "lsof"}
	advanced.Config["PORTGUARD_DEBUG"] = "1"
	advanced.Config["PORTGUARD_TIMEOUT"] = "15"
	advanced.Config["PORTGUARD_HEALTH_CHECK"] = "1"

	// Update hook configurations for advanced template
	for i := range advanced.Hooks {
		if advanced.Hooks[i].Name == "preToolUse" {
			advanced.Hooks[i].Timeout = 15 * time.Second
			advanced.Hooks[i].FailureMode = FailureWarn
			advanced.Hooks[i].Environment["PORTGUARD_ADVANCED"] = "1"
			advanced.Hooks[i].Description = "Advanced command analysis with health checking and smart conflict resolution"
		} else if advanced.Hooks[i].Name == "postToolUse" {
			advanced.Hooks[i].Timeout = 10 * time.Second
			advanced.Hooks[i].FailureMode = FailureWarn
			advanced.Hooks[i].Environment["PORTGUARD_ADVANCED"] = "1"
			advanced.Hooks[i].Description = "Comprehensive process registration with health monitoring setup"
		}
	}

	return advanced
}

// getDeveloperTemplate returns the developer workflow template
func getDeveloperTemplate() Template {
	basic := getBasicTemplate()
	developer := basic
	developer.Name = "developer"
	developer.Description = "Full development workflow optimization with smart port management"
	developer.Dependencies = []string{"jq", "portguard", "ps", "lsof", "netstat"}
	developer.Config["PORTGUARD_DEBUG"] = "1"
	developer.Config["PORTGUARD_TIMEOUT"] = "20"
	developer.Config["PORTGUARD_AUTO_PORT"] = "1"
	developer.Config["PORTGUARD_SMART_RESTART"] = "1"

	// Update hook configurations for developer template
	for i := range developer.Hooks {
		if developer.Hooks[i].Name == "preToolUse" {
			developer.Hooks[i].Timeout = 20 * time.Second
			developer.Hooks[i].FailureMode = FailureWarn
			developer.Hooks[i].Environment["PORTGUARD_DEVELOPER"] = "1"
			developer.Hooks[i].Description = "Smart workflow optimization with automatic port assignment and process reuse"
		} else if developer.Hooks[i].Name == "postToolUse" {
			developer.Hooks[i].Timeout = 15 * time.Second
			developer.Hooks[i].FailureMode = FailureWarn
			developer.Hooks[i].Environment["PORTGUARD_DEVELOPER"] = "1"
			developer.Hooks[i].Description = "Advanced process tracking with workflow optimization"
		}
	}

	return developer
}

// Script content getters

func getBasicPreToolUseScript() string {
	return "#!/bin/bash\n" +
		"set -euo pipefail\n\n" +
		"PORTGUARD_BIN=\"${PORTGUARD_BIN:-portguard}\"\n\n" +
		"if ! command -v \"$PORTGUARD_BIN\" >/dev/null 2>&1; then\n" +
		"    echo '{\"action\":\"allow\",\"reason\":\"portguard not available\"}'\n" +
		"    exit 0\n" +
		"fi\n\n" +
		"if ! command -v jq >/dev/null 2>&1; then\n" +
		"    echo '{\"action\":\"allow\",\"reason\":\"jq not available\"}'\n" +
		"    exit 0\n" +
		"fi\n\n" +
		"json_input=$(cat)\n" +
		"tool_name=$(echo \"$json_input\" | jq -r '.tool_name // \"unknown\"')\n\n" +
		"if [[ \"$tool_name\" != \"Bash\" ]]; then\n" +
		"    echo '{\"action\":\"allow\",\"reason\":\"non-bash tool\"}'\n" +
		"    exit 0\n" +
		"fi\n\n" +
		"command=$(echo \"$json_input\" | jq -r '.parameters.command // \"\"')\n" +
		"if [[ -z \"$command\" ]]; then\n" +
		"    echo '{\"action\":\"allow\",\"reason\":\"no command\"}'\n" +
		"    exit 0\n" +
		"fi\n\n" +
		"intercept_request=$(cat << EOF\n" +
		"{\n" +
		"  \"command\": \"$command\",\n" +
		"  \"tool_name\": \"$tool_name\",\n" +
		"  \"session_id\": \"${CLAUDE_SESSION_ID:-unknown}\",\n" +
		"  \"working_dir\": \"${CLAUDE_WORKING_DIR:-unknown}\"\n" +
		"}\n" +
		"EOF\n" +
		")\n\n" +
		"echo \"$intercept_request\" | \"$PORTGUARD_BIN\" intercept 2>/dev/null || echo '{\"action\":\"allow\",\"reason\":\"intercept failed\"}'\n"
}

func getBasicPostToolUseScript() string {
	return "#!/bin/bash\n" +
		"set -euo pipefail\n\n" +
		"PORTGUARD_BIN=\"${PORTGUARD_BIN:-portguard}\"\n\n" +
		"if ! command -v \"$PORTGUARD_BIN\" >/dev/null 2>&1 || ! command -v jq >/dev/null 2>&1; then\n" +
		"    exit 0\n" +
		"fi\n\n" +
		"json_input=$(cat)\n" +
		"tool_name=$(echo \"$json_input\" | jq -r '.tool_name // \"unknown\"')\n\n" +
		"if [[ \"$tool_name\" != \"Bash\" ]]; then\n" +
		"    exit 0\n" +
		"fi\n\n" +
		"command=$(echo \"$json_input\" | jq -r '.parameters.command // \"\"')\n" +
		"success=$(echo \"$json_input\" | jq -r '.result.success // false')\n" +
		"exit_code=$(echo \"$json_input\" | jq -r '.result.exit_code // -1')\n\n" +
		"if [[ \"$success\" != \"true\" ]] || [[ \"$exit_code\" != \"0\" ]]; then\n" +
		"    exit 0\n" +
		"fi\n\n" +
		"# Simple server pattern matching\n" +
		"if [[ \"$command\" =~ (npm|yarn|pnpm).*dev|next.*dev|vite|flask.*run ]]; then\n" +
		"    # Background process registration to avoid blocking\n" +
		"    (timeout 10s \"$PORTGUARD_BIN\" start --command \"$command\" --background --json >/dev/null 2>&1 || true) &\n" +
		"fi\n"
}
