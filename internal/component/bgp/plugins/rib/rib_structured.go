// Design: docs/architecture/plugin/rib-storage-design.md — RIB plugin structured delivery
// Overview: rib.go — RIB plugin main entry and JSON dispatch
// Related: rib_bestchange.go — best-path change tracking and Bus publishing
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
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// dispatchStructured routes a StructuredEvent to the appropriate handler.
func (r *RIBManager) dispatchStructured(se *rpc.StructuredEvent) {
	switch se.EventType {
	case "update":
		if se.Direction == "sent" {
			r.handleSentStructured(se)
		} else {
			r.handleReceivedStructured(se)
		}
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

// affectedPrefix tracks a prefix that was inserted or removed for best-path checking.
type affectedPrefix struct {
	fam       family.Family
	nlriBytes []byte
	addPath   bool
}

// handleReceivedStructured processes received UPDATE events from wire types.
// Reads raw bytes directly from WireUpdate sections -- no hex encode/decode round-trip.
// After all inserts/removes, checks best-path changes for affected prefixes and
// publishes a batch event to the Bus (collected under lock, published after release).
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

	// Track affected prefixes for best-path change detection. Preallocate
	// for a typical stress-sized UPDATE; cap 16 left ~70 MB of regrowth on
	// the profile, cap 128 covers the common case without over-allocating
	// the small-UPDATE path (entries are ~40 bytes each).
	affected := make([]affectedPrefix, 0, 128)

	r.mu.Lock()
	locked := true
	defer func() {
		if locked {
			r.mu.Unlock()
		}
	}()

	// Track peer metadata for best-path comparison and capability lookup.
	r.peerMeta[peerAddr] = &PeerMeta{
		PeerASN:   se.PeerAS,
		LocalASN:  se.LocalAS,
		ContextID: wu.SourceCtxID(),
	}

	// Initialize PeerRIB if needed.
	if r.ribInPool[peerAddr] == nil {
		r.ribInPool[peerAddr] = storage.NewPeerRIB(peerAddr)
	}
	peerRIB := r.ribInPool[peerAddr]

	// Process IPv4 unicast announces (legacy NLRI section).
	ipv4Family := family.Family{AFI: 1, SAFI: 1}
	nlriData, err := wu.NLRI()
	if err == nil && len(nlriData) > 0 {
		addPath := ctx != nil && ctx.AddPath(ipv4Family)
		if isSimplePrefixFamilyNLRI(ipv4Family) {
			prefixes := splitNLRIs(nlriData, addPath)
			for _, wirePrefix := range prefixes {
				peerRIB.Insert(ipv4Family, attrBytes, wirePrefix)
				affected = append(affected, affectedPrefix{fam: ipv4Family, nlriBytes: wirePrefix, addPath: addPath})
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
				affected = append(affected, affectedPrefix{fam: ipv4Family, nlriBytes: wd, addPath: addPath})
			}
		}
	}

	// Process MP_REACH_NLRI announces (multiprotocol families).
	mpReach, err := wu.MPReach()
	if err == nil && mpReach != nil {
		fam := mpReach.Family()
		if isSimplePrefixFamilyNLRI(fam) {
			nlriBytes := mpReach.NLRIBytes()
			if len(nlriBytes) > 0 {
				addPath := ctx != nil && ctx.AddPath(fam)
				prefixes := splitNLRIs(nlriBytes, addPath)
				for _, wirePrefix := range prefixes {
					peerRIB.Insert(fam, attrBytes, wirePrefix)
					affected = append(affected, affectedPrefix{fam: fam, nlriBytes: wirePrefix, addPath: addPath})
				}
			}
		}
	}

	// Process MP_UNREACH_NLRI withdrawals (multiprotocol families).
	mpUnreach, err := wu.MPUnreach()
	if err == nil && mpUnreach != nil {
		fam := mpUnreach.Family()
		if isSimplePrefixFamilyNLRI(fam) {
			wdBytes := mpUnreach.WithdrawnBytes()
			if len(wdBytes) > 0 {
				addPath := ctx != nil && ctx.AddPath(fam)
				withdrawns := splitNLRIs(wdBytes, addPath)
				for _, wd := range withdrawns {
					peerRIB.Remove(fam, wd)
					affected = append(affected, affectedPrefix{fam: fam, nlriBytes: wd, addPath: addPath})
				}
			}
		}
	}

	// Check best-path changes for all affected prefixes (under lock).
	// Group changes by family so each family gets its own batch with correct metadata.
	// Preallocate the per-family slices with len(affected) -- all changes in a single
	// UPDATE almost always belong to one family, so one grow-free append per batch.
	changesByFamily := make(map[string][]bestChangeEntry)
	for _, ap := range affected {
		change, ok := r.checkBestPathChange(ap.fam, ap.nlriBytes, ap.addPath)
		if !ok {
			continue
		}
		familyStr := ap.fam.String()
		slice, seen := changesByFamily[familyStr]
		if !seen {
			slice = make([]bestChangeEntry, 0, len(affected))
		}
		changesByFamily[familyStr] = append(slice, change)
	}

	r.mu.Unlock()
	locked = false

	// Publish one batch per family after lock release.
	for famName, changes := range changesByFamily {
		publishBestChanges(changes, famName)
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

	// Skip config-static routes: they are always re-sent from config on
	// reconnection by sendInitialRoutes. Storing them in ribOut would cause
	// duplicates (config re-send + RIB replay).
	if se.Meta != nil {
		if _, isConfigStatic := se.Meta["config-static"]; isConfigStatic {
			return
		}
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
	ipv4Family := family.Family{AFI: 1, SAFI: 1}
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
		fam := mpReach.Family()
		familyStr := fam.String()
		mpNextHop := mpReach.NextHop().String()
		nlriBytes := mpReach.NLRIBytes()
		if len(nlriBytes) > 0 {
			addPath := ctx != nil && ctx.AddPath(fam)
			r.storeSentNLRIs(peerAddr, familyStr, nlriBytes, addPath, msgID, mpNextHop,
				core.origin, core.asPath, core.med, core.localPref, comm.communities, comm.largeCommunities, comm.extCommunities,
				rawAttrsHex, se.Meta)
		}
	}

	// Process MP_UNREACH_NLRI withdrawals.
	mpUnreach, err := wu.MPUnreach()
	if err == nil && mpUnreach != nil {
		fam := mpUnreach.Family()
		familyStr := fam.String()
		wdBytes := mpUnreach.WithdrawnBytes()
		if len(wdBytes) > 0 {
			addPath := ctx != nil && ctx.AddPath(fam)
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
	famStr := family.Family{AFI: family.AFI(afi), SAFI: family.SAFI(safi)}.String()

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
	if familyRoutes := r.ribOut[peerAddr][famStr]; familyRoutes != nil {
		routesToSend = make([]*Route, 0, len(familyRoutes))
		for _, rt := range familyRoutes {
			routesToSend = append(routesToSend, rt)
		}
	}
	r.mu.RUnlock()

	r.updateRoute(peerAddr, "borr "+famStr)
	r.sendRoutes(peerAddr, routesToSend)
	r.updateRoute(peerAddr, "eorr "+famStr)
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
func isSimplePrefixFamilyNLRI(fam family.Family) bool {
	s := fam.String()
	return s == "ipv4/unicast" || s == "ipv4/multicast" ||
		s == "ipv6/unicast" || s == "ipv6/multicast"
}

// wirePrefixToString converts NLRI wire prefix bytes [prefix-len][prefix-bytes...] to "ip/len" string.
// Returns "" if the wire bytes are malformed.
// Uses stack-allocated [16]byte to avoid heap allocation.
//
// Does not bounds-check prefixLen against the family maximum (32 for IPv4,
// 128 for IPv6). That validation lives at the wire layer: RFC 7606 Section
// 5.3 requires treat-as-withdraw for over-length prefixes, enforced in
// internal/component/bgp/message/rfc7606.go during UPDATE parse. By the time
// bytes reach this helper they are guaranteed in-range, so an asymmetry with
// store.NLRIToPrefix (which rejects over-length) is unreachable in
// practice.
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
