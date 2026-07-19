// Design: docs/architecture/plugin/rib-storage-design.md — RIB plugin structured delivery
// RFC: rfc/short/rfc7313.md -- Enhanced Route Refresh (refresh subtype handling)
// Overview: rib.go — RIB plugin main entry and JSON dispatch
// Related: rib_bestchange.go — best-path change tracking and Bus publishing
//
// Structured event handlers for DirectBridge delivery.
// These handlers read from StructuredEvent metadata fields and RawMessage wire types
// instead of parsing JSON, eliminating the JSON round-trip for internal plugins.
package rib

import (
	"fmt"
	"net/netip"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attrpool"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/pool"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/storage"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/core/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/core/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/core/bgp/nlri/nlrisplit"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

var affectedPool = sync.Pool{
	New: func() any {
		s := make([]affectedPrefix, 0, 128)
		return &s
	},
}

// parsePeerAddr converts a peer address string to netip.Addr at the event /
// command boundary. The engine produces canonical netip.Addr.String() values
// (PeerInfo.AddrStr), so a failure means a malformed producer or operator
// input; the caller must log or return the error and stop (fail closed,
// never a zero-Addr map key).
func parsePeerAddr(peerAddr string) (netip.Addr, error) {
	addr, err := netip.ParseAddr(peerAddr)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("bgp rib: invalid peer address %q: %w (expected an IP address)", peerAddr, err)
	}
	return addr, nil
}

// dispatchStructured routes a StructuredEvent to the appropriate handler.
func (r *RIBManager) dispatchStructured(se *rpc.StructuredEvent) {
	switch se.EventType { //nolint:exhaustive // RIB handles update+state+refresh on structured path; borr/eorr are text-only
	case rpc.EventKindUpdate:
		if se.Direction == rpc.DirectionSent {
			r.handleSentStructured(se)
		} else {
			r.handleReceivedStructured(se)
		}
	case rpc.EventKindState:
		r.handleStructuredState(se)
	case rpc.EventKindRefresh:
		r.handleRefreshStructured(se)
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
	if se.PeerAddress == "" {
		return
	}
	peerAddr, err := parsePeerAddr(se.PeerAddress)
	if err != nil {
		logger().Warn("received structured event dropped", "error", err)
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

	// Wrap the UPDATE wire bytes in a locrib.ForwardHandle so Change
	// subscribers can retain the payload past this handler's return.
	// Subscribers that call AddRef inside their ChangeHandler trigger a
	// sync.Once copy of RawBytes into an owned buffer; subscriber-free
	// UPDATEs pay the handle alloc but no byte copy. Created outside
	// r.peerMu because it is lock-independent -- the lock protects RIB
	// state, not handle construction. Returns nil when RawBytes is
	// empty (InsertForward then dispatches Forward == nil).
	forward := newForwardHandle(msg.RawBytes)

	// Get encoding context for add-path flags and ASN4 capability.
	ctx := bgpctx.Registry.Get(wu.SourceCtxID())
	asn4 := ctx == nil || ctx.ASN4()

	// Track affected prefixes for best-path change detection.
	// Pooled to amortize the per-UPDATE allocation across calls.
	affectedPtr, _ := affectedPool.Get().(*[]affectedPrefix)
	if affectedPtr == nil {
		s := make([]affectedPrefix, 0, 128)
		affectedPtr = &s
	}
	affected := (*affectedPtr)[:0]
	defer func() {
		*affectedPtr = affected
		affectedPool.Put(affectedPtr)
	}()

	// Phase 1: lazily create the peer's slots under a brief write lock.
	// Only mutates the peer-keyed maps; the rest of UPDATE processing runs
	// outside peerMu so other peer goroutines can proceed concurrently.
	//
	// Race with handleStructuredState DOWN: if a DOWN event lands between
	// here and Phase 3, that handler takes r.peerMu.Lock, calls
	// peerRIB.Release, and delete(r.bgpPeers, peerAddr). Phase 2 below
	// keeps writing to the local peerRIB pointer -- those writes land on
	// an orphan PeerRIB that no future gatherCandidates sees. Semantics
	// stay correct because Phase 3's checkBestPathChange still emits
	// withdraws for every prefix whose best came from the now-absent
	// peer (newBest == nil, havePrev == true). The orphan writes are
	// wasted work, not lost state.
	r.peerMu.Lock()
	// Compare against the cached peerMetadata on a stack-local candidate so
	// rapid-flap sessions with unchanging (PeerASN, LocalASN, ContextID)
	// skip both the heap alloc AND the map write. Only when the struct
	// changes do we take the address (triggering the escape).
	candidate := peerMetadata{
		PeerASN:   se.PeerAS,
		LocalASN:  se.LocalAS,
		RouterID:  se.RouterID,
		ContextID: wu.SourceCtxID(),
	}
	if cur := r.peerMeta[peerAddr]; cur == nil || *cur != candidate {
		m := candidate
		r.peerMeta[peerAddr] = &m
	}
	peerRIB := r.bgpPeers[peerAddr]
	if peerRIB == nil {
		// Canonical string: PeerRIB.PeerAddr() feeds the best-path interner
		// and metric labels, which must match netip.Addr.String() everywhere.
		peerRIB = storage.NewPeerRIB(peerAddr.String())
		r.bgpPeers[peerAddr] = peerRIB
	}
	r.peerMu.Unlock()

	// Parse attributes once for all announcements in this UPDATE.
	// An UPDATE carries one attribute block shared by all announced NLRIs.
	// Parsing per-NLRI was the dominant RIB allocation site (~57% of space).
	var parsed storage.RouteEntry
	var fp uint64
	var attrLen uint32
	var haveParsed bool
	if len(attrBytes) > 0 {
		var parseErr error
		parsed, fp, attrLen, parseErr = storage.ParseRouteEntry(attrBytes, asn4)
		if parseErr == nil {
			haveParsed = true
			defer parsed.Release()
		}
	}

	// Process IPv4 unicast announces (legacy NLRI section).
	ipv4Family := family.Family{AFI: 1, SAFI: 1}
	nlriData, err := wu.NLRI()
	if err == nil && len(nlriData) > 0 {
		addPath := ctx != nil && ctx.AddPath(ipv4Family)
		// Record ADD-PATH for the family before the first insert so the FamilyRIB is
		// created with (prefix, path-id) keying. Mirrors the JSON path
		// (insertPoolNLRIs); without it the structured path would collapse ADD-PATH
		// siblings of a prefix (RFC 7911), silently dropping paths.
		if addPath {
			peerRIB.SetAddPath(ipv4Family, true)
		}
		if nlrisplit.Supported(ipv4Family) {
			prefixes, _ := nlrisplit.Split(ipv4Family, nlriData, addPath)
			for _, wirePrefix := range prefixes {
				if haveParsed {
					peerRIB.InsertEntry(ipv4Family, parsed, fp, attrLen, wirePrefix)
				} else {
					peerRIB.Insert(ipv4Family, attrBytes, wirePrefix, asn4)
				}
				affected = append(affected, affectedPrefix{fam: ipv4Family, nlriBytes: wirePrefix, addPath: addPath})
			}
		}
	}

	// Process IPv4 unicast withdrawals (legacy Withdrawn section).
	wdData, err := wu.Withdrawn()
	if err == nil && len(wdData) > 0 {
		addPath := ctx != nil && ctx.AddPath(ipv4Family)
		if nlrisplit.Supported(ipv4Family) {
			withdrawns, _ := nlrisplit.Split(ipv4Family, wdData, addPath)
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
		if nlrisplit.Supported(fam) {
			nlriBytes := mpReach.NLRIBytes()
			if len(nlriBytes) > 0 {
				addPath := ctx != nil && ctx.AddPath(fam)
				// Record ADD-PATH before the first insert so the FamilyRIB keys by
				// (prefix, path-id) (RFC 7911), mirroring the JSON insert path.
				if addPath {
					peerRIB.SetAddPath(fam, true)
				}
				prefixes, _ := nlrisplit.Split(fam, nlriBytes, addPath)
				isLabeled := fam.SAFI == family.SAFIMPLSLabel
				for _, wirePrefix := range prefixes {
					if isLabeled {
						if haveParsed {
							r.insertLabeledEntry(peerRIB, fam, parsed, fp, attrLen, wirePrefix, addPath, &affected)
						} else {
							r.insertLabeled(peerRIB, fam, attrBytes, wirePrefix, addPath, asn4, &affected)
						}
					} else {
						if haveParsed {
							peerRIB.InsertEntry(fam, parsed, fp, attrLen, wirePrefix)
						} else {
							peerRIB.Insert(fam, attrBytes, wirePrefix, asn4)
						}
						affected = append(affected, affectedPrefix{fam: fam, nlriBytes: wirePrefix, addPath: addPath})
					}
				}
			}
		}
	}

	// Process MP_UNREACH_NLRI withdrawals (multiprotocol families).
	mpUnreach, err := wu.MPUnreach()
	if err == nil && mpUnreach != nil {
		fam := mpUnreach.Family()
		if nlrisplit.Supported(fam) {
			wdBytes := mpUnreach.WithdrawnBytes()
			if len(wdBytes) > 0 {
				addPath := ctx != nil && ctx.AddPath(fam)
				withdrawns, _ := nlrisplit.Split(fam, wdBytes, addPath)
				isLabeled := fam.SAFI == family.SAFIMPLSLabel
				for _, wd := range withdrawns {
					if isLabeled {
						r.removeLabeled(peerRIB, fam, wd, addPath, &affected)
					} else {
						peerRIB.Remove(fam, wd)
						affected = append(affected, affectedPrefix{fam: fam, nlriBytes: wd, addPath: addPath})
					}
				}
			}
		}
	}

	// Phase 3: best-path change detection. Runs with no r.peerMu held --
	// gatherCandidates and bestCandidateNextHopAddr acquire peerMu.RLock
	// internally for their brief map reads. The sharded bestPrev and the
	// self-locking bestPathInterner handle their own concurrency.
	//
	// >99% of UPDATEs carry a single address family. Use a stack-local
	// slice for the common case; spill to a map only when a second family
	// appears.
	var singleFam family.Family
	var singleChanges []bestChangeEntry
	var multiFam map[family.Family][]bestChangeEntry

	for _, ap := range affected {
		change, ok := r.checkBestPathChange(ap.fam, ap.nlriBytes, ap.addPath, forward)
		if !ok {
			continue
		}
		if multiFam != nil {
			multiFam[ap.fam] = append(multiFam[ap.fam], change)
			continue
		}
		if singleChanges == nil {
			singleFam = ap.fam
			singleChanges = make([]bestChangeEntry, 0, len(affected))
			singleChanges = append(singleChanges, change)
			continue
		}
		if ap.fam == singleFam {
			singleChanges = append(singleChanges, change)
			continue
		}
		// Second family: spill to map.
		multiFam = make(map[family.Family][]bestChangeEntry, 2)
		multiFam[singleFam] = singleChanges
		multiFam[ap.fam] = append(multiFam[ap.fam], change)
		singleChanges = nil
	}

	if multiFam != nil {
		for fam, changes := range multiFam {
			publishBestChanges(changes, fam)
		}
	} else if len(singleChanges) > 0 {
		publishBestChanges(singleChanges, singleFam)
	}
}

// handleSentStructured processes sent UPDATE events from wire types.
// Interns wire attribute bytes into pool.RibOut for deduplication.
func (r *RIBManager) handleSentStructured(se *rpc.StructuredEvent) {
	msgID := se.MessageID

	if se.PeerAddress == "" {
		return
	}
	peerAddr, err := parsePeerAddr(se.PeerAddress)
	if err != nil {
		logger().Warn("sent structured event dropped", "error", err)
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

	// Intern wire attribute bytes once for the entire UPDATE.
	var attrHandle attrpool.Handle
	if msg.AttrsWire != nil {
		if packed := msg.AttrsWire.Packed(); len(packed) > 0 {
			attrHandle, _ = pool.RibOut.Intern(packed)
		}
	}

	sourcePeer := se.SourcePeerStr

	r.peerMu.Lock()
	defer r.peerMu.Unlock()

	if r.ribOut[peerAddr] == nil {
		r.ribOut[peerAddr] = make(map[family.Family]map[ribOutKey]ribOutEntry)
	}

	// Process IPv4 unicast announces (NLRI section).
	ipv4Family := family.IPv4Unicast
	nlriData, err := wu.NLRI()
	if err == nil && len(nlriData) > 0 {
		addPath := ctx != nil && ctx.AddPath(ipv4Family)
		r.storeSentEntries(peerAddr, ipv4Family, nlriData, addPath, msgID, attrHandle, sourcePeer)
	}

	// Process IPv4 unicast withdrawals.
	wdData, err := wu.Withdrawn()
	if err == nil && len(wdData) > 0 {
		addPath := ctx != nil && ctx.AddPath(ipv4Family)
		r.removeSentNLRIs(peerAddr, ipv4Family, wdData, addPath)
	}

	// Process MP_REACH_NLRI announces.
	mpReach, err := wu.MPReach()
	if err == nil && mpReach != nil {
		fam := mpReach.Family()
		nlriBytes := mpReach.NLRIBytes()
		if len(nlriBytes) > 0 {
			addPath := ctx != nil && ctx.AddPath(fam)
			r.storeSentEntries(peerAddr, fam, nlriBytes, addPath, msgID, attrHandle, sourcePeer)
		}
	}

	// Process MP_UNREACH_NLRI withdrawals.
	mpUnreach, err := wu.MPUnreach()
	if err == nil && mpUnreach != nil {
		fam := mpUnreach.Family()
		wdBytes := mpUnreach.WithdrawnBytes()
		if len(wdBytes) > 0 {
			addPath := ctx != nil && ctx.AddPath(fam)
			r.removeSentNLRIs(peerAddr, fam, wdBytes, addPath)
		}
	}

	// Release the initial intern ref (each stored entry holds its own AddRef).
	if attrHandle.IsValid() {
		_ = pool.RibOut.Release(attrHandle)
	}
}

// storeSentEntries walks NLRI bytes and stores ribOutEntry records in ribOut.
// Caller must hold write lock.
func (r *RIBManager) storeSentEntries(peerAddr netip.Addr, fam family.Family, nlriData []byte, addPath bool,
	msgID uint64, attrHandle attrpool.Handle, sourcePeer string) {

	if r.ribOut[peerAddr][fam] == nil {
		r.ribOut[peerAddr][fam] = make(map[ribOutKey]ribOutEntry)
	}

	iter := nlri.NewNLRIIterator(nlriData, addPath)
	for {
		wirePrefix, pathID, ok := iter.Next()
		if !ok {
			break
		}
		pfx, valid := nlri.WirePrefixToKey(wirePrefix, fam)
		if !valid {
			continue
		}
		key := ribOutKey{Prefix: pfx, PathID: pathID}
		_, existed := r.ribOut[peerAddr][fam][key]
		if existed {
			r.ribOut[peerAddr][fam][key].release()
		}
		if attrHandle.IsValid() {
			_ = pool.RibOut.AddRef(attrHandle)
		}
		r.ribOut[peerAddr][fam][key] = ribOutEntry{
			MsgID:      msgID,
			AttrHandle: attrHandle,
		}
		r.setRibOutSource(fam, key, sourcePeer, !existed)
	}
}

// removeSentNLRIs walks NLRI bytes and removes ribOutEntry records from ribOut.
// Caller must hold write lock.
func (r *RIBManager) removeSentNLRIs(peerAddr netip.Addr, fam family.Family, wdData []byte, addPath bool) {
	familyRoutes := r.ribOut[peerAddr][fam]
	if familyRoutes == nil {
		return
	}

	iter := nlri.NewNLRIIterator(wdData, addPath)
	for {
		wirePrefix, pathID, ok := iter.Next()
		if !ok {
			break
		}
		pfx, valid := nlri.WirePrefixToKey(wirePrefix, fam)
		if !valid {
			continue
		}
		key := ribOutKey{Prefix: pfx, PathID: pathID}
		if old, exists := familyRoutes[key]; exists {
			old.release()
			delete(familyRoutes, key)
		}
		r.releaseRibOutSource(fam, key)
	}

	if len(familyRoutes) == 0 {
		delete(r.ribOut[peerAddr], fam)
	}
	if len(r.ribOut[peerAddr]) == 0 {
		delete(r.ribOut, peerAddr)
	}
}

// handleRefreshStructured processes refresh events from wire types.
// RFC 7313: subtype 0 = normal refresh (replay routes), 1 = BoRR marker, 2 = EoRR marker.
func (r *RIBManager) handleRefreshStructured(se *rpc.StructuredEvent) {
	msg, ok := se.RawMessage.(*bgptypes.RawMessage)
	if !ok || msg == nil || msg.RawBytes == nil || len(msg.RawBytes) < 4 {
		return
	}

	// Route refresh wire: AFI (2) + subtype (1) + SAFI (1) = 4 bytes.
	afi := uint16(msg.RawBytes[0])<<8 | uint16(msg.RawBytes[1])
	subtype := msg.RawBytes[2]
	safi := msg.RawBytes[3]
	fam := family.Family{AFI: family.AFI(afi), SAFI: family.SAFI(safi)}

	if se.PeerAddress == "" {
		return
	}
	peerAddr, err := parsePeerAddr(se.PeerAddress)
	if err != nil {
		logger().Warn("refresh structured event dropped", "error", err)
		return
	}

	// Only subtype 0 (normal refresh) triggers route replay.
	if subtype != 0 {
		return
	}

	r.peerMu.RLock()
	if !r.peerUp[peerAddr] {
		r.peerMu.RUnlock()
		return
	}

	routesToSend := r.collectRibOutRoutes(peerAddr, fam)
	r.peerMu.RUnlock()

	// The RPC selector stays the original event string.
	var tb textbuf.Buffer
	r.dispatchPeerAction(se.PeerAddress, tb.Str("borr ").Str(fam.String()).String())
	r.sendRoutes(se.PeerAddress, routesToSend)
	r.dispatchPeerAction(se.PeerAddress, tb.Reset().Str("eorr ").Str(fam.String()).String())
}

// insertLabeled handles a single labeled unicast NLRI announce. It strips
// MPLS labels from the wire entry, stores the route under CIDR bytes (same
// as FRR's SAFI remap), and stores labels as side-data on the FamilyRIB.
func (r *RIBManager) insertLabeled(peerRIB *storage.PeerRIB, fam family.Family, attrBytes, wireEntry []byte, addPath, asn4 bool, affected *[]affectedPrefix) {
	labels, cidrBytes, err := nlrisplit.ExtractLabels(wireEntry, addPath)
	if err != nil || len(cidrBytes) == 0 {
		return
	}
	peerRIB.Insert(fam, attrBytes, cidrBytes, asn4)
	labelHandle := pool.InternLabels(labels)
	if !peerRIB.SetLabelsIfRouteExists(fam, cidrBytes, labelHandle) {
		if labelHandle.IsValid() {
			_ = pool.Labels.Release(labelHandle)
		}
		return
	}
	*affected = append(*affected, affectedPrefix{fam: fam, nlriBytes: cidrBytes, addPath: addPath})
}

// insertLabeledEntry handles a single labeled unicast NLRI announce using a
// pre-parsed RouteEntry. Same as insertLabeled but avoids re-parsing attributes.
func (r *RIBManager) insertLabeledEntry(peerRIB *storage.PeerRIB, fam family.Family, entry storage.RouteEntry, fp uint64, attrLen uint32, wireEntry []byte, addPath bool, affected *[]affectedPrefix) {
	labels, cidrBytes, err := nlrisplit.ExtractLabels(wireEntry, addPath)
	if err != nil || len(cidrBytes) == 0 {
		return
	}
	peerRIB.InsertEntry(fam, entry, fp, attrLen, cidrBytes)
	labelHandle := pool.InternLabels(labels)
	if !peerRIB.SetLabelsIfRouteExists(fam, cidrBytes, labelHandle) {
		if labelHandle.IsValid() {
			_ = pool.Labels.Release(labelHandle)
		}
		return
	}
	*affected = append(*affected, affectedPrefix{fam: fam, nlriBytes: cidrBytes, addPath: addPath})
}

// removeLabeled handles a single labeled unicast NLRI withdrawal.
func (r *RIBManager) removeLabeled(peerRIB *storage.PeerRIB, fam family.Family, wireEntry []byte, addPath bool, affected *[]affectedPrefix) {
	_, cidrBytes, err := nlrisplit.ExtractLabels(wireEntry, addPath)
	if err != nil || len(cidrBytes) == 0 {
		return
	}
	peerRIB.Remove(fam, cidrBytes)
	peerRIB.RemoveLabels(fam, cidrBytes)
	*affected = append(*affected, affectedPrefix{fam: fam, nlriBytes: cidrBytes, addPath: addPath})
}
