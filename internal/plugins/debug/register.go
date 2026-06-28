// Design: plan/learned/891-granular-debug.md -- debug CLI registration
// Related: debug.go -- Run handler

// codegen:skip -- CLI command wired via cmd/ze/main.go, not a runtime plugin.

package debug

import (
	"codeberg.org/thomas-mangin/ze/internal/component/command/registry"
)

func init() {
	registry.MustRegisterRootHandler("debug", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		Description: "Granular debug with toggle semantics and named profiles (stored in debug.zefs)",
		Mode:        "offline",
		Section:     registry.SectionOperations,
		Subs:        "show, restore, clear, profile, timeout, <module>",
	})
	registry.MustRegisterLocalMeta("debug show", func(args []string) int {
		return Run(append([]string{"show"}, args...))
	}, registry.Meta{Description: "Show active debug state (module, level, flags, scopes)."})
	registry.MustRegisterLocalMeta("debug clear", func(args []string) int {
		return Run(append([]string{"clear"}, args...))
	}, registry.Meta{Description: "Clear default debug profile."})
	registry.MustRegisterLocalMeta("debug restore", func(args []string) int {
		return Run(append([]string{"restore"}, args...))
	}, registry.Meta{Description: "Load and apply saved debug profile."})
	registry.MustRegisterLocalMeta("debug profile", func(args []string) int {
		return Run(append([]string{"profile"}, args...))
	}, registry.Meta{Description: "Manage named debug profiles (save/list/delete)."})
}
