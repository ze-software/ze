package hub

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	zeconfig "github.com/ze-software/ze/internal/component/config"
)

func TestListenerDiff_MigratorLocal(t *testing.T) {
	tests := []struct {
		name       string
		old, new   []string
		wantKeep   []string
		wantAdd    []string
		wantRemove []string
	}{
		{
			name:     "no change",
			old:      []string{"127.0.0.1:3443"},
			new:      []string{"127.0.0.1:3443"},
			wantKeep: []string{"127.0.0.1:3443"},
		},
		{
			name:       "add and remove",
			old:        []string{"127.0.0.1:3443", "127.0.0.1:9443"},
			new:        []string{"127.0.0.1:3443", "127.0.0.1:8443"},
			wantKeep:   []string{"127.0.0.1:3443"},
			wantAdd:    []string{"127.0.0.1:8443"},
			wantRemove: []string{"127.0.0.1:9443"},
		},
		{
			name:       "complete replace",
			old:        []string{"127.0.0.1:3443"},
			new:        []string{"127.0.0.1:8443"},
			wantAdd:    []string{"127.0.0.1:8443"},
			wantRemove: []string{"127.0.0.1:3443"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// VALIDATES: listenerDiff keeps listener migration independent from internal/component/web.
			// PREVENTS: always-on hub listener reload pinning the web package into ze_core builds.
			keep, add, remove := listenerDiff(tt.old, tt.new)
			assert.Equal(t, tt.wantKeep, keep, "keep")
			assert.Equal(t, tt.wantAdd, add, "add")
			assert.Equal(t, tt.wantRemove, remove, "remove")
		})
	}
}

type recordingReconfigurable struct {
	addrs []string
	fail  error
	calls [][]string
}

func (r *recordingReconfigurable) Addresses() []string {
	return append([]string(nil), r.addrs...)
}

func (r *recordingReconfigurable) Reconfigure(_ context.Context, newAddrs []string) error {
	r.calls = append(r.calls, append([]string(nil), newAddrs...))
	if r.fail != nil {
		return r.fail
	}
	r.addrs = append([]string(nil), newAddrs...)
	return nil
}

func TestReloadListenersRollsBackAppliedServiceOnLaterFailure(t *testing.T) {
	web := &recordingReconfigurable{addrs: []string{"127.0.0.1:3443"}}
	lg := &recordingReconfigurable{addrs: []string{"127.0.0.1:8443"}, fail: fmt.Errorf("lg refused")}
	migrator := newListenerMigrator()
	migrator.web = web
	migrator.lg = lg

	// VALIDATES: listener migration is all-or-revert inside a rejected reload.
	// PREVENTS: one service staying on the rejected address after a later service fails.
	_, err := migrator.reloadListeners(context.Background(), listenerMigrationTree())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lg refused")

	assert.Equal(t, [][]string{{"127.0.0.1:3444"}, {"127.0.0.1:3443"}}, web.calls)
	assert.Equal(t, []string{"127.0.0.1:3443"}, web.addrs)
	assert.Equal(t, [][]string{{"127.0.0.1:8444"}}, lg.calls)
}

// recordingAuthServer is a Reconfigurable that also implements AuthUpdatable,
// standing in for the REST and gRPC servers, whose authentication a reload
// rebuilds in place.
type recordingAuthServer struct {
	recordingReconfigurable
	token         string
	authenticator func(authHeader string) (string, bool)
	updateErr     error
	updates       []string
}

func (r *recordingAuthServer) Authenticated() bool {
	return r.authenticator != nil || r.token != ""
}

func (r *recordingAuthServer) UpdateAuth(token string, authenticator func(authHeader string) (string, bool)) (func(), error) {
	if r.updateErr != nil {
		return nil, r.updateErr
	}
	prevToken, prevAuthenticator := r.token, r.authenticator
	r.token, r.authenticator = token, authenticator
	r.updates = append(r.updates, token)
	return func() {
		r.token, r.authenticator = prevToken, prevAuthenticator
	}, nil
}

// staticAuth returns a reloader that always resolves to the same intent, which
// is what a reloaded config tree amounts to for the migrator.
func staticAuth(authenticated bool, token string) authReloader {
	return func(*zeconfig.Tree) (authIntent, bool, error) {
		return authIntent{authenticated: authenticated, token: token}, true, nil
	}
}

// VALIDATES: AC-1 -- a reload that turns authentication ON rebuilds it on the
// running server, even though no listen address changed.
// PREVENTS: the silent no-op this whole path replaces. Before the rebuild
// existed, an unchanged address produced an empty change set, reloadListeners
// returned nil without logging, and the listener kept serving unauthenticated
// while the operator believed the reload had taken effect.
func TestReloadListenersRebuildsAuthenticationOn(t *testing.T) {
	rest := &recordingAuthServer{recordingReconfigurable: recordingReconfigurable{addrs: []string{"127.0.0.1:8081"}}}
	migrator := newListenerMigrator()
	migrator.setREST(rest)
	migrator.markUnauthenticated("rest")
	migrator.setAuthReloader("rest", staticAuth(true, "secret"))

	require.False(t, rest.Authenticated())
	_, reloadErr := migrator.reloadListeners(context.Background(), zeconfig.NewTree())
	require.NoError(t, reloadErr)

	assert.True(t, rest.Authenticated(), "the running server must demand authentication after the reload")
	assert.Equal(t, []string{"secret"}, rest.updates)
	assert.Empty(t, rest.calls, "rebuilding authentication must not rebind a listener")
}

// VALIDATES: AC-1 (other direction) -- a reload that turns authentication OFF
// reaches the running server too.
// PREVENTS: an implementation that only ever adds credentials, so the migrator's
// view of a server drifts from what the server serves.
func TestReloadListenersRebuildsAuthenticationOff(t *testing.T) {
	rest := &recordingAuthServer{recordingReconfigurable: recordingReconfigurable{addrs: []string{"127.0.0.1:8081"}}, token: "secret"}
	migrator := newListenerMigrator()
	migrator.setREST(rest)
	migrator.markAuthenticated("rest")
	migrator.setAuthReloader("rest", staticAuth(false, ""))

	require.True(t, rest.Authenticated())
	_, reloadErr := migrator.reloadListeners(context.Background(), zeconfig.NewTree())
	require.NoError(t, reloadErr)

	assert.False(t, rest.Authenticated(), "the reloaded config no longer asks for credentials")
}

// VALIDATES: AC-2 -- a listener migration that fails after the rebuild restores
// the authentication every server started the reload with.
// PREVENTS: a half-applied reload leaving a server less authenticated than it
// was, which is the failure mode a rebuild introduces and a rollback removes.
func TestReloadListenersRestoresAuthenticationWhenMigrationFails(t *testing.T) {
	rest := &recordingAuthServer{recordingReconfigurable: recordingReconfigurable{addrs: []string{"127.0.0.1:8081"}}, token: "original"}
	lg := &recordingReconfigurable{addrs: []string{"127.0.0.1:8443"}, fail: fmt.Errorf("lg refused")}
	migrator := newListenerMigrator()
	migrator.setREST(rest)
	migrator.setLG(lg)
	migrator.markAuthenticated("rest")
	migrator.setAuthReloader("rest", staticAuth(false, ""))

	_, err := migrator.reloadListeners(context.Background(), listenerMigrationTree())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lg refused")

	// Both halves matter. Without the first, this test passes on an
	// implementation that never rebuilds anything: the credentials are still
	// there because nothing ever removed them, and the assertion reads as a
	// rollback that never ran.
	assert.Equal(t, []string{""}, rest.updates, "the rebuild must actually have run")
	assert.True(t, rest.Authenticated(), "the failed reload must put the previous credentials back")
	assert.Equal(t, "original", rest.token)
}

// VALIDATES: AC-3 -- the exposure guard is re-run over the (address,
// authentication) pair the reload PRODUCES, so a reload that both drops
// authentication and moves the listener off loopback is refused.
// PREVENTS: the rebuild opening the exposure the boot guard exists to close. A
// guard reading the boot-time record would classify this server as
// authenticated, because that is what it was when it started.
func TestReloadListenersRefusesRebuiltUnauthenticatedNonLoopback(t *testing.T) {
	web := &recordingAuthServer{recordingReconfigurable: recordingReconfigurable{addrs: []string{"127.0.0.1:3443"}}, token: "secret"}
	migrator := newListenerMigrator()
	migrator.setWeb(web)
	migrator.markAuthenticated("web")
	migrator.setAuthReloader("web", staticAuth(false, ""))

	_, err := migrator.reloadListeners(context.Background(), webOnlyTree(nonLoopbackServiceTree("3443")))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "web")
	assert.Contains(t, err.Error(), "0.0.0.0:3443")

	assert.True(t, web.Authenticated(), "the refused reload must leave the credentials in place")
	assert.Empty(t, web.calls, "nothing may be rebound once the guard refuses")
	assert.Equal(t, []string{"127.0.0.1:3443"}, web.addrs)
}

// VALIDATES: AC-4 -- a service whose authentication is fixed at construction
// and whose reloaded config asks for a different mode FAILS the reload, and
// nothing is rebound.
// PREVENTS: the defect this whole change exists to remove, surviving for the
// surfaces that cannot rebuild. Logging the mismatch and returning nil let
// runReload promote the candidate, so `ze config commit` reported success over
// a web server still serving unauthenticated. The error is what reaches the
// operator's exit status.
func TestReloadListenersFailsWhenAuthCannotBeRebuilt(t *testing.T) {
	web := &recordingReconfigurable{addrs: []string{"127.0.0.1:3443"}}
	lg := &recordingReconfigurable{addrs: []string{"127.0.0.1:8443"}}
	migrator := newListenerMigrator()
	migrator.setWeb(web)
	migrator.setLG(lg)
	migrator.markUnauthenticated("web")
	migrator.setAuthReloader("web", staticAuth(true, ""))

	_, err := migrator.reloadListeners(context.Background(), listenerMigrationTree())
	require.Error(t, err, "a reload that cannot apply the operator's auth change must not report success")
	assert.Contains(t, err.Error(), "web")
	assert.Contains(t, err.Error(), "restart")

	// Refused before anything moved, so there is nothing to roll back and no
	// listener sits on an address the rejected config describes.
	assert.Empty(t, web.calls, "web must keep its listeners")
	assert.Empty(t, lg.calls, "the refusal comes before any service migrates")
	assert.Equal(t, []string{"127.0.0.1:8443"}, lg.addrs)
}

// VALIDATES: a surface the boot guard never classified is left alone, so a
// binary compiled without that surface does not refuse reloads over a config
// block describing a server it cannot run.
// PREVENTS: registerMgmtAuthReloaders registering web/mcp/rest/grpc
// unconditionally turning into a hard reload failure on every SIGHUP for a
// service that is not in the binary.
func TestReloadListenersIgnoresUnclassifiedService(t *testing.T) {
	lg := &recordingReconfigurable{addrs: []string{"127.0.0.1:8443"}}
	migrator := newListenerMigrator()
	migrator.setLG(lg)
	// No markAuthenticated / markUnauthenticated for web: the boot guard never
	// declared it, which is what a compiled-out surface looks like here.
	migrator.setAuthReloader("web", staticAuth(true, ""))

	_, err := migrator.reloadListeners(context.Background(), listenerMigrationTree())
	require.NoError(t, err, "an unclassified surface must not fail the reload")
	assert.Equal(t, [][]string{{"127.0.0.1:8444"}}, lg.calls, "the running service still migrates")
}

// VALIDATES: an unclassified surface's reloader is never CALLED, so a reloader
// that fails cannot fail a reload for a service that is not running.
// PREVENTS: registerMgmtAuthReloaders registering web/mcp/rest/grpc
// unconditionally turning apiAuthReloader's fail-closed branch into a hard
// reload failure on a binary compiled without that surface. Skipping only the
// rebuildable CHECK leaves this open, because the resolve step fails first.
func TestReloadListenersNeverConsultsUnclassifiedService(t *testing.T) {
	lg := &recordingReconfigurable{addrs: []string{"127.0.0.1:8443"}}
	migrator := newListenerMigrator()
	migrator.setLG(lg)

	called := false
	migrator.setAuthReloader("rest", func(*zeconfig.Tree) (authIntent, bool, error) {
		called = true
		return authIntent{}, false, fmt.Errorf("live API users are no longer readable")
	})

	_, err := migrator.reloadListeners(context.Background(), listenerMigrationTree())
	require.NoError(t, err, "a service the boot guard never classified must not fail the reload")
	assert.False(t, called, "the reloader of a service that is not running must not be consulted")
	assert.Equal(t, [][]string{{"127.0.0.1:8444"}}, lg.calls)
}

// VALIDATES: a reloader failing AFTER an earlier service was rebuilt restores
// that earlier service's credentials.
// PREVENTS: a partially applied auth pass. The sorted walk is grpc, mcp, rest,
// web, so grpc can succeed before rest fails; every other test registers one
// reloader, which leaves the restore list empty and cannot pin this.
func TestReloadListenersRestoresEarlierServiceWhenLaterResolveFails(t *testing.T) {
	grpc := &recordingAuthServer{recordingReconfigurable: recordingReconfigurable{addrs: []string{"127.0.0.1:50051"}}, token: "grpc-original"}
	rest := &recordingAuthServer{recordingReconfigurable: recordingReconfigurable{addrs: []string{"127.0.0.1:8081"}}, token: "rest-original"}
	migrator := newListenerMigrator()
	migrator.setGRPC(grpc)
	migrator.setREST(rest)
	migrator.markAuthenticated("grpc")
	migrator.markAuthenticated("rest")
	migrator.setAuthReloader("grpc", staticAuth(false, ""))
	migrator.setAuthReloader("rest", func(*zeconfig.Tree) (authIntent, bool, error) {
		return authIntent{}, false, fmt.Errorf("live API users unreadable")
	})

	_, err := migrator.reloadListeners(context.Background(), zeconfig.NewTree())
	require.Error(t, err)

	// Resolution happens for every service before anything is applied, so this
	// failure lands before grpc is touched at all.
	assert.Empty(t, grpc.updates, "no service may be rebuilt while another's mode is unknown")
	assert.Equal(t, "grpc-original", grpc.token)
	assert.Equal(t, "rest-original", rest.token)
}

// VALIDATES: an UpdateAuth failure on a later service restores the credentials
// already installed on an earlier one.
// PREVENTS: the auth pass leaving grpc rebuilt and rest untouched, a split the
// migrator would then classify as if it were the operator's intent.
func TestReloadListenersRestoresEarlierServiceWhenLaterRebuildFails(t *testing.T) {
	grpc := &recordingAuthServer{recordingReconfigurable: recordingReconfigurable{addrs: []string{"127.0.0.1:50051"}}, token: "grpc-original"}
	rest := &recordingAuthServer{
		recordingReconfigurable: recordingReconfigurable{addrs: []string{"127.0.0.1:8081"}},
		token:                   "rest-original",
		updateErr:               fmt.Errorf("server has been shut down"),
	}
	migrator := newListenerMigrator()
	migrator.setGRPC(grpc)
	migrator.setREST(rest)
	migrator.markAuthenticated("grpc")
	migrator.markAuthenticated("rest")
	migrator.setAuthReloader("grpc", staticAuth(false, ""))
	migrator.setAuthReloader("rest", staticAuth(false, ""))

	_, err := migrator.reloadListeners(context.Background(), zeconfig.NewTree())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rest")

	// grpc WAS rebuilt, and then put back. Both halves matter: without the
	// first, an implementation that rebuilt nothing would pass this test.
	assert.Equal(t, []string{""}, grpc.updates, "grpc must have been rebuilt before rest failed")
	assert.Equal(t, "grpc-original", grpc.token, "the failed pass must put grpc back")
	assert.True(t, grpc.Authenticated())
}

// VALIDATES: the undo reloadListeners returns reverts the credentials it
// installed, so runReload can unwind a reload that fails at a later step.
// PREVENTS: a reload the operator is told FAILED leaving REST and gRPC
// authenticating against the config the daemon rolled back
// (updateWebCertificate and PromoteCandidate both run after reloadListeners).
func TestReloadListenersUndoRevertsInstalledCredentials(t *testing.T) {
	rest := &recordingAuthServer{recordingReconfigurable: recordingReconfigurable{addrs: []string{"127.0.0.1:8081"}}, token: "original"}
	migrator := newListenerMigrator()
	migrator.setREST(rest)
	migrator.markAuthenticated("rest")
	migrator.setAuthReloader("rest", staticAuth(false, ""))

	undo, err := migrator.reloadListeners(context.Background(), zeconfig.NewTree())
	require.NoError(t, err)
	require.NotNil(t, undo)
	require.False(t, rest.Authenticated(), "the reload applied the new mode")

	undo()

	assert.True(t, rest.Authenticated(), "the undo must put the previous credentials back")
	assert.Equal(t, "original", rest.token)
}

// VALIDATES: a service whose configured authentication cannot be resolved fails
// the reload closed; nothing is rebound and no credential is dropped.
// PREVENTS: an unresolvable auth mode reading as "no authentication", which is
// the permissive no-op ai/rules/evidence.md names as the way a guard hides.
func TestReloadListenersFailsClosedWhenAuthCannotBeResolved(t *testing.T) {
	rest := &recordingAuthServer{recordingReconfigurable: recordingReconfigurable{addrs: []string{"127.0.0.1:8081"}}, token: "secret"}
	web := &recordingReconfigurable{addrs: []string{"127.0.0.1:3443"}}
	migrator := newListenerMigrator()
	migrator.setREST(rest)
	migrator.setWeb(web)
	migrator.markAuthenticated("rest")
	migrator.setAuthReloader("rest", func(*zeconfig.Tree) (authIntent, bool, error) {
		return authIntent{}, false, fmt.Errorf("live API users unreadable")
	})

	_, err := migrator.reloadListeners(context.Background(), listenerMigrationTree())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "live API users unreadable")

	assert.True(t, rest.Authenticated(), "an unresolvable mode must never remove credentials")
	assert.Empty(t, web.calls, "no service may migrate while one service's authentication is unknown")
}

// VALIDATES: a reloaded config that does not describe a service leaves that
// service's authentication alone.
// PREVENTS: deleting an api-server block from the config silently stripping the
// credentials off a server that is still listening.
func TestReloadListenersLeavesAuthAloneWhenConfigIsSilent(t *testing.T) {
	rest := &recordingAuthServer{recordingReconfigurable: recordingReconfigurable{addrs: []string{"127.0.0.1:8081"}}, token: "secret"}
	migrator := newListenerMigrator()
	migrator.setREST(rest)
	migrator.markAuthenticated("rest")
	migrator.setAuthReloader("rest", func(*zeconfig.Tree) (authIntent, bool, error) {
		return authIntent{}, false, nil
	})

	_, reloadErr := migrator.reloadListeners(context.Background(), zeconfig.NewTree())
	require.NoError(t, reloadErr)

	assert.True(t, rest.Authenticated())
	assert.Equal(t, "secret", rest.token)
	assert.Empty(t, rest.updates)
}

// VALIDATES: a rebuild the server itself refuses fails the reload closed.
// PREVENTS: the migrator recording an authentication state the server never
// accepted, then classifying a later reload against that fiction.
func TestReloadListenersFailsClosedWhenServerRefusesRebuild(t *testing.T) {
	rest := &recordingAuthServer{
		recordingReconfigurable: recordingReconfigurable{addrs: []string{"127.0.0.1:8081"}},
		token:                   "secret",
		updateErr:               fmt.Errorf("server has been shut down"),
	}
	migrator := newListenerMigrator()
	migrator.setREST(rest)
	migrator.markAuthenticated("rest")
	migrator.setAuthReloader("rest", staticAuth(false, ""))

	_, err := migrator.reloadListeners(context.Background(), listenerMigrationTree())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server has been shut down")
	assert.True(t, rest.Authenticated())
}

func listenerMigrationTree() *zeconfig.Tree {
	tree := zeconfig.NewTree()
	env := zeconfig.NewTree()
	env.SetContainer("web", listenerServiceTree("3444"))
	env.SetContainer("looking-glass", listenerServiceTree("8444"))
	tree.SetContainer("environment", env)
	return tree
}

func listenerServiceTree(port string) *zeconfig.Tree {
	svc := zeconfig.NewTree()
	svc.Set("enabled", "true")
	srv := zeconfig.NewTree()
	srv.Set("ip", "127.0.0.1")
	srv.Set("port", port)
	svc.AddListEntry("server", "main", srv)
	return svc
}

// VALIDATES: applyAuthIntents INSTALLS the reloaded material on every server
// that can take it, and its restore puts the previous material back.
// PREVENTS: the install being skipped while every surrounding step still
// reports success. A reload that resolves an intent, passes both guards and
// migrates addresses without ever calling UpdateAuth leaves the listener
// serving the credentials it booted with, and nothing downstream says so: the
// operator is told the reload completed. Asserted at this seam rather than
// through reloadListeners because this is the one call that changes what the
// running server demands.
func TestApplyAuthIntentsInstallsAndRestoresCredentials(t *testing.T) {
	rest := &recordingAuthServer{recordingReconfigurable: recordingReconfigurable{addrs: []string{"127.0.0.1:8081"}}, token: "boot-token"}
	grpc := &recordingAuthServer{recordingReconfigurable: recordingReconfigurable{addrs: []string{"127.0.0.1:50051"}}, token: "boot-token"}
	migrator := newListenerMigrator()
	migrator.setREST(rest)
	migrator.setGRPC(grpc)

	restore, err := migrator.applyAuthIntents([]resolvedAuth{
		{name: svcREST, intent: authIntent{authenticated: true, token: "rotated-token"}},
		{name: svcGRPC, intent: authIntent{authenticated: true, token: "rotated-token"}},
	})
	require.NoError(t, err)

	assert.Equal(t, "rotated-token", rest.token, "the running REST server must serve the reloaded token")
	assert.Equal(t, "rotated-token", grpc.token, "the running gRPC server must serve the reloaded token")
	assert.Equal(t, []string{"rotated-token"}, rest.updates, "UpdateAuth must be called, not skipped")
	assert.Equal(t, []string{"rotated-token"}, grpc.updates, "UpdateAuth must be called, not skipped")

	restore()
	assert.Equal(t, "boot-token", rest.token, "the restore puts the previous credentials back")
	assert.Equal(t, "boot-token", grpc.token, "the restore puts the previous credentials back")
}
