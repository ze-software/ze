// Design: docs/architecture/testing/ci-format.md -- the harness command `le test interop-bgp`
//
// One package owns one harness command. Composition imports it from
// internal/le/register.go, and the handler stays in the harness packages.

package testinteropbgp

import (
	"github.com/ze-software/ze/internal/component/command"
	interopbgp "github.com/ze-software/ze/internal/le/interoplab/bgp"
	"github.com/ze-software/ze/internal/le/le/root"
	"github.com/ze-software/ze/internal/le/test/harnesstool"
)

// name is the le command this package registers.
const name = "test interop-bgp"

func init() {
	leroot.Register(name, leroot.GroupSuite, harnesstool.Answer(interopbgp.Helper), harnesstool.Meta("Compiled BGP interop process, speaker, and collector personalities"))
	leroot.RegisterShape(name, command.ShapeDoc)

	// Every word after the name is the harness's own command line, so a
	// trailing help word reaches it and it prints its own help.
	leroot.RegisterForwarding(name)
}
