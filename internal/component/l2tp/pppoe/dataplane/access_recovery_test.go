// Design: docs/architecture/l2tp/bng-5-pppoe.md -- original access settlement
// Related: access_recovery.go -- recovery never changes the first Close result
package dataplane

import (
	"context"
	"errors"
	"testing"
	"time"
)

type recoveryAccessOwner struct {
	closes, settlements         int
	closeFailure, settleFailure error
	deadline                    time.Time
}

func (o *recoveryAccessOwner) Close() error { o.closes++; return o.closeFailure }

func (*recoveryAccessOwner) Open(ctx context.Context, request Request) (OpenedSession, error) {
	return testUnusedOpen(ctx, request)
}
func (o *recoveryAccessOwner) Settle(ctx context.Context) error {
	o.settlements++
	var ok bool
	o.deadline, ok = ctx.Deadline()
	if !ok {
		return errors.New("settlement context has no deadline")
	}
	return o.settleFailure
}

// Goal: Settlement preserves the original Close result.
// Method: Repair the same owner, settle it, then repeat Close and check both call counts.
func TestAccessSettlementRetainsFirstCloseOutcome(t *testing.T) {
	original := errors.New("original map delete failed")
	owner := &recoveryAccessOwner{closeFailure: original}
	access, err := NewRecoverableAccess(testAccessInfo(), owner)
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(access.Close(), original) {
		t.Fatal("original Close failure lost")
	}
	if err := SettleAccess(t.Context(), access); err != nil {
		t.Fatal(err)
	}
	if owner.closes != 1 {
		t.Fatal("recovery replayed Close", owner.closes, owner.settlements)
	}
	if owner.settlements != 1 {
		t.Fatal("recovery replayed Close", owner.closes, owner.settlements)
	}
	if !errors.Is(access.Close(), original) {
		t.Fatal("settlement rewrote the cached Close result")
	}
	if owner.closes != 1 {
		t.Fatal("cached Close called its owner twice")
	}
}

// Goal: Settlement cannot consume an access that is still open.
// Method: Call SettleAccess before Close and require refusal without either owner call.
func TestAccessSettlementRequiresCompletedClose(t *testing.T) {
	owner := &recoveryAccessOwner{}
	access, err := NewRecoverableAccess(testAccessInfo(), owner)
	if err != nil {
		t.Fatal(err)
	}
	if err := SettleAccess(t.Context(), access); err == nil {
		t.Fatal("settled an access still admitting sessions")
	}
	if owner.closes != 0 {
		t.Fatal("settlement consumed the active owner")
	}
	if owner.settlements != 0 {
		t.Fatal("settlement consumed the active owner")
	}
	if err := access.Close(); err != nil {
		t.Fatal(err)
	}
}

// Goal: An ordinary failed owner cannot acquire recovery authority.
// Method: Close a NewAccess result unsuccessfully and require its original failure on settlement.
func TestAccessSettlementCannotInventMissingRecovery(t *testing.T) {
	failure := errors.New("ordinary owner cleanup failed")
	owner := &accessTestOwner{err: failure}
	access, err := NewAccess(testAccessInfo(), owner)
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(access.Close(), failure) {
		t.Fatal("setup did not fail Close")
	}
	if err := SettleAccess(t.Context(), access); !errors.Is(err, failure) {
		t.Fatal("missing recovery became success", err)
	}
	if owner.calls != 1 {
		t.Fatal("ordinary owner was closed again")
	}
}

// Goal: An ordinary successful owner remains settled without another Close.
// Method: Close a NewAccess result successfully, then settle it and count original owner calls.
func TestAccessSettlementPreservesStableKernelClose(t *testing.T) {
	owner := &accessTestOwner{}
	access, err := NewAccess(testAccessInfo(), owner)
	if err != nil {
		t.Fatal(err)
	}
	if err := access.Close(); err != nil {
		t.Fatal(err)
	}
	if err := SettleAccess(t.Context(), access); err != nil {
		t.Fatal(err)
	}
	if owner.calls != 1 {
		t.Fatal("stable owner was closed again")
	}
}

type embeddedRecoveryAccess struct{ Access }

// Goal: Embedding an access cannot borrow its settlement authority.
// Method: Close the original, pass an embedded alias and require refusal before owner settlement.
func TestAccessSettlementRefusesEmbeddedAlias(t *testing.T) {
	owner := &recoveryAccessOwner{}
	access, err := NewRecoverableAccess(testAccessInfo(), owner)
	if err != nil {
		t.Fatal(err)
	}
	if err := access.Close(); err != nil {
		t.Fatal(err)
	}
	if err := SettleAccess(t.Context(), embeddedRecoveryAccess{access}); err == nil {
		t.Fatal("foreign embedding acquired original settlement authority")
	}
	if owner.settlements != 0 {
		t.Fatal("embedded alias reached original owner")
	}
}

// Goal: Uncertain settlement retains both the first Close and current failure.
// Method: Give the same owner distinct errors and inspect both identities after SettleAccess.
func TestAccessSettlementKeepsOriginalAndCurrentFailures(t *testing.T) {
	original := errors.New("original close")
	current := errors.New("same owner still uncertain")
	owner := &recoveryAccessOwner{closeFailure: original, settleFailure: current}
	access, err := NewRecoverableAccess(testAccessInfo(), owner)
	if err != nil {
		t.Fatal(err)
	}
	_ = access.Close() //nolint:errcheck // These cases intentionally seed failed first Close before testing settlement.
	err = SettleAccess(t.Context(), access)
	if !errors.Is(err, original) {
		t.Fatal("settlement dropped original uncertainty", err)
	}
	if !errors.Is(err, current) {
		t.Fatal("settlement dropped original uncertainty", err)
	}
}

// Goal: Settlement never extends an earlier caller deadline.
// Method: Let the original owner record the actual bounded context deadline and compare it exactly.
func TestAccessSettlementUsesEarlierCallerDeadline(t *testing.T) {
	owner := &recoveryAccessOwner{closeFailure: errors.New("original close")}
	access, err := NewRecoverableAccess(testAccessInfo(), owner)
	if err != nil {
		t.Fatal(err)
	}
	_ = access.Close() //nolint:errcheck // These cases intentionally seed failed first Close before testing settlement.
	deadline := time.Now().Add(time.Second)
	ctx, cancel := context.WithDeadline(t.Context(), deadline)
	defer cancel()
	if err := SettleAccess(ctx, access); err != nil {
		t.Fatal(err)
	}
	if !owner.deadline.Equal(deadline) {
		t.Fatal("settlement extended the caller deadline")
	}
}

// Goal: An already canceled settlement cannot reach its owner.
// Method: Cancel the caller context after first Close and require cancellation with zero settlements.
func TestAccessSettlementRejectsCanceledContextBeforeOwner(t *testing.T) {
	owner := &recoveryAccessOwner{closeFailure: errors.New("original close")}
	access, err := NewRecoverableAccess(testAccessInfo(), owner)
	if err != nil {
		t.Fatal(err)
	}
	_ = access.Close() //nolint:errcheck // These cases intentionally seed failed first Close before testing settlement.
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := SettleAccess(ctx, access); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if owner.settlements != 0 {
		t.Fatal("canceled settlement reached its owner")
	}
}

// Goal: Settlement without a caller deadline has at most twenty seconds.
// Method: Record the actual owner context deadline and require the literal upper bound.
func TestAccessSettlementHasLiteralTwentySecondBudget(t *testing.T) {
	owner := &recoveryAccessOwner{closeFailure: errors.New("original close")}
	access, err := NewRecoverableAccess(testAccessInfo(), owner)
	if err != nil {
		t.Fatal(err)
	}
	_ = access.Close() //nolint:errcheck // These cases intentionally seed failed first Close before testing settlement.
	if err := SettleAccess(context.Background(), access); err != nil {
		t.Fatal(err)
	}
	if owner.deadline.After(time.Now().Add(20 * time.Second)) {
		t.Fatal("settlement extended its twenty-second budget")
	}
}

type cancelingAccessRecovery struct{ cancel context.CancelFunc }

func (*cancelingAccessRecovery) Close() error { return errors.New("original close") }

func (*cancelingAccessRecovery) Open(ctx context.Context, request Request) (OpenedSession, error) {
	return testUnusedOpen(ctx, request)
}
func (o *cancelingAccessRecovery) Settle(context.Context) error { o.cancel(); return nil }

// Goal: Cancellation during settlement cannot become success.
// Method: Have the same original owner cancel the input while returning nil and require cancellation.
func TestAccessSettlementCancellationCannotBecomeSuccess(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	access, err := NewRecoverableAccess(testAccessInfo(), &cancelingAccessRecovery{cancel: cancel})
	if err != nil {
		t.Fatal(err)
	}
	_ = access.Close() //nolint:errcheck // These cases intentionally seed failed first Close before testing settlement.
	if err := SettleAccess(ctx, access); !errors.Is(err, context.Canceled) {
		t.Fatal("late canceled settlement reported success", err)
	}
}
