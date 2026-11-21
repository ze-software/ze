// Design: docs/architecture/l2tp/bng-5-pppoe.md -- quarantine before PADS
package vpp

import (
	"context"
	"errors"
	"testing"

	"go.fd.io/govpp/api"

	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane/vpp/binapi/feature"

	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
	pppoeapi "github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane/vpp/binapi/pppoe"
)

// The actual factory must confirm all four receive gates before returning
// the ownership envelope which permits the server to send PADS.
func TestVPPFactoryQuarantinesBothFamiliesBeforePublication(t *testing.T) {
	h := &factoryHarness{}
	opened, err := openWithOps(context.Background(), factoryRequest(), h.ops())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := opened.Dataplane().Close(); err != nil {
			t.Error(err)
		}
	}()
	confirmed := make(map[string]bool)
	for _, message := range h.requests {
		if request, ok := message.(*feature.FeatureIsEnabled); ok {
			if request.SwIfIndex != 73 {
				t.Fatal("quarantine read targeted another index")
			}
			confirmed[request.ArcName] = true
		}
	}
	for _, arc := range []string{"ip4-unicast", "ip4-multicast", "ip6-unicast", "ip6-multicast"} {
		if !confirmed[arc] || !h.gates[arc] {
			t.Fatalf("published without confirmed closed gate %s", arc)
		}
	}
}

type quarantineReadFailure struct {
	api.Stream
	query bool
}

func (s *quarantineReadFailure) SendMsg(message api.Message) error {
	_, s.query = message.(*feature.FeatureIsEnabled)
	return s.Stream.SendMsg(message)
}

func (s *quarantineReadFailure) RecvMsg() (api.Message, error) {
	if s.query {
		return nil, errors.New("quarantine readback unavailable")
	}
	return s.Stream.RecvMsg()
}

// Failed construction cannot recycle the confirmed native session when its
// receive gates remain unknown, even though no route or local ready flag exists.
func TestVPPFactoryUnknownQuarantineRetainsNativeIndex(t *testing.T) {
	h := &factoryHarness{}
	ops := h.ops()
	lookup := ops.access
	ops.access = func(ctx context.Context, request dataplane.Request) (preparedSessionAccess, error) {
		access, err := lookup(ctx, request)
		if err != nil {
			return access, err
		}
		open := access.api.open
		access.api.open = func(ctx context.Context) (api.Stream, error) {
			stream, err := open(ctx)
			if err != nil {
				return nil, err
			}
			return &quarantineReadFailure{Stream: stream}, nil
		}
		return access, nil
	}
	request := factoryRequest()
	request.PeerMAC[5] = 98
	opened, err := openWithOps(context.Background(), request, ops)
	if err == nil || opened != nil {
		t.Fatalf("unknown quarantine published %v, %v", opened, err)
	}
	if h.stream.closes != 1 {
		t.Fatalf("socket closes %d", h.stream.closes)
	}
	for _, message := range h.requests {
		if mutation, ok := message.(*pppoeapi.PppoeAddDelSession); ok && !mutation.IsAdd {
			t.Fatal("unknown gates allowed native index recycling")
		}
	}
	macReservations.Lock()
	reservation := macReservations.held[request.PeerMAC]
	macReservations.Unlock()
	if reservation == nil {
		t.Fatal("unknown quarantine lost its MAC owner")
	}
	t.Cleanup(func() {
		macReservations.Lock()
		defer macReservations.Unlock()
		if macReservations.held[request.PeerMAC] == reservation {
			delete(macReservations.held, request.PeerMAC)
		}
	})
	if reservation.pending == nil || reservation.pending.session == nil {
		t.Fatal("unknown quarantine lost its exact session owner")
	}
}
