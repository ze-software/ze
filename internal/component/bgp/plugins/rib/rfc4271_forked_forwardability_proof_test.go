// Design: docs/architecture/route-selection.md -- forked next-hop forwardability and revision invalidation
// Related: rfc4271_forwardability_session_test.go -- in-process session forwardability proofs
// Related: rib_remote_test.go -- forked socket metric feed fixture
// RFC: rfc/short/rfc4271.md -- Sections 9.1.2, 9.1.3, and 9.2

package rib

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/igpcost"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

type forkedSelectedEvent struct {
	prefix         netip.Prefix
	kind           rpc.SelectedRouteKind
	emittedNextHop netip.Addr
	destResolved   bool
	nhResolved     bool
	stableRevision bool
	revision       uint64
}

type forkedForwardabilityHarness struct {
	ctx              context.Context
	r                *RIBManager
	destPeer         netip.Addr
	igpProto         redistevents.ProtocolID
	mu               sync.Mutex
	engineRIB        *locrib.RIB
	events           chan forkedSelectedEvent
	serverErrors     chan error
	prospectiveCalls atomic.Int64
	pollCalls        atomic.Int64
	msgSeq           atomic.Uint64
}

func newForkedForwardabilityHarness(t *testing.T) *forkedForwardabilityHarness {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	clientSide, engineSide := net.Pipe()
	client := sdk.NewWithConn("bgp-rib", clientSide)
	engine := rpc.NewMuxConn(rpc.NewConn(engineSide, engineSide))
	h := &forkedForwardabilityHarness{
		ctx:          ctx,
		destPeer:     netip.MustParseAddr("192.0.2.250"),
		igpProto:     redistevents.RegisterProtocol("rfc4271-forked-igp"),
		engineRIB:    locrib.NewRIB(),
		events:       make(chan forkedSelectedEvent, 32),
		serverErrors: make(chan error, 4),
	}
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		h.serveEngine(engine)
	}()

	r := newRIBManager(client)
	require.Nil(t, r.locRIB.Load(), "forked proof must not use an in-process Loc-RIB")
	r.sourceLeases = newRIBSourceLeases(r)
	r.setupRemoteRIB()
	d, err := newSelectedDispatch(ctx, client.SelectRoutes, nil, nil)
	require.NoError(t, err)
	r.selectedDispatch = d
	require.NoError(t, d.destinationUp(h.destPeer, 901))
	h.r = r

	pollStopped := make(chan struct{})
	go func() {
		defer close(pollStopped)
		r.runRemoteMetrics(ctx)
	}()

	t.Cleanup(func() {
		cancel()
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()
		_ = d.stop(stopCtx)
		_ = client.Close()
		_ = engine.Close()
		<-pollStopped
		<-serverDone
		for _, routes := range r.bgpPeers {
			routes.Release()
		}
	})
	return h
}

func (h *forkedForwardabilityHarness) serveEngine(engine *rpc.MuxConn) {
	for req := range engine.Requests() {
		result, err := h.handleEngineRequest(req)
		if err == nil {
			err = engine.SendResult(h.ctx, req.ID, result)
		}
		if err != nil {
			if h.ctx.Err() != nil {
				return
			}
			select {
			case h.serverErrors <- err:
			default:
			}
			_ = engine.SendError(h.ctx, req.ID, err.Error())
			return
		}
	}
}

func (h *forkedForwardabilityHarness) handleEngineRequest(req *rpc.Request) (any, error) {
	switch req.Method {
	case rpc.MethodRouteMetrics:
		return h.handleRouteMetrics(req.Params)
	case rpc.MethodRouteInstall:
		return h.handleRouteInstall(req.Params)
	case rpc.MethodRouteRemove:
		return h.handleRouteRemove(req.Params)
	case rpc.MethodSelectRoutes:
		return h.handleSelectRoutes(req.Params)
	case rpc.MethodSourceLease:
		var input rpc.SourceLeaseInput
		if err := json.Unmarshal(req.Params, &input); err != nil {
			return nil, err
		}
		return &rpc.SourceLeaseOutput{
			Action:        input.Action,
			SourceReceipt: input.SourceReceipt,
			PeerOwner:     input.PeerOwner,
		}, nil
	case rpc.MethodReportIssue:
		return struct{}{}, nil
	default:
		return nil, fmt.Errorf("unexpected engine RPC %s", req.Method)
	}
}

func (h *forkedForwardabilityHarness) handleRouteMetrics(params json.RawMessage) (*rpc.RouteMetricsOutput, error) {
	var input rpc.RouteMetricsInput
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, err
	}
	if len(input.Addresses) == 0 {
		h.pollCalls.Add(1)
	}
	if len(input.ProspectiveRoutes) > 0 {
		if len(input.ProspectiveRoutes) != len(input.Addresses) {
			return nil, fmt.Errorf("prospective count %d != address count %d", len(input.ProspectiveRoutes), len(input.Addresses))
		}
		h.prospectiveCalls.Add(1)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	out := &rpc.RouteMetricsOutput{
		Revision:  h.engineRIB.Revision(),
		Distances: make([]rpc.RouteMetric, len(input.Addresses)),
	}
	for i, text := range input.Addresses {
		addr, err := netip.ParseAddr(text)
		if err != nil {
			return nil, err
		}
		d := igpcost.Resolve(h.engineRIB, addr)
		if len(input.ProspectiveRoutes) > 0 {
			entry := &input.ProspectiveRoutes[i]
			fam, prefix, path, err := decodeForkedInstallEntry(entry)
			if err != nil {
				return nil, err
			}
			d = igpcost.ResolveProspective(h.engineRIB, addr, fam, prefix, path)
		}
		out.Distances[i] = rpc.RouteMetric{
			Cost:        d.Cost,
			Resolved:    d.Resolved,
			Recursive:   d.Recursive,
			MissingAIGP: d.MissingAIGP,
		}
	}
	return out, nil
}

func (h *forkedForwardabilityHarness) handleRouteInstall(params json.RawMessage) (*rpc.RouteInstallOutput, error) {
	var input rpc.RouteInstallInput
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := range input.Routes {
		fam, prefix, path, err := decodeForkedInstallEntry(&input.Routes[i])
		if err != nil {
			return nil, err
		}
		h.engineRIB.InsertForward(fam, prefix, path, nil)
	}
	return &rpc.RouteInstallOutput{Installed: uint32(len(input.Routes))}, nil
}

func (h *forkedForwardabilityHarness) handleRouteRemove(params json.RawMessage) (*rpc.RouteRemoveOutput, error) {
	var input rpc.RouteRemoveInput
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := range input.Routes {
		e := &input.Routes[i]
		prefix, err := netip.ParsePrefix(e.Prefix)
		if err != nil {
			return nil, err
		}
		fam := family.Family{AFI: family.AFI(e.AFI), SAFI: family.SAFI(e.SAFI)}
		h.engineRIB.Remove(fam, prefix, bgpProtocolID, e.Instance)
	}
	return &rpc.RouteRemoveOutput{Removed: uint32(len(input.Routes))}, nil
}

func (h *forkedForwardabilityHarness) handleSelectRoutes(params json.RawMessage) (*rpc.SelectRoutesOutput, error) {
	var input rpc.SelectRoutesInput
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, err
	}
	for _, route := range input.Routes {
		prefix, err := decodeForkedIPv4Prefix(route.KeyNLRI)
		if err != nil {
			return nil, err
		}
		if route.Kind == rpc.SelectedRouteAbsent {
			h.events <- forkedSelectedEvent{prefix: prefix, kind: rpc.SelectedRouteAbsent}
			continue
		}
		if route.Route == nil {
			return nil, fmt.Errorf("selected route %s missing stored route payload", prefix)
		}
		emittedNH, err := decodeForkedNextHopAttr(route.Route.AttrHex)
		if err != nil {
			return nil, err
		}
		h.mu.Lock()
		revBefore := h.engineRIB.Revision()
		destResolved := igpcost.Resolve(h.engineRIB, prefix.Addr().Next()).Resolved
		nhResolved := igpcost.Resolve(h.engineRIB, emittedNH).Resolved
		revAfter := h.engineRIB.Revision()
		h.mu.Unlock()
		h.events <- forkedSelectedEvent{
			prefix:         prefix,
			kind:           route.Kind,
			emittedNextHop: emittedNH,
			destResolved:   destResolved,
			nhResolved:     nhResolved,
			stableRevision: revBefore == revAfter,
			revision:       revBefore,
		}
	}
	return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
}

func decodeForkedInstallEntry(entry *rpc.RouteInstallEntry) (family.Family, netip.Prefix, locrib.Path, error) {
	prefix, err := netip.ParsePrefix(entry.Prefix)
	if err != nil {
		return family.Family{}, netip.Prefix{}, locrib.Path{}, err
	}
	var nh netip.Addr
	if entry.NextHop != "" {
		nh, err = netip.ParseAddr(entry.NextHop)
		if err != nil {
			return family.Family{}, netip.Prefix{}, locrib.Path{}, err
		}
	}
	fam := family.Family{AFI: family.AFI(entry.AFI), SAFI: family.SAFI(entry.SAFI)}
	path := locrib.Path{
		Source:      bgpProtocolID,
		Instance:    entry.Instance,
		NextHop:     nh,
		IsBGP:       entry.IsBGP,
		IsEBGP:      entry.IsEBGP,
		Metric:      entry.Metric,
		AIGP:        entry.AIGP,
		AIGPPresent: entry.AIGPPresent,
	}
	return fam, prefix, path, nil
}

func encodeForkedIPv4Prefix(prefix netip.Prefix) []byte {
	bits := prefix.Bits()
	nbytes := (bits + 7) / 8
	raw := prefix.Addr().As4()
	out := make([]byte, 1+nbytes)
	out[0] = byte(bits)
	copy(out[1:], raw[:nbytes])
	return out
}

func decodeForkedIPv4Prefix(wire []byte) (netip.Prefix, error) {
	if len(wire) < 1 {
		return netip.Prefix{}, fmt.Errorf("empty IPv4 wire NLRI")
	}
	bits := int(wire[0])
	nbytes := (bits + 7) / 8
	if bits > 32 || len(wire) != 1+nbytes {
		return netip.Prefix{}, fmt.Errorf("malformed IPv4 wire NLRI %x", wire)
	}
	var raw [4]byte
	copy(raw[:], wire[1:])
	return netip.PrefixFrom(netip.AddrFrom4(raw), bits), nil
}

func decodeForkedNextHopAttr(attrHex string) (netip.Addr, error) {
	attrs, err := hex.DecodeString(attrHex)
	if err != nil {
		return netip.Addr{}, err
	}
	for len(attrs) > 0 {
		_, code, length, header, err := attribute.ParseHeader(attrs)
		if err != nil {
			return netip.Addr{}, err
		}
		if len(attrs)-header < int(length) {
			return netip.Addr{}, fmt.Errorf("truncated path attribute %d", code)
		}
		if code == attribute.AttrNextHop {
			if length != 4 {
				return netip.Addr{}, fmt.Errorf("unexpected NEXT_HOP length %d", length)
			}
			return netip.AddrFrom4([4]byte(attrs[header : header+4])), nil
		}
		attrs = attrs[header+int(length):]
	}
	return netip.Addr{}, fmt.Errorf("emitted attributes carry no NEXT_HOP")
}

func (h *forkedForwardabilityHarness) setIGP(prefix netip.Prefix, cost uint32) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.engineRIB.Insert(family.IPv4Unicast, prefix, locrib.Path{Source: h.igpProto, Metric: cost})
}

func (h *forkedForwardabilityHarness) removeIGP(prefix netip.Prefix) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.engineRIB.Remove(family.IPv4Unicast, prefix, h.igpProto, 0)
}

func (h *forkedForwardabilityHarness) engineHasPrefix(prefix netip.Prefix) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	_, ok := h.engineRIB.Best(family.IPv4Unicast, prefix)
	return ok
}

func (h *forkedForwardabilityHarness) receiveUpdate(t *testing.T, owner uint64, peerAddr string, prefix netip.Prefix, nextHop netip.Addr) {
	t.Helper()
	seq := h.msgSeq.Add(1)
	attrs := aigpSelectionAttrs(nextHop.As4(), nil, 100, 1)
	wireNLRI := encodeForkedIPv4Prefix(prefix)
	encoded := fmt.Sprintf(
		`{"type":"bgp","bgp":{"peer-owner":%d,"connection-id":"41","source-policy-epoch":1,"source-receipt":%d,"message":{"type":"update","id":%d,"direction":"received"},"peer":{"address":%q,"remote":{"address":%q,"as":65001},"local":{"as":65000}},"raw":{"attributes":%q,"nlri":{"ipv4/unicast":%q}},"update":{"nlri":{"ipv4/unicast":[{"action":"add","next-hop":%q,"nlri":[%q]}]}}}}`,
		owner, owner*1000+seq, seq, peerAddr, peerAddr, hex.EncodeToString(attrs), hex.EncodeToString(wireNLRI), nextHop.String(), prefix.String(),
	)
	event, err := parseEvent([]byte(encoded))
	require.NoError(t, err)
	h.r.dispatch(event)
}

func (h *forkedForwardabilityHarness) awaitEvent(t *testing.T, timeout time.Duration, predicate func(forkedSelectedEvent) bool, desc string) forkedSelectedEvent {
	t.Helper()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case ev := <-h.events:
			if predicate(ev) {
				return ev
			}
		case err := <-h.serverErrors:
			t.Fatalf("engine server error while waiting for %s: %v", desc, err)
		case <-timer.C:
			t.Fatalf("timed out waiting for %s", desc)
		case <-h.ctx.Done():
			t.Fatalf("context canceled waiting for %s: %v", desc, h.ctx.Err())
		}
	}
}

func (h *forkedForwardabilityHarness) drainEvents() []forkedSelectedEvent {
	var out []forkedSelectedEvent
	for {
		select {
		case ev := <-h.events:
			out = append(out, ev)
		default:
			return out
		}
	}
}

// TestRFC4271ForkedRemotePublicationForwardableAtStableRevision exercises the
// forked-plugin remote publication boundary (RouteMetricsProspective,
// RouteInstall/RouteRemove, and SelectRoutes over framed JSON socket RPC).
// Every successful SelectedRouteReceived publication MUST find both the
// destination host and the emitted wire NEXT_HOP resolvable in the engine
// Routing Table at one unchanged revision, and reachability loss/restoration
// MUST withdraw and recover the route cleanly without losing the control.
//
// RFC requirement: RFC4271-9.2-2 positive -- across the forked-plugin remote
// route publication boundary, every published route resolves both its
// destination and emitted NEXT_HOP in the engine Routing Table at one stable
// revision, withdraws when reachability disappears, and recovers when
// reachability returns.
func TestRFC4271ForkedRemotePublicationForwardableAtStableRevision(t *testing.T) {
	h := newForkedForwardabilityHarness(t)
	controlNH := netip.MustParseAddr("198.51.100.10")
	subjectNH := netip.MustParseAddr("198.51.100.20")
	controlPrefix := netip.MustParsePrefix("203.0.113.0/24")
	subjectPrefix := netip.MustParsePrefix("198.18.20.0/24")
	subjectHost := netip.PrefixFrom(subjectNH, 32)

	h.setIGP(netip.PrefixFrom(controlNH, 32), 5)
	h.receiveUpdate(t, 1, "192.0.2.1", controlPrefix, controlNH)
	h.receiveUpdate(t, 2, "192.0.2.2", subjectPrefix, subjectNH)

	controlEv := h.awaitEvent(t, 4*time.Second, func(ev forkedSelectedEvent) bool {
		return ev.prefix == controlPrefix && ev.kind == rpc.SelectedRouteReceived
	}, "control route published")
	require.True(t, controlEv.stableRevision, "control publication must observe one unchanged Routing Table revision")
	require.True(t, controlEv.destResolved, "control destination must already be forwardable in the engine Routing Table on SelectRoutes")
	require.True(t, controlEv.nhResolved, "control emitted NEXT_HOP must be forwardable in the engine Routing Table")
	require.Equal(t, controlNH, controlEv.emittedNextHop)
	require.False(t, h.engineHasPrefix(subjectPrefix), "unreachable subject must not be installed in engine Routing Table")
	for _, ev := range h.drainEvents() {
		require.NotEqual(t, subjectPrefix, ev.prefix, "unreachable subject must not be published before reachability exists")
	}

	h.setIGP(subjectHost, 10)
	subjectAnnounce := h.awaitEvent(t, 5*time.Second, func(ev forkedSelectedEvent) bool {
		return ev.prefix == subjectPrefix && ev.kind == rpc.SelectedRouteReceived
	}, "subject route published after IGP reachability arrives")
	require.True(t, subjectAnnounce.stableRevision, "subject publication must observe one unchanged Routing Table revision")
	require.True(t, subjectAnnounce.destResolved, "subject destination must already be forwardable in the engine Routing Table on SelectRoutes")
	require.True(t, subjectAnnounce.nhResolved, "subject emitted NEXT_HOP must be forwardable in the engine Routing Table")
	require.Equal(t, subjectNH, subjectAnnounce.emittedNextHop)
	require.Greater(t, h.prospectiveCalls.Load(), int64(0), "forked admission must query RouteMetricsProspective")

	h.removeIGP(subjectHost)
	h.awaitEvent(t, 5*time.Second, func(ev forkedSelectedEvent) bool {
		return ev.prefix == subjectPrefix && ev.kind == rpc.SelectedRouteAbsent
	}, "subject route withdrawn after IGP reachability is removed")
	require.False(t, h.engineHasPrefix(subjectPrefix), "withdrawn subject must be removed from engine Routing Table")
	require.True(t, h.engineHasPrefix(controlPrefix), "control route must remain installed in engine Routing Table")

	h.setIGP(subjectHost, 12)
	recovered := h.awaitEvent(t, 5*time.Second, func(ev forkedSelectedEvent) bool {
		return ev.prefix == subjectPrefix && ev.kind == rpc.SelectedRouteReceived
	}, "subject route recovered without a new UPDATE")
	require.True(t, recovered.stableRevision)
	require.True(t, recovered.destResolved, "recovered subject destination must be forwardable at stable revision")
	require.True(t, recovered.nhResolved, "recovered subject emitted NEXT_HOP must be forwardable at stable revision")
	require.Equal(t, subjectNH, recovered.emittedNextHop)
}

// TestRFC4271ForkedProspectiveResolutionExcludesUnforwardableRoutes proves the
// forked prospective-resolution RPC excludes both ordinary unresolved next hops
// and self-capturing more-specific prefixes whose NEXT_HOP is currently covered
// by a less-specific IGP route but would become unresolvable if installed,
// while an eligible control route still publishes and passes the stable-revision
// forwardability oracle.
//
// RFC requirement: RFC4271-9.2-2 negative -- across the forked
// RouteMetricsProspective RPC boundary, neither an unreachable-NEXT_HOP route
// nor a prospectively self-capturing route is installed or published to
// Adj-RIB-Out, while a forwardable control route is published normally.
func TestRFC4271ForkedProspectiveResolutionExcludesUnforwardableRoutes(t *testing.T) {
	h := newForkedForwardabilityHarness(t)
	controlNH := netip.MustParseAddr("198.51.100.10")
	unreachableNH := netip.MustParseAddr("198.51.100.99")
	selfCaptureNH := netip.MustParseAddr("198.51.101.20")

	controlPrefix := netip.MustParsePrefix("203.0.113.0/24")
	unreachablePrefix := netip.MustParsePrefix("198.18.30.0/24")
	selfCapturePrefix := netip.MustParsePrefix("198.51.101.0/25")

	h.setIGP(netip.PrefixFrom(controlNH, 32), 5)
	h.setIGP(netip.MustParsePrefix("198.51.101.0/24"), 10)

	h.mu.Lock()
	require.True(t, igpcost.Resolve(h.engineRIB, selfCaptureNH).Resolved, "less-specific IGP /24 must currently resolve the self-capturing next hop")
	require.False(t, igpcost.ResolveProspective(h.engineRIB, selfCaptureNH, family.IPv4Unicast, selfCapturePrefix, locrib.Path{
		Source: bgpProtocolID, Instance: bgpLocRIBInstance, NextHop: selfCaptureNH, IsBGP: true, IsEBGP: true,
	}).Resolved, "installing the /25 must make its own NEXT_HOP prospectively unresolvable")
	h.mu.Unlock()

	h.receiveUpdate(t, 1, "192.0.2.1", unreachablePrefix, unreachableNH)
	h.receiveUpdate(t, 2, "192.0.2.2", selfCapturePrefix, selfCaptureNH)
	h.receiveUpdate(t, 3, "192.0.2.3", controlPrefix, controlNH)

	controlEv := h.awaitEvent(t, 4*time.Second, func(ev forkedSelectedEvent) bool {
		return ev.prefix == controlPrefix && ev.kind == rpc.SelectedRouteReceived
	}, "control route published alongside excluded unforwardable candidates")
	require.True(t, controlEv.stableRevision)
	require.True(t, controlEv.destResolved)
	require.True(t, controlEv.nhResolved)
	require.Equal(t, controlNH, controlEv.emittedNextHop)

	require.NoError(t, h.r.selectedDispatch.barrier(h.ctx, h.destPeer))
	require.GreaterOrEqual(t, h.prospectiveCalls.Load(), int64(3), "each candidate must cross remoteRIB.prospectiveDistance")
	require.False(t, h.engineHasPrefix(unreachablePrefix), "unreachable route must not enter engine Routing Table")
	require.False(t, h.engineHasPrefix(selfCapturePrefix), "prospectively self-capturing route must not enter engine Routing Table")
	for _, ev := range h.drainEvents() {
		require.NotEqual(t, unreachablePrefix, ev.prefix, "unreachable route must never be published")
		require.NotEqual(t, selfCapturePrefix, ev.prefix, "prospectively self-capturing route must never be published")
	}
}

// TestRFC4271ForkedRevisionInvalidationWithdrawsOnReachabilityLoss proves that
// once remoteRIB.prospective caches Resolved=true for a forwardable route, a
// Routing Table revision change detected by runRemoteMetrics invalidates that
// prospective cache and withdraws the newly unforwardable route from both the
// engine Routing Table and Adj-RIB-Out while keeping the reachable control.
//
// RFC requirement: RFC4271-9.2-2 negative -- when a Routing Table revision
// removes next-hop reachability for a previously cached forwardable route, the
// forked metric worker invalidates its prospective cache and withdraws the
// route rather than serving stale forwardability.
func TestRFC4271ForkedRevisionInvalidationWithdrawsOnReachabilityLoss(t *testing.T) {
	h := newForkedForwardabilityHarness(t)
	controlNH := netip.MustParseAddr("198.51.100.10")
	subjectNH := netip.MustParseAddr("198.51.100.30")
	controlPrefix := netip.MustParsePrefix("203.0.113.0/24")
	subjectPrefix := netip.MustParsePrefix("198.18.40.0/24")
	subjectHost := netip.PrefixFrom(subjectNH, 32)

	h.setIGP(netip.PrefixFrom(controlNH, 32), 5)
	h.setIGP(subjectHost, 8)
	h.receiveUpdate(t, 1, "192.0.2.1", controlPrefix, controlNH)
	h.receiveUpdate(t, 2, "192.0.2.2", subjectPrefix, subjectNH)

	seen := map[netip.Prefix]bool{}
	for len(seen) < 2 {
		ev := h.awaitEvent(t, 4*time.Second, func(ev forkedSelectedEvent) bool {
			return ev.kind == rpc.SelectedRouteReceived
		}, "initial control and subject publications")
		require.True(t, ev.stableRevision)
		require.True(t, ev.destResolved)
		require.True(t, ev.nhResolved)
		seen[ev.prefix] = true
	}
	require.True(t, seen[controlPrefix] && seen[subjectPrefix])

	h.r.forkRIB.mu.Lock()
	cachedCount := len(h.r.forkRIB.prospective)
	h.r.forkRIB.mu.Unlock()
	require.GreaterOrEqual(t, cachedCount, 2, "prospective resolution cache must hold the admitted routes before invalidation")

	callsBefore := h.prospectiveCalls.Load()
	h.removeIGP(subjectHost)
	h.awaitEvent(t, 4*time.Second, func(ev forkedSelectedEvent) bool {
		return ev.prefix == subjectPrefix && ev.kind == rpc.SelectedRouteAbsent
	}, "cached subject route withdrawn after Routing Table revision change")

	require.Greater(t, h.prospectiveCalls.Load(), callsBefore, "revision invalidation must force a fresh prospective RPC instead of reusing stale cache")
	require.False(t, h.engineHasPrefix(subjectPrefix), "unforwardable subject must be removed from engine Routing Table")
	require.True(t, h.engineHasPrefix(controlPrefix), "reachable control must remain in engine Routing Table")
	for _, ev := range h.drainEvents() {
		require.False(t, ev.prefix == controlPrefix && ev.kind == rpc.SelectedRouteAbsent, "reachable control must not be withdrawn")
	}
}
