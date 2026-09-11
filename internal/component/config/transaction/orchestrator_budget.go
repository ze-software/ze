// Design: docs/architecture/config/transaction-protocol.md -- the transaction deadline
// Related: orchestrator.go -- the phases these deadlines bound
// Related: config_tx_bridge.go -- turns the deadline into each plugin RPC's context deadline

package transaction

import "time"

func (o *TxCoordinator) computeVerifyDeadline() time.Duration {
	if o.verifyDeadlineOverride > 0 {
		return o.verifyDeadlineOverride
	}
	return o.computeSequentialDeadline(func(p Participant) int { return p.VerifyBudget })
}

func (o *TxCoordinator) computeApplyDeadline() time.Duration {
	if o.applyDeadlineOverride > 0 {
		return o.applyDeadlineOverride
	}
	return o.computeSequentialDeadline(func(p Participant) int { return p.ApplyBudget })
}

// computeSequentialDeadline sums every participant's budget, because a phase
// reaches its participants one at a time and the deadline covers the whole
// phase.
//
// Nothing in a transaction phase runs concurrently, on either path. An engine
// handler fires inside the emitter's goroutine (Server.dispatchEngineEvent),
// and the bridge performs the plugin RPC inside that handler, so a publish
// returns only after the plugin has answered (configTxBridge, "Concurrency
// model"). The ordered path then applies one node at a time
// (OperationExecutor.Execute), and every node carries the ONE absolute
// instant runOperationPath derives from this duration, which the bridge turns
// into each RPC's context deadline (deadlineCtx). The last plugin in the
// sequence gets what the earlier ones left.
//
// So the phase can cost the sum of what its participants declared, and a
// deadline sized for one of them aborts a transaction that was applying
// correctly. Until 2026-09-11 this function took the max within a dependency
// tier and summed the tiers, on the premise that a tier ran concurrently. No
// tier ever did.
//
// The cost of the sum is the time a HUNG plugin takes to be noticed, which
// grows with the participant count. That is the trade: a late abort of a
// broken reload, against a false abort of a working one.
//
// A participant that declared no budget contributes nothing, and a set that
// declares nothing at all takes the 30-second default rather than a zero
// deadline, which would time out before the first plugin answered.
func (o *TxCoordinator) computeSequentialDeadline(budget func(Participant) int) time.Duration {
	totalSecs := 0
	for _, p := range o.participants {
		secs := capBudget(budget(p))
		if secs <= 0 {
			continue
		}
		totalSecs += secs
	}

	if totalSecs == 0 {
		return 30 * time.Second
	}
	return time.Duration(totalSecs) * time.Second
}

// capBudget clamps a budget to MaxBudgetSeconds.
func capBudget(secs int) int {
	if secs > MaxBudgetSeconds {
		return MaxBudgetSeconds
	}
	return secs
}

// updateParticipantApplyBudget updates a participant's apply budget.
// Caller MUST hold o.mu.
func (o *TxCoordinator) updateParticipantApplyBudget(name string, secs int) {
	secs = capBudget(secs)
	for i := range o.participants {
		if o.participants[i].Name == name {
			o.participants[i].ApplyBudget = secs
			return
		}
	}
}

// updateParticipantVerifyBudget updates a participant's verify budget.
// Caller MUST hold o.mu.
func (o *TxCoordinator) updateParticipantVerifyBudget(name string, secs int) {
	secs = capBudget(secs)
	for i := range o.participants {
		if o.participants[i].Name == name {
			o.participants[i].VerifyBudget = secs
			return
		}
	}
}

func (o *TxCoordinator) computeRollbackDeadline() time.Duration {
	o.mu.Lock()
	applyDL := o.applyDeadline
	o.mu.Unlock()
	if applyDL == 0 {
		return 90 * time.Second
	}
	return 3 * applyDL
}
