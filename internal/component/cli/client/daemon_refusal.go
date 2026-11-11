// Design: docs/architecture/cli/error-surface.md -- how an operator error reads on each surface
// Related: main.go -- Run, the site that prints this refusal

package client

import (
	"errors"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// daemonUnreachable is the refusal Run prints when the daemon's SSH endpoint
// host:port does not answer at the connection level. `ze status` checks
// whether a daemon runs, and `ze start` starts one. A missing credential is
// sshclient.CredentialsUnavailable, printed before any connection is tried.
func daemonUnreachable(host, port string, err error) error {
	// A trust refusal already explains why this daemon cannot be contacted
	// and how to repair that refusal. Keep its final fix and error identity.
	var refusal *opererr.Error
	if errors.As(err, &refusal) {
		return err
	}
	var problem textbuf.Buffer
	problem.Str("ze cli cannot reach the daemon at ").HostPort(host, port)
	return opererr.Wrap(err,
		problem.String(),
		"ze cli sends every daemon command over SSH to that address, and the connection failed",
		"ze status (and ze start <config> when no daemon runs)")
}
