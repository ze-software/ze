package env

import "github.com/ze-software/ze/internal/component/command/registry"

func init() {
	registry.MustRegisterRootHandler("env", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		Description: "Environment variable inspection",
		Mode:        "offline",
		Section:     registry.SectionSystem,
		Subs:        "list, get, registered",
	})
	registry.MustRegisterLocal("show env list", func(args []string) int {
		return Run(append([]string{"list"}, args...))
	})
	registry.MustRegisterLocal("show env get", func(args []string) int {
		return Run(append([]string{"get"}, args...))
	})
	registry.MustRegisterLocal("show env registered", func(args []string) int {
		return Run(append([]string{"registered"}, args...))
	})
}
