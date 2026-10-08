package sdk

import (
	"context"
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
		errCh <- p.Run(ctx, Registration{})
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
