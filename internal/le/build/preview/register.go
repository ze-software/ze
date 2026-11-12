// Design: docs/contributing/preview-artifacts.md -- local preview preparation.
package buildpreview

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func init() {
	leroot.Register(area, leroot.GroupWorkflow, Answer, registry.Meta{ShortHelp: "prepare and check unpublished preview artifacts", Description: "Prepare identified static Linux amd64 and arm64 archives, optionally export native Docker images, and check their source and binary identity. No release or registry publication occurs.", Mode: "offline", Section: registry.SectionTest, SubsFunc: Subs})
	leroot.RegisterActions(area, Actions)
	leroot.RegisterShape(area, command.ShapeMap)
}
