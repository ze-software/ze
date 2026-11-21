// Design: docs/architecture/l2tp/bng-5-pppoe.md -- forwarding publication barrier.
// Method: observe native paths and gates before and after post-ACK Connect.
package vpp

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/ip"
	"go.fd.io/govpp/binapi/ip_types"

	"github.com/ze-software/ze/internal/core/rtproto"
)

// Installing a late NCP pair must not publish a native egress route before
// accounting publication has acknowledged the new assignment.
func TestVPPPeerRouteWaitsForPostACKConnect(t *testing.T) {
	wire := &ownershipWire{}
	owner := ownershipSession(t, wire)
	if err := owner.Connect(); err != nil {
		t.Fatal(err)
	}
	if err := owner.AddAddressP2P(netip.MustParsePrefix("192.0.2.1/32"), netip.MustParsePrefix("192.0.2.7/32")); err != nil {
		t.Fatal(err)
	}
	if wire.paths != 0 {
		t.Fatalf("native peer routes before ACK = %d", wire.paths)
	}
	if err := owner.Connect(); err != nil {
		t.Fatal(err)
	}
	if wire.paths != 1 {
		t.Fatalf("native peer routes after ACK = %d", wire.paths)
	}
	if wire.references != [2]int{1, 0} {
		t.Fatalf("published families = %v", wire.references)
	}
}

// A framed route cannot provide a back door around the peer-route barrier;
// Connect without a family witness leaves it unpublished.
func TestVPPFramedRouteWaitsForOpenedFamily(t *testing.T) {
	wire := &ownershipWire{}
	owner := ownershipSession(t, wire)
	if err := owner.AddRoute(netip.MustParsePrefix("198.51.100.0/24"), netip.MustParseAddr("192.0.2.7"), 0, rtproto.Iface); err != nil {
		t.Fatal(err)
	}
	if err := owner.Connect(); err != nil {
		t.Fatal(err)
	}
	if wire.paths != 0 {
		t.Fatalf("framed route without an opened family: %d", wire.paths)
	}
	if err := owner.AddAddressP2P(netip.MustParsePrefix("192.0.2.1/32"), netip.MustParsePrefix("192.0.2.7/32")); err != nil {
		t.Fatal(err)
	}
	if wire.paths != 0 {
		t.Fatalf("native routes before assignment ACK: %d", wire.paths)
	}
	if err := owner.Connect(); err != nil {
		t.Fatal(err)
	}
	if wire.paths != 2 {
		t.Fatalf("published peer and framed routes = %d", wire.paths)
	}
	if wire.references != [2]int{1, 0} {
		t.Fatalf("published families = %v", wire.references)
	}
}

// Staged routes that never reached VPP are discarded on close without native
// withdrawals, while the created session is still released normally.
func TestVPPUnpublishedRoutesNeedNoNativeWithdrawal(t *testing.T) {
	wire := &ownershipWire{}
	owner := ownershipSession(t, wire)
	if err := owner.AddRoute(netip.MustParsePrefix("198.51.100.0/24"), netip.Addr{}, 0, rtproto.Iface); err != nil {
		t.Fatal(err)
	}
	if err := owner.AddAddressP2P(netip.MustParsePrefix("192.0.2.1/32"), netip.MustParsePrefix("192.0.2.7/32")); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if wire.paths != 0 {
		t.Fatalf("unpublished native routes = %d", wire.paths)
	}
	if wire.pathDeletes != 0 {
		t.Fatalf("native withdrawals for unpublished intents = %d", wire.pathDeletes)
	}
	if wire.sessionDeletes != 1 {
		t.Fatalf("native session deletes = %d", wire.sessionDeletes)
	}
}

// All routes of one new family must be confirmed before its gate opens. A
// missing framed-route receipt preserves the independent family's service.
func TestVPPPartiallyPublishedRoutesKeepFamilyClosed(t *testing.T) {
	wire := &ownershipWire{}
	owner := ownershipSession(t, wire)
	if err := owner.AddAddressP2P(netip.MustParsePrefix("fe80::1/64"), netip.MustParsePrefix("fe80::2/64")); err != nil {
		t.Fatal(err)
	}
	if err := owner.Connect(); err != nil {
		t.Fatal(err)
	}
	framed := netip.MustParsePrefix("198.51.100.0/24")
	want := errors.New("framed route reply lost")
	open := owner.api.open
	owner.api.open = func(ctx context.Context) (api.Stream, error) {
		stream, err := open(ctx)
		if err != nil {
			return nil, err
		}
		return &stagedReplyFailure{Stream: stream, destination: ip_types.Prefix{Address: vppAddress(framed.Addr()), Len: uint8(framed.Bits())}, failure: want}, nil
	}
	if err := owner.AddAddressP2P(netip.MustParsePrefix("192.0.2.1/32"), netip.MustParsePrefix("192.0.2.7/32")); err != nil {
		t.Fatal(err)
	}
	if err := owner.AddRoute(framed, netip.MustParseAddr("192.0.2.7"), 0, rtproto.Iface); err != nil {
		t.Fatal(err)
	}
	if err := owner.Connect(); !errors.Is(err, want) {
		t.Fatalf("publication error = %v", err)
	}
	if wire.references != [2]int{0, 1} {
		t.Fatalf("partial family or independent survivor = %v", wire.references)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if wire.paths != 0 {
		t.Fatalf("partial publication escaped cleanup: %d paths", wire.paths)
	}
	if wire.unsafeRecycle {
		t.Fatal("partial publication recycled its native index")
	}
}

// Retrying an uncertain exact path follows VPP's duplicate-path semantics:
// publication confirms one path, and one withdrawal removes it completely.
func TestVPPUnknownStagedRouteRetryDoesNotDuplicatePath(t *testing.T) {
	want := errors.New("peer route reply lost")
	wire := &ownershipWire{addReadError: want}
	owner := ownershipSession(t, wire)
	paths := make(map[ip_types.Prefix]bool)
	open := owner.api.open
	owner.api.open = func(ctx context.Context) (api.Stream, error) {
		stream, err := open(ctx)
		if err != nil {
			return nil, err
		}
		return &stagedPathSet{Stream: stream, paths: paths, wire: wire}, nil
	}
	if err := owner.AddAddressP2P(netip.MustParsePrefix("192.0.2.1/32"), netip.MustParsePrefix("192.0.2.7/32")); err != nil {
		t.Fatal(err)
	}
	if err := owner.Connect(); !errors.Is(err, want) {
		t.Fatalf("publication error = %v", err)
	}
	if wire.references != [2]int{} {
		t.Fatalf("uncertain family opened: %v", wire.references)
	}
	if len(paths) != 1 {
		t.Fatalf("uncertain native path count = %d", len(paths))
	}
	wire.addReadError = nil
	if err := owner.Connect(); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 {
		t.Fatalf("reassertion duplicated the exact path: %d", len(paths))
	}
	if wire.references != [2]int{1, 0} {
		t.Fatalf("confirmed family remained closed: %v", wire.references)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 0 {
		t.Fatalf("confirmed route escaped one withdrawal: %d", len(paths))
	}
	if wire.unsafeRecycle {
		t.Fatal("session index reused before the exact path was gone")
	}
}

// The route bound covers unpublished intents too, so a closed family cannot
// accumulate an unbounded queue while waiting for assignment publication.
func TestVPPStagedRoutesShareNativeResourceBound(t *testing.T) {
	wire := &ownershipWire{}
	owner := ownershipSession(t, wire)
	for index := range sessionRouteMax {
		address := netip.AddrFrom4([4]byte{198, 51, byte(index >> 8), byte(index)})
		if err := owner.AddRoute(netip.PrefixFrom(address, 32), netip.Addr{}, 0, rtproto.Iface); err != nil {
			t.Fatal(err)
		}
	}
	if err := owner.AddRoute(netip.MustParsePrefix("203.0.113.1/32"), netip.Addr{}, 0, rtproto.Iface); err == nil {
		t.Fatal("unpublished routes exceeded the session resource bound")
	}
	if wire.paths != 0 {
		t.Fatalf("unopened family published %d routes", wire.paths)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if wire.pathDeletes != 0 {
		t.Fatalf("unpublished intents needed %d native deletes", wire.pathDeletes)
	}
}

// Withdrawing an uncertain staged framed route cancels its publication
// intent before deletion. A failed delete stays owned but is never re-added.
func TestVPPFailedStagedRouteWithdrawalCancelsPublication(t *testing.T) {
	wire := &ownershipWire{}
	owner := ownershipSession(t, wire)
	destination := netip.MustParsePrefix("198.51.100.0/24")
	prefix := ip_types.Prefix{Address: vppAddress(destination.Addr()), Len: uint8(destination.Bits())}
	want := errors.New("framed route reply lost")
	open := owner.api.open
	owner.api.open = func(ctx context.Context) (api.Stream, error) {
		stream, err := open(ctx)
		if err != nil {
			return nil, err
		}
		return &stagedReplyFailure{Stream: stream, destination: prefix, failure: want}, nil
	}
	if err := owner.AddAddressP2P(netip.MustParsePrefix("192.0.2.1/32"), netip.MustParsePrefix("192.0.2.7/32")); err != nil {
		t.Fatal(err)
	}
	if err := owner.AddRoute(destination, netip.Addr{}, 0, rtproto.Iface); err != nil {
		t.Fatal(err)
	}
	if err := owner.Connect(); !errors.Is(err, want) {
		t.Fatalf("publication error = %v", err)
	}
	owner.api.open = open
	wire.deleteRouteRetval = -1
	if err := owner.RemoveRoute(destination, netip.Addr{}, 0, rtproto.Iface); err == nil {
		t.Fatal("native withdrawal refusal was hidden")
	}
	var adds int
	owner.api.open = func(ctx context.Context) (api.Stream, error) {
		stream, err := open(ctx)
		if err != nil {
			return nil, err
		}
		return &stagedAddCounter{Stream: stream, destination: prefix, adds: &adds}, nil
	}
	if err := owner.Connect(); err != nil {
		t.Fatal(err)
	}
	if adds != 0 {
		t.Fatal("a retiring staged route was reasserted during Connect")
	}
	wire.deleteRouteRetval = 0
	if err := owner.RemoveRoute(destination, netip.Addr{}, 0, rtproto.Iface); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if wire.paths != 0 {
		t.Fatalf("retiring framed path escaped cleanup: %d", wire.paths)
	}
}

// Closing the last pair withdraws its peer and every framed path of that
// family. Reopening restores generic intents after ACK, never the old peer.
func TestVPPLastPairWithdrawalRestagesOnlyItsFamilyRoutes(t *testing.T) {
	wire := &ownershipWire{}
	owner := ownershipSession(t, wire)
	paths := stagedNativePaths(owner, wire)
	local := netip.MustParsePrefix("192.0.2.1/32")
	oldPeer := netip.MustParsePrefix("192.0.2.7/32")
	newPeer := netip.MustParsePrefix("192.0.2.8/32")
	framed4 := netip.MustParsePrefix("198.51.100.0/24")
	framed6 := netip.MustParsePrefix("2001:db8:100::/56")
	if err := owner.AddAddressP2P(local, oldPeer); err != nil {
		t.Fatal(err)
	}
	if err := owner.AddAddressP2P(netip.MustParsePrefix("fe80::1/64"), netip.MustParsePrefix("fe80::2/64")); err != nil {
		t.Fatal(err)
	}
	if err := owner.Connect(); err != nil {
		t.Fatal(err)
	}
	if err := owner.AddRoute(framed4, netip.Addr{}, 0, rtproto.Iface); err != nil {
		t.Fatal(err)
	}
	if err := owner.AddRoute(framed6, netip.Addr{}, 0, rtproto.Iface); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 4 {
		t.Fatalf("initial native paths = %d", len(paths))
	}
	if err := owner.RemoveAddress(local); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 {
		t.Fatalf("last-pair withdrawal retained family paths: %d", len(paths))
	}
	if paths[stagedRoutePrefix(oldPeer)] {
		t.Fatal("old peer path survived NCP closure")
	}
	if paths[stagedRoutePrefix(framed4)] {
		t.Fatal("closed family retained native framed egress")
	}
	if !paths[stagedRoutePrefix(framed6)] {
		t.Fatal("independent IPv6 framed path was withdrawn")
	}
	if wire.references != [2]int{0, 1} {
		t.Fatalf("family gates after withdrawal = %v", wire.references)
	}
	if err := owner.Connect(); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 {
		t.Fatal("Connect without a new pair republished the closed family")
	}
	if err := owner.AddAddressP2P(local, newPeer); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 {
		t.Fatal("replacement published before its assignment ACK")
	}
	if err := owner.Connect(); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 4 {
		t.Fatalf("reopened native paths = %d", len(paths))
	}
	if paths[stagedRoutePrefix(oldPeer)] {
		t.Fatal("reopening restored the retired peer")
	}
	if !paths[stagedRoutePrefix(newPeer)] {
		t.Fatal("reopening did not install the negotiated peer")
	}
	if !paths[stagedRoutePrefix(framed4)] {
		t.Fatal("reopening lost the owned generic framed intent")
	}
}

// A failed family withdrawal is a quarantine obligation. Even a newly staged
// replacement cannot reopen it until every old native family path is gone.
func TestVPPFailedFamilyWithdrawalBlocksRepublication(t *testing.T) {
	wire := &ownershipWire{}
	owner := ownershipSession(t, wire)
	paths := stagedNativePaths(owner, wire)
	local := netip.MustParsePrefix("192.0.2.1/32")
	oldPeer := netip.MustParsePrefix("192.0.2.7/32")
	framed := netip.MustParsePrefix("198.51.100.0/24")
	if err := owner.AddAddressP2P(local, oldPeer); err != nil {
		t.Fatal(err)
	}
	if err := owner.AddAddressP2P(netip.MustParsePrefix("fe80::1/64"), netip.MustParsePrefix("fe80::2/64")); err != nil {
		t.Fatal(err)
	}
	if err := owner.Connect(); err != nil {
		t.Fatal(err)
	}
	if err := owner.AddRoute(framed, netip.Addr{}, 0, rtproto.Iface); err != nil {
		t.Fatal(err)
	}
	wire.deleteRouteRetval = -1
	if err := owner.RemoveAddress(local); err == nil {
		t.Fatal("family route withdrawal refusal was hidden")
	}
	if err := owner.AddAddressP2P(netip.MustParsePrefix("192.0.2.9/32"), netip.MustParsePrefix("192.0.2.8/32")); err != nil {
		t.Fatal(err)
	}
	if err := owner.Connect(); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 3 {
		t.Fatalf("failed family cleanup published a replacement: %d paths", len(paths))
	}
	if wire.references != [2]int{0, 1} {
		t.Fatalf("failed family cleanup reopened its gate: %v", wire.references)
	}
	wire.deleteRouteRetval = 0
	if err := owner.RemoveAddress(local); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 {
		t.Fatalf("old family paths after retry = %d", len(paths))
	}
	if err := owner.Connect(); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 3 {
		t.Fatalf("recovered family did not publish both intended paths: %d", len(paths))
	}
	if paths[stagedRoutePrefix(oldPeer)] {
		t.Fatal("recovery restored the retired peer")
	}
	if wire.references != [2]int{1, 1} {
		t.Fatalf("recovered family gates = %v", wire.references)
	}
}

func stagedNativePaths(owner *session, wire *ownershipWire) map[ip_types.Prefix]bool {
	paths := make(map[ip_types.Prefix]bool)
	open := owner.api.open
	owner.api.open = func(ctx context.Context) (api.Stream, error) {
		stream, err := open(ctx)
		if err != nil {
			return nil, err
		}
		return &stagedPathSet{Stream: stream, paths: paths, wire: wire}, nil
	}
	return paths
}

func stagedRoutePrefix(prefix netip.Prefix) ip_types.Prefix {
	return ip_types.Prefix{Address: vppAddress(prefix.Addr()), Len: uint8(prefix.Bits())}
}

type stagedAddCounter struct {
	api.Stream
	destination ip_types.Prefix
	adds        *int
}

func (s *stagedAddCounter) SendMsg(message api.Message) error {
	if request, ok := message.(*ip.IPRouteAddDelV2); ok {
		if request.IsAdd {
			if request.Route.Prefix == s.destination {
				*s.adds++
			}
		}
	}
	return s.Stream.SendMsg(message)
}

// Route-ownership regressions need a genuinely published family before their
// framed-route mutation. Faults affect the original tested mutation only.
func openedOwnershipSession(t *testing.T, wire *ownershipWire) *session {
	t.Helper()
	readError, closeError := wire.addReadError, wire.addCloseError
	deleteRetval := wire.deleteRouteRetval
	wire.addReadError, wire.addCloseError, wire.deleteRouteRetval = nil, nil, 0
	defer func() {
		wire.addReadError, wire.addCloseError, wire.deleteRouteRetval = readError, closeError, deleteRetval
	}()
	owner := ownershipSession(t, wire)
	if err := owner.AddAddressP2P(netip.MustParsePrefix("192.0.2.1/32"), netip.MustParsePrefix("192.0.2.7/32")); err != nil {
		t.Fatal(err)
	}
	if err := owner.Connect(); err != nil {
		t.Fatal(err)
	}
	return owner
}

type stagedPathSet struct {
	api.Stream
	paths map[ip_types.Prefix]bool
	wire  *ownershipWire
}

func (s *stagedPathSet) SendMsg(message api.Message) error {
	if err := s.Stream.SendMsg(message); err != nil {
		return err
	}
	if request, ok := message.(*ip.IPRouteAddDelV2); ok {
		if request.IsAdd {
			s.paths[request.Route.Prefix] = true
		} else if s.wire.deleteRouteRetval == 0 {
			delete(s.paths, request.Route.Prefix)
		}
		s.wire.paths = len(s.paths)
	}
	return nil
}

type stagedReplyFailure struct {
	api.Stream
	destination ip_types.Prefix
	failure     error
	request     api.Message
}

func (s *stagedReplyFailure) SendMsg(message api.Message) error {
	s.request = message
	return s.Stream.SendMsg(message)
}

func (s *stagedReplyFailure) RecvMsg() (api.Message, error) {
	if request, ok := s.request.(*ip.IPRouteAddDelV2); ok {
		if request.IsAdd {
			if request.Route.Prefix == s.destination {
				return nil, s.failure
			}
		}
	}
	return s.Stream.RecvMsg()
}
