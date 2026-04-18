// Register the bgp root command and its `show bgp decode` / `show bgp
// encode` offline shortcuts with the cmd/ze dispatcher. Imported by
// cmd/ze/main.go for its side effects.

package bgp

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

func init() {
	cmdregistry.RegisterRoot("bgp", cmdregistry.Meta{
		Description: "BGP protocol tools",
		Mode:        "offline",
		Subs:        "decode <hex>, encode <route>, plugin",
	})
	cmdregistry.MustRegisterLocal("show bgp decode", func(args []string) int {
		return Run(append([]string{"decode"}, args...))
	})
	cmdregistry.MustRegisterLocal("show bgp encode", func(args []string) int {
		return Run(append([]string{"encode"}, args...))
	})
}
