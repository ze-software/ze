// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package dockerhost

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	// `le setup docker-host` is its own area: the image build and the Colima
	// lifecycle hold their own lock and Mac-only steps (spec D-8).
	leroot.Register(dockerHostArea, leroot.GroupWorkflow, DockerHostAnswer, registry.Meta{
		ShortHelp: "build Ze's docker-host image and run it as the `ze` Colima profile, the macOS lab Docker host",
		Mode:      "offline",
		Section:   registry.SectionTest,
		SubsFunc:  DockerHostSubs,
	})
	leroot.RegisterActions(dockerHostArea, DockerHostActions)
	leroot.RegisterShape(dockerHostArea, command.ShapeDoc)
}
