// Design: docs/architecture/wire/attributes.md — path attribute encoding
// Related: text.go — text parsing + legacy Format* helpers (deleted post-migration)
//
// Zero-allocation filter-text serialization. Each attribute type appends its
// filter-text rendering ("<name> <value>" form, except AtomicAggregate which
// is the bare token "atomic-aggregate") into a caller-provided []byte and
// returns the extended slice. Element types (LargeCommunity, ExtendedCommunity,
// *Aggregator) append the bare value form; list-attribute AppendText wraps
// them with a name prefix and optional [...] brackets for multi-element lists.
//
// No fmt.Sprintf, no strings.Builder, no strconv.FormatUint, no strings.Join
// in this file. Primitives: strconv.AppendUint, netip.Addr.AppendTo,
// hex.AppendEncode, literal `append(buf, "..."...)` for token strings.

package attribute

import (
	"encoding/hex"
	"net/netip"
	"strconv"
)

// -----------------------------------------------------------------------------
// Element-level AppendText methods (bare value, no name prefix).
// -----------------------------------------------------------------------------

// AppendText appends this aggregator's value form ("<asn>:<ip>") to buf.
// Returns buf unchanged when the aggregator address is invalid (zero value);
// this prevents "aggregator 0:invalid IP" from breaking the space-delimited
// filter text contract. Wire parsing always produces a valid address from
// 4 or 16 octets, so this guard is defensive only.
func (a *Aggregator) AppendText(buf []byte) []byte {
	if !a.Address.IsValid() {
		return buf
	}
	buf = strconv.AppendUint(buf, uint64(a.ASN), 10)
	buf = append(buf, ':')
	buf = a.Address.AppendTo(buf)
	return buf
}

// AppendText appends this large community's value form ("<ga>:<ld1>:<ld2>") to buf.
func (l LargeCommunity) AppendText(buf []byte) []byte {
	buf = strconv.AppendUint(buf, uint64(l.GlobalAdmin), 10)
	buf = append(buf, ':')
	buf = strconv.AppendUint(buf, uint64(l.LocalData1), 10)
	buf = append(buf, ':')
	buf = strconv.AppendUint(buf, uint64(l.LocalData2), 10)
	return buf
}

// AppendText appends this extended community's 8-byte lowercase hex form to buf.
func (e ExtendedCommunity) AppendText(buf []byte) []byte {
	return hex.AppendEncode(buf, e[:])
}

// appendCommunityText appends a single standard community (32-bit value) using
// its well-known lowercase name if known, otherwise "<asn>:<val>". Matches the
// legacy FormatCommunity output byte-for-byte.
func appendCommunityText(buf []byte, c uint32) []byte {
	switch c {
	case uint32(CommunityNoExport):
		return append(buf, "no-export"...)
	case uint32(CommunityNoAdvertise):
		return append(buf, "no-advertise"...)
	case uint32(CommunityNoExportSubconfed):
		return append(buf, "no-export-subconfed"...)
	case uint32(CommunityNoPeer):
		return append(buf, "nopeer"...)
	case 0xFFFF029A: // RFC 7999 blackhole
		return append(buf, "blackhole"...)
	}
	buf = strconv.AppendUint(buf, uint64(c>>16), 10)
	buf = append(buf, ':')
	buf = strconv.AppendUint(buf, uint64(c&0xFFFF), 10)
	return buf
}

// appendClusterID appends a cluster ID as dotted-decimal "a.b.c.d".
func appendClusterID(buf []byte, id uint32) []byte {
	buf = strconv.AppendUint(buf, uint64(byte(id>>24)), 10)
	buf = append(buf, '.')
	buf = strconv.AppendUint(buf, uint64(byte(id>>16)), 10)
	buf = append(buf, '.')
	buf = strconv.AppendUint(buf, uint64(byte(id>>8)), 10)
	buf = append(buf, '.')
	buf = strconv.AppendUint(buf, uint64(byte(id)), 10)
	return buf
}

// -----------------------------------------------------------------------------
// Attribute-level AppendText methods (filter-text "<name> <value>" form).
// -----------------------------------------------------------------------------

// AppendText appends "origin <value>" where value is "igp", "egp", or "incomplete".
// RFC 4271 Section 4.3 defines only values 0-2; any other value maps to the
// "incomplete" token to match legacy FormatOrigin behavior.
func (o Origin) AppendText(buf []byte) []byte {
	buf = append(buf, "origin "...)
	switch o {
	case OriginIGP:
		return append(buf, "igp"...)
	case OriginEGP:
		return append(buf, "egp"...)
	case OriginIncomplete:
		return append(buf, "incomplete"...)
	}
	return append(buf, "incomplete"...)
}

// AppendText appends "as-path <asns>" where asns is either a single ASN or
// a bracketed list. Returns buf unchanged if the AS path has no ASNs.
//
// Filter-text format intentionally flattens every segment type (AS_SEQUENCE,
// AS_SET, AS_CONFED_SEQUENCE, AS_CONFED_SET) into a single space-separated
// list. The set-versus-sequence distinction is NOT preserved on this path;
// filter plugins that need segment types must consume the raw wire bytes via
// `FilterUpdateInput.Raw` (raw=true). Preserving this flattening matches the
// legacy FormatASPath output byte-for-byte.
func (p *ASPath) AppendText(buf []byte) []byte {
	var total int
	for _, seg := range p.Segments {
		total += len(seg.ASNs)
	}
	if total == 0 {
		return buf
	}
	buf = append(buf, "as-path "...)
	if total == 1 {
		for _, seg := range p.Segments {
			if len(seg.ASNs) > 0 {
				return strconv.AppendUint(buf, uint64(seg.ASNs[0]), 10)
			}
		}
		return buf
	}
	buf = append(buf, '[')
	first := true
	for _, seg := range p.Segments {
		for _, asn := range seg.ASNs {
			if !first {
				buf = append(buf, ' ')
			}
			buf = strconv.AppendUint(buf, uint64(asn), 10)
			first = false
		}
	}
	buf = append(buf, ']')
	return buf
}

// AppendText appends "next-hop <addr>". Returns buf unchanged if addr invalid.
func (n *NextHop) AppendText(buf []byte) []byte {
	if !n.Addr.IsValid() {
		return buf
	}
	buf = append(buf, "next-hop "...)
	return n.Addr.AppendTo(buf)
}

// AppendText appends "med <value>".
func (m MED) AppendText(buf []byte) []byte {
	buf = append(buf, "med "...)
	return strconv.AppendUint(buf, uint64(m), 10)
}

// AppendText appends "local-preference <value>".
func (l LocalPref) AppendText(buf []byte) []byte {
	buf = append(buf, "local-preference "...)
	return strconv.AppendUint(buf, uint64(l), 10)
}

// AppendText appends the bare token "atomic-aggregate" (no value).
func (AtomicAggregate) AppendText(buf []byte) []byte {
	return append(buf, "atomic-aggregate"...)
}

// AppendText on *Aggregator (defined above) returns the element form
// "<asn>:<ip>" without a "aggregator " name prefix. Filter-text dispatchers
// prepend the token explicitly, keeping the method symmetrical with other
// element types (LargeCommunity, ExtendedCommunity).

// AppendText appends "community <comm>" or "community [<c1> <c2> ...]".
// Returns buf unchanged if the list is empty.
func (c Communities) AppendText(buf []byte) []byte {
	if len(c) == 0 {
		return buf
	}
	buf = append(buf, "community "...)
	if len(c) == 1 {
		return appendCommunityText(buf, uint32(c[0]))
	}
	buf = append(buf, '[')
	for i, comm := range c {
		if i > 0 {
			buf = append(buf, ' ')
		}
		buf = appendCommunityText(buf, uint32(comm))
	}
	buf = append(buf, ']')
	return buf
}

// AppendText appends "originator-id <addr>" per RFC 4456.
// Returns buf unchanged when the originator address is invalid (zero value).
func (o OriginatorID) AppendText(buf []byte) []byte {
	addr := netip.Addr(o)
	if !addr.IsValid() {
		return buf
	}
	buf = append(buf, "originator-id "...)
	return addr.AppendTo(buf)
}

// AppendText appends "cluster-list <id1> <id2> ...". Cluster-list uses
// space-separated dotted-decimal IDs without brackets (legacy format).
// Returns buf unchanged if the list is empty.
func (c ClusterList) AppendText(buf []byte) []byte {
	if len(c) == 0 {
		return buf
	}
	buf = append(buf, "cluster-list "...)
	for i, id := range c {
		if i > 0 {
			buf = append(buf, ' ')
		}
		buf = appendClusterID(buf, id)
	}
	return buf
}

// AppendText appends "large-community <lc>" or "large-community [<lc1> <lc2> ...]".
// Returns buf unchanged if the list is empty.
func (l LargeCommunities) AppendText(buf []byte) []byte {
	if len(l) == 0 {
		return buf
	}
	buf = append(buf, "large-community "...)
	if len(l) == 1 {
		return l[0].AppendText(buf)
	}
	buf = append(buf, '[')
	for i, lc := range l {
		if i > 0 {
			buf = append(buf, ' ')
		}
		buf = lc.AppendText(buf)
	}
	buf = append(buf, ']')
	return buf
}

// AppendText appends "extended-community <hex>" or "extended-community [<hex1> <hex2> ...]".
// Returns buf unchanged if the list is empty.
func (e ExtendedCommunities) AppendText(buf []byte) []byte {
	if len(e) == 0 {
		return buf
	}
	buf = append(buf, "extended-community "...)
	if len(e) == 1 {
		return e[0].AppendText(buf)
	}
	buf = append(buf, '[')
	for i, ec := range e {
		if i > 0 {
			buf = append(buf, ' ')
		}
		buf = ec.AppendText(buf)
	}
	buf = append(buf, ']')
	return buf
}
