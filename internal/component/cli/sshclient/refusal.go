// Design: docs/architecture/cli/error-surface.md -- how an operator error reads on each surface
// Related: client.go -- LoadCredentialsWithFlags, ResolveStoreDir: the load this refusal reports

package sshclient

import (
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

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
