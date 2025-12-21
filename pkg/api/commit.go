package api

import "fmt"

// RegisterCommitHandlers registers commit-related command handlers.
func RegisterCommitHandlers(d *Dispatcher) {
	d.Register("commit start", handleCommitStart, "Begin a transaction for batched route updates")
	d.Register("commit end", handleCommitEnd, "Commit the current transaction")
	d.Register("commit rollback", handleCommitRollback, "Rollback the current transaction")
}

// handleCommitStart begins a new transaction.
// Usage: commit start [label].
func handleCommitStart(ctx *CommandContext, args []string) (*Response, error) {
	label := ""
	if len(args) > 0 {
		label = args[0]
	}

	peerSelector := ctx.NeighborSelector()

	if err := ctx.Reactor.BeginTransaction(peerSelector, label); err != nil {
		return &Response{
			Status: "error",
			Error:  fmt.Sprintf("failed to start transaction: %v", err),
		}, err
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"neighbor":    peerSelector,
			"transaction": label,
			"message":     "transaction started",
		},
	}, nil
}

// handleCommitEnd commits the current transaction.
// Usage: commit end [label].
func handleCommitEnd(ctx *CommandContext, args []string) (*Response, error) {
	var result TransactionResult
	var err error

	peerSelector := ctx.NeighborSelector()

	if len(args) > 0 {
		result, err = ctx.Reactor.CommitTransactionWithLabel(peerSelector, args[0])
	} else {
		result, err = ctx.Reactor.CommitTransaction(peerSelector)
	}

	if err != nil {
		return &Response{
			Status: "error",
			Error:  fmt.Sprintf("commit failed: %v", err),
		}, err
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"neighbor":         peerSelector,
			"routes_announced": result.RoutesAnnounced,
			"routes_withdrawn": result.RoutesWithdrawn,
			"updates_sent":     result.UpdatesSent,
			"eor_sent":         result.Families,
			"transaction":      result.TransactionID,
		},
	}, nil
}

// handleCommitRollback discards all queued routes in the current transaction.
// Usage: commit rollback [label].
func handleCommitRollback(ctx *CommandContext, args []string) (*Response, error) {
	peerSelector := ctx.NeighborSelector()

	result, err := ctx.Reactor.RollbackTransaction(peerSelector)
	if err != nil {
		return &Response{
			Status: "error",
			Error:  fmt.Sprintf("rollback failed: %v", err),
		}, err
	}

	txLabel := result.TransactionID

	// Note: label check on rollback is advisory only (not enforced).
	_ = args // Label argument is accepted but not validated.

	return &Response{
		Status: "done",
		Data: map[string]any{
			"neighbor":         peerSelector,
			"routes_discarded": result.RoutesDiscarded,
			"transaction":      txLabel,
		},
	}, nil
}
