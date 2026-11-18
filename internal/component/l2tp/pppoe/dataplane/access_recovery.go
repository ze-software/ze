// Design: docs/architecture/l2tp/bng-5-pppoe.md -- original prepared access settlement
// Related: access.go -- immutable binding and cached first Close outcome
package dataplane

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
)

// AccessRecovery is the original preparation owner's explicit cleanup path.
// Close MUST stop admission and consume this owner's one reference. After it
// returned, Settle MUST reconcile only that same retained obligation, obey the
// context for native work and return nil only when cleanup and physical joins
// finished. Joining the original physical owner can outlast that context;
// abandoning its worker is never settlement.
// Implementations MUST serialize their native ownership and support concurrent
// Settle calls. Neither connection loss nor an absent replacement ledger is a
// release receipt. A successful caller MUST keep the access until Settle ends.
type AccessRecovery interface {
	io.Closer
	Settle(context.Context) error
}

// NewRecoverableAccess consumes owner on every outcome, exactly as NewAccess.
// It binds recovery to this concrete original owner without exposing that owner
// or permitting an embedded Access to acquire its settlement authority.
// Recovery MUST bind to the private constructed lease before interface publication.
func NewRecoverableAccess(info AccessInfo, owner AccessRecovery) (Access, error) {
	if err := validateAccess(info); err != nil {
		return nil, RefuseAccess(err, owner)
	}
	access, err := newAccess(info, owner)
	if err != nil {
		return nil, err
	}
	access.recovery = owner
	return access, nil
}

// SettleAccess MUST be called only after this exact access's Close returned and
// its sessions/discovery detached. It never calls Close again or changes that
// cached outcome. A provider without recovery retains its original failure.
// Native work gets at most twenty seconds or the earlier caller deadline.
// Existing physical ownership joins remain mandatory even after that deadline.
func SettleAccess(ctx context.Context, access Access) error {
	if nilResource(ctx) {
		return errors.New("PPPoE access settlement requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	original, ok := access.(*preparedAccess)
	if !ok {
		return errors.New("PPPoE access settlement requires the original constructed access")
	}
	if original == nil {
		return errors.New("PPPoE access settlement requires the original constructed access")
	}
	select {
	case <-original.closed:
	default:
		return errors.New("PPPoE access settlement requires completed original Close")
	}
	if original.closeErr == nil {
		return nil
	}
	if original.recovery == nil {
		return original.closeErr
	}
	bounded, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	failure := settleAccessOwner(bounded, original.recovery)
	failure = errors.Join(failure, bounded.Err())
	if failure != nil {
		return errors.Join(original.closeErr, failure)
	}
	return nil
}

func settleAccessOwner(ctx context.Context, owner AccessRecovery) (err error) {
	defer func() {
		if failure := recover(); failure != nil {
			if cause, ok := failure.(error); ok {
				err = fmt.Errorf("PPPoE access settlement panicked: %w", cause)
				return
			}
			err = fmt.Errorf("PPPoE access settlement panicked: %v", failure)
		}
	}()
	return owner.Settle(ctx)
}
