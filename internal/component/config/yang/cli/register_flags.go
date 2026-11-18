// Design: docs/architecture/cli/command-completion.md -- offline YANG tree flags.
// Related: main.go -- parseTreeOptions owns both tree filters.

package cli

import "github.com/ze-software/ze/internal/component/command/registry"

func init() {
	registry.RegisterCommandFlags("yang tree", []registry.FlagSpec{
		{Name: "--commands", Description: "show command nodes only", ValueHint: registry.FlagValueNone},
		{Name: "--config", Description: "show config nodes only", ValueHint: registry.FlagValueNone},
	})
}
