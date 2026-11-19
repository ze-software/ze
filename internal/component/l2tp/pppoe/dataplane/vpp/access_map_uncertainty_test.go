// Design: docs/architecture/l2tp/bng-5-pppoe.md -- uncertain control-map ownership
// Related: access_map_recovery_test.go -- confirmed cleanup and lost-delete proof
package vpp

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"go.fd.io/govpp/api"

	pppoeapi "github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane/vpp/binapi/pppoe"
)

// A lost add receipt cannot grant ownership just because an exact native map
// later appears. Method: lose only the successful add reply, then reacquire.
func TestVPPAccessMapUnknownAddCannotBorrowVisiblePair(t *testing.T) {
	fixture := newAccessMapFixture()
	failure := errors.New("control-map add reply was lost")
	connection := unknownAddMapAPI(fixture, failure, true)
	owners := newControlMapOwners()
	lease, err := owners.acquire(t.Context(), connection, controlPair{11, 42})
	if !errors.Is(err, failure) {
		t.Fatalf("unknown add = %v", err)
	}
	if lease != nil {
		t.Fatal("unknown add published a lease")
	}
	lease, err = owners.acquire(t.Context(), fixture.api(), controlPair{11, 42})
	if !errors.Is(err, failure) {
		t.Fatalf("unknown map was adopted: %v", err)
	}
	if lease != nil {
		t.Fatal("visible unknown map became a borrowed lease")
	}
	if fixture.added != 1 {
		t.Fatalf("unknown add repeated %d native adds", fixture.added)
	}
	if fixture.removed != 0 {
		t.Fatal("unknown add gained deletion authority")
	}
	if fixture.bindings[11] != 42 {
		t.Fatal("unknown native map changed")
	}
}

// Each index of an unconfirmed request remains reserved even when native
// readback currently sees no map. A late request must not target a replacement.
func TestVPPAccessMapUnknownAddReservesBothIndices(t *testing.T) {
	for _, replacement := range []controlPair{{11, 99}, {88, 42}, {42, 99}, {88, 11}} {
		t.Run(fmt.Sprintf("%d-%d", replacement.dataplane, replacement.control), func(t *testing.T) {
			fixture := newAccessMapFixture()
			failure := errors.New("add transport ended without a receipt")
			owners := newControlMapOwners()
			lease, err := owners.acquire(t.Context(), unknownAddMapAPI(fixture, failure, false), controlPair{11, 42})
			if !errors.Is(err, failure) {
				t.Fatalf("unknown add = %v", err)
			}
			if lease != nil {
				t.Fatal("unconfirmed add published a lease")
			}
			lease, err = owners.acquire(t.Context(), fixture.api(), replacement)
			if !errors.Is(err, failure) {
				t.Fatalf("overlapping request forgot the original error: %v", err)
			}
			if lease != nil {
				t.Fatal("overlapping request published a lease")
			}
			if fixture.added != 0 {
				t.Fatal("overlapping request reached native creation")
			}
		})
	}
}

// A disjoint pair stays usable despite another pair's uncertain mutation.
// Method: retain one lost add, then acquire and release distinct native indices.
func TestVPPAccessMapUnknownAddPreservesDisjointPair(t *testing.T) {
	fixture := newAccessMapFixture()
	failure := errors.New("add receipt unavailable")
	owners := newControlMapOwners()
	if _, err := owners.acquire(t.Context(), unknownAddMapAPI(fixture, failure, false), controlPair{11, 42}); !errors.Is(err, failure) {
		t.Fatalf("unknown add = %v", err)
	}
	lease, err := owners.acquire(t.Context(), fixture.api(), controlPair{88, 99})
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if fixture.added != 1 {
		t.Fatalf("disjoint adds = %d", fixture.added)
	}
	if fixture.removed != 1 {
		t.Fatalf("disjoint removals = %d", fixture.removed)
	}
	if len(owners.entries) != 1 {
		t.Fatalf("uncertain owner lost or duplicated: %d", len(owners.entries))
	}
}

// An active lease reserves both indices even if another writer removed the
// native map. A replacement must wait until its owner confirms cleanup.
func TestVPPAccessMapLeaseReservesMissingPair(t *testing.T) {
	fixture := newAccessMapFixture()
	owners := newControlMapOwners()
	lease, err := owners.acquire(t.Context(), fixture.api(), controlPair{11, 42})
	if err != nil {
		t.Fatal(err)
	}
	delete(fixture.bindings, 11)
	if replacement, err := owners.acquire(t.Context(), fixture.api(), controlPair{11, 99}); err == nil {
		if replacement != nil {
			if closeErr := replacement.Close(); closeErr != nil {
				t.Error(closeErr)
			}
		}
		t.Fatal("missing native map released an admitted owner's indices")
	}
	if fixture.added != 1 {
		t.Fatal("overlapping replacement reached native creation")
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if len(owners.entries) != 0 {
		t.Fatal("confirmed absence retained the finished lease")
	}
}

// Retained uncertainty is bounded without refusing an already admitted owner.
// Method: fill the remaining slots, reject a new pair, and end a shared lease.
func TestVPPAccessMapOwnershipBoundPreservesExistingLeases(t *testing.T) {
	fixture := newAccessMapFixture()
	owners := newControlMapOwners()
	first, err := owners.acquire(t.Context(), fixture.api(), controlPair{11, 42})
	if err != nil {
		t.Fatal(err)
	}
	failure := errors.New("retained unconfirmed map")
	for index := range controlMapMax - 1 {
		key := controlMapKey{pair: controlPair{uint32(100 + index), uint32(10000 + index)}}
		owners.entries[key] = &controlMapUnconfirmed{failure: failure}
	}
	if lease, err := owners.acquire(t.Context(), fixture.api(), controlPair{88, 99}); err == nil {
		if lease != nil {
			if closeErr := lease.Close(); closeErr != nil {
				t.Error(closeErr)
			}
		}
		t.Fatal("full ownership table admitted a new pair")
	}
	if fixture.added != 1 {
		t.Fatal("full ownership table mutated native mappings")
	}
	second, err := owners.acquire(t.Context(), fixture.api(), controlPair{11, 42})
	if err != nil {
		t.Fatalf("ownership bound refused an existing lease: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if fixture.removed != 0 {
		t.Fatal("existing shared map was removed early")
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	if fixture.removed != 1 {
		t.Fatal("existing owner could not finish cleanup at the bound")
	}
}

func unknownAddMapAPI(fixture *accessMapFixture, failure error, mutate bool) *binaryAPI {
	base := fixture.api()
	return &binaryAPI{open: func(ctx context.Context) (api.Stream, error) {
		stream, err := base.open(ctx)
		if err != nil {
			return nil, err
		}
		return &unknownAddMapStream{Stream: stream, failure: failure, mutate: mutate}, nil
	}}
}

type unknownAddMapStream struct {
	api.Stream
	failure error
	mutate  bool
	adding  bool
}

func (s *unknownAddMapStream) SendMsg(message api.Message) error {
	if request, ok := message.(*pppoeapi.PppoeAddDelCp); ok {
		if request.IsAdd == 1 {
			s.adding = true
			if !s.mutate {
				return s.failure
			}
		}
	}
	return s.Stream.SendMsg(message)
}

func (s *unknownAddMapStream) RecvMsg() (api.Message, error) {
	if s.adding {
		return nil, s.failure
	}
	return s.Stream.RecvMsg()
}
