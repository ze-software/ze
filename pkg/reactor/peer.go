package reactor

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"net/netip"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/exa-networks/zebgp/pkg/bgp/attribute"
	"github.com/exa-networks/zebgp/pkg/bgp/capability"
	"github.com/exa-networks/zebgp/pkg/bgp/fsm"
	"github.com/exa-networks/zebgp/pkg/bgp/message"
	"github.com/exa-networks/zebgp/pkg/bgp/nlri"
	"github.com/exa-networks/zebgp/pkg/rib"
	"github.com/exa-networks/zebgp/pkg/trace"
)

// PeerState represents the high-level state of a peer.
type PeerState int32

const (
	// PeerStateStopped means the peer is not running.
	PeerStateStopped PeerState = iota
	// PeerStateConnecting means the peer is attempting to connect.
	PeerStateConnecting
	// PeerStateActive means the peer is waiting for incoming connection.
	PeerStateActive
	// PeerStateEstablished means the BGP session is established.
	PeerStateEstablished
)

func (s PeerState) String() string {
	switch s {
	case PeerStateStopped:
		return "Stopped"
	case PeerStateConnecting:
		return "Connecting"
	case PeerStateActive:
		return "Active"
	case PeerStateEstablished:
		return "Established"
	default:
		return "Unknown"
	}
}

// Default reconnect delays.
const (
	DefaultReconnectMin = 5 * time.Second
	DefaultReconnectMax = 60 * time.Second
)

// PeerCallback is called when peer state changes.
type PeerCallback func(from, to PeerState)

// Peer wraps a Session with reconnection logic.
//
// It manages the connection lifecycle in its own goroutine,
// automatically reconnecting on failure with exponential backoff.
type Peer struct {
	settings *PeerSettings
	session  *Session

	state    atomic.Int32
	callback PeerCallback

	// Reconnect configuration
	reconnectMin time.Duration
	reconnectMax time.Duration

	// Adj-RIB-Out: routes pending announcement to this peer
	adjRIBOut *rib.OutgoingRIB

	// Goroutine control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu sync.RWMutex
}

// NewPeer creates a new peer for the given settings.
func NewPeer(settings *PeerSettings) *Peer {
	return &Peer{
		settings:     settings,
		reconnectMin: DefaultReconnectMin,
		reconnectMax: DefaultReconnectMax,
		adjRIBOut:    rib.NewOutgoingRIB(),
	}
}

// Settings returns the configured peer settings.
func (p *Peer) Settings() *PeerSettings {
	return p.settings
}

// AdjRIBOut returns the peer's Adj-RIB-Out.
func (p *Peer) AdjRIBOut() *rib.OutgoingRIB {
	return p.adjRIBOut
}

// State returns the current peer state.
func (p *Peer) State() PeerState {
	return PeerState(p.state.Load())
}

// setState updates state and calls callback.
func (p *Peer) setState(s PeerState) {
	old := PeerState(p.state.Swap(int32(s)))
	if old != s {
		p.mu.RLock()
		cb := p.callback
		p.mu.RUnlock()
		if cb != nil {
			cb(old, s)
		}
	}
}

// SetCallback sets the state change callback.
func (p *Peer) SetCallback(cb PeerCallback) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callback = cb
}

// SetReconnectDelay configures reconnection delays.
func (p *Peer) SetReconnectDelay(min, max time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.reconnectMin = min
	p.reconnectMax = max
}

// Start begins the peer goroutine with a background context.
func (p *Peer) Start() {
	p.StartWithContext(context.Background())
}

// StartWithContext begins the peer goroutine with the given context.
func (p *Peer) StartWithContext(ctx context.Context) {
	p.mu.Lock()
	if p.cancel != nil {
		p.mu.Unlock()
		return // Already running
	}
	p.ctx, p.cancel = context.WithCancel(ctx)
	p.mu.Unlock()

	p.wg.Add(1)
	go p.run()
}

// Stop signals the peer to stop.
func (p *Peer) Stop() {
	p.mu.Lock()
	cancel := p.cancel
	p.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

// Wait waits for the peer to stop.
func (p *Peer) Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// run is the main peer loop.
func (p *Peer) run() {
	defer p.wg.Done()
	defer p.cleanup()

	delay := p.reconnectMin

	for {
		select {
		case <-p.ctx.Done():
			return
		default:
		}

		// Attempt connection
		err := p.runOnce()

		select {
		case <-p.ctx.Done():
			return
		default:
		}

		if err != nil {
			// Backoff before retry
			p.setState(PeerStateConnecting)

			select {
			case <-p.ctx.Done():
				return
			case <-time.After(delay):
			}

			// Exponential backoff
			delay *= 2
			p.mu.RLock()
			maxDelay := p.reconnectMax
			p.mu.RUnlock()
			if delay > maxDelay {
				delay = maxDelay
			}
		} else {
			// Reset delay on successful session
			delay = p.reconnectMin
		}
	}
}

// runOnce attempts a single connection cycle.
func (p *Peer) runOnce() error {
	// Create session
	session := NewSession(p.settings)

	p.mu.Lock()
	p.session = session
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		p.session = nil
		p.mu.Unlock()
	}()

	// Update state based on FSM mode
	if p.settings.Passive {
		p.setState(PeerStateActive)
	} else {
		p.setState(PeerStateConnecting)
	}

	// Start FSM
	if err := session.Start(); err != nil {
		return err
	}

	// Connect (for active mode)
	if !p.settings.Passive {
		if err := session.Connect(p.ctx); err != nil {
			return err
		}
	}

	// Monitor FSM state
	session.fsm.SetCallback(func(from, to fsm.State) {
		addr := p.settings.Address.String()
		trace.FSMTransition(addr, from.String(), to.String())

		if to == fsm.StateEstablished {
			p.setState(PeerStateEstablished)
			trace.SessionEstablished(addr, p.settings.LocalAS, p.settings.PeerAS)
			// Send static routes from config.
			go p.sendInitialRoutes()
		} else if from == fsm.StateEstablished {
			p.setState(PeerStateConnecting)
			trace.SessionClosed(addr, "FSM left Established state")
		}
	})

	// Run session loop
	return session.Run(p.ctx)
}

// AcceptConnection accepts an incoming TCP connection for this peer.
// Used by the reactor to hand incoming connections to passive peers.
func (p *Peer) AcceptConnection(conn net.Conn) error {
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	if session == nil {
		return ErrNotConnected
	}

	return session.Accept(conn)
}

// SendUpdate sends a BGP UPDATE message to this peer.
// Returns ErrNotConnected if no session is active.
// Returns an error if the session is not in ESTABLISHED state.
func (p *Peer) SendUpdate(update *message.Update) error {
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	if session == nil {
		return ErrNotConnected
	}

	return session.SendUpdate(update)
}

// messageNegotiated returns message.Negotiated for use with CommitService.
// Returns nil if session is not established.
func (p *Peer) messageNegotiated() *message.Negotiated {
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	if session == nil {
		return nil
	}

	neg := session.Negotiated()
	if neg == nil {
		return nil
	}

	return &message.Negotiated{
		ASN4:    neg.ASN4,
		LocalAS: neg.LocalASN,
		PeerAS:  neg.PeerASN,
	}
}

// cleanup runs when peer stops.
func (p *Peer) cleanup() {
	p.mu.Lock()
	if p.session != nil {
		_ = p.session.Close()
		p.session = nil
	}
	p.cancel = nil
	p.mu.Unlock()

	p.setState(PeerStateStopped)
}

// sendInitialRoutes sends static routes configured for this peer.
// Routes with identical attributes are grouped into a single UPDATE message.
func (p *Peer) sendInitialRoutes() {
	addr := p.settings.Address.String()
	trace.Log(trace.Routes, "peer %s: sending %d static routes", addr, len(p.settings.StaticRoutes))

	// Get negotiated capabilities for AS_PATH encoding.
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	asn4 := true // Default to 4-byte if no session (shouldn't happen)
	if session != nil {
		if neg := session.Negotiated(); neg != nil {
			asn4 = neg.ASN4
		}
	}

	// Determine which address families are configured from capabilities.
	hasIPv4Family := false
	hasIPv6Family := false
	for _, cap := range p.settings.Capabilities {
		if mp, ok := cap.(*capability.Multiprotocol); ok {
			if mp.AFI == 1 && mp.SAFI == 1 { // IPv4 unicast
				hasIPv4Family = true
			}
			if mp.AFI == 2 && mp.SAFI == 1 { // IPv6 unicast
				hasIPv6Family = true
			}
		}
	}

	// Send routes - either grouped or individually based on config.
	if p.settings.GroupUpdates {
		// Group routes by attributes (same attributes = same UPDATE).
		groups := groupRoutesByAttributes(p.settings.StaticRoutes)

		for _, routes := range groups {
			update := buildGroupedUpdate(routes, p.settings.LocalAS, p.settings.IsIBGP(), asn4)
			if err := p.SendUpdate(update); err != nil {
				trace.Log(trace.Routes, "peer %s: send error: %v", addr, err)
				break
			}
			for _, route := range routes {
				trace.RouteSent(addr, route.Prefix.String(), route.NextHop.String())
			}
		}
	} else {
		// Send each route in its own UPDATE.
		for _, route := range p.settings.StaticRoutes {
			update := buildStaticRouteUpdate(route, p.settings.LocalAS, p.settings.IsIBGP(), asn4)
			if err := p.SendUpdate(update); err != nil {
				trace.Log(trace.Routes, "peer %s: send error: %v", addr, err)
				break
			}
			trace.RouteSent(addr, route.Prefix.String(), route.NextHop.String())
		}
	}

	// Send End-of-RIB marker for each configured address family.
	if hasIPv4Family {
		eor := message.BuildEOR(nlri.IPv4Unicast)
		_ = p.SendUpdate(eor)
		trace.Log(trace.Routes, "peer %s: sent IPv4 unicast EOR marker", addr)
	}
	if hasIPv6Family {
		eor := message.BuildEOR(nlri.IPv6Unicast)
		_ = p.SendUpdate(eor)
		trace.Log(trace.Routes, "peer %s: sent IPv6 unicast EOR marker", addr)
	}

	// Send MVPN routes
	p.sendMVPNRoutes()

	// Send VPLS routes
	p.sendVPLSRoutes()

	// Send FlowSpec routes
	p.sendFlowSpecRoutes()

	// Send MUP routes
	p.sendMUPRoutes()
}

// buildStaticRouteUpdate builds an UPDATE message for a static route.
// asn4 indicates whether to use 4-byte AS numbers in AS_PATH.
func buildStaticRouteUpdate(route StaticRoute, localAS uint32, isIBGP, asn4 bool) *message.Update {
	var attrBytes []byte

	// 1. ORIGIN (IGP by default, use configured value if set)
	origin := attribute.Origin(route.Origin)
	attrBytes = append(attrBytes, attribute.PackAttribute(origin)...)

	// 2. AS_PATH
	// - For iBGP: use configured AS_PATH or empty
	// - For eBGP: prepend local AS to configured AS_PATH
	var asPath *attribute.ASPath
	switch {
	case len(route.ASPath) > 0:
		// Use configured AS_PATH, prepend local AS for eBGP
		asns := make([]uint32, 0, len(route.ASPath)+1)
		if !isIBGP {
			asns = append(asns, localAS)
		}
		asns = append(asns, route.ASPath...)
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: asns},
			},
		}
	case isIBGP:
		// Empty AS_PATH for iBGP self-originated routes
		asPath = &attribute.ASPath{Segments: nil}
	default:
		// Prepend local AS for eBGP
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: []uint32{localAS}},
			},
		}
	}
	attrBytes = append(attrBytes, attribute.PackASPathAttribute(asPath, asn4)...)

	// 3. NEXT_HOP (always include for IPv4 routes, even VPN - for compatibility)
	if route.NextHop.Is4() {
		nextHop := &attribute.NextHop{Addr: route.NextHop}
		attrBytes = append(attrBytes, attribute.PackAttribute(nextHop)...)
	}

	// 4. MED (if set) - must come before LOCAL_PREF per RFC 4271 attribute ordering
	if route.MED > 0 {
		med := attribute.MED(route.MED)
		attrBytes = append(attrBytes, attribute.PackAttribute(med)...)
	}

	// 5. LOCAL_PREF (for iBGP, default 100 if not set)
	if isIBGP {
		localPref := route.LocalPreference
		if localPref == 0 {
			localPref = 100 // Default LOCAL_PREF
		}
		attrBytes = append(attrBytes, attribute.PackAttribute(attribute.LocalPref(localPref))...)
	}

	// 6. ATOMIC_AGGREGATE (if set)
	if route.AtomicAggregate {
		attrBytes = append(attrBytes, attribute.PackAttribute(attribute.AtomicAggregate{})...)
	}

	// 7. AGGREGATOR (if set)
	if route.HasAggregator {
		agg := &attribute.Aggregator{
			ASN:     route.AggregatorASN,
			Address: netip.AddrFrom4(route.AggregatorIP),
		}
		attrBytes = append(attrBytes, attribute.PackAttribute(agg)...)
	}

	// 8. COMMUNITIES (RFC 1997) - sorted per RFC.
	if len(route.Communities) > 0 {
		sorted := make([]uint32, len(route.Communities))
		copy(sorted, route.Communities)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

		comms := make(attribute.Communities, len(sorted))
		for i, c := range sorted {
			comms[i] = attribute.Community(c)
		}
		attrBytes = append(attrBytes, attribute.PackAttribute(comms)...)
	}

	// 7. EXTENDED_COMMUNITY (if set)
	if len(route.ExtCommunityBytes) > 0 {
		// Pack as attribute: flags=0xC0 (optional, transitive), type=16, len, value
		ecAttr := make([]byte, 0, 3+len(route.ExtCommunityBytes))
		ecAttr = append(ecAttr, 0xC0, 0x10, byte(len(route.ExtCommunityBytes)))
		ecAttr = append(ecAttr, route.ExtCommunityBytes...)
		attrBytes = append(attrBytes, ecAttr...)
	}

	// 8. LARGE_COMMUNITIES (RFC 8092) - if set
	if len(route.LargeCommunities) > 0 {
		lcs := make(attribute.LargeCommunities, len(route.LargeCommunities))
		for i, lc := range route.LargeCommunities {
			lcs[i] = attribute.LargeCommunity{
				GlobalAdmin: lc[0],
				LocalData1:  lc[1],
				LocalData2:  lc[2],
			}
		}
		attrBytes = append(attrBytes, attribute.PackAttribute(lcs)...)
	}

	// 9. RAW ATTRIBUTES - pass through as-is from config
	for _, ra := range route.RawAttributes {
		attrBytes = append(attrBytes, packRawAttribute(ra)...)
	}

	// Build NLRI - use MP_REACH_NLRI for VPN/IPv6, inline NLRI for IPv4 unicast
	var nlriBytes []byte
	switch {
	case route.IsVPN():
		// VPN route: use MP_REACH_NLRI attribute (returns raw bytes)
		attrBytes = append(attrBytes, buildMPReachNLRI(route)...)
	case route.Prefix.Addr().Is4():
		// IPv4 unicast: inline NLRI with optional path-id
		inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, route.Prefix, route.PathID)
		nlriBytes = inet.Bytes()
	default:
		// IPv6 unicast: use MP_REACH_NLRI attribute (RFC 4760)
		attrBytes = append(attrBytes, buildMPReachNLRIUnicast(route)...)
	}

	return &message.Update{
		PathAttributes: attrBytes,
		NLRI:           nlriBytes,
	}
}

// packRawAttribute packs a raw attribute into wire format.
// Format: flags (1 byte) + code (1 byte) + length (1 or 2 bytes) + value.
func packRawAttribute(ra RawAttribute) []byte {
	flags := ra.Flags
	valueLen := len(ra.Value)

	// Use extended length if value > 255 bytes OR if extended length flag is set
	if valueLen > 255 || (flags&0x10) != 0 {
		flags |= 0x10 // Ensure extended length flag is set
		buf := make([]byte, 4+valueLen)
		buf[0] = flags
		buf[1] = ra.Code
		buf[2] = byte(valueLen >> 8)
		buf[3] = byte(valueLen)
		copy(buf[4:], ra.Value)
		return buf
	}

	buf := make([]byte, 3+valueLen)
	buf[0] = flags
	buf[1] = ra.Code
	buf[2] = byte(valueLen)
	copy(buf[3:], ra.Value)
	return buf
}

// routeGroupKey generates a string key for grouping routes by attributes.
// Routes with the same key can be combined into a single UPDATE.
func routeGroupKey(r StaticRoute) string {
	// Sort communities for consistent key.
	comms := make([]uint32, len(r.Communities))
	copy(comms, r.Communities)
	sort.Slice(comms, func(i, j int) bool { return comms[i] < comms[j] })

	// Sort large communities.
	lcs := make([][3]uint32, len(r.LargeCommunities))
	copy(lcs, r.LargeCommunities)
	sort.Slice(lcs, func(i, j int) bool {
		if lcs[i][0] != lcs[j][0] {
			return lcs[i][0] < lcs[j][0]
		}
		if lcs[i][1] != lcs[j][1] {
			return lcs[i][1] < lcs[j][1]
		}
		return lcs[i][2] < lcs[j][2]
	})

	// Key includes: nexthop, origin, localpref, med, communities, large-communities, ext-communities, vpn, ipv4/ipv6,
	// as-path, atomic-aggregate, aggregator.
	// For IPv6 routes, include prefix in key to prevent grouping (each needs separate MP_REACH_NLRI UPDATE).
	// IPv4 routes can be grouped since multiple NLRIs can be in one UPDATE.
	prefixKey := ""
	if !r.Prefix.Addr().Is4() {
		prefixKey = r.Prefix.String()
	}
	return fmt.Sprintf("%s|%d|%d|%d|%v|%v|%s|%s|%v|%s|%v|%v|%d|%v",
		r.NextHop.String(),
		r.Origin,
		r.LocalPreference,
		r.MED,
		comms,
		lcs,
		hex.EncodeToString(r.ExtCommunityBytes),
		r.RD,
		r.Prefix.Addr().Is4(),
		prefixKey,
		r.ASPath,
		r.AtomicAggregate,
		r.AggregatorASN,
		r.AggregatorIP,
	)
}

// groupRoutesByAttributes groups routes by their attribute key.
// Returns groups sorted: multi-route groups first (by first prefix), then singletons (by prefix).
// This matches ExaBGP's behavior for UPDATE grouping.
func groupRoutesByAttributes(routes []StaticRoute) [][]StaticRoute {
	groups := make(map[string][]StaticRoute)

	for _, r := range routes {
		key := routeGroupKey(r)
		groups[key] = append(groups[key], r)
	}

	// Collect groups into slice.
	result := make([][]StaticRoute, 0, len(groups))
	for _, g := range groups {
		// Sort routes within group by prefix.
		sort.Slice(g, func(i, j int) bool {
			return g[i].Prefix.Addr().Compare(g[j].Prefix.Addr()) < 0
		})
		result = append(result, g)
	}

	// Sort groups: multi-route first, then singletons, each ordered by first prefix.
	sort.Slice(result, func(i, j int) bool {
		// Multi-route groups come before singletons.
		if len(result[i]) > 1 && len(result[j]) == 1 {
			return true
		}
		if len(result[i]) == 1 && len(result[j]) > 1 {
			return false
		}
		// Same category: sort by first prefix.
		return result[i][0].Prefix.Addr().Compare(result[j][0].Prefix.Addr()) < 0
	})

	return result
}

// buildGroupedUpdate builds an UPDATE message for multiple routes with same attributes.
// asn4 indicates whether to use 4-byte AS numbers in AS_PATH.
func buildGroupedUpdate(routes []StaticRoute, localAS uint32, isIBGP, asn4 bool) *message.Update {
	if len(routes) == 0 {
		return &message.Update{}
	}

	// Use first route as representative for attributes.
	route := routes[0]
	var attrBytes []byte

	// 1. ORIGIN.
	origin := attribute.Origin(route.Origin)
	attrBytes = append(attrBytes, attribute.PackAttribute(origin)...)

	// 2. AS_PATH.
	var asPath *attribute.ASPath
	switch {
	case len(route.ASPath) > 0:
		asns := make([]uint32, 0, len(route.ASPath)+1)
		if !isIBGP {
			asns = append(asns, localAS)
		}
		asns = append(asns, route.ASPath...)
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: asns},
			},
		}
	case isIBGP:
		asPath = &attribute.ASPath{Segments: nil}
	default:
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: []uint32{localAS}},
			},
		}
	}
	attrBytes = append(attrBytes, attribute.PackASPathAttribute(asPath, asn4)...)

	// 3. NEXT_HOP (for IPv4 routes).
	if route.NextHop.Is4() {
		nextHop := &attribute.NextHop{Addr: route.NextHop}
		attrBytes = append(attrBytes, attribute.PackAttribute(nextHop)...)
	}

	// 4. LOCAL_PREF (for iBGP).
	if isIBGP {
		localPref := route.LocalPreference
		if localPref == 0 {
			localPref = 100
		}
		attrBytes = append(attrBytes, attribute.PackAttribute(attribute.LocalPref(localPref))...)
	}

	// 5. ATOMIC_AGGREGATE (if set).
	if route.AtomicAggregate {
		attrBytes = append(attrBytes, attribute.PackAttribute(attribute.AtomicAggregate{})...)
	}

	// 6. AGGREGATOR (if set).
	if route.HasAggregator {
		agg := &attribute.Aggregator{
			ASN:     route.AggregatorASN,
			Address: netip.AddrFrom4(route.AggregatorIP),
		}
		attrBytes = append(attrBytes, attribute.PackAttribute(agg)...)
	}

	// 7. MED (if set).
	if route.MED > 0 {
		med := attribute.MED(route.MED)
		attrBytes = append(attrBytes, attribute.PackAttribute(med)...)
	}

	// 6. COMMUNITIES (RFC 1997) - sorted per RFC 1997.
	if len(route.Communities) > 0 {
		// Copy and sort communities.
		sorted := make([]uint32, len(route.Communities))
		copy(sorted, route.Communities)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

		comms := make(attribute.Communities, len(sorted))
		for i, c := range sorted {
			comms[i] = attribute.Community(c)
		}
		attrBytes = append(attrBytes, attribute.PackAttribute(comms)...)
	}

	// 7. EXTENDED_COMMUNITY.
	if len(route.ExtCommunityBytes) > 0 {
		ecAttr := make([]byte, 0, 3+len(route.ExtCommunityBytes))
		ecAttr = append(ecAttr, 0xC0, 0x10, byte(len(route.ExtCommunityBytes)))
		ecAttr = append(ecAttr, route.ExtCommunityBytes...)
		attrBytes = append(attrBytes, ecAttr...)
	}

	// 8. LARGE_COMMUNITIES (RFC 8092).
	if len(route.LargeCommunities) > 0 {
		lcs := make(attribute.LargeCommunities, len(route.LargeCommunities))
		for i, lc := range route.LargeCommunities {
			lcs[i] = attribute.LargeCommunity{
				GlobalAdmin: lc[0],
				LocalData1:  lc[1],
				LocalData2:  lc[2],
			}
		}
		attrBytes = append(attrBytes, attribute.PackAttribute(lcs)...)
	}

	// Build NLRI for all routes in group.
	var nlriBytes []byte
	for _, r := range routes {
		switch {
		case r.IsVPN():
			// VPN routes need separate handling - for now, just use first.
			attrBytes = append(attrBytes, buildMPReachNLRI(r)...)
		case r.Prefix.Addr().Is4():
			// IPv4 unicast: append to NLRI.
			inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, r.Prefix, r.PathID)
			nlriBytes = append(nlriBytes, inet.Bytes()...)
		default:
			// IPv6 unicast: use MP_REACH_NLRI (handle separately).
			attrBytes = append(attrBytes, buildMPReachNLRIUnicast(r)...)
		}
	}

	return &message.Update{
		PathAttributes: attrBytes,
		NLRI:           nlriBytes,
	}
}

// buildMPReachNLRI builds MP_REACH_NLRI for VPN routes.
// Returns raw attribute bytes (flags + type + length + value).
func buildMPReachNLRI(route StaticRoute) []byte {
	// Determine AFI/SAFI
	afi := uint16(1) // IPv4
	if route.Prefix.Addr().Is6() {
		afi = 2 // IPv6
	}
	safi := byte(128) // MPLS VPN

	// Build next-hop: RD (all zeros) + IP address
	var nhBytes []byte
	if route.NextHop.Is4() {
		nhBytes = make([]byte, 12) // 8-byte RD + 4-byte IPv4
		copy(nhBytes[8:], route.NextHop.AsSlice())
	} else {
		nhBytes = make([]byte, 24) // 8-byte RD + 16-byte IPv6
		copy(nhBytes[8:], route.NextHop.AsSlice())
	}

	// Build VPN NLRI
	vpnNLRI := buildVPNNLRIBytes(route)

	// MP_REACH_NLRI value: AFI(2) + SAFI(1) + NH_Len(1) + NextHop + Reserved(1) + NLRI
	valueLen := 2 + 1 + 1 + len(nhBytes) + 1 + len(vpnNLRI)
	value := make([]byte, valueLen)
	value[0] = byte(afi >> 8)
	value[1] = byte(afi)
	value[2] = safi
	value[3] = byte(len(nhBytes))
	copy(value[4:], nhBytes)
	value[4+len(nhBytes)] = 0 // Reserved
	copy(value[4+len(nhBytes)+1:], vpnNLRI)

	// Build attribute header: flags + type + length + value
	// Flags: 0x80 = optional, 0x90 = optional + extended length
	var attr []byte
	if valueLen > 255 {
		attr = make([]byte, 4+valueLen)
		attr[0] = 0x90 // optional + extended length
		attr[1] = 14   // MP_REACH_NLRI
		attr[2] = byte(valueLen >> 8)
		attr[3] = byte(valueLen)
		copy(attr[4:], value)
	} else {
		attr = make([]byte, 3+valueLen)
		attr[0] = 0x80 // optional
		attr[1] = 14   // MP_REACH_NLRI
		attr[2] = byte(valueLen)
		copy(attr[3:], value)
	}

	return attr
}

// buildVPNNLRIBytes builds the raw VPN NLRI bytes (for MP_REACH_NLRI).
func buildVPNNLRIBytes(route StaticRoute) []byte {
	// Label encoding: 20-bit label, 3-bit TC, 1-bit BOS (bottom of stack)
	// Label is in upper 20 bits: label << 4, BOS in bit 0
	label := route.Label
	labelBytes := []byte{
		byte(label >> 12),
		byte(label >> 4),
		byte(label<<4) | 0x01, // BOS = 1
	}

	// Prefix bytes
	prefixBits := route.Prefix.Bits()
	prefixBytes := (prefixBits + 7) / 8
	prefixData := route.Prefix.Addr().AsSlice()[:prefixBytes]

	// Total bits: 24 (label) + 64 (RD) + prefix bits
	totalBits := 24 + 64 + prefixBits

	// Build: [path-id] + length + label + RD + prefix
	var buf []byte
	if route.PathID != 0 {
		buf = make([]byte, 4+1+3+8+prefixBytes)
		buf[0] = byte(route.PathID >> 24)
		buf[1] = byte(route.PathID >> 16)
		buf[2] = byte(route.PathID >> 8)
		buf[3] = byte(route.PathID)
		buf[4] = byte(totalBits)
		copy(buf[5:8], labelBytes)
		copy(buf[8:16], route.RDBytes[:])
		copy(buf[16:], prefixData)
	} else {
		buf = make([]byte, 1+3+8+prefixBytes)
		buf[0] = byte(totalBits)
		copy(buf[1:4], labelBytes)
		copy(buf[4:12], route.RDBytes[:])
		copy(buf[12:], prefixData)
	}

	return buf
}

// buildMPReachNLRIUnicast builds MP_REACH_NLRI for IPv6 unicast routes.
// Returns raw attribute bytes (flags + type + length + value).
func buildMPReachNLRIUnicast(route StaticRoute) []byte {
	// Build NLRI bytes for the prefix
	prefixBits := route.Prefix.Bits()
	prefixBytes := (prefixBits + 7) / 8
	nlriData := make([]byte, 1+prefixBytes)
	nlriData[0] = byte(prefixBits)
	copy(nlriData[1:], route.Prefix.Addr().AsSlice()[:prefixBytes])

	// Determine AFI based on address family
	var afi attribute.AFI
	var nhLen int
	if route.Prefix.Addr().Is6() {
		afi = attribute.AFIIPv6
		nhLen = 16
	} else {
		afi = attribute.AFIIPv4
		nhLen = 4
	}

	mpReach := &attribute.MPReachNLRI{
		AFI:      afi,
		SAFI:     attribute.SAFIUnicast,
		NextHops: []netip.Addr{route.NextHop},
		NLRI:     nlriData,
	}

	// Pack the attribute with header
	value := mpReach.Pack()
	valueLen := len(value)

	// Build attribute: flags + type + length + value
	// Flags: 0x80 = optional, 0x90 = optional + extended length
	var attr []byte
	if valueLen > 255 {
		attr = make([]byte, 4+valueLen)
		attr[0] = 0x90 // optional + extended length
		attr[1] = byte(attribute.AttrMPReachNLRI)
		attr[2] = byte(valueLen >> 8)
		attr[3] = byte(valueLen)
		copy(attr[4:], value)
	} else {
		attr = make([]byte, 3+valueLen)
		attr[0] = 0x80 // optional
		attr[1] = byte(attribute.AttrMPReachNLRI)
		attr[2] = byte(valueLen)
		copy(attr[3:], value)
	}

	// Silence unused variable
	_ = nhLen

	return attr
}

// sendMVPNRoutes sends MVPN routes configured for this peer.
func (p *Peer) sendMVPNRoutes() {
	if len(p.settings.MVPNRoutes) == 0 {
		return
	}

	addr := p.settings.Address.String()
	trace.Log(trace.Routes, "peer %s: sending %d MVPN routes", addr, len(p.settings.MVPNRoutes))

	// Get negotiated capabilities for AS_PATH encoding.
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	asn4 := true // Default to 4-byte if no session
	if session != nil {
		if neg := session.Negotiated(); neg != nil {
			asn4 = neg.ASN4
		}
	}

	// Group MVPN routes by AFI and attributes for efficient UPDATE grouping
	ipv4Routes := make([]MVPNRoute, 0)
	ipv6Routes := make([]MVPNRoute, 0)

	for _, route := range p.settings.MVPNRoutes {
		if route.IsIPv6 {
			ipv6Routes = append(ipv6Routes, route)
		} else {
			ipv4Routes = append(ipv4Routes, route)
		}
	}

	// Group IPv4 routes by next-hop (different next-hop = different UPDATE)
	ipv4Groups := groupMVPNRoutesByNextHop(ipv4Routes)
	for nh, routes := range ipv4Groups {
		update := buildMVPNUpdate(routes, p.settings.LocalAS, p.settings.IsIBGP(), false, asn4)
		if err := p.SendUpdate(update); err != nil {
			trace.Log(trace.Routes, "peer %s: MVPN send error: %v", addr, err)
		} else {
			trace.Log(trace.Routes, "peer %s: sent %d IPv4 MVPN routes (NH=%s)", addr, len(routes), nh)
		}
	}

	// Group IPv6 routes by next-hop
	ipv6Groups := groupMVPNRoutesByNextHop(ipv6Routes)
	for nh, routes := range ipv6Groups {
		update := buildMVPNUpdate(routes, p.settings.LocalAS, p.settings.IsIBGP(), true, asn4)
		if err := p.SendUpdate(update); err != nil {
			trace.Log(trace.Routes, "peer %s: MVPN send error: %v", addr, err)
		} else {
			trace.Log(trace.Routes, "peer %s: sent %d IPv6 MVPN routes (NH=%s)", addr, len(routes), nh)
		}
	}
}

// groupMVPNRoutesByNextHop groups MVPN routes by next-hop address.
func groupMVPNRoutesByNextHop(routes []MVPNRoute) map[string][]MVPNRoute {
	groups := make(map[string][]MVPNRoute)
	for _, route := range routes {
		key := route.NextHop.String()
		groups[key] = append(groups[key], route)
	}
	return groups
}

// buildMVPNUpdate builds an UPDATE message for MVPN routes.
func buildMVPNUpdate(routes []MVPNRoute, localAS uint32, isIBGP, isIPv6, asn4 bool) *message.Update {
	if len(routes) == 0 {
		return &message.Update{}
	}

	// Use first route for common attributes
	first := routes[0]

	var attrBytes []byte

	// 1. ORIGIN (default to IGP if not set; Origin 0 is IGP, which is correct default)
	origin := attribute.Origin(first.Origin)
	attrBytes = append(attrBytes, attribute.PackAttribute(origin)...)

	// 2. AS_PATH (empty for iBGP)
	var asPath *attribute.ASPath
	if isIBGP {
		asPath = &attribute.ASPath{Segments: nil}
	} else {
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: []uint32{localAS}},
			},
		}
	}
	attrBytes = append(attrBytes, attribute.PackASPathAttribute(asPath, asn4)...)

	// 3. NEXT_HOP (via MP_REACH_NLRI for MVPN)
	// Build as traditional NEXT_HOP for test compatibility
	if first.NextHop.Is4() {
		nextHop := &attribute.NextHop{Addr: first.NextHop}
		attrBytes = append(attrBytes, attribute.PackAttribute(nextHop)...)
	}

	// 4. LOCAL_PREF (for iBGP, default to 100 if not set)
	if isIBGP {
		lp := first.LocalPreference
		if lp == 0 {
			lp = 100 // Default LOCAL_PREF for iBGP
		}
		attrBytes = append(attrBytes, attribute.PackAttribute(attribute.LocalPref(lp))...)
	}

	// 5. Extended Communities (manually build attribute)
	if len(first.ExtCommunityBytes) > 0 {
		ecAttr := make([]byte, 0, 3+len(first.ExtCommunityBytes))
		ecAttr = append(ecAttr, 0xC0, 0x10, byte(len(first.ExtCommunityBytes)))
		ecAttr = append(ecAttr, first.ExtCommunityBytes...)
		attrBytes = append(attrBytes, ecAttr...)
	}

	// 6. MP_REACH_NLRI with MVPN NLRIs
	mpReach := buildMVPNMPReachNLRI(routes, isIPv6)
	attrBytes = append(attrBytes, mpReach...)

	return &message.Update{
		PathAttributes: attrBytes,
	}
}

// buildMVPNMPReachNLRI builds MP_REACH_NLRI for MVPN routes.
func buildMVPNMPReachNLRI(routes []MVPNRoute, isIPv6 bool) []byte {
	if len(routes) == 0 {
		return nil
	}

	first := routes[0]

	// Build NLRI data for all routes
	var nlriData []byte
	for _, route := range routes {
		nlri := buildMVPNNLRI(route)
		nlriData = append(nlriData, nlri...)
	}

	// AFI/SAFI
	var afi uint16 = 1 // IPv4
	if isIPv6 {
		afi = 2 // IPv6
	}
	const safiMVPN uint8 = 5

	// Next-hop
	nhBytes := first.NextHop.AsSlice()
	nhLen := len(nhBytes)

	// Build MP_REACH_NLRI value
	// AFI (2) + SAFI (1) + NH Len (1) + NH + Reserved (1) + NLRI
	valueLen := 2 + 1 + 1 + nhLen + 1 + len(nlriData)
	value := make([]byte, valueLen)
	value[0] = byte(afi >> 8)
	value[1] = byte(afi)
	value[2] = safiMVPN
	value[3] = byte(nhLen)
	copy(value[4:4+nhLen], nhBytes)
	value[4+nhLen] = 0 // reserved
	copy(value[5+nhLen:], nlriData)

	// Build attribute header
	var attr []byte
	if valueLen > 255 {
		attr = make([]byte, 4+valueLen)
		attr[0] = 0x90 // optional + extended length
		attr[1] = byte(attribute.AttrMPReachNLRI)
		attr[2] = byte(valueLen >> 8)
		attr[3] = byte(valueLen)
		copy(attr[4:], value)
	} else {
		attr = make([]byte, 3+valueLen)
		attr[0] = 0x80 // optional
		attr[1] = byte(attribute.AttrMPReachNLRI)
		attr[2] = byte(valueLen)
		copy(attr[3:], value)
	}

	return attr
}

// buildMVPNNLRI builds a single MVPN NLRI.
func buildMVPNNLRI(route MVPNRoute) []byte {
	// MVPN NLRI format:
	// Route Type (1) + Length (1) + Route Type Specific Data

	var data []byte

	switch route.RouteType {
	case 5: // Source Active A-D
		// RD (8) + Source (4/16) + Group (4/16)
		data = append(data, route.RD[:]...)
		if route.Source.Is4() {
			// Encoded as /32 prefix
			data = append(data, 32) // prefix len
			src4 := route.Source.As4()
			data = append(data, src4[:]...)
		} else {
			data = append(data, 128) // prefix len
			src16 := route.Source.As16()
			data = append(data, src16[:]...)
		}
		if route.Group.Is4() {
			data = append(data, 32) // prefix len
			grp4 := route.Group.As4()
			data = append(data, grp4[:]...)
		} else {
			data = append(data, 128) // prefix len
			grp16 := route.Group.As16()
			data = append(data, grp16[:]...)
		}

	case 6, 7: // Shared Tree Join (6) or Source Tree Join (7)
		// RD (8) + Source-AS (4) + Source/RP (4/16) + Group (4/16)
		data = append(data, route.RD[:]...)
		// Source-AS as 4 bytes
		data = append(data, byte(route.SourceAS>>24), byte(route.SourceAS>>16),
			byte(route.SourceAS>>8), byte(route.SourceAS))
		if route.Source.Is4() {
			data = append(data, 32) // prefix len
			src4 := route.Source.As4()
			data = append(data, src4[:]...)
		} else {
			data = append(data, 128) // prefix len
			src16 := route.Source.As16()
			data = append(data, src16[:]...)
		}
		if route.Group.Is4() {
			data = append(data, 32) // prefix len
			grp4 := route.Group.As4()
			data = append(data, grp4[:]...)
		} else {
			data = append(data, 128) // prefix len
			grp16 := route.Group.As16()
			data = append(data, grp16[:]...)
		}
	}

	// Build NLRI: Route Type (1) + Length (1) + Data
	nlri := make([]byte, 2+len(data))
	nlri[0] = route.RouteType
	nlri[1] = byte(len(data))
	copy(nlri[2:], data)

	return nlri
}

// sendVPLSRoutes sends VPLS routes configured for this peer.
func (p *Peer) sendVPLSRoutes() {
	if len(p.settings.VPLSRoutes) == 0 {
		return
	}

	addr := p.settings.Address.String()
	trace.Log(trace.Routes, "peer %s: sending %d VPLS routes", addr, len(p.settings.VPLSRoutes))

	// Get negotiated capabilities for AS_PATH encoding.
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	asn4 := true // Default to 4-byte if no session
	if session != nil {
		if neg := session.Negotiated(); neg != nil {
			asn4 = neg.ASN4
		}
	}

	for _, route := range p.settings.VPLSRoutes {
		update := buildVPLSUpdate(route, p.settings.LocalAS, p.settings.IsIBGP(), asn4)
		if err := p.SendUpdate(update); err != nil {
			trace.Log(trace.Routes, "peer %s: VPLS send error: %v", addr, err)
		}
	}

	// Send EOR for L2VPN VPLS (AFI=25, SAFI=65)
	eor := message.BuildEOR(nlri.Family{AFI: 25, SAFI: 65})
	_ = p.SendUpdate(eor)
}

// buildVPLSUpdate builds an UPDATE message for a VPLS route.
// asn4 indicates whether to use 4-byte AS numbers in AS_PATH.
func buildVPLSUpdate(route VPLSRoute, localAS uint32, isIBGP, asn4 bool) *message.Update {
	var attrBytes []byte

	// 1. ORIGIN
	origin := attribute.Origin(route.Origin)
	attrBytes = append(attrBytes, attribute.PackAttribute(origin)...)

	// 2. AS_PATH
	var asPath *attribute.ASPath
	switch {
	case isIBGP && len(route.ASPath) == 0:
		asPath = &attribute.ASPath{Segments: nil}
	case len(route.ASPath) > 0:
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: route.ASPath},
			},
		}
	default:
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: []uint32{localAS}},
			},
		}
	}
	attrBytes = append(attrBytes, attribute.PackASPathAttribute(asPath, asn4)...)

	// Note: NEXT_HOP is in MP_REACH_NLRI for VPLS, not as separate attribute

	// 3. MED (type 4, before LOCAL_PREF for RFC ordering)
	if route.MED > 0 {
		attrBytes = append(attrBytes, attribute.PackAttribute(attribute.MED(route.MED))...)
	}

	// 4. LOCAL_PREF (type 5)
	if isIBGP {
		lp := route.LocalPreference
		if lp == 0 {
			lp = 100
		}
		attrBytes = append(attrBytes, attribute.PackAttribute(attribute.LocalPref(lp))...)
	}

	// 6. COMMUNITIES
	if len(route.Communities) > 0 {
		sorted := make([]uint32, len(route.Communities))
		copy(sorted, route.Communities)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
		commAttr := make([]byte, 0, 3+len(sorted)*4)
		commAttr = append(commAttr, 0xC0, 0x08, byte(len(sorted)*4))
		for _, c := range sorted {
			commAttr = append(commAttr, byte(c>>24), byte(c>>16), byte(c>>8), byte(c))
		}
		attrBytes = append(attrBytes, commAttr...)
	}

	// 7. ORIGINATOR_ID (type 9)
	if route.OriginatorID != 0 {
		origAttr := []byte{0x80, 0x09, 0x04,
			byte(route.OriginatorID >> 24), byte(route.OriginatorID >> 16),
			byte(route.OriginatorID >> 8), byte(route.OriginatorID)}
		attrBytes = append(attrBytes, origAttr...)
	}

	// 8. CLUSTER_LIST (type 10)
	if len(route.ClusterList) > 0 {
		clAttr := make([]byte, 0, 3+len(route.ClusterList)*4)
		clAttr = append(clAttr, 0x80, 0x0A, byte(len(route.ClusterList)*4))
		for _, c := range route.ClusterList {
			clAttr = append(clAttr, byte(c>>24), byte(c>>16), byte(c>>8), byte(c))
		}
		attrBytes = append(attrBytes, clAttr...)
	}

	// 9. EXTENDED_COMMUNITY (type 16)
	if len(route.ExtCommunityBytes) > 0 {
		ecAttr := make([]byte, 0, 3+len(route.ExtCommunityBytes))
		ecAttr = append(ecAttr, 0xC0, 0x10, byte(len(route.ExtCommunityBytes)))
		ecAttr = append(ecAttr, route.ExtCommunityBytes...)
		attrBytes = append(attrBytes, ecAttr...)
	}

	// 10. MP_REACH_NLRI for VPLS (type 14)
	mpReach := buildVPLSMPReachNLRI(route)
	attrBytes = append(attrBytes, mpReach...)

	return &message.Update{
		PathAttributes: attrBytes,
	}
}

// buildVPLSMPReachNLRI builds MP_REACH_NLRI for a VPLS route.
func buildVPLSMPReachNLRI(route VPLSRoute) []byte {
	// Build VPLS NLRI
	var rd nlri.RouteDistinguisher
	copy(rd.Value[:], route.RD[2:8])
	rd.Type = nlri.RDType(uint16(route.RD[0])<<8 | uint16(route.RD[1]))

	vpls := nlri.NewVPLSFull(rd, route.Endpoint, route.Offset, route.Size, route.Base)
	nlriData := vpls.Bytes()

	// Next-hop
	nhBytes := route.NextHop.AsSlice()

	// Build MP_REACH_NLRI
	// AFI=25 (L2VPN), SAFI=65 (VPLS)
	valueLen := 2 + 1 + 1 + len(nhBytes) + 1 + len(nlriData)
	value := make([]byte, valueLen)
	value[0], value[1] = 0, 25 // AFI
	value[2] = 65              // SAFI
	value[3] = byte(len(nhBytes))
	copy(value[4:4+len(nhBytes)], nhBytes)
	value[4+len(nhBytes)] = 0 // Reserved
	copy(value[5+len(nhBytes):], nlriData)

	// Build attribute header - use extended length only if needed
	var attr []byte
	if valueLen > 255 {
		attr = make([]byte, 4+valueLen)
		attr[0] = 0x90                // Optional, Extended Length
		attr[1] = 14                  // MP_REACH_NLRI
		attr[2] = byte(valueLen >> 8) // Length high
		attr[3] = byte(valueLen)      // Length low
		copy(attr[4:], value)
	} else {
		attr = make([]byte, 3+valueLen)
		attr[0] = 0x80 // Optional
		attr[1] = 14   // MP_REACH_NLRI
		attr[2] = byte(valueLen)
		copy(attr[3:], value)
	}

	return attr
}

// sendFlowSpecRoutes sends FlowSpec routes configured for this peer.
func (p *Peer) sendFlowSpecRoutes() {
	if len(p.settings.FlowSpecRoutes) == 0 {
		return
	}

	addr := p.settings.Address.String()
	trace.Log(trace.Routes, "peer %s: sending %d FlowSpec routes", addr, len(p.settings.FlowSpecRoutes))

	// Get negotiated capabilities for AS_PATH encoding.
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	asn4 := true // Default to 4-byte if no session
	if session != nil {
		if neg := session.Negotiated(); neg != nil {
			asn4 = neg.ASN4
		}
	}

	// Track which AFI/SAFIs we've sent for EOR
	var hasIPv4, hasIPv6, hasIPv4VPN bool

	// Send routes in config order
	for _, route := range p.settings.FlowSpecRoutes {
		update := buildFlowSpecUpdate(route, p.settings.LocalAS, p.settings.IsIBGP(), asn4)
		if err := p.SendUpdate(update); err != nil {
			trace.Log(trace.Routes, "peer %s: FlowSpec send error: %v", addr, err)
		}
		// Track AFI/SAFI
		switch {
		case route.IsIPv6:
			hasIPv6 = true
		case route.RD != [8]byte{}:
			hasIPv4VPN = true
		default:
			hasIPv4 = true
		}
	}

	// Send EORs
	if hasIPv4 {
		eor := message.BuildEOR(nlri.Family{AFI: 1, SAFI: 133}) // IPv4 FlowSpec
		_ = p.SendUpdate(eor)
	}
	if hasIPv4VPN {
		eor := message.BuildEOR(nlri.Family{AFI: 1, SAFI: 134}) // IPv4 FlowSpec VPN
		_ = p.SendUpdate(eor)
	}
	if hasIPv6 {
		eor := message.BuildEOR(nlri.Family{AFI: 2, SAFI: 133}) // IPv6 FlowSpec
		_ = p.SendUpdate(eor)
	}
}

// buildFlowSpecUpdate builds an UPDATE message for a FlowSpec route.
// asn4 indicates whether to use 4-byte AS numbers in AS_PATH.
func buildFlowSpecUpdate(route FlowSpecRoute, localAS uint32, isIBGP, asn4 bool) *message.Update {
	var attrBytes []byte

	// 1. ORIGIN (IGP)
	origin := attribute.Origin(0)
	attrBytes = append(attrBytes, attribute.PackAttribute(origin)...)

	// 2. AS_PATH
	var asPath *attribute.ASPath
	if isIBGP {
		asPath = &attribute.ASPath{Segments: nil}
	} else {
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: []uint32{localAS}},
			},
		}
	}
	attrBytes = append(attrBytes, attribute.PackASPathAttribute(asPath, asn4)...)

	// 3. LOCAL_PREF
	if isIBGP {
		attrBytes = append(attrBytes, attribute.PackAttribute(attribute.LocalPref(100))...)
	}

	// 4. COMMUNITY (if set)
	if len(route.CommunityBytes) > 0 {
		commAttr := make([]byte, 0, 3+len(route.CommunityBytes))
		commAttr = append(commAttr, 0xC0, 0x08, byte(len(route.CommunityBytes)))
		commAttr = append(commAttr, route.CommunityBytes...)
		attrBytes = append(attrBytes, commAttr...)
	}

	// 5. EXTENDED_COMMUNITY (for actions like rate-limit, redirect)
	if len(route.ExtCommunityBytes) > 0 {
		ecAttr := make([]byte, 0, 3+len(route.ExtCommunityBytes))
		ecAttr = append(ecAttr, 0xC0, 0x10, byte(len(route.ExtCommunityBytes)))
		ecAttr = append(ecAttr, route.ExtCommunityBytes...)
		attrBytes = append(attrBytes, ecAttr...)
	}

	// 6. MP_REACH_NLRI for FlowSpec
	mpReach := buildFlowSpecMPReachNLRI(route)
	attrBytes = append(attrBytes, mpReach...)

	return &message.Update{
		PathAttributes: attrBytes,
	}
}

// buildFlowSpecMPReachNLRI builds MP_REACH_NLRI for a FlowSpec route.
func buildFlowSpecMPReachNLRI(route FlowSpecRoute) []byte {
	nlriData := route.NLRI
	if len(nlriData) == 0 {
		return nil
	}

	// Determine AFI/SAFI
	var afi uint16 = 1  // IPv4
	var safi byte = 133 // FlowSpec
	if route.IsIPv6 {
		afi = 2 // IPv6
	}
	isVPN := route.RD != [8]byte{}
	if isVPN {
		safi = 134 // FlowSpec VPN
	}

	// Next-hop (usually 0 length for FlowSpec, or actual for VPN)
	var nhBytes []byte
	if route.NextHop.IsValid() {
		nhBytes = route.NextHop.AsSlice()
	}

	// Build the full NLRI with length prefix.
	// For FlowSpec VPN, format is: length + RD (8 bytes) + FlowSpec NLRI
	var fullNLRI []byte
	if isVPN {
		// VPN format: length (1 or 2 bytes) + RD (8 bytes) + FlowSpec NLRI
		nlriLen := 8 + len(nlriData)
		if nlriLen < 240 {
			fullNLRI = make([]byte, 1+nlriLen)
			fullNLRI[0] = byte(nlriLen)
			copy(fullNLRI[1:9], route.RD[:])
			copy(fullNLRI[9:], nlriData)
		} else {
			fullNLRI = make([]byte, 2+nlriLen)
			fullNLRI[0] = 0xF0 | byte(nlriLen>>8)
			fullNLRI[1] = byte(nlriLen)
			copy(fullNLRI[2:10], route.RD[:])
			copy(fullNLRI[10:], nlriData)
		}
	} else {
		// Non-VPN format: FlowSpec NLRI already includes length prefix
		fullNLRI = nlriData
	}

	// Build MP_REACH_NLRI value
	valueLen := 2 + 1 + 1 + len(nhBytes) + 1 + len(fullNLRI)
	value := make([]byte, valueLen)
	value[0] = byte(afi >> 8)
	value[1] = byte(afi)
	value[2] = safi
	value[3] = byte(len(nhBytes))
	copy(value[4:4+len(nhBytes)], nhBytes)
	value[4+len(nhBytes)] = 0 // Reserved
	copy(value[5+len(nhBytes):], fullNLRI)

	// Build attribute header (use short form when possible)
	var attr []byte
	if valueLen > 255 {
		attr = make([]byte, 4+valueLen)
		attr[0] = 0x90 // Optional + Extended Length
		attr[1] = 14   // MP_REACH_NLRI
		attr[2] = byte(valueLen >> 8)
		attr[3] = byte(valueLen)
		copy(attr[4:], value)
	} else {
		attr = make([]byte, 3+valueLen)
		attr[0] = 0x80 // Optional
		attr[1] = 14   // MP_REACH_NLRI
		attr[2] = byte(valueLen)
		copy(attr[3:], value)
	}

	return attr
}

// sendMUPRoutes sends MUP routes configured for this peer.
func (p *Peer) sendMUPRoutes() {
	if len(p.settings.MUPRoutes) == 0 {
		return
	}

	addr := p.settings.Address.String()
	trace.Log(trace.Routes, "peer %s: sending %d MUP routes", addr, len(p.settings.MUPRoutes))

	// Get negotiated capabilities for AS_PATH encoding.
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	asn4 := true // Default to 4-byte if no session
	if session != nil {
		if neg := session.Negotiated(); neg != nil {
			asn4 = neg.ASN4
		}
	}

	// Separate IPv4 and IPv6 routes
	var ipv4Routes, ipv6Routes []MUPRoute
	for _, route := range p.settings.MUPRoutes {
		if route.IsIPv6 {
			ipv6Routes = append(ipv6Routes, route)
		} else {
			ipv4Routes = append(ipv4Routes, route)
		}
	}

	// Send IPv4 MUP routes
	for _, route := range ipv4Routes {
		update := buildMUPUpdate(route, p.settings.LocalAS, p.settings.IsIBGP(), asn4)
		if err := p.SendUpdate(update); err != nil {
			trace.Log(trace.Routes, "peer %s: MUP send error: %v", addr, err)
		}
	}

	// Send IPv6 MUP routes
	for _, route := range ipv6Routes {
		update := buildMUPUpdate(route, p.settings.LocalAS, p.settings.IsIBGP(), asn4)
		if err := p.SendUpdate(update); err != nil {
			trace.Log(trace.Routes, "peer %s: MUP send error: %v", addr, err)
		}
	}

	// Send EORs
	if len(ipv4Routes) > 0 {
		eor := message.BuildEOR(nlri.Family{AFI: 1, SAFI: 85}) // IPv4 MUP
		_ = p.SendUpdate(eor)
	}
	if len(ipv6Routes) > 0 {
		eor := message.BuildEOR(nlri.Family{AFI: 2, SAFI: 85}) // IPv6 MUP
		_ = p.SendUpdate(eor)
	}
}

// buildMUPUpdate builds an UPDATE message for a MUP route.
// asn4 indicates whether to use 4-byte AS numbers in AS_PATH.
func buildMUPUpdate(route MUPRoute, localAS uint32, isIBGP, asn4 bool) *message.Update {
	var attrBytes []byte

	// 1. ORIGIN (IGP)
	origin := attribute.Origin(0)
	attrBytes = append(attrBytes, attribute.PackAttribute(origin)...)

	// 2. AS_PATH
	var asPath *attribute.ASPath
	if isIBGP {
		asPath = &attribute.ASPath{Segments: nil}
	} else {
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: []uint32{localAS}},
			},
		}
	}
	attrBytes = append(attrBytes, attribute.PackASPathAttribute(asPath, asn4)...)

	// 3. LOCAL_PREF
	if isIBGP {
		attrBytes = append(attrBytes, attribute.PackAttribute(attribute.LocalPref(100))...)
	}

	// 4. EXTENDED_COMMUNITY
	if len(route.ExtCommunityBytes) > 0 {
		ecAttr := make([]byte, 0, 3+len(route.ExtCommunityBytes))
		ecAttr = append(ecAttr, 0xC0, 0x10, byte(len(route.ExtCommunityBytes)))
		ecAttr = append(ecAttr, route.ExtCommunityBytes...)
		attrBytes = append(attrBytes, ecAttr...)
	}

	// 5. PREFIX_SID (if present)
	if len(route.PrefixSID) > 0 {
		// Type 40 (Prefix SID)
		if len(route.PrefixSID) <= 255 {
			sidAttr := make([]byte, 0, 3+len(route.PrefixSID))
			sidAttr = append(sidAttr, 0xC0, 0x28, byte(len(route.PrefixSID)))
			sidAttr = append(sidAttr, route.PrefixSID...)
			attrBytes = append(attrBytes, sidAttr...)
		}
	}

	// 6. MP_REACH_NLRI for MUP
	mpReach := buildMUPMPReachNLRI(route)
	attrBytes = append(attrBytes, mpReach...)

	return &message.Update{
		PathAttributes: attrBytes,
	}
}

// buildMUPMPReachNLRI builds MP_REACH_NLRI for a MUP route.
func buildMUPMPReachNLRI(route MUPRoute) []byte {
	nlriData := route.NLRI
	if len(nlriData) == 0 {
		return nil
	}

	// Determine AFI
	var afi uint16 = 1 // IPv4
	if route.IsIPv6 {
		afi = 2 // IPv6
	}
	var safi byte = 85 // MUP

	// Next-hop
	nhBytes := route.NextHop.AsSlice()

	// Build MP_REACH_NLRI value
	valueLen := 2 + 1 + 1 + len(nhBytes) + 1 + len(nlriData)
	value := make([]byte, valueLen)
	value[0] = byte(afi >> 8)
	value[1] = byte(afi)
	value[2] = safi
	value[3] = byte(len(nhBytes))
	copy(value[4:4+len(nhBytes)], nhBytes)
	value[4+len(nhBytes)] = 0 // Reserved
	copy(value[5+len(nhBytes):], nlriData)

	// Build attribute header
	attr := make([]byte, 4+valueLen)
	attr[0] = 0x90
	attr[1] = 14
	attr[2] = byte(valueLen >> 8)
	attr[3] = byte(valueLen)
	copy(attr[4:], value)

	return attr
}
