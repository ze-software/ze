// Register the env root command and its `show env *` offline
// shortcuts with the cmd/ze dispatcher.

package environ

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

func init() {
	cmdregistry.RegisterRoot("env", cmdregistry.Meta{
		Description: "Environment variable inspection",
		Mode:        "offline",
		Subs:        "list, get, registered",
	})
	cmdregistry.MustRegisterLocal("show env list", func(args []string) int {
		return Run(append([]string{"list"}, args...))
	})
	cmdregistry.MustRegisterLocal("show env get", func(args []string) int {
		return Run(append([]string{"get"}, args...))
	})
	cmdregistry.MustRegisterLocal("show env registered", func(args []string) int {
		return Run(append([]string{"registered"}, args...))
	})
}
