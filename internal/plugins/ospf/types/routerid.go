// Design: docs/architecture/ospf/ospf-1-types.md -- RouterID 4-byte OSPF router identifier
// Related: format.go -- dotted-quad parse and append helpers
package types

// RouterIDLen is the fixed OSPF Router ID width in octets.
const RouterIDLen = 4

// RouterID uniquely identifies an OSPF router in the autonomous system.
//
// RFC 2328 Appendix A.3.1: "Router ID" is a 4-octet field in the common OSPF
// header. It is a fixed array so it is comparable with == and usable directly as
// a Go map key, avoiding net.IP slice identity bugs.
type RouterID [RouterIDLen]byte

// ParseRouterID parses a canonical dotted-quad Router ID.
func ParseRouterID(s string) (RouterID, error) { return parseFixed4[RouterID](s) }

// RouterIDFromBytes copies a 4-octet big-endian Router ID from b.
func RouterIDFromBytes(b []byte) (RouterID, error) {
	return fixed4FromBytes[RouterID](b)
}

// Bytes returns the Router ID octets as a fresh slice. Hot paths should prefer WriteTo.
func (id RouterID) Bytes() []byte { return fixed4Bytes(id) }

// WriteTo writes the 4 big-endian octets into buf at off and returns RouterIDLen.
func (id RouterID) WriteTo(buf []byte, off int) int {
	return fixed4WriteTo(id, buf, off)
}

// AppendTo appends the canonical dotted-quad form without allocating.
func (id RouterID) AppendTo(dst []byte) []byte { return fixed4AppendTo(id, dst) }

// String returns the canonical dotted-quad form.
func (id RouterID) String() string { return fixed4String(id) }

// Equal reports whether two Router IDs are identical.
func (id RouterID) Equal(other RouterID) bool { return id == other }
