// Design: docs/architecture/ospf/ospf-1-types.md -- LinkStateID 4-byte LSA identifier
// Related: format.go -- dotted-quad parse and append helpers
// Related: lsakey.go -- LinkStateID participates in the LSDB key
package types

// LinkStateIDLen is the fixed Link State ID width in octets.
const LinkStateIDLen = 4

// LinkStateID is the type-specific 4-octet LSA identifier.
//
// RFC 2328 Appendix A.4.1: "Link State ID" identifies the described piece of
// topology and is interpreted by LS type. It is a fixed comparable array because
// it participates in LSAKey.
type LinkStateID [LinkStateIDLen]byte

// ParseLinkStateID parses a canonical dotted-quad Link State ID.
func ParseLinkStateID(s string) (LinkStateID, error) {
	return parseFixed4[LinkStateID](s)
}

// LinkStateIDFromBytes copies a 4-octet big-endian Link State ID from b.
func LinkStateIDFromBytes(b []byte) (LinkStateID, error) {
	return fixed4FromBytes[LinkStateID](b)
}

// Bytes returns the Link State ID octets as a fresh slice. Hot paths should prefer WriteTo.
func (id LinkStateID) Bytes() []byte { return fixed4Bytes(id) }

// WriteTo writes the 4 big-endian octets into buf at off and returns LinkStateIDLen.
func (id LinkStateID) WriteTo(buf []byte, off int) int {
	return fixed4WriteTo(id, buf, off)
}

// AppendTo appends the canonical dotted-quad form without allocating.
func (id LinkStateID) AppendTo(dst []byte) []byte { return fixed4AppendTo(id, dst) }

// String returns the canonical dotted-quad form.
func (id LinkStateID) String() string { return fixed4String(id) }

// Equal reports whether two Link State IDs are identical.
func (id LinkStateID) Equal(other LinkStateID) bool { return id == other }
