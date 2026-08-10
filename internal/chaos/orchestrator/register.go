// Design: docs/architecture/chaos-web-dashboard.md -- chaos root handler registration

package orchestrator

import "github.com/ze-software/ze/internal/component/command/registry"

func init() {
	registry.MustRegisterRootHandler("chaos", func(_ *registry.RuntimeContext, args []string) int {
		return cLIRun(args)
	}, registry.Meta{
		Description: "Chaos monkey for BGP testing",
		Mode:        "offline",
		Section:     registry.SectionTest,
	})
}
