// Design: docs/architecture/core-design.md — family-specific route announce/withdraw (L3VPN, labeled unicast, MUP, FlowSpec, VPLS, L2VPN)
// Related: reactor_api.go — API command handling core
// Related: reactor_api_batch.go — NLRI batch operations
// Related: reactor_api_forward.go — forwarding and grouped sending
package reactor

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/rib"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/types"
)

// AnnounceFlowSpec announces a FlowSpec route to matching peers.
// TODO: Implement when FlowSpec RIB integration is complete.
func (a *reactorAPIAdapter) AnnounceFlowSpec(_ string, _ bgptypes.FlowSpecRoute) error {
	return errors.New("flowspec: not implemented")
}

// WithdrawFlowSpec withdraws a FlowSpec route from matching peers.
// TODO: Implement when FlowSpec RIB integration is complete.
func (a *reactorAPIAdapter) WithdrawFlowSpec(_ string, _ bgptypes.FlowSpecRoute) error {
	return errors.New("flowspec: not implemented")
}

// AnnounceVPLS announces a VPLS route to matching peers.
// TODO: Implement when VPLS RIB integration is complete.
func (a *reactorAPIAdapter) AnnounceVPLS(_ string, _ bgptypes.VPLSRoute) error {
	return errors.New("vpls: not implemented")
}

// WithdrawVPLS withdraws a VPLS route from matching peers.
// TODO: Implement when VPLS RIB integration is complete.
func (a *reactorAPIAdapter) WithdrawVPLS(_ string, _ bgptypes.VPLSRoute) error {
	return errors.New("vpls: not implemented")
}

// AnnounceL2VPN announces an L2VPN/EVPN route to matching peers.
// TODO: Implement when L2VPN/EVPN RIB integration is complete.
func (a *reactorAPIAdapter) AnnounceL2VPN(_ string, _ bgptypes.L2VPNRoute) error {
	return errors.New("l2vpn: not implemented")
}

// WithdrawL2VPN withdraws an L2VPN/EVPN route from matching peers.
// TODO: Implement when L2VPN/EVPN RIB integration is complete.
func (a *reactorAPIAdapter) WithdrawL2VPN(_ string, _ bgptypes.L2VPNRoute) error {
	return errors.New("l2vpn: not implemented")
}

// AnnounceL3VPN announces an L3VPN (MPLS VPN) route to matching peers.
// RFC 4364 - BGP/MPLS IP Virtual Private Networks.
//
// Behavior:
//   - Established peer: sends UPDATE immediately
//   - Non-established peer: queues to peer's operation queue (sent on connect)
func (a *reactorAPIAdapter) AnnounceL3VPN(peerSelector string, route bgptypes.L3VPNRoute) error {
	// RFC 4364: L3VPN routes require RD
	if route.RD == "" {
		return errors.New("l3vpn route requires route-distinguisher (rd)")
	}
	// RFC 4364: L3VPN routes require labels
	if len(route.Labels) == 0 {
		return errors.New("l3vpn route requires at least one label")
	}

	peers := a.getMatchingPeers(peerSelector)
	if len(peers) == 0 {
		return errors.New("no peers match selector")
	}

	// Build VPNParams once (peer-independent)
	params, err := a.buildL3VPNParams(route)
	if err != nil {
		return fmt.Errorf("invalid route: %w", err)
	}

	var lastErr error
	for _, peer := range peers {
		isIBGP := peer.Settings().IsIBGP()

		if !peer.ShouldQueue() {
			// Send immediately
			family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIVPN} // RFC 4364
			if route.Prefix.Addr().Is6() {
				family.AFI = nlri.AFIIPv6
			}
			asn4 := peer.asn4()
			addPath := peer.addPathFor(family)

			// Build UPDATE using UpdateBuilder for immediate send
			ub := message.NewUpdateBuilder(a.r.config.LocalAS, isIBGP, asn4, addPath)
			update := ub.BuildVPN(&params)

			if err := peer.SendUpdate(update); err != nil {
				lastErr = err
			}
		} else {
			// Session not established or queue draining: queue to preserve order
			ribRoute, err := a.buildL3VPNRIBRoute(route, isIBGP)
			if err != nil {
				lastErr = err
				continue
			}
			peer.QueueAnnounce(ribRoute)
		}
	}
	return lastErr
}

// WithdrawL3VPN withdraws an L3VPN route from matching peers.
// RFC 4364 - Uses MP_UNREACH_NLRI with SAFI 128.
//
// Behavior:
//   - Established peer: sends UPDATE with MP_UNREACH_NLRI immediately
//   - Non-established peer: queues withdrawal (sent on connect)
func (a *reactorAPIAdapter) WithdrawL3VPN(peerSelector string, route bgptypes.L3VPNRoute) error {
	// RFC 4364: RD required to identify the VPN route
	if route.RD == "" {
		return errors.New("l3vpn withdrawal requires route-distinguisher (rd)")
	}

	peers := a.getMatchingPeers(peerSelector)
	if len(peers) == 0 {
		return errors.New("no peers match selector")
	}

	// Parse RD for NLRI
	rd, err := nlri.ParseRDString(route.RD)
	if err != nil {
		return fmt.Errorf("invalid rd: %w", err)
	}

	// Use first label from stack for withdrawal (RFC allows - prefix identifies route)
	labels := route.Labels
	if len(labels) == 0 {
		labels = []uint32{0x800000} // RFC 3107 withdrawal label
	}

	// Build NLRI for withdrawal
	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIVPN} // RFC 4364
	if route.Prefix.Addr().Is6() {
		family.AFI = nlri.AFIIPv6
	}

	n, err := encodeVPNNLRI(family, rd, labels[:1], route.Prefix)
	if err != nil {
		return fmt.Errorf("encode VPN withdrawal NLRI: %w", err)
	}

	// Build StaticRoute for withdrawal
	staticRoute := StaticRoute{
		Prefix: route.Prefix,
		RD:     route.RD,
		Labels: labels[:1],
	}
	copy(staticRoute.RDBytes[:], rd.Bytes())

	var lastErr error
	for _, peer := range peers {
		if !peer.ShouldQueue() {
			// Build MP_UNREACH_NLRI for VPN
			attrBuf := getBuildBuf()
			update := buildMPUnreachVPN(attrBuf, &staticRoute)
			sendErr := peer.SendUpdate(update)
			putBuildBuf(attrBuf)
			if sendErr != nil {
				lastErr = sendErr
			}
		} else {
			// Session not established or queue draining: queue to preserve order
			peer.QueueWithdraw(n)
		}
	}
	return lastErr
}

// buildL3VPNParams converts an bgptypes.L3VPNRoute to message.VPNParams.
// RFC 4364 - VPN route parameters.
func (a *reactorAPIAdapter) buildL3VPNParams(route bgptypes.L3VPNRoute) (message.VPNParams, error) {
	// Parse RD
	rd, err := nlri.ParseRDString(route.RD)
	if err != nil {
		return message.VPNParams{}, fmt.Errorf("invalid rd: %w", err)
	}

	params := message.VPNParams{
		Prefix:  route.Prefix,
		NextHop: route.NextHop,
		Labels:  route.Labels,
		Origin:  attribute.OriginIGP,
	}

	// Copy RD bytes
	rdBytes := rd.Bytes()
	copy(params.RDBytes[:], rdBytes)

	// Extract optional attributes from Wire (wire-first approach)
	if route.Wire != nil {
		// Extract ORIGIN
		if originAttr, err := route.Wire.Get(attribute.AttrOrigin); err == nil {
			if o, ok := originAttr.(attribute.Origin); ok {
				params.Origin = o
			}
		}
		// Extract LOCAL_PREF
		if lpAttr, err := route.Wire.Get(attribute.AttrLocalPref); err == nil {
			if lp, ok := lpAttr.(attribute.LocalPref); ok {
				params.LocalPreference = uint32(lp)
			}
		}
		// Extract MED
		if medAttr, err := route.Wire.Get(attribute.AttrMED); err == nil {
			if m, ok := medAttr.(attribute.MED); ok {
				params.MED = uint32(m)
			}
		}
		// Extract AS_PATH
		if asPathAttr, err := route.Wire.Get(attribute.AttrASPath); err == nil {
			if asp, ok := asPathAttr.(*attribute.ASPath); ok && len(asp.Segments) > 0 {
				params.ASPath = asp.Segments[0].ASNs
			}
		}
		// Extract COMMUNITY
		if commAttr, err := route.Wire.Get(attribute.AttrCommunity); err == nil {
			if comms, ok := commAttr.(attribute.Communities); ok {
				params.Communities = make([]uint32, len(comms))
				for i, c := range comms {
					params.Communities[i] = uint32(c)
				}
			}
		}
		// Extract LARGE_COMMUNITY
		if lcAttr, err := route.Wire.Get(attribute.AttrLargeCommunity); err == nil {
			if lcs, ok := lcAttr.(attribute.LargeCommunities); ok {
				params.LargeCommunities = make([][3]uint32, len(lcs))
				for i, c := range lcs {
					params.LargeCommunities[i] = [3]uint32{c.GlobalAdmin, c.LocalData1, c.LocalData2}
				}
			}
		}
		// Extract EXTENDED_COMMUNITIES
		if ecAttr, err := route.Wire.Get(attribute.AttrExtCommunity); err == nil {
			if ecs, ok := ecAttr.(attribute.ExtendedCommunities); ok {
				start := len(params.ExtCommunityBytes)
				needed := ecs.Len()
				params.ExtCommunityBytes = slices.Grow(params.ExtCommunityBytes, needed)[:start+needed]
				ecs.WriteTo(params.ExtCommunityBytes, start)
			}
		}
	}

	// Handle RT (Route Target) - convert to extended community
	if route.RT != "" {
		rtBytes, err := parseRouteTarget(route.RT)
		if err != nil {
			return message.VPNParams{}, fmt.Errorf("invalid rt: %w", err)
		}
		params.ExtCommunityBytes = append(params.ExtCommunityBytes, rtBytes...)
	}

	return params, nil
}

// buildL3VPNRIBRoute creates a rib.Route from an bgptypes.L3VPNRoute for queueing.
// RFC 4364: VPN routes include RD + labels in NLRI.
func (a *reactorAPIAdapter) buildL3VPNRIBRoute(route bgptypes.L3VPNRoute, isIBGP bool) (*rib.Route, error) {
	// Parse RD
	rd, err := nlri.ParseRDString(route.RD)
	if err != nil {
		return nil, fmt.Errorf("invalid rd: %w", err)
	}

	// Build NLRI
	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIVPN} // RFC 4364
	if route.Prefix.Addr().Is6() {
		family.AFI = nlri.AFIIPv6
	}

	n, err := encodeVPNNLRI(family, rd, route.Labels, route.Prefix)
	if err != nil {
		return nil, fmt.Errorf("encode VPN NLRI: %w", err)
	}

	// Build attributes from Wire (wire-first approach)
	var attrs []attribute.Attribute
	var userASPath []uint32

	if route.Wire != nil {
		// Parse attributes from wire format
		attrs, err = route.Wire.All()
		if err != nil {
			return nil, fmt.Errorf("failed to parse route attributes: %w", err)
		}
		// Extract AS_PATH if present
		if asPathAttr, err := route.Wire.Get(attribute.AttrASPath); err == nil {
			if asp, ok := asPathAttr.(*attribute.ASPath); ok && len(asp.Segments) > 0 {
				userASPath = asp.Segments[0].ASNs
			}
		}
	} else {
		// No wire attributes - use defaults
		attrs = append(attrs, attribute.OriginIGP)
	}

	// Handle RT (Route Target) - convert to extended community
	if route.RT != "" {
		rtBytes, err := parseRouteTarget(route.RT)
		if err != nil {
			return nil, fmt.Errorf("invalid rt: %w", err)
		}
		ec, err := attribute.ParseExtendedCommunities(rtBytes)
		if err != nil {
			return nil, fmt.Errorf("invalid rt extended community: %w", err)
		}
		attrs = append(attrs, ec)
	}

	// Build AS_PATH
	var asPath *attribute.ASPath
	switch {
	case len(userASPath) > 0:
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: userASPath},
			},
		}
	case isIBGP:
		asPath = &attribute.ASPath{Segments: nil}
	default: // eBGP: prepend local AS
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: []uint32{a.r.config.LocalAS}},
			},
		}
	}

	return rib.NewRouteWithASPath(n, route.NextHop, attrs, asPath), nil
}

// Extended community type codes per RFC 4360 Section 3.
const (
	ecTypeTransitive2ByteAS = 0x00 // 2-byte AS, transitive
	ecTypeTransitiveIPv4    = 0x01 // IPv4 address, transitive
	ecTypeTransitive4ByteAS = 0x02 // 4-byte AS, transitive
	ecSubtypeRouteTarget    = 0x02 // Route Target subtype
)

// parseRouteTarget parses a Route Target string to extended community bytes.
//
// RFC 4360 Section 3 - Extended Community format.
// Supported formats:
//   - "target:ASN:NN" or "ASN:NN" - 2-byte ASN with 4-byte value
//   - "target:IP:NN" or "IP:NN" - IPv4 address with 2-byte value
//   - 4-byte ASN automatically uses Type 2 format
func parseRouteTarget(s string) ([]byte, error) {
	// Remove "target:" prefix if present
	s = strings.TrimPrefix(s, "target:")

	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid rt format: %s (expected ASN:NN or IP:NN)", s)
	}

	// Check if first part is an IP address (Type 1 format)
	if ip, err := netip.ParseAddr(parts[0]); err == nil && ip.Is4() {
		val, err := strconv.ParseUint(parts[1], 10, 16)
		if err != nil {
			return nil, fmt.Errorf("invalid rt value %q (must be 0-65535 for IP:NN format)", parts[1])
		}
		b := ip.As4()
		return []byte{
			ecTypeTransitiveIPv4, ecSubtypeRouteTarget,
			b[0], b[1], b[2], b[3],
			byte(val >> 8), byte(val),
		}, nil
	}

	// Parse as ASN:NN format
	asn, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid ASN in rt: %s", parts[0])
	}

	val, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid value in rt: %s", parts[1])
	}

	// RFC 4360 Section 3 - Extended Community encoding
	if asn <= 65535 {
		// Type 0: 2-byte ASN, 4-byte value
		var buf [8]byte
		buf[0], buf[1] = ecTypeTransitive2ByteAS, ecSubtypeRouteTarget
		binary.BigEndian.PutUint16(buf[2:], uint16(asn))
		binary.BigEndian.PutUint32(buf[4:], uint32(val))
		return buf[:], nil
	}

	// ASN > 65535: Use Type 2 (4-byte ASN) if value fits in 16 bits
	if val > 65535 {
		return nil, fmt.Errorf("invalid rt: 4-byte ASN requires value <= 65535, got %d", val)
	}
	var buf [8]byte
	buf[0], buf[1] = ecTypeTransitive4ByteAS, ecSubtypeRouteTarget
	binary.BigEndian.PutUint32(buf[2:], uint32(asn))
	binary.BigEndian.PutUint16(buf[6:], uint16(val))
	return buf[:], nil
}

// AnnounceLabeledUnicast announces an MPLS labeled unicast route (SAFI 4).
// RFC 8277 - Using BGP to Bind MPLS Labels to Address Prefixes.
//
// Supports three modes like AnnounceRoute:
//   - Transaction mode: queues to Adj-RIB-Out (sent on commit)
//   - Established: sends immediately and tracks for re-announcement
//   - Not established: queues to peer's operation queue.
func (a *reactorAPIAdapter) AnnounceLabeledUnicast(peerSelector string, route bgptypes.LabeledUnicastRoute) error {
	// RFC 8277: Labeled unicast routes require at least one label
	if len(route.Labels) == 0 {
		return errors.New("labeled unicast route requires at least one label")
	}

	peers := a.getMatchingPeers(peerSelector)
	if len(peers) == 0 {
		return errors.New("no peers match selector")
	}

	var lastErr error
	for _, peer := range peers {
		isIBGP := peer.Settings().IsIBGP()

		// Build rib.Route with ALL attributes (not just Origin like AnnounceRoute bug)
		ribRoute, err := a.buildLabeledUnicastRIBRoute(route, isIBGP)
		if err != nil {
			lastErr = err
			continue
		}

		if !peer.ShouldQueue() {
			// Send immediately
			family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIMPLSLabel}
			if route.Prefix.Addr().Is6() {
				family.AFI = nlri.AFIIPv6
			}
			addPath := peer.addPathFor(family)
			asn4 := peer.asn4()

			// Build UPDATE using UpdateBuilder for immediate send
			ub := message.NewUpdateBuilder(a.r.config.LocalAS, isIBGP, asn4, addPath)
			params := a.buildLabeledUnicastParams(route)
			update := ub.BuildLabeledUnicast(&params)

			if err := peer.SendUpdate(update); err != nil {
				lastErr = err
			}
		} else {
			// Session not established or queue draining: queue to preserve order
			peer.QueueAnnounce(ribRoute)
		}
	}
	return lastErr
}

// buildLabeledUnicastParams converts an API route to message.LabeledUnicastParams.
func (a *reactorAPIAdapter) buildLabeledUnicastParams(route bgptypes.LabeledUnicastRoute) message.LabeledUnicastParams {
	params := message.LabeledUnicastParams{
		Prefix:  route.Prefix,
		PathID:  route.PathID, // RFC 7911 ADD-PATH
		NextHop: route.NextHop,
		Labels:  route.Labels, // RFC 8277: Multi-label support
		Origin:  attribute.OriginIGP,
	}

	// Extract optional attributes from Wire (wire-first approach)
	if route.Wire != nil {
		// Extract ORIGIN
		if originAttr, err := route.Wire.Get(attribute.AttrOrigin); err == nil {
			if o, ok := originAttr.(attribute.Origin); ok {
				params.Origin = o
			}
		}
		// Extract LOCAL_PREF
		if lpAttr, err := route.Wire.Get(attribute.AttrLocalPref); err == nil {
			if lp, ok := lpAttr.(attribute.LocalPref); ok {
				params.LocalPreference = uint32(lp)
			}
		}
		// Extract MED
		if medAttr, err := route.Wire.Get(attribute.AttrMED); err == nil {
			if m, ok := medAttr.(attribute.MED); ok {
				params.MED = uint32(m)
			}
		}
		// Extract AS_PATH
		if asPathAttr, err := route.Wire.Get(attribute.AttrASPath); err == nil {
			if asp, ok := asPathAttr.(*attribute.ASPath); ok && len(asp.Segments) > 0 {
				params.ASPath = asp.Segments[0].ASNs
			}
		}
		// Extract COMMUNITY
		if commAttr, err := route.Wire.Get(attribute.AttrCommunity); err == nil {
			if comms, ok := commAttr.(attribute.Communities); ok {
				params.Communities = make([]uint32, len(comms))
				for i, c := range comms {
					params.Communities[i] = uint32(c)
				}
			}
		}
		// Extract LARGE_COMMUNITY
		if lcAttr, err := route.Wire.Get(attribute.AttrLargeCommunity); err == nil {
			if lcs, ok := lcAttr.(attribute.LargeCommunities); ok {
				params.LargeCommunities = make([][3]uint32, len(lcs))
				for i, c := range lcs {
					params.LargeCommunities[i] = [3]uint32{c.GlobalAdmin, c.LocalData1, c.LocalData2}
				}
			}
		}
		// Extract EXTENDED_COMMUNITIES
		if ecAttr, err := route.Wire.Get(attribute.AttrExtCommunity); err == nil {
			if ecs, ok := ecAttr.(attribute.ExtendedCommunities); ok {
				buf := make([]byte, ecs.Len())
				ecs.WriteTo(buf, 0)
				params.ExtCommunityBytes = buf
			}
		}
	}

	return params
}

// buildLabeledUnicastRIBRoute creates a rib.Route from a LabeledUnicastRoute.
// Unlike AnnounceRoute which only stores OriginIGP, this stores ALL attributes.
// This ensures attributes are preserved when routes are queued and replayed.
//
// RFC 8277: Labeled unicast routes include MPLS labels in the NLRI.
// RFC 7911: PathID is included when ADD-PATH is negotiated.
func (a *reactorAPIAdapter) buildLabeledUnicastRIBRoute(route bgptypes.LabeledUnicastRoute, isIBGP bool) (*rib.Route, error) {
	// 1. Build NLRI with nlri.LabeledUnicast
	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIMPLSLabel}
	if route.Prefix.Addr().Is6() {
		family.AFI = nlri.AFIIPv6
	}

	// Default label if not specified
	labels := route.Labels
	if len(labels) == 0 {
		labels = []uint32{0}
	}

	n, err := encodeLabeledNLRI(family, route.Prefix, labels, route.PathID)
	if err != nil {
		return nil, fmt.Errorf("encode labeled NLRI: %w", err)
	}

	// 2. Build attributes from Wire (wire-first approach)
	var attrs []attribute.Attribute
	var userASPath []uint32

	if route.Wire != nil {
		// Parse attributes from wire format
		var err error
		attrs, err = route.Wire.All()
		if err != nil {
			return nil, fmt.Errorf("failed to parse route attributes: %w", err)
		}
		// Extract AS_PATH if present
		if asPathAttr, err := route.Wire.Get(attribute.AttrASPath); err == nil {
			if asp, ok := asPathAttr.(*attribute.ASPath); ok && len(asp.Segments) > 0 {
				userASPath = asp.Segments[0].ASNs
			}
		}
	} else {
		// No wire attributes - use defaults
		attrs = append(attrs, attribute.OriginIGP)
	}

	// 3. Build AS-PATH
	// RFC 4271 §5.1.2: iBGP SHALL NOT modify AS_PATH; eBGP prepends local AS
	var asPath *attribute.ASPath
	switch {
	case len(userASPath) > 0:
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: userASPath},
			},
		}
	case isIBGP:
		asPath = &attribute.ASPath{Segments: nil}
	default: // eBGP: prepend local AS
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: []uint32{a.r.config.LocalAS}},
			},
		}
	}

	return rib.NewRouteWithASPath(n, route.NextHop, attrs, asPath), nil
}

// WithdrawLabeledUnicast withdraws an MPLS labeled unicast route.
// RFC 8277 - Uses MP_UNREACH_NLRI with SAFI 4.
//
// Supports three modes like WithdrawRoute:
//   - Transaction mode: queues to Adj-RIB-Out (sent on commit)
//   - Established: sends immediately and removes from sent cache
//   - Not established: queues to peer's operation queue.
func (a *reactorAPIAdapter) WithdrawLabeledUnicast(peerSelector string, route bgptypes.LabeledUnicastRoute) error {
	peers := a.getMatchingPeers(peerSelector)
	if len(peers) == 0 {
		return errors.New("no peers match selector")
	}

	// Build NLRI for queueing
	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIMPLSLabel}
	if route.Prefix.Addr().Is6() {
		family.AFI = nlri.AFIIPv6
	}

	// Default label for withdrawal
	labels := route.Labels
	if len(labels) == 0 {
		labels = []uint32{0x800000} // RFC 8277 withdrawal label
	}

	n, err := encodeLabeledNLRI(family, route.Prefix, labels, route.PathID)
	if err != nil {
		return fmt.Errorf("encode labeled withdrawal NLRI: %w", err)
	}

	var lastErr error
	for _, peer := range peers {
		if !peer.ShouldQueue() {
			// Send immediately
			addPath := peer.addPathFor(family)

			// Build withdrawal using existing helper
			staticRoute := StaticRoute{
				Prefix: route.Prefix,
				Labels: labels,
			}

			attrBuf := getBuildBuf()
			update := buildMPUnreachLabeledUnicast(attrBuf, &staticRoute, addPath)
			sendErr := peer.SendUpdate(update)
			putBuildBuf(attrBuf)
			if sendErr != nil {
				lastErr = sendErr
			}
		} else {
			// Session not established or queue draining: queue to preserve order
			peer.QueueWithdraw(n)
		}
	}
	return lastErr
}

// AnnounceMUPRoute announces a MUP route (SAFI 85) to matching peers.
// draft-mpmz-bess-mup-safi - Mobile User Plane.
func (a *reactorAPIAdapter) AnnounceMUPRoute(peerSelector string, spec bgptypes.MUPRouteSpec) error {
	return a.sendMUPRoute(peerSelector, spec, false)
}

// WithdrawMUPRoute withdraws a MUP route from matching peers.
// Uses MP_UNREACH_NLRI with SAFI 85.
func (a *reactorAPIAdapter) WithdrawMUPRoute(peerSelector string, spec bgptypes.MUPRouteSpec) error {
	return a.sendMUPRoute(peerSelector, spec, true)
}

// sendMUPRoute is a common helper for announce/withdraw MUP routes.
func (a *reactorAPIAdapter) sendMUPRoute(peerSelector string, spec bgptypes.MUPRouteSpec, isWithdraw bool) error {
	peers := a.getMatchingPeers(peerSelector)
	if len(peers) == 0 {
		return errors.New("no peers match selector")
	}

	// Convert API spec to reactor MUPRoute
	mupRoute, err := convertAPIMUPRoute(spec)
	if err != nil {
		return fmt.Errorf("convert MUP route: %w", err)
	}

	var lastErr error
	for _, peer := range peers {
		if peer.State() != PeerStateEstablished {
			continue
		}

		// Check if MUP family is negotiated
		nc := peer.negotiated.Load()
		if nc == nil {
			continue
		}
		if spec.IsIPv6 && !nc.Has(nlri.IPv6MUP) {
			continue // Skip peer that doesn't support IPv6 MUP
		}
		if !spec.IsIPv6 && !nc.Has(nlri.IPv4MUP) {
			continue // Skip peer that doesn't support IPv4 MUP
		}

		family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: safiMUP}
		if spec.IsIPv6 {
			family.AFI = nlri.AFIIPv6
		}
		addPath := peer.addPathFor(family)
		asn4 := peer.asn4()

		// Build UPDATE using UpdateBuilder
		ub := message.NewUpdateBuilder(peer.settings.LocalAS, peer.settings.IsIBGP(), asn4, addPath)
		var update *message.Update
		if isWithdraw {
			update = ub.BuildMUPWithdraw(toMUPParams(mupRoute))
		} else {
			update = ub.BuildMUP(toMUPParams(mupRoute))
		}

		if err := peer.SendUpdate(update); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// convertAPIMUPRoute converts an bgptypes.MUPRouteSpec to a reactor.MUPRoute.
// This function parses the string fields in the API spec into wire-format bytes.
func convertAPIMUPRoute(spec bgptypes.MUPRouteSpec) (MUPRoute, error) {
	route := MUPRoute{
		IsIPv6: spec.IsIPv6,
	}

	// Convert route type string to numeric
	switch spec.RouteType {
	case "mup-isd":
		route.RouteType = 1
	case "mup-dsd":
		route.RouteType = 2
	case "mup-t1st":
		route.RouteType = 3
	case "mup-t2st":
		route.RouteType = 4
	default: // unknown MUP route type
		return route, fmt.Errorf("unknown MUP route type: %s", spec.RouteType)
	}

	// Parse NextHop
	if spec.NextHop != "" {
		ip, err := netip.ParseAddr(spec.NextHop)
		if err != nil {
			return route, fmt.Errorf("parse next-hop: %w", err)
		}
		route.NextHop = ip
	}

	// Build MUP NLRI bytes via plugin registry
	nlriBytes, err := encodeMUPNLRIBytes(spec)
	if err != nil {
		return route, fmt.Errorf("build MUP NLRI: %w", err)
	}
	route.NLRI = nlriBytes

	// Parse extended communities if present
	if spec.ExtCommunity != "" {
		ecBytes, err := parseAPIExtCommunity(spec.ExtCommunity)
		if err != nil {
			return route, fmt.Errorf("parse extended-community: %w", err)
		}
		route.ExtCommunityBytes = ecBytes
	}

	// Parse SRv6 Prefix-SID if present
	if spec.PrefixSID != "" {
		sidBytes, err := parseAPIPrefixSIDSRv6(spec.PrefixSID)
		if err != nil {
			return route, fmt.Errorf("parse prefix-sid-srv6: %w", err)
		}
		route.PrefixSID = sidBytes
	}

	return route, nil
}

// parseAPIExtCommunity parses extended community string to bytes.
func parseAPIExtCommunity(s string) ([]byte, error) {
	// Strip brackets if present: "[target:10:10]" -> "target:10:10"
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	s = strings.TrimSpace(s)

	// Parse "type:ASN:value" format (e.g., "target:10:10")
	parts := strings.SplitN(s, ":", 3)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid extended community: %s", s)
	}

	ecType := strings.ToLower(parts[0])
	switch ecType {
	case "target":
		// Route Target: Type 0x00, Subtype 0x02
		if len(parts) != 3 {
			return nil, fmt.Errorf("target requires ASN:value format")
		}
		var asn, val uint64
		if _, err := fmt.Sscanf(parts[1]+":"+parts[2], "%d:%d", &asn, &val); err != nil {
			return nil, fmt.Errorf("invalid target values: %s:%s", parts[1], parts[2])
		}
		ec := [8]byte{0x00, 0x02}
		ec[2] = byte(asn >> 8)
		ec[3] = byte(asn)
		ec[4] = byte(val >> 24)
		ec[5] = byte(val >> 16)
		ec[6] = byte(val >> 8)
		ec[7] = byte(val)
		return ec[:], nil

	case "origin":
		// Route Origin: Type 0x00, Subtype 0x03
		if len(parts) != 3 {
			return nil, fmt.Errorf("origin requires ASN:value format")
		}
		var asn, val uint64
		if _, err := fmt.Sscanf(parts[1]+":"+parts[2], "%d:%d", &asn, &val); err != nil {
			return nil, fmt.Errorf("invalid origin values: %s:%s", parts[1], parts[2])
		}
		ec := [8]byte{0x00, 0x03}
		ec[2] = byte(asn >> 8)
		ec[3] = byte(asn)
		ec[4] = byte(val >> 24)
		ec[5] = byte(val >> 16)
		ec[6] = byte(val >> 8)
		ec[7] = byte(val)
		return ec[:], nil

	default: // unknown extended community type
		return nil, fmt.Errorf("unknown extended community type: %s", ecType)
	}
}

// parseAPIPrefixSIDSRv6 parses SRv6 Prefix-SID string to bytes.
func parseAPIPrefixSIDSRv6(s string) ([]byte, error) {
	if s == "" {
		return nil, nil
	}

	// Parse service type
	var serviceType byte
	switch {
	case strings.HasPrefix(s, "l3-service"):
		serviceType = 5 // TLV Type 5: SRv6 L3 Service
		s = strings.TrimPrefix(s, "l3-service")
	case strings.HasPrefix(s, "l2-service"):
		serviceType = 6 // TLV Type 6: SRv6 L2 Service
		s = strings.TrimPrefix(s, "l2-service")
	default: // invalid srv6 prefix-sid
		return nil, fmt.Errorf("invalid srv6 prefix-sid: expected l3-service or l2-service")
	}
	s = strings.TrimSpace(s)

	// Parse IPv6 address
	fields := strings.Fields(s)
	if len(fields) < 1 {
		return nil, fmt.Errorf("invalid srv6 prefix-sid: missing IPv6 address")
	}
	ipv6, err := netip.ParseAddr(fields[0])
	if err != nil || !ipv6.Is6() {
		return nil, fmt.Errorf("invalid srv6 prefix-sid: expected IPv6 address, got %q", fields[0])
	}

	var behavior byte
	var sidStruct []byte

	// Parse optional behavior (0xNN format)
	for _, f := range fields[1:] {
		if strings.HasPrefix(f, "0x") || strings.HasPrefix(f, "0X") {
			behVal, err := parseHexByte(f)
			if err != nil {
				return nil, fmt.Errorf("invalid srv6 behavior %q: %w", f, err)
			}
			behavior = behVal
		} else if after, ok := strings.CutPrefix(f, "["); ok {
			// Parse SID structure [LB,LN,Func,Arg,TransLen,TransOffset]
			structStr := after
			structStr = strings.TrimSuffix(structStr, "]")
			parts := strings.Split(structStr, ",")
			if len(parts) != 6 {
				return nil, fmt.Errorf("invalid srv6 SID structure: expected 6 values")
			}
			for _, p := range parts {
				v, err := parseUint8(strings.TrimSpace(p))
				if err != nil {
					return nil, fmt.Errorf("invalid srv6 SID structure value %q: %w", p, err)
				}
				sidStruct = append(sidStruct, v)
			}
		}
	}

	// Build wire format per RFC 9252 — single allocation.
	// Inner value: reserved(1) + IPv6(16) + flags(1) + reserved(1) + behavior(1) = 20
	// Optional SID struct sub-TLV: type(2) + length(2) + struct(6) = 10
	// Inner TLV header: type(2) + length(2) = 4
	// Outer TLV header: type(1) + length(2) = 3
	innerValueLen := 20
	if len(sidStruct) == 6 {
		innerValueLen += 2 + 2 + 6 // sub-TLV
	}
	totalLen := 3 + 4 + innerValueLen
	result := make([]byte, totalLen)
	off := 0

	// Outer header
	outerLen := totalLen - 3
	result[off] = serviceType
	result[off+1] = byte(outerLen >> 8)
	result[off+2] = byte(outerLen)
	off += 3

	// Inner TLV header
	result[off] = 0
	result[off+1] = 1
	result[off+2] = byte(innerValueLen >> 8)
	result[off+3] = byte(innerValueLen)
	off += 4

	// Inner value
	result[off] = 0 // reserved
	off++
	a16 := ipv6.As16()
	copy(result[off:], a16[:])
	off += 16
	result[off] = 0   // flags
	result[off+1] = 0 // reserved
	result[off+2] = behavior
	off += 3

	// Optional SID structure sub-TLV
	if len(sidStruct) == 6 {
		result[off] = 0
		result[off+1] = 1
		result[off+2] = 0
		result[off+3] = byte(len(sidStruct))
		copy(result[off+4:], sidStruct)
	}

	return result, nil
}

func parseHexByte(s string) (byte, error) {
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	var v uint64
	_, err := fmt.Sscanf(s, "%x", &v)
	if err != nil || v > 255 {
		return 0, fmt.Errorf("invalid hex byte: %s", s)
	}
	return byte(v), nil
}

func parseUint8(s string) (byte, error) {
	var v uint64
	_, err := fmt.Sscanf(s, "%d", &v)
	if err != nil || v > 255 {
		return 0, fmt.Errorf("invalid uint8: %s", s)
	}
	return byte(v), nil
}
