// Design: docs/architecture/testing/ci-format.md -- the harness command `le dns query`
//
// One package owns one harness command. Composition imports it from
// internal/le/register.go, and the handler stays in the harness packages.

package query

import (
	"github.com/ze-software/ze/internal/component/command"
	leroot "github.com/ze-software/ze/internal/le/le/root"
	"github.com/ze-software/ze/internal/le/test/harnesstool"
	"github.com/ze-software/ze/internal/test/dnsquery"
)

// name is the le command this package registers.
const name = "dns query"

func init() {
	leroot.Register(name, leroot.GroupTool, harnesstool.Answer(dnsquery.Run), harnesstool.Meta("send one DNS query and print every reply field"))
	leroot.RegisterShape(name, command.ShapeDoc)

	// Every word after the name is the tool's own command line, so a
	// trailing help word reaches it and it prints its own help.
	leroot.RegisterForwarding(name, harnesstool.Helper)
}
