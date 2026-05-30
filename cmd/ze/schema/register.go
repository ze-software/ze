// Register the schema root command and its `show schema *` offline
// shortcuts with the cmd/ze dispatcher.

package schema

import (
	"sort"
	"strings"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

// schemaCommands lists the user-facing subcommand names, kept in sync
// with the switch cases in Run (main.go). "help" and "show" (which
// takes a module argument) are excluded from Meta.Subs because "show"
// is accessed via "ze schema show <module>" and help is universal.
var schemaCommands = []string{"list", "methods", "events", "handlers", "protocol"}

// subcommands returns the sorted, comma-separated list of user-facing
// subcommands (single source of truth for Meta.Subs and error messages).
func subcommands() string {
	sorted := make([]string, len(schemaCommands))
	copy(sorted, schemaCommands)
	sort.Strings(sorted)
	return strings.Join(sorted, ", ")
}

func init() {
	cmdregistry.RegisterRoot("schema", cmdregistry.Meta{
		Description: "Schema discovery",
		Mode:        "offline",
		Section:     cmdregistry.SectionConfiguration,
		Subs:        subcommands(),
	})
	cmdregistry.MustRegisterLocal("show schema list", func(args []string) int {
		return Run(append([]string{"list"}, args...), nil)
	})
	cmdregistry.MustRegisterLocal("show schema methods", func(args []string) int {
		return Run(append([]string{"methods"}, args...), nil)
	})
	cmdregistry.MustRegisterLocal("show schema events", func(args []string) int {
		return Run(append([]string{"events"}, args...), nil)
	})
	cmdregistry.MustRegisterLocal("show schema handlers", func(args []string) int {
		return Run(append([]string{"handlers"}, args...), nil)
	})
	cmdregistry.MustRegisterLocal("show schema protocol", func(_ []string) int {
		return Run([]string{"protocol"}, nil)
	})
}
