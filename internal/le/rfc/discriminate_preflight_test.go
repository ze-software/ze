package rfc

import (
	"errors"
	"testing"
)

// A discrimination observation of a .ci carrier refuses before it builds the
// break when the host cannot run the .ci, and an escape, which observes
// nothing, never asks. The method calls observeFor with no source reader, so
// any step past the preflight would fail for another reason, and counts the
// calls to a host check that always refuses.
func TestObserveForChecksTheHostOfAFunctionalCarrier(t *testing.T) {
	missing := errors.New("this host is missing loopback addresses")
	calls := 0
	host := func() error {
		calls++
		return missing
	}
	carrier := Carrier{Kind: kindFunctional}

	record := DiscriminationRecord{Route: RouteRevert}
	_, err := observeFor(t.TempDir(), nil, nil, carrier, Tag{}, &record, recordRequest{}, host)
	if !errors.Is(err, missing) {
		t.Fatalf("a .ci observation started on a host without its addresses: %v", err)
	}
	if calls != 1 {
		t.Fatalf("the host was checked %d times for one observation", calls)
	}

	calls = 0
	escape := DiscriminationRecord{Route: RouteNoBreak}
	if _, err := observeFor(t.TempDir(), nil, nil, carrier, Tag{}, &escape, recordRequest{}, host); err != nil {
		t.Fatalf("an escape, which runs nothing, was refused: %v", err)
	}
	if calls != 0 {
		t.Fatalf("an escape checked the host %d times", calls)
	}

	// A unit carrier runs `go test`, which binds no loopback address. With no
	// source reader the step past the preflight fails or panics, and only the
	// call count is the answer.
	calls = 0
	observeUnit := func() {
		defer func() { _ = recover() }()
		unit := DiscriminationRecord{Route: RouteRevert}
		_, _ = observeFor(t.TempDir(), nil, nil, Carrier{Kind: kindUnit}, Tag{}, &unit, recordRequest{}, host)
	}
	observeUnit()
	if calls != 0 {
		t.Fatalf("a unit observation checked the host %d times", calls)
	}
}
