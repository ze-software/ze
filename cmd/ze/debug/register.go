// Design: docs/architecture/zefs-format.md -- debug flags stored as zefs keys
// Overview: debug.go -- Run handler for enable/disable/show

package debug

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

func init() {
	cmdregistry.RegisterRoot("debug", cmdregistry.Meta{
		Description: "Toggle persistent debug flags (stored in ZeFS, survive restarts)",
		Mode:        "offline",
		Section:     cmdregistry.SectionOperations,
		Subs:        "enable, disable, show",
	})
	cmdregistry.MustRegisterLocalMeta("debug enable", func(args []string) int {
		return Run(append([]string{"enable"}, args...))
	}, cmdregistry.Meta{Description: "Turn on a debug flag. Persisted in ZeFS so it survives restarts."})
	cmdregistry.MustRegisterLocalMeta("debug disable", func(args []string) int {
		return Run(append([]string{"disable"}, args...))
	}, cmdregistry.Meta{Description: "Turn off a debug flag and remove it from ZeFS."})
	cmdregistry.MustRegisterLocalMeta("debug show", func(args []string) int {
		return Run(append([]string{"show"}, args...))
	}, cmdregistry.Meta{Description: "List all debug flags currently enabled."})
}
