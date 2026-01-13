package capability

// EncodingCaps holds capabilities that affect wire encoding.
// Shared between Negotiated and EncodingContexts (recv/send).
// Immutable after session creation.
type EncodingCaps struct {
	// RFC 6793: Use 4-byte AS numbers when true.
	ASN4 bool

	// RFC 8654: Extended Message Support for BGP.
	// Affects max message size: 4096 (standard) vs 65535 (extended).
	ExtendedMessage bool

	// Negotiated address families (sorted for determinism).
	Families []Family

	// RFC 7911: ADD-PATH modes per family.
	// Mode indicates what we negotiated (Send/Receive/Both).
	AddPathMode map[Family]AddPathMode

	// RFC 8950: Extended next-hop encoding per family.
	// Value is the next-hop AFI (e.g., AFIIPv6 for IPv4 prefix with IPv6 NH).
	ExtendedNextHop map[Family]AFI
}

// SupportsFamily returns true if the family was negotiated.
// RFC 4760: A family is supported only if both peers advertise it.
func (e *EncodingCaps) SupportsFamily(f Family) bool {
	for _, fam := range e.Families {
		if fam == f {
			return true
		}
	}
	return false
}

// AddPathFor returns the negotiated ADD-PATH mode for a family.
// Returns AddPathNone if ADD-PATH is not negotiated for this family.
// RFC 7911 Section 4: Mode determines path ID inclusion.
func (e *EncodingCaps) AddPathFor(f Family) AddPathMode {
	if e.AddPathMode == nil {
		return AddPathNone
	}
	return e.AddPathMode[f]
}

// ExtendedNextHopAFI returns the negotiated next-hop AFI for a family.
// Returns 0 if extended next-hop is not negotiated for this family.
// RFC 8950: When non-zero, the family can use next-hops of the returned AFI.
func (e *EncodingCaps) ExtendedNextHopAFI(f Family) AFI {
	if e.ExtendedNextHop == nil {
		return 0
	}
	return e.ExtendedNextHop[f]
}
