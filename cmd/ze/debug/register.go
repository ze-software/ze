// Design: docs/architecture/zefs-format.md -- debug flags stored as zefs keys
// Overview: debug.go -- Run handler for enable/disable/show

package debug

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

func init() {
	cmdregistry.RegisterRoot("debug", cmdregistry.Meta{
		Description: "Runtime debug flags (persistent, stored in ZeFS)",
		Mode:        "offline",
		Section:     cmdregistry.SectionOperations,
		Subs:        "enable, disable, show",
	})
	cmdregistry.MustRegisterLocal("debug enable", func(args []string) int {
		return Run(append([]string{"enable"}, args...))
	})
	cmdregistry.MustRegisterLocal("debug disable", func(args []string) int {
		return Run(append([]string{"disable"}, args...))
	})
	cmdregistry.MustRegisterLocal("debug show", func(args []string) int {
		return Run(append([]string{"show"}, args...))
	})
}
