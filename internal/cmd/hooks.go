package cmd

import (
	"fmt"
	"os"

	"github.com/paveg/portguard/internal/hooks"
	"github.com/spf13/cobra"
)

// hooksCmd represents the hooks management command
var hooksCmd = &cobra.Command{
	Use:   "hooks",
	Short: "Manage Claude Code hooks integration",
	Long: `Manage Claude Code hooks integration for seamless AI development workflow.

This command provides tools to install, update, and manage Portguard's Claude Code 
hooks without manual script copying or configuration.

Available subcommands:
  install  - Install hooks for Claude Code integration
  update   - Update existing hook installations
  list     - List available templates and installed hooks
  remove   - Remove installed hooks
  status   - Check hook installation status`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := cmd.Help(); err != nil {
			fmt.Fprintf(os.Stderr, "Error displaying help: %v\n", err)
		}
	},
}

// hooksInstallCmd installs Claude Code hooks
var hooksInstallCmd = &cobra.Command{
	Use:   "install [template]",
	Short: "Install Claude Code hooks",
	Long: `Install Claude Code hooks for seamless integration.

This command automatically:
1. Creates hook scripts in the appropriate location
2. Updates your Claude Code configuration
3. Sets up the necessary permissions and dependencies

Templates:
  basic      - Basic server conflict prevention (default)
  advanced   - Advanced process management with lifecycle tracking
  developer  - Full development workflow optimization

Examples:
  portguard hooks install                    # Install basic template
  portguard hooks install basic             # Install basic template explicitly  
  portguard hooks install --claude-config ~/.config/claude-code
  portguard hooks install advanced --dry-run`,
	Run: func(cmd *cobra.Command, args []string) {
		runner := NewCommandRunner(jsonOutput, dryRun)

		template := "basic"
		if len(args) > 0 {
			template = args[0]
		}

		config := &hooks.InstallConfig{
			Template:     template,
			ClaudeConfig: claudeConfigPath,
			DryRun:       dryRun,
			Force:        force,
		}

		installer := hooks.NewInstaller()
		result, err := installer.Install(config)
		if err != nil {
			runner.OutputHandler.PrintError("Failed to install hooks", err)
			return
		}

		if jsonOutput {
			if err := runner.OutputHandler.PrintJSON(result); err != nil {
				runner.OutputHandler.PrintError("Failed to output result", err)
			}
		} else {
			runner.OutputHandler.PrintSuccess("Hooks installed successfully", result)
		}
	},
}

// hooksListCmd lists available templates and installed hooks
var hooksListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available templates and installed hooks",
	Long: `List available hook templates and show installation status.

This command shows:
- Available templates with descriptions
- Currently installed hooks and their status
- Configuration locations and settings

Options:
  --templates  Show only available templates
  --installed  Show only installed hooks`,
	Run: func(cmd *cobra.Command, args []string) {
		runner := NewCommandRunner(jsonOutput, false)

		manager := hooks.NewManager()

		var result interface{}
		var err error

		if showTemplates {
			result, err = manager.ListTemplates()
		} else if showInstalled {
			result, err = manager.ListInstalled()
		} else {
			result, err = manager.ListAll()
		}

		if err != nil {
			runner.OutputHandler.PrintError("Failed to list hooks", err)
			return
		}

		if jsonOutput {
			if err := runner.OutputHandler.PrintJSON(result); err != nil {
				runner.OutputHandler.PrintError("Failed to output result", err)
			}
		} else {
			printHooksInfo(result)
		}
	},
}

// hooksUpdateCmd updates installed hooks
var hooksUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update installed Claude Code hooks",
	Long: `Update installed Claude Code hooks to the latest version.

This command:
- Updates hook scripts to match the current portguard version
- Preserves user configuration and customizations
- Validates the updated installation

The update process is safe and maintains backwards compatibility.`,
	Run: func(cmd *cobra.Command, args []string) {
		runner := NewCommandRunner(jsonOutput, dryRun)

		updater := hooks.NewUpdater()
		result, err := updater.Update(&hooks.UpdateConfig{
			DryRun: dryRun,
			Force:  force,
		})

		if err != nil {
			runner.OutputHandler.PrintError("Failed to update hooks", err)
			return
		}

		if jsonOutput {
			if err := runner.OutputHandler.PrintJSON(result); err != nil {
				runner.OutputHandler.PrintError("Failed to output result", err)
			}
		} else {
			runner.OutputHandler.PrintSuccess("Hooks updated successfully", result)
		}
	},
}

// hooksRemoveCmd removes installed hooks
var hooksRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove installed Claude Code hooks",
	Long: `Remove installed Claude Code hooks and clean up configuration.

This command:
- Removes hook scripts from Claude Code configuration
- Cleans up settings.json entries
- Optionally preserves user customizations

Use --force to skip confirmation prompts.`,
	Run: func(cmd *cobra.Command, args []string) {
		runner := NewCommandRunner(jsonOutput, dryRun)

		remover := hooks.NewRemover()
		result, err := remover.Remove(&hooks.RemoveConfig{
			DryRun:         dryRun,
			Force:          force,
			PreserveConfig: !cleanAll,
		})

		if err != nil {
			runner.OutputHandler.PrintError("Failed to remove hooks", err)
			return
		}

		if jsonOutput {
			if err := runner.OutputHandler.PrintJSON(result); err != nil {
				runner.OutputHandler.PrintError("Failed to output result", err)
			}
		} else {
			runner.OutputHandler.PrintSuccess("Hooks removed successfully", result)
		}
	},
}

// hooksStatusCmd shows hook installation status
var hooksStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check Claude Code hooks installation status",
	Long: `Check the installation status of Claude Code hooks.

This command shows:
- Hook installation status and versions
- Claude Code configuration status
- Dependency availability (jq, portguard binary)
- Configuration file locations and permissions`,
	Run: func(cmd *cobra.Command, args []string) {
		runner := NewCommandRunner(jsonOutput, false)

		checker := hooks.NewStatusChecker()
		status, err := checker.Check()

		if err != nil {
			runner.OutputHandler.PrintError("Failed to check hook status", err)
			return
		}

		if jsonOutput {
			if err := runner.OutputHandler.PrintJSON(status); err != nil {
				runner.OutputHandler.PrintError("Failed to output status", err)
			}
		} else {
			printHooksStatus(status)
		}
	},
}

// Variables for hooks command flags
var (
	claudeConfigPath string
	showTemplates    bool
	showInstalled    bool
	cleanAll         bool
)

// printHooksInfo prints hooks information in human-readable format
func printHooksInfo(data interface{}) {
	switch v := data.(type) {
	case *hooks.ListResult:
		fmt.Println("Available Templates:")
		fmt.Println("==================")
		//nolint:gocritic // TODO: Consider using pointers or indexing to avoid copying 128 bytes per iteration
		for _, template := range v.Templates {
			fmt.Printf("  %s - %s\n", template.Name, template.Description)
		}

		fmt.Println("\nInstalled Hooks:")
		fmt.Println("================")
		if len(v.Installed) == 0 {
			fmt.Println("  No hooks currently installed")
		} else {
			for _, hook := range v.Installed {
				fmt.Printf("  %s (%s) - %s\n", hook.Name, hook.Version, hook.Status)
			}
		}
	default:
		fmt.Printf("Hooks Information: %+v\n", v)
	}
}

// printHooksStatus prints hook status in human-readable format
func printHooksStatus(status *hooks.StatusResult) {
	fmt.Println("Claude Code Hooks Status")
	fmt.Println("========================")

	if status.Installed {
		fmt.Printf("Status:       ✓ Installed\n")
		fmt.Printf("Version:      %s\n", status.Version)
		fmt.Printf("Template:     %s\n", status.Template)
		fmt.Printf("Config Path:  %s\n", status.ConfigPath)
	} else {
		fmt.Printf("Status:       ✗ Not Installed\n")
	}

	fmt.Printf("Dependencies: ")
	if status.DependenciesOK {
		fmt.Printf("✓ OK\n")
	} else {
		fmt.Printf("✗ Missing\n")
		for _, missing := range status.MissingDeps {
			fmt.Printf("  - %s\n", missing)
		}
	}
}

func init() {
	rootCmd.AddCommand(hooksCmd)

	// Add subcommands
	hooksCmd.AddCommand(hooksInstallCmd)
	hooksCmd.AddCommand(hooksListCmd)
	hooksCmd.AddCommand(hooksUpdateCmd)
	hooksCmd.AddCommand(hooksRemoveCmd)
	hooksCmd.AddCommand(hooksStatusCmd)

	// Install command flags
	hooksInstallCmd.Flags().StringVar(&claudeConfigPath, "claude-config", "", "Path to Claude Code configuration directory")
	hooksInstallCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be installed without making changes")
	hooksInstallCmd.Flags().BoolVar(&force, "force", false, "Force installation even if hooks already exist")
	AddCommonJSONFlag(hooksInstallCmd)

	// List command flags
	hooksListCmd.Flags().BoolVar(&showTemplates, "templates", false, "Show only available templates")
	hooksListCmd.Flags().BoolVar(&showInstalled, "installed", false, "Show only installed hooks")
	AddCommonJSONFlag(hooksListCmd)

	// Update command flags
	hooksUpdateCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be updated without making changes")
	hooksUpdateCmd.Flags().BoolVar(&force, "force", false, "Force update even if no changes detected")
	AddCommonJSONFlag(hooksUpdateCmd)

	// Remove command flags
	hooksRemoveCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be removed without making changes")
	hooksRemoveCmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompts")
	hooksRemoveCmd.Flags().BoolVar(&cleanAll, "clean-all", false, "Remove all configurations and customizations")
	AddCommonJSONFlag(hooksRemoveCmd)

	// Status command flags
	AddCommonJSONFlag(hooksStatusCmd)
}
