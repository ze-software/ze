// Design: docs/architecture/l2tp/bng-5-pppoe.md -- VPP creation and rollback
// VALIDATES: confirmed native creation precedes publication and rollback owns it.
package vpp

import (
	"context"
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane/vpp/binapi/feature"
	pppoeapi "github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane/vpp/binapi/pppoe"
	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/ethernet_types"
	"go.fd.io/govpp/binapi/fib"
	interfaces "go.fd.io/govpp/binapi/interface"
	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/memclnt"
)

type factoryHarness struct {
	requests []api.Message
	fail     string
	closes   int
	release  int
	stream   *ownerStream
	gates    map[string]bool
}
type factoryStream struct {
	ctx      context.Context
	owner    *factoryHarness
	request  api.Message
	received int
}

func (s *factoryStream) Context() context.Context { return s.ctx }
func (s *factoryStream) SendMsg(message api.Message) error {
	if s.request == nil {
		s.request = message
	}
	s.owner.requests = append(s.owner.requests, message)
	return nil
}
func (s *factoryStream) RecvMsg() (api.Message, error) {
	s.received++
	switch request := s.request.(type) {
	case *feature.FeatureIsEnabled:
		return &feature.FeatureIsEnabledReply{IsEnabled: s.owner.gates[request.ArcName]}, nil
	case *feature.FeatureEnableDisable:
		if s.owner.gates == nil {
			s.owner.gates = make(map[string]bool)
		}
		s.owner.gates[request.ArcName] = request.Enable
		return &feature.FeatureEnableDisableReply{}, nil
	case *interfaces.SwInterfaceGetTable:
		table := uint32(41)
		if request.IsIPv6 {
			table = 42
		}
		return &interfaces.SwInterfaceGetTableReply{VrfID: table}, nil
	case *fib.FibSourceDump:
		if s.received == 1 {
			return &fib.FibSourceDetails{Src: fib.FibSource{Name: routeSourceName, Priority: routeSourcePriority, ID: 17}}, nil
		}
		return &memclnt.ControlPingReply{}, nil
	case *pppoeapi.PppoeAddDelSession:
		if request.IsAdd {
			if s.owner.fail == "create-no-reply" {
				return nil, errors.New("create reply unavailable")
			}
		}
		return &pppoeapi.PppoeAddDelSessionReply{SwIfIndex: 73}, nil
	case *pppoeapi.PppoeSessionDump:
		if s.received > 1 {
			return &memclnt.ControlPingReply{}, nil
		}
		access := uint32(17)
		if s.owner.fail == "wrong-access" {
			access = 18
		}
		return &pppoeapi.PppoeSessionDetails{SwIfIndex: 73, EncapIfIndex: interface_types.InterfaceIndex(access), SessionID: 19, DecapVrfID: 41, ClientMac: ethernet_types.MacAddress{2, 0, 0, 0, 0, 1}, LocalMac: ethernet_types.MacAddress{2, 0, 0, 0, 0, 2}}, nil
	case *interfaces.SwInterfaceDump:
		if s.owner.fail == "readback" {
			return nil, errors.New("readback failed")
		}
		if s.received == 1 {
			return &interfaces.SwInterfaceDetails{SwIfIndex: 73, InterfaceName: "pppoe_session9"}, nil
		}
		return &memclnt.ControlPingReply{}, nil
	case *interfaces.SwInterfaceSetFlags:
		return &interfaces.SwInterfaceSetFlagsReply{}, nil
	case *interfaces.SwInterfaceSetTable:
		return &interfaces.SwInterfaceSetTableReply{}, nil
	case *interfaces.SwInterfaceSetMtu:
		return &interfaces.SwInterfaceSetMtuReply{}, nil
	default:
		return nil, errors.New("unexpected factory request")
	}
}
func (s *factoryStream) Close() error {
	if request, ok := s.request.(*pppoeapi.PppoeAddDelSession); ok {
		if request.IsAdd {
			if s.owner.fail == "create-close" {
				return errors.New("create stream close failed")
			}
		}
	}
	return nil
}
func (h *factoryHarness) ops() factoryOps {
	client := &binaryAPI{open: func(ctx context.Context) (api.Stream, error) { return &factoryStream{ctx: ctx, owner: h}, nil }}
	return factoryOps{
		access: func(context.Context, dataplane.Request) (preparedSessionAccess, error) {
			return preparedSessionAccess{api: client, dpIndex: 17, mtu: 1500, localMAC: [6]byte{2, 0, 0, 0, 0, 2}, release: func() { h.release++ }}, nil
		},
		socket: func(context.Context, dataplane.Request) (*controlSocket, error) {
			h.stream = &ownerStream{done: make(chan struct{})}
			return newControlSocket(h.stream), nil
		},
		counters: func(*binaryAPI) func(*api.InterfaceStats) error {
			return func(*api.InterfaceStats) error { return errors.New("test stats unavailable") }
		},
	}
}
func factoryRequest() dataplane.Request {
	return dataplane.Request{AccessName: "wan", Device: "host-eth0", ControlDevice: "cp0", Namespace: "dataplane", SessionID: 19, PeerMAC: [6]byte{2, 0, 0, 0, 0, 1}}
}

// Publication requires the actual created name/index, table assignment and MTU
// acknowledgement. Returning from Open is the server's permission to send PADS.
func TestVPPFactoryPublishesOnlyCreatedInterface(t *testing.T) {
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
	identity := opened.Dataplane().Identity()
	if identity.InterfaceName != "pppoe_session9" || identity.NativeIndex != 73 {
		t.Fatalf("identity %+v", identity)
	}
	if h.stream.closes != 0 || h.release != 1 {
		t.Fatalf("premature socket close %d / construction leases %d", h.stream.closes, h.release)
	}
	var tables, mtus int
	for _, message := range h.requests {
		switch request := message.(type) {
		case *interfaces.SwInterfaceSetTable:
			tables++
			if request.SwIfIndex != 73 {
				t.Fatalf("table targets access index %+v", request)
			}
		case *interfaces.SwInterfaceSetMtu:
			mtus++
			if request.SwIfIndex != 73 || request.Mtu[0] != 1492 {
				t.Fatalf("MTU %+v", request)
			}
		}
	}
	if tables != 2 || mtus != 1 {
		t.Fatalf("initial operations tables=%d MTU=%d", tables, mtus)
	}
}

// A successful mutation receipt remains owned even when closing its API stream
// fails. Later readback refusal has the same rollback obligations.
func TestVPPFactoryRollbackClosesSocketAndConfirmedSession(t *testing.T) {
	for _, failure := range []string{"create-close", "readback", "wrong-access"} {
		t.Run(failure, func(t *testing.T) {
			h := &factoryHarness{fail: failure}
			opened, err := openWithOps(context.Background(), factoryRequest(), h.ops())
			if opened != nil || err == nil {
				t.Fatalf("failed construction %v, %v", opened, err)
			}
			if h.stream.closes != 1 || h.release != 1 {
				t.Fatalf("cleanup socket=%d lease=%d", h.stream.closes, h.release)
			}
			deletes := 0
			for _, message := range h.requests {
				if request, ok := message.(*pppoeapi.PppoeAddDelSession); ok {
					if !request.IsAdd {
						deletes++
					}
				}
			}
			if deletes != 1 {
				t.Fatalf("confirmed session deletes %d", deletes)
			}
		})
	}
}

// Upstream MAC-only learning forbids simultaneous owners on different accesses.
// Rejecting the second attempt must leave the first owner and socket untouched.
func TestVPPFactoryRefusesMACReuseAcrossAccesses(t *testing.T) {
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
	other := factoryRequest()
	other.AccessName = "other"
	other.SessionID = 20
	if second, err := openWithOps(context.Background(), other, h.ops()); second != nil || err == nil {
		t.Fatalf("MAC alias %v, %v", second, err)
	}
	if h.stream.closes != 0 {
		t.Fatal("refusal closed admitted socket")
	}
}

// Without a create receipt, the factory cannot assume the request failed or
// delete an unproven foreign key. It retains the exact ambiguity and blocks key
// reuse until reconciliation resolves it; temporary socket ownership still ends.
func TestVPPFactoryUnknownCreateRetainsReservation(t *testing.T) {
	h := &factoryHarness{fail: "create-no-reply"}
	request := factoryRequest()
	request.PeerMAC[5] = 99
	opened, err := openWithOps(context.Background(), request, h.ops())
	if opened != nil || err == nil {
		t.Fatalf("unknown mutation %v, %v", opened, err)
	}
	if h.stream.closes != 1 {
		t.Fatalf("unpublished socket closes %d", h.stream.closes)
	}
	macReservations.Lock()
	reservation := macReservations.held[request.PeerMAC]
	macReservations.Unlock()
	if reservation == nil {
		t.Fatal("ambiguous key was released")
	}
	t.Cleanup(func() {
		macReservations.Lock()
		defer macReservations.Unlock()
		if macReservations.held[request.PeerMAC] == reservation {
			delete(macReservations.held, request.PeerMAC)
		}
	})
	if reservation.pending == nil || reservation.pending.request.SessionID != request.SessionID {
		t.Fatal("ambiguity lost its exact mutation receipt")
	}
	before := len(h.requests)
	if retry, err := openWithOps(context.Background(), request, h.ops()); retry != nil || err == nil {
		t.Fatalf("ambiguous key reuse %v, %v", retry, err)
	}
	if len(h.requests) != before {
		t.Fatal("refused key reuse reached VPP")
	}
	for _, message := range h.requests {
		if mutation, ok := message.(*pppoeapi.PppoeAddDelSession); ok {
			if !mutation.IsAdd {
				t.Fatal("unconfirmed key was deleted")
			}
		}
	}
}
