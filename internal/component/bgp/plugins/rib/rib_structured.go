// Design: docs/architecture/plugin/rib-storage-design.md — RIB plugin structured delivery
// Overview: rib.go — RIB plugin main entry and JSON dispatch
//
// Structured event handlers for DirectBridge delivery.
// These handlers read from StructuredEvent metadata fields and RawMessage wire types
// instead of parsing JSON, eliminating the JSON round-trip for internal plugins.
package rib

import (
	"encoding/hex"
	"net/netip"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/storage"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// dispatchStructured routes a StructuredEvent to the appropriate handler.
func (r *RIBManager) dispatchStructured(se *rpc.StructuredEvent) {
	switch se.EventType {
	case "sent":
		r.handleSentStructured(se)
	case "update":
		r.handleReceivedStructured(se)
	case "state":
		r.handleStructuredState(se)
	case "refresh":
		r.handleRefreshStructured(se)
	case "borr":
		logger().Debug("received BoRR marker", "peer", se.PeerAddress)
	case "eorr":
		logger().Debug("received EoRR marker", "peer", se.PeerAddress)
	}
}

// handleReceivedStructured processes received UPDATE events from wire types.
// Reads raw bytes directly from WireUpdate sections — no hex encode/decode round-trip.
func (r *RIBManager) handleReceivedStructured(se *rpc.StructuredEvent) {
	peerAddr := se.PeerAddress
	if peerAddr == "" {
		return
	}

	msg, ok := se.RawMessage.(*bgptypes.RawMessage)
	if !ok || msg == nil || msg.WireUpdate == nil {
		return
	}

	wu := msg.WireUpdate

	// Get raw attribute bytes directly (no hex encode/decode).
	var attrBytes []byte
	if msg.AttrsWire != nil {
		attrBytes = msg.AttrsWire.Packed()
	}

	// Get encoding context for add-path flags.
	ctx := bgpctx.Registry.Get(wu.SourceCtxID())

	r.mu.Lock()
	defer r.mu.Unlock()

	// Track peer metadata for best-path comparison.
	r.peerMeta[peerAddr] = &PeerMeta{
		PeerASN:  se.PeerAS,
		LocalASN: se.LocalAS,
	}

	// Initialize PeerRIB if needed.
	if r.ribInPool[peerAddr] == nil {
		r.ribInPool[peerAddr] = storage.NewPeerRIB(peerAddr)
	}
	peerRIB := r.ribInPool[peerAddr]

	// Process IPv4 unicast announces (legacy NLRI section).
	ipv4Family := nlri.Family{AFI: 1, SAFI: 1}
	nlriData, err := wu.NLRI()
	if err == nil && len(nlriData) > 0 {
		addPath := ctx != nil && ctx.AddPath(ipv4Family)
		if isSimplePrefixFamilyNLRI(ipv4Family) {
			prefixes := splitNLRIs(nlriData, addPath)
			for _, wirePrefix := range prefixes {
				peerRIB.Insert(ipv4Family, attrBytes, wirePrefix)
			}
		}
	}

	// Process IPv4 unicast withdrawals (legacy Withdrawn section).
	wdData, err := wu.Withdrawn()
	if err == nil && len(wdData) > 0 {
		addPath := ctx != nil && ctx.AddPath(ipv4Family)
		if isSimplePrefixFamilyNLRI(ipv4Family) {
			withdrawns := splitNLRIs(wdData, addPath)
			for _, wd := range withdrawns {
				peerRIB.Remove(ipv4Family, wd)
			}
		}
	}

	// Process MP_REACH_NLRI announces (multiprotocol families).
	mpReach, err := wu.MPReach()
	if err == nil && mpReach != nil {
		family := mpReach.Family()
		if isSimplePrefixFamilyNLRI(family) {
			nlriBytes := mpReach.NLRIBytes()
			if len(nlriBytes) > 0 {
				addPath := ctx != nil && ctx.AddPath(family)
				prefixes := splitNLRIs(nlriBytes, addPath)
				for _, wirePrefix := range prefixes {
					peerRIB.Insert(family, attrBytes, wirePrefix)
				}
			}
		}
	}

	// Process MP_UNREACH_NLRI withdrawals (multiprotocol families).
	mpUnreach, err := wu.MPUnreach()
	if err == nil && mpUnreach != nil {
		family := mpUnreach.Family()
		if isSimplePrefixFamilyNLRI(family) {
			wdBytes := mpUnreach.WithdrawnBytes()
			if len(wdBytes) > 0 {
				addPath := ctx != nil && ctx.AddPath(family)
				withdrawns := splitNLRIs(wdBytes, addPath)
				for _, wd := range withdrawns {
					peerRIB.Remove(family, wd)
				}
			}
		}
	}
}

// handleSentStructured processes sent UPDATE events from wire types.
// Reads attributes lazily via AttrsWire.Get() per attribute type.
func (r *RIBManager) handleSentStructured(se *rpc.StructuredEvent) {
	peerAddr := se.PeerAddress
	msgID := se.MessageID

	if peerAddr == "" {
		return
	}

	msg, ok := se.RawMessage.(*bgptypes.RawMessage)
	if !ok || msg == nil || msg.WireUpdate == nil {
		return
	}

	wu := msg.WireUpdate

	// Get encoding context for add-path flags.
	ctx := bgpctx.Registry.Get(wu.SourceCtxID())

	// Extract parsed attributes lazily from AttrsWire.
	core := extractCoreAttrs(msg.AttrsWire)
	comm := extractCommunityAttrs(msg.AttrsWire)

	// Get raw attributes hex for route replay.
	var rawAttrsHex string
	if msg.AttrsWire != nil {
		rawAttrsHex = hex.EncodeToString(msg.AttrsWire.Packed())
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.ribOut[peerAddr] == nil {
		r.ribOut[peerAddr] = make(map[string]map[string]*Route)
	}

	// Process IPv4 unicast announces (legacy NLRI section).
	ipv4Family := nlri.Family{AFI: 1, SAFI: 1}
	nextHop := extractNextHop(msg.AttrsWire)
	nlriData, err := wu.NLRI()
	if err == nil && len(nlriData) > 0 {
		addPath := ctx != nil && ctx.AddPath(ipv4Family)
		familyStr := ipv4Family.String()
		r.storeSentNLRIs(peerAddr, familyStr, nlriData, addPath, msgID, nextHop,
			core.origin, core.asPath, core.med, core.localPref, comm.communities, comm.largeCommunities, comm.extCommunities,
			rawAttrsHex, se.Meta)
	}

	// Process IPv4 unicast withdrawals.
	wdData, err := wu.Withdrawn()
	if err == nil && len(wdData) > 0 {
		addPath := ctx != nil && ctx.AddPath(ipv4Family)
		familyStr := ipv4Family.String()
		r.removeSentNLRIs(peerAddr, familyStr, wdData, addPath)
	}

	// Process MP_REACH_NLRI announces.
	mpReach, err := wu.MPReach()
	if err == nil && mpReach != nil {
		family := mpReach.Family()
		familyStr := family.String()
		mpNextHop := mpReach.NextHop().String()
		nlriBytes := mpReach.NLRIBytes()
		if len(nlriBytes) > 0 {
			addPath := ctx != nil && ctx.AddPath(family)
			r.storeSentNLRIs(peerAddr, familyStr, nlriBytes, addPath, msgID, mpNextHop,
				core.origin, core.asPath, core.med, core.localPref, comm.communities, comm.largeCommunities, comm.extCommunities,
				rawAttrsHex, se.Meta)
		}
	}

	// Process MP_UNREACH_NLRI withdrawals.
	mpUnreach, err := wu.MPUnreach()
	if err == nil && mpUnreach != nil {
		family := mpUnreach.Family()
		familyStr := family.String()
		wdBytes := mpUnreach.WithdrawnBytes()
		if len(wdBytes) > 0 {
			addPath := ctx != nil && ctx.AddPath(family)
			r.removeSentNLRIs(peerAddr, familyStr, wdBytes, addPath)
		}
	}
}

// storeSentNLRIs walks NLRI bytes and stores Route entries in ribOut.
// Caller must hold write lock.
func (r *RIBManager) storeSentNLRIs(peerAddr, family string, nlriData []byte, addPath bool,
	msgID uint64, nextHop, origin string, asPath []uint32, med, localPref *uint32,
	communities, largeCommunities, extCommunities []string,
	rawAttrsHex string, meta map[string]any) {

	if r.ribOut[peerAddr][family] == nil {
		r.ribOut[peerAddr][family] = make(map[string]*Route)
	}

	iter := nlri.NewNLRIIterator(nlriData, addPath)
	for {
		wirePrefix, pathID, ok := iter.Next()
		if !ok {
			break
		}
		prefix := wirePrefixToString(wirePrefix, family)
		if prefix == "" {
			continue
		}
		key := outRouteKey(prefix, pathID)
		r.ribOut[peerAddr][family][key] = &Route{
			MsgID:               msgID,
			Family:              family,
			Prefix:              prefix,
			PathID:              pathID,
			NextHop:             nextHop,
			Origin:              origin,
			ASPath:              asPath,
			MED:                 med,
			LocalPreference:     localPref,
			Communities:         communities,
			LargeCommunities:    largeCommunities,
			ExtendedCommunities: extCommunities,
			RawAttrs:            rawAttrsHex,
			Meta:                meta,
		}
	}
}

// removeSentNLRIs walks NLRI bytes and removes Route entries from ribOut.
// Caller must hold write lock.
func (r *RIBManager) removeSentNLRIs(peerAddr, family string, wdData []byte, addPath bool) {
	familyRoutes := r.ribOut[peerAddr][family]
	if familyRoutes == nil {
		return
	}

	iter := nlri.NewNLRIIterator(wdData, addPath)
	for {
		wirePrefix, pathID, ok := iter.Next()
		if !ok {
			break
		}
		prefix := wirePrefixToString(wirePrefix, family)
		if prefix == "" {
			continue
		}
		key := outRouteKey(prefix, pathID)
		delete(familyRoutes, key)
	}

	if len(familyRoutes) == 0 {
		delete(r.ribOut[peerAddr], family)
	}
	if len(r.ribOut[peerAddr]) == 0 {
		delete(r.ribOut, peerAddr)
	}
}

// handleRefreshStructured processes refresh events from wire types.
func (r *RIBManager) handleRefreshStructured(se *rpc.StructuredEvent) {
	msg, ok := se.RawMessage.(*bgptypes.RawMessage)
	if !ok || msg == nil || msg.RawBytes == nil || len(msg.RawBytes) < 4 {
		return
	}

	// Route refresh wire: AFI (2) + reserved (1) + SAFI (1) = 4 bytes.
	afi := uint16(msg.RawBytes[0])<<8 | uint16(msg.RawBytes[1])
	safi := msg.RawBytes[3]
	family := nlri.Family{AFI: nlri.AFI(afi), SAFI: nlri.SAFI(safi)}.String()

	peerAddr := se.PeerAddress
	if peerAddr == "" {
		return
	}

	r.mu.RLock()
	if !r.peerUp[peerAddr] {
		r.mu.RUnlock()
		return
	}

	var routesToSend []*Route
	if familyRoutes := r.ribOut[peerAddr][family]; familyRoutes != nil {
		routesToSend = make([]*Route, 0, len(familyRoutes))
		for _, rt := range familyRoutes {
			routesToSend = append(routesToSend, rt)
		}
	}
	r.mu.RUnlock()

	r.updateRoute(peerAddr, "borr "+family)
	r.sendRoutes(peerAddr, routesToSend)
	r.updateRoute(peerAddr, "eorr "+family)
}

// coreAttrs holds parsed core path attributes from AttrsWire.
type coreAttrs struct {
	origin    string
	asPath    []uint32
	med       *uint32
	localPref *uint32
}

// extractCoreAttrs reads Origin, ASPath, MED, LocalPref from AttrsWire.
func extractCoreAttrs(attrs *attribute.AttributesWire) coreAttrs {
	var result coreAttrs
	if attrs == nil {
		return result
	}

	if attr, err := attrs.Get(attribute.AttrOrigin); err == nil && attr != nil {
		o, ok := attr.(attribute.Origin)
		if ok {
			switch o {
			case attribute.OriginIGP:
				result.origin = "igp"
			case attribute.OriginEGP:
				result.origin = "egp"
			case attribute.OriginIncomplete:
				result.origin = "incomplete"
			}
		}
	}

	if attr, err := attrs.Get(attribute.AttrASPath); err == nil && attr != nil {
		if asp, ok := attr.(*attribute.ASPath); ok {
			for _, seg := range asp.Segments {
				result.asPath = append(result.asPath, seg.ASNs...)
			}
		}
	}

	if attr, err := attrs.Get(attribute.AttrMED); err == nil && attr != nil {
		if m, ok := attr.(attribute.MED); ok {
			v := uint32(m)
			result.med = &v
		}
	}

	if attr, err := attrs.Get(attribute.AttrLocalPref); err == nil && attr != nil {
		if lp, ok := attr.(attribute.LocalPref); ok {
			v := uint32(lp)
			result.localPref = &v
		}
	}

	return result
}

// communityAttrs holds parsed community attributes from AttrsWire.
type communityAttrs struct {
	communities      []string
	largeCommunities []string
	extCommunities   []string
}

// extractCommunityAttrs reads community attributes from AttrsWire.
func extractCommunityAttrs(attrs *attribute.AttributesWire) communityAttrs {
	var result communityAttrs
	if attrs == nil {
		return result
	}

	if attr, err := attrs.Get(attribute.AttrCommunity); err == nil && attr != nil {
		if c, ok := attr.(attribute.Communities); ok {
			result.communities = make([]string, len(c))
			for i, comm := range c {
				result.communities[i] = comm.String()
			}
		}
	}

	if attr, err := attrs.Get(attribute.AttrLargeCommunity); err == nil && attr != nil {
		if lc, ok := attr.(attribute.LargeCommunities); ok {
			result.largeCommunities = make([]string, len(lc))
			for i, comm := range lc {
				result.largeCommunities[i] = comm.String()
			}
		}
	}

	if attr, err := attrs.Get(attribute.AttrExtCommunity); err == nil && attr != nil {
		if ec, ok := attr.(attribute.ExtendedCommunities); ok {
			result.extCommunities = make([]string, len(ec))
			for i, comm := range ec {
				result.extCommunities[i] = hex.EncodeToString(comm[:])
			}
		}
	}

	return result
}

// extractNextHop reads the NEXT_HOP attribute as string.
func extractNextHop(attrs *attribute.AttributesWire) string {
	if attrs == nil {
		return ""
	}
	attr, err := attrs.Get(attribute.AttrNextHop)
	if err != nil || attr == nil {
		return ""
	}
	nh, ok := attr.(*attribute.NextHop)
	if !ok {
		return ""
	}
	return nh.Addr.String()
}

// isSimplePrefixFamilyNLRI returns true for families with simple [prefix-len][prefix-bytes] format.
// Complex families (VPN, EVPN, FlowSpec) have different NLRI structures.
func isSimplePrefixFamilyNLRI(family nlri.Family) bool {
	s := family.String()
	return s == "ipv4/unicast" || s == "ipv4/multicast" ||
		s == "ipv6/unicast" || s == "ipv6/multicast"
}

// wirePrefixToString converts NLRI wire prefix bytes [prefix-len][prefix-bytes...] to "ip/len" string.
// Returns "" if the wire bytes are malformed.
// Uses stack-allocated [16]byte to avoid heap allocation.
func wirePrefixToString(wire []byte, family string) string {
	if len(wire) == 0 {
		return ""
	}
	prefixLen := int(wire[0])
	prefixBytes := wire[1:]
	byteCount := (prefixLen + 7) / 8
	if len(prefixBytes) < byteCount {
		return ""
	}

	// Stack-allocated buffer — large enough for IPv6 (16 bytes), zeroed by default.
	var buf [16]byte
	copy(buf[:], prefixBytes[:byteCount])

	isIPv6 := strings.HasPrefix(family, "ipv6")
	var addrLen int
	if isIPv6 {
		addrLen = 16
	} else {
		addrLen = 4
	}

	addr, ok := netip.AddrFromSlice(buf[:addrLen])
	if !ok {
		return ""
	}
	return netip.PrefixFrom(addr, prefixLen).String()
}
