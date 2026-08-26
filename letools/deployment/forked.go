// Design: docs/architecture/core-design.md -- a gate whose work is another program's
// Overview: actions.go -- the table these rows sit in
//
// forked.go runs the deployment proofs whose driver is still a script. Five
// gates in this family use Python drivers of 500 to 1500 lines apiece. Each one
// requires its own port. Until then, the AREA is ported but the DRIVER is not.
// Thus, `le deployment vpp-test` starts the same process that
// `make ze-deployment-vpp-test` started and answers the same code.
//
// The gates are ROWS OF THIS TABLE because the gate name identifies the family.
// Every ze-deployment- gate belongs here. A row in two areas would be the drift
// that the migration's parity gate exists to prevent. Two of the five drive a
// lab under test/, which is outside this spec's scope. They remain rows here
// after their siblings under scripts/ are ported.

package deployment

import (
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/letools/gaterun"
	"github.com/ze-software/ze/letools/gotoolchain"
	"github.com/ze-software/ze/letools/leaction"
	"github.com/ze-software/ze/letools/lepath"
)

// python is the interpreter every driver in this file is started with.
const python = "python3"

// forkedAction answers one row whose work is another program's.
//
// The argv is stated ONCE and used twice. The row runs it, and Forks publishes
// it so letools/parity counts this gate as claimed rather than converted. Two
// statements of one argv are exactly the drift the census exists to detect. The
// row cannot declare one command and run another.
func forkedAction(gate, why string, argv []string) leaction.Action {
	return leaction.Action{Gate: gate, Why: why, Forks: argv, Answer: forked(gate, argv...)}
}

// forked answers the action that runs one driver over the checkout in which this
// command runs.
//
// Each action loads the toolchain. The area does not load it once for all
// actions. A listing must not pay for a go.mod read and a feature-manifest read.
// The commands `le deployment` with no verb and `le --help`, as well as
// completion, all build this table.
func forked(gate string, argv ...string) func() (any, int) {
	return func() (any, int) {
		root, err := lepath.Root()
		if err != nil {
			leaction.ReportError(err)
			return nil, 1
		}
		tc, err := gotoolchain.New(root)
		if err != nil {
			leaction.ReportError(err)
			return nil, 1
		}
		return gaterun.Run(gate, argv, root, tc.Environment(gotoolchain.EnvOptions{}))
	}
}

// evidenceDriver answers the argv of one scripts/evidence/ driver.
func evidenceDriver(script string) []string {
	var tb textbuf.Buffer
	return []string{python, tb.Str("scripts/evidence/").Str(script).String()}
}

// labRunner answers the argv of one lab under test/, whose runner this spec's
// scope does not reach.
func labRunner(lab string) []string {
	var tb textbuf.Buffer
	return []string{python, tb.Str("test/").Str(lab).Str("/run.py").String()}
}
