// Design: docs/architecture/wire/attributes.md — path attribute encoding
// RFC: rfc/short/rfc4271.md — Partial bit semantics (Sections 4.3, 5 and 9)
// Related: attribute.go — Recognized, the registry this walk asks
// Related: span.go — SpanIndex, the index built over the bytes this walk rewrites

package attribute

// ClearPartialOnWellKnownAndNonTransitive clears the Partial bit on every
// well-known attribute and every optional non-transitive attribute of a
// path-attribute section, in place, and reports how many attributes it cleared.
//
// RFC 4271 Section 4.3: "For well-known attributes and for optional
// non-transitive attributes, the Partial bit MUST be set to 0."
//
// It is the exact complement of SetPartialOnUnrecognizedTransitive, and the two
// act on disjoint classes, so their order is free. The stamp acts only where the
// Optional AND Transitive bits are both 1; this walk acts only where at least one
// of them is 0. No attribute can be in both sets.
//
// The two class questions are answered from the flags octet the sender wrote,
// because Section 4.3 defines both classes out of that same octet: the Optional
// bit "defines whether the attribute is optional (if set to 1) or well-known (if
// set to 0)", and the Transitive bit "defines whether an optional attribute is
// transitive (if set to 1) or non-transitive (if set to 0)". Reading the octet is
// safe here because RFC 7606 validation has already rejected an octet that
// conflicts with the values the attribute's own specification fixes
// (flags_spec.go, AttributeCode.FlagsConflict), so a class read here is the class
// the specification names. An attribute ze holds no specification for keeps
// whatever octet arrived, which is what RFC 4271 Section 5 requires of it.
//
// One class is deliberately left alone, and leaving it alone is its own
// requirement. An OPTIONAL TRANSITIVE attribute keeps whatever Partial bit
// arrived. RFC 4271 Section 5: a Partial bit "set to 1 by some previous AS ...
// MUST NOT be set back to 0 by the current AS", and Section 9 requires ze to SET
// that bit on an unrecognized one. Clearing it there would undo both.
//
// It writes only bit 0x20 of a flags octet, so it changes no attribute's code,
// length, header size or value, and an index built over the same bytes stays
// valid. It allocates nothing.
//
// A section that stops parsing (a header or a value running past the end) clears
// what it read and returns, for the same reason the stamp does: this runs on the
// receive path AFTER RFC 7606 validation has accepted the UPDATE, and the early
// return exists so an in-process caller handing over malformed bytes cannot walk
// off the end.
func ClearPartialOnWellKnownAndNonTransitive(section []byte) int {
	const optionalTransitive = FlagOptional | FlagTransitive

	cleared := 0
	for pos := 0; pos+3 <= len(section); {
		flags := AttributeFlags(section[pos])

		hdrLen, valLen := 3, int(section[pos+2])
		if flags.IsExtLength() {
			if pos+4 > len(section) {
				return cleared
			}
			hdrLen, valLen = 4, int(section[pos+2])<<8|int(section[pos+3])
		}
		if pos+hdrLen+valLen > len(section) {
			return cleared
		}

		// Partial set, on an attribute that is not optional transitive. The mask
		// comparison is the negation of the stamp's: there, all of Optional,
		// Transitive and a clear Partial had to hold; here it is enough that
		// Optional and Transitive are not both set.
		if flags&FlagPartial != 0 && flags&optionalTransitive != optionalTransitive {
			section[pos] = byte(flags &^ FlagPartial)
			cleared++
		}

		pos += hdrLen + valLen
	}
	return cleared
}

// SetPartialOnUnrecognizedTransitive sets the Partial bit on every unrecognized
// transitive optional attribute of a path-attribute section, in place, and
// reports how many attributes it stamped.
//
// RFC 4271 Section 9: "If an optional transitive attribute is unrecognized, the
// Partial bit (the third high-order bit) in the attribute flags octet is set to
// 1, and the attribute is retained for propagation to other BGP speakers."
// Section 5 states the same obligation from the sending side: "the unrecognized
// transitive optional attribute of that path MUST be passed, along with the path,
// to other BGP peers with the Partial bit in the Attribute Flags octet set to 1."
//
// Three classes are left alone, and each is its own requirement:
//
//   - Well-known attributes and optional NON-transitive attributes. RFC 4271
//     Section 4.3: "For well-known attributes and for optional non-transitive
//     attributes, the Partial bit MUST be set to 0."
//   - RECOGNIZED optional transitive attributes. Section 5 stamps the bit for
//     attributes the speaker could not read; stamping one ze did read would state
//     that the information is partial when it is not.
//   - An attribute that already carries the bit. RFC 4271 Section 5: a Partial bit
//     "set to 1 by some previous AS ... MUST NOT be set back to 0 by the current
//     AS". The bit is only ever OR-ed in here, so no input clears it.
//
// It writes only bit 0x20 of a flags octet, so it changes no attribute's code,
// length, header size or value, and an index built over the same bytes stays
// valid. It allocates nothing.
//
// A section that stops parsing (a header or a value running past the end) stamps
// what it read and returns. This runs on the receive path AFTER RFC 7606
// validation has accepted the UPDATE, so a section that does not walk cleanly here
// is not reachable from the wire; the early return exists so an in-process caller
// handing over malformed bytes cannot walk off the end.
func SetPartialOnUnrecognizedTransitive(section []byte) int {
	const optionalTransitive = FlagOptional | FlagTransitive

	stamped := 0
	for pos := 0; pos+3 <= len(section); {
		flags := AttributeFlags(section[pos])
		code := AttributeCode(section[pos+1])

		hdrLen, valLen := 3, int(section[pos+2])
		if flags.IsExtLength() {
			if pos+4 > len(section) {
				return stamped
			}
			hdrLen, valLen = 4, int(section[pos+2])<<8|int(section[pos+3])
		}
		if pos+hdrLen+valLen > len(section) {
			return stamped
		}

		// Optional AND transitive AND Partial still 0. The mask tests all three in
		// one comparison, so no ordering of the bits can make a partial match pass.
		if flags&(optionalTransitive|FlagPartial) == optionalTransitive && !code.Recognized() {
			section[pos] = byte(flags | FlagPartial)
			stamped++
		}

		pos += hdrLen + valLen
	}
	return stamped
}
