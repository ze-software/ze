// Design: docs/architecture/doctor-and-health-checks.md -- the local route asks the running daemon first
// Related: client.go -- DaemonAddress, ProbeDaemon and the credential readers this file orders
//
// reach.go is the one walk a local command takes to a running daemon before
// it sends it a command of its own: the daemon's address, a probe of its
// listener, then a login credential. `ze doctor` and `ze config validate` both
// ask the daemon first and answer alone when none answers, and each gives the
// operator one fix per step that failed, so the step is a typed fact here.
// `ze doctor` words each step itself. The commands that ask a running ze for
// what this process does not hold (`ze config validate`, `fmt`, `dump`, `diff`
// and `graph` for an external plugin's section, and `ze <verb> <path>` for a
// path this binary's tree does not hold) say the same thing about the same
// step, so they share one wording: Unreached.Explain.

package sshclient

import (
	"net"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The fix lines Unreached.Explain answers, one per step that a command other
// than the asking one fixes.
const (
	// FixStartDaemon starts ze, the answer when nothing listens.
	FixStartDaemon = "ze start"
	// FixDaemonStatus says whether a ze runs, which one, and where it listens:
	// the answer when the address could not be read.
	FixDaemonStatus = "ze status"
)

// ReachStep names the step at which the walk to the running daemon stopped.
type ReachStep uint8

const (
	// ReachStepUnspecified is the zero value and names no step.
	ReachStepUnspecified ReachStep = iota
	// ReachStepAddress: the address of the daemon's SSH listener could not be
	// read.
	ReachStepAddress
	// ReachStepListener: nothing accepts a connection at that address.
	ReachStepListener
	// ReachStepCredentials: a daemon listens, and this account holds nothing
	// to log in to it with.
	ReachStepCredentials
)

// Unreached is why the walk to the running daemon stopped: the step, the
// address when it was resolved, and the failure underneath.
type Unreached struct {
	Step    ReachStep
	Address string
	Cause   error
}

func (e *Unreached) Error() string { return e.Cause.Error() }

// Unwrap answers the failure underneath.
func (e *Unreached) Unwrap() error { return e.Cause }

// Explain words the failure for an operator: why no running ze answered, as a
// clause with no capital and no full stop, and the one command that makes one
// answer, after which the operator runs the asking command again. fix is ""
// at ReachStepCredentials: a ze listens there, and the fix is the asking
// command run from an account that can log in, which only the caller can
// spell.
func (e *Unreached) Explain() (why, fix string) {
	var text textbuf.Buffer
	switch e.Step {
	case ReachStepAddress:
		text.Str("the address of ze's SSH listener could not be read (").Err(e.Cause).Str(")")
		return text.String(), FixDaemonStatus
	case ReachStepListener:
		text.Str("no ze listens at ").Str(e.Address).Str(" (").Err(e.Cause).Str(")")
		return text.String(), FixStartDaemon
	case ReachStepCredentials:
		text.Str("a ze listens at ").Str(e.Address).
			Str(", but this account has no credentials to log in to it (").Err(e.Cause).Str(")")
		return text.String(), ""
	case ReachStepUnspecified:
		panic("BUG: sshclient.ReachDaemon answered a failure that names no step")
	default:
		panic("BUG: unknown sshclient.ReachStep")
	}
}

// Reached is a daemon that listens and a credential to log in to it with.
type Reached struct {
	// Address is the host:port of the daemon's SSH listener.
	Address string
	// Credentials log in to that listener.
	Credentials Credentials
}

// ReachDaemon walks to the running daemon: it resolves the daemon's SSH
// address as every `ze <verb>` daemon command does (DaemonAddress), probes the
// listener before it reads any credential (ProbeDaemon), so "no daemon runs"
// and "this account cannot log in" stay two facts, then loads a credential.
//
// user is the account to log in as. With user empty the credential is the one
// `ze init` stored, read without ever prompting (LoadCredentialsNoPrompt): the
// caller asks unattended. With user set the operator named an account, so the
// password comes from ze.ssh.password or from a prompt on the terminal
// (LoadCredentialsWithFlags).
//
// The error is always an *Unreached. It logs in to nothing: the caller sends
// its command with the credentials and reads that failure itself.
func ReachDaemon(user string) (Reached, error) {
	host, port, err := DaemonAddress()
	if err != nil {
		return Reached{}, &Unreached{Step: ReachStepAddress, Cause: err}
	}
	address := net.JoinHostPort(host, port)
	if err := ProbeDaemon(address); err != nil {
		return Reached{}, &Unreached{Step: ReachStepListener, Address: address, Cause: err}
	}
	creds, err := reachCredentials(user)
	if err != nil {
		return Reached{}, &Unreached{Step: ReachStepCredentials, Address: address, Cause: err}
	}
	return Reached{Address: address, Credentials: creds}, nil
}

// reachCredentials loads the credential ReachDaemon answers: the stored one
// with no prompt, or the named account's.
func reachCredentials(user string) (Credentials, error) {
	if user == "" {
		return LoadCredentialsNoPrompt()
	}
	return LoadCredentialsWithFlags(user)
}
