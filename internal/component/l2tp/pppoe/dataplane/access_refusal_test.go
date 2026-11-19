// Design: docs/architecture/l2tp/bng-5-pppoe.md -- failed preparation ownership
// Related: access_refusal.go -- original error-owned cleanup receipts
package dataplane

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// Goal: A refused prepared access retains only its original cleanup owner.
// Method: Return the real constructed access with an error, settle its wrapped receipt and recheck cached Close.
func TestAccessPrepareRefusalRetainsExactRecoverableOwner(t *testing.T) {
	original := errors.New("provider refused preparation")
	cleanup := errors.New("original map cleanup failed")
	owner := &recoveryAccessOwner{closeFailure: cleanup}
	access, err := NewRecoverableAccess(testAccessInfo(), owner)
	if err != nil {
		t.Fatal(err)
	}
	var registry sessionRegistry
	if err := registry.register("refused", func(context.Context, string) (Access, error) { return access, original }); err != nil {
		t.Fatal(err)
	}
	backend, err := registry.lookup("refused")
	if err != nil {
		t.Fatal(err)
	}
	result, failure := backend.Prepare(t.Context(), "wan")
	if result != nil {
		t.Fatal("partial access/cause lost", result, failure)
	}
	if !errors.Is(failure, original) {
		t.Fatal("partial access/cause lost", result, failure)
	}
	if !errors.Is(failure, cleanup) {
		t.Fatal("partial access/cause lost", result, failure)
	}
	retained, err := SettleAccessFailure(t.Context(), fmt.Errorf("outer: %w", failure))
	if !retained {
		t.Fatal("original refused owner not settled", retained, err)
	}
	if err != nil {
		t.Fatal("original refused owner not settled", retained, err)
	}
	if owner.closes != 1 {
		t.Fatal("original refused owner not settled", retained, err)
	}
	if owner.settlements != 1 {
		t.Fatal("original refused owner not settled", retained, err)
	}
	if !errors.Is(access.Close(), cleanup) {
		t.Fatal("cached original Close changed")
	}
	if owner.closes != 1 {
		t.Fatal("cached original Close changed")
	}
}

// Goal: Ordinary and embedded results cannot acquire constructed recovery authority.
// Method: Refuse both through the real backend and require original cleanup failure without settlement.
func TestAccessPrepareForeignRefusalCannotAcquireRecovery(t *testing.T) {
	for _, foreign := range []bool{false, true} {
		t.Run(fmt.Sprint(foreign), func(t *testing.T) {
			cleanup := errors.New("original cleanup failed")
			owner := &recoveryAccessOwner{closeFailure: cleanup}
			var access Access
			var err error
			if foreign {
				access, err = NewRecoverableAccess(testAccessInfo(), owner)
				access = embeddedRecoveryAccess{access}
			} else {
				access, err = NewAccess(testAccessInfo(), owner)
			}
			if err != nil {
				t.Fatal(err)
			}
			var registry sessionRegistry
			if err := registry.register("partial", func(context.Context, string) (Access, error) { return access, errors.New("refused") }); err != nil {
				t.Fatal(err)
			}
			backend, err := registry.lookup("partial")
			if err != nil {
				t.Fatal(err)
			}
			result, failure := backend.Prepare(t.Context(), "wan")
			if result != nil {
				t.Fatal("foreign partial access escaped")
			}
			retained, err := SettleAccessFailure(t.Context(), failure)
			if !retained {
				t.Fatal("unrecoverable result gained cleanup authority", retained, err)
			}
			if !errors.Is(err, cleanup) {
				t.Fatal("unrecoverable result gained cleanup authority", retained, err)
			}
			if owner.closes != 1 {
				t.Fatal("unrecoverable result gained cleanup authority", retained, err)
			}
			if owner.settlements != 0 {
				t.Fatal("unrecoverable result gained cleanup authority", retained, err)
			}
		})
	}
}

// Goal: Invalid construction retains cleanup only when its original Close fails.
// Method: Construct the same invalid device with successful and failed owners, then settle the refusal.
func TestInvalidRecoverableAccessRetainsOnlyFailedCleanup(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(fmt.Sprint(failed), func(t *testing.T) {
			owner := &recoveryAccessOwner{}
			if failed {
				owner.closeFailure = errors.New("invalid access original cleanup failed")
			}
			info := testAccessInfo()
			info.Device = ""
			access, failure := NewRecoverableAccess(info, owner)
			if access != nil {
				t.Fatal("invalid access escaped or owner not consumed")
			}
			if failure == nil {
				t.Fatal("invalid access escaped or owner not consumed")
			}
			if owner.closes != 1 {
				t.Fatal("invalid access escaped or owner not consumed")
			}
			retained, err := SettleAccessFailure(t.Context(), failure)
			if retained != failed {
				t.Fatal("invalid access cleanup disposition wrong", retained, err)
			}
			if err != nil {
				t.Fatal("invalid access cleanup disposition wrong", retained, err)
			}
			want := 0
			if failed {
				want = 1
				if !errors.Is(failure, owner.closeFailure) {
					t.Fatal("original cleanup cause lost")
				}
			}
			if owner.settlements != want {
				t.Fatal("invalid owner closed twice or recovery skipped")
			}
			if owner.closes != 1 {
				t.Fatal("invalid owner closed twice or recovery skipped")
			}
		})
	}
}

type accessFailureCycle struct{ child error }

func (*accessFailureCycle) Error() string   { return "cyclic access failure" }
func (e *accessFailureCycle) Unwrap() error { return e.child }

// Goal: Invalid error graphs and canceled input refuse before any original cleanup.
// Method: Add an original receipt before canceled, cyclic and oversized cases, then check zero settlements.
func TestAccessFailureValidationPrecedesAllCleanup(t *testing.T) {
	for _, kind := range []string{"canceled", "cycle", "oversized"} {
		t.Run(kind, func(t *testing.T) {
			owner := &recoveryAccessOwner{closeFailure: errors.New("original")}
			failure := RefuseAccess(errors.New("prepare"), owner)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			switch kind {
			case "cycle":
				cycle := &accessFailureCycle{}
				cycle.child = cycle
				failure = errors.Join(failure, cycle)
			case "oversized":
				chain := errors.New("leaf")
				for range 65537 {
					chain = &accessFailureCycle{child: chain}
				}
				failure = errors.Join(failure, chain)
			case "canceled":
				cancel()
			}
			_, err := SettleAccessFailure(ctx, failure)
			if err == nil {
				t.Fatal("invalid error/context reached original cleanup", err)
			}
			if owner.settlements != 0 {
				t.Fatal("invalid error/context reached original cleanup", err)
			}
			if kind == "cycle" {
				if !strings.Contains(err.Error(), "active-path cycle") {
					t.Fatal("shared graph/cycle classification wrong", err)
				}
			}
			if kind == "oversized" {
				if !strings.Contains(err.Error(), "finite inspection bound") {
					t.Fatal("overbound traversal did not retain root", err)
				}
			}
		})
	}
}

// Goal: Shared graph nodes settle every original owner exactly once.
// Method: Repeat shared wrapped receipts in a DAG, then contrast an ordinary owner-free refusal.
func TestAccessFailureSharedGraphSettlesEveryOriginalOnce(t *testing.T) {
	first := &recoveryAccessOwner{closeFailure: errors.New("first close")}
	second := &recoveryAccessOwner{closeFailure: errors.New("second close")}
	a := RefuseAccess(errors.New("first prepare"), first)
	b := RefuseAccess(errors.New("second prepare"), second)
	shared := fmt.Errorf("shared: %w", errors.Join(a, b))
	retained, err := SettleAccessFailure(t.Context(), errors.Join(shared, shared, a, b))
	if !retained {
		t.Fatal("shared graph lost/duplicated owner or became a cycle", retained, err)
	}
	if err != nil {
		t.Fatal("shared graph lost/duplicated owner or became a cycle", retained, err)
	}
	if first.settlements != 1 {
		t.Fatal("shared graph lost/duplicated owner or became a cycle", retained, err)
	}
	if second.settlements != 1 {
		t.Fatal("shared graph lost/duplicated owner or became a cycle", retained, err)
	}
	if first.closes != 1 {
		t.Fatal("shared graph lost/duplicated owner or became a cycle", retained, err)
	}
	if second.closes != 1 {
		t.Fatal("shared graph lost/duplicated owner or became a cycle", retained, err)
	}
	none, err := SettleAccessFailure(t.Context(), errors.New("not ready without an owner"))
	if none {
		t.Fatal("ordinary not-ready error invented pending cleanup", none, err)
	}
	if err != nil {
		t.Fatal("ordinary not-ready error invented pending cleanup", none, err)
	}
}

// Goal: Typed-nil error nodes refuse without being mistaken for plain nil entries.
// Method: Traverse private and foreign typed-nil nodes after an original receipt, then settle plain nil joins.
func TestAccessFailureTypedNilNodesRefuseBeforeCleanup(t *testing.T) {
	var private *refusedAccessError
	var foreign *accessFailureCycle // Unwrap reads this receiver; a missing guard panics.
	cases := []struct {
		name      string
		malformed error
	}{{"private receipt", private}, {"foreign unwrap", foreign}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			owner := &recoveryAccessOwner{closeFailure: errors.New("original cleanup failed")}
			original := RefuseAccess(errors.New("prepare refused"), owner)
			forest := errors.Join(original, nil, errors.Join(nil, tc.malformed))
			retained, err := SettleAccessFailure(t.Context(), forest)
			if !retained {
				t.Fatal("typed-nil node was not explicitly refused", retained, err)
			}
			if _, ok := errors.AsType[*accessFailureTraversalError](err); !ok {
				t.Fatal("typed-nil node was not explicitly refused", retained, err)
			}
			if !strings.Contains(err.Error(), "typed-nil error node") {
				t.Fatal("typed-nil node was not explicitly refused", retained, err)
			}
			if owner.closes != 1 {
				t.Fatal("invalid forest invoked original cleanup", owner.closes, owner.settlements)
			}
			if owner.settlements != 0 {
				t.Fatal("invalid forest invoked original cleanup", owner.closes, owner.settlements)
			}
		})
	}
	ordinary := &recoveryAccessOwner{closeFailure: errors.New("ordinary original cleanup")}
	retained, err := SettleAccessFailure(t.Context(), errors.Join(nil, RefuseAccess(errors.New("prepare"), ordinary), nil))
	if !retained {
		t.Fatal("plain nil join entries lost their normal meaning", retained, err)
	}
	if err != nil {
		t.Fatal("plain nil join entries lost their normal meaning", retained, err)
	}
	if ordinary.settlements != 1 {
		t.Fatal("plain nil join entries lost their normal meaning", retained, err)
	}
}
