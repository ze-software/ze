// Register the schema root command and its `show schema *` offline
// shortcuts with the cmd/ze dispatcher.

package schema

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

func init() {
	cmdregistry.RegisterRoot("schema", cmdregistry.Meta{
		Description: "Schema discovery",
		Mode:        "offline",
		Subs:        "list, methods, events, handlers, protocol",
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
