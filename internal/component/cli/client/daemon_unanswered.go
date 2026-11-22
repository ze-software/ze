// Design: docs/architecture/api/commands.md -- the offline fallback and the cause it is handed
// Related: main.go -- runBGP, whose two failed asks of the daemon these word; runOfflineFallback
// Related: daemon_refusal.go -- daemonUnreachable, printed when no fallback answers
// Related: ../sshclient/reach.go -- Unreached, the step each cause is classified as

package client

import (
	"errors"
	"net"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
	cmd "github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// unansweredWithoutCredentials is the cause an offline fallback of command is
// handed when the credentials that reach the daemon could not be loaded
// (credentialErr). That failure alone does not say whether a daemon runs, so
// the walk to the daemon is taken without a credential
// (sshclient.UnreachedWithoutCredentials): nothing listening is "no ze
// listens", with the start as its fix, and a listener is "this account has no
// credentials", with the command asked from an account that can log in.
func unansweredWithoutCredentials(command string, credentialErr error) cmd.DaemonUnanswered {
	return unansweredAtStep(command, sshclient.UnreachedWithoutCredentials(credentialErr))
}

// unansweredAfterConnect is the cause an offline fallback of command is handed
// when the first request to the daemon at host:port failed with err. A refused
// login never reaches here: endFailedAsk prints it and consults no fallback.
//
// The listener is probed, the one test that tells a daemon that is not there
// from one that is (sshclient.ProbeDaemon, as the local ze doctor probes it).
// Nothing listening is the listener step. Something listening means the
// connection itself failed: a host key this client does not trust, a
// handshake cut short, an answer that never came. A failure that already
// names its own repair (the trust refusal) keeps that repair as the fix;
// any other gets `ze status`, which says whether a ze runs there and where
// it listens.
func unansweredAfterConnect(command, host, port string, err error) cmd.DaemonUnanswered {
	address := net.JoinHostPort(host, port)
	if probeErr := sshclient.ProbeDaemon(address); probeErr != nil {
		return unansweredAtStep(command, &sshclient.Unreached{
			Step: sshclient.ReachStepListener, Address: address, Cause: probeErr,
		})
	}
	var why textbuf.Buffer
	why.Str("something listens at ").Str(address).Str(", but the connection to it failed (")
	if actionable, ok := errors.AsType[*opererr.Error](err); ok {
		why.Str(actionable.Problem()).Byte(')')
		return cmd.NewDaemonUnanswered(why.String(), actionable.Fix())
	}
	why.Err(err).Byte(')')
	return cmd.NewDaemonUnanswered(why.String(), sshclient.FixDaemonStatus)
}

// unansweredAtStep words the step at which the walk to the daemon stopped as
// every command that asks a running ze words it (sshclient.Unreached.Explain).
// The credentials step has no fix of its own there: it is command, asked from
// an account that can log in.
func unansweredAtStep(command string, unreached *sshclient.Unreached) cmd.DaemonUnanswered {
	why, fix := unreached.Explain()
	if fix == "" {
		fix = sshclient.FixAskAsAccount(command)
	}
	return cmd.NewDaemonUnanswered(why, fix)
}
