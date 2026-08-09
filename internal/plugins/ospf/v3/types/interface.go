// Design: docs/architecture/ospf/ospfv3-1-types.md -- InterfaceID 32-bit router-local interface id.
// RFC: rfc/short/rfc5340.md (§A.3.2 Hello, §A.4.3 Router-LSA -- Interface ID)
//
// OSPFv3 identifies each local interface by a 32-bit Interface ID, not by its IP subnet
// (RFC 5340 §2.1 runs OSPF per link). The Interface ID appears in Hello, Router-LSA,
// Network-LSA, Link-LSA, and the SPF graph.

package types

// InterfaceID is a router-local 32-bit interface identifier.
type InterfaceID uint32

// InterfaceIDFromBytes reads a 4-octet big-endian Interface ID from b.
func InterfaceIDFromBytes(b []byte) (InterfaceID, error) {
	if len(b) != 4 {
		return 0, ErrWrongLength
	}
	return InterfaceID(uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])), nil
}

// WriteTo writes the 4 big-endian octets into buf at off and returns 4.
func (id InterfaceID) WriteTo(buf []byte, off int) int {
	return writeUint32(buf, off, uint32(id))
}

// IsActive reports whether the Interface ID identifies an active interface (non-zero).
// Zero is reserved for placeholder contexts (e.g. a not-yet-assigned interface).
func (id InterfaceID) IsActive() bool { return id != 0 }
