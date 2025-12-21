package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTransactionReactor extends mockReactor with transaction support.
type mockTransactionReactor struct {
	mockReactor // Embed base mock

	inTransaction bool
	transactionID string
	txError       error // Error to return from transaction operations
}

func (m *mockTransactionReactor) BeginTransaction(label string) error {
	if m.txError != nil {
		return m.txError
	}
	if m.inTransaction {
		return ErrAlreadyInTransaction
	}
	m.inTransaction = true
	m.transactionID = label
	return nil
}

func (m *mockTransactionReactor) CommitTransaction() (TransactionResult, error) {
	return m.CommitTransactionWithLabel("")
}

func (m *mockTransactionReactor) CommitTransactionWithLabel(label string) (TransactionResult, error) {
	if m.txError != nil {
		return TransactionResult{}, m.txError
	}
	if !m.inTransaction {
		return TransactionResult{}, ErrNoTransaction
	}
	if label != "" && m.transactionID != label {
		return TransactionResult{}, ErrLabelMismatch
	}

	result := TransactionResult{
		RoutesAnnounced: len(m.announcedRoutes),
		RoutesWithdrawn: len(m.withdrawnRoutes),
		UpdatesSent:     1,
		Families:        []string{"ipv4 unicast"},
		TransactionID:   m.transactionID,
	}

	m.inTransaction = false
	m.transactionID = ""
	m.announcedRoutes = nil
	m.withdrawnRoutes = nil

	return result, nil
}

func (m *mockTransactionReactor) RollbackTransaction() (TransactionResult, error) {
	if m.txError != nil {
		return TransactionResult{}, m.txError
	}
	if !m.inTransaction {
		return TransactionResult{}, ErrNoTransaction
	}

	discarded := len(m.announcedRoutes) + len(m.withdrawnRoutes)
	result := TransactionResult{
		RoutesDiscarded: discarded,
		TransactionID:   m.transactionID,
	}

	m.inTransaction = false
	m.transactionID = ""
	m.announcedRoutes = nil
	m.withdrawnRoutes = nil

	return result, nil
}

func (m *mockTransactionReactor) InTransaction() bool {
	return m.inTransaction
}

func (m *mockTransactionReactor) TransactionID() string {
	return m.transactionID
}

// TestCommitStart verifies commit start command.
//
// VALIDATES: BeginTransaction called with label.
//
// PREVENTS: Transaction not starting.
func TestCommitStart(t *testing.T) {
	reactor := &mockTransactionReactor{}
	ctx := &CommandContext{Reactor: reactor}

	resp, err := handleCommitStart(ctx, []string{"batch1"})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "batch1", data["transaction"])

	assert.True(t, reactor.inTransaction)
	assert.Equal(t, "batch1", reactor.transactionID)
}

// TestCommitStartNoLabel verifies commit start without label.
//
// VALIDATES: Empty label allowed.
//
// PREVENTS: Error on missing optional label.
func TestCommitStartNoLabel(t *testing.T) {
	reactor := &mockTransactionReactor{}
	ctx := &CommandContext{Reactor: reactor}

	resp, err := handleCommitStart(ctx, nil)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
	assert.True(t, reactor.inTransaction)
}

// TestCommitStartNested verifies nested transaction error.
//
// VALIDATES: Error returned if already in transaction.
//
// PREVENTS: Undefined nested transaction behavior.
func TestCommitStartNested(t *testing.T) {
	reactor := &mockTransactionReactor{inTransaction: true, transactionID: "first"}
	ctx := &CommandContext{Reactor: reactor}

	resp, err := handleCommitStart(ctx, []string{"second"})

	require.Error(t, err)
	assert.Equal(t, ErrAlreadyInTransaction, err)
	assert.Equal(t, "error", resp.Status)
	assert.Contains(t, resp.Error, "already")
}

// TestCommitEnd verifies commit end command.
//
// VALIDATES: CommitTransaction called, stats returned.
//
// PREVENTS: Routes not being sent on commit.
func TestCommitEnd(t *testing.T) {
	reactor := &mockTransactionReactor{
		inTransaction: true,
		transactionID: "batch1",
	}
	// Simulate queued routes
	reactor.announcedRoutes = make([]struct {
		selector string
		route    RouteSpec
	}, 3)

	ctx := &CommandContext{Reactor: reactor}

	resp, err := handleCommitEnd(ctx, []string{"batch1"})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 3, data["routes_announced"])
	assert.Equal(t, "batch1", data["transaction"])

	assert.False(t, reactor.inTransaction)
}

// TestCommitEndNoTransaction verifies error when not in transaction.
//
// VALIDATES: Error returned if no transaction active.
//
// PREVENTS: Commit outside transaction context.
func TestCommitEndNoTransaction(t *testing.T) {
	reactor := &mockTransactionReactor{}
	ctx := &CommandContext{Reactor: reactor}

	resp, err := handleCommitEnd(ctx, nil)

	require.Error(t, err)
	assert.Equal(t, ErrNoTransaction, err)
	assert.Equal(t, "error", resp.Status)
}

// TestCommitEndLabelMismatch verifies label matching.
//
// VALIDATES: Error if commit label doesn't match start label.
//
// PREVENTS: Committing wrong transaction.
func TestCommitEndLabelMismatch(t *testing.T) {
	reactor := &mockTransactionReactor{inTransaction: true, transactionID: "batch1"}
	ctx := &CommandContext{Reactor: reactor}

	resp, err := handleCommitEnd(ctx, []string{"batch2"})

	require.Error(t, err)
	assert.Equal(t, ErrLabelMismatch, err)
	assert.Equal(t, "error", resp.Status)
	assert.Contains(t, resp.Error, "mismatch")

	// Still in transaction
	assert.True(t, reactor.inTransaction)
}

// TestCommitRollback verifies rollback command.
//
// VALIDATES: RollbackTransaction called, discarded count returned.
//
// PREVENTS: Rollback not discarding routes.
func TestCommitRollback(t *testing.T) {
	reactor := &mockTransactionReactor{
		inTransaction: true,
		transactionID: "batch1",
	}
	// Simulate queued routes
	reactor.announcedRoutes = make([]struct {
		selector string
		route    RouteSpec
	}, 5)

	ctx := &CommandContext{Reactor: reactor}

	resp, err := handleCommitRollback(ctx, nil)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 5, data["routes_discarded"])

	assert.False(t, reactor.inTransaction)
}

// TestCommitRollbackNoTransaction verifies error when not in transaction.
//
// VALIDATES: Error returned if no transaction active.
//
// PREVENTS: Rollback outside transaction context.
func TestCommitRollbackNoTransaction(t *testing.T) {
	reactor := &mockTransactionReactor{}
	ctx := &CommandContext{Reactor: reactor}

	resp, err := handleCommitRollback(ctx, nil)

	require.Error(t, err)
	assert.Equal(t, ErrNoTransaction, err)
	assert.Equal(t, "error", resp.Status)
}

// TestCommitCommandsRegistered verifies commit commands are registered.
//
// VALIDATES: All commit commands available.
//
// PREVENTS: Missing commit commands.
func TestCommitCommandsRegistered(t *testing.T) {
	d := NewDispatcher()
	RegisterCommitHandlers(d)

	commands := []string{
		"commit start",
		"commit end",
		"commit rollback",
	}

	for _, cmd := range commands {
		c := d.Lookup(cmd)
		assert.NotNil(t, c, "command %q must be registered", cmd)
	}
}
