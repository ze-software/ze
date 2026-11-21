// Design: docs/architecture/l2tp/bng-5-pppoe.md -- VPP control-map ownership
// Related: prepare.go -- access publication; api.go -- captured connection epoch
package vpp

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/interface_types"

	pppoeapi "github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane/vpp/binapi/pppoe"
	vppcomp "github.com/ze-software/ze/internal/component/vpp"
)

// controlPair contains the distinct actual VPP indices validated by access
// readback. It is private: only validateControlPair and scripted tests construct it.
type controlPair struct{ dataplane, control uint32 }
type controlMapKey struct {
	connector  *vppcomp.Connector
	generation uint64
	pair       controlPair
}

// A confirmed lease can own or borrow a map. An unconfirmed add has no lease
// and cannot acquire deletion authority merely because its exact pair appears.
type controlMapEntry interface{ controlMapEntry() }

type controlMapLeaseState struct {
	api        *binaryAPI
	owned      bool
	references int
}

func (*controlMapLeaseState) controlMapEntry() {}

type controlMapUnconfirmed struct {
	api     *binaryAPI
	failure error
}

func (*controlMapUnconfirmed) controlMapEntry() {}

// Retained failed mutations count toward this limit too. Refusal keeps an
// uncertain native map from becoming an unbounded admission-side cache.
const controlMapMax = 4096

// controlMapOwners serializes reciprocal map changes, including pairs which
// overlap on one index. The gate is cancellable; it is never held by a session
// packet path. Failed cleanup retains its confirmed provenance, while unknown
// adds reserve both indices without claiming authority to remove the map.
type controlMapOwners struct {
	gate    chan struct{}
	entries map[controlMapKey]controlMapEntry
}

func newControlMapOwners() *controlMapOwners {
	owners := &controlMapOwners{gate: make(chan struct{}, 1), entries: make(map[controlMapKey]controlMapEntry)}
	owners.gate <- struct{}{}
	return owners
}

// acquire retains this exact map pair. The caller MUST Close the returned lease
// after every session and preparation consumer has released it.
func (o *controlMapOwners) acquire(ctx context.Context, connection *binaryAPI, pair controlPair) (*controlMapLease, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-o.gate:
	}
	defer func() { o.gate <- struct{}{} }()
	key := controlMapKey{connector: connection.connector, generation: connection.generation, pair: pair}
	if err := o.refuseOverlap(key); err != nil {
		return nil, err
	}
	present, err := connection.controlPairPresent(ctx, pair)
	if err != nil {
		return nil, err
	}
	if entry, ok := o.entries[key].(*controlMapLeaseState); ok {
		if present {
			entry.references++
			return &controlMapLease{owners: o, key: key}, nil
		}
		if entry.references != 0 {
			return nil, errors.New("pppoe-vpp: an active control mapping disappeared")
		}
		// Both native directions now confirm that the retained cleanup ended.
		delete(o.entries, key)
	}
	if len(o.entries) >= controlMapMax {
		return nil, errors.New("pppoe-vpp: control mapping ownership limit reached; unresolved mappings must be reconciled before admitting another access")
	}
	owned := false
	if !present {
		receipt, changeErr := connection.changeControlPair(ctx, pair, true)
		err = changeErr
		if receipt == controlMutationUnknown {
			o.entries[key] = &controlMapUnconfirmed{api: connection, failure: err}
			reportMapReconciliation(key, err)
			return nil, err
		}
		owned = receipt == controlMutationConfirmed
		if owned {
			o.entries[key] = &controlMapLeaseState{api: connection, owned: true}
		}
		if err != nil {
			if owned {
				err = errors.Join(err, o.rollback(key))
			}
			return nil, err
		}
		present, err = connection.controlPairPresent(ctx, pair)
		if err != nil {
			return nil, errors.Join(err, o.rollback(key))
		}
		if !present {
			return nil, errors.Join(errors.New("pppoe-vpp: created control mapping is absent from readback"), o.rollback(key))
		}
	}
	o.entries[key] = &controlMapLeaseState{api: connection, owned: owned, references: 1}
	return &controlMapLease{owners: o, key: key}, nil
}

func (o *controlMapOwners) refuseOverlap(key controlMapKey) error {
	// At most controlMapMax entries exist; this admission path is not a packet
	// path. Either native index can alias an earlier uncertain mutation.
	for existing, entry := range o.entries {
		if !controlPairsOverlap(existing.pair, key.pair) {
			continue
		}
		if o.processExited(existing) {
			continue
		}
		if existing.connector != key.connector {
			return errors.New("pppoe-vpp: overlapping control mapping belongs to an unretired API owner")
		}
		if existing.generation != key.generation {
			return errors.New("pppoe-vpp: overlapping control mapping belongs to an unretired connection generation")
		}
		switch owner := entry.(type) {
		case *controlMapUnconfirmed:
			return errors.Join(errors.New("pppoe-vpp: interface has an unconfirmed control mapping mutation"), owner.failure)
		case *controlMapLeaseState:
			if existing.pair != key.pair {
				return errors.New("pppoe-vpp: interface still belongs to another retained control mapping lease")
			}
		}
	}
	return nil
}

func controlPairsOverlap(left, right controlPair) bool {
	if left.dataplane == right.dataplane {
		return true
	}
	if left.dataplane == right.control {
		return true
	}
	if left.control == right.dataplane {
		return true
	}
	return left.control == right.control
}

// controlMapLease MUST close after its access sessions and discovery stop.
// Close is concurrent and idempotent; only the final owned reference removes
// the pair. A pre-existing exact pair is borrowed and is never deleted.
type controlMapLease struct {
	owners *controlMapOwners
	key    controlMapKey
	once   sync.Once
	err    error
}

func (l *controlMapLease) Close() error {
	l.once.Do(func() { l.err = l.owners.release(l.key) })
	return l.err
}

func (o *controlMapOwners) release(key controlMapKey) error {
	// Close cannot abandon its retained reference when a competing admitted
	// map operation is still completing. Each such operation owns bounded API
	// streams; start the cleanup deadline after obtaining serialization.
	<-o.gate
	ctx, cancel := context.WithTimeout(context.Background(), 4*apiTimeout)
	defer cancel()
	defer func() { o.gate <- struct{}{} }()
	entry, ok := o.entries[key].(*controlMapLeaseState)
	if !ok {
		return errors.New("pppoe-vpp: control mapping lease has no owner")
	}
	entry.references--
	if entry.references > 0 {
		return nil
	}
	if !entry.owned {
		delete(o.entries, key)
		return nil
	}
	return o.remove(ctx, key)
}

// remove MUST run under gate after the final lease ends or publication fails.
// It retains deletion authority until the native result proves removal.
func (o *controlMapOwners) remove(ctx context.Context, key controlMapKey) error {
	entry, ok := o.entries[key].(*controlMapLeaseState)
	if !ok {
		return errors.New("pppoe-vpp: control mapping removal has no confirmed owner")
	}
	if entry.api.processExited() {
		delete(o.entries, key)
		clearMapReconciliation(key)
		return nil
	}
	absent, err := entry.api.removeControlPairContext(ctx, key.pair)
	switch {
	case absent:
		delete(o.entries, key)
		clearMapReconciliation(key)
	case entry.api.processExited():
		delete(o.entries, key)
		clearMapReconciliation(key)
		return nil
	default:
		reportMapReconciliation(key, err)
	}
	return err
}

// rollback MUST run under gate for a confirmed, unpublished mutation. Its
// bounded cleanup remains possible after the admission context is canceled.
func (o *controlMapOwners) rollback(key controlMapKey) error {
	ctx, cancel := context.WithTimeout(context.Background(), 4*apiTimeout)
	defer cancel()
	return o.remove(ctx, key)
}

func (a *binaryAPI) readControlBinding(ctx context.Context, index uint32) (pair controlPair, present bool, err error) {
	seen := false
	err = a.dump(ctx, &pppoeapi.PppoeCpBindingDump{SwIfIndex: interface_types.InterfaceIndex(index)}, 1,
		func(message api.Message) error {
			details, ok := message.(*pppoeapi.PppoeCpBindingDetails)
			if !ok {
				return fmt.Errorf("pppoe-vpp: control mapping reply %T", message)
			}
			if details == nil {
				return errors.New("pppoe-vpp: control mapping returned nil details")
			}
			if seen {
				return errors.New("pppoe-vpp: targeted control query returned several mappings")
			}
			seen = true
			pair = controlPair{uint32(details.DpSwIfIndex), uint32(details.CpSwIfIndex)}
			// The pinned targeted handler reports a missing binding explicitly,
			// using INVALID_INTERFACE and both all-ones indices, then ping.
			if details.Retval == int32(api.INVALID_INTERFACE) {
				if pair.dataplane == noInterface {
					if pair.control == noInterface {
						return nil
					}
				}
			}
			if details.Retval != 0 {
				return api.RetvalToVPPApiError(details.Retval)
			}
			if index != pair.dataplane {
				if index != pair.control {
					return errors.New("pppoe-vpp: control reply names another interface")
				}
			}
			present = true
			return nil
		})
	if err != nil {
		return controlPair{}, false, err
	}
	if !seen {
		return controlPair{}, false, errors.New("pppoe-vpp: targeted control query omitted its answer")
	}
	return pair, present, nil
}

func (a *binaryAPI) controlPairPresent(ctx context.Context, wanted controlPair) (bool, error) {
	forward, hasForward, err := a.readControlBinding(ctx, wanted.dataplane)
	if err != nil {
		return false, err
	}
	reverse, hasReverse, err := a.readControlBinding(ctx, wanted.control)
	if err != nil {
		return false, err
	}
	if hasForward {
		if forward != wanted {
			return false, errors.New("pppoe-vpp: data interface belongs to another control mapping")
		}
	}
	if hasReverse {
		if reverse != wanted {
			return false, errors.New("pppoe-vpp: control interface belongs to another data mapping")
		}
	}
	if hasForward != hasReverse {
		return false, errors.New("pppoe-vpp: control mapping is not reciprocal")
	}
	return hasForward, nil
}

type controlMutation uint8

const (
	controlMutationUnspecified controlMutation = iota
	controlMutationConfirmed
	controlMutationRefused
	controlMutationUnknown
)

// changeControlPair retains a successful receipt even when stream close fails.
// A timeout without a successful reply does not prove who owns the live map.
func (a *binaryAPI) changeControlPair(ctx context.Context, pair controlPair, add bool) (controlMutation, error) {
	isAdd := uint8(0)
	if add {
		isAdd = 1
	}
	message, err := a.exchange(ctx, &pppoeapi.PppoeAddDelCp{DpSwIfIndex: interface_types.InterfaceIndex(pair.dataplane),
		CpSwIfIndex: interface_types.InterfaceIndex(pair.control), IsAdd: isAdd})
	if requestWasNotSent(err) {
		return controlMutationRefused, err
	}
	reply, ok := message.(*pppoeapi.PppoeAddDelCpReply)
	if !ok {
		return controlMutationUnknown, errors.Join(err, fmt.Errorf("pppoe-vpp: unconfirmed control mapping mutation, reply %T", message))
	}
	if reply == nil {
		return controlMutationUnknown, errors.Join(err, errors.New("pppoe-vpp: control mutation returned no reply"))
	}
	if reply.Retval != 0 {
		return controlMutationRefused, errors.Join(err, api.RetvalToVPPApiError(reply.Retval))
	}
	return controlMutationConfirmed, err
}

func (a *binaryAPI) removeControlPairContext(ctx context.Context, pair controlPair) (bool, error) {
	present, err := a.controlPairPresent(ctx, pair)
	if err != nil {
		return false, err
	}
	if !present {
		return true, nil
	}
	receipt, err := a.changeControlPair(ctx, pair, false)
	if receipt == controlMutationConfirmed {
		return true, err
	}
	// A lost reply may have followed a completed native deletion. Read both
	// directions before forgetting authority, while preserving its error.
	present, readErr := a.controlPairPresent(ctx, pair)
	if readErr != nil {
		return false, errors.Join(err, readErr)
	}
	return !present, err
}
