// Design: docs/architecture/l2tp/bng-5-pppoe.md -- VPP control-map ownership
// Related: access_map.go -- acquired mapping leases; api.go -- bounded streams
package vpp

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/memclnt"

	pppoeapi "github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane/vpp/binapi/pppoe"
)

// A pair shared by two configured accesses is installed once and remains
// present until both independently idempotent leases have ended.
func TestVPPAccessMapLastLeaseRemovesOwnedPair(t *testing.T) {
	fixture := newAccessMapFixture()
	owners := newControlMapOwners()
	pair := controlPair{dataplane: 11, control: 42}
	first, err := owners.acquire(t.Context(), fixture.api(), pair)
	if err != nil {
		t.Fatal(err)
	}
	second, err := owners.acquire(t.Context(), fixture.api(), pair)
	if err != nil {
		t.Fatal(err)
	}
	if fixture.added != 1 {
		t.Fatalf("mapping adds=%d", fixture.added)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if fixture.removed != 0 {
		t.Fatal("shared mapping removed before final lease")
	}
	var joined sync.WaitGroup
	for range 8 {
		joined.Go(func() {
			if err := second.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	joined.Wait()
	if fixture.removed != 1 {
		t.Fatalf("mapping deletes=%d", fixture.removed)
	}
	if len(fixture.bindings) != 0 {
		t.Fatalf("owned map remains: %v", fixture.bindings)
	}
}

// A pre-existing exact reciprocal pair is useful but belongs to somebody
// else. Acquiring and ending a lease must never claim or delete that pair.
func TestVPPAccessMapBorrowsExistingPair(t *testing.T) {
	fixture := newAccessMapFixture()
	fixture.bindings[11] = 42
	lease, err := newControlMapOwners().acquire(t.Context(), fixture.api(), controlPair{11, 42})
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if fixture.added != 0 {
		t.Fatal("existing map was overwritten")
	}
	if fixture.removed != 0 {
		t.Fatal("foreign map was deleted")
	}
	if fixture.bindings[11] != 42 {
		t.Fatal("foreign map changed")
	}
}

// Either side can already belong to another pair. Targeted readback of both
// indices must refuse the conflict before an add or delete touches VPP.
func TestVPPAccessMapRefusesConflictingPair(t *testing.T) {
	for _, existing := range []controlPair{{11, 99}, {88, 42}} {
		t.Run(fmt.Sprintf("%d-%d", existing.dataplane, existing.control), func(t *testing.T) {
			fixture := newAccessMapFixture()
			fixture.bindings[existing.dataplane] = existing.control
			lease, err := newControlMapOwners().acquire(t.Context(), fixture.api(), controlPair{11, 42})
			if err == nil {
				t.Fatal("conflicting map admitted")
			}
			if lease != nil {
				t.Fatal("failed mapping published a lease")
			}
			if fixture.added != 0 {
				t.Fatal("conflict reached add")
			}
			if fixture.removed != 0 {
				t.Fatal("conflict reached delete")
			}
			if fixture.bindings[existing.dataplane] != existing.control {
				t.Fatal("foreign map changed")
			}
		})
	}
}

// A successful mutation reply proves ownership even if closing that stream
// fails. The failure must refuse publication and roll back the confirmed map.
func TestVPPAccessMapRollsBackConfirmedAddAfterCloseError(t *testing.T) {
	want := errors.New("stream close failed after confirmed add")
	fixture := newAccessMapFixture()
	fixture.addCloseError = want
	lease, err := newControlMapOwners().acquire(t.Context(), fixture.api(), controlPair{11, 42})
	if !errors.Is(err, want) {
		t.Fatalf("close error lost: %v", err)
	}
	if lease != nil {
		t.Fatal("failed add published a lease")
	}
	if fixture.added != 1 {
		t.Fatalf("confirmed adds=%d", fixture.added)
	}
	if fixture.removed != 1 {
		t.Fatalf("rollback deletes=%d", fixture.removed)
	}
}

// If another writer replaced our pair, ending our lease must report the
// changed ownership and leave the replacement untouched.
func TestVPPAccessMapCloseProtectsReplacement(t *testing.T) {
	fixture := newAccessMapFixture()
	lease, err := newControlMapOwners().acquire(t.Context(), fixture.api(), controlPair{11, 42})
	if err != nil {
		t.Fatal(err)
	}
	fixture.bindings[11] = 99
	if err := lease.Close(); err == nil {
		t.Fatal("replacement mapping was not reported")
	}
	if fixture.removed != 0 {
		t.Fatal("replacement mapping was deleted")
	}
	if fixture.bindings[11] != 99 {
		t.Fatal("replacement mapping changed")
	}
}

// An old API generation must never select a replacement stream while
// releasing its owned pair. The retained API refuses instead of reconnecting.
func TestVPPAccessMapCloseRefusesLostGeneration(t *testing.T) {
	fixture := newAccessMapFixture()
	lease, err := newControlMapOwners().acquire(t.Context(), fixture.api(), controlPair{11, 42})
	if err != nil {
		t.Fatal(err)
	}
	want := errors.New("connection generation ended")
	fixture.openError = want
	if err := lease.Close(); !errors.Is(err, want) {
		t.Fatalf("lost generation: %v", err)
	}
	if fixture.removed != 0 {
		t.Fatal("lost owner mutated replacement connection")
	}
}

type accessMapFixture struct {
	bindings                 map[uint32]uint32
	added, removed           int
	openError, addCloseError error
}

func newAccessMapFixture() *accessMapFixture {
	return &accessMapFixture{bindings: make(map[uint32]uint32)}
}
func (f *accessMapFixture) api() *binaryAPI {
	return &binaryAPI{open: func(ctx context.Context) (api.Stream, error) {
		if f.openError != nil {
			return nil, f.openError
		}
		return &accessMapStream{ctx: ctx, fixture: f}, nil
	}}
}

type accessMapStream struct {
	ctx        context.Context
	fixture    *accessMapFixture
	replies    []api.Message
	closeError error
}

func (s *accessMapStream) Context() context.Context { return s.ctx }
func (s *accessMapStream) SendMsg(message api.Message) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	switch request := message.(type) {
	case *pppoeapi.PppoeCpBindingDump:
		index := uint32(request.SwIfIndex)
		details := &pppoeapi.PppoeCpBindingDetails{Retval: int32(api.INVALID_INTERFACE),
			DpSwIfIndex: interface_types.InterfaceIndex(noInterface), CpSwIfIndex: interface_types.InterfaceIndex(noInterface)}
		for dataplane, control := range s.fixture.bindings {
			if index != dataplane {
				if index != control {
					continue
				}
			}
			details.Retval = 0
			details.DpSwIfIndex, details.CpSwIfIndex = interface_types.InterfaceIndex(dataplane), interface_types.InterfaceIndex(control)
		}
		s.replies = append(s.replies, details)
	case *memclnt.ControlPing:
		s.replies = append(s.replies, &memclnt.ControlPingReply{})
	case *pppoeapi.PppoeAddDelCp:
		dataplane, control := uint32(request.DpSwIfIndex), uint32(request.CpSwIfIndex)
		if request.IsAdd == 1 {
			s.fixture.bindings[dataplane] = control
			s.fixture.added++
			s.closeError = s.fixture.addCloseError
		} else {
			delete(s.fixture.bindings, dataplane)
			s.fixture.removed++
		}
		s.replies = append(s.replies, &pppoeapi.PppoeAddDelCpReply{})
	default:
		return fmt.Errorf("unexpected access-map request %T", message)
	}
	return nil
}
func (s *accessMapStream) RecvMsg() (api.Message, error) {
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	if len(s.replies) == 0 {
		return nil, errors.New("no scripted access-map reply")
	}
	reply := s.replies[0]
	s.replies = s.replies[1:]
	return reply, nil
}
func (s *accessMapStream) Close() error { return s.closeError }
