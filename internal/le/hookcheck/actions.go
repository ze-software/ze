// Design: docs/architecture/core-design.md -- hook selftests and runtime are one le action area
// Overview: hookcheck.go -- native selftest implementation
package hookcheck

import (
	"bytes"
	"io"
	"os"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/hookruntime"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

const area = "hook-check"

// payloadWait bounds how long a hook verb waits for its JSON payload.
//
// Every verb below is a hook rather than a command: the runner writes one
// payload on stdin and the verb takes no arguments, so a real payload is there
// at once and this bound is never reached. Typed at a shell no payload ever
// comes, and stdin is a socket the harness holds open, so the decode waits on a
// descriptor that will not reach EOF and the process outlives the command that
// started it. Three were found alive at once, aged 59 minutes, 7 hours and 21
// hours (`plan/journal/command-waits-for-input-it-was-not-given.md`).
const payloadWait = 10 * time.Second

var actions = func() leaction.Area {
	verbs := [...]string{
		"session-start", "compaction-reminder", "verify-claim-reminder", categoryDelegationReminder,
		"block-until-lsp", "pretool-bash", "pretool-writeedit", "pretool-agent-skill",
		"pre-compact-save", "block-premature-stop", "rule-coverage-report", "session-end-summary",
		categorySubagentContext, "mark-lsp-invoked", categoryMarkSourceRead,
		"mark-agent-spawned", categoryValidateSpec, "posttool-writeedit", categorySessionID,
	}
	list := make([]leaction.Action, 0, 1+len(verbs))
	list = append(list, leaction.Action{
		Verb:   "unit",
		Why:    "run every hook dispatcher golden row and every behavioral fixture category in-process",
		Answer: runHere,
	})
	for _, verb := range verbs {
		kind := verb
		list = append(list, leaction.Action{
			Verb: kind,
			Why:  "run the " + kind + " hook JSON protocol in the native le process",
			Answer: func() (any, int) {
				in, waited := payloadReader(os.Stdin, payloadWait)
				code := hookruntime.Run(kind, in, os.Stdout, os.Stderr)
				if waited {
					reportNoPayload(kind)
				}
				return nil, code
			},
		})
	}
	return leaction.New(area, list...)
}()

// Actions answers the command surface as structured data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le hook-check` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// payloadReader bounds the wait for the FIRST byte of a hook payload, and
// answers whether that bound was reached rather than a byte.
//
// The bound is a timer over a goroutine, not a read deadline. os.File carries
// SetReadDeadline, and it refuses os.Stdin: the Go runtime never registers the
// standard streams with its poller, because they are shared with the parent
// process. A deadline therefore reports ErrNoDeadline here and bounds nothing,
// which is a fix that passes over os.Pipe and does nothing in the product.
//
// Only the first byte is bounded. A payload that has started arriving is a real
// hook invocation, and the checks it runs take as long as they take.
//
// On the bound, the reading goroutine is left blocked on in. It holds one
// goroutine and the descriptor until the process exits, which is the next thing
// that happens: the caller reports and returns, and this is a one-shot command.
func payloadReader(in io.Reader, wait time.Duration) (io.Reader, bool) {
	head := make([]byte, 1)
	arrived := make(chan int, 1)
	go func() {
		read, _ := io.ReadFull(in, head)
		arrived <- read
	}()

	select {
	case read := <-arrived:
		if read == 0 {
			// EOF or a read error, which is an absent payload rather than an
			// unending wait. hookruntime.Run already decides what each hook
			// kind does with one, so hand it the empty stream it expects.
			return bytes.NewReader(nil), false
		}
		return io.MultiReader(bytes.NewReader(head[:read]), in), false
	case <-time.After(wait):
		return bytes.NewReader(nil), true
	}
}

// reportNoPayload names the mistake that reaches this line: the verb was typed
// as a command. It says what the verb is and what it wanted, because the reader
// has just watched it do nothing.
func reportNoPayload(kind string) {
	var tb textbuf.Buffer
	tb.Str(area).Byte(' ').Str(kind)
	tb.Str(": no hook payload arrived on stdin in ").Int(int64(payloadWait / time.Second))
	tb.Str(" seconds, so NOTHING WAS CHECKED.\n")
	tb.Str("  This is a hook, not a command. The hook runner writes a JSON payload on stdin, and the verb takes no arguments.\n")
	_ = tb.StdErr()
}

func runHere() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, code := Run(root)
	return report, code
}
