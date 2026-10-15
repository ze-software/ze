package sdk

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// journalledValue is the smallest section-apply plugin with the shape every
// production one has: apply records its undo in an sdk.Journal kept in the
// closure, and the rollback handler replays whichever journal is kept there.
// Safe for concurrent use: the SDK event loop writes, the test reads.
type journalledValue struct {
	mu      sync.Mutex
	value   string
	journal *Journal
}

func (v *journalledValue) get() string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.value
}

func (v *journalledValue) register(p *Plugin) {
	p.OnConfigApply(func(sections []ConfigDiffSection) error {
		next := sections[0].Changed
		j := NewJournal()
		v.mu.Lock()
		previous := v.value
		v.mu.Unlock()
		err := j.Record(
			func() error { v.mu.Lock(); v.value = next; v.mu.Unlock(); return nil },
			func() error { v.mu.Lock(); v.value = previous; v.mu.Unlock(); return nil },
		)
		if err != nil {
			return err
		}
		v.journal = j
		return nil
	})
	p.OnConfigRollback(func(_ string) error {
		j := v.journal
		v.journal = nil
		if j == nil {
			return nil
		}
		if errs := j.Rollback(); len(errs) > 0 {
			return errs[0]
		}
		return nil
	})
}

// TestConfigRollbackOwedOnlyForUncommittedApply drives a journalled plugin
// through the engine's callback RPCs in the order a reload produces them.
//
// VALIDATES: a rollback undoes only an apply the plugin accepted in the
// transaction being rolled back. After `config-committed`, a later
// transaction's rollback that reaches a plugin which never applied in it is a
// no-op, so the plugin keeps the committed state (case 1). A rollback after
// the plugin applied returns it to the last committed state (case 2).
// PREVENTS: the journal of the previous COMMITTED transaction being replayed
// by an unrelated transaction's rollback, which restored the state from two
// commits back (plan/journal/rollback-forgets-partial-apply.md).
func TestConfigRollbackOwedOnlyForUncommittedApply(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)
	plugin := &journalledValue{value: "t0"}
	plugin.register(p)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, &Registration{})
	}()
	completeStartup(t, ctx, engine)

	apply := func(value string) {
		t.Helper()
		input := struct {
			Sections []ConfigDiffSection `json:"sections"`
		}{Sections: []ConfigDiffSection{{Root: "test", Changed: value}}}
		require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:config-apply", input))
	}
	txCall := func(method, txID string) error {
		input := struct {
			TransactionID string `json:"transaction-id"`
		}{TransactionID: txID}
		_, err := engine.mux.CallRPC(ctx, method, input)
		return err
	}

	// T1 applies and commits.
	apply("t1")
	assert.NoError(t, txCall("ze-plugin-callback:config-committed", "tx-1"), "the commit signal must reach the plugin")

	// Case 1: T2 fails before this plugin applies anything, and its rollback
	// is broadcast to every participant.
	require.NoError(t, txCall("ze-plugin-callback:config-rollback", "tx-2"))
	assert.Equal(t, "t1", plugin.get(), "a rollback for a transaction the plugin never applied must keep the committed state")

	// Case 2: T3 applies here, then fails elsewhere.
	apply("t3")
	require.NoError(t, txCall("ze-plugin-callback:config-rollback", "tx-3"))
	assert.Equal(t, "t1", plugin.get(), "a rollback after the plugin applied must return it to the last committed state")

	// A second rollback has nothing left to undo.
	require.NoError(t, txCall("ze-plugin-callback:config-rollback", "tx-3"))
	assert.Equal(t, "t1", plugin.get(), "a repeated rollback must not undo anything a second time")

	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:bye", struct {
		Reason string `json:"reason"`
	}{Reason: "done"}))
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit")
	}
}

// TestConfigCommittedRunsOnlyForOpenApply drives a plugin with a commit
// handler through the engine's callback RPCs.
//
// VALIDATES: the commit handler runs for a transaction the plugin accepted an
// apply in, and not for one it never applied in, nor for one that rolled
// back, nor twice for one commit.
// PREVENTS: a plugin performing a deferred, irreversible step of an apply
// that was rolled back, or of an apply that belongs to another transaction.
func TestConfigCommittedRunsOnlyForOpenApply(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)
	var mu sync.Mutex
	var committed []string
	p.OnConfigApply(func([]ConfigDiffSection) error { return nil })
	p.OnConfigRollback(func(string) error { return nil })
	p.OnConfigCommitted(func(txID string) error {
		mu.Lock()
		defer mu.Unlock()
		committed = append(committed, txID)
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, &Registration{})
	}()
	completeStartup(t, ctx, engine)

	apply := func() {
		t.Helper()
		input := struct {
			Sections []ConfigDiffSection `json:"sections"`
		}{Sections: []ConfigDiffSection{{Root: "test"}}}
		require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:config-apply", input))
	}
	txCall := func(method, txID string) {
		t.Helper()
		input := struct {
			TransactionID string `json:"transaction-id"`
		}{TransactionID: txID}
		_, err := engine.mux.CallRPC(ctx, method, input)
		require.NoError(t, err)
	}

	apply()
	txCall("ze-plugin-callback:config-committed", "tx-1")
	txCall("ze-plugin-callback:config-committed", "tx-1")
	txCall("ze-plugin-callback:config-committed", "tx-2")
	apply()
	txCall("ze-plugin-callback:config-rollback", "tx-3")
	txCall("ze-plugin-callback:config-committed", "tx-3")

	mu.Lock()
	got := append([]string(nil), committed...)
	mu.Unlock()
	assert.Equal(t, []string{"tx-1"}, got, "the commit handler runs once, for the one committed apply")

	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:bye", struct {
		Reason string `json:"reason"`
	}{Reason: "done"}))
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit")
	}
}

// VALIDATES: an apply that fails with ErrRunningConfigLost opens the rollback
// gate, so the transaction's rollback reaches the handler and its error
// answers the rollback RPC (which the engine's bridge acks as CodeBroken and
// the orchestrator answers with a restart). An apply that fails with any
// other error keeps the gate closed.
// PREVENTS: a plugin that lost its running config during a failed apply
// staying down, because the rollback never reached it.
func TestConfigApplyRunningConfigLostReachesRollback(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)
	var applyErr error
	var rollbacks int
	errRestore := errors.New("previous listener still unbindable")
	p.OnConfigApply(func(_ []ConfigDiffSection) error { return applyErr })
	p.OnConfigRollback(func(_ string) error { rollbacks++; return errRestore })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, &Registration{})
	}()
	completeStartup(t, ctx, engine)

	apply := func() {
		t.Helper()
		input := struct {
			Sections []ConfigDiffSection `json:"sections"`
		}{Sections: []ConfigDiffSection{{Root: "test", Changed: "x"}}}
		_, err := engine.mux.CallRPC(ctx, "ze-plugin-callback:config-apply", input)
		require.NoError(t, err)
	}
	rollback := func(txID string) error {
		_, err := engine.mux.CallRPC(ctx, "ze-plugin-callback:config-rollback", struct {
			TransactionID string `json:"transaction-id"`
		}{TransactionID: txID})
		return err
	}

	applyErr = errors.New("cannot listen on 127.0.0.1:53")
	apply()
	require.NoError(t, rollback("tx-1"), "a failed apply that undid itself owes no rollback")
	assert.Equal(t, 0, rollbacks)

	applyErr = fmt.Errorf("cannot listen on 127.0.0.1:53: %w", ErrRunningConfigLost)
	apply()
	err := rollback("tx-2")
	require.Error(t, err, "the rollback of a lost running config must answer the restore failure")
	assert.Contains(t, err.Error(), errRestore.Error())
	assert.Equal(t, 1, rollbacks)

	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:bye", struct {
		Reason string `json:"reason"`
	}{Reason: "done"}))
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit")
	}
}
