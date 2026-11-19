// Design: docs/architecture/l2tp/bng-5-pppoe.md -- bounded session policer ownership
// Related: rate_transaction.go -- two-direction replacement and compensation.
package vpp

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"go.fd.io/govpp/binapi/policer"
)

// policerRateAPI is the generation-bound API the session already captured.
// Implementations MUST distinguish an unavailable read from confirmed absence.
// The private method set keeps native index ownership inside this package.
type policerRateAPI interface {
	createPolicer(context.Context, *policerPlan) (creation, error)
	readPolicer(context.Context, uint32, string) (*policer.PolicerDetails, error)
	verifyPolicer(context.Context, uint32, *policerPlan) error
	bindPolicerInput(context.Context, uint32, uint32, bool) (policerReceipt, error)
	bindPolicerOutput(context.Context, uint32, uint32, bool) (policerReceipt, error)
	setPolicerInputFeatures(context.Context, uint32, bool) error
	quarantinePolicers(context.Context, uint32, bool, bool) error
	updatePolicer(context.Context, uint32, *policerPlan) (policerReceipt, error)
	deletePolicer(context.Context, uint32) (policerReceipt, error)
}

// policerRates owns exactly two directional objects, at most one pending
// authorization change and one terminal outcome. Its zero/nil are invalid;
// newPolicerRates validates its immutable interface identity and name prefix.
// Safe for concurrent use. The session MUST close this owner before recycling
// its native index, and MUST retain the session whenever Close cannot confirm
// removal. No worker, timer or second connection is owned here.
type policerRates struct {
	mu      sync.Mutex
	index   uint32
	prefix  string
	api     policerRateAPI
	objects [2]policerOwnership // RX/upload, then TX/download; never grows.
	pending policerRateChange
	last    policerRateChange
	closed  bool
}

func newPolicerRates(index uint32, prefix string, client policerRateAPI) (*policerRates, error) {
	if index == 0 {
		return nil, errors.New("pppoe-vpp: session policers cannot target local0")
	}
	if index == noInterface {
		return nil, errors.New("pppoe-vpp: session policers require a native interface index")
	}
	if client == nil {
		return nil, errors.New("pppoe-vpp: session policers require their captured API")
	}
	// All implementations of this private interface are package-owned pointers.
	if reflect.ValueOf(client).IsNil() {
		return nil, errors.New("pppoe-vpp: session policer API is a typed nil")
	}
	if !strings.HasPrefix(prefix, "ze-pppoe/") {
		return nil, errors.New("pppoe-vpp: session policer name must use its owned ze-pppoe namespace")
	}
	if _, err := newPolicerPlan(prefix+"/rx", 1000, 1); err != nil {
		return nil, err
	}
	return &policerRates{index: index, prefix: prefix, api: client}, nil
}

type policerOwnership interface {
	plan() *policerPlan
	policerOwnership()
}

// An unknown creation reserves only its exact name. The pinned dump API does
// not return indices, so this variant deliberately has no index to delete.
type unknownPolicer struct{ wanted *policerPlan }

func (p *unknownPolicer) plan() *policerPlan { return p.wanted }
func (*unknownPolicer) policerOwnership()    {}

// ownedPolicer exists only after an acknowledged valid pool index. Zero is a
// valid pool index. binding is a raw mutation receipt, not an inferred feature
// state: an unknown first bind may not have allocated VPP's slot vector.
type ownedPolicer struct {
	index   uint32
	wanted  *policerPlan
	binding policerBinding
}

func (p *ownedPolicer) plan() *policerPlan { return p.wanted }
func (*ownedPolicer) policerOwnership()    {}

type policerBinding uint8

const (
	policerBindingUnspecified policerBinding = iota
	policerUnbound
	policerBound
	policerBindingUnknown
)

// Close invalidates pending/last transactions before touching native state.
// It confirms every possibly active policer feature off before any unbind or
// deletion. Known failed cleanups may be retried; unknown creates/binds remain
// quarantined until the captured VPP generation ends, never guessed or rebound.
func (o *policerRates) Close(ctx context.Context) error {
	if ctx == nil {
		return errors.New("pppoe-vpp: policer cleanup requires a context")
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.closed = true
	o.pending, o.last = nil, nil
	bounded, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()
	return o.cleanup(bounded)
}

// cleanup MUST run under mu, including during compensation of initial setup.
// Its fixed two slots are also the bound on cleanup after partial acquisition.
func (o *policerRates) cleanup(ctx context.Context) error {
	var possible [2]bool
	for direction, ownership := range o.objects {
		if object, ok := ownership.(*ownedPolicer); ok {
			possible[direction] = object.binding != policerUnbound
		}
	}
	if policerBindingPossible(possible) {
		if err := o.api.quarantinePolicers(ctx, o.index, possible[0], possible[1]); err != nil {
			return err
		}
	}
	var result error
	for direction, ownership := range o.objects {
		switch object := ownership.(type) {
		case nil:
		case *unknownPolicer:
			result = errors.Join(result, fmt.Errorf("pppoe-vpp: creation of policer %q remains unconfirmed; its session index stays quarantined", object.wanted.name))
		case *ownedPolicer:
			result = errors.Join(result, o.cleanupObject(ctx, direction, object))
		}
	}
	return result
}

func policerBindingPossible(possible [2]bool) bool {
	if possible[0] {
		return true
	}
	return possible[1]
}

func (o *policerRates) cleanupObject(ctx context.Context, direction int, object *ownedPolicer) error {
	if object.binding == policerBindingUnknown {
		return fmt.Errorf("pppoe-vpp: binding of policer %q remains unconfirmed; refusing an unsafe unbind", object.wanted.name)
	}
	var result error
	if object.binding == policerBound {
		if err := o.requireOwned(ctx, object); err != nil {
			return err
		}
		object.binding = policerBindingUnknown
		receipt, err := o.bind(ctx, direction, object.index, false)
		result = errors.Join(result, err)
		switch receipt {
		case policerConfirmed:
			object.binding = policerUnbound
		case policerRefused:
			object.binding = policerBound
		case policerUnknown:
		}
		if object.binding != policerUnbound {
			return errors.Join(result, errors.New("pppoe-vpp: policer unbind was not confirmed"))
		}
	}
	found, err := o.api.readPolicer(ctx, object.index, object.wanted.name)
	if err != nil {
		return errors.Join(result, err)
	}
	if found == nil {
		o.objects[direction] = nil
		return result
	}
	receipt, err := o.api.deletePolicer(ctx, object.index)
	result = errors.Join(result, err)
	if receipt == policerConfirmed {
		o.objects[direction] = nil
		return result
	}
	// A lost delete receipt is recoverable only by this exact indexed read.
	found, err = o.api.readPolicer(ctx, object.index, object.wanted.name)
	result = errors.Join(result, err)
	if err == nil {
		if found == nil {
			o.objects[direction] = nil
		}
	}
	if o.objects[direction] != nil {
		return errors.Join(result, errors.New("pppoe-vpp: policer deletion remains unconfirmed"))
	}
	return result
}

func (o *policerRates) requireOwned(ctx context.Context, object *ownedPolicer) error {
	found, err := o.api.readPolicer(ctx, object.index, object.wanted.name)
	if err != nil {
		return err
	}
	if found == nil {
		return errors.New("pppoe-vpp: owned bound policer is absent; its slot cannot be safely unbound")
	}
	return nil
}

func (o *policerRates) bind(ctx context.Context, direction int, index uint32, apply bool) (policerReceipt, error) {
	if direction == 0 {
		return o.api.bindPolicerInput(ctx, o.index, index, apply)
	}
	return o.api.bindPolicerOutput(ctx, o.index, index, apply)
}
