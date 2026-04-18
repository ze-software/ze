// Register the data root command and its `show data *` offline
// shortcuts with the cmd/ze dispatcher.

package data

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

func init() {
	cmdregistry.RegisterRoot("data", cmdregistry.Meta{
		Description: "ZeFS blob store management",
		Mode:        "offline",
		Subs:        "import, rm, ls, cat",
	})
	cmdregistry.MustRegisterLocal("show data ls", func(args []string) int {
		return Run(append([]string{"ls"}, args...))
	})
	cmdregistry.MustRegisterLocal("show data cat", func(args []string) int {
		return Run(append([]string{"cat"}, args...))
	})
	cmdregistry.MustRegisterLocal("show data registered", func(args []string) int {
		return Run(append([]string{"registered"}, args...))
	})
}
