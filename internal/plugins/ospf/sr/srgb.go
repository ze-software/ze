// Design: docs/architecture/wire/ospf.md -- OSPF Segment Routing shared control
// plane. SRGB index->label arithmetic across ordered ranges (RFC 8665 §3.2,
// RFC 8666 reuses the same capability semantics). Shared by both address families.
// RFC: rfc/short/rfc8665.md (§3.2 SID/Label Range, index->label mapping order)

// Package sr holds the address-family-neutral control plane for OSPF Segment
// Routing (RFC 8665 for the IPv4 family, RFC 8666 for the IPv6 family). The SRGB
// index->label arithmetic, the SRLB local-label allocator, the SR configuration
// and its range validation, the NP/E/M forwarding truth table, and the SR TLV
// wire codecs live here so both OSPF address families share one implementation.
// Only the LSA carriage (which OSPF opaque / Extended LSA carries the TLVs)
// differs by address family and is wired from the engine package.
package sr

// MPLS label space bounds (RFC 3031 / RFC 3032). Labels 0..15 are reserved; a
// usable SID/Label range therefore starts at 16 and the 20-bit label space caps
// at 2^20-1.
const (
	// MinLabel is the first non-reserved MPLS label.
	MinLabel uint32 = 16
	// MaxLabel is the largest 20-bit MPLS label (2^20-1).
	MaxLabel uint32 = 1048575
	// MaxRangeSize is the largest SRGB/SRLB Range Size (24-bit wire field,
	// RFC 8665 §3.2/§3.3).
	MaxRangeSize uint32 = 16777215
)

// LabelRange is one contiguous SID/Label range: Base is the first label in the
// range and Size is the number of labels it covers (RFC 8665 §3.2 "Range Size ...
// Number of SIDs/labels including the first"). Size MUST be greater than zero.
type LabelRange struct {
	Base uint32
	Size uint32
}

// Last returns the last label covered by the range.
func (r LabelRange) Last() uint32 { return r.Base + r.Size - 1 }

// contains reports whether label falls inside the range.
func (r LabelRange) contains(label uint32) bool {
	return r.Size > 0 && label >= r.Base && label <= r.Last()
}

// overlaps reports whether two label ranges share any label.
func (r LabelRange) overlaps(o LabelRange) bool {
	if r.Size == 0 || o.Size == 0 {
		return false
	}
	return r.Base <= o.Last() && o.Base <= r.Last()
}

// SRGB is an originator's Segment Routing Global Block: an ordered list of
// SID/Label ranges. A SID index is mapped to a label by concatenating the ranges
// in their advertised order (RFC 8665 §3.2: "The receiving router MUST adhere to
// the advertised order of the ... ranges when calculating a SID/Label from a SID
// index"). The order is significant and is preserved verbatim.
type SRGB struct {
	ranges []LabelRange
}

// NewSRGB builds an SRGB from ranges in advertised order. The slice is copied so
// the SRGB owns no caller memory.
func NewSRGB(ranges []LabelRange) SRGB {
	if len(ranges) == 0 {
		return SRGB{}
	}
	cp := make([]LabelRange, len(ranges))
	copy(cp, ranges)
	return SRGB{ranges: cp}
}

// Ranges returns the SRGB ranges in advertised order (read-only view).
func (g SRGB) Ranges() []LabelRange { return g.ranges }

// Empty reports whether the SRGB carries no ranges.
func (g SRGB) Empty() bool { return len(g.ranges) == 0 }

// TotalSize is the number of labels the SRGB can map (the sum of the range
// sizes). It is the exclusive upper bound on a valid SID index.
func (g SRGB) TotalSize() uint32 {
	var total uint32
	for _, r := range g.ranges {
		total += r.Size
	}
	return total
}

// Label maps a SID index to its MPLS label by walking the ranges in advertised
// order and accumulating the prior range sizes: label = range_base + (index -
// cumulative_prior_sizes) for the range covering index (RFC 8665 §3.2). It
// returns false when the index is at or beyond the total range size, in which
// case no label is installed. Arithmetic only; no allocation.
func (g SRGB) Label(index uint32) (uint32, bool) {
	var cumulative uint32
	for _, r := range g.ranges {
		if r.Size == 0 {
			continue
		}
		if index < cumulative+r.Size {
			return r.Base + (index - cumulative), true
		}
		cumulative += r.Size
	}
	return 0, false
}
