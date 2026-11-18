// Design: docs/architecture/cli/command-completion.md -- offline inspection flags.
// Related: cmd_diff.go, cmd_dump.go, cmd_fmt.go, cmd_graph.go, cmd_validate.go -- parsers.

package cli

import "github.com/ze-software/ze/internal/component/command/registry"

const inspectionAccountHelp = "account for requesting external plugin schemas from the running ze"

func init() {
	registry.RegisterCommandFlags("config diff", []registry.FlagSpec{
		{Name: flagTokenBackup, Description: "compare configs inside a backup artifact", ValueHint: registry.FlagValueFile},
		{Name: flagTokenUser, Description: inspectionAccountHelp, ValueHint: registry.FlagValueText},
	})
	registry.RegisterCommandFlags("config dump", []registry.FlagSpec{
		{Name: "--strip-private", Description: "replace sensitive values with /* SECRET-DATA */", ValueHint: registry.FlagValueNone},
		{Name: flagTokenUser, Description: inspectionAccountHelp, ValueHint: registry.FlagValueText},
	})
	registry.RegisterCommandFlags("config fmt", []registry.FlagSpec{
		{Name: "-w", Description: "write result to source file", ValueHint: registry.FlagValueNone},
		{Name: "--check", Description: "check if formatting is needed", ValueHint: registry.FlagValueNone},
		{Name: "--diff", Description: "show unified diff of changes", ValueHint: registry.FlagValueNone},
		{Name: flagTokenUser, Description: inspectionAccountHelp, ValueHint: registry.FlagValueText},
	})
	registry.RegisterCommandFlags("config graph", []registry.FlagSpec{
		{Name: flagTokenUser, Description: inspectionAccountHelp, ValueHint: registry.FlagValueText},
	})
	registry.RegisterCommandFlags("config validate", []registry.FlagSpec{
		{Name: "-v", Description: "verbose output", ValueHint: registry.FlagValueNone},
		{Name: "-q", Description: "quiet mode (exit code only)", ValueHint: registry.FlagValueNone},
		{Name: "--pending", Description: "validate pending draft config", ValueHint: registry.FlagValueNone},
		{Name: "--limit", Description: "limit validation to section (environment)", ValueHint: registry.FlagValueText},
		{Name: flagTokenUser, Description: inspectionAccountHelp, ValueHint: registry.FlagValueText},
	})
}
