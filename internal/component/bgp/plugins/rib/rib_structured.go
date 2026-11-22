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
	"context"
	"fmt"
	"net/netip"
	"sync"

	"github.com/ze-software/ze/internal/component/bgp/attrpool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
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
	switch se.EventType {
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
	case rpc.EventKindUnspecified, rpc.EventKindOpen, rpc.EventKindNotification,
		rpc.EventKindKeepalive, rpc.EventKindEOR, rpc.EventKindBoRR, rpc.EventKindEoRR,
		rpc.EventKindSent, rpc.EventKindNegotiated, rpc.EventKindUpdateFiltered,
		rpc.EventKindCount:
		// These structured events do not change the RIB; BoRR/EoRR are text-only.
	default:
		// The plugin event set is open; unknown events do not change the RIB.
	}
}

// affectedPrefix tracks a prefix that was inserted or removed for best-path checking.
//
// withdraw says nlriBytes was split from a withdrawal, whose route key is read
// with the withdrawal framing (checkRouteBestChange).
type affectedPrefix struct {
	fam       family.Family
	nlriBytes []byte
	addPath   bool
	withdraw  bool
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
	committed := false
	valid := true
	defer func() {
		if !committed {
			r.failSourceProcessed(peerAddr, se.PeerOwner, se.ConnectionID)
		}
	}()
	// An UPDATE arrives only in Established, where the peer's AS is always
	// negotiated. One without it is a Ze defect, and iBGP or eBGP cannot be
	// decided for its routes, so it is refused rather than stored under AS 0.
	peerAS, peerASKnown := se.PeerAS.ASN()
	if !peerASKnown {
		logger().Warn("received UPDATE dropped: the peer's AS is not negotiated", "peer", se.PeerAddress)
		return
	}

	msg, ok := se.RawMessage.(*bgptypes.RawMessage)
	if !ok || msg == nil || msg.WireUpdate == nil {
		return
	}

	wu := msg.WireUpdate
	source, err := r.sourceLeases.acquire(context.Background(), peerAddr, se.PeerOwner, msg.SourceReceipt, se.ConnectionID)
	if err != nil {
		r.sourceLeases.reportFailures(context.Background())
		return
	}
	defer source.Release()
	if se.PeerOwner == 0 {
		return // A passive unrecorded event cannot select a source store.
	}

	// Get raw attribute bytes directly (no hex encode/decode).
	var attrBytes []byte
	if msg.AttrsWire != nil {
		attrBytes = msg.AttrsWire.Packed()
	} else {
		attributes, err := wu.Attrs()
		if err != nil {
			return
		}
		if attributes != nil {
			attrBytes = attributes.Packed()
		}
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

	// Get encoding context for add-path flags. The AS number width is no
	// longer read here: every received UPDATE reaches this handler with its
	// AS-path family already reconciled to four octets at ingest
	// (attribute.ReconcileASPathFamily, reached from Session.processMessage).
	ctx := bgpctx.Registry.Get(wu.SourceCtxID())

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
	// peerRIB.Release, and delete(r.bgpPeers, se.PeerOwner). Phase 2 below
	// keeps writing to the local peerRIB pointer -- those writes land on
	// an orphan PeerRIB that no future candidate gather sees. Semantics
	// stay correct because Phase 3's checkBestPathChange still emits
	// withdraws for every prefix whose best came from the now-absent
	// peer (newBest == nil, havePrev == true). The orphan writes are
	// wasted work, not lost state.
	peerRIB, admitted, err := r.admitReceivedPeer(se, peerAddr, peerAS, wu.SourceCtxID(), msg.MessageID)
	if err != nil {
		r.failAdvertise(err)
		return
	}
	if !admitted {
		return
	}
	defer func() {
		r.peerMu.RLock()
		current := r.bgpPeers[se.PeerOwner] == peerRIB
		r.peerMu.RUnlock()
		if !current {
			// A concurrent DOWN may have purged this store before our inserts.
			// Release those orphan inserts too; no candidate owns their lease.
			peerRIB.Release()
		}
	}()

	// WITHDRAWALS RUN FIRST, AND THE ORDER IS LOAD-BEARING.
	//
	// RFC 4271 Section 4.3: "An UPDATE message SHOULD NOT include the same address
	// prefix in the WITHDRAWN ROUTES and Network Layer Reachability Information
	// fields. However, a BGP speaker MUST be able to process UPDATE messages in
	// this form. A BGP speaker SHOULD treat an UPDATE message of this form as
	// though the WITHDRAWN ROUTES do not contain the address prefix."
	//
	// Removing before inserting is what makes the announce win: the withdrawal
	// lands on the previous state and the insert then re-adds the prefix, which is
	// indistinguishable from the withdrawal never having named it. Inserting first
	// left the prefix REMOVED, the opposite of what Section 4.3 asks for
	// (RFC4271-4.3-5, RFC4271-4.3-7).
	//
	// All withdrawals precede all announces, across both the legacy sections and
	// the multiprotocol ones, so the guarantee does not depend on which section
	// carried the prefix.
	ipv4Family := family.Family{AFI: 1, SAFI: 1}

	// Process IPv4 unicast withdrawals (legacy Withdrawn section).
	wdData, err := wu.Withdrawn()
	if err != nil {
		valid = false
	}
	if err == nil && len(wdData) > 0 {
		addPath := ctx != nil && ctx.AddPath(ipv4Family)
		if nlrisplit.Supported(ipv4Family) {
			withdrawns, splitErr := nlrisplit.Split(ipv4Family, wdData, addPath)
			if splitErr != nil {
				valid = false
			}
			for _, wd := range withdrawns {
				peerRIB.Remove(ipv4Family, wd)
				affected = append(affected, affectedPrefix{fam: ipv4Family, nlriBytes: wd, addPath: addPath})
			}
		}
	}

	// Process MP_UNREACH_NLRI withdrawals (multiprotocol families).
	mpUnreach, err := wu.MPUnreach()
	if err != nil {
		valid = false
	}
	if err == nil && mpUnreach != nil {
		fam := mpUnreach.Family()
		if nlrisplit.Supported(fam) {
			wdBytes := mpUnreach.WithdrawnBytes()
			if len(wdBytes) > 0 {
				addPath := ctx != nil && ctx.AddPath(fam)
				// RFC 8277 Section 2.4: a withdrawal frames its label field as
				// one Compatibility field, so it is split and read as such.
				withdrawns, splitErr := nlrisplit.SplitWithdrawn(fam, wdBytes, addPath)
				if splitErr != nil {
					valid = false
				}
				isLabeled := fam.SAFI == family.SAFIMPLSLabel
				for _, wd := range withdrawns {
					if isLabeled {
						r.removeLabeled(peerRIB, fam, wd, addPath, &affected)
					} else {
						peerRIB.Withdraw(fam, wd)
						affected = append(affected, affectedPrefix{fam: fam, nlriBytes: wd, addPath: addPath, withdraw: true})
					}
				}
			}
		}
	}

	// RFC 9494 Section 4.3: the GR ingress step judged this UPDATE a received
	// LLGR_STALE route from a neighbor Ze sent the LLGR Capability to. Written
	// on every insert below, the no-op branches included, so the mark follows
	// the latest UPDATE for the route.
	receivedStale := receivedLLGRStaleFromMeta(se.Meta)

	// The route clock is read once for this UPDATE and the reading handed to
	// every route it announces, so the inserts below do not read the clock per
	// route. insertLabeled, the unparsed labeled fallback, is the one
	// exception and says why. The reading is stored with a route the UPDATE
	// adds or changes, and an equal re-announcement keeps the stored one
	// (storage.FamilyRIB.InsertEntry).
	changedAt := r.routeClock.now()

	// Parse attributes once for all announcements in this UPDATE.
	// An UPDATE carries one attribute block shared by all announced NLRIs.
	// Parsing per-NLRI was the dominant RIB allocation site (~57% of space).
	var parsed storage.RouteEntry
	var fp uint64
	var attrLen uint32
	var haveParsed bool
	if len(attrBytes) > 0 {
		var parseErr error
		parsed, fp, attrLen, parseErr = storage.ParseRouteEntry(attrBytes)
		if parseErr != nil {
			valid = false
		}
		if parseErr == nil {
			haveParsed = true
			parsed.MsgID = msg.MessageID
			parsed.SourcePolicyEpoch = msg.SourcePolicyEpoch
			parsed.SourceReceipt = msg.SourceReceipt
			parsed.SetSourceReference(source)
			parsed.ReceivedLLGRStale = receivedStale
			parsed.ChangedAt = changedAt
			defer parsed.Release()
		}
	}

	// Process IPv4 unicast announces (legacy NLRI section).
	nlriData, err := wu.NLRI()
	if err != nil {
		valid = false
	}
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
			prefixes, splitErr := nlrisplit.Split(ipv4Family, nlriData, addPath)
			if splitErr != nil {
				valid = false
			}
			for _, wirePrefix := range prefixes {
				if haveParsed {
					peerRIB.InsertEntry(ipv4Family, parsed, fp, attrLen, wirePrefix)
				} else {
					peerRIB.InsertAt(ipv4Family, attrBytes, wirePrefix, changedAt)
				}
				peerRIB.ModifyFamilyEntry(ipv4Family, wirePrefix, func(entry *storage.RouteEntry) {
					entry.MsgID = msg.MessageID
					entry.SourcePolicyEpoch = msg.SourcePolicyEpoch
					entry.SourceReceipt = msg.SourceReceipt
					entry.SetSourceReference(source)
					entry.ReceivedLLGRStale = receivedStale
				})
				affected = append(affected, affectedPrefix{fam: ipv4Family, nlriBytes: wirePrefix, addPath: addPath})
			}
		}
	}

	// Process MP_REACH_NLRI announces (multiprotocol families).
	mpReach, err := wu.MPReach()
	if err != nil {
		valid = false
	}
	if err == nil && mpReach != nil {
		fam := mpReach.Family()
		if !nlrisplit.Supported(fam) {
			r.reportUnstorableFamily(peerRIB.PeerAddr(), fam)
		}
		if nlrisplit.Supported(fam) {
			nlriBytes := mpReach.NLRIBytes()
			if len(nlriBytes) > 0 {
				addPath := ctx != nil && ctx.AddPath(fam)
				// Record ADD-PATH before the first insert so the FamilyRIB keys by
				// (prefix, path-id) (RFC 7911), mirroring the JSON insert path.
				if addPath {
					peerRIB.SetAddPath(fam, true)
				}
				prefixes, splitErr := nlrisplit.Split(fam, nlriBytes, addPath)
				if splitErr != nil {
					valid = false
				}
				isLabeled := fam.SAFI == family.SAFIMPLSLabel
				for _, wirePrefix := range prefixes {
					if isLabeled {
						if haveParsed {
							r.insertLabeledEntry(peerRIB, fam, parsed, fp, attrLen, wirePrefix, addPath, receivedStale, &affected)
						} else {
							r.insertLabeled(peerRIB, fam, attrBytes, wirePrefix, addPath, receivedStale, msg.MessageID, msg.SourcePolicyEpoch, msg.SourceReceipt, &affected, source)
						}
					} else {
						if haveParsed {
							peerRIB.InsertEntry(fam, parsed, fp, attrLen, wirePrefix)
						} else {
							peerRIB.InsertAt(fam, attrBytes, wirePrefix, changedAt)
						}
						peerRIB.ModifyFamilyEntry(fam, wirePrefix, func(entry *storage.RouteEntry) {
							entry.MsgID = msg.MessageID
							entry.SourcePolicyEpoch = msg.SourcePolicyEpoch
							entry.SourceReceipt = msg.SourceReceipt
							entry.SetSourceReference(source)
							entry.ReceivedLLGRStale = receivedStale
						})
						affected = append(affected, affectedPrefix{fam: fam, nlriBytes: wirePrefix, addPath: addPath})
					}
				}
			}
		}
	}

	// Phase 3: best-path change detection. Runs with no r.peerMu held --
	// gatherPrefixCandidates, gatherKeyCandidates and bestCandidateNextHopAddr acquire peerMu.RLock
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
		change, ok := r.checkRouteBestChange(ap.fam, ap.nlriBytes, ap.addPath, ap.withdraw, forward)
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
	if valid {
		committed = true
		r.noteSourceProcessed(peerAddr, se.PeerOwner, se.ConnectionID, msg.MessageID)
	}
}

// admitReceivedPeer is Phase 1 of handleReceivedStructured: under one brief
// write lock it refreshes the peer's metadata and lazily creates its received
// store. admitted is false, with no error, when the RIB is stopping or the
// event's physical source is no longer current. A store that cannot be created
// is returned as err for the caller to report once the lock is released.
//
// The unlock is deferred because the caller's deferred failSourceProcessed
// takes peerMu.Lock on the same goroutine: a panic in this section that left
// the lock held would wedge that goroutine on its own lock rather than
// propagate. Caller MUST NOT hold peerMu.
func (r *RIBManager) admitReceivedPeer(se *rpc.StructuredEvent, peerAddr netip.Addr, peerAS uint32, contextID bgpctx.ContextID, messageID uint64) (peerRIB *storage.PeerRIB, admitted bool, err error) {
	r.peerMu.Lock()
	defer r.peerMu.Unlock()
	if r.stopping {
		return nil, false, nil
	}
	if !r.sourceUpdateCurrentLocked(se.PeerOwner, peerAddr, se.ConnectionID) {
		return nil, false, nil
	}
	// Compare against the cached peerMetadata on a stack-local candidate so
	// rapid-flap sessions with unchanging (PeerASN, LocalASN, ContextID)
	// skip both the heap alloc AND the map write. Only when the struct
	// changes do we take the address (triggering the escape).
	candidate := peerMetadata{
		ConnectionID:   se.ConnectionID,
		Address:        peerAddr,
		PeerASN:        peerAS,
		LocalASN:       se.LocalAS,
		RemoteRouterID: se.RemoteRouterID,
		ContextID:      contextID,
		GroupName:      se.PeerGroup,
		// This speaker's end of the session. Parsed here, on the same
		// changed-only path as the fields above, because it is what "itself"
		// means to RFC 4271 Section 5.1.3 (isSelfNextHop, rib_self_nexthop.go).
		// An address that does not parse leaves the zero Addr, which names no
		// address rather than a wrong one.
		LocalAddress: parseLocalAddr(se.LocalAddress),
	}
	r.copyRefreshSessionLocked(se.PeerOwner, &candidate)
	if cur := r.peerMeta[se.PeerOwner]; cur != nil {
		candidate.Session = cur.Session
		candidate.processedMsgID = r.peerProcessedLocked(se.PeerOwner, peerAddr, se.ConnectionID)
		candidate.processingFailed = r.peerProcessingFailedLocked(se.PeerOwner, peerAddr, se.ConnectionID)
		if cur.ConnectionID != 0 {
			candidate.ConnectionID = cur.ConnectionID
		}
	}
	if cur := r.peerMeta[se.PeerOwner]; cur == nil || *cur != candidate {
		m := candidate
		r.peerMeta[se.PeerOwner] = &m
		r.refreshSelfNextHopsLocked()
	}
	peerRIB = r.bgpPeers[se.PeerOwner]
	if peerRIB == nil {
		// Canonical string: PeerRIB.PeerAddr() feeds the best-path interner
		// and metric labels, which must match netip.Addr.String() everywhere.
		created, createErr := storage.NewAddressPeerRIB(peerAddr)
		if createErr != nil {
			return nil, false, createErr
		}
		peerRIB = created
		r.bgpPeers[se.PeerOwner] = peerRIB
	}
	// Noted before the inserts below: events reach this handler one at a
	// time, so no sent projection runs between here and the inserts, and a
	// lifecycle removal that fences in between leaves this fresh path alone.
	r.noteReceivedLocked(se.PeerOwner, messageID)
	return peerRIB, true, nil
}

// handleSentStructured processes sent UPDATE events from wire types.
// Interns wire attribute bytes into pool.RibOut for deduplication.
func (r *RIBManager) handleSentStructured(se *rpc.StructuredEvent) {
	msgID := se.MessageID
	// Replay and RIB-owned lifecycle writes are feedback, not new ownership.
	// Delayed feedback MUST NOT restore purged routes or their old attributes.
	if _, replay := se.Meta[metaKeyReplay]; replay {
		return
	}
	if _, lifecycle := se.Meta["rib-lifecycle"]; lifecycle {
		return
	}

	if se.PeerAddress == "" {
		return
	}
	address, err := parsePeerAddr(se.PeerAddress)
	if err != nil {
		logger().Warn("sent structured event dropped", "error", err)
		return
	}
	destinationOwner := se.PeerOwner
	if destinationOwner == 0 {
		return
	}

	// RFC 2918 Section 4: "Otherwise, the BGP speaker shall re-advertise to
	// that peer the Adj-RIB-Out of the <AFI, SAFI> carried in the message,
	// based on its outbound route filtering policy."
	// Config owns initial advertisement, but refresh needs its sent routes too.
	_, configStatic := se.Meta["config-static"]
	_, replay := se.Meta[metaKeyReplay]
	_, recovery := se.Meta[bgptypes.RecoveryWriteMeta]

	msg, ok := se.RawMessage.(*bgptypes.RawMessage)
	if !ok || msg == nil || msg.WireUpdate == nil {
		return
	}
	sourcePaths := msg.SentPathSources
	sourceOwner := msg.SourceOwner
	localOrigin := msg.SourceLocal

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
	r.recordSentDestinationLocked(destinationOwner, address)

	if r.ribOut[destinationOwner] == nil {
		r.ribOut[destinationOwner] = make(map[family.Family]map[ribOutKey]ribOutEntry)
	}

	// Withdrawals first here too, and for the same reason as the receive path above.
	// RFC 4271 Section 4.3 says an UPDATE naming one prefix in both WITHDRAWN ROUTES and
	// NLRI is treated as though WITHDRAWN did not name it (RFC4271-4.3-5,
	// RFC4271-4.3-7). This is the record of what Ze SENT, so storing before removing made
	// that record say "withdrawn" for a prefix the receiver will install: the two ends of
	// one session disagreeing about the same message.
	ipv4Family := family.IPv4Unicast

	// Process IPv4 unicast withdrawals.
	wdData, err := wu.Withdrawn()
	if err == nil && len(wdData) > 0 {
		addPath := ctx != nil && ctx.AddPath(ipv4Family)
		r.removeSentNLRIs(destinationOwner, ipv4Family, wdData, addPath)
	}

	// Process MP_UNREACH_NLRI withdrawals.
	mpUnreach, err := wu.MPUnreach()
	if err == nil && mpUnreach != nil {
		fam := mpUnreach.Family()
		wdBytes := mpUnreach.WithdrawnBytes()
		if len(wdBytes) > 0 {
			addPath := ctx != nil && ctx.AddPath(fam)
			r.removeSentNLRIs(destinationOwner, fam, wdBytes, addPath)
		}
	}

	// Process IPv4 unicast announces (NLRI section).
	var ipv4Ordinal uint32
	nlriData, err := wu.NLRI()
	if err == nil && len(nlriData) > 0 {
		addPath := ctx != nil && ctx.AddPath(ipv4Family)
		ipv4Ordinal = r.storeSentEntries(destinationOwner, ipv4Family, nlriData, addPath,
			ribOutEntry{MsgID: msgID, DestinationSession: se.DestinationSession, AttrHandle: attrHandle, ConfigStatic: configStatic, SourceMessageID: msg.SourceMessageID,
				SourcePolicyEpoch: msg.SourcePolicyEpoch,
				SourceID:          msg.SourceID, SourceOwner: sourceOwner, SourceConnectionID: msg.SourceConnectionID, LocalOrigin: localOrigin}, sourcePeer, replay, recovery, sourcePaths, 0)
	}

	// Process MP_REACH_NLRI announces.
	mpReach, err := wu.MPReach()
	if err == nil && mpReach != nil {
		fam := mpReach.Family()
		nlriBytes := mpReach.NLRIBytes()
		if len(nlriBytes) > 0 {
			addPath := ctx != nil && ctx.AddPath(fam)
			var ordinal uint32
			if fam == ipv4Family {
				ordinal = ipv4Ordinal
			}
			r.storeSentEntries(destinationOwner, fam, nlriBytes, addPath,
				ribOutEntry{MsgID: msgID, DestinationSession: se.DestinationSession, AttrHandle: attrHandle, ConfigStatic: configStatic, SourceMessageID: msg.SourceMessageID,
					SourcePolicyEpoch: msg.SourcePolicyEpoch,
					SourceID:          msg.SourceID, SourceOwner: sourceOwner, SourceConnectionID: msg.SourceConnectionID, LocalOrigin: localOrigin}, sourcePeer, replay, recovery, sourcePaths, ordinal)
		}
	}

	// Release the initial intern ref (each stored entry holds its own AddRef).
	if attrHandle.IsValid() {
		_ = pool.RibOut.Release(attrHandle)
	}
}

// storeSentEntries walks NLRI bytes and stores ribOutEntry records in ribOut.
// recovery marks the sent event of a recovery write: each stored entry then
// passes projectRecoveryWriteLocked. Any other write whose received message a
// lifecycle removal of its source followed passes projectForwardedWriteLocked.
// Caller must hold write lock.
func (r *RIBManager) storeSentEntries(destinationOwner uint64, fam family.Family, nlriData []byte, addPath bool,
	entry ribOutEntry, sourcePeer string, replay, recovery bool, sources []wireu.SentPathSource, ordinal uint32) uint32 {

	if replay {
		return ordinal
	}
	followed := !recovery && r.lifecycleFollowedLocked(entry.SourceOwner, entry.SourceMessageID)
	split := nlrisplit.Get(fam)
	if split == nil {
		logger().Warn("sent: unsupported NLRI family", "family", fam)
		return ordinal
	}
	if r.ribOut[destinationOwner] == nil {
		r.ribOut[destinationOwner] = make(map[family.Family]map[ribOutKey]ribOutEntry)
	}
	if r.ribOut[destinationOwner][fam] == nil {
		r.ribOut[destinationOwner][fam] = make(map[ribOutKey]ribOutEntry)
	}
	sourceIndex := 0
	_, err := split(nlriData, addPath, func(raw []byte) bool {
		stored := entry
		setSentPathSource(&stored, fam, ordinal, sources, &sourceIndex)
		ordinal++
		key, valid := ribOutRouteKey(fam, raw, addPath)
		if !valid {
			logger().Warn("sent: invalid NLRI key", "family", fam)
			return true
		}
		old, existed := r.ribOut[destinationOwner][fam][key]
		stored.AddPath = addPath
		stored.SourcePeer = sourcePeer
		if !key.Prefix.IsValid() {
			if key.Native == string(raw) {
				stored.NativeNLRI = key.Native
			} else {
				stored.NativeNLRI = string(raw)
			}
		}
		if existed {
			old.release()
		}
		if stored.AttrHandle.IsValid() {
			_ = pool.RibOut.AddRef(stored.AttrHandle)
		}
		r.ribOut[destinationOwner][fam][key] = stored
		if recovery {
			r.projectRecoveryWriteLocked(destinationOwner, fam, key)
		}
		if followed {
			r.projectForwardedWriteLocked(destinationOwner, fam, key)
		}
		return true
	})
	if err != nil {
		logger().Warn("sent: invalid NLRI framing", "family", fam, "error", err)
	}
	return ordinal
}

// removeSentNLRIs walks NLRI bytes and removes ribOutEntry records from ribOut.
// Caller must hold write lock.
func (r *RIBManager) removeSentNLRIs(destinationOwner uint64, fam family.Family, wdData []byte, addPath bool) {
	familyRoutes := r.ribOut[destinationOwner][fam]
	if familyRoutes == nil {
		return
	}

	split := nlrisplit.GetWithdraw(fam)
	if split == nil {
		logger().Warn("sent withdrawal: unsupported NLRI family", "family", fam)
		return
	}
	_, err := split(wdData, addPath, func(raw []byte) bool {
		key, valid := ribOutRouteKeyForAction(fam, raw, addPath, true)
		if !valid {
			logger().Warn("sent withdrawal: invalid NLRI key", "family", fam)
			return true
		}
		if old, exists := familyRoutes[key]; exists {
			old.release()
			delete(familyRoutes, key)
		}
		return true
	})
	if err != nil {
		logger().Warn("sent withdrawal: invalid NLRI framing", "family", fam, "error", err)
	}

	if len(familyRoutes) == 0 {
		delete(r.ribOut[destinationOwner], fam)
	}
	if len(r.ribOut[destinationOwner]) == 0 {
		delete(r.ribOut, destinationOwner)
	}
}

// handleRefreshStructured processes refresh events from wire types.
// RFC 7313: subtype 0 = normal refresh (replay routes), 1 = BoRR marker, 2 = EoRR marker.
func (r *RIBManager) handleRefreshStructured(se *rpc.StructuredEvent) {
	msg, ok := se.RawMessage.(*bgptypes.RawMessage)
	if !ok || msg == nil || len(msg.RawBytes) < 4 {
		return
	}
	// The captured message retains its wire bytes. Only effective normal
	// requests belong here; RFC2918-only reserved normalization is upstream.
	if msg.RawBytes[2] != 0 {
		return
	}
	fam := family.Family{AFI: family.AFI(uint16(msg.RawBytes[0])<<8 | uint16(msg.RawBytes[1])),
		SAFI: family.SAFI(msg.RawBytes[3])}
	r.queueSelectedRefresh(se.PeerAddress, se.PeerOwner,
		se.DestinationSession, se.SelectedDestination, fam)
}

// insertLabeled handles a single labeled unicast NLRI announce. It strips
// MPLS labels from the wire entry, stores the route under CIDR bytes (same
// as FRR's SAFI remap), and stores labels as side-data on the FamilyRIB.
// receivedStale is written on the stored route whichever insert branch ran, as
// insertLabeledEntry does (RFC 9494 Section 4.3). message is the received
// message ID, written as the unlabeled branch writes it: a stale ID would make
// R-12 judge a live path gone (sourcePathGone). Source provenance is copied
// from the admitted event, never from the source peer's current metadata.
func (r *RIBManager) insertLabeled(peerRIB *storage.PeerRIB, fam family.Family, attrBytes, wireEntry []byte, addPath, receivedStale bool, message, sourcePolicyEpoch, sourceReceipt uint64, affected *[]affectedPrefix, source *storage.SourceReference) {
	labels, cidrBytes, err := nlrisplit.ExtractLabels(wireEntry, addPath)
	if err != nil || len(cidrBytes) == 0 {
		return
	}
	// The route clock is read here, per route, and not handed down from the
	// handler: this is the fallback for an UPDATE whose attributes were not
	// parsed once, which already parses them again for each route, and the
	// parsed path (insertLabeledEntry) carries the handler's one reading.
	peerRIB.InsertAt(fam, attrBytes, cidrBytes, r.routeClock.now())
	peerRIB.ModifyFamilyEntry(fam, cidrBytes, func(stored *storage.RouteEntry) {
		stored.MsgID = message
		stored.SourcePolicyEpoch = sourcePolicyEpoch
		stored.SourceReceipt = sourceReceipt
		stored.SetSourceReference(source)
		stored.ReceivedLLGRStale = receivedStale
	})
	labelHandle := pool.InternLabels(labels)
	if !peerRIB.SetLabelsIfRouteExists(fam, cidrBytes, labelHandle) {
		if labelHandle.IsValid() {
			_ = pool.Labels.Release(labelHandle)
		}
		return
	}
	*affected = append(*affected, affectedPrefix{fam: fam, nlriBytes: cidrBytes, addPath: addPath})
}

// receivedLLGRStaleFromMeta reads the stale level the GR ingress step wrote for
// a received UPDATE (gr_ingress.go LLGRIngressFilter). A missing key, a nil map
// and a level below the depreference threshold all answer false: only that step
// writes the key on a received event, and it writes the threshold or nothing.
// The structured event carries the uint8 the step wrote; a JSON event's
// route-meta decodes it as a float64.
func receivedLLGRStaleFromMeta(meta map[string]any) bool {
	switch level := meta["stale"].(type) {
	case uint8:
		return level >= storage.DepreferenceThreshold
	case float64:
		return level >= float64(storage.DepreferenceThreshold)
	}
	return false
}

// insertLabeledEntry handles a single labeled unicast NLRI announce using a
// pre-parsed RouteEntry. Same as insertLabeled but avoids re-parsing attributes.
// receivedStale is written on the stored route whichever insert branch ran, as
// the unlabeled path does (RFC 9494 Section 4.3).
func (r *RIBManager) insertLabeledEntry(peerRIB *storage.PeerRIB, fam family.Family, entry storage.RouteEntry, fp uint64, attrLen uint32, wireEntry []byte, addPath, receivedStale bool, affected *[]affectedPrefix) {
	labels, cidrBytes, err := nlrisplit.ExtractLabels(wireEntry, addPath)
	if err != nil || len(cidrBytes) == 0 {
		return
	}
	peerRIB.InsertEntry(fam, entry, fp, attrLen, cidrBytes)
	peerRIB.ModifyFamilyEntry(fam, cidrBytes, func(stored *storage.RouteEntry) {
		stored.ReceivedLLGRStale = receivedStale
	})
	labelHandle := pool.InternLabels(labels)
	if !peerRIB.SetLabelsIfRouteExists(fam, cidrBytes, labelHandle) {
		if labelHandle.IsValid() {
			_ = pool.Labels.Release(labelHandle)
		}
		return
	}
	*affected = append(*affected, affectedPrefix{fam: fam, nlriBytes: cidrBytes, addPath: addPath})
}

// removeLabeled handles a single labeled unicast NLRI withdrawal, wireEntry
// framed by nlrisplit.SplitWithdrawn. The route goes with its label binding,
// and the election runs on the prefix the route was stored under.
//
// RFC 8277 Section 2.4: "Upon reception, the value of the Compatibility field
// MUST be ignored." storage.LabeledWithdrawnPrefix skips it whatever it holds.
func (r *RIBManager) removeLabeled(peerRIB *storage.PeerRIB, fam family.Family, wireEntry []byte, addPath bool, affected *[]affectedPrefix) {
	prefixNLRI, ok := storage.LabeledWithdrawnPrefix(fam, wireEntry, addPath, nil)
	if !ok {
		return
	}
	peerRIB.Remove(fam, prefixNLRI)
	*affected = append(*affected, affectedPrefix{fam: fam, nlriBytes: prefixNLRI, addPath: addPath})
}
