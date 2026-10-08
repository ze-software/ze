// Design: docs/architecture/core-design.md -- peer-down route inventory for route server
// Overview: server.go -- route server plugin orchestration
// Related: server_withdrawal.go -- withdrawal map management and NLRI walking

package rs

import (
	"net/netip"
	"sync"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// nlriRecord is a compact representation of one NLRI extracted from a wire
// UPDATE before forwarding. For unicast families, prefix is set (16 bytes,
// no allocation). For non-unicast families, nlriStr is set (allocating but
// rare in the grouped-input benchmark). wireForm and addPath carry the same
// meaning as on withdrawalKey (server.go).
type nlriRecord struct {
	fam        family.Family
	familyName string
	action     string // actionAdd or actionDel
	prefix     netip.Prefix
	nlriStr    string // non-empty only for non-unicast families
	nativeKey  string // registered VPN identity, excluding label/Compatibility
	wireForm   bool   // nlriStr is hex of one NLRI, not a text token
	addPath    bool   // wireForm hex carries a 4-octet path identifier
	// cidrKeyed says the route is keyed by prefix and pathID, because its
	// family names routes by a CIDR (nlrisplit.KeysByCIDR). An announcement
	// keeps its hex in nlriStr for the peer-down withdrawal.
	cidrKeyed bool
	pathID    uint32
}

// nlriRecords holds one extraction's records and CIDR scratch together so the
// registered CIDR decoder cannot make a scratch allocation for each NLRI.
type nlriRecords struct {
	records []nlriRecord
	scratch [nlrisplit.PrefixKeyScratchSize]byte
}

// nlriRecordPool amortizes records and scratch allocation for NLRI extraction.
// Typical grouped UPDATEs carry 100-200 IPv4 prefixes; initial capacity 256
// covers the common case without resize.
var nlriRecordPool = sync.Pool{
	New: func() any {
		return &nlriRecords{records: make([]nlriRecord, 0, 256)}
	},
}

// extractWireNLRIRecords extracts compact NLRI records from a raw wire UPDATE.
// MUST be called BEFORE forwarding (buffer lifetime safety: cache eviction can
// free the pool buffer backing msg.WireUpdate after ForwardCached).
// The caller MUST call returnNLRIRecords when done with the pooled handle.
func extractWireNLRIRecords(msg *bgptypes.RawMessage) *nlriRecords {
	if msg.WireUpdate == nil {
		return nil
	}
	wu := msg.WireUpdate

	sp, ok := nlriRecordPool.Get().(*nlriRecords)
	if !ok {
		return nil
	}
	sp.records = sp.records[:0]

	var encCtx *bgpctx.EncodingContext
	if msg.AttrsWire != nil {
		encCtx = bgpctx.Registry.Get(msg.AttrsWire.SourceContext())
	}

	// Withdrawn records are appended BEFORE announced ones, and the order is
	// load-bearing: every consumer walks this slice in order, and
	// updateWithdrawalMapText applies each record to a prefix-keyed map. RFC 4271
	// Section 4.3 says an UPDATE naming one prefix in both WITHDRAWN ROUTES and NLRI is
	// treated as though WITHDRAWN did not name it, so the "add" has to land last
	// (RFC4271-4.3-5, RFC4271-4.3-7). Appending adds first deleted that prefix from the
	// map, so a route the peer is still announcing would never be withdrawn when the
	// peer goes down.
	addPathV4 := encCtx != nil && encCtx.AddPath(family.IPv4Unicast)

	// MP_UNREACH_NLRI -- withdrawn routes.
	if mp, err := wu.MPUnreach(); err == nil && mp != nil {
		fam := mp.Family()
		addPath := encCtx != nil && encCtx.AddPath(fam)
		if isUnicast(fam) {
			sp.records = appendUnicastRecords(sp.records, fam, fam.String(), mp.NLRIIterator(addPath), actionDel)
		} else {
			nlris, nlriErr := mp.NLRIs(addPath)
			sp.records = appendAllocatingUnreachRecords(sp.records, fam, nlris, nlriErr, sp.scratch[:])
		}
	}

	// IPv4 body Withdrawn -- withdrawn routes.
	if iter, err := wu.WithdrawnIterator(addPathV4); err == nil && iter != nil {
		sp.records = appendUnicastRecords(sp.records, family.IPv4Unicast, "ipv4/unicast", iter, actionDel)
	}

	// MP_REACH_NLRI -- announced routes.
	if mp, err := wu.MPReach(); err == nil && mp != nil {
		fam := mp.Family()
		addPath := encCtx != nil && encCtx.AddPath(fam)
		if isUnicast(fam) {
			sp.records = appendUnicastRecords(sp.records, fam, fam.String(), mp.NLRIIterator(addPath), actionAdd)
		} else {
			sp.records = appendAllocatingRecords(sp.records, fam, mp, addPath, actionAdd, sp.scratch[:])
		}
	}

	// IPv4 body NLRIs -- announced routes.
	if iter, err := wu.NLRIIterator(addPathV4); err == nil && iter != nil {
		sp.records = appendUnicastRecords(sp.records, family.IPv4Unicast, "ipv4/unicast", iter, actionAdd)
	}

	return sp
}

// returnNLRIRecords MUST be called after extractWireNLRIRecords. The caller MUST
// stop using the handle and its records before returning it to the pool.
func returnNLRIRecords(sp *nlriRecords) {
	if sp == nil {
		return
	}
	sp.records = sp.records[:0]
	nlriRecordPool.Put(sp)
}

// appendUnicastRecords appends compact prefix records from an NLRIIterator.
// Uses netip.PrefixFrom for zero-allocation prefix extraction.
func appendUnicastRecords(records []nlriRecord, f family.Family, famName string, iter *nlri.NLRIIterator, action string) []nlriRecord {
	if iter == nil {
		return records
	}
	isV6 := famName == "ipv6/unicast" || famName == family.IPv6Unicast.String()
	for {
		raw, _, ok := iter.Next()
		if !ok {
			break
		}
		if len(raw) == 0 {
			continue
		}
		bitLen := int(raw[0])
		addrBytes := raw[1:]
		var p netip.Prefix
		if isV6 {
			var addr [16]byte
			copy(addr[:], addrBytes)
			p = netip.PrefixFrom(netip.AddrFrom16(addr), bitLen)
		} else {
			var addr [4]byte
			copy(addr[:], addrBytes)
			p = netip.PrefixFrom(netip.AddrFrom4(addr), bitLen)
		}
		records = append(records, nlriRecord{
			fam:        f,
			familyName: famName,
			action:     action,
			prefix:     p.Masked(),
		})
	}
	return records
}

// appendAllocatingRecords appends records for non-unicast MP_REACH families.
// Falls back to NLRIs() which allocates -- acceptable for rare non-unicast traffic.
func appendAllocatingRecords(records []nlriRecord, fam family.Family, mp interface {
	NLRIs(bool) ([]nlri.NLRI, error)
}, addPath bool, action string, scratch []byte) []nlriRecord {
	nlris, err := mp.NLRIs(addPath)
	if err != nil || len(nlris) == 0 {
		return records
	}
	return appendParsedRecords(records, fam, nlris, action, scratch)
}

// appendAllocatingUnreachRecords appends records for non-unicast MP_UNREACH families.
func appendAllocatingUnreachRecords(records []nlriRecord, fam family.Family, nlris []nlri.NLRI, err error, scratch []byte) []nlriRecord {
	if err != nil || len(nlris) == 0 {
		return records
	}
	return appendParsedRecords(records, fam, nlris, actionDel, scratch)
}

// appendParsedRecords turns parsed NLRIs into inventory records.
//
// An NLRI ze parses has a text spelling, and String() is it. An NLRI ze does
// NOT parse arrives as *nlri.WireNLRI, and its String() is a size summary
// ("wire[bgp-ls/bgp-ls](23 bytes)") that carries none of the bytes: it can
// neither identify the route in the withdrawal set nor be re-parsed by any
// command grammar. Those go in as hex instead, and their withdrawal goes out
// as "update hex" (sendBatchedWithdrawals).
func appendParsedRecords(records []nlriRecord, fam family.Family, nlris []nlri.NLRI, action string, scratch []byte) []nlriRecord {
	famStr := fam.String()
	cidrKeyed := nlrisplit.KeysByCIDR(fam)
	for _, n := range nlris {
		if w, ok := n.(*nlri.WireNLRI); ok {
			records = appendOpaqueRecords(records, fam, famStr, w, action, cidrKeyed, scratch)
			continue
		}
		// A withdrawal of a CIDR-keyed family arrives as an INET of the prefix
		// it names (wireu.ParseWithdrawnNLRIs), and is keyed the way
		// appendOpaqueRecords keys its announcement.
		if inet, ok := n.(*nlri.INET); ok && cidrKeyed {
			records = append(records, nlriRecord{
				fam:        fam,
				familyName: famStr,
				action:     action,
				prefix:     inet.Prefix(),
				pathID:     inet.PathID(),
				cidrKeyed:  true,
			})
			continue
		}
		records = append(records, nlriRecord{
			fam:        fam,
			familyName: famStr,
			action:     action,
			nlriStr:    n.String(),
		})
	}
	return records
}

// appendOpaqueRecords retains one native route already framed by wireu.ParseNLRIs.
// The registered framer validates that the carrier contains exactly one route;
// unsupported, malformed, or concatenated carriers cannot become inventory keys.
//
// The hex is a copy, which the buffer lifetime requires: the caller runs before
// ForwardCached and the wire buffer can be freed after it.
//
// A family that names its routes by a CIDR (nlrisplit.RouteCIDR) is keyed by
// that prefix and the path identifier rather than the hex, so its withdrawal,
// which carries a Compatibility field where this carried a label stack, keys
// the same (RFC 8277 Section 2.4), and a relabel replaces the entry (Section
// 2.5). The hex stays in the record: the peer-down withdrawal is sent as it.
func appendOpaqueRecords(records []nlriRecord, fam family.Family, famStr string, w *nlri.WireNLRI, action string, cidrKeyed bool, scratch []byte) []nlriRecord {
	data := w.Bytes()
	if len(data) == 0 {
		return records
	}
	addPath := w.HasAddPath()
	walk := nlrisplit.Get(fam)
	if action == actionDel {
		// RFC 8277 Section 2.4: a withdrawal carries Compatibility, not labels.
		walk = nlrisplit.GetWithdraw(fam)
	}
	if walk == nil {
		logger().Warn("opaque NLRI inventory has no registered framer", "family", famStr)
		return records
	}
	count, err := walk(data, addPath, nil)
	if err != nil {
		logger().Warn("opaque NLRI inventory framing rejected", "family", famStr, "error", err)
		return records
	}
	if count != 1 {
		logger().Warn("opaque NLRI inventory requires one framed route", "family", famStr, "count", count)
		return records
	}

	var tb textbuf.Buffer
	rec := nlriRecord{
		fam:        fam,
		familyName: famStr,
		action:     action,
		nlriStr:    tb.Hex(data).String(),
		wireForm:   true,
		addPath:    addPath,
	}
	if cidrKeyed {
		rec.prefix, rec.pathID, rec.cidrKeyed = opaqueRouteCIDR(fam, data, addPath, scratch)
	} else if fam.SAFI == family.SAFIVPN {
		pathID, payload, splitErr := nlri.SplitPathID(data, addPath)
		if splitErr != nil {
			logger().Warn("VPN inventory path identifier rejected", "family", famStr, "error", splitErr)
			return records
		}
		// RFC 8277 Section 2.4: "Upon reception, the value of the
		// Compatibility field MUST be ignored." The registered key
		// retains RD and prefix, but neither label nor Compatibility.
		key, keyErr := nlrisplit.GetPrefixKey(fam)(payload, scratch, action == actionDel)
		if keyErr != nil {
			logger().Warn("VPN inventory identity rejected", "family", famStr, "error", keyErr)
			return records
		}
		rec.pathID = pathID
		rec.nativeKey = string(key)
	}
	records = append(records, rec)
	return records
}

// opaqueRouteCIDR answers the prefix and path identifier one framed
// announcement of fam names, and false when fam does not name its routes by a
// CIDR or the bytes do not hold one. A malformed NLRI then stays keyed by its
// hex, as every opaque family's is. scratch is the pooled extraction's
// nlrisplit.PrefixKeyScratchSize-byte buffer and MUST NOT be retained.
func opaqueRouteCIDR(fam family.Family, part []byte, addPath bool, scratch []byte) (netip.Prefix, uint32, bool) {
	pathID, payload, err := nlri.SplitPathID(part, addPath)
	if err != nil {
		return netip.Prefix{}, 0, false
	}
	cidr, err := nlrisplit.RouteCIDR(fam, payload, scratch, false)
	if err != nil {
		return netip.Prefix{}, 0, false
	}
	prefix, ok := nlri.WirePrefixToKey(cidr, fam)
	if !ok {
		return netip.Prefix{}, 0, false
	}
	if !prefix.IsValid() {
		return netip.Prefix{}, 0, false
	}
	return prefix, pathID, true
}

// recordKey derives the withdrawal-set key for one record. The add and the del
// arm MUST derive it the same way, or a withdrawal never cancels its announce.
func recordKey(rec *nlriRecord) withdrawalKey {
	if rec.nativeKey != "" {
		return withdrawalKey{fam: rec.fam, nlriStr: rec.nativeKey, pathID: rec.pathID, wireForm: true, addPath: rec.addPath}
	}
	if rec.cidrKeyed {
		return withdrawalKey{fam: rec.fam, prefix: rec.prefix, pathID: rec.pathID}
	}
	if rec.nlriStr != "" {
		return withdrawalKey{fam: rec.fam, nlriStr: rec.nlriStr, wireForm: rec.wireForm, addPath: rec.addPath}
	}
	return withdrawalKey{fam: rec.fam, prefix: rec.prefix}
}

// recordEntry is what the withdrawal set holds for an announced record: the
// hex its peer-down withdrawal goes out as when the key is not that hex.
func recordEntry(rec *nlriRecord) withdrawalEntry {
	if rec.nativeKey != "" {
		return withdrawalEntry{wire: rec.nlriStr, addPath: rec.addPath}
	}
	if rec.cidrKeyed {
		return withdrawalEntry{wire: rec.nlriStr, addPath: rec.addPath}
	}
	return withdrawalEntry{}
}

// applyNLRIRecords updates the withdrawal map from pre-extracted NLRI records.
// Called AFTER forwarding, off the forward critical path.
// Caller must hold rs.withdrawalMu.
func (rs *routeServer) applyNLRIRecords(sourcePeer string, records []nlriRecord) {
	for i := range records {
		rec := &records[i]
		switch rec.action {
		case actionAdd:
			if rs.withdrawals[sourcePeer] == nil {
				rs.withdrawals[sourcePeer] = make(map[withdrawalKey]withdrawalEntry)
			}
			rs.withdrawals[sourcePeer][recordKey(rec)] = recordEntry(rec)
		case actionDel:
			if rs.withdrawals[sourcePeer] != nil {
				delete(rs.withdrawals[sourcePeer], recordKey(rec))
			}
		}
	}
}
