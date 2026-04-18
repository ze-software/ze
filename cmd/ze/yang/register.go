// Register the yang root command and its `show yang *` offline
// shortcuts with the cmd/ze dispatcher.

package yang

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

func init() {
	cmdregistry.RegisterRoot("yang", cmdregistry.Meta{
		Description: "YANG tree analysis",
		Mode:        "offline",
		Subs:        "tree, completion, doc",
	})
	cmdregistry.MustRegisterLocal("show yang tree", func(args []string) int {
		return Run(append([]string{"tree"}, args...))
	})
	cmdregistry.MustRegisterLocal("show yang completion", func(args []string) int {
		return Run(append([]string{"completion"}, args...))
	})
	cmdregistry.MustRegisterLocal("show yang doc", func(args []string) int {
		return Run(append([]string{"doc"}, args...))
	})
}
