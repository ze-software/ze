// Design: docs/architecture/core-design.md -- le's command surface as one tree
// Related: describe_test.go -- the test a text fault narrows the run to

package le

import (
	"flag"
	"testing"

	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// textFaultTest is the one test that reports every text fault without
// building le's tree.
const textFaultTest = "^TestEveryWordLeOffersSaysWhatItDoes$"

// TestMain narrows the run to textFaultTest when a registered area carries a
// text fault (leroot.TextFaults). Every other test here that builds le's tree,
// completes or dispatches runs the startup check, whose BUG panic stops the
// whole binary at the first such test with one fault and a stack trace.
// Narrowed, the run fails with one error per fault, naming the item and its
// package. With no fault the run is the one the flags asked for.
func TestMain(m *testing.M) {
	flag.Parse()
	if len(leroot.TextFaults(leroot.Commands())) > 0 {
		if err := flag.Set("test.run", textFaultTest); err != nil {
			panic("BUG: le test flag test.run is not registered: " + err.Error())
		}
	}
	m.Run()
}
