// Design: docs/architecture/testing/ci-format.md -- the harness command `le test dns`
//
// One package owns one harness command. Composition imports it from
// internal/le/register.go, and the handler stays in the harness packages.

package testdns

import (
	"github.com/ze-software/ze/internal/component/command"
	leroot "github.com/ze-software/ze/internal/le/le/root"
	"github.com/ze-software/ze/internal/le/test/harnesstool"
	dnsmock "github.com/ze-software/ze/internal/test/mock/dns"
)

// name is the le command this package registers.
const name = "test dns"

func init() {
	leroot.Register(name, leroot.GroupSuite, harnesstool.Answer(dnsmock.Run), harnesstool.Meta("Deterministic DNS mock server (A/AAAA zone with NODATA, NXDOMAIN, SERVFAIL and REFUSED names)"))
	leroot.RegisterShape(name, command.ShapeDoc)

	// Every word after the name is the harness's own command line, so a
	// trailing help word reaches it and it prints its own help.
	leroot.RegisterForwarding(name)
}
