// Design: docs/architecture/testing/ci-format.md -- the harness area `le test wire`
//
// One package owns one harness command. Composition imports it from
// internal/le/register.go, and the handler stays in the harness packages.

// Package testwire registers `le test wire`, the area that groups the offline
// wire-decode suites. Its first word picks the suite: `le test wire isis -a`.
// Each suite keeps its own test directory and its own runner configuration.
package testwire

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/le/le/root"
	"github.com/ze-software/ze/internal/le/test/harnesstool"
	"github.com/ze-software/ze/internal/test/cli"
)

// name is the le command this package registers.
const name = "test wire"

// suites are the members of the area, in listing order. A suite's Name is
// what its usage prints after `le test `, so it carries the area word.
var suites = []cli.CIRunnerConfig{
	{
		Name:        "wire isis",
		TestSubdir:  "isis-wire",
		Description: "IS-IS wire-level decode",
		Detail:      "Run IS-IS wire-level functional tests (.ci files in test/isis-wire/).\nCovers offline PDU decode: ze isis decode parses a hex IS-IS PDU to JSON.",
	},
	{
		Name:        "wire ospf",
		TestSubdir:  "ospf-wire",
		Description: "OSPFv2 wire-level decode",
		Detail:      "Run OSPFv2 wire-level functional tests (.ci files in test/ospf-wire/).\nCovers offline packet decode: ze ospf decode parses hex OSPFv2 packets to JSON.",
	},
	{
		Name:        "wire l2tp",
		TestSubdir:  "l2tp-wire",
		Description: "L2TP wire-level decode",
		Detail:      "Run L2TP wire-level functional tests (.ci files in test/l2tp-wire/).\nCovers control message decode (SCCRQ) and truncated packet handling.",
	},
}

// members answers the area members, one per suite. The word is the suite
// name after the area word.
func members() []harnesstool.AreaMember {
	out := make([]harnesstool.AreaMember, 0, len(suites))
	for _, suite := range suites {
		out = append(out, harnesstool.SuiteMember(suite.Name[len("wire "):], suite))
	}
	return out
}

func init() {
	// The area runs the functional runner, so it admits its run on the host
	// (harnesstool.RunnerAnswer), whichever suite the first word picks.
	answer := harnesstool.RunnerAnswer(name, harnesstool.Area(name, members()))
	leroot.Register(name, leroot.GroupSuite, answer, harnesstool.Meta("Run a wire-level decode suite: isis, ospf or l2tp"))
	leroot.RegisterShape(name, command.ShapeDoc)

	// Every word after the name reaches the area, so the suite word and a
	// trailing help word reach the harness and it prints its own help.
	leroot.RegisterForwarding(name)
}
