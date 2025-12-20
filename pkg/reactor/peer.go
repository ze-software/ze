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
	"github.com/exa-networks/zebgp/pkg/bgp/fsm"
	"github.com/exa-networks/zebgp/pkg/bgp/message"
	"github.com/exa-networks/zebgp/pkg/bgp/nlri"
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
	neighbor *Neighbor
	session  *Session

	state    atomic.Int32
	callback PeerCallback

	// Reconnect configuration
	reconnectMin time.Duration
	reconnectMax time.Duration

	// Goroutine control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu sync.RWMutex
}

// NewPeer creates a new peer for the given neighbor.
func NewPeer(neighbor *Neighbor) *Peer {
	return &Peer{
		neighbor:     neighbor,
		reconnectMin: DefaultReconnectMin,
		reconnectMax: DefaultReconnectMax,
	}
}

// Neighbor returns the configured neighbor.
func (p *Peer) Neighbor() *Neighbor {
	return p.neighbor
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
	session := NewSession(p.neighbor)

	p.mu.Lock()
	p.session = session
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		p.session = nil
		p.mu.Unlock()
	}()

	// Update state based on FSM mode
	if p.neighbor.Passive {
		p.setState(PeerStateActive)
	} else {
		p.setState(PeerStateConnecting)
	}

	// Start FSM
	if err := session.Start(); err != nil {
		return err
	}

	// Connect (for active mode)
	if !p.neighbor.Passive {
		if err := session.Connect(p.ctx); err != nil {
			return err
		}
	}

	// Monitor FSM state
	session.fsm.SetCallback(func(from, to fsm.State) {
		addr := p.neighbor.Address.String()
		trace.FSMTransition(addr, from.String(), to.String())

		if to == fsm.StateEstablished {
			p.setState(PeerStateEstablished)
			trace.SessionEstablished(addr, p.neighbor.LocalAS, p.neighbor.PeerAS)
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

// sendInitialRoutes sends static routes configured for this neighbor.
// Routes with identical attributes are grouped into a single UPDATE message.
func (p *Peer) sendInitialRoutes() {
	addr := p.neighbor.Address.String()
	trace.Log(trace.Routes, "neighbor %s: sending %d static routes", addr, len(p.neighbor.StaticRoutes))

	// Group routes by attributes (same attributes = same UPDATE).
	groups := groupRoutesByAttributes(p.neighbor.StaticRoutes)

	// Track which address families have routes.
	hasIPv4 := false
	hasIPv6 := false

	for _, routes := range groups {
		update := buildGroupedUpdate(routes, p.neighbor.LocalAS, p.neighbor.IsIBGP())
		if err := p.SendUpdate(update); err != nil {
			trace.Log(trace.Routes, "neighbor %s: send error: %v", addr, err)
			break
		}
		for _, route := range routes {
			trace.RouteSent(addr, route.Prefix.String(), route.NextHop.String())
			if route.Prefix.Addr().Is4() {
				hasIPv4 = true
			} else {
				hasIPv6 = true
			}
		}
	}

	// Send End-of-RIB marker for each address family with routes.
	if hasIPv4 {
		eor := buildEORUpdate(1, 1) // IPv4 unicast
		_ = p.SendUpdate(eor)
		trace.Log(trace.Routes, "neighbor %s: sent IPv4 unicast EOR marker", addr)
	}
	if hasIPv6 {
		eor := buildEORUpdate(2, 1) // IPv6 unicast
		_ = p.SendUpdate(eor)
		trace.Log(trace.Routes, "neighbor %s: sent IPv6 unicast EOR marker", addr)
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
func buildStaticRouteUpdate(route StaticRoute, localAS uint32, isIBGP bool) *message.Update {
	var attrBytes []byte

	// 1. ORIGIN (IGP by default, use configured value if set)
	origin := attribute.Origin(route.Origin)
	attrBytes = append(attrBytes, attribute.PackAttribute(origin)...)

	// 2. AS_PATH
	// - For iBGP self-originated routes: empty AS_PATH
	// - For eBGP: prepend local AS
	var asPath *attribute.ASPath
	if isIBGP {
		// Empty AS_PATH for iBGP self-originated routes
		asPath = &attribute.ASPath{Segments: nil}
	} else {
		// Prepend local AS for eBGP
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: []uint32{localAS}},
			},
		}
	}
	attrBytes = append(attrBytes, attribute.PackAttribute(asPath)...)

	// 3. NEXT_HOP (always include for IPv4 routes, even VPN - for compatibility)
	if route.NextHop.Is4() {
		nextHop := &attribute.NextHop{Addr: route.NextHop}
		attrBytes = append(attrBytes, attribute.PackAttribute(nextHop)...)
	}

	// 4. LOCAL_PREF (for iBGP, default 100 if not set)
	if isIBGP {
		localPref := route.LocalPreference
		if localPref == 0 {
			localPref = 100 // Default LOCAL_PREF
		}
		attrBytes = append(attrBytes, attribute.PackAttribute(attribute.LocalPref(localPref))...)
	}

	// 5. MED (if set)
	if route.MED > 0 {
		med := attribute.MED(route.MED)
		attrBytes = append(attrBytes, attribute.PackAttribute(med)...)
	}

	// 6. COMMUNITIES (RFC 1997) - sorted per RFC.
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

	// Key includes: nexthop, origin, localpref, med, communities, large-communities, ext-communities, vpn, ipv4/ipv6.
	// For IPv6 routes, include prefix in key to prevent grouping (each needs separate MP_REACH_NLRI UPDATE).
	// IPv4 routes can be grouped since multiple NLRIs can be in one UPDATE.
	prefixKey := ""
	if !r.Prefix.Addr().Is4() {
		prefixKey = r.Prefix.String()
	}
	return fmt.Sprintf("%s|%d|%d|%d|%v|%v|%s|%s|%v|%s",
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
func buildGroupedUpdate(routes []StaticRoute, localAS uint32, isIBGP bool) *message.Update {
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
	if isIBGP {
		asPath = &attribute.ASPath{Segments: nil}
	} else {
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: []uint32{localAS}},
			},
		}
	}
	attrBytes = append(attrBytes, attribute.PackAttribute(asPath)...)

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

	// 5. MED (if set).
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

// sendMVPNRoutes sends MVPN routes configured for this neighbor.
func (p *Peer) sendMVPNRoutes() {
	if len(p.neighbor.MVPNRoutes) == 0 {
		return
	}

	addr := p.neighbor.Address.String()
	trace.Log(trace.Routes, "neighbor %s: sending %d MVPN routes", addr, len(p.neighbor.MVPNRoutes))

	// Group MVPN routes by AFI and attributes for efficient UPDATE grouping
	ipv4Routes := make([]MVPNRoute, 0)
	ipv6Routes := make([]MVPNRoute, 0)

	for _, route := range p.neighbor.MVPNRoutes {
		if route.IsIPv6 {
			ipv6Routes = append(ipv6Routes, route)
		} else {
			ipv4Routes = append(ipv4Routes, route)
		}
	}

	// Group IPv4 routes by next-hop (different next-hop = different UPDATE)
	ipv4Groups := groupMVPNRoutesByNextHop(ipv4Routes)
	for nh, routes := range ipv4Groups {
		update := buildMVPNUpdate(routes, p.neighbor.LocalAS, p.neighbor.IsIBGP(), false)
		if err := p.SendUpdate(update); err != nil {
			trace.Log(trace.Routes, "neighbor %s: MVPN send error: %v", addr, err)
		} else {
			trace.Log(trace.Routes, "neighbor %s: sent %d IPv4 MVPN routes (NH=%s)", addr, len(routes), nh)
		}
	}

	// Group IPv6 routes by next-hop
	ipv6Groups := groupMVPNRoutesByNextHop(ipv6Routes)
	for nh, routes := range ipv6Groups {
		update := buildMVPNUpdate(routes, p.neighbor.LocalAS, p.neighbor.IsIBGP(), true)
		if err := p.SendUpdate(update); err != nil {
			trace.Log(trace.Routes, "neighbor %s: MVPN send error: %v", addr, err)
		} else {
			trace.Log(trace.Routes, "neighbor %s: sent %d IPv6 MVPN routes (NH=%s)", addr, len(routes), nh)
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
func buildMVPNUpdate(routes []MVPNRoute, localAS uint32, isIBGP, isIPv6 bool) *message.Update {
	if len(routes) == 0 {
		return &message.Update{}
	}

	// Use first route for common attributes
	first := routes[0]

	var attrBytes []byte

	// 1. ORIGIN (default to IGP if not set)
	originVal := first.Origin
	if originVal == 0 && first.Origin == 0 {
		// Origin 0 is IGP, which is correct default
	}
	origin := attribute.Origin(originVal)
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
	attrBytes = append(attrBytes, attribute.PackAttribute(asPath)...)

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

// sendVPLSRoutes sends VPLS routes configured for this neighbor.
func (p *Peer) sendVPLSRoutes() {
	if len(p.neighbor.VPLSRoutes) == 0 {
		return
	}
	// TODO: Implement VPLS route sending
	trace.Log(trace.Routes, "neighbor %s: VPLS routes not yet implemented (%d routes)",
		p.neighbor.Address.String(), len(p.neighbor.VPLSRoutes))
}

// sendFlowSpecRoutes sends FlowSpec routes configured for this neighbor.
func (p *Peer) sendFlowSpecRoutes() {
	if len(p.neighbor.FlowSpecRoutes) == 0 {
		return
	}
	// TODO: Implement FlowSpec route sending
	trace.Log(trace.Routes, "neighbor %s: FlowSpec routes not yet implemented (%d routes)",
		p.neighbor.Address.String(), len(p.neighbor.FlowSpecRoutes))
}

// sendMUPRoutes sends MUP routes configured for this neighbor.
func (p *Peer) sendMUPRoutes() {
	if len(p.neighbor.MUPRoutes) == 0 {
		return
	}
	// TODO: Implement MUP route sending
	trace.Log(trace.Routes, "neighbor %s: MUP routes not yet implemented (%d routes)",
		p.neighbor.Address.String(), len(p.neighbor.MUPRoutes))
}
