// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package dockerkernel

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	// `le setup docker kernel` is its own area: its install changes a host's
	// boot kernel, so it never shares the `setup install` run (dockerkernel.go).
	leroot.Register(dockerKernelArea, leroot.GroupWorkflow, DockerKernelAnswer, registry.Meta{
		Description: "The Docker daemon's kernel: check it carries every feature Ze's labs need, or install Ze's own runtime kernel and AppArmor profile on a Linux Docker host.",
		ShortHelp:   "check or install Ze's Docker host kernel",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    DockerKernelSubs,
	})
	leroot.RegisterActions(dockerKernelArea, DockerKernelActions)
	leroot.RegisterShape(dockerKernelArea, command.ShapeDoc)

	// No parity.Claim: docker-kernel has no migration-census gate, so it
	// declares none of those rows.
}
