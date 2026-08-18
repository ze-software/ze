// Design: docs/architecture/wire/isis.md -- core TLV codecs (1, 8, 9, 22, 129, 132-shared, 137, 240)
// ISO/IEC 10589 clause 9.2 (TLV 1), 9.10 (TLV 8), 9.14 (TLV 9).
//
// RFC: rfc/short/rfc5305.md -- TLV 22 (Extended IS Reachability), sub-TLV framing (sec 2, 3)
// RFC: rfc/short/rfc5301.md -- TLV 137 (Dynamic Hostname)
// RFC: rfc/short/rfc5303.md -- TLV 240 (Point-to-Point Three-Way Adjacency)
// RFC: rfc/short/rfc1195.md -- TLV 129 (Protocols Supported), NLPID values

package packet

import "github.com/ze-software/ze/internal/plugins/isis/types"

// NLPID values carried in TLV 129 (Protocols Supported, RFC 1195 / RFC 5308).
const (
	NLPIDIPv4 = 0xCC // RFC 1195
	NLPIDIPv6 = 0x8E // RFC 5308 sec 4
)

// ---- TLV 1: Area Addresses (ISO/IEC 10589 clause 9.2) ----
//
// Value is a sequence of length-prefixed area addresses: one length octet
// (1..13) followed by that many area-address octets, repeated.

// AreaAddressesTLV is the decoded TLV 1: the list of area addresses an IS
// belongs to. Originated in IIHs (isis-5) and LSPs (isis-6).
type AreaAddressesTLV struct {
	Areas []types.AreaID
}

// DecodeAreaAddressesTLV parses a TLV 1 value. Each entry is a 1-octet length
// followed by that many octets. Bound-checked (security review): a length that
// overruns the value terminates with ErrTruncated and no partial area leaks.
func DecodeAreaAddressesTLV(value []byte) (AreaAddressesTLV, error) {
	var out AreaAddressesTLV
	off := 0
	for off < len(value) {
		alen := int(value[off])
		off++
		if alen < types.MinAreaIDLen || alen > types.MaxAreaIDLen {
			return AreaAddressesTLV{}, ErrLength
		}
		if off+alen > len(value) {
			return AreaAddressesTLV{}, ErrTruncated
		}
		area, err := types.AreaIDFromBytes(value[off : off+alen])
		if err != nil {
			return AreaAddressesTLV{}, err
		}
		out.Areas = append(out.Areas, area)
		off += alen
	}
	return out, nil
}

// EncodedLen returns the TLV 1 value length (sum of 1+areaLen per entry).
func (t AreaAddressesTLV) valueLen() int {
	n := 0
	for _, a := range t.Areas {
		n += 1 + a.Len()
	}
	return n
}

// writeAreaAddressesTLV emits TLV 1 (type+length+value) into buf at off.
func writeAreaAddressesTLV(buf []byte, off int, t AreaAddressesTLV) int {
	vlen := t.valueLen()
	buf[off] = TLVAreaAddresses
	buf[off+1] = byte(vlen)
	off += TLVHeaderLen
	for _, a := range t.Areas {
		buf[off] = byte(a.Len())
		off++
		off += a.WriteTo(buf, off)
	}
	return off
}

// ---- TLV 8: Padding (ISO/IEC 10589 clause 9.10) ----
//
// The value is meaningless padding (zeros) used to pad an IIH to the interface
// MTU for MTU-mismatch detection (clause 8.2.3). The codec encodes a run of
// zero octets of a requested length and exposes the decoded length.

// WritePaddingTLV emits one TLV 8 with n zero value octets (0 <= n <= 255) into
// buf at off and returns the new offset. The padding owner (isis-5) computes
// how many padding TLVs are needed to reach the interface MTU before
// authentication (ISO/IEC 10589 clause 8.2.3) and calls this per TLV;
// buffer-first, assumes room. Exported because the originator lives in a
// sibling package (isis-5), not this codec child.
func WritePaddingTLV(buf []byte, off, n int) int {
	buf[off] = TLVPadding
	buf[off+1] = byte(n)
	off += TLVHeaderLen
	for i := range n {
		buf[off+i] = 0
	}
	return off + n
}

// ---- TLV 9: LSP Entries (ISO/IEC 10589 clause 9.14) ----
//
// Carried in CSNP/PSNP. Each entry is 16 octets: Remaining Lifetime (2),
// LSP ID (8), Sequence Number (4), Checksum (2). (ISO/IEC 10589 clause 9.14
// orders the fields lifetime, LSPID, sequence, checksum.)

// LSPEntryLen is the fixed size of one LSP Entries TLV record.
const LSPEntryLen = types.LifetimeLen + types.LSPIDLen + types.SequenceNumberLen + 2

// LSPEntry summarizes one LSP for CSNP/PSNP synchronization. The checksum is
// the LSP's stored Fletcher checksum (opaque to a CSNP/PSNP, compared as a
// value). isis-7 builds these from the LSDB.
type LSPEntry struct {
	RemainingLifetime types.RemainingLifetime
	LSPID             types.LSPID
	SequenceNumber    types.SequenceNumber
	Checksum          uint16
}

// LSPEntriesTLV is the decoded TLV 9: a list of LSP summaries.
type LSPEntriesTLV struct {
	Entries []LSPEntry
}

// DecodeLSPEntriesTLV parses a TLV 9 value into fixed-size entries. A value
// length that is not a multiple of LSPEntryLen is rejected (ErrLength) so a
// crafted partial entry cannot be read past its bounds.
func DecodeLSPEntriesTLV(value []byte) (LSPEntriesTLV, error) {
	if len(value)%LSPEntryLen != 0 {
		return LSPEntriesTLV{}, ErrLength
	}
	n := len(value) / LSPEntryLen
	out := LSPEntriesTLV{Entries: make([]LSPEntry, 0, n)}
	off := 0
	for range n {
		lifetime, _ := types.RemainingLifetimeFromBytes(value[off : off+types.LifetimeLen])
		off += types.LifetimeLen
		lspid, _ := types.LSPIDFromBytes(value[off : off+types.LSPIDLen])
		off += types.LSPIDLen
		seq, _ := types.SequenceNumberFromBytes(value[off : off+types.SequenceNumberLen])
		off += types.SequenceNumberLen
		cksum := uint16(value[off])<<8 | uint16(value[off+1])
		off += 2
		out.Entries = append(out.Entries, LSPEntry{
			RemainingLifetime: lifetime,
			LSPID:             lspid,
			SequenceNumber:    seq,
			Checksum:          cksum,
		})
	}
	return out, nil
}

// EncodedLen returns the on-wire size of TLV 9 (type+length+value) for this
// entry list: the 2-octet TLV header plus 16 octets per entry. The caller (the
// flooding spec, isis-7) sizes its build buffer with this. The entry count must
// fit one TLV (<= 15 entries); isis-7 chunks larger lists.
func (t LSPEntriesTLV) EncodedLen() int {
	return TLVHeaderLen + len(t.Entries)*LSPEntryLen
}

// WriteLSPEntriesTLV emits TLV 9 (type+length+value) into buf at off and returns
// the new offset. It is the exported entry point the flooding spec (isis-7) uses
// to build CSNP/PSNP bodies via the canonical isis-2 codec rather than
// re-encoding the entry layout. The caller ensures the entry count fits one TLV
// (255/16 = 15 entries); isis-7 splits larger lists across multiple TLV 9s.
func WriteLSPEntriesTLV(buf []byte, off int, t LSPEntriesTLV) int {
	return writeLSPEntriesTLV(buf, off, t)
}

// writeLSPEntry writes one 16-octet LSP entry into buf at off.
func writeLSPEntry(buf []byte, off int, e LSPEntry) int {
	off += e.RemainingLifetime.WriteTo(buf, off)
	off += e.LSPID.WriteTo(buf, off)
	off += e.SequenceNumber.WriteTo(buf, off)
	buf[off] = byte(e.Checksum >> 8)
	buf[off+1] = byte(e.Checksum)
	return off + 2
}

// writeLSPEntriesTLV emits TLV 9 (type+length+value) into buf at off. The
// caller ensures the entry count fits one TLV (255/16 = 15 entries max);
// isis-7 splits across multiple TLV 9s otherwise.
func writeLSPEntriesTLV(buf []byte, off int, t LSPEntriesTLV) int {
	vlen := len(t.Entries) * LSPEntryLen
	buf[off] = TLVLSPEntries
	buf[off+1] = byte(vlen)
	off += TLVHeaderLen
	for _, e := range t.Entries {
		off = writeLSPEntry(buf, off, e)
	}
	return off
}

// ---- TLV 129: Protocols Supported (RFC 1195) ----
//
// Value is a flat list of 1-octet NLPID values (0xCC IPv4, 0x8E IPv6).

// protocolsSupportedTLV is the decoded TLV 129.
type protocolsSupportedTLV struct {
	NLPIDs []uint8
}

// DecodeProtocolsSupportedTLV parses a TLV 129 value (one NLPID per octet).
func DecodeProtocolsSupportedTLV(value []byte) protocolsSupportedTLV {
	out := protocolsSupportedTLV{NLPIDs: make([]uint8, len(value))}
	copy(out.NLPIDs, value)
	return out
}

// ---- TLV 137: Dynamic Hostname (RFC 5301 sec 3) ----
//
// Value is a 1..255 byte hostname with no NUL at its end. RFC 5301 sec 3 says
// the value is encoded in 7-bit ASCII, and this codec does not check that.
// ISISHostnameValidator produces that guarantee at the config boundary
// (internal/component/config/validators.go), and hostnameTLV carries it
// unchanged (internal/plugins/isis/lsdb/encode.go). A value arriving from a PEER
// carries no such guarantee, and the display path filters it instead
// (sanitizeHostname, internal/plugins/isis/show.go).

// writeHostnameTLV emits TLV 137 carrying name into buf at off. The caller
// ensures len(name) is 1..255 (RFC 5301 sec 3); a longer name is a caller
// error. DecodeHostnameTLV is just the raw value, returned as a string by the
// caller; no dedicated decoder struct is needed.
func writeHostnameTLV(buf []byte, off int, name []byte) int {
	return writeTLV(buf, off, TLVDynamicHostname, name)
}

// ---- TLV 240: Point-to-Point Three-Way Adjacency (RFC 5303 sec 3.1) ----
//
// Value is 1, 5, or 15 octets: Adjacency Three-Way State (1) + Extended Local
// Circuit ID (4) + Neighbor System ID (6) + Neighbor Extended Local Circuit
// ID (4). Shorter forms omit the trailing fields.

// AdjThreeWayState is the RFC 5303 three-way adjacency state (sec 2.1, 3.1).
type AdjThreeWayState uint8

// Three-way adjacency state values (RFC 5303 sec 3.1: 0 = Up, 1 = Initializing,
// 2 = Down).
const (
	AdjThreeWayUp           AdjThreeWayState = 0
	AdjThreeWayInitializing AdjThreeWayState = 1
	AdjThreeWayDown         AdjThreeWayState = 2
)

// P2PThreeWayTLV is the decoded TLV 240. HasNeighbor reports whether the
// neighbor fields are present (the 15-octet form); HasCircuitID whether the
// extended local circuit ID is present (the 5- or 15-octet form).
type P2PThreeWayTLV struct {
	State           AdjThreeWayState
	HasCircuitID    bool
	LocalCircuitID  uint32
	HasNeighbor     bool
	NeighborID      types.SystemID
	NeighborCircuit uint32
}

// TLV 240 valid value lengths (RFC 5303 sec 3.1).
const (
	p2pThreeWayLenStateOnly = 1                             // state
	p2pThreeWayLenWithLocal = 1 + 4                         // + extended local circuit ID
	p2pThreeWayLenFull      = 1 + 4 + types.SystemIDLen + 4 // + neighbor ID + neighbor circuit
)

// DecodeP2PThreeWayTLV parses a TLV 240 value (length 1, 5, or 15). Any other
// length is rejected (ErrLength) per RFC 5303 sec 3.1; the codec does not
// validate the state value here (isis-5 discards an invalid state).
func DecodeP2PThreeWayTLV(value []byte) (P2PThreeWayTLV, error) {
	var out P2PThreeWayTLV
	switch len(value) {
	case p2pThreeWayLenStateOnly:
		out.State = AdjThreeWayState(value[0])
	case p2pThreeWayLenWithLocal:
		out.State = AdjThreeWayState(value[0])
		out.HasCircuitID = true
		out.LocalCircuitID = beUint32(value[1:5])
	case p2pThreeWayLenFull:
		out.State = AdjThreeWayState(value[0])
		out.HasCircuitID = true
		out.LocalCircuitID = beUint32(value[1:5])
		out.HasNeighbor = true
		copy(out.NeighborID[:], value[5:5+types.SystemIDLen])
		out.NeighborCircuit = beUint32(value[5+types.SystemIDLen : 5+types.SystemIDLen+4])
	default:
		return P2PThreeWayTLV{}, ErrLength
	}
	return out, nil
}

// valueLen returns the encoded TLV 240 value length implied by the present
// fields.
func (t P2PThreeWayTLV) valueLen() int {
	switch {
	case t.HasNeighbor:
		return p2pThreeWayLenFull
	case t.HasCircuitID:
		return p2pThreeWayLenWithLocal
	default:
		return p2pThreeWayLenStateOnly
	}
}

// writeP2PThreeWayTLV emits TLV 240 into buf at off, writing only the fields
// the struct marks present (matching the 1/5/15-octet forms).
func writeP2PThreeWayTLV(buf []byte, off int, t P2PThreeWayTLV) int {
	vlen := t.valueLen()
	buf[off] = TLVP2PThreeWay
	buf[off+1] = byte(vlen)
	off += TLVHeaderLen
	buf[off] = byte(t.State)
	off++
	if t.HasCircuitID {
		putBeUint32(buf[off:], t.LocalCircuitID)
		off += 4
	}
	if t.HasNeighbor {
		off += t.NeighborID.WriteTo(buf, off)
		putBeUint32(buf[off:], t.NeighborCircuit)
		off += 4
	}
	return off
}

// beUint32 reads a big-endian uint32 from b[:4]. The caller guarantees len>=4.
func beUint32(b []byte) uint32 {
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

// putBeUint32 writes v big-endian into b[:4]. The caller guarantees room.
func putBeUint32(b []byte, v uint32) {
	b[0] = byte(v >> 24)
	b[1] = byte(v >> 16)
	b[2] = byte(v >> 8)
	b[3] = byte(v)
}

// ---- TLV 22: Extended IS Reachability (RFC 5305 sec 3) ----
//
// Each entry (RFC 5305 sec 3): 7-octet neighbor System ID + pseudonode number
// (a SourceID) + 3-octet (24-bit) default metric + 1-octet sub-TLV length +
// the sub-TLV block. Multiple entries may appear in one TLV 22 value. Note the
// metric is 24-bit here, distinct from the 32-bit prefix metric of TLV 135/236.

// extISReachFixedLen is the fixed part of one TLV 22 entry before the sub-TLV
// block: neighbor SourceID (7) + 24-bit metric (3) + sub-TLV length (1) = 11.
const extISReachFixedLen = types.SourceIDLen + types.MetricLen + 1

// Sub-TLV type codes for TLV 22 the spec calls out (RFC 5305 sec 3, sec 5.2.1).
// The decoder retains ANY sub-TLV as an opaque SubTLV; these constants name the
// types RFC 5305 defines, for a reader that inspects a decoded entry.
const (
	SubTLVLinkLocalRemoteID = 4 // RFC 5305 sec 5.2.1 / RFC 5307: link local/remote identifiers
	SubTLVIPv4InterfaceAddr = 6 // RFC 5305 sec 3.2: IPv4 interface address
	SubTLVIPv4NeighborAddr  = 8 // RFC 5305 sec 3.3: IPv4 neighbor address
)

// ExtISReachEntry is one decoded TLV 22 neighbor entry. The 24-bit metric is
// carried in types.Metric (which range-checks the 24-bit bound). SubTLVs are
// retained opaquely, so a sub-TLV type Ze does not interpret stays on the
// decoded entry rather than being dropped (RFC 5305 sec 2).
type ExtISReachEntry struct {
	Neighbor types.SourceID
	Metric   types.Metric
	SubTLVs  []SubTLV
}

// ExtendedISReachTLV is the decoded TLV 22: a list of neighbor entries.
type ExtendedISReachTLV struct {
	Entries []ExtISReachEntry
}

// DecodeExtendedISReachTLV parses a TLV 22 value. Every field is bound-checked
// before slicing (security review): an entry whose declared sub-TLV length
// overruns the value is rejected with ErrTruncated, and the 24-bit metric
// cannot overflow by construction (3 octets). It does NOT cap the metric at
// MAX_PATH_METRIC; that SPF clamp is isis-9's concern (the codec preserves the
// wire value).
func DecodeExtendedISReachTLV(value []byte) (ExtendedISReachTLV, error) {
	var out ExtendedISReachTLV
	off := 0
	for off < len(value) {
		if off+extISReachFixedLen > len(value) {
			return ExtendedISReachTLV{}, ErrTruncated
		}
		neigh, err := types.SourceIDFromBytes(value[off : off+types.SourceIDLen])
		if err != nil {
			return ExtendedISReachTLV{}, err
		}
		off += types.SourceIDLen
		metric, err := types.MetricFromBytes(value[off : off+types.MetricLen])
		if err != nil {
			return ExtendedISReachTLV{}, err
		}
		off += types.MetricLen
		subLen := int(value[off])
		off++
		if off+subLen > len(value) {
			return ExtendedISReachTLV{}, ErrTruncated
		}
		subs, err := decodeSubTLVs(value[off : off+subLen])
		if err != nil {
			return ExtendedISReachTLV{}, err
		}
		off += subLen
		out.Entries = append(out.Entries, ExtISReachEntry{
			Neighbor: neigh,
			Metric:   metric,
			SubTLVs:  subs,
		})
	}
	return out, nil
}
