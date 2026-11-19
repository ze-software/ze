// Design: docs/architecture/l2tp/bng-5-pppoe.md -- exact native path ownership
// Native model: pinned VPP fib_api_route_add_del uses is_multipath for path
// add/remove; false replaces or deletes the entire prefix/source entry.
package vpp

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/ip"

	"github.com/ze-software/ze/internal/core/rtproto"
)

type sharedPathStream struct {
	api.Stream
	paths       map[uint32]bool
	destination netip.Prefix
}

func (s *sharedPathStream) SendMsg(message api.Message) error {
	request, ok := message.(*ip.IPRouteAddDelV2)
	if !ok {
		return s.Stream.SendMsg(message)
	}
	if len(request.Route.Paths) != 1 {
		return errors.New("expected one exact subscriber path")
	}
	if request.Route.TableID != 41 || request.Route.Src != 17 {
		return errors.New("subscriber path targeted another table/source")
	}
	if request.Route.Prefix.Address != vppAddress(s.destination.Addr()) {
		return s.Stream.SendMsg(message)
	}
	if request.Route.Prefix.Len != uint8(s.destination.Bits()) {
		return s.Stream.SendMsg(message)
	}
	if !request.IsMultipath {
		clear(s.paths)
	}
	index := request.Route.Paths[0].SwIfIndex
	if request.IsAdd {
		s.paths[index] = true
	} else {
		delete(s.paths, index)
	}
	return s.Stream.SendMsg(message)
}

// Two subscribers may own the same prefix under ze-pppoe. Adding or removing
// either exact native path must preserve the other subscriber's working path.
func TestVPPSharedPrefixPreservesOtherSessionPath(t *testing.T) {
	paths := make(map[uint32]bool)
	first, _, _ := testSession(t)
	second, _, _ := testSession(t)
	second.identity.NativeIndex = 74
	destination := netip.MustParsePrefix("198.51.100.0/24")
	for _, owner := range []*session{first, second} {
		open := owner.api.open
		owner.api.open = func(ctx context.Context) (api.Stream, error) {
			stream, err := open(ctx)
			if err != nil {
				return nil, err
			}
			return &sharedPathStream{Stream: stream, paths: paths, destination: destination}, nil
		}
		if err := owner.AddAddressP2P(netip.MustParsePrefix("192.0.2.1/32"), netip.MustParsePrefix("192.0.2.7/32")); err != nil {
			t.Fatal(err)
		}
		if err := owner.Connect(); err != nil {
			t.Fatal(err)
		}
	}
	if err := first.AddRoute(destination, netip.Addr{}, 0, rtproto.Iface); err != nil {
		t.Fatal(err)
	}
	if err := second.AddRoute(destination, netip.Addr{}, 0, rtproto.Iface); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 || !paths[73] || !paths[74] {
		t.Fatalf("paths after second add: %v", paths)
	}
	if err := first.RemoveRoute(destination, netip.Addr{}, 0, rtproto.Iface); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || !paths[74] {
		t.Fatalf("second path after first withdrawal: %v", paths)
	}
	if err := second.RemoveRoute(destination, netip.Addr{}, 0, rtproto.Iface); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 0 {
		t.Fatalf("paths after final withdrawal: %v", paths)
	}
}

type withdrawalCloseFailure struct {
	api.Stream
	deleting bool
	failure  error
}

func (s *withdrawalCloseFailure) SendMsg(message api.Message) error {
	if request, ok := message.(*ip.IPRouteAddDelV2); ok {
		s.deleting = !request.IsAdd
	}
	return s.Stream.SendMsg(message)
}

func (s *withdrawalCloseFailure) Close() error {
	err := s.Stream.Close()
	if s.deleting {
		err = errors.Join(err, s.failure)
	}
	return err
}

// An acknowledged peer-route withdrawal ends its address witness even if
// releasing the API stream fails. The caller must still receive that failure.
func TestVPPConfirmedAddressWithdrawalDropsWitnessDespiteCloseError(t *testing.T) {
	wire := &ownershipWire{}
	owner := ownershipSession(t, wire)
	want := errors.New("withdrawal stream close failed")
	open := owner.api.open
	owner.api.open = func(ctx context.Context) (api.Stream, error) {
		stream, err := open(ctx)
		if err != nil {
			return nil, err
		}
		return &withdrawalCloseFailure{Stream: stream, failure: want}, nil
	}
	local := netip.MustParsePrefix("192.0.2.1/32")
	if err := owner.AddAddressP2P(local, netip.MustParsePrefix("192.0.2.7/32")); err != nil {
		t.Fatal(err)
	}
	if err := owner.Connect(); err != nil {
		t.Fatal(err)
	}
	if err := owner.RemoveAddress(local); !errors.Is(err, want) {
		t.Fatalf("withdrawal error: %v", err)
	}
	if len(owner.addresses) != 0 {
		t.Fatal("confirmed withdrawal retained a stale NCP address witness")
	}
	if wire.paths != 0 {
		t.Fatal("acknowledged peer route remained installed")
	}
}
