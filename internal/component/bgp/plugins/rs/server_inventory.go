// Design: docs/architecture/core-design.md -- peer-down route inventory for route server
// Overview: server.go -- route server plugin orchestration
// Related: server_withdrawal.go -- withdrawal map management and NLRI walking

package rs

import (
	"net/netip"
	"sync"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
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
	wireForm   bool   // nlriStr is hex of one NLRI, not a text token
	addPath    bool   // wireForm hex carries a 4-octet path identifier
}

// nlriRecordPool amortizes slice allocation for NLRI extraction.
// Typical grouped UPDATEs carry 100-200 IPv4 prefixes; initial capacity 256
// covers the common case without resize.
var nlriRecordPool = sync.Pool{
	New: func() any {
		s := make([]nlriRecord, 0, 256)
		return &s
	},
}

// extractWireNLRIRecords extracts compact NLRI records from a raw wire UPDATE.
// Must be called BEFORE forwarding (buffer lifetime safety: cache eviction can
// free the pool buffer backing msg.WireUpdate after ForwardCached).
// Returns a pooled handle -- caller must call returnNLRIRecords when done.
func extractWireNLRIRecords(msg *bgptypes.RawMessage) *[]nlriRecord {
	if msg.WireUpdate == nil {
		return nil
	}
	wu := msg.WireUpdate

	sp, ok := nlriRecordPool.Get().(*[]nlriRecord)
	if !ok {
		return nil
	}
	*sp = (*sp)[:0]

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
			*sp = appendUnicastRecords(*sp, fam, fam.String(), mp.NLRIIterator(addPath), actionDel)
		} else {
			nlris, nlriErr := mp.NLRIs(addPath)
			*sp = appendAllocatingUnreachRecords(*sp, fam, nlris, nlriErr)
		}
	}

	// IPv4 body Withdrawn -- withdrawn routes.
	if iter, err := wu.WithdrawnIterator(addPathV4); err == nil && iter != nil {
		*sp = appendUnicastRecords(*sp, family.IPv4Unicast, "ipv4/unicast", iter, actionDel)
	}

	// MP_REACH_NLRI -- announced routes.
	if mp, err := wu.MPReach(); err == nil && mp != nil {
		fam := mp.Family()
		addPath := encCtx != nil && encCtx.AddPath(fam)
		if isUnicast(fam) {
			*sp = appendUnicastRecords(*sp, fam, fam.String(), mp.NLRIIterator(addPath), actionAdd)
		} else {
			*sp = appendAllocatingRecords(*sp, fam, mp, addPath, actionAdd)
		}
	}

	// IPv4 body NLRIs -- announced routes.
	if iter, err := wu.NLRIIterator(addPathV4); err == nil && iter != nil {
		*sp = appendUnicastRecords(*sp, family.IPv4Unicast, "ipv4/unicast", iter, actionAdd)
	}

	return sp
}

// returnNLRIRecords returns the pooled record handle.
func returnNLRIRecords(sp *[]nlriRecord) {
	if sp == nil {
		return
	}
	*sp = (*sp)[:0]
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
}, addPath bool, action string) []nlriRecord {
	nlris, err := mp.NLRIs(addPath)
	if err != nil || len(nlris) == 0 {
		return records
	}
	return appendParsedRecords(records, fam, nlris, action)
}

// appendAllocatingUnreachRecords appends records for non-unicast MP_UNREACH families.
func appendAllocatingUnreachRecords(records []nlriRecord, fam family.Family, nlris []nlri.NLRI, err error) []nlriRecord {
	if err != nil || len(nlris) == 0 {
		return records
	}
	return appendParsedRecords(records, fam, nlris, actionDel)
}

// appendParsedRecords turns parsed NLRIs into inventory records.
//
// An NLRI ze parses has a text spelling, and String() is it. An NLRI ze does
// NOT parse arrives as *nlri.WireNLRI, and its String() is a size summary
// ("wire[bgp-ls/bgp-ls](23 bytes)") that carries none of the bytes: it can
// neither identify the route in the withdrawal set nor be re-parsed by any
// command grammar. Those go in as hex instead, and their withdrawal goes out
// as "update hex" (sendBatchedWithdrawals).
func appendParsedRecords(records []nlriRecord, fam family.Family, nlris []nlri.NLRI, action string) []nlriRecord {
	famStr := fam.String()
	for _, n := range nlris {
		if w, ok := n.(*nlri.WireNLRI); ok {
			records = appendOpaqueRecords(records, fam, famStr, w, action)
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

// appendOpaqueRecords appends one hex record per NLRI carried by an opaque
// wire blob.
//
// wireu.ParseNLRIs hands back the WHOLE NLRI section as a single *WireNLRI for
// any family with no dedicated parser, so the blob has to be split here or one
// key would stand for every NLRI in the UPDATE: a later MP_UNREACH naming one
// of them would miss the key and leave the rest of the section un-withdrawn on
// peer-down. message.GetNLRISizeFunc is the same sizer the wire command parser
// uses to split them again (splitWireNLRIs), so the two agree by construction.
//
// The hex is a copy, which the buffer lifetime requires: the caller runs before
// ForwardCached and the wire buffer can be freed after it.
func appendOpaqueRecords(records []nlriRecord, fam family.Family, famStr string, w *nlri.WireNLRI, action string) []nlriRecord {
	data := w.Bytes()
	if len(data) == 0 {
		return records
	}
	addPath := w.HasAddPath()
	sizeFunc := message.GetNLRISizeFunc(fam.AFI, fam.SAFI, addPath)

	var tb textbuf.Buffer
	for offset := 0; offset < len(data); {
		size, err := sizeFunc(data[offset:])
		if err != nil || size <= 0 || offset+size > len(data) {
			// Say something rather than degrade silently (ai/rules/evidence.md).
			// The remainder still gets recorded as one blob: the receiving sizer
			// will refuse it the same way, and losing the route entirely would
			// leave it announced forever after the source peer goes down.
			logger().Warn("opaque NLRI split failed; recording the remainder as one blob",
				"family", famStr, "offset", offset, "remaining", len(data)-offset, "error", err)
			size = len(data) - offset
		}
		records = append(records, nlriRecord{
			fam:        fam,
			familyName: famStr,
			action:     action,
			nlriStr:    tb.Reset().Hex(data[offset : offset+size]).String(),
			wireForm:   true,
			addPath:    addPath,
		})
		offset += size
	}
	return records
}

// recordKey derives the withdrawal-set key for one record. The add and the del
// arm MUST derive it the same way, or a withdrawal never cancels its announce.
func recordKey(rec *nlriRecord) withdrawalKey {
	if rec.nlriStr != "" {
		return withdrawalKey{fam: rec.fam, nlriStr: rec.nlriStr, wireForm: rec.wireForm, addPath: rec.addPath}
	}
	return withdrawalKey{fam: rec.fam, prefix: rec.prefix}
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
				rs.withdrawals[sourcePeer] = make(map[withdrawalKey]struct{})
			}
			rs.withdrawals[sourcePeer][recordKey(rec)] = struct{}{}
		case actionDel:
			if rs.withdrawals[sourcePeer] != nil {
				delete(rs.withdrawals[sourcePeer], recordKey(rec))
			}
		}
	}
}
