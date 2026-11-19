// Design: docs/architecture/l2tp/bng-5-pppoe.md -- retained control-map ownership
// Related: access_map.go -- reciprocal identity and captured API generations
package vpp

import (
	"context"
	"errors"

	vppcomp "github.com/ze-software/ze/internal/component/vpp"
)

func (o *controlMapOwners) reconcileRetained(ctx context.Context) (int, error) {
	return o.reconcileCovered(ctx, o.processExited)
}

func (o *controlMapOwners) reconcileCovered(ctx context.Context, covered func(controlMapKey) bool) (int, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-o.gate:
	}
	defer func() { o.gate <- struct{}{} }()
	var failure error
	for key, entry := range o.entries {
		exited := covered(key)
		switch owner := entry.(type) {
		case *controlMapUnconfirmed:
			if exited {
				delete(o.entries, key)
				clearMapReconciliation(key)
			} else {
				reportMapReconciliation(key, owner.failure)
			}
		case *controlMapLeaseState:
			if exited {
				if owner.references == 0 {
					delete(o.entries, key)
				} else {
					owner.owned = false
				}
				clearMapReconciliation(key)
				continue
			}
			if owner.references != 0 {
				continue
			}
			if !owner.owned {
				delete(o.entries, key)
				continue
			}
			if err := o.remove(ctx, key); err != nil {
				reportMapReconciliation(key, err)
				failure = errors.Join(failure, err)
			} else {
				clearMapReconciliation(key)
			}
		}
	}
	pending := 0
	for _, entry := range o.entries {
		switch owner := entry.(type) {
		case *controlMapUnconfirmed:
			pending++
		case *controlMapLeaseState:
			if owner.references == 0 && owner.owned {
				pending++
			}
		}
	}
	return pending, failure
}

func (o *controlMapOwners) processExited(key controlMapKey) bool {
	switch owner := o.entries[key].(type) {
	case *controlMapUnconfirmed:
		return owner.api.processExited()
	case *controlMapLeaseState:
		return owner.api.processExited()
	}
	return false
}

// Only exact numeric exit proofs cross the component boundary. Existing
// bounded lease owners retain the local fact for active session aliases too.
func (o *controlMapOwners) retireProcess(proof vppcomp.ProcessExit) {
	o.retireCovered(func(key controlMapKey) bool {
		if key.connector == nil {
			return false
		}
		return proof.Covers(key.connector.Identity(), key.generation)
	})
}

func (o *controlMapOwners) retireCovered(covered func(controlMapKey) bool) {
	<-o.gate
	defer func() { o.gate <- struct{}{} }()
	for key, entry := range o.entries {
		if !covered(key) {
			continue
		}
		switch owner := entry.(type) {
		case *controlMapUnconfirmed:
			if owner.api != nil {
				owner.api.retired.Store(true)
			}
			delete(o.entries, key)
		case *controlMapLeaseState:
			if owner.api != nil {
				owner.api.retired.Store(true)
			}
			owner.owned = false
			if owner.references == 0 {
				delete(o.entries, key)
			}
		}
		clearMapReconciliation(key)
	}
}
