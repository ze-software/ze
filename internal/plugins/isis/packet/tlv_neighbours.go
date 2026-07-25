// Design: plan/learned/928-isis-2-wire.md -- TLV 6 (IS Neighbors, SNPA list) + TLV 2 (narrow IS Reachability, decode-only)
// ISO/IEC 10589 clause 9.4 (TLV 6 IS Neighbors), clause 9.3 (TLV 2 IS Reachability).

package packet

import "github.com/ze-software/ze/internal/plugins/isis/types"

// SNPALen is the length of a Subnetwork Point of Attachment address (a 48-bit
// IEEE 802 MAC) carried in TLV 6.
const SNPALen = 6

// ---- TLV 6: IS Neighbors (ISO/IEC 10589 clause 9.4) ----
//
// On a LAN, TLV 6 lists the SNPAs (MAC addresses) the originator has heard
// Hellos from. It is the basis for LAN three-way adjacency detection: an IS
// declares an adjacency Up once it sees its own SNPA echoed in a neighbor's
// LAN IIH (isis-5). The value is a flat list of 6-octet SNPA addresses.

// ISNeighborsTLV is the decoded TLV 6: the list of neighbor SNPAs (MACs).
type ISNeighborsTLV struct {
	SNPAs [][SNPALen]byte
}

// DecodeISNeighborsTLV parses a TLV 6 value (one 6-octet SNPA per entry). A
// value length that is not a multiple of SNPALen is rejected (ErrLength) so a
// crafted partial address cannot be read past its bounds.
func DecodeISNeighborsTLV(value []byte) (ISNeighborsTLV, error) {
	if len(value)%SNPALen != 0 {
		return ISNeighborsTLV{}, ErrLength
	}
	n := len(value) / SNPALen
	out := ISNeighborsTLV{SNPAs: make([][SNPALen]byte, 0, n)}
	for off := 0; off < len(value); off += SNPALen {
		var snpa [SNPALen]byte
		copy(snpa[:], value[off:off+SNPALen])
		out.SNPAs = append(out.SNPAs, snpa)
	}
	return out, nil
}

// valueLen returns the encoded TLV 6 value length.
func (t ISNeighborsTLV) valueLen() int { return len(t.SNPAs) * SNPALen }

// writeISNeighborsTLV emits TLV 6 (type+length+value) into buf at off. The
// caller ensures the count fits one TLV (255/6 = 42 SNPAs max). Buffer-first.
func writeISNeighborsTLV(buf []byte, off int, t ISNeighborsTLV) int {
	vlen := t.valueLen()
	buf[off] = TLVISNeighbors
	buf[off+1] = byte(vlen)
	off += TLVHeaderLen
	for _, snpa := range t.SNPAs {
		off += copy(buf[off:], snpa[:])
	}
	return off
}

// ---- TLV 2: IS Reachability, narrow metric (ISO/IEC 10589 clause 9.3) ----
//
// DECODE-ONLY (spec AC-14): Ze never originates TLV 2 (it originates the wide
// TLV 22 instead), but it must parse a peer's TLV 2 without panicking for
// interop. The value begins with one octet of "virtual flag" + reserved, then
// per-neighbor entries of: default metric (1 octet: I/E bit + 6-bit value),
// delay metric (1), expense metric (1), error metric (1), neighbor ID (7) = 11
// octets per entry. Only the default metric's low 6 bits and the I/E
// (internal/external) bit are meaningful here; the other metrics carry a
// supported/unsupported high bit.

// narrowMetricEntryLen is the size of one TLV 2 entry following the leading
// virtual-flag octet: 4 metric octets + 7-octet neighbor ID.
const narrowMetricEntryLen = 4 + types.SourceIDLen

// NarrowISReachEntry is one decoded TLV 2 entry. Only the fields a wide-metric
// IS needs for interop are surfaced; the delay/expense/error metrics are kept
// raw. DefaultMetricValue is the 6-bit metric; DefaultMetricInternal reports
// the I/E bit (0 = internal, 1 = external) of the default metric octet.
type NarrowISReachEntry struct {
	DefaultMetricValue    uint8 // low 6 bits of the default metric octet
	DefaultMetricExternal bool  // I/E bit of the default metric octet (set = external)
	DelayMetric           uint8
	ExpenseMetric         uint8
	ErrorMetric           uint8
	Neighbor              types.SourceID
}

// NarrowISReachTLV is the decoded TLV 2 (decode-only). VirtualFlag is the
// leading octet preceding the entries (ISO/IEC 10589 clause 9.3).
type NarrowISReachTLV struct {
	VirtualFlag uint8
	Entries     []NarrowISReachEntry
}

// narrowMetricValueMask isolates the 6-bit metric value; the top two bits are
// the supported (S) and internal/external (I/E) flags (ISO/IEC 10589 clause
// 9.3).
const (
	narrowMetricValueMask  = 0x3f
	narrowMetricExternalIE = 0x40 // I/E bit of the default metric octet
)

// DecodeNarrowISReachTLV parses a TLV 2 value (decode-only, AC-14). The value
// must be at least 1 octet (the virtual flag) and the remainder a whole number
// of 11-octet entries; anything else is rejected with ErrLength rather than
// reading past the buffer. This never panics on arbitrary input (R-3).
func DecodeNarrowISReachTLV(value []byte) (NarrowISReachTLV, error) {
	if len(value) < 1 {
		return NarrowISReachTLV{}, ErrLength
	}
	rest := value[1:]
	if len(rest)%narrowMetricEntryLen != 0 {
		return NarrowISReachTLV{}, ErrLength
	}
	out := NarrowISReachTLV{VirtualFlag: value[0]}
	n := len(rest) / narrowMetricEntryLen
	out.Entries = make([]NarrowISReachEntry, 0, n)
	off := 0
	for range n {
		def := rest[off]
		entry := NarrowISReachEntry{
			DefaultMetricValue:    def & narrowMetricValueMask,
			DefaultMetricExternal: def&narrowMetricExternalIE != 0,
			DelayMetric:           rest[off+1],
			ExpenseMetric:         rest[off+2],
			ErrorMetric:           rest[off+3],
		}
		neigh, err := types.SourceIDFromBytes(rest[off+4 : off+4+types.SourceIDLen])
		if err != nil {
			return NarrowISReachTLV{}, err
		}
		entry.Neighbor = neigh
		out.Entries = append(out.Entries, entry)
		off += narrowMetricEntryLen
	}
	return out, nil
}
