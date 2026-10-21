// Design: docs/architecture/testing/qemu-integration.md -- stopping a guest command
// Overview: run_exec.go -- the bounded VM lifecycle that runs this command
//
// A guest command is stopped the way a terminal stops a command on a host: its
// process group receives SIGINT, and the command runs its own teardown. Ending
// the ssh session or the VM under it is what happens only when that teardown
// does not finish.
package testqemu

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// guestCommandGroupFile is where the guest command's shell writes its
	// process group at startup, so the interrupt a second ssh session sends
	// signals that group.
	guestCommandGroupFile = "/tmp/ze-guest-command.pgid"
	// sshExitTransport is the status ssh answers when the connection failed,
	// whatever the remote command did.
	sshExitTransport = 255
	// guestTeardownGrace bounds the wait for the guest command to finish its
	// own teardown after the run forwards it SIGINT. A lab removes its
	// containers, networks and images in seconds; this is the bound, not the
	// expectation.
	guestTeardownGrace = 2 * time.Minute
	// guestInterruptTimeout bounds the ssh session that delivers the SIGINT.
	guestInterruptTimeout = 30 * time.Second
	// sshProgram is the ssh client every guest session runs.
	sshProgram = "ssh"
)

// guestOutcome is how the guest command's ssh session ended.
type guestOutcome struct {
	code int
	err  error
}

// guestCommandBody is the shell body the guest command runs. It traps SIGINT
// first, then records its process group at groupFile for guestInterruptRemote:
// field 5 of /proc/<pid>/stat is the group, and sshd starts the session's
// command as the leader of a session of its own, so that group holds the guest
// command and its children and nothing else. A shell that traps SIGINT waits
// for its foreground command (POSIX, "trap") and then runs the trap, which
// exits with that command's status: the host reads the guest le's exit 1
// after its removals rather than the signal, and no later step of the line
// runs after the interrupted one (busybox ash 1.36, checked 2026-10-21). An
// EXIT trap the command sets still runs on that exit. The trap comes first so
// a SIGINT forwarded as soon as the group file exists lands under it. The
// braces keep a failed or interrupted setup from falling through to the
// second step of a command that is itself a list; the newline before the
// closing brace accepts a command that ends in a semicolon.
func guestCommandBody(groupFile, setup, command string) string {
	var b textbuf.Buffer
	return b.Str("trap 'exit $?' INT; cut -d' ' -f5 /proc/$$/stat > ").Str(groupFile).Str("; ").Str(setup).
		Str(" && {\n").Str(command).Str("\n}").String()
}

// guestInterruptRemote is the shell line a second ssh session runs to send
// SIGINT to the process group recorded at groupFile, as a terminal's Ctrl-C
// reaches every process of the foreground group. It prints the group it
// signaled. Busybox's kill reads every operand after the signal as a number
// and refuses "--" ("sh: invalid number '--'", first guest run, 2026-10-21),
// so the negative group follows the signal directly.
func guestInterruptRemote(groupFile string) string {
	var b textbuf.Buffer
	return b.Str("pgid=$(cat ").Str(groupFile).Str(") && kill -INT -$pgid && echo $pgid").String()
}

// runGuestCommand runs the plan's command in the guest and answers its exit
// status as the verdict. When the run's signal root ends or the command's
// timeout passes while it runs, stopGuestCommand forwards SIGINT to it rather
// than ending its ssh session, because a guest le whose session ends dies of
// SIGPIPE on its next write and never runs its teardown.
func (r *Run) runGuestCommand(ctx context.Context, report *RunReport) (RunReport, error) {
	var b textbuf.Buffer
	b.Str("  running: ").Str(r.Options.Command).Byte('\n').StdErr() //nolint:errcheck // progress output
	body := guestCommandBody(guestCommandGroupFile, report.Plan.SetupCommand, r.Options.Command)
	b.Reset()
	remote := b.Str("sh -c ").Str(shellQuote(body)).String()

	// The session outlives the root: only stopGuestCommand ends it.
	session, endSession := context.WithCancel(context.WithoutCancel(ctx))
	defer endSession()
	finished := make(chan guestOutcome, 1)
	go r.awaitGuestCommand(session, &report.Plan, remote, finished)

	limit, stopLimit := context.WithTimeout(ctx, r.Options.Timeout)
	defer stopLimit()
	select {
	case outcome := <-finished:
		return r.guestCommandEnded(report, outcome)
	case <-limit.Done():
	}
	// A command that ended as the root did needs no interrupt: its outcome
	// is the answer, and a forward to a group that is gone would fail.
	select {
	case outcome := <-finished:
		return r.guestCommandEnded(report, outcome)
	default:
	}
	cause := "an interrupt, SIGTERM or SIGHUP"
	if ctx.Err() == nil {
		cause = "the " + r.Options.Timeout.String() + " command timeout"
	}
	return r.stopGuestCommand(ctx, report, cause, finished, endSession)
}

// guestCommandEnded answers the verdict of a guest command that ended on its
// own: its exit status, or the ssh error that ended its session.
func (r *Run) guestCommandEnded(report *RunReport, outcome guestOutcome) (RunReport, error) {
	if outcome.err != nil {
		return *report, outcome.err
	}
	report.GuestExitCode = outcome.code
	if outcome.code == 0 {
		report.Verdict = RunVerdictPass
		return *report, nil
	}
	report.Verdict = RunVerdictFail
	return *report, nil
}

// awaitGuestCommand is the process-wait bridge of the guest command's ssh
// session: it runs the session to its end and sends the outcome, once.
func (r *Run) awaitGuestCommand(session context.Context, plan *RunPlan, remote string, finished chan<- guestOutcome) {
	code, err := r.sshSession(session, plan, remote, true)
	finished <- guestOutcome{code: code, err: err}
}

// stopGuestCommand forwards SIGINT to the guest command, then waits for the
// command to end on its own, for at most the teardown grace. A second SIGINT
// or SIGTERM stops that wait at once. Every path that does not see the
// command end ends its session and waits for it, so the caller's stopVM runs
// after the guest command, never under it. It always answers an error: an
// interrupted run is never a pass.
func (r *Run) stopGuestCommand(ctx context.Context, report *RunReport, cause string,
	finished <-chan guestOutcome, endSession context.CancelFunc,
) (RunReport, error) {
	again, release := r.ops.Interrupts()
	defer release()
	// The verdict stays unassigned until the guest command answers an exit
	// status: a FAIL would render "exit code 0" for a command nobody heard end.
	var note textbuf.Buffer
	note.Str("  ").Str(cause).Str(": forwarding SIGINT to the guest command so its own teardown runs (waiting up to ").
		Str(r.teardownGrace.String()).Str("; interrupt again to stop the VM now)\n").StdErr() //nolint:errcheck // progress output

	delivered, err := r.interruptGuest(ctx, &report.Plan)
	if err != nil {
		endSession()
		<-finished
		return *report, fmt.Errorf("%s stopped the run, and le could not forward SIGINT to the guest command (%w), "+
			"so the VM was stopped under it and its own teardown never ran", cause, err)
	}
	note.Reset()
	note.Str("  SIGINT sent to guest process group ").Str(delivered).Byte('\n').StdErr() //nolint:errcheck // progress output

	grace := time.NewTimer(r.teardownGrace)
	defer grace.Stop()
	select {
	case outcome := <-finished:
		if outcome.err != nil {
			return *report, fmt.Errorf("%s stopped the run, and the guest command's ssh session failed after the forwarded SIGINT: %w", cause, outcome.err)
		}
		if outcome.code == sshExitTransport {
			// The verdict stays unassigned: 255 is ssh's own status, so
			// nobody heard how the guest command ended.
			return *report, fmt.Errorf("%s stopped the run, and the guest command's ssh failed (exit %d) after the forwarded SIGINT, "+
				"so le never heard whether its teardown finished; the VM was then stopped", cause, sshExitTransport)
		}
		report.Verdict = RunVerdictFail
		report.GuestExitCode = outcome.code
		return *report, fmt.Errorf("%s stopped the guest command; it finished its own teardown and exited %d, and the VM was then stopped",
			cause, outcome.code)
	case <-grace.C:
		endSession()
		<-finished
		return *report, fmt.Errorf("%s stopped the run, and the guest command did not finish its teardown within %s of the forwarded SIGINT, "+
			"so the VM was stopped under it: what it had not removed inside the guest went with the VM", cause, r.teardownGrace)
	case <-again:
		endSession()
		<-finished
		return *report, fmt.Errorf("%s stopped the run, and a second interrupt stopped the wait for the guest command's teardown, "+
			"so the VM was stopped under it: what it had not removed inside the guest went with the VM", cause)
	}
}

// interruptGuest runs guestInterruptRemote over a second ssh session, under a
// bound of its own because the run's root has already ended, and answers the
// process group it signaled.
func (r *Run) interruptGuest(ctx context.Context, plan *RunPlan) (string, error) {
	bounded, stop := context.WithTimeout(context.WithoutCancel(ctx), guestInterruptTimeout)
	defer stop()
	args := append(r.sshOptions(plan.SSHPort), "-o", "ConnectTimeout=5", "root@localhost", guestInterruptRemote(guestCommandGroupFile))
	result, err := r.runCommand(bounded, commandSpec{
		Name: sshProgram, Args: args, Env: r.ops.Environ(), Stdin: strings.NewReader(""), OwnGroup: true,
	})
	if err != nil {
		return "", err
	}
	if result.Code != 0 {
		var b textbuf.Buffer
		b.Str("the guest answered exit ").Int(int64(result.Code))
		if detail := strings.TrimSpace(result.Stderr); detail != "" {
			b.Str(": ").Str(detail)
		}
		return "", errors.New(b.String())
	}
	return strings.TrimSpace(result.Stdout), nil
}
