// Design: docs/architecture/core-design.md -- admission for heavy jobs in a shared checkout
// Overview: job.go -- the admission this entry point claims through
// Related: registry.go -- take, which writes an unshared claim with no log
// Related: register.go -- where admitCentral fills the dispatcher's seam
// Related: ../le/root/admitter.go -- the seam, and the order the dispatcher asks in
//
// Every le line that declares itself heavy takes one job slot before its body
// runs, unless its own declaration says otherwise (leroot.SlotOfLine). The
// dispatcher cannot name this package, which imports it, so it calls the
// function this file registers.
//
// A centrally admitted action never agreed to anything about its run: not that
// another run's verdict is its own, and not to write a progress log. Its claim
// is therefore unshared (Admission.Unshared). A waiter and `le job report` see
// who holds the slot, under which command line, and no stage.

package job

import (
	"context"
	"errors"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/gaterun"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// The codes a central line answers when its body did not run.
const (
	// centralRefused is a line whose slot could not be asked for.
	centralRefused = 2
	// centralInterrupted is a wait for a slot that SIGINT or SIGTERM ended,
	// the code a shell gives a command an interrupt stopped.
	centralInterrupted = 130
)

// centralLabelOf answers the job label of a centrally admitted line: its
// command name with every byte outside the label alphabet as a hyphen, so
// `test integration` holds its slot as `test-integration`. A command name is
// letters, digits, hyphens and spaces, and any other byte is replaced the same
// way, so the answer is a valid label for every name but the empty one.
func centralLabelOf(commandName string) string {
	label := []byte(commandName)
	for i, c := range label {
		if !validLabel(string(c)) {
			label[i] = '-'
		}
	}
	return string(label)
}

// NameParent names entry, a registry entry's absolute path, as the job every
// process this one starts runs inside, and answers the function that puts the
// previous value back.
//
// It writes the process environment, so a child inherits it whether its
// environment is this process's or a copy taken after this call. A child that
// finds the entry answers KindInside and never waits for the slot its own
// parent holds (insideParent).
//
// The entry is written under two spellings, ParentKey and ParentVariable, and
// every other spelling is removed (env.Set). The dotted one is what env.Get
// prefers in this process and in a child le started directly. The underscore
// one is the only one that reaches a child started through a shell: the `le`
// launcher and every tool shim are /bin/sh scripts, and dash hands on only the
// variables whose names are shell identifiers, so `test qemu docker-lab`
// starting `le test integration` on the host would otherwise wait for the slot
// its own parent holds.
//
// childEnviron strips every spelling but one because a child that keeps an
// older dotted value beside a newer underscore one reads the older. That
// cannot arise here: both spellings are written by this one call with one
// value and no third spelling survives it, so a reader gets this entry
// whichever spelling it prefers, and a Run below still hands its own child
// one spelling.
//
// The restore function puts the environment back as it was: each spelling the
// key had, with its value and in its order, and no spelling it did not have,
// so a spelling that was unset is unset again. A caller MUST call it once its
// nested work has ended, and before it releases the slot the entry is. NOT
// safe for concurrent use: the environment is the process's.
func NameParent(entry string) (restore func() error, err error) {
	previous := env.Get(ParentKey)
	spellings := parentSpellings(os.Environ())
	restore = func() error { return restoreParent(previous, spellings) }

	if err := env.Set(ParentKey, entry); err != nil {
		return nil, errors.Join(err, restore())
	}
	if err := os.Setenv(ParentVariable, entry); err != nil {
		return nil, errors.Join(err, restore())
	}
	return restore, nil
}

// parentSpellings answers the entries of environ that are a spelling of
// ParentKey, as env.Get reads one, in environ's order.
func parentSpellings(environ []string) []string {
	key := env.Normalize(ParentKey)
	var spellings []string
	for _, entry := range environ {
		name, _, _ := strings.Cut(entry, "=")
		if env.Normalize(name) == key {
			spellings = append(spellings, entry)
		}
	}
	return spellings
}

// restoreParent undoes NameParent: previous is the value env.Get read before
// it, and spellings the entries the process environment held then.
//
// env.Set comes first because it is what puts previous back in env's cache; it
// also removes every spelling but the dotted one. The dotted one is then
// removed too and the saved entries are written in their order, so the
// spellings regain the order env.Get's "last entry wins" reads them in.
func restoreParent(previous string, spellings []string) error {
	if err := env.Set(ParentKey, previous); err != nil {
		return err
	}
	if err := os.Unsetenv(ParentKey); err != nil {
		return err
	}
	for _, entry := range spellings {
		name, value, _ := strings.Cut(entry, "=")
		if err := os.Setenv(name, value); err != nil {
			return err
		}
	}
	return nil
}

// Hold runs body inside one unshared job slot and answers body's code.
//
// It waits for the slot as Admit does, under ctx: a done ctx ends the wait
// with ctx's error. Inside a parent's slot it claims nothing and runs body
// there. Otherwise it claims, names its entry as the parent of every process
// body starts (NameParent), runs body, restores the previous parent and
// releases the slot with body's code, exactly once.
//
// A non-nil error means body did not run, and the code beside it is
// gaterun.CannotStart. A panic in body releases the slot with that code and
// restores the parent before it continues, so a crashed action leaves no slot
// held by a process that is still alive.
//
// label MUST be a valid label, and argv is the line the entry and the waiter
// record show. body MUST NOT be nil.
func (a *Admission) Hold(ctx context.Context, label string, argv []string, body func() int) (int, error) {
	// A copy, so the caller's policy is not left unshared for its next claim.
	held := *a
	held.Unshared = true

	ticket, err := held.admit(ctx, label, argv)
	if err != nil {
		return gaterun.CannotStart, err
	}

	switch ticket.Kind {
	case KindInside:
		return body(), nil
	case KindClaimed:
		return held.holdClaimed(ticket, body)
	case KindUnspecified, KindAttached, KindUnadmitted:
		panic("BUG: an unshared job admission answered a ticket of kind " + ticket.Kind.String())
	default:
		panic("BUG: job admission returned an unknown ticket kind")
	}
}

// holdClaimed is Hold once the slot is claimed: the parent naming, the body
// and the one release.
func (a *Admission) holdClaimed(ticket *Ticket, body func() int) (code int, err error) {
	restore, err := NameParent(a.abs(ticket.Entry))
	if err != nil {
		ticket.Release(gaterun.CannotStart)
		return gaterun.CannotStart, opererr.Wrap(err,
			"the job slot "+ticket.Entry+" could not be named as the parent of the work it holds",
			"a child le that cannot see its parent's slot waits for a slot of its own, and deadlocks when the parent holds the last one",
			"run ./le job report, and run the command again once the environment of this shell can be written")
	}

	// Deferred, so a panic in body still releases: code is then the
	// cannot-start code it was given here, and the panic continues after.
	code = gaterun.CannotStart
	defer func() {
		if restoreErr := restore(); restoreErr != nil {
			var tb textbuf.Buffer
			a.note(tb.Byte('[').Str(ticket.Label).Str("] ").Str(ParentKey).
				Str(" could not be restored, so later work in this process names a slot that is gone: ").
				Err(restoreErr).String())
		}
		ticket.Release(code)
	}()

	code = body()
	return code, nil
}

// centralPoint is the central admitter with what a test replaces: where the
// checkout is, the policy for it, the cancellation root of the wait, and where
// a refusal is written.
type centralPoint struct {
	root   func() (string, error)
	policy func(root string) (*Admission, error)
	// waitRoot answers the root that ends the wait for a slot, and the
	// function that removes it.
	waitRoot func() (context.Context, context.CancelFunc)
	errOut   io.Writer
}

// admitCentral is the admitter the dispatcher calls for every line it reads
// as centrally admitted (leroot.Admitter, registered in register.go). It
// answers the exit code of the line.
//
// Where no checkout exists there is no registry to share, so the body runs
// unadmitted, as a suite runner does there. Every other failure to ask for a
// slot refuses the line with an operator error and code 2, and the body does
// not run: a policy the environment states out of range, a registry that
// cannot be listed or locked, a parent entry that cannot be read.
//
// While every slot is held the line waits as `le job run` waits: its waiter
// record is in `le job report`, and the banner on stderr names the holder.
// SIGINT or SIGTERM ends that wait with code 130 and the body does not run.
// The signal root is removed before the body starts, so the body's own signal
// handling is what a later interrupt meets.
func admitCentral(commandName string, line []string, body func() int) int {
	point := centralPoint{root: lepath.Root, policy: NewIn, waitRoot: signalRoot, errOut: os.Stderr}
	return point.admit(commandName, line, body)
}

// signalRoot answers a root that SIGINT or SIGTERM ends.
func signalRoot() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

// admit is admitCentral over the point's sources.
func (c centralPoint) admit(commandName string, line []string, body func() int) int {
	typed := strings.Join(line, " ")

	root, err := c.root()
	if errors.Is(err, lepath.ErrNoCheckout) {
		return body()
	}
	if err != nil {
		return c.refuse(opererr.Wrap(err,
			typed+": the Ze checkout this command runs in could not be resolved",
			"a heavy action takes a job slot in the checkout's "+JobsDir+", so it was not started",
			"unset ZE_REPO_ROOT, cd into the Ze checkout and run the command again"))
	}

	adm, err := c.policy(root)
	if err != nil {
		return c.refuse(opererr.Wrap(err,
			typed+": the job admission policy in the environment is out of range",
			"a heavy action takes one of the job slots that policy counts, so it was not started",
			"unset ZE_RUN_SLOTS ZE_JOB_STALL_SECONDS and run the command again"))
	}

	label := centralLabelOf(commandName)
	if !validLabel(label) {
		panic("BUG: job: the dispatcher admitted a line that resolved to no command name")
	}

	ctx, stop := c.waitRoot()
	defer stop()

	interrupted := false
	code, err := adm.Hold(ctx, label, line, func() int {
		// The signal root exists for the wait alone. A signal that arrived
		// between the claim and this point still ends the line here.
		if ctx.Err() != nil {
			interrupted = true
			return centralInterrupted
		}
		stop()
		return body()
	})
	if err == nil {
		if interrupted {
			adm.noteWaitInterrupted(label)
		}
		return code
	}
	if ctx.Err() != nil {
		// The body's wrapper never ran, so only a signal ended this root.
		adm.noteWaitInterrupted(label)
		return centralInterrupted
	}
	return c.refuse(opererr.Wrap(err,
		typed+": its job slot could not be asked for",
		"a heavy action runs only inside a slot of the job registry "+JobsDir+", which could not be read or written, so it was not started",
		"run ./le job report to see what blocks the registry, restore read and write access to "+JobsDir+", and run the command again"))
}

// refuse writes one refusal and answers the code of a line that did not run.
func (c centralPoint) refuse(refusal *opererr.Error) int {
	if c.errOut != nil {
		var tb textbuf.Buffer
		io.WriteString(c.errOut, tb.Str("error: ").Err(refusal).Byte('\n').String()) //nolint:errcheck // CLI output
	}
	return centralRefused
}

// noteWaitInterrupted tells the person watching that the wait for a slot was
// ended and nothing ran.
func (a *Admission) noteWaitInterrupted(label string) {
	var tb textbuf.Buffer
	a.note(tb.Byte('[').Str(label).
		Str("] stopped by an interrupt or SIGTERM while waiting for a job slot: nothing ran").String())
}
