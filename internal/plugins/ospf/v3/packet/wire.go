// Design: docs/architecture/ospf/ospfv3-2-wire.md -- shared fixed-width big-endian wire helpers.
// RFC: rfc/short/rfc5340.md (all OSPFv3 wire fields are big-endian)

package packet

func readUint16(buf []byte, off int) uint16 {
	return uint16(buf[off])<<8 | uint16(buf[off+1])
}

func writeUint16(buf []byte, off int, v uint16) int {
	buf[off] = byte(v >> 8)
	buf[off+1] = byte(v)
	return 2
}

func readUint24(buf []byte, off int) uint32 {
	return uint32(buf[off])<<16 | uint32(buf[off+1])<<8 | uint32(buf[off+2])
}

func readUint32(buf []byte, off int) uint32 {
	return uint32(buf[off])<<24 | uint32(buf[off+1])<<16 | uint32(buf[off+2])<<8 | uint32(buf[off+3])
}

func writeUint32(buf []byte, off int, v uint32) int {
	buf[off] = byte(v >> 24)
	buf[off+1] = byte(v >> 16)
	buf[off+2] = byte(v >> 8)
	buf[off+3] = byte(v)
	return 4
}
