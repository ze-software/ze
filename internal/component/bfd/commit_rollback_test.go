// Design: docs/guide/bfd.md -- Editing live
// Related: bfd.go -- commitConfig, rollbackConfig
// Related: auth_join_fix_test.go -- the edits and requests these tests reuse
//
// VALIDATES: a commit is all or nothing for the BFD plugin. When its own apply
// fails, the config it runs is the one before the commit, because the engine
// sends no rollback to a plugin whose apply failed
// (docs/architecture/config/transaction-protocol.md). When another plugin's
// apply fails after this one's succeeded, the rollback restores the config
// before the commit. The documented route past a refused profile edit,
// releasing the other client in an earlier commit, then lets the edit through.
// PREVENTS: a refused commit whose edited profiles stay published, so that
// the next client to ask resolves a profile the running config does not hold.
package bfd

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/engine"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// commitAuthJoinEdit commits section to svc the way OnConfigApply does
// (runtimeState.commitConfig) and answers its error.
func commitAuthJoinEdit(t *testing.T, svc *pluginService, section string) error {
	t.Helper()

	cfg, err := parseSections([]sdk.ConfigSection{{Root: configRoot, Data: section}})
	if err != nil {
		t.Fatalf("parseSections(edit): %v", err)
	}
	runtimeStateGuard.Lock()
	defer runtimeStateGuard.Unlock()
	return svc.state.commitConfig(cfg)
}

// sharedSecureSession opens the auth join fixture with one session to the
// BGP neighbor held by a client of "secure" and a client of "secure-twin",
// both Keyed SHA1 key-id 7, and answers the service and the second client.
func sharedSecureSession(t *testing.T) (*pluginService, api.SessionHandle) {
	t.Helper()

	svc, loop, _ := authJoinFixture(t)
	// As loopFor does for every loop it creates (engine.ProfileAuth).
	loop.SetProfileAuth(svc.state.profileAuthFor(api.SingleHop))
	if _, err := svc.EnsureSession(bgpPeerSessionRequest("secure")); err != nil {
		t.Fatalf("EnsureSession(secure): %v", err)
	}
	twin, err := svc.EnsureSession(bgpPeerSessionRequest("secure-twin"))
	if err != nil {
		t.Fatalf("EnsureSession(secure-twin): %v", err)
	}
	return svc, twin
}

// TestRefusedProfileEditKeepsRunningConfig commits an edit moving "secure" to
// key-id 9 while a client of "secure-twin" relies on key-id 7. The apply is
// refused, and a client asking under "secure" afterwards resolves the profile
// as the running config holds it, key-id 7, so it joins the session.
// MUTATION: publishing the edited config and keeping it after the refusal
// resolves "secure" to key-id 9, and the join is refused.
func TestRefusedProfileEditKeepsRunningConfig(t *testing.T) {
	svc, _ := sharedSecureSession(t)

	if err := commitAuthJoinEdit(t, svc, authJoinEditedSection); !errors.Is(err, engine.ErrAuthMismatch) {
		t.Fatalf("commit(edit) = %v, want the edit refused with ErrAuthMismatch", err)
	}

	if _, err := svc.EnsureSession(bgpPeerSessionRequest("secure")); err != nil {
		t.Fatalf("EnsureSession(secure) after the refused commit: %v, want a join under the running key-id 7", err)
	}
}

// TestRefusedProfileEditAppliesAfterRelease follows the route the refusal
// names: the client of "secure-twin" is released, as an earlier commit
// releases it, and the same edit is committed again. It applies, and the
// session runs key-id 9: a client of "secure" joins, a client of
// "secure-twin" (key-id 7) is refused.
// MUTATION: an apply that leaves the session's authentication alone once the
// other client is gone lets "secure-twin" join.
func TestRefusedProfileEditAppliesAfterRelease(t *testing.T) {
	svc, twin := sharedSecureSession(t)

	if err := commitAuthJoinEdit(t, svc, authJoinEditedSection); !errors.Is(err, engine.ErrAuthMismatch) {
		t.Fatalf("commit(edit) = %v, want the edit refused with ErrAuthMismatch", err)
	}
	if err := svc.ReleaseSession(twin); err != nil {
		t.Fatalf("ReleaseSession(secure-twin): %v", err)
	}
	if err := commitAuthJoinEdit(t, svc, authJoinEditedSection); err != nil {
		t.Fatalf("commit(edit) after the release: %v", err)
	}

	if _, err := svc.EnsureSession(bgpPeerSessionRequest("secure")); err != nil {
		t.Fatalf("EnsureSession(secure) after the edit: %v", err)
	}
	if _, err := svc.EnsureSession(bgpPeerSessionRequest("secure-twin")); !errors.Is(err, engine.ErrAuthMismatch) {
		t.Fatalf("EnsureSession(secure-twin) after the edit = %v, want ErrAuthMismatch: the session runs key-id 9", err)
	}
}

// TestConfigRollbackRestoresPreviousConfig commits an edit moving "secure" to
// key-id 9 while only a client of "secure" holds the session, so the apply
// succeeds and the session runs key-id 9. Another participant's apply then
// fails and the engine rolls the transaction back: the session runs key-id 7
// again, so a client of "secure-twin" joins, and a second rollback with no
// apply in between changes nothing.
// MUTATION: a rollback that keeps the applied edit leaves the session on
// key-id 9, and the join is refused.
func TestConfigRollbackRestoresPreviousConfig(t *testing.T) {
	svc, loop, _ := authJoinFixture(t)
	loop.SetProfileAuth(svc.state.profileAuthFor(api.SingleHop))
	if _, err := svc.EnsureSession(bgpPeerSessionRequest("secure")); err != nil {
		t.Fatalf("EnsureSession(secure): %v", err)
	}
	if err := commitAuthJoinEdit(t, svc, authJoinEditedSection); err != nil {
		t.Fatalf("commit(edit): %v", err)
	}
	if _, err := svc.EnsureSession(bgpPeerSessionRequest("secure-twin")); !errors.Is(err, engine.ErrAuthMismatch) {
		t.Fatalf("EnsureSession(secure-twin) after the edit = %v, want ErrAuthMismatch", err)
	}

	runtimeStateGuard.Lock()
	err := svc.state.rollbackConfig()
	runtimeStateGuard.Unlock()
	if err != nil {
		t.Fatalf("rollbackConfig: %v", err)
	}

	if _, err := svc.EnsureSession(bgpPeerSessionRequest("secure-twin")); err != nil {
		t.Fatalf("EnsureSession(secure-twin) after the rollback: %v, want a join under key-id 7", err)
	}
	runtimeStateGuard.Lock()
	err = svc.state.rollbackConfig()
	runtimeStateGuard.Unlock()
	if err != nil {
		t.Fatalf("second rollbackConfig: %v", err)
	}
	if _, err := svc.EnsureSession(bgpPeerSessionRequest("secure")); err != nil {
		t.Fatalf("EnsureSession(secure) after a second rollback: %v, want the session left on key-id 7", err)
	}
}

// disabledSection turns the plugin off: bfd > enabled false.
const disabledSection = `{"bfd":{"enabled":"false"}}`

// subscriptionClosed drains what sub holds and reports whether it is closed.
// A stopped loop closes every subscription before Stop returns, so a
// subscription still open once drained belongs to a running loop.
func subscriptionClosed(sub <-chan api.StateChange) bool {
	for {
		select {
		case _, ok := <-sub:
			if !ok {
				return true
			}
		default:
			return false
		}
	}
}

// TestDisableRolledBackKeepsProtocolClientSession commits bfd > enabled false
// while a BGP client holds a session, then rolls the transaction back as the
// engine does when another participant's apply fails. The client's
// subscription stays open, so it still learns the session's state, and a
// second client joins the same session. While the disable was applied, no
// client got a session.
// MUTATION: stopping the loops when the disable is applied closes the
// client's subscription, and the rollback starts loops the client holds no
// handle into, so this goes red.
func TestDisableRolledBackKeepsProtocolClientSession(t *testing.T) {
	svc, loop, _ := authJoinFixture(t)
	loop.SetProfileAuth(svc.state.profileAuthFor(api.SingleHop))
	client, err := svc.EnsureSession(bgpPeerSessionRequest("secure"))
	if err != nil {
		t.Fatalf("EnsureSession(secure): %v", err)
	}
	sub := client.Subscribe()

	if err := commitAuthJoinEdit(t, svc, disabledSection); err != nil {
		t.Fatalf("commit(disable): %v", err)
	}
	if _, err := svc.EnsureSession(bgpPeerSessionRequest("secure")); !errors.Is(err, errBfdDisabled) {
		t.Fatalf("EnsureSession while disabled = %v, want errBfdDisabled", err)
	}

	runtimeStateGuard.Lock()
	err = svc.state.rollbackConfig()
	runtimeStateGuard.Unlock()
	if err != nil {
		t.Fatalf("rollbackConfig: %v", err)
	}

	if subscriptionClosed(sub) {
		t.Fatal("the client's subscription closed across a rolled-back disable, want it open")
	}
	if _, err := svc.EnsureSession(bgpPeerSessionRequest("secure")); err != nil {
		t.Fatalf("EnsureSession(secure) after the rollback: %v, want a join", err)
	}
}

// TestDisableCommittedStopsLoops commits bfd > enabled false and then the
// transaction's commit: the client's subscription is open until the commit
// and closed after it.
// MUTATION: a commit that leaves the deferred stop undone keeps the loop
// running for a disabled plugin, so this goes red.
func TestDisableCommittedStopsLoops(t *testing.T) {
	svc, loop, _ := authJoinFixture(t)
	loop.SetProfileAuth(svc.state.profileAuthFor(api.SingleHop))
	client, err := svc.EnsureSession(bgpPeerSessionRequest("secure"))
	if err != nil {
		t.Fatalf("EnsureSession(secure): %v", err)
	}
	sub := client.Subscribe()

	if err := commitAuthJoinEdit(t, svc, disabledSection); err != nil {
		t.Fatalf("commit(disable): %v", err)
	}
	if subscriptionClosed(sub) {
		t.Fatal("the subscription closed before the transaction committed")
	}

	runtimeStateGuard.Lock()
	svc.state.committedConfig()
	runtimeStateGuard.Unlock()

	if !subscriptionClosed(sub) {
		t.Fatal("the subscription is open after the disable committed, want the loop stopped")
	}
}

// TestDeferredStopRunsAtNextApply commits bfd > enabled false, gets no commit
// signal, and applies the next transaction: the disable's transaction
// committed, so its stop runs before the next apply.
// MUTATION: a next apply that forgets the deferred stop leaves the disabled
// config's loop running, so this goes red.
func TestDeferredStopRunsAtNextApply(t *testing.T) {
	svc, loop, _ := authJoinFixture(t)
	loop.SetProfileAuth(svc.state.profileAuthFor(api.SingleHop))
	client, err := svc.EnsureSession(bgpPeerSessionRequest("secure"))
	if err != nil {
		t.Fatalf("EnsureSession(secure): %v", err)
	}
	sub := client.Subscribe()

	if err := commitAuthJoinEdit(t, svc, disabledSection); err != nil {
		t.Fatalf("commit(disable): %v", err)
	}
	if err := commitAuthJoinEdit(t, svc, disabledSection); err != nil {
		t.Fatalf("commit(disable) again: %v", err)
	}

	if !subscriptionClosed(sub) {
		t.Fatal("the subscription is open after the next apply, want the deferred stop run")
	}
}
