package reactor

import (
	"context"
	"net"
	"net/netip"
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
func (p *Peer) sendInitialRoutes() {
	addr := p.neighbor.Address.String()
	trace.Log(trace.Routes, "neighbor %s: sending %d static routes", addr, len(p.neighbor.StaticRoutes))

	for _, route := range p.neighbor.StaticRoutes {
		update := buildStaticRouteUpdate(route, p.neighbor.LocalAS, p.neighbor.IsIBGP())
		if err := p.SendUpdate(update); err != nil {
			trace.Log(trace.Routes, "neighbor %s: send error: %v", addr, err)
			// Log error but continue - connection may have closed.
			break
		}
		trace.RouteSent(addr, route.Prefix.String(), route.NextHop.String())
	}

	// Send End-of-RIB marker for IPv4 unicast.
	if len(p.neighbor.StaticRoutes) > 0 {
		eor := buildEORUpdate(1, 1) // IPv4 unicast
		_ = p.SendUpdate(eor)
		trace.Log(trace.Routes, "neighbor %s: sent EOR marker", addr)
	}
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

	// 6. COMMUNITIES (RFC 1997) - if set
	if len(route.Communities) > 0 {
		comms := make(attribute.Communities, len(route.Communities))
		for i, c := range route.Communities {
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
	if route.IsVPN() {
		// VPN route: use MP_REACH_NLRI attribute (returns raw bytes)
		attrBytes = append(attrBytes, buildMPReachNLRI(route)...)
	} else if route.Prefix.Addr().Is4() {
		// IPv4 unicast: inline NLRI with optional path-id
		inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, route.Prefix, route.PathID)
		nlriBytes = inet.Bytes()
	} else {
		// IPv6 unicast: use MP_REACH_NLRI attribute (RFC 4760)
		attrBytes = append(attrBytes, buildMPReachNLRIUnicast(route)...)
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
