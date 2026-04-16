// Design: docs/architecture/wire/nlri.md — VPLS NLRI plugin
// RFC: rfc/short/rfc4761.md
//
// Package bgp_vpls implements VPLS NLRI (RFC 4761, SAFI 65).
package vpls

import (
	"encoding/binary"
	"errors"
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// Type aliases for shared nlri types.
type (
	Family             = family.Family
	AFI                = family.AFI
	SAFI               = family.SAFI
	NLRI               = nlri.NLRI
	RouteDistinguisher = nlri.RouteDistinguisher
)

// Re-export constants.
const (
	AFIL2VPN = family.AFIL2VPN
	SAFIVPLS = family.SAFIVPLS
	RDType0  = nlri.RDType0
	RDType1  = nlri.RDType1
)

// Family registration for VPLS.
var L2VPNVPLS = family.MustRegister(AFIL2VPN, SAFIVPLS, "l2vpn", "vpls")

var ParseRDString = nlri.ParseRDString

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
	buf := make([]byte, v.Len())
	v.WriteTo(buf, 0)
	return buf
}

// Len returns the length in bytes (2-byte length prefix + 17-byte fixed NLRI).
func (v *VPLS) Len() int { return 19 }

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
