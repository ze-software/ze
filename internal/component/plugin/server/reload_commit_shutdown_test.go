// Design: docs/architecture/config/transaction-protocol.md -- shutdown while a transaction runs
// Related: reload.go -- reloadConfig opens its shutdown scope under the caller's
// Related: reload_shutdown_complete_test.go -- the same stand-down through Stop's own cancel

package server

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/transaction"
)

// TestReloadStoodDownByItsCallersScopeMidApply runs a reload the way a config
// commit reaches the server: under a shutdown scope its caller owns (the
// hub's reload gate, cmd/ze/hub reloadGate.admit) and the reload's own
// deadline. The one plugin never answers its apply, and the caller then ends
// its scope with transaction.ErrShutdown, which is what the daemon's stop
// does before the server itself stops.
//
// VALIDATES: the caller's scope stands the server's transaction down
// mid-apply: the reload returns at once as a shutdown, and the plugin is sent
// no config-rollback.
// PREVENTS: a commit-driven reload that only Server.Stop can cancel, which
// runs after the hub has already waited for the reload to end.
func TestReloadStoodDownByItsCallersScopeMidApply(t *testing.T) {
	root := fmt.Sprintf("commit-shutdown-%d", time.Now().UnixNano())
	plugins := []pluginDef{{name: "silent-in-apply", roots: []string{root}}}
	oldTree := map[string]any{root: map[string]any{"value": "old"}}
	newTree := map[string]any{root: map[string]any{"value": "new"}}
	s := newTestReloadServer(t, &mockReloadReactor{tree: oldTree}, plugins)

	applying := make(chan struct{})
	released := make(chan struct{})
	// Registered after the server's own cleanups, so it runs before them: the
	// responder leaves its apply before its connection closes.
	t.Cleanup(func() { close(released) })
	responder := plugins[0].responder
	responder.mu.Lock()
	responder.beforeApplyRsp = func() {
		close(applying)
		<-released
	}
	responder.mu.Unlock()

	gated, standDown := transaction.WithShutdownScope(context.Background())
	defer standDown(nil)
	bounded, cancelBound := context.WithTimeout(gated, time.Hour)
	defer cancelBound()

	reloaded := make(chan error, 1)
	go func() { reloaded <- s.ReloadConfig(bounded, newTree) }()
	select {
	case <-applying:
	case err := <-reloaded:
		t.Fatalf("the reload returned before its apply reached the plugin: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("the apply did not reach the plugin")
	}

	standDown(transaction.ErrShutdown)

	select {
	case err := <-reloaded:
		require.Error(t, err, "a reload stood down by shutdown must fail")
		if !errors.Is(err, transaction.ErrShutdown) {
			t.Fatalf("reload error = %v, want it to wrap %v", err, transaction.ErrShutdown)
		}
	case <-time.After(txShutdownGrace + 2*time.Second):
		t.Fatal("the reload did not return: the caller's shutdown scope did not stand it down")
	}
	require.Zero(t, responder.getRollbackCalls(), "a reload stood down by shutdown sent the plugin a rollback")
}
