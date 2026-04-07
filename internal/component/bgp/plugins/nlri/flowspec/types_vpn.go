// Design: docs/architecture/wire/nlri-flowspec.md — FlowSpec VPN (SAFI 134)
// RFC: rfc/short/rfc5575.md
// Overview: types.go — core FlowSpec types, constants, and interface
// Related: types_prefix.go — prefix component implementations
// Related: types_numeric.go — numeric/bitmask component implementations

package flowspec

import (
	"fmt"
)

// ============================================================================
// FlowSpec VPN (RFC 8955 Section 8, SAFI 134)
// ============================================================================

// FlowSpecVPN wraps FlowSpec with a Route Distinguisher for VPN use.
// RFC 8955 Section 8 defines the VPNv4 Flow Specification (AFI=1, SAFI=134).
// The NLRI format per RFC 8955 Figure 7:
//
//	+--------------------------------+
//	| length (0xnn or 0xfnnn)        |
//	+--------------------------------+
//	| Route Distinguisher (8 octets) |
//	+--------------------------------+
//	|    NLRI value  (variable)      |
//	+--------------------------------+
type FlowSpecVPN struct {
	rd       RouteDistinguisher
	flowSpec *FlowSpec
	cached   []byte
}

// NewFlowSpecVPN creates a new FlowSpec VPN NLRI (SAFI 134).
// RFC 8955 Section 8: "This document defines an additional BGP NLRI type
// (AFI=1, SAFI=134) value, which can be used to propagate Flow Specification
// in a BGP/MPLS VPN environment.".
func NewFlowSpecVPN(fam Family, rd RouteDistinguisher) *FlowSpecVPN {
	// Convert SAFI to FlowSpecVPN if needed
	fsFamily := fam
	if fam.SAFI == SAFIFlowSpecVPN {
		fsFamily = Family{AFI: fam.AFI, SAFI: SAFIFlowSpec}
	}
	return &FlowSpecVPN{
		rd:       rd,
		flowSpec: NewFlowSpec(fsFamily),
	}
}

// Family returns the address family (with SAFI 134).
func (f *FlowSpecVPN) Family() Family {
	return Family{AFI: f.flowSpec.family.AFI, SAFI: SAFIFlowSpecVPN}
}

// RD returns the Route Distinguisher.
func (f *FlowSpecVPN) RD() RouteDistinguisher {
	return f.rd
}

// FlowSpec returns the underlying FlowSpec.
func (f *FlowSpecVPN) FlowSpec() *FlowSpec {
	return f.flowSpec
}

// AddComponent adds a component to the FlowSpec.
func (f *FlowSpecVPN) AddComponent(c FlowComponent) {
	f.flowSpec.AddComponent(c)
	f.cached = nil
}

// Components returns the FlowSpec components.
func (f *FlowSpecVPN) Components() []FlowComponent {
	return f.flowSpec.Components()
}

// Bytes returns the wire-format encoding per RFC 8955 Section 8.
// Format: Length (1-2 bytes) + RD (8 bytes) + FlowSpec components.
// RFC 8955 Section 8: "The NLRI length field shall include both the
// 8 octets of the Route Distinguisher as well as the subsequent
// Flow Specification NLRI value.".
func (f *FlowSpecVPN) Bytes() []byte {
	if f.cached != nil {
		return f.cached
	}

	// Get component bytes (without FlowSpec length prefix)
	compBytes := f.flowSpec.ComponentBytes()

	// Total payload = RD (8) + components per RFC 8955 Section 8
	payloadLen := 8 + len(compBytes)

	// Build with length prefix per RFC 8955 Section 4.1
	if payloadLen < 240 {
		f.cached = make([]byte, 1+payloadLen)
		f.cached[0] = byte(payloadLen)
		copy(f.cached[1:9], f.rd.Bytes())
		copy(f.cached[9:], compBytes)
	} else {
		f.cached = make([]byte, 2+payloadLen)
		f.cached[0] = 0xF0 | byte(payloadLen>>8)
		f.cached[1] = byte(payloadLen)
		copy(f.cached[2:10], f.rd.Bytes())
		copy(f.cached[10:], compBytes)
	}

	return f.cached
}

// Len returns the length in bytes.
func (f *FlowSpecVPN) Len() int {
	return len(f.Bytes())
}

// PathID returns 0 (FlowSpecVPN doesn't use ADD-PATH).
func (f *FlowSpecVPN) PathID() uint32 {
	return 0
}

// HasPathID returns false.
func (f *FlowSpecVPN) HasPathID() bool {
	return false
}

// SupportsAddPath returns false - FlowSpec VPN doesn't support ADD-PATH per RFC 8955.
func (f *FlowSpecVPN) SupportsAddPath() bool {
	return false
}

// String returns command-style format for API round-trip compatibility.
// Format: flow-vpn rd <rd> <components>.
func (f *FlowSpecVPN) String() string {
	compStr := f.flowSpec.ComponentString()
	if compStr == "" {
		return fmt.Sprintf("flow-vpn rd %s", f.rd)
	}
	return fmt.Sprintf("flow-vpn rd %s %s", f.rd, compStr)
}

// ParseFlowSpecVPN parses a FlowSpec VPN from wire format per RFC 8955 Section 8.
// The NLRI consists of length + Route Distinguisher (8 octets) + FlowSpec components.
func ParseFlowSpecVPN(fam Family, data []byte) (*FlowSpecVPN, error) {
	if len(data) == 0 {
		return nil, ErrFlowSpecTruncated
	}

	// Parse length per RFC 8955 Section 4.1
	nlriLen := int(data[0])
	offset := 1
	if nlriLen >= 240 {
		if len(data) < 2 {
			return nil, ErrFlowSpecTruncated
		}
		nlriLen = int(data[0]&0x0F)<<8 | int(data[1])
		offset = 2
	}

	if len(data) < offset+nlriLen {
		return nil, ErrFlowSpecTruncated
	}

	// Need at least 8 bytes for RD per RFC 8955 Section 8
	if nlriLen < 8 {
		return nil, ErrFlowSpecTruncated
	}

	// Parse Route Distinguisher (RFC 4364)
	rd, err := ParseRouteDistinguisher(data[offset : offset+8])
	if err != nil {
		return nil, err
	}

	// Parse FlowSpec components (remaining data after RD)
	fsFamily := Family{AFI: fam.AFI, SAFI: SAFIFlowSpec}
	fs := NewFlowSpec(fsFamily)

	remaining := data[offset+8 : offset+nlriLen]
	for len(remaining) > 0 {
		comp, rest, err := parseFlowComponent(remaining, fsFamily)
		if err != nil {
			return nil, err
		}
		fs.components = append(fs.components, comp)
		remaining = rest
	}

	return &FlowSpecVPN{
		rd:       rd,
		flowSpec: fs,
	}, nil
}

// WriteTo writes the FlowSpecVPN NLRI directly to buf at offset (zero-alloc).
// RFC 8955 Section 8: Length + RD (8 bytes) + sorted components.
func (f *FlowSpecVPN) WriteTo(buf []byte, off int) int {
	// Fallback: if we have cached bytes but no components, use cached bytes
	if len(f.flowSpec.components) == 0 && f.cached != nil {
		return copy(buf[off:], f.cached)
	}

	pos := off

	// Calculate payload length: RD (8) + components
	compLen := f.flowSpec.componentLen()
	payloadLen := 8 + compLen

	// Write NLRI length prefix per RFC 8955 Section 4.1
	if payloadLen < 240 {
		buf[pos] = byte(payloadLen)
		pos++
	} else {
		buf[pos] = 0xF0 | byte(payloadLen>>8)
		buf[pos+1] = byte(payloadLen)
		pos += 2
	}

	// Write Route Distinguisher (8 bytes)
	pos += f.rd.WriteTo(buf, pos)

	// Write components in sorted order
	pos += f.flowSpec.writeComponentsSorted(buf, pos)

	return pos - off
}
