// Design: docs/architecture/api/commands.md — data (ZeFS) command ownership
//
// Register the `data` root command and its `show data *` offline shortcuts with
// the importable command registry. This is the owner package: the offline ZeFS
// blob-store management CLI lives with internal/component/config/storage, not
// under cmd/ze.
package cli

import "github.com/ze-software/ze/internal/component/command/registry"

func init() {
	registry.MustRegisterRootHandler("data", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		Description: "ZeFS blob store management",
		Mode:        "offline",
		Section:     registry.SectionConfiguration,
		Subs:        "import, rm, ls, cat",
	})
	registry.MustRegisterLocal("show data ls", func(args []string) int {
		return Run(append([]string{"ls"}, args...))
	})
	registry.MustRegisterLocal("show data cat", func(args []string) int {
		return Run(append([]string{"cat"}, args...))
	})
	registry.MustRegisterLocal("show data registered", func(args []string) int {
		return Run(append([]string{"registered"}, args...))
	})
}
