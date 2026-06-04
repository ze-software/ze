// Design: docs/architecture/zefs-format.md -- debug flags stored as zefs keys
// Overview: debug.go -- Run handler for enable/disable/show

// codegen:skip -- CLI command wired via cmd/ze/main.go, not a runtime plugin.

package debug

import (
	"codeberg.org/thomas-mangin/ze/internal/component/command/registry"
)

func init() {
	registry.MustRegisterRootHandler("debug", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		Description: "Toggle persistent debug flags (stored in ZeFS, survive restarts)",
		Mode:        "offline",
		Section:     registry.SectionOperations,
		Subs:        "enable, disable, show",
	})
	registry.MustRegisterLocalMeta("debug enable", func(args []string) int {
		return Run(append([]string{"enable"}, args...))
	}, registry.Meta{Description: "Turn on a debug flag. Persisted in ZeFS so it survives restarts."})
	registry.MustRegisterLocalMeta("debug disable", func(args []string) int {
		return Run(append([]string{"disable"}, args...))
	}, registry.Meta{Description: "Turn off a debug flag and remove it from ZeFS."})
	registry.MustRegisterLocalMeta("debug show", func(args []string) int {
		return Run(append([]string{"show"}, args...))
	}, registry.Meta{Description: "List all debug flags currently enabled."})
}
