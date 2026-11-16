// Design: docs/architecture/wire/attributes.md -- shared MP next-hop field contract
// Related: mpnlri.go -- typed MP_REACH codec and built-in profile declarations
// RFC 4760 Section 3 -- see rfc/short/rfc4760.md.
// RFC 4364 Section 4.3.4 -- see rfc/short/rfc4364.md.

package attribute

import (
	"fmt"
	"net/netip"
	"slices"
)

// mpNextHopProfiles holds optional plugin-owned declarations. Registration is
// init-only and is not safe concurrently with lookup. Reads after init allocate
// nothing and require no lock. Stored length slices are shared read-only data.
var mpNextHopProfiles = make(map[uint32]NextHopProfile)

// RegisterMPNextHopProfile installs a plugin-owned field contract. Callers MUST
// register only during package initialization, before sessions or concurrent
// readers start. The registry copies input lengths once and never changes them.
// A plugin cannot replace a built-in profile or an earlier registration.
func RegisterMPNextHopProfile(afi AFI, safi SAFI, profile NextHopProfile) error {
	key := uint32(afi)<<8 | uint32(safi)
	if _, exists := mpNextHopProfiles[key]; exists {
		return fmt.Errorf("attribute: MP next-hop profile %d/%d already registered", afi, safi)
	}
	builtin := builtinMPNextHopProfile(afi, safi)
	if len(builtin.Lengths) != 0 {
		return fmt.Errorf("attribute: MP next-hop profile %d/%d is built in", afi, safi)
	}
	if builtin.ExtendedIPv6 {
		return fmt.Errorf("attribute: MP next-hop profile %d/%d is built in", afi, safi)
	}
	if err := profile.validateRegistration(); err != nil {
		return fmt.Errorf("attribute: MP next-hop profile %d/%d: %w", afi, safi, err)
	}
	profile.Lengths = slices.Clone(profile.Lengths)
	mpNextHopProfiles[key] = profile
	return nil
}

// validateRegistration refuses layouts the common codec cannot represent and
// declarations whose mapping permission or default has no admitted wire form.
func (p NextHopProfile) validateRegistration() error {
	if len(p.Lengths) == 0 {
		return fmt.Errorf("declare at least one next-hop length")
	}
	for i, width := range p.Lengths {
		if slices.Contains(p.Lengths[:i], width) {
			return fmt.Errorf("duplicate next-hop length %d", width)
		}
		if p.RDPerAddress {
			switch width {
			case 12, 24, 48:
				continue
			default:
				return fmt.Errorf("unsupported RD-framed next-hop length %d", width)
			}
		}
		switch width {
		case 4, 16, 32:
		default:
			return fmt.Errorf("unsupported plain next-hop length %d", width)
		}
	}
	if p.EncodeIPv4Mapped {
		if !p.MappedIPv4 {
			return fmt.Errorf("default IPv4 mapping requires mapped-address admission")
		}
	}
	if p.MappedIPv4 {
		mappedWidth := 16
		if p.RDPerAddress {
			mappedWidth += RDSize
		}
		if !slices.Contains(p.Lengths, mappedWidth) {
			return fmt.Errorf("mapped-address admission requires next-hop length %d", mappedWidth)
		}
	}
	if p.DecodeUnprefixedIPv6 {
		if !p.RDPerAddress {
			return fmt.Errorf("unprefixed IPv6 decoding requires RD framing")
		}
	}
	return nil
}

// SelectAddress chooses the emitted representation once, before admission,
// sizing or writing. An explicit request works for unknown plugin profiles as
// well. MappedIPv4 grants admission, not conversion. Only EncodeIPv4Mapped or
// mapV4 requests conversion. Already mapped and native IPv6 inputs are retained.
func (p NextHopProfile) SelectAddress(address netip.Addr, mapV4 bool) netip.Addr {
	if !address.Is4() {
		return address
	}
	if p.EncodeIPv4Mapped {
		return netip.AddrFrom16(address.As16())
	}
	if mapV4 {
		return netip.AddrFrom16(address.As16())
	}
	return address
}

// NextHopOctets counts one already-selected address and its optional RD. An
// unset address has no wire form and contributes no RD. Callers MUST apply
// SelectAddress before sizing; this method never changes address form.
func (p NextHopProfile) NextHopOctets(address netip.Addr) int {
	width := address.BitLen() / 8
	if width == 0 {
		return 0
	}
	if p.RDPerAddress {
		width += RDSize
	}
	return width
}

// NextHopLen counts the complete field of already-selected addresses. The zero
// second address denotes a single-address field. This is framing, not admission.
func (p NextHopProfile) NextHopLen(first, second netip.Addr) int {
	return p.NextHopOctets(first) + p.NextHopOctets(second)
}

// WriteNextHops writes already-selected addresses into the caller's buffer and
// returns bytes written. Callers MUST supply space for NextHopLen. Private
// templates and unchecked codecs may encode before admission; an outbound
// producer MUST establish family, role and session admission before sending
// the selected field. RFC 4364 Section 4.3.4: "The Route
// Distinguisher component of the Next Hop field SHALL be set to all zeros."
// RD-framed profiles write that zero prefix before each address, including the
// link-local one. The writer never chooses mapping or changes received form.
func (p NextHopProfile) WriteNextHops(buf []byte, off int, first, second netip.Addr) int {
	pos := off
	for _, address := range [2]netip.Addr{first, second} {
		if !address.IsValid() {
			continue
		}
		if p.RDPerAddress {
			clear(buf[pos : pos+RDSize])
			pos += RDSize
		}
		if address.Is4() {
			ip := address.As4()
			pos += copy(buf[pos:], ip[:])
			continue
		}
		ip := address.As16()
		pos += copy(buf[pos:], ip[:])
	}
	return pos - off
}

// AdmitsNextHopOctets checks the complete-field widths the delta writer can
// materialize, followed by the known family's declaration. An unknown profile
// retains the six generic widths, without granting address or session admission.
func (p NextHopProfile) AdmitsNextHopOctets(octets int) bool {
	switch octets {
	case 4, 12, 16, 24, 32, 48:
	default:
		return false
	}
	if len(p.Lengths) == 0 {
		return true
	}
	return slices.Contains(p.Lengths, octets)
}

// DecodeNextHopField classifies one complete forwarding field. Known profiles
// retain their declared framing and codec-only unprefixed IPv6 compatibility.
// Unknown profiles retain the generic six-width field interpretation regardless
// of AFI. DecodeNextHops separately preserves the typed codec's address lists.
// Neither method grants outbound family or session admission.
func (p NextHopProfile) DecodeNextHopField(afi AFI, data []byte) (NextHopAddrs, error) {
	if p.RDPerAddress {
		return p.DecodeNextHops(afi, data)
	}
	if len(p.Lengths) == 0 {
		switch len(data) {
		case 12, 24, 48:
			return decodeMPNextHopRD(data)
		}
	}
	switch len(data) {
	case 0, 4, 16, 32:
		return p.DecodeNextHops(afi, data)
	default:
		return NextHopAddrs{}, ErrInvalidNextHopLen
	}
}

// DecodeNextHops reads the typed codec's address list into inline storage. It neither
// normalizes mapped addresses nor rewrites raw fields. Received RD values are
// skipped. Zero-RD emission does not introduce a new receive refusal policy.
// Known RD profiles use their declared framing. Only a profile explicitly
// declaring DecodeUnprefixedIPv6 retains the legacy codec-only IPv6 form.
// Unknown profiles retain the existing AFI/length-based generic decoder.
func (p NextHopProfile) DecodeNextHops(afi AFI, data []byte) (NextHopAddrs, error) {
	if len(data) == 0 {
		return NextHopAddrs{}, nil
	}
	if p.RDPerAddress {
		if p.DecodeUnprefixedIPv6 {
			if len(data) == 16 {
				return decodeMPNextHopAddresses(data, 0, 16, 16, 1), nil
			}
			if len(data) == 32 {
				return decodeMPNextHopAddresses(data, 0, 16, 16, 2), nil
			}
		}
		return decodeMPNextHopRD(data)
	}
	// RFC 8950 Section 3: the field length determines its address protocol,
	// independently of the NLRI AFI. Preserve mapped addresses as received.
	switch len(data) {
	case 16:
		return decodeMPNextHopAddresses(data, 0, 16, 16, 1), nil
	case 32:
		return decodeMPNextHopAddresses(data, 0, 16, 16, 2), nil
	}
	switch afi {
	case AFIIPv4:
		if len(data)%4 != 0 {
			return NextHopAddrs{}, ErrInvalidNextHopLen
		}
		// The typed attribute has always retained at most two addresses, even
		// when its generic IPv4 decoder consumes a longer complete field.
		return decodeMPNextHopAddresses(data, 0, 4, 4, min(len(data)/4, 2)), nil
	case AFIIPv6:
		if len(p.Lengths) != 0 {
			if !slices.Contains(p.Lengths, 4) {
				return NextHopAddrs{}, ErrInvalidNextHopLen
			}
		}
	case AFIL2VPN:
		// EVPN and independently registered layouts retain native IPv4.
	default:
		// The AFI set is open. Retain generic four-octet address decoding.
	}
	if len(data) != 4 {
		return NextHopAddrs{}, ErrInvalidNextHopLen
	}
	return decodeMPNextHopAddresses(data, 0, 4, 4, 1), nil
}

// decodeMPNextHopRD owns the RD-per-address layouts for both codec contracts.
// Received RDs are skipped without normalization or an added refusal policy.
func decodeMPNextHopRD(data []byte) (NextHopAddrs, error) {
	switch len(data) {
	case 12:
		return decodeMPNextHopAddresses(data, RDSize, 4, 12, 1), nil
	case 24:
		return decodeMPNextHopAddresses(data, RDSize, 16, 24, 1), nil
	case 48:
		return decodeMPNextHopAddresses(data, RDSize, 16, 24, 2), nil
	default:
		return NextHopAddrs{}, ErrInvalidNextHopLen
	}
}

// decodeMPNextHopAddresses receives offsets and bounds already established by
// its decoder. The two-slot result never borrows or allocates slice storage.
func decodeMPNextHopAddresses(data []byte, off, width, stride, count int) NextHopAddrs {
	var hops NextHopAddrs
	hops.count = uint8(count)
	for i := range count {
		pos := off + i*stride
		if width == 4 {
			hops.addrs[i] = netip.AddrFrom4([4]byte(data[pos : pos+4]))
			continue
		}
		hops.addrs[i] = netip.AddrFrom16([16]byte(data[pos : pos+16]))
	}
	return hops
}
