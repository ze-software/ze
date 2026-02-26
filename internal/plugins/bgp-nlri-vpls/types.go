// Design: docs/architecture/wire/nlri.md — VPLS NLRI plugin
// RFC: rfc/short/rfc4761.md
//
// Package bgp_vpls implements VPLS NLRI (RFC 4761, SAFI 65).
package bgp_nlri_vpls

import (
	"encoding/binary"
	"errors"
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
)

// Type aliases for shared nlri types.
type (
	Family             = nlri.Family
	AFI                = nlri.AFI
	SAFI               = nlri.SAFI
	NLRI               = nlri.NLRI
	RouteDistinguisher = nlri.RouteDistinguisher
)

// Re-export constants.
const (
	AFIL2VPN = nlri.AFIL2VPN
	SAFIVPLS = nlri.SAFIVPLS
	RDType0  = nlri.RDType0
	RDType1  = nlri.RDType1
)

var (
	L2VPNVPLS     = nlri.L2VPNVPLS
	ParseRDString = nlri.ParseRDString
)

// Errors for VPLS parsing.
var ErrVPLSTruncated = errors.New("vpls: truncated data")

// VPLS represents a VPLS NLRI (RFC 4761 Section 3.2.2).
//
// A VPLS BGP NLRI contains information for establishing pseudowires
// between VPLS Edge (VE) devices.
type VPLS struct {
	rd            RouteDistinguisher
	veID          uint16 // VE ID - unique identifier within a VPLS
	veBlockOffset uint16 // Starting VE ID for the label block
	veBlockSize   uint16 // Number of labels in the block
	labelBase     uint32 // 20-bit MPLS label base
	cached        []byte
}

// NewVPLS creates a new VPLS NLRI.
func NewVPLS(rd RouteDistinguisher, veBlockOffset, veBlockSize uint16, labelBase []byte) *VPLS {
	var label uint32
	if len(labelBase) >= 3 {
		label = uint32(labelBase[0])<<12 | uint32(labelBase[1])<<4 | uint32(labelBase[2]>>4)
	}
	return &VPLS{
		rd:            rd,
		veBlockOffset: veBlockOffset,
		veBlockSize:   veBlockSize,
		labelBase:     label,
	}
}

// NewVPLSFull creates a VPLS NLRI with all fields.
func NewVPLSFull(rd RouteDistinguisher, veID, veBlockOffset, veBlockSize uint16, labelBase uint32) *VPLS {
	return &VPLS{
		rd:            rd,
		veID:          veID,
		veBlockOffset: veBlockOffset,
		veBlockSize:   veBlockSize,
		labelBase:     labelBase,
	}
}

// ParseVPLS parses a VPLS NLRI from wire format.
//
// RFC 4761 Section 3.2.2 defines the VPLS NLRI format.
// The minimum NLRI length is 17 bytes (8 RD + 2 VE ID + 2 offset + 2 size + 3 label).
func ParseVPLS(data []byte) (*VPLS, []byte, error) {
	if len(data) < 2 {
		return nil, nil, ErrVPLSTruncated
	}

	nlriLen := int(binary.BigEndian.Uint16(data[:2]))
	if len(data) < 2+nlriLen {
		return nil, nil, ErrVPLSTruncated
	}

	if nlriLen < 17 {
		return nil, nil, ErrVPLSTruncated
	}

	nlriData := data[2 : 2+nlriLen]

	rd, err := nlri.ParseRouteDistinguisher(nlriData[:8])
	if err != nil {
		return nil, nil, err
	}

	vpls := &VPLS{
		rd:            rd,
		veID:          binary.BigEndian.Uint16(nlriData[8:10]),
		veBlockOffset: binary.BigEndian.Uint16(nlriData[10:12]),
		veBlockSize:   binary.BigEndian.Uint16(nlriData[12:14]),
		cached:        data[:2+nlriLen],
	}

	if nlriLen >= 17 {
		vpls.labelBase = uint32(nlriData[14])<<12 | uint32(nlriData[15])<<4 | uint32(nlriData[16]>>4)
	}

	return vpls, data[2+nlriLen:], nil
}

// Family returns the address family.
func (v *VPLS) Family() Family {
	return Family{AFI: AFIL2VPN, SAFI: SAFIVPLS}
}

// RD returns the route distinguisher.
func (v *VPLS) RD() RouteDistinguisher { return v.rd }

// VEID returns the VE ID.
func (v *VPLS) VEID() uint16 { return v.veID }

// VEBlockOffset returns the VE block offset.
func (v *VPLS) VEBlockOffset() uint16 { return v.veBlockOffset }

// VEBlockSize returns the VE block size.
func (v *VPLS) VEBlockSize() uint16 { return v.veBlockSize }

// LabelBase returns the label base value.
func (v *VPLS) LabelBase() uint32 { return v.labelBase }

// Bytes returns the wire-format encoding.
func (v *VPLS) Bytes() []byte {
	if v.cached != nil {
		return v.cached
	}

	v.cached = make([]byte, 19)
	binary.BigEndian.PutUint16(v.cached[0:2], 17)

	copy(v.cached[2:10], v.rd.Bytes())

	binary.BigEndian.PutUint16(v.cached[10:12], v.veID)
	binary.BigEndian.PutUint16(v.cached[12:14], v.veBlockOffset)
	binary.BigEndian.PutUint16(v.cached[14:16], v.veBlockSize)

	v.cached[16] = byte(v.labelBase >> 12)
	v.cached[17] = byte(v.labelBase >> 4)
	v.cached[18] = byte(v.labelBase<<4) | 0x01

	return v.cached
}

// Len returns the length in bytes.
func (v *VPLS) Len() int { return len(v.Bytes()) }

// PathID returns 0.
func (v *VPLS) PathID() uint32 { return 0 }

// HasPathID returns false.
func (v *VPLS) HasPathID() bool { return false }

// SupportsAddPath returns false - VPLS doesn't support ADD-PATH.
func (v *VPLS) SupportsAddPath() bool { return false }

// String returns command-style format for API round-trip compatibility.
func (v *VPLS) String() string {
	return fmt.Sprintf("rd %s ve-id %d label %d", v.rd, v.veID, v.labelBase)
}

// WriteTo writes the VPLS NLRI directly to buf at offset.
func (v *VPLS) WriteTo(buf []byte, off int) int {
	if v.cached != nil {
		return copy(buf[off:], v.cached)
	}

	pos := off

	binary.BigEndian.PutUint16(buf[pos:], 17)
	pos += 2

	pos += v.rd.WriteTo(buf, pos)

	binary.BigEndian.PutUint16(buf[pos:], v.veID)
	pos += 2

	binary.BigEndian.PutUint16(buf[pos:], v.veBlockOffset)
	pos += 2

	binary.BigEndian.PutUint16(buf[pos:], v.veBlockSize)
	pos += 2

	buf[pos] = byte(v.labelBase >> 12)
	buf[pos+1] = byte(v.labelBase >> 4)
	buf[pos+2] = byte(v.labelBase<<4) | 0x01
	pos += 3

	return pos - off
}
