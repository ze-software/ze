// Design: docs/architecture/testing/ci-format.md -- the harness command `le mock httpd`
//
// One package owns one harness command. Composition imports it from
// internal/le/register.go, and the handler stays in the harness packages.

package mockhttpd

import (
	"github.com/ze-software/ze/internal/component/command"
	leroot "github.com/ze-software/ze/internal/le/le/root"
	"github.com/ze-software/ze/internal/le/test/harnesstool"
	"github.com/ze-software/ze/internal/test/cli"
)

// name is the le command this package registers.
const name = "mock httpd"

func init() {
	leroot.Register(name, leroot.GroupTool, harnesstool.Answer(cli.CmdStaticHTTP), harnesstool.Meta("serve a directory over HTTP for tests"))
	leroot.RegisterShape(name, command.ShapeDoc)

	// Every word after the name is the harness's own command line, so a
	// trailing help word reaches it and it prints its own help.
	leroot.RegisterForwarding(name, harnesstool.Helper)
}
