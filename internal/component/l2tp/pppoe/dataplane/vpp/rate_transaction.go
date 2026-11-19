// Design: docs/architecture/l2tp/bng-5-pppoe.md -- confirmed two-direction rate changes
// Related: rate_owner.go -- native objects retained until safe cleanup.
package vpp

import (
	"context"
	"errors"
	"fmt"
)

// A transaction is a package-owned capability. The command owner MUST bind its
// token to the authenticated caller and exact admitted session. A caller MUST
// commit only after all authorization owners applied, or roll back on failure.
// Aliases remain valid Go pointers, so every method checks current ownership
// under the session's rate mutex before any native operation.
type policerRateTransaction struct {
	owner  *policerRates
	before [2]*policerPlan
	after  [2]*policerPlan
}

type policerRateChange interface {
	transaction() *policerRateTransaction
}
type preparedPolicerRate struct{ change *policerRateTransaction }
type attemptedPolicerRate struct{ change *policerRateTransaction }
type appliedPolicerRate struct{ change *policerRateTransaction }
type committedPolicerRate struct{ change *policerRateTransaction }
type rolledBackPolicerRate struct{ change *policerRateTransaction }

func (c *preparedPolicerRate) transaction() *policerRateTransaction   { return c.change }
func (c *attemptedPolicerRate) transaction() *policerRateTransaction  { return c.change }
func (c *appliedPolicerRate) transaction() *policerRateTransaction    { return c.change }
func (c *committedPolicerRate) transaction() *policerRateTransaction  { return c.change }
func (c *rolledBackPolicerRate) transaction() *policerRateTransaction { return c.change }

// Prepare validates exact native units and the current indexed ownership.
// The caller MUST supply both authorized rates and their burst sizes; an
// omitted CoA upload rate is resolved against its accepted profile before here.
// Preparation changes no native state. At most one transaction may be pending.
func (o *policerRates) Prepare(ctx context.Context, download, upload, downloadBurst, uploadBurst uint64) (*policerRateTransaction, error) {
	if ctx == nil {
		return nil, errors.New("pppoe-vpp: policer preparation requires a context")
	}
	rx, err := newPolicerPlan(o.prefix+"/rx", upload, uploadBurst)
	if err != nil {
		return nil, err
	}
	tx, err := newPolicerPlan(o.prefix+"/tx", download, downloadBurst)
	if err != nil {
		return nil, err
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if err := ctx.Err(); err != nil { return nil, err }
	if o.closed {
		return nil, errors.New("pppoe-vpp: session policer owner is closed")
	}
	if o.pending != nil {
		return nil, errors.New("pppoe-vpp: another rate transaction is pending")
	}
	change := &policerRateTransaction{owner: o, after: [2]*policerPlan{rx, tx}}
	bounded, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()
	for direction, ownership := range o.objects {
		switch object := ownership.(type) {
		case nil:
		case *unknownPolicer:
			return nil, errors.New("pppoe-vpp: previous policer creation remains unconfirmed")
		case *ownedPolicer:
			if object.binding != policerBound {
				return nil, errors.New("pppoe-vpp: previous policer binding is not confirmed active")
			}
			if err := o.api.verifyPolicer(bounded, object.index, object.wanted); err != nil {
				return nil, err
			}
			change.before[direction] = object.wanted
		}
	}
	o.pending = &preparedPolicerRate{change: change}
	o.last = nil // An older token cannot compensate a newer authorization.
	return change, nil
}

func (change *policerRateTransaction) Apply(ctx context.Context) error {
	if ctx == nil {
		return errors.New("pppoe-vpp: policer apply requires a context")
	}
	o := change.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	state, err := o.transactionState(change)
	if err != nil {
		return err
	}
	switch state.(type) {
	case *appliedPolicerRate, *committedPolicerRate:
		return nil
	case *attemptedPolicerRate, *rolledBackPolicerRate:
		return errors.New("pppoe-vpp: rate transaction is no longer prepared")
	case *preparedPolicerRate:
	}
	// Retain the compensation authority before an API request can mutate.
	o.pending = &attemptedPolicerRate{change: change}
	bounded, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()
	if err := o.apply(bounded, change); err != nil {
		cleanup, finish := context.WithTimeout(context.WithoutCancel(ctx), apiTimeout)
		defer finish()
		return errors.Join(err, o.restore(cleanup, change))
	}
	o.pending = &appliedPolicerRate{change: change}
	return nil
}

func (o *policerRates) apply(ctx context.Context, change *policerRateTransaction) error {
	for direction, plan := range change.after {
		if o.objects[direction] == nil {
			receipt, err := o.api.createPolicer(ctx, plan)
			if receipt.uncertain {
				o.objects[direction] = &unknownPolicer{wanted: plan}
			}
			if receipt.confirmed {
				if receipt.index == noInterface {
					o.objects[direction] = &unknownPolicer{wanted: plan}
				} else {
					o.objects[direction] = &ownedPolicer{index: receipt.index, wanted: plan, binding: policerUnbound}
				}
			}
			if err != nil {
				return err
			}
		}
		object, ok := o.objects[direction].(*ownedPolicer)
		if !ok {
			return errors.New("pppoe-vpp: policer creation did not provide an owned valid index")
		}
		if err := o.requireOwned(ctx, object); err != nil {
			return err
		}
		if change.before[direction] != nil {
			// PolicerUpdate writes the same pool slot and never frees its index.
			// Preserve before until both directions are confirmed or restored.
			if _, err := o.api.updatePolicer(ctx, object.index, plan); err != nil {
				return err
			}
		}
		if err := o.api.verifyPolicer(ctx, object.index, plan); err != nil {
			return err
		}
		object.wanted = plan
	}
	for direction, ownership := range o.objects {
		object := ownership.(*ownedPolicer) // Both fixed slots validated above.
		if object.binding == policerBound {
			continue
		}
		object.binding = policerBindingUnknown
		receipt, err := o.bind(ctx, direction, object.index, true)
		switch receipt {
		case policerConfirmed:
			object.binding = policerBound
		case policerRefused:
			object.binding = policerUnbound
		case policerUnknown:
		}
		if err != nil {
			return err
		}
		if object.binding != policerBound {
			return errors.New("pppoe-vpp: policer binding was not confirmed")
		}
		if direction == 0 {
			if err := o.api.setPolicerInputFeatures(ctx, o.index, true); err != nil {
				return err
			}
		}
	}
	return nil
}

func (change *policerRateTransaction) Commit() error {
	o := change.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	state, err := o.transactionState(change)
	if err != nil {
		return err
	}
	switch state.(type) {
	case *committedPolicerRate:
		return nil
	case *preparedPolicerRate, *attemptedPolicerRate, *rolledBackPolicerRate:
		return errors.New("pppoe-vpp: rate transaction has not been applied")
	case *appliedPolicerRate:
	}
	o.last, o.pending = &committedPolicerRate{change: change}, nil
	return nil
}

func (change *policerRateTransaction) Rollback(ctx context.Context) error {
	if ctx == nil {
		return errors.New("pppoe-vpp: policer rollback requires a context")
	}
	o := change.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	state, err := o.transactionState(change)
	if err != nil {
		return err
	}
	switch state.(type) {
	case *rolledBackPolicerRate:
		return nil
	case *preparedPolicerRate:
		o.last, o.pending = &rolledBackPolicerRate{change: change}, nil
		return nil
	case *attemptedPolicerRate, *appliedPolicerRate, *committedPolicerRate:
	}
	bounded, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()
	return o.restore(bounded, change)
}

func (o *policerRates) restore(ctx context.Context, change *policerRateTransaction) error {
	var result error
	if change.before[0] == nil {
		result = o.cleanup(ctx)
	} else {
		for direction, plan := range change.before {
			object, ok := o.objects[direction].(*ownedPolicer)
			if !ok {
				result = errors.Join(result, errors.New("pppoe-vpp: compensation lost its indexed policer owner"))
				continue
			}
			if err := o.requireOwned(ctx, object); err != nil {
				result = errors.Join(result, err)
				continue
			}
			_, mutationErr := o.api.updatePolicer(ctx, object.index, plan)
			readErr := o.api.verifyPolicer(ctx, object.index, plan)
			result = errors.Join(result, mutationErr, readErr)
			if readErr == nil {
				object.wanted = plan
			}
		}
	}
	if result != nil {
		return fmt.Errorf("restore subscriber policers: %w", result)
	}
	o.last, o.pending = &rolledBackPolicerRate{change: change}, nil
	return nil
}

func (o *policerRates) transactionState(change *policerRateTransaction) (policerRateChange, error) {
	if o.closed {
		return nil, errors.New("pppoe-vpp: session policer lifetime ended")
	}
	for _, state := range [...]policerRateChange{o.pending, o.last} {
		if state != nil {
			if state.transaction() == change {
				return state, nil
			}
		}
	}
	return nil, errors.New("pppoe-vpp: rate transaction no longer belongs to this session")
}
