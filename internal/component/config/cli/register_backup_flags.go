// Design: docs/architecture/cli/command-completion.md -- offline backup flags.
// Related: cmd_edit.go, cmd_list.go, cmd_set.go, cmd_show.go -- complete parser sets.

package cli

import "github.com/ze-software/ze/internal/component/command/registry"

func init() {
	registry.RegisterCommandFlags("config edit", []registry.FlagSpec{
		{Name: "-f", Description: "edit a loose file without the live store", ValueHint: registry.FlagValueNone},
		{Name: "--backup", Description: "edit a config inside a backup artifact, with no daemon", ValueHint: registry.FlagValueFile},
		{Name: "--user", Description: "SSH login username (overrides stored super-admin)", ValueHint: registry.FlagValueText},
		{Name: "-u", Description: "short alias for --user", ValueHint: registry.FlagValueText},
		{Name: "--web", Description: "start web UI on the given port", ValueHint: registry.FlagValueText},
		{Name: "--insecure-web", Description: "disable web authentication with localhost binding", ValueHint: registry.FlagValueNone},
	})
	registry.RegisterCommandFlags("config list", []registry.FlagSpec{
		{Name: "--backup", Description: "list the configs inside a backup artifact", ValueHint: registry.FlagValueFile},
	})
	registry.RegisterCommandFlags("config set", []registry.FlagSpec{
		{Name: "--dry-run", Description: "show what would change without writing", ValueHint: registry.FlagValueNone},
		{Name: "--reload", Description: "notify the running daemon to reload after save", ValueHint: registry.FlagValueNone},
		{Name: "--user", Description: "SSH login username (overrides stored super-admin)", ValueHint: registry.FlagValueText},
		{Name: "-u", Description: "short alias for --user", ValueHint: registry.FlagValueText},
		{Name: "--backup", Description: "set the value in a config inside a backup artifact", ValueHint: registry.FlagValueFile},
	})
	registry.RegisterCommandFlags("config show", []registry.FlagSpec{
		{Name: "--backup", Description: "read the config from a backup artifact", ValueHint: registry.FlagValueFile},
	})
}
