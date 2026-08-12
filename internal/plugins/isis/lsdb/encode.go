// Design: docs/architecture/isis/isis-6-lsdb.md -- TLV value builders + fragment packer for origination.
// ISO/IEC 10589 clause 9 (TLV framing: 1-octet length, value 0..255).
//
// RFC: rfc/short/rfc5305.md -- TLV 22 / TLV 135 entry layout (sec 3/4)
// RFC: rfc/short/rfc1195.md -- TLV 129 (Protocols Supported), TLV 132 (IP Interface Address)
// RFC: rfc/short/rfc5301.md -- TLV 137 (Dynamic Hostname)
//
// These helpers turn the origination inputs into the entry-value bytes the
// fragment packer places, and implement the packer that splits TLV entries
// across LSP fragments without splitting a single entry (spec AC-5, R-3). They
// build the raw VALUE bytes (not full type+length+value) because the packer owns
// the TLV header: it must repeat the TLV 22 / TLV 135 header whenever a run of
// entries crosses a fragment or the 255-octet TLV value limit.

package lsdb

import (
	"net/netip"

	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// protocolNLPIDs returns the TLV 129 (Protocols Supported) value: one NLPID
// octet per advertised family (RFC 1195: 0xCC IPv4; RFC 5308: 0x8E IPv6).
func protocolNLPIDs(node NodeInfo) []byte {
	var out []byte
	if node.AdvertiseIPv4 {
		out = append(out, packet.NLPIDIPv4)
	}
	if node.AdvertiseIPv6 {
		out = append(out, packet.NLPIDIPv6)
	}
	return out
}

// encodeAreaTLV builds a whole TLV 1 (Area Addresses) as an opaque packet.TLV.
// The value is a sequence of (1-octet length, area octets) per area (ISO/IEC
// 10589 clause 9.2). Areas whose combined length would overflow one TLV value
// (255 octets) are not expected for a node's own area list; the value is built
// whole and the packer treats it as a single non-splittable TLV.
func encodeAreaTLV(areas []types.AreaID) packet.TLV {
	n := 0
	for _, a := range areas {
		n += 1 + a.Len()
	}
	val := make([]byte, 0, n)
	var scratch [types.MaxAreaIDLen]byte
	for _, a := range areas {
		val = append(val, byte(a.Len()))
		m := a.WriteTo(scratch[:], 0)
		val = append(val, scratch[:m]...)
	}
	return packet.TLV{Type: packet.TLVAreaAddresses, Value: val}
}

// hostnameTLV builds a whole TLV 137 (Dynamic Hostname, RFC 5301 sec 3) from the
// configured name, unchanged.
//
// RFC 5301 Section 3: "The Value field is encoded in 7-bit ASCII." This function
// does not produce that guarantee. ISISHostnameValidator produces it at the
// config boundary (internal/component/config/validators.go). That validator
// refuses any name carrying an octet outside 0x20..0x7e, and any name breaking
// RFC 2181 section 11's label lengths. This function must not sanitize. A
// value the operator configured and a value Ze advertises have to be the same
// string (ai/rules/protocol.md).
//
// The 255-octet bound below is a defensive guard, not a policy. A config-shaped
// name can never reach it, because the validator refuses a name over 255 octets
// first (TestISISHostnameTLVTruncationUnreachable pins that). It fires only for
// a NodeInfo built in Go that bypassed config, which is an invariant violation.
// Shortening a name is NOT intended behavior. It bounds the TLV rather than
// overflow it, and the caller is already wrong by the time it runs.
func hostnameTLV(name string) packet.TLV {
	b := []byte(name)
	if len(b) > packet.MaxTLVValueLen {
		b = b[:packet.MaxTLVValueLen]
	}
	return packet.TLV{Type: packet.TLVDynamicHostname, Value: b}
}

// interfaceAddrTLVs builds one or more whole TLV 132 (IP Interface Address,
// RFC 1195) values from the node's own IPv4 interface addresses. Each address is
// 4 octets; a TLV value holds at most 255/4 = 63 addresses, so a long list is
// split across several whole TLV 132s (each still a non-splittable unit for the
// packer). Non-IPv4 addresses are skipped.
func interfaceAddrTLVs(addrs []netip.Addr) []packet.TLV {
	const maxPerTLV = packet.MaxTLVValueLen / packet.IPv4AddrLen
	var out []packet.TLV
	var val []byte
	flush := func() {
		if len(val) > 0 {
			out = append(out, packet.TLV{Type: packet.TLVIPInterfaceAddress, Value: val})
			val = nil
		}
	}
	for _, a := range addrs {
		if !a.Is4() {
			continue
		}
		a4 := a.As4()
		val = append(val, a4[:]...)
		if len(val)/packet.IPv4AddrLen >= maxPerTLV {
			flush()
		}
	}
	flush()
	return out
}

// extISReachEntryBytes builds one TLV 22 (Extended IS Reachability) ENTRY value
// (RFC 5305 sec 3): 7-octet neighbor Source ID + 3-octet 24-bit metric +
// 1-octet sub-TLV length (0; no sub-TLVs originated by this spec). It is an
// entry, not a whole TLV: the packer repeats the TLV 22 header around a run of
// these. Fixed 11 octets.
func extISReachEntryBytes(n AdjacencyInfo) []byte {
	const entryLen = types.SourceIDLen + types.MetricLen + 1
	b := make([]byte, entryLen)
	off := n.Neighbor.WriteTo(b, 0)
	off += n.Metric.WriteTo(b, off)
	b[off] = 0 // sub-TLV length: none
	return b
}

// extIPReachEntryBytes builds one TLV 135 (Extended IP Reachability) ENTRY value
// (RFC 5305 sec 4, umbrella canonical layout): 4-octet metric + 1-octet control
// (up/down bit 0x80 + 6-bit prefix length; no sub-TLVs) + ceil(len/8) packed
// prefix octets. It reuses the codec's single-entry encoder via ExtendedIPReachTLV
// then strips the 2-octet TLV header so the packer owns the framing.
func extIPReachEntryBytes(p PrefixInfo) []byte {
	tlv := packet.ExtendedIPReachTLV{Entries: []packet.ExtIPReachEntry{{
		Metric: p.Metric,
		UpDown: p.UpDown,
		Prefix: p.Prefix,
	}}}
	buf := make([]byte, tlv.EncodedLen())
	n := tlv.WriteTo(buf, 0)
	// Strip the leading TLV header (type + length) to get the entry value bytes.
	return buf[packet.TLVHeaderLen:n]
}

// interfaceAddrV6TLVs builds one or more whole TLV 232 (IPv6 Interface Address,
// RFC 5308 sec 3) values from the node's own NON-LINK-LOCAL IPv6 interface
// addresses. Each address is 16 octets; a TLV value holds at most 255/16 = 15
// addresses (RFC 5308 sec 3), so a long list is split across several whole TLV
// 232s (each a non-splittable unit for the packer). Non-IPv6 addresses are
// skipped. The caller (isis-12 origination) is responsible for excluding
// link-local addresses from an LSP TLV 232 (RFC 5308 sec 3).
func interfaceAddrV6TLVs(addrs []netip.Addr) []packet.TLV {
	const maxPerTLV = packet.MaxTLVValueLen / packet.IPv6AddrLen
	var out []packet.TLV
	var val []byte
	flush := func() {
		if len(val) > 0 {
			out = append(out, packet.TLV{Type: packet.TLVIPv6InterfaceAddress, Value: val})
			val = nil
		}
	}
	for _, a := range addrs {
		if !a.Is6() || a.Is4In6() {
			continue
		}
		a16 := a.As16()
		val = append(val, a16[:]...)
		if len(val)/packet.IPv6AddrLen >= maxPerTLV {
			flush()
		}
	}
	flush()
	return out
}

// extIPv6ReachEntryBytes builds one TLV 236 (IPv6 Reachability) ENTRY value
// (RFC 5308 sec 2, umbrella canonical layout): 4-octet metric + 1-octet flags
// (up/down 0x80 + external 0x20; no sub-TLVs originated) + 1-octet prefix length
// + ceil(len/8) packed prefix octets. It reuses the codec's single-entry encoder
// via IPv6ReachabilityTLV then strips the 2-octet TLV header so the packer owns
// the framing (mirrors extIPReachEntryBytes for IPv4).
func extIPv6ReachEntryBytes(p PrefixInfoV6) []byte {
	tlv := packet.IPv6ReachabilityTLV{Entries: []packet.IPv6ReachEntry{{
		Metric:   p.Metric,
		UpDown:   p.UpDown,
		External: p.External,
		Prefix:   p.Prefix,
	}}}
	buf := make([]byte, tlv.EncodedLen())
	n := tlv.WriteTo(buf, 0)
	// Strip the leading TLV header (type + length) to get the entry value bytes.
	return buf[packet.TLVHeaderLen:n]
}

// ---- Fragment packer ----
//
// The packer places TLVs and TLV entries into LSP fragments so no fragment's
// encoded TLV region exceeds the per-fragment budget and no single TLV entry is
// split (ISO/IEC 10589; RFC 5305). It models each fragment as an ordered TLV
// list; entries of the same TLV type accumulate into the current TLV of that
// type until they would overflow the 255-octet TLV value limit or the fragment
// budget, at which point a new TLV (or a new fragment) starts.

// fragmentPacker accumulates per-fragment TLV lists.
type fragmentPacker struct {
	budget int // per-fragment TLV-region byte budget

	frags [][]packet.TLV // completed + current fragments, by index
	used  []int          // encoded TLV-region bytes used per fragment

	// openType / openIdx track the currently-open accumulating TLV (TLV 22 / 135)
	// in the current (last) fragment: its type and its index in frags[last]. -1
	// means no open accumulating TLV.
	openType int
	openIdx  int
}

// newFragmentPacker starts a packer with one empty fragment (fragment 0, always
// present and valid -- RFC 3786). budget is the TLV-region bytes per fragment.
func newFragmentPacker(budget int) *fragmentPacker {
	return &fragmentPacker{
		budget:   budget,
		frags:    [][]packet.TLV{{}},
		used:     []int{0},
		openType: -1,
	}
}

// last returns the index of the current (last) fragment.
func (p *fragmentPacker) last() int { return len(p.frags) - 1 }

// newFragment appends a fresh empty fragment and makes it current, resetting the
// open-TLV tracking. Bounded by maxFragments by the caller (origination drops
// state beyond 256 fragments; a node never legitimately needs more in v1).
func (p *fragmentPacker) newFragment() {
	p.frags = append(p.frags, []packet.TLV{})
	p.used = append(p.used, 0)
	p.openType = -1
}

// addWholeTLV places a complete, non-splittable TLV (the fixed fragment-0 TLVs
// and TLV 132). It goes in the current fragment if it fits, else a new fragment.
// A whole TLV closes any open accumulating TLV (a later entry reopens one). A
// TLV that cannot fit even an empty fragment's budget is dropped (cannot happen
// for the fixed TLVs given the minLSPSize floor).
func (p *fragmentPacker) addWholeTLV(t packet.TLV) {
	cost := t.EncodedLen()
	if p.used[p.last()]+cost > p.budget && len(p.frags[p.last()]) > 0 {
		p.newFragment()
	}
	if cost > p.budget {
		return // un-encodable in any fragment; skip (floor prevents this in practice)
	}
	p.frags[p.last()] = append(p.frags[p.last()], t)
	p.used[p.last()] += cost
	p.openType = -1 // a whole TLV breaks the accumulating run
}

// addEntry places one TLV entry (a TLV 22 or TLV 135 record) of the given type.
// It appends to the open TLV of that type in the current fragment when the entry
// fits within the 255-octet TLV value limit and the fragment budget; otherwise
// it starts a new TLV of that type (in a new fragment if the current one cannot
// hold even a fresh TLV header + this entry). A single entry is never split.
func (p *fragmentPacker) addEntry(tlvType int, entry []byte) {
	entryLen := len(entry)

	// Can we extend the currently-open TLV of this type?
	if p.openType == tlvType {
		cur := &p.frags[p.last()][p.openIdx]
		// The TLV value must stay <= 255 octets, and the fragment must have room
		// for the extra entry bytes (the TLV header is already counted).
		if len(cur.Value)+entryLen <= packet.MaxTLVValueLen && p.used[p.last()]+entryLen <= p.budget {
			cur.Value = append(cur.Value, entry...)
			p.used[p.last()] += entryLen
			return
		}
	}

	// Need a new TLV of this type: header (2) + this entry.
	cost := packet.TLVHeaderLen + entryLen
	if p.used[p.last()]+cost > p.budget {
		// Not even a fresh TLV fits this fragment: start a new fragment (unless the
		// current one is empty, in which case the entry is larger than a whole
		// fragment -- impossible for the fixed entry sizes here).
		if len(p.frags[p.last()]) > 0 {
			p.newFragment()
		}
		if cost > p.budget {
			return // un-encodable; skip
		}
	}
	idx := len(p.frags[p.last()])
	p.frags[p.last()] = append(p.frags[p.last()], packet.TLV{
		Type:  uint8(tlvType),
		Value: append([]byte(nil), entry...),
	})
	p.used[p.last()] += cost
	p.openType = tlvType
	p.openIdx = idx
}

// fragments returns the packed per-fragment TLV lists, dropping any trailing
// empty fragments beyond fragment 0 (fragment 0 is always returned, even empty,
// because it carries the node's fixed fields and is required, RFC 3786). The
// result is capped at maxFragments; state that would need more is dropped (v1
// does not implement RFC 3786 extended fragments).
func (p *fragmentPacker) fragments() [][]packet.TLV {
	out := p.frags
	// Trim trailing empty fragments (keep at least fragment 0).
	for len(out) > 1 && len(out[len(out)-1]) == 0 {
		out = out[:len(out)-1]
	}
	if len(out) > maxFragments {
		out = out[:maxFragments]
	}
	return out
}
