// Design: docs/architecture/cli/error-surface.md -- how an operator error reads on each surface
// Related: client.go -- LoadCredentialsWithFlags, ResolveStoreDir: the load this refusal reports
// Related: answer.go -- dialDaemon, which answers a LoginRefused

package sshclient

import (
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// LoginRefused is the dial failure of a daemon that answered: its SSH server
// presented a host key the client accepted, then the login of User failed.
// Something runs at Address, so a caller that serves a command without the
// daemon (an offline fallback) MUST NOT read this as a daemon that is not
// there. Cause is the SSH library's error, which names the methods tried.
type LoginRefused struct {
	Address string
	User    string
	Cause   error
}

// Error names the address, the user and the cause.
func (e *LoginRefused) Error() string {
	var tb textbuf.Buffer
	tb.Str("the daemon at ").Str(e.Address).Str(" refused the login of ").Str(e.User).
		Str(": ").Str(e.Cause.Error())
	return tb.String()
}

// Unwrap answers the SSH library's error.
func (e *LoginRefused) Unwrap() error { return e.Cause }

// LoginRefusal is the refusal a CLI tool prints for a LoginRefused: the daemon
// runs and does not accept the stored login, so the fix is a login it accepts,
// never a start.
func LoginRefusal(refused *LoginRefused) error {
	var problem textbuf.Buffer
	problem.Str("the daemon at ").Str(refused.Address).Str(" refused the login of ").Str(refused.User)
	var why textbuf.Buffer
	why.Str("a daemon runs there and answered, but it does not accept the credentials stored in ")
	if storeDir := ResolveStoreDir(""); storeDir == "" {
		why.Str("the ze database, whose location Ze could not determine")
	} else {
		why.Str(storeDir)
	}
	why.Str(": it checks a login against the users of its own configuration and store")
	// The cause, not refused: the problem line already names the address and
	// the user that refused.Error repeats.
	return opererr.Wrap(refused.Cause,
		problem.String(),
		why.String(),
		"ze cli --user <name>, with a user the daemon's configuration declares")
}

// CredentialsUnavailable is the refusal a CLI tool prints when the SSH
// credentials that reach the daemon cannot be loaded (LoadCredentialsWithFlags
// or LoadCredentialsForRemote failed with err). It names the store the
// credentials are read from: `ze init` writes that store and the super-admin
// login in it.
func CredentialsUnavailable(err error) error {
	var why textbuf.Buffer
	why.Str("the client logs in to the daemon over SSH with the credentials stored in ")
	if storeDir := ResolveStoreDir(""); storeDir == "" {
		why.Str("the ze database, whose location Ze could not determine")
	} else {
		why.Str(storeDir)
	}
	return opererr.Wrap(err,
		"cannot load the credentials needed to reach the daemon",
		why.String(),
		"ze init")
}
