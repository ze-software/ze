// Design: docs/architecture/testing/runner-architecture.md -- the step trace the report prints
// Overview: runner_exec.go -- the command loop that records these steps
// Related: runner_output_assert.go -- recordStep, the assertion half of the same trace

package runner

import (
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/test/trace"
)

// stepKindExec is the trace.StepResult kind for a command the runner RAN, as
// opposed to stepKindExpect, which is an assertion about what it produced.
const stepKindExec = "exec"

// recordExecStep records the command the runner ran: the argv it built, and
// where the named stdin= block went.
//
// The .ci says what to run and the runner can rewrite it. A `ze -` daemon
// launch becomes `ze [flags] start <file>`, and a ze-peer line carrying no `-`
// gains a path argument. Both rewrites are deliberate, and neither appeared
// anywhere an author reads, so a test whose stimulus was replaced looked
// exactly like a test whose stimulus was honored: test/ui/bgp-decode-pcap-stdin.ci
// asserted a decoded capture while the capture reached the child as a file path
// it never wrote, and passed.
//
// The step always passes, because it is an observation rather than an
// assertion. It carries everything in Assert: the human trace prints Detail
// only for a failed step, and this step is what the reader of a PASSING run
// came for (trace.writeHuman).
func (r *Record) recordExecStep(binPath string, args []string, blockName string, piped bool) {
	var b textbuf.Buffer
	b.Str(binPath)
	for _, arg := range args {
		b.Byte(' ').Str(arg)
	}

	b.Str("  [")
	switch {
	case blockName == "":
		b.Str("no stdin block")
	case piped:
		b.Str("stdin=").Str(blockName).Str(" piped")
	default:
		b.Str("stdin=").Str(blockName).Str(" written to a file, named in the argv above")
	}
	b.Byte(']')

	r.StepTrace = append(r.StepTrace, trace.StepResult{
		Step: len(r.StepTrace) + 1, Kind: stepKindExec, Assert: b.String(), Passed: true,
	})
}
