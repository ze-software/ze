// Design: docs/architecture/l2tp/bng-5-pppoe.md -- retained forwarding ownership
// Method: drive the actual session owner against scripted native replies and
// observe API mutations, including the recycled interface's remaining state.

package vpp

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane/vpp/binapi/feature"
	"go.fd.io/govpp/api"
	interfaces "go.fd.io/govpp/binapi/interface"
	"go.fd.io/govpp/binapi/ip"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	pppoeapi "github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane/vpp/binapi/pppoe"
	"github.com/ze-software/ze/internal/core/rtproto"
)

type ownershipWire struct {
	addReadError             error
	addCloseError            error
	deleteRouteRetval        int32
	enableReadError          [2]error
	gateReadErrorAfterEnable [2]error
	gates                    [2][2]bool
	enableRetval             [2]int32
	disableRetval            [2]int32
	references               [2]int
	enables                  [2]int
	disables                 [2]int
	paths                    int
	pathDeletes              int
	sessionDeletes           int
	unsafeRecycle            bool
	overdraw                 bool
	retained                 []error
}

func ownershipSession(t *testing.T, wire *ownershipWire) *session {
	t.Helper()
	wire.gates = [2][2]bool{{true, true}, {true, true}}
	endpoint := &ownerStream{done: make(chan struct{})}
	owner := &session{
		identity: ppp.SessionIdentity{InterfaceName: "pppoe_session9", NativeIndex: 73},
		api: &binaryAPI{open: func(ctx context.Context) (api.Stream, error) {
			return &ownershipReplyStream{ctx: ctx, wire: wire}, nil
		}},
		socket: newControlSocket(endpoint), tables: [2]uint32{41, 42}, source: 17,
		sid: 19, mac: [6]byte{2, 0, 0, 0, 0, 1},
		routes: make(map[sessionRoute]struct{}), addresses: make(map[netip.Prefix]sessionRoute),
		release: func() {}, retain: func(err error) { wire.retained = append(wire.retained, err) },
	}
	// Only a synthetic endpoint is allocated. A failed Close deliberately
	// retains simulated forwarding state; test cleanup must not erase it.
	t.Cleanup(func() {
		if err := owner.socket.Close(); err != nil {
			t.Error(err)
		}
	})
	return owner
}

type ownershipReplyStream struct {
	ctx     context.Context
	wire    *ownershipWire
	request api.Message
}

func (s *ownershipReplyStream) Context() context.Context { return s.ctx }
func (s *ownershipReplyStream) SendMsg(message api.Message) error {
	s.request = message
	switch request := message.(type) {
	case *feature.FeatureEnableDisable:
		family, arc, err := ownershipGate(request.ArcName, request.FeatureName)
		if err != nil {
			return err
		}
		if request.SwIfIndex != 73 {
			return errors.New("feature mutation targeted another interface")
		}
		if arc == 0 {
			if request.Enable {
				s.wire.disables[family]++
			} else {
				s.wire.enables[family]++
			}
		}
		retval := s.wire.enableRetval[family]
		if request.Enable {
			retval = s.wire.disableRetval[family]
		}
		if retval == 0 {
			s.wire.gates[family][arc] = request.Enable
		}
		s.wire.references[family] = 0
		if !s.wire.gates[family][0] || !s.wire.gates[family][1] {
			s.wire.references[family] = 1
		}
	case *ip.IPRouteAddDelV2:
		if request.IsAdd {
			s.wire.paths++
		} else {
			s.wire.pathDeletes++
			if s.wire.deleteRouteRetval == 0 {
				s.wire.paths--
			}
		}
	case *ip.SwInterfaceIP4EnableDisable:
		s.programFamily(0, request.Enable)
	case *ip.SwInterfaceIP6EnableDisable:
		s.programFamily(1, request.Enable)
	case *pppoeapi.PppoeAddDelSession:
		if !request.IsAdd {
			s.wire.sessionDeletes++
			if s.wire.paths != 0 {
				s.wire.unsafeRecycle = true
			}
			if s.wire.references != [2]int{} {
				s.wire.unsafeRecycle = true
			}
		}
	}
	return nil
}

func (s *ownershipReplyStream) programFamily(family int, enable bool) {
	if enable {
		s.wire.enables[family]++
		if s.wire.enableRetval[family] == 0 {
			s.wire.references[family]++
		}
		return
	}
	s.wire.disables[family]++
	if s.wire.references[family] == 0 {
		s.wire.overdraw = true
	}
	if s.wire.disableRetval[family] == 0 {
		s.wire.references[family]--
	}
}

func (s *ownershipReplyStream) RecvMsg() (api.Message, error) {
	switch request := s.request.(type) {
	case *feature.FeatureIsEnabled:
		family, arc, err := ownershipGate(request.ArcName, request.FeatureName)
		if err != nil {
			return nil, err
		}
		if request.SwIfIndex != 73 {
			return nil, errors.New("feature query targeted another interface")
		}
		if s.wire.enables[family] > 0 && s.wire.gateReadErrorAfterEnable[family] != nil {
			return nil, s.wire.gateReadErrorAfterEnable[family]
		}
		return &feature.FeatureIsEnabledReply{IsEnabled: s.wire.gates[family][arc]}, nil
	case *feature.FeatureEnableDisable:
		family, _, err := ownershipGate(request.ArcName, request.FeatureName)
		if err != nil {
			return nil, err
		}
		if !request.Enable {
			if s.wire.enableReadError[family] != nil {
				return nil, s.wire.enableReadError[family]
			}
			return &feature.FeatureEnableDisableReply{Retval: s.wire.enableRetval[family]}, nil
		}
		return &feature.FeatureEnableDisableReply{Retval: s.wire.disableRetval[family]}, nil
	case *ip.IPRouteAddDelV2:
		if request.IsAdd {
			if s.wire.addReadError != nil {
				return nil, s.wire.addReadError
			}
			return &ip.IPRouteAddDelV2Reply{}, nil
		}
		return &ip.IPRouteAddDelV2Reply{Retval: s.wire.deleteRouteRetval}, nil
	case *ip.SwInterfaceIP4EnableDisable:
		if request.Enable {
			if s.wire.enableReadError[0] != nil {
				return nil, s.wire.enableReadError[0]
			}
			return &ip.SwInterfaceIP4EnableDisableReply{Retval: s.wire.enableRetval[0]}, nil
		}
		return &ip.SwInterfaceIP4EnableDisableReply{Retval: s.wire.disableRetval[0]}, nil
	case *ip.SwInterfaceIP6EnableDisable:
		if request.Enable {
			if s.wire.enableReadError[1] != nil {
				return nil, s.wire.enableReadError[1]
			}
			return &ip.SwInterfaceIP6EnableDisableReply{Retval: s.wire.enableRetval[1]}, nil
		}
		return &ip.SwInterfaceIP6EnableDisableReply{Retval: s.wire.disableRetval[1]}, nil
	case *pppoeapi.PppoeAddDelSession:
		return &pppoeapi.PppoeAddDelSessionReply{}, nil
	case *interfaces.SwInterfaceSetFlags:
		return &interfaces.SwInterfaceSetFlagsReply{}, nil
	}
	return nil, errors.New("unexpected ownership test request")
}

func (s *ownershipReplyStream) Close() error {
	if request, ok := s.request.(*ip.IPRouteAddDelV2); ok {
		if request.IsAdd {
			return s.wire.addCloseError
		}
	}
	return nil
}

// A failed path withdrawal must leave its session index unavailable for reuse.
func TestVPPCloseRouteFailureKeepsNativeIndex(t *testing.T) {
	wire := &ownershipWire{deleteRouteRetval: -1}
	owner := openedOwnershipSession(t, wire)
	if err := owner.AddRoute(netip.MustParsePrefix("198.51.100.0/24"), netip.Addr{}, 0, rtproto.Iface); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err == nil {
		t.Fatal("route cleanup failure was hidden")
	}
	if wire.pathDeletes != 2 {
		t.Fatalf("peer and framed route withdrawal attempts = %d", wire.pathDeletes)
	}
	if wire.sessionDeletes != 0 {
		t.Fatal("session index recycled while an owned route remained")
	}
	if wire.paths != 2 {
		t.Fatalf("remaining native paths = %d", wire.paths)
	}
	if len(wire.retained) == 0 {
		t.Fatal("failed forwarding owner was not retained")
	}
}

// A confirmed native add remains owned when closing its stream fails.
func TestVPPRouteConfirmedCloseErrorKeepsOwnership(t *testing.T) {
	want := errors.New("stream close after route add")
	wire := &ownershipWire{addCloseError: want}
	owner := openedOwnershipSession(t, wire)
	if err := owner.AddRoute(netip.MustParsePrefix("198.51.100.0/24"), netip.Addr{}, 0, rtproto.Iface); !errors.Is(err, want) {
		t.Fatalf("route error = %v", err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if wire.pathDeletes != 2 {
		t.Fatal("confirmed installed route escaped teardown ownership")
	}
	if wire.paths != 0 {
		t.Fatal("confirmed route remained after successful teardown")
	}
	if wire.unsafeRecycle {
		t.Fatal("session recycled before its confirmed route was removed")
	}
}

// A lost reply leaves an uncertain path, and failed cleanup cannot release it.
func TestVPPRouteUnknownAddKeepsNativeIndex(t *testing.T) {
	want := errors.New("route reply lost")
	wire := &ownershipWire{addReadError: want, deleteRouteRetval: -1}
	owner := openedOwnershipSession(t, wire)
	if err := owner.AddRoute(netip.MustParsePrefix("198.51.100.0/24"), netip.Addr{}, 0, rtproto.Iface); !errors.Is(err, want) {
		t.Fatalf("route error = %v", err)
	}
	if err := owner.Close(); err == nil {
		t.Fatal("uncertain route cleanup was reported complete")
	}
	if wire.pathDeletes != 2 {
		t.Fatalf("peer and framed route withdrawal attempts = %d", wire.pathDeletes)
	}
	if wire.sessionDeletes != 0 {
		t.Fatal("uncertain route allowed native index reuse")
	}
	if len(wire.retained) == 0 {
		t.Fatal("uncertain route owner was discarded")
	}
}

// IPv4-only negotiation must never enable the independently declined IPv6 NCP.
func TestVPPConnectEnablesOnlyNegotiatedAddressFamily(t *testing.T) {
	wire := &ownershipWire{}
	owner := ownershipSession(t, wire)
	if err := owner.AddAddressP2P(netip.MustParsePrefix("192.0.2.1/32"), netip.MustParsePrefix("192.0.2.7/32")); err != nil {
		t.Fatal(err)
	}
	if err := owner.Connect(); err != nil {
		t.Fatal(err)
	}
	if wire.enables != [2]int{1, 0} {
		t.Fatalf("family enables = %v, want IPv4 only", wire.enables)
	}
}

// IPv6-only startup connects before its service installs negotiated link-local
// addresses. That late address pair enables only its own family.
func TestVPPConnectedIPv6PairEnablesOnlyIPv6(t *testing.T) {
	wire := &ownershipWire{}
	owner := ownershipSession(t, wire)
	if err := owner.Connect(); err != nil {
		t.Fatal(err)
	}
	if wire.enables != [2]int{} {
		t.Fatalf("unconfigured family enables = %v", wire.enables)
	}
	if err := owner.AddAddressP2P(netip.MustParsePrefix("fe80::1/64"), netip.MustParsePrefix("fe80::2/64")); err != nil {
		t.Fatal(err)
	}
	if err := owner.Connect(); err != nil {
		t.Fatal(err)
	}
	if wire.enables != [2]int{0, 1} {
		t.Fatalf("late family enables = %v", wire.enables)
	}
}

// Removing the last negotiated pair withdraws that family's forwarding gate.
func TestVPPRemoveLastAddressDisablesItsFamily(t *testing.T) {
	wire := &ownershipWire{}
	owner := ownershipSession(t, wire)
	local := netip.MustParsePrefix("192.0.2.1/32")
	if err := owner.AddAddressP2P(local, netip.MustParsePrefix("192.0.2.7/32")); err != nil {
		t.Fatal(err)
	}
	if err := owner.Connect(); err != nil {
		t.Fatal(err)
	}
	if err := owner.RemoveAddress(local); err != nil {
		t.Fatal(err)
	}
	if wire.references[0] != 0 {
		t.Fatal("family remained enabled after its final pair was removed")
	}
	if wire.disables[0] != 1 {
		t.Fatalf("IPv4 disables = %d", wire.disables[0])
	}
}

// Native enable references must be reversed before the index enters VPP's pool.
func TestVPPCloseDisablesFamiliesBeforeRecycling(t *testing.T) {
	wire := &ownershipWire{}
	owner := ownershipSession(t, wire)
	if err := owner.AddAddressP2P(netip.MustParsePrefix("192.0.2.1/32"), netip.MustParsePrefix("192.0.2.7/32")); err != nil {
		t.Fatal(err)
	}
	if err := owner.Connect(); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if wire.unsafeRecycle {
		t.Fatal("native session recycled with an enabled family or retained route")
	}
	if wire.sessionDeletes != 1 {
		t.Fatalf("native deletes = %d", wire.sessionDeletes)
	}
	if wire.references != [2]int{} {
		t.Fatalf("enable references survived teardown: %v", wire.references)
	}
}

// A refused disable cannot be hidden by deleting and reusing the interface.
func TestVPPCloseDisableFailureKeepsNativeIndex(t *testing.T) {
	wire := &ownershipWire{disableRetval: [2]int32{-1, 0}}
	owner := ownershipSession(t, wire)
	if err := owner.AddAddressP2P(netip.MustParsePrefix("192.0.2.1/32"), netip.MustParsePrefix("192.0.2.7/32")); err != nil {
		t.Fatal(err)
	}
	if err := owner.Connect(); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err == nil {
		t.Fatal("failed family disable was hidden")
	}
	if wire.sessionDeletes != 0 {
		t.Fatal("failed disable allowed native index reuse")
	}
	if len(wire.retained) == 0 {
		t.Fatal("unreleased family owner was discarded")
	}
}

// A lost open receipt and unavailable readback cannot prove quarantine; the
// unknown family must never cause an unproven decrement or native index reuse.
func TestVPPUnknownFamilyEnableKeepsNativeIndex(t *testing.T) {
	want := errors.New("IPv6 enable reply lost")
	wire := &ownershipWire{enableReadError: [2]error{nil, want}, gateReadErrorAfterEnable: [2]error{nil, errors.New("gate readback unavailable")}}
	owner := ownershipSession(t, wire)
	if err := owner.AddAddressP2P(netip.MustParsePrefix("192.0.2.1/32"), netip.MustParsePrefix("192.0.2.7/32")); err != nil {
		t.Fatal(err)
	}
	if err := owner.AddAddressP2P(netip.MustParsePrefix("fe80::1/64"), netip.MustParsePrefix("fe80::2/64")); err != nil {
		t.Fatal(err)
	}
	if err := owner.Connect(); !errors.Is(err, want) {
		t.Fatalf("enable error = %v", err)
	}
	if err := owner.Close(); err == nil {
		t.Fatal("unknown enable outcome was reported cleaned")
	}
	if wire.sessionDeletes != 0 {
		t.Fatal("unknown enable allowed native index reuse")
	}
	if wire.disables[1] != 0 {
		t.Fatal("unknown IPv6 enable caused an unproven decrement")
	}
	if len(wire.retained) == 0 {
		t.Fatal("unknown enable owner was discarded")
	}
}

func ownershipGate(arc, featureName string) (int, int, error) {
	family, index := -1, -1
	switch arc {
	case "ip4-unicast":
		family, index = 0, 0
	case "ip4-multicast":
		family, index = 0, 1
	case "ip6-unicast":
		family, index = 1, 0
	case "ip6-multicast":
		family, index = 1, 1
	}
	if family < 0 {
		return 0, 0, errors.New("unexpected feature arc")
	}
	want := "ip4-not-enabled"
	if family == 1 {
		want = "ip6-not-enabled"
	}
	if featureName != want {
		return 0, 0, errors.New("unexpected feature node")
	}
	return family, index, nil
}

// A later exact readback resolves an uncertain open without adding another
// not-enabled feature. Confirmed closed gates then permit native index reuse.
func TestVPPUnknownFamilyEnableVerifiedQuarantineRecovers(t *testing.T) {
	want := errors.New("IPv6 enable reply lost")
	wire := &ownershipWire{enableReadError: [2]error{nil, want}, gateReadErrorAfterEnable: [2]error{nil, errors.New("gate readback unavailable")}}
	owner := ownershipSession(t, wire)
	if err := owner.AddAddressP2P(netip.MustParsePrefix("fe80::1/64"), netip.MustParsePrefix("fe80::2/64")); err != nil {
		t.Fatal(err)
	}
	if err := owner.Connect(); !errors.Is(err, want) {
		t.Fatalf("open error = %v", err)
	}
	wire.gateReadErrorAfterEnable[1] = nil
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if wire.unsafeRecycle || wire.overdraw {
		t.Fatal("unresolved forwarding state recycled")
	}
	if wire.sessionDeletes != 1 || wire.references != [2]int{} {
		t.Fatalf("native deletes=%d open families=%v", wire.sessionDeletes, wire.references)
	}
	if wire.disables[1] != 1 {
		t.Fatalf("quarantine mutations=%d", wire.disables[1])
	}
}
