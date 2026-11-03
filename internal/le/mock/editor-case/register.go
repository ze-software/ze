// Design: docs/architecture/testing/runner-architecture.md -- owned editor child.

package mockeditorcase

import (
	"github.com/ze-software/ze/internal/component/command"
	leroot "github.com/ze-software/ze/internal/le/le/root"
	"github.com/ze-software/ze/internal/le/test/harnesstool"
	"github.com/ze-software/ze/internal/test/editorcase"
)

const name = "mock editor-case"

func init() {
	leroot.Register(name, leroot.GroupTool, harnesstool.Answer(editorcase.Main), harnesstool.Meta("run one owned editor case over inherited descriptors"))
	leroot.RegisterShape(name, command.ShapeDoc)
	leroot.RegisterHidden(name)
	leroot.RegisterForwarding(name, harnesstool.Helper)
}
