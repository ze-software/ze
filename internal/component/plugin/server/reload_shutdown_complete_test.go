package server

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/transaction"
)

// TestReloadShutdownCompletesOpenTransaction proves a transaction Stop stands
// down still sends its completion owners config-complete(false), bounded by
// the shutdown grace (completeShutdown, reload_tx.go). The orchestrator skips
// the rollback for shutdown, so without the receipt a completion owner that
// paused a BGP session for the transaction waits for its hold timer instead.
//
// Method: the completion owner's verify answer is held while the transaction
// is canceled with transaction.ErrShutdown, the cause Stop gives
// (stopTransaction). The reload must fail, and the owner must receive exactly
// one config-complete(false) for the transaction, whose deadline is within
// txShutdownGrace of the send.
//
// VALIDATES: a shutdown during an open transaction sends a bounded
// config-complete(false) to every completion owner.
// PREVENTS: a paused session waiting for a receipt nobody sends at shutdown.
func TestReloadShutdownCompletesOpenTransaction(t *testing.T) {
	root := fmt.Sprintf("shutdown-complete-%d", time.Now().UnixNano())
	owner := "shutdown-completion-owner"
	trace := &orderRecorder{}
	plugins := []pluginDef{{name: owner, roots: []string{root}, configCompletion: true, trace: trace}}
	oldTree := map[string]any{root: map[string]any{"value": "old"}}
	newTree := map[string]any{root: map[string]any{"value": "new"}}
	s := newTestReloadServer(t, &mockReloadReactor{tree: oldTree}, plugins)

	responder := plugins[0].responder
	responder.mu.Lock()
	responder.beforeVerifyRsp = func() {
		cancel, _ := s.txLock.inFlight()
		if cancel == nil {
			t.Error("no transaction in flight at verify") // the responder goroutine cannot FailNow
			return
		}
		cancel(transaction.ErrShutdown)
	}
	responder.mu.Unlock()

	sent := time.Now()
	err := s.ReloadConfig(context.Background(), newTree)
	require.Error(t, err, "a transaction stood down by shutdown must fail the reload")

	want := "complete:" + owner + ":false"
	require.Eventually(t, func() bool { return slices.Contains(trace.snapshot(), want) }, 5*time.Second, 10*time.Millisecond,
		"shutdown sent the completion owner no config-complete(false); trace %v", trace.snapshot())
	require.Equal(t, 1, countOf(trace.snapshot(), want), "completion sent more than once: %v", trace.snapshot())

	responder.mu.Lock()
	inputs := slices.Clone(responder.completeInputs)
	responder.mu.Unlock()
	require.Len(t, inputs, 1)
	require.False(t, inputs[0].Accepted)
	require.NotEmpty(t, inputs[0].TransactionID)
	deadline := time.UnixMilli(inputs[0].DeadlineMS)
	require.False(t, deadline.Before(sent), "receipt deadline %v precedes the reload", deadline)
	require.False(t, deadline.After(time.Now().Add(txShutdownGrace)),
		"receipt deadline %v is not bounded by the shutdown grace %v", deadline, txShutdownGrace)
}
