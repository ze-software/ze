// Design: docs/architecture/l2tp/bng-5-pppoe.md -- control-map cleanup provenance
// Related: access_map.go -- ownership survives an unsuccessful final release
package vpp

import (
	"context"
	"errors"
	"testing"

	"go.fd.io/govpp/api"

	pppoeapi "github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane/vpp/binapi/pppoe"
)

// A failed delete leaves our confirmed map in VPP. A later access must retain
// that provenance and remove the map when its final lease ends.
func TestVPPAccessMapFailedDeleteRetainsOwnership(t *testing.T) {
	fixture := newAccessMapFixture()
	failure := errors.New("native control-map delete failed")
	connection := recoverableAccessMapAPI(fixture, &failure)
	owners := newControlMapOwners()
	pair := controlPair{11, 42}
	first, err := owners.acquire(t.Context(), connection, pair)
	if err != nil {
		t.Fatal(err)
	}
	want := failure
	if err := first.Close(); !errors.Is(err, want) {
		t.Fatalf("delete failure lost: %v", err)
	}
	if fixture.bindings[11] != 42 {
		t.Fatal("failed native delete changed the map")
	}
	failure = nil
	second, err := owners.acquire(t.Context(), connection, pair)
	if err != nil {
		t.Fatal(err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	if fixture.removed != 1 || len(fixture.bindings) != 0 {
		t.Fatalf("confirmed ownership was lost: deletes=%d, map=%v", fixture.removed, fixture.bindings)
	}
	if err := first.Close(); !errors.Is(err, want) {
		t.Fatalf("first lease forgot its cleanup outcome: %v", err)
	}
}

// A successful add reply proves ownership even when its stream close fails.
// If rollback also fails, reacquiring must not turn that map into a foreign one.
func TestVPPAccessMapFailedRollbackRetainsOwnership(t *testing.T) {
	fixture := newAccessMapFixture()
	addFailure := errors.New("confirmed add stream close failed")
	deleteFailure := errors.New("rollback delete failed")
	fixture.addCloseError = addFailure
	connection := recoverableAccessMapAPI(fixture, &deleteFailure)
	owners := newControlMapOwners()
	lease, err := owners.acquire(t.Context(), connection, controlPair{11, 42})
	if !errors.Is(err, addFailure) || !errors.Is(err, deleteFailure) {
		t.Fatalf("rollback failures lost: %v", err)
	}
	if lease != nil {
		t.Fatal("failed acquisition published a lease")
	}
	if fixture.bindings[11] != 42 {
		t.Fatal("failed rollback did not leave the confirmed map")
	}
	fixture.addCloseError, deleteFailure = nil, nil
	lease, err = owners.acquire(t.Context(), connection, controlPair{11, 42})
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if fixture.added != 1 || fixture.removed != 1 || len(fixture.bindings) != 0 {
		t.Fatalf("rollback provenance lost: adds=%d, deletes=%d, map=%v", fixture.added, fixture.removed, fixture.bindings)
	}
}

// A lost delete reply is not proof of removal. Query both native directions
// before releasing provenance; an absent pair settles cleanup despite the
// original stream error remaining visible to its caller.
func TestVPPAccessMapLostDeleteReplyReadsBackAbsence(t *testing.T) {
	fixture := newAccessMapFixture()
	base := fixture.api()
	lostReply := errors.New("delete reply was lost")
	readbacks := 0
	connection := &binaryAPI{open: func(ctx context.Context) (api.Stream, error) {
		stream, err := base.open(ctx)
		if err != nil {
			return nil, err
		}
		return &lostDeleteReplyStream{Stream: stream, fixture: fixture, readbacks: &readbacks, failure: lostReply}, nil
	}}
	owners := newControlMapOwners()
	lease, err := owners.acquire(t.Context(), connection, controlPair{11, 42})
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.Close(); !errors.Is(err, lostReply) {
		t.Fatalf("lost delete reply hidden: %v", err)
	}
	if readbacks != 2 {
		t.Fatalf("unconfirmed deletion did not query both native indices: readbacks=%d", readbacks)
	}
	if len(owners.entries) != 0 || len(fixture.bindings) != 0 || fixture.removed != 1 {
		t.Fatalf("confirmed absence retained cleanup state: owners=%d, map=%v, deletes=%d", len(owners.entries), fixture.bindings, fixture.removed)
	}
}

type lostDeleteReplyStream struct {
	api.Stream
	fixture   *accessMapFixture
	readbacks *int
	failure   error
	deleting  bool
}

func (s *lostDeleteReplyStream) SendMsg(message api.Message) error {
	if _, ok := message.(*pppoeapi.PppoeCpBindingDump); ok && s.fixture.removed != 0 {
		*s.readbacks++
	}
	if request, ok := message.(*pppoeapi.PppoeAddDelCp); ok && request.IsAdd == 0 {
		s.deleting = true
	}
	return s.Stream.SendMsg(message)
}
func (s *lostDeleteReplyStream) RecvMsg() (api.Message, error) {
	if s.deleting {
		return nil, s.failure
	}
	return s.Stream.RecvMsg()
}

func recoverableAccessMapAPI(fixture *accessMapFixture, deleteFailure *error) *binaryAPI { //nolint:gocritic // ptrToRefParam: the caller clears its failure after construction to let the delete recover.
	base := fixture.api()
	return &binaryAPI{open: func(ctx context.Context) (api.Stream, error) {
		stream, err := base.open(ctx)
		if err != nil {
			return nil, err
		}
		return &recoverableAccessMapStream{Stream: stream, deleteFailure: deleteFailure}, nil
	}}
}

type recoverableAccessMapStream struct {
	api.Stream
	deleteFailure *error
}

func (s *recoverableAccessMapStream) SendMsg(message api.Message) error {
	if request, ok := message.(*pppoeapi.PppoeAddDelCp); ok && request.IsAdd == 0 && *s.deleteFailure != nil {
		return *s.deleteFailure
	}
	return s.Stream.SendMsg(message)
}
