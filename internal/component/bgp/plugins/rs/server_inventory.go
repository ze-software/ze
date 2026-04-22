// Design: docs/architecture/core-design.md -- peer-down route inventory for route server
// Overview: server.go -- route server plugin orchestration
// Related: server_withdrawal.go -- withdrawal map management and NLRI walking

package rs

import (
	"net/netip"
	"sync"

	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// nlriRecord is a compact representation of one NLRI extracted from a wire
// UPDATE before forwarding. For unicast families, prefix is set (16 bytes,
// no allocation). For non-unicast families, nlriStr is set (allocating but
// rare in the grouped-input benchmark).
type nlriRecord struct {
	familyName string
	action     string // actionAdd or actionDel
	prefix     netip.Prefix
	nlriStr    string // non-empty only for non-unicast families
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

	// MP_REACH_NLRI -- announced routes.
	if mp, err := wu.MPReach(); err == nil && mp != nil {
		fam := mp.Family()
		addPath := encCtx != nil && encCtx.AddPath(fam)
		if isUnicast(fam) {
			*sp = appendUnicastRecords(*sp, fam.String(), mp.NLRIIterator(addPath), actionAdd)
		} else {
			*sp = appendAllocatingRecords(*sp, fam, mp, addPath, actionAdd)
		}
	}

	// MP_UNREACH_NLRI -- withdrawn routes.
	if mp, err := wu.MPUnreach(); err == nil && mp != nil {
		fam := mp.Family()
		addPath := encCtx != nil && encCtx.AddPath(fam)
		if isUnicast(fam) {
			*sp = appendUnicastRecords(*sp, fam.String(), mp.NLRIIterator(addPath), actionDel)
		} else {
			nlris, nlriErr := mp.NLRIs(addPath)
			*sp = appendAllocatingUnreachRecords(*sp, fam, nlris, nlriErr)
		}
	}

	// IPv4 body NLRIs -- announced routes.
	addPathV4 := encCtx != nil && encCtx.AddPath(family.IPv4Unicast)
	if iter, err := wu.NLRIIterator(addPathV4); err == nil && iter != nil {
		*sp = appendUnicastRecords(*sp, "ipv4/unicast", iter, actionAdd)
	}

	// IPv4 body Withdrawn -- withdrawn routes.
	if iter, err := wu.WithdrawnIterator(addPathV4); err == nil && iter != nil {
		*sp = appendUnicastRecords(*sp, "ipv4/unicast", iter, actionDel)
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
func appendUnicastRecords(records []nlriRecord, famName string, iter *nlri.NLRIIterator, action string) []nlriRecord {
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
	famStr := fam.String()
	for _, n := range nlris {
		records = append(records, nlriRecord{
			familyName: famStr,
			action:     action,
			nlriStr:    n.String(),
		})
	}
	return records
}

// appendAllocatingUnreachRecords appends records for non-unicast MP_UNREACH families.
func appendAllocatingUnreachRecords(records []nlriRecord, fam family.Family, nlris []nlri.NLRI, err error) []nlriRecord {
	if err != nil || len(nlris) == 0 {
		return records
	}
	famStr := fam.String()
	for _, n := range nlris {
		records = append(records, nlriRecord{
			familyName: famStr,
			action:     actionDel,
			nlriStr:    n.String(),
		})
	}
	return records
}

// applyNLRIRecords updates the withdrawal map from pre-extracted NLRI records.
// Called AFTER forwarding, off the forward critical path.
// Caller must hold rs.withdrawalMu.
func (rs *RouteServer) applyNLRIRecords(sourcePeer string, records []nlriRecord) {
	for i := range records {
		rec := &records[i]
		switch rec.action {
		case actionAdd:
			if rs.withdrawals[sourcePeer] == nil {
				rs.withdrawals[sourcePeer] = make(map[string]withdrawalInfo)
			}
			if rec.nlriStr != "" {
				routeKey := rec.familyName + "|" + nlriKey(rec.nlriStr)
				rs.withdrawals[sourcePeer][routeKey] = withdrawalInfo{Family: rec.familyName, Prefix: rec.nlriStr}
			} else {
				key := rec.prefix.String()
				routeKey := rec.familyName + "|" + key
				rs.withdrawals[sourcePeer][routeKey] = withdrawalInfo{Family: rec.familyName, Prefix: "prefix " + key}
			}
		case actionDel:
			if rs.withdrawals[sourcePeer] != nil {
				if rec.nlriStr != "" {
					delete(rs.withdrawals[sourcePeer], rec.familyName+"|"+nlriKey(rec.nlriStr))
				} else {
					delete(rs.withdrawals[sourcePeer], rec.familyName+"|"+rec.prefix.String())
				}
			}
		}
	}
}
