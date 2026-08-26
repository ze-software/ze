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

// VALIDATES: when a later service fails after a partial apply and the internal
// rollback also fails, reloadListeners returns an undo that retries restoration
// at the outer reload boundary.
// PREVENTS: replacing that retry with noListenerRestore and allowing accepted
// API credentials to return on a listener still at its rejected address.
func TestReloadListenersReturnsRetryAfterInternalRollbackFailure(t *testing.T) {
	web := &rollbackFailReconfigurable{addrs: []string{"127.0.0.1:3443"}}
	lg := &recordingReconfigurable{addrs: []string{"127.0.0.1:8443"}, fail: fmt.Errorf("lg refused")}
	migrator := newListenerMigrator()
	migrator.web = web
	migrator.lg = lg

	restore, err := migrator.reloadListeners(t.Context(), listenerMigrationTree())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listener rollback failed")
	require.Equal(t, 2, web.calls, "migration and failed internal rollback must both run")
	assert.Equal(t, []string{"127.0.0.1:3444"}, web.addrs,
		"the failed internal rollback leaves the candidate address live")

	require.NoError(t, restore(), "the returned undo must retry the incomplete rollback")
	assert.Equal(t, 3, web.calls)
	assert.Equal(t, []string{"127.0.0.1:3443"}, web.addrs)
}

func TestReloadListenersUndoRestoresSuccessfulMigration(t *testing.T) {
	srv := &recordingReconfigurable{addrs: []string{"127.0.0.1:1001"}}
	migrator := newListenerMigrator()
	migrator.setWeb(srv)

	tree := zeconfig.NewTree()
	env := zeconfig.NewTree()
	env.SetContainer("web", listenerServiceTree("1002"))
	tree.SetContainer("environment", env)
	restore, err := migrator.reloadListeners(t.Context(), tree)
	require.NoError(t, err)
	require.Equal(t, []string{"127.0.0.1:1002"}, srv.addrs)

	require.NoError(t, restore())
	assert.Equal(t, []string{"127.0.0.1:1001"}, srv.addrs,
		"a later reload rejection must restore the accepted listener address")
}

// recordingAuthServer is a Reconfigurable that reports its live accepted
// authentication mode.
type recordingAuthServer struct {
	recordingReconfigurable
	authenticated bool
}

func (r *recordingAuthServer) Authenticated() bool {
	return r.authenticated
}

// staticAuth returns a reloader that always resolves to the same candidate
// exposure mode.
func staticAuth(authenticated bool, _ string) authReloader {
	return func(*zeconfig.Tree) (authIntent, bool, error) {
		return authIntent{authenticated: authenticated}, true, nil
	}
}

// VALIDATES: AC-3 -- the exposure guard is re-run over the (address,
// authentication) pair the reload PRODUCES, so a reload that both drops
// authentication and moves the listener off loopback is refused.
// PREVENTS: the rebuild opening the exposure the boot guard exists to close. A
// guard reading the boot-time record would classify this server as
// authenticated, because that is what it was when it started.
func TestReloadListenersRefusesRebuiltUnauthenticatedNonLoopback(t *testing.T) {
	web := &recordingAuthServer{addrs: []string{"127.0.0.1:3443"}, authenticated: true}
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

// VALIDATES: a service whose configured authentication cannot be resolved fails
// the reload closed; nothing is rebound and no credential is dropped.
// PREVENTS: an unresolvable auth mode reading as "no authentication", which is
// the permissive no-op ai/rules/evidence.md names as the way a guard hides.
func TestReloadListenersFailsClosedWhenAuthCannotBeResolved(t *testing.T) {
	rest := &recordingAuthServer{addrs: []string{"127.0.0.1:8081"}, authenticated: true}
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
