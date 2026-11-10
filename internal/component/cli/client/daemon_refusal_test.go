// Design: docs/architecture/cli/error-surface.md -- how an operator error reads on each surface
// Related: daemon_refusal.go -- daemonUnreachable, the refusal this judges.

package client

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/opererr"
)

// TestCLINoDaemonNamesEndpointAndFix reads the two refusals `ze cli` prints
// when it cannot reach a daemon: the credentials could not be loaded, or the
// SSH endpoint refused the connection.
//
// VALIDATES: AC-9 -- the message names the endpoint it tried (the store the
// credentials come from; the daemon's SSH host and port), the cause, and the
// command that checks or prepares the daemon, on its own fix line; the fix
// commands `ze status` and `ze init` are registered root commands (`ze
// start` is registered by cmd/ze, ze_core_dispatch.go, which this package
// does not link).
// PREVENTS: "hint: is the daemon running?", a question with no command.
// Not parallel: it sets ze.config.dir, which is process state.
func TestCLINoDaemonNamesEndpointAndFix(t *testing.T) {
	refused := errors.New("connect to 127.0.0.1:2222: connection refused")
	err := daemonUnreachable("127.0.0.1", "2222", refused)
	assert.Equal(t, "ze cli cannot reach the daemon at 127.0.0.1:2222\n"+
		"ze cli sends every daemon command over SSH to that address, and the connection failed: "+
		"connect to 127.0.0.1:2222: connection refused\n"+
		"fix: ze status (and ze start <config> when no daemon runs)", err.Error())
	assert.ErrorIs(t, err, refused)
	var operr *opererr.Error
	require.True(t, errors.As(err, &operr))

	missing := errors.New("no credentials for 127.0.0.1:2222")
	// A test binary runs from the toolchain's cache, not from a bin directory,
	// so paths.DefaultConfigDir derives no store from it: set ze.config.dir,
	// the override the client reads first, as an operator's environment would.
	// It MUST be set before the refusal is built, because the refusal reads
	// the store location when it is built.
	configDir := t.TempDir()
	previous := env.Get("ze.config.dir")
	t.Cleanup(func() { _ = env.Set("ze.config.dir", previous) })
	require.NoError(t, env.Set("ze.config.dir", configDir))
	credErr := sshclient.CredentialsUnavailable(missing)
	storeDir := sshclient.ResolveStoreDir("")
	require.Equal(t, configDir, storeDir, "the test reads the store location the client reads")
	assert.Equal(t, "cannot load the credentials needed to reach the daemon\n"+
		"the client logs in to the daemon over SSH with the credentials stored in "+storeDir+
		": no credentials for 127.0.0.1:2222\n"+
		"fix: ze init", credErr.Error())
	assert.ErrorIs(t, credErr, missing)

	for _, root := range []string{"status", "init"} {
		assert.NotNil(t, registry.LookupRoot(root), "the fix names `ze %s`, a registered root command", root)
	}
}
