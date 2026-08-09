// Design: docs/architecture/ospf/ospf-2-wire.md -- shared fixed-width wire helpers

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

func writeUint24(buf []byte, off int, v uint32) int {
	buf[off] = byte(v >> 16)
	buf[off+1] = byte(v >> 8)
	buf[off+2] = byte(v)
	return 3
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

func readUint64(buf []byte, off int) uint64 {
	return uint64(readUint32(buf, off))<<32 | uint64(readUint32(buf, off+4))
}

func writeUint64(buf []byte, off int, v uint64) {
	writeUint32(buf, off, uint32(v>>32))
	writeUint32(buf, off+4, uint32(v))
}

func readIPv4(buf []byte, off int) [4]byte {
	return [4]byte{buf[off], buf[off+1], buf[off+2], buf[off+3]}
}

func writeIPv4(buf []byte, off int, v [4]byte) int {
	copy(buf[off:off+4], v[:])
	return 4
}

func appendIPv4(dst []byte, v [4]byte) []byte {
	dst = appendDecimal(dst, v[0])
	dst = append(dst, '.')
	dst = appendDecimal(dst, v[1])
	dst = append(dst, '.')
	dst = appendDecimal(dst, v[2])
	dst = append(dst, '.')
	return appendDecimal(dst, v[3])
}

func appendDecimal(dst []byte, v byte) []byte {
	if v >= 100 {
		dst = append(dst, '0'+v/100)
		v %= 100
		dst = append(dst, '0'+v/10)
		return append(dst, '0'+v%10)
	}
	if v >= 10 {
		dst = append(dst, '0'+v/10)
		return append(dst, '0'+v%10)
	}
	return append(dst, '0'+v)
}

func ipv4String(v [4]byte) string {
	var scratch [15]byte
	return string(appendIPv4(scratch[:0], v))
}
