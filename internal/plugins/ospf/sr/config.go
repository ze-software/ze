// Design: docs/architecture/wire/ospf.md -- OSPF Segment Routing shared control
// plane. Resolved SR config plus SRGB/SRLB range validation (non-overlap with
// each other and with the LDP/RSVP-TE label space; Range Size > 0; label bounds).
// Shared by both address families; consumed by the YANG validator and the doctor.
// RFC: rfc/short/rfc8665.md (§3.2/§3.3 Range Size > 0, non-overlapping ranges)

package sr

import (
	"errors"
	"fmt"
	"net/netip"
)

// PrefixSIDConfig is a configured node Prefix-SID: an index into this router's
// SRGB advertised for one local prefix (typically the loopback). NodeSID sets the
// N-Flag on the parent Extended Prefix TLV; NoPHP/ExplicitNull set the NP/E flags
// on the Prefix-SID sub-TLV.
type PrefixSIDConfig struct {
	Prefix       netip.Prefix
	Index        uint32
	NodeSID      bool
	NoPHP        bool
	ExplicitNull bool
}

// SRConfig is the resolved Segment Routing configuration for one OSPF address
// family. The SRGB is the global label block this node owns and advertises; the
// SRLB is the local block Adj-SIDs are allocated from. On originate a single SRGB
// range is the norm; the receive path accepts multiple ranges (RFC 8665 §3.2).
type SRConfig struct {
	Enabled        bool
	SRGB           []LabelRange
	SRLB           []LabelRange
	Prefixes       []PrefixSIDConfig
	SRMSPreference uint8
	HasSRMS        bool
}

// ErrSRNoSRGB reports that SR is enabled without a usable SRGB range.
var ErrSRNoSRGB = errors.New("segment-routing enabled without an SRGB range")

// srgbTotal is the total number of labels the configured SRGB can map.
func (c SRConfig) srgbTotal() uint32 {
	var total uint32
	for _, r := range c.SRGB {
		total += r.Size
	}
	return total
}

// validRange checks a single SRGB/SRLB range against the 20-bit label bounds and
// the 24-bit Range Size field.
func validRange(kind string, r LabelRange) error {
	if r.Size == 0 {
		return fmt.Errorf("%s range at label %d has Range Size 0 (must be > 0)", kind, r.Base)
	}
	if r.Size > MaxRangeSize {
		return fmt.Errorf("%s Range Size %d exceeds the 24-bit maximum %d", kind, r.Size, MaxRangeSize)
	}
	if r.Base < MinLabel {
		return fmt.Errorf("%s base label %d is below the first usable label %d", kind, r.Base, MinLabel)
	}
	if uint64(r.Base)+uint64(r.Size)-1 > uint64(MaxLabel) {
		return fmt.Errorf("%s range %d..%d extends past the 20-bit label maximum %d", kind, r.Base, r.Last(), MaxLabel)
	}
	return nil
}

// noSelfOverlap rejects any two ranges in the same list that share a label.
func noSelfOverlap(kind string, ranges []LabelRange) error {
	for i := range ranges {
		for j := i + 1; j < len(ranges); j++ {
			if ranges[i].overlaps(ranges[j]) {
				return fmt.Errorf("%s ranges %d..%d and %d..%d overlap", kind,
					ranges[i].Base, ranges[i].Last(), ranges[j].Base, ranges[j].Last())
			}
		}
	}
	return nil
}

// noCrossOverlap rejects any range in a that shares a label with any range in b.
func noCrossOverlap(kindA, kindB string, a, b []LabelRange) error {
	for _, ra := range a {
		for _, rb := range b {
			if ra.overlaps(rb) {
				return fmt.Errorf("%s range %d..%d overlaps %s range %d..%d",
					kindA, ra.Base, ra.Last(), kindB, rb.Base, rb.Last())
			}
		}
	}
	return nil
}

// Validate checks the SR configuration. reserved carries any other label ranges
// this node owns (the LDP and RSVP-TE label spaces) that the SRGB/SRLB MUST NOT
// overlap. A disabled SR block always validates.
func (c SRConfig) Validate(reserved []LabelRange) error {
	if !c.Enabled {
		return nil
	}
	if len(c.SRGB) == 0 {
		return ErrSRNoSRGB
	}
	for _, r := range c.SRGB {
		if err := validRange("SRGB", r); err != nil {
			return err
		}
	}
	for _, r := range c.SRLB {
		if err := validRange("SRLB", r); err != nil {
			return err
		}
	}
	if err := noSelfOverlap("SRGB", c.SRGB); err != nil {
		return err
	}
	if err := noSelfOverlap("SRLB", c.SRLB); err != nil {
		return err
	}
	if err := noCrossOverlap("SRGB", "SRLB", c.SRGB, c.SRLB); err != nil {
		return err
	}
	if len(reserved) > 0 {
		if err := noCrossOverlap("SRGB", "reserved", c.SRGB, reserved); err != nil {
			return err
		}
		if err := noCrossOverlap("SRLB", "reserved", c.SRLB, reserved); err != nil {
			return err
		}
	}
	total := c.srgbTotal()
	for _, p := range c.Prefixes {
		if p.Index >= total {
			return fmt.Errorf("prefix-SID index %d for %s is beyond the total SRGB size %d",
				p.Index, p.Prefix, total)
		}
	}
	return nil
}
