package grpc

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	zepb "github.com/ze-software/ze/api/proto"
)

// startAuthReloadServer starts a real gRPC server on a bound loopback listener
// and returns it alongside a client, so a test can flip the server's
// authentication and prove the change reaches a request in flight.
func startAuthReloadServer(t *testing.T, cfg GRPCConfig) (*GRPCServer, zepb.ZeServiceClient) {
	t.Helper()

	if len(cfg.ListenAddrs) == 0 {
		cfg.ListenAddrs = []string{"127.0.0.1:0"}
	}
	srv, err := NewGRPCServer(cfg, testEngine(), nil)
	require.NoError(t, err)

	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	serveBackground(srv.srv, ln)
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient(ln.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })

	return srv, zepb.NewZeServiceClient(conn)
}

// listCommandsCode issues a real RPC and reports its gRPC status code.
func listCommandsCode(t *testing.T, client zepb.ZeServiceClient, token string) codes.Code {
	t.Helper()
	ctx := t.Context()
	if token != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
	}
	_, err := client.ListCommands(ctx, &zepb.ListCommandsRequest{})
	return status.Code(err)
}

// VALIDATES: AC-1 -- turning authentication ON in a reloaded config makes the
// RUNNING gRPC server demand it, proven by a real RPC and with no rebind.
// PREVENTS: a reload that reports success while the listener keeps accepting
// every RPC unauthenticated, which is what the daemon did while authentication
// was fixed at construction.
func TestGRPCUpdateAuthTurnsAuthenticationOn(t *testing.T) {
	srv, client := startAuthReloadServer(t, GRPCConfig{})

	assert.False(t, srv.Authenticated())
	assert.Equal(t, codes.OK, listCommandsCode(t, client, ""))

	restore, err := srv.UpdateAuth("secret", nil)
	require.NoError(t, err)
	require.NotNil(t, restore)

	assert.True(t, srv.Authenticated())
	assert.Equal(t, codes.Unauthenticated, listCommandsCode(t, client, ""), "an unauthenticated RPC must be refused after the reload")
	assert.Equal(t, codes.Unauthenticated, listCommandsCode(t, client, "wrong"))
	assert.Equal(t, codes.OK, listCommandsCode(t, client, "secret"))
}

// VALIDATES: AC-1 (other direction) -- turning authentication OFF in a reloaded
// config also takes effect on the running gRPC server.
// PREVENTS: an implementation that only ever adds credentials, leaving the
// exposure guard's view of the server drifting from what it serves.
func TestGRPCUpdateAuthTurnsAuthenticationOff(t *testing.T) {
	srv, client := startAuthReloadServer(t, GRPCConfig{Token: "secret"})

	assert.True(t, srv.Authenticated())
	assert.Equal(t, codes.Unauthenticated, listCommandsCode(t, client, ""))

	_, err := srv.UpdateAuth("", nil)
	require.NoError(t, err)

	assert.False(t, srv.Authenticated())
	assert.Equal(t, codes.OK, listCommandsCode(t, client, ""), "the reloaded config no longer asks for credentials")
}

// VALIDATES: AC-2 -- the restore function UpdateAuth returns puts the previous
// credentials back on the running server.
// PREVENTS: a reload that fails after the rebuild leaving the server less
// authenticated than it was before the reload started.
func TestGRPCUpdateAuthRestoreRevertsCredentials(t *testing.T) {
	srv, client := startAuthReloadServer(t, GRPCConfig{Token: "original"})

	restore, err := srv.UpdateAuth("", nil)
	require.NoError(t, err)
	assert.Equal(t, codes.OK, listCommandsCode(t, client, ""))

	restore()

	assert.True(t, srv.Authenticated())
	assert.Equal(t, codes.Unauthenticated, listCommandsCode(t, client, ""))
	assert.Equal(t, codes.OK, listCommandsCode(t, client, "original"))
}

// VALIDATES: AC-3 -- gRPC refuses to drop its authentication while it is bound
// to a non-loopback address, so a reload cannot produce the pair the boot guard
// refuses to start with.
// PREVENTS: a reload turning a remotely reachable gRPC listener into an
// unauthenticated one, which is the exposure the whole guard exists to stop.
func TestGRPCUpdateAuthRefusesToUnauthenticateNonLoopback(t *testing.T) {
	srv, err := NewGRPCServer(GRPCConfig{ListenAddrs: []string{"127.0.0.1:0"}, Token: "secret"}, testEngine(), nil)
	require.NoError(t, err)

	// Bound non-loopback. Reaching this state through the constructor needs TLS
	// material a unit test has no fixture for, so the addresses are set here
	// directly: the guard under test reads them, and reading them is the
	// behavior being proven.
	srv.bound = []string{"192.0.2.10:50051"}

	restore, updErr := srv.UpdateAuth("", nil)
	require.Error(t, updErr)
	assert.Contains(t, updErr.Error(), "192.0.2.10:50051")
	assert.Nil(t, restore)
	assert.True(t, srv.Authenticated(), "a refused update must leave the credentials untouched")

	// A loopback address in the same set is not an escape: one non-loopback
	// entry is enough to refuse.
	srv.bound = []string{"127.0.0.1:50051", "192.0.2.10:50051"}
	_, updErr = srv.UpdateAuth("", nil)
	require.Error(t, updErr)

	// Loopback only: the same call is allowed.
	srv.bound = []string{"127.0.0.1:50051"}
	_, updErr = srv.UpdateAuth("", nil)
	require.NoError(t, updErr)
	assert.False(t, srv.Authenticated())
}

// VALIDATES: Reconfigure applies the constructor's full rule -- authenticated
// AND encrypted -- to the addresses a reload asks for.
// PREVENTS: a SIGHUP reaching a state the daemon refuses to boot into. gRPC on
// loopback with a token and no TLS, migrated to 0.0.0.0, sends every bearer
// token across the network in cleartext. The hub's exposure guard passes it,
// because that guard classifies authentication and cannot see a certificate.
func TestGRPCReconfigureRefusesNonLoopbackWithoutTLS(t *testing.T) {
	srv, err := NewGRPCServer(GRPCConfig{ListenAddrs: []string{"127.0.0.1:0"}, Token: "secret"}, testEngine(), nil)
	require.NoError(t, err)
	require.False(t, srv.tlsConfigured)

	err = srv.Reconfigure(t.Context(), []string{"0.0.0.0:50051"})
	require.Error(t, err, "an authenticated plaintext listener must not move off loopback")
	assert.Contains(t, err.Error(), "TLS")
	assert.Contains(t, err.Error(), "0.0.0.0:50051")

	// Unauthenticated is refused first, and names the other missing condition.
	_, updErr := srv.UpdateAuth("", nil)
	require.NoError(t, updErr)
	err = srv.Reconfigure(t.Context(), []string{"0.0.0.0:50051"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires authentication")

	// A loopback move is unaffected: the rule gates exposure, not reloads.
	require.NoError(t, srv.Reconfigure(t.Context(), []string{"127.0.0.1:0"}))
}

// VALIDATES: the undo UpdateAuth returns re-checks the listener rule and KEEPS
// the reloaded credentials when putting the old ones back would expose the
// listener.
// PREVENTS: the original defect mirrored. A single reload installs a token and
// then moves gRPC off loopback; both guards pass, because the credentials are
// in place when each one looks. If a LATER step of the reload fails, runReload
// runs this undo and nothing moves the listener back, so an unconditional
// restore leaves 0.0.0.0 serving with no credentials -- the operator told the
// reload FAILED, and the port ungated.
func TestGRPCUpdateAuthUndoRefusesToExposeMigratedListener(t *testing.T) {
	srv, err := NewGRPCServer(GRPCConfig{ListenAddrs: []string{"127.0.0.1:0"}}, testEngine(), nil)
	require.NoError(t, err)
	srv.tlsConfigured = true // as a boot with tls-cert/tls-key would leave it
	require.False(t, srv.Authenticated())

	// The reload installs a token while the listener is still on loopback.
	restore, err := srv.UpdateAuth("reloaded-token", nil)
	require.NoError(t, err)
	require.True(t, srv.Authenticated())

	// ...and then migrates it off loopback, which the rule allows now that the
	// server is authenticated and encrypted.
	require.NoError(t, srv.Reconfigure(t.Context(), []string{"0.0.0.0:0"}))

	// A later step of the reload fails, so runReload unwinds the credentials.
	restore()

	assert.True(t, srv.Authenticated(), "the undo must not strip credentials off a listener it cannot move back")
	require.NoError(t, checkGRPCListenAddr(srv.bound[0], srv.Authenticated(), srv.tlsConfigured),
		"no address the server holds may be left reachable without credentials")
}

// VALIDATES: the undo still restores when doing so is safe, so keeping the
// reloaded credentials is the exception and not the rule.
// PREVENTS: a fix that simply stops restoring, which would leave every failed
// reload authenticating against the config the daemon rolled back.
func TestGRPCUpdateAuthUndoRestoresWhenLoopback(t *testing.T) {
	srv, err := NewGRPCServer(GRPCConfig{ListenAddrs: []string{"127.0.0.1:50051"}, Token: "original"}, testEngine(), nil)
	require.NoError(t, err)

	restore, err := srv.UpdateAuth("reloaded", nil)
	require.NoError(t, err)
	require.Equal(t, "reloaded", srv.token)

	restore()

	assert.Equal(t, "original", srv.token, "a loopback listener must get its previous credentials back")
}

// VALIDATES: UpdateAuth fails closed on a server that has been stopped.
// PREVENTS: a reload believing it rebuilt authentication on a server that is no
// longer serving.
func TestGRPCUpdateAuthRefusedAfterStop(t *testing.T) {
	srv, err := NewGRPCServer(GRPCConfig{ListenAddrs: []string{"127.0.0.1:0"}, Token: "secret"}, testEngine(), nil)
	require.NoError(t, err)
	srv.Stop()

	restore, updErr := srv.UpdateAuth("", nil)
	require.Error(t, updErr)
	assert.Nil(t, restore)
	assert.True(t, srv.Authenticated())
}
