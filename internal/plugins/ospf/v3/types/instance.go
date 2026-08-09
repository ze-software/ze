// Design: docs/architecture/ospf/ospfv3-1-types.md -- InstanceID 8-bit link-local instance selector.
// RFC: rfc/short/rfc5340.md (§A.3.1 common header Instance ID)
//
// OSPFv3 carries an 8-bit Instance ID in the common header with link-local significance;
// a packet whose Instance ID does not match the interface is dropped (the FSM spec does
// the matching). This leaf type bounds the value by type and serializes it.

package types

// InstanceID is the 8-bit OSPFv3 link-local instance selector.
type InstanceID uint8

// WriteTo writes the single octet into buf at off and returns 1.
func (id InstanceID) WriteTo(buf []byte, off int) int {
	buf[off] = byte(id)
	return 1
}
