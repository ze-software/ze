// Design: docs/architecture/l2tp/bng-5-pppoe.md -- VPP subscriber session lifetime
// VALIDATES: created-index counters, exact routes and teardown ownership.
package vpp

import (
	"context"
	"errors"
	"io"
	"net/netip"
	"sync"
	"testing"
	"time"

	"go.fd.io/govpp/api"
	interfaces "go.fd.io/govpp/binapi/interface"
	"go.fd.io/govpp/binapi/ip"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane/vpp/binapi/feature"
	pppoeapi "github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane/vpp/binapi/pppoe"
	"github.com/ze-software/ze/internal/core/rtproto"
)

type ownerStream struct {
	once   sync.Once
	done   chan struct{}
	closes int
}

func (s *ownerStream) Read([]byte) (int, error) { <-s.done; return 0, io.EOF }
func (s *ownerStream) Write(b []byte) (int, error) {
	select {
	case <-s.done:
		return 0, io.ErrClosedPipe
	default:
		return len(b), nil
	}
}
func (s *ownerStream) Close() error                    { s.once.Do(func() { s.closes++; close(s.done) }); return nil }
func (s *ownerStream) SetReadDeadline(time.Time) error { return nil }

type sessionRecorder struct {
	requests []api.Message
	failure  error
	gates    map[string]bool
}

func (r *sessionRecorder) open(ctx context.Context) (api.Stream, error) {
	return &sessionReplyStream{ctx: ctx, recorder: r}, nil
}

type sessionReplyStream struct {
	ctx      context.Context
	recorder *sessionRecorder
	request  api.Message
}

func (s *sessionReplyStream) Context() context.Context { return s.ctx }
func (s *sessionReplyStream) SendMsg(m api.Message) error {
	s.request = m
	s.recorder.requests = append(s.recorder.requests, m)
	return nil
}
func (s *sessionReplyStream) RecvMsg() (api.Message, error) {
	if s.recorder.failure != nil {
		return nil, s.recorder.failure
	}
	switch request := s.request.(type) {
	case *feature.FeatureIsEnabled:
		return &feature.FeatureIsEnabledReply{IsEnabled: s.recorder.gates[request.ArcName]}, nil
	case *feature.FeatureEnableDisable:
		s.recorder.gates[request.ArcName] = request.Enable
		return &feature.FeatureEnableDisableReply{}, nil
	case *ip.IPRouteAddDelV2:
		return &ip.IPRouteAddDelV2Reply{}, nil
	case *pppoeapi.PppoeAddDelSession:
		return &pppoeapi.PppoeAddDelSessionReply{}, nil
	case *interfaces.SwInterfaceSetFlags:
		return &interfaces.SwInterfaceSetFlagsReply{}, nil
	case *interfaces.SwInterfaceSetMtu:
		return &interfaces.SwInterfaceSetMtuReply{}, nil
	case *ip.SwInterfaceIP4EnableDisable:
		return &ip.SwInterfaceIP4EnableDisableReply{}, nil
	case *ip.SwInterfaceIP6EnableDisable:
		return &ip.SwInterfaceIP6EnableDisableReply{}, nil
	default:
		return nil, errors.New("unexpected operation")
	}
}
func (s *sessionReplyStream) Close() error { return nil }

func testSession(t *testing.T) (*session, *sessionRecorder, *ownerStream) {
	t.Helper()
	recorder := &sessionRecorder{gates: map[string]bool{"ip4-unicast": true, "ip4-multicast": true, "ip6-unicast": true, "ip6-multicast": true}}
	stream := &ownerStream{done: make(chan struct{})}
	s := &session{identity: ppp.SessionIdentity{InterfaceName: "pppoe_session9", NativeIndex: 73}, api: &binaryAPI{open: recorder.open}, socket: newControlSocket(stream), tables: [2]uint32{41, 42}, source: 17, sid: 19, mac: [6]byte{2, 0, 0, 0, 0, 1}, routes: make(map[sessionRoute]struct{}), addresses: make(map[netip.Prefix]sessionRoute), release: func() {}, retain: func(error) {}}
	t.Cleanup(func() {
		if err := s.Close(); err != nil && recorder.failure == nil {
			t.Error(err)
		}
	})
	return s, recorder, stream
}

// IPCP's peer address becomes an attached host route in the selected table,
// under the PPPoE-owned FIB source and the actual returned interface index.
func TestVPPSessionPeerRouteUsesCreatedIndexAndTable(t *testing.T) {
	s, r, _ := testSession(t)
	if err := s.AddAddressP2P(netip.MustParsePrefix("192.0.2.1/32"), netip.MustParsePrefix("192.0.2.7/32")); err != nil {
		t.Fatal(err)
	}
	if err := s.Connect(); err != nil {
		t.Fatal(err)
	}
	req, ok := r.requests[0].(*ip.IPRouteAddDelV2)
	if !ok {
		t.Fatalf("request %T", r.requests[0])
	}
	if !req.IsAdd || req.Route.Src != 17 || req.Route.TableID != 41 || len(req.Route.Paths) != 1 || req.Route.Paths[0].SwIfIndex != 73 || req.Route.Prefix.Len != 32 {
		t.Fatalf("route %+v", req)
	}
	if err := s.RemoveAddress(netip.MustParsePrefix("192.0.2.1/32")); err != nil {
		t.Fatal(err)
	}
	if r.requests[len(r.requests)-1].(*ip.IPRouteAddDelV2).IsAdd {
		t.Fatal("remove sent an add")
	}
}

// Close wakes the borrowed reader, removes every owned path before the VPP
// session, and leaves later operations refused without issuing another API call.
func TestVPPSessionCloseOwnsSocketAndTeardown(t *testing.T) {
	s, r, stream := testSession(t)
	if err := s.AddAddressP2P(netip.MustParsePrefix("192.0.2.1/32"), netip.MustParsePrefix("192.0.2.7/32")); err != nil {
		t.Fatal(err)
	}
	if err := s.Connect(); err != nil {
		t.Fatal(err)
	}
	if err := s.AddRoute(netip.MustParsePrefix("198.51.100.0/24"), netip.MustParseAddr("192.0.2.7"), 0, rtproto.Iface); err != nil {
		t.Fatal(err)
	}
	readDone := make(chan error, 1)
	go func() { var b [2]byte; _, err := s.Control().Read(b[:]); readDone <- err }()
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-readDone:
	case <-time.After(time.Second):
		t.Fatal("close did not wake reader")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if stream.closes != 1 {
		t.Fatalf("socket closes %d", stream.closes)
	}
	var removeRoute, removeSession int
	var removedFramed bool
	for i, request := range r.requests {
		switch req := request.(type) {
		case *ip.IPRouteAddDelV2:
			if !req.IsAdd {
				removeRoute = i
				if req.Route.Prefix.Address == vppAddress(netip.MustParseAddr("198.51.100.0")) {
					if req.Route.Prefix.Len == 24 {
						removedFramed = true
					}
				}
			}
		case *pppoeapi.PppoeAddDelSession:
			if !req.IsAdd {
				removeSession = i
			}
		}
	}
	if !removedFramed {
		t.Fatal("framed route remained after native session teardown")
	}
	if removeRoute >= removeSession {
		t.Fatalf("teardown order %+v", r.requests)
	}
	before := len(r.requests)
	if err := s.SetMTU(1492); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("closed MTU %v", err)
	}
	if len(r.requests) != before {
		t.Fatal("closed owner sent another API request")
	}
}

// Stats use the session index returned by VPP, never its access device or a
// renamed operator string. A missing/failed read is an error, not idle zero.
// The reader models the shared snapshot: it holds rows 17 and 73 and refuses
// any other index, as vppcomp.InterfaceCountersForGeneration does; its
// missing- and duplicate-row refusals are proven in the vpp package.
func TestVPPSessionCountersRequireExactCreatedIndex(t *testing.T) {
	s, _, _ := testSession(t)
	rows := map[uint32]api.InterfaceCounters{
		17: {InterfaceIndex: 17, Rx: api.InterfaceCounterCombined{Bytes: 1}},
		73: {InterfaceIndex: 73, Rx: api.InterfaceCounterCombined{Bytes: 123, Packets: 3}, Tx: api.InterfaceCounterCombined{Bytes: 456, Packets: 4}},
	}
	s.readCounters = func(index uint32, _ time.Time) (api.InterfaceCounters, error) {
		row, found := rows[index]
		if !found {
			return api.InterfaceCounters{}, errors.New("stats have no row for this index")
		}
		return row, nil
	}
	counters, err := s.Counters()
	if err != nil {
		t.Fatal(err)
	}
	if counters != (ppp.SessionCounters{RxBytes: 123, TxBytes: 456, RxPackets: 3, TxPackets: 4}) {
		t.Fatalf("counters %+v", counters)
	}
	s.readCounters = func(uint32, time.Time) (api.InterfaceCounters, error) {
		return api.InterfaceCounters{}, errors.New("stats unavailable")
	}
	if _, err := s.Counters(); err == nil {
		t.Fatal("stats error became an idle zero")
	}
	delete(rows, 73)
	s.readCounters = func(index uint32, _ time.Time) (api.InterfaceCounters, error) {
		row, found := rows[index]
		if !found {
			return api.InterfaceCounters{}, errors.New("stats have no row for this index")
		}
		return row, nil
	}
	if _, err := s.Counters(); err == nil {
		t.Fatal("missing created index became an idle zero")
	}
}

// Goal: Counters asks for a fresh dump, CountersWithin for one no older than
// its bound, and neither accepts a dump from before the interface existed.
// Method: record the bound each read passes to the shared snapshot reader.
// MUTATION: dropping the creation clamp, or serving Counters from the
// interval bound, turns this red.
func TestVPPSessionCounterFreshnessBounds(t *testing.T) {
	s, _, _ := testSession(t)
	s.created = time.Now().Add(-time.Hour)
	var asked time.Time
	s.readCounters = func(_ uint32, notBefore time.Time) (api.InterfaceCounters, error) {
		asked = notBefore
		return api.InterfaceCounters{InterfaceIndex: 73}, nil
	}
	within := func(name string, read func() (ppp.SessionCounters, error), offset time.Duration) {
		t.Helper()
		before := time.Now()
		if _, err := read(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		after := time.Now()
		if asked.Before(before.Add(-offset)) {
			t.Fatalf("%s asked for a dump from %v, older than %v", name, asked, before.Add(-offset))
		}
		if asked.After(after.Add(-offset)) {
			t.Fatalf("%s asked for a dump from %v, newer than %v", name, asked, after.Add(-offset))
		}
	}
	within("Counters", s.Counters, 0)
	within("CountersWithin(0)", func() (ppp.SessionCounters, error) { return s.CountersWithin(0) }, 0)
	within("CountersWithin(1m)", func() (ppp.SessionCounters, error) { return s.CountersWithin(time.Minute) }, time.Minute)
	if _, err := s.CountersWithin(2 * time.Hour); err != nil {
		t.Fatal(err)
	}
	if !asked.Equal(s.created) {
		t.Fatalf("a bound older than the interface asked for %v, want its creation %v", asked, s.created)
	}
	s.readCounters = nil
	if _, err := s.CountersWithin(time.Minute); err == nil {
		t.Fatal("a session with no stats connection answered")
	}
}

// Connection opens every negotiated family's unicast and multicast gates
// only once; repeated ready and disconnect transitions cannot stack features.
func TestVPPSessionConnectionProgramsIPPathsOnce(t *testing.T) {
	s, r, _ := testSession(t)
	if err := s.AddAddressP2P(netip.MustParsePrefix("192.0.2.1/32"), netip.MustParsePrefix("192.0.2.7/32")); err != nil {
		t.Fatal(err)
	}
	if err := s.AddAddressP2P(netip.MustParsePrefix("fe80::1/64"), netip.MustParsePrefix("fe80::2/64")); err != nil {
		t.Fatal(err)
	}
	if err := s.Connect(); err != nil {
		t.Fatal(err)
	}
	if err := s.Connect(); err != nil {
		t.Fatal(err)
	}
	mutations := func() map[string][2]int {
		counts := make(map[string][2]int)
		for _, message := range r.requests {
			if request, ok := message.(*feature.FeatureEnableDisable); ok {
				pair := counts[request.ArcName]
				if request.Enable {
					pair[1]++
				} else {
					pair[0]++
				}
				counts[request.ArcName] = pair
			}
		}
		return counts
	}
	for _, arc := range []string{"ip4-unicast", "ip4-multicast", "ip6-unicast", "ip6-multicast"} {
		if mutations()[arc] != [2]int{1, 0} {
			t.Fatalf("open mutations for %s: %v", arc, mutations()[arc])
		}
		if r.gates[arc] {
			t.Fatalf("gate %s remained closed", arc)
		}
	}
	if err := s.Disconnect(); err != nil {
		t.Fatal(err)
	}
	if err := s.Disconnect(); err != nil {
		t.Fatal(err)
	}
	for _, arc := range []string{"ip4-unicast", "ip4-multicast", "ip6-unicast", "ip6-multicast"} {
		if mutations()[arc] != [2]int{1, 1} {
			t.Fatalf("close mutations for %s: %v", arc, mutations()[arc])
		}
		if !r.gates[arc] {
			t.Fatalf("gate %s remained open", arc)
		}
	}
}
