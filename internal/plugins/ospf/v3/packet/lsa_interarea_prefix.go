// Design: docs/architecture/ospf/ospfv3-2-wire.md -- OSPFv3 Inter-Area-Prefix-LSA body codec.
// RFC: rfc/short/rfc5340.md (§A.4.5 Inter-Area-Prefix-LSA)

package packet

// Inter-Area-Prefix-LSA body field offsets (RFC 5340 §A.4.5, body-relative).
const (
	interAreaPrefixMetricOff  = 1 // 24-bit metric
	interAreaPrefixLenOff     = 4 // PrefixLength
	interAreaPrefixOptionsOff = 5 // PrefixOptions
	interAreaPrefixField16Off = 6 // reserved 16-bit field
	interAreaPrefixAddrOff    = 8 // AddressPrefix
)

// InterAreaPrefixLSA is the OSPFv3 Inter-Area-Prefix-LSA body (RFC 5340 §A.4.5):
// an area-scoped summary of an IPv6 prefix reachable in another area, with a
// 24-bit metric and an inlined prefix.
type InterAreaPrefixLSA struct {
	Metric uint32
	Prefix Prefix
}

// decodeInterAreaPrefixLSA parses an Inter-Area-Prefix-LSA body.
func decodeInterAreaPrefixLSA(body []byte) (InterAreaPrefixLSA, error) {
	if len(body) < interAreaPrefixAddrOff {
		return InterAreaPrefixLSA{}, ErrTruncated
	}
	pfx, addrLen, err := decodeInlinePrefix(body,
		interAreaPrefixLenOff, interAreaPrefixOptionsOff, interAreaPrefixField16Off, interAreaPrefixAddrOff)
	if err != nil {
		return InterAreaPrefixLSA{}, err
	}
	if interAreaPrefixAddrOff+addrLen != len(body) {
		return InterAreaPrefixLSA{}, ErrLength
	}
	return InterAreaPrefixLSA{Metric: readUint24(body, interAreaPrefixMetricOff), Prefix: pfx}, nil
}

// EncodedLen returns the Inter-Area-Prefix-LSA body length.
func (l InterAreaPrefixLSA) EncodedLen() int {
	return interAreaPrefixAddrOff + l.Prefix.Length.ByteLen()
}

// WriteTo serializes the Inter-Area-Prefix-LSA body into buf at off. The leading
// reserved octet and the 16-bit reserved field are written zero (RFC 5340
// §A.4.5).
func (l InterAreaPrefixLSA) WriteTo(buf []byte, off int) int {
	start := off
	buf[off] = 0
	writeUint24(buf, off+interAreaPrefixMetricOff, l.Metric)
	buf[start+interAreaPrefixLenOff] = byte(l.Prefix.Length)
	buf[start+interAreaPrefixOptionsOff] = byte(l.Prefix.Options)
	writeUint16(buf, start+interAreaPrefixField16Off, 0)
	end := writePrefixAddress(buf, start+interAreaPrefixAddrOff, l.Prefix.Length, l.Prefix.Address)
	return end
}

// writeUint24 writes the 3 big-endian octets of a 24-bit value into buf at off.
func writeUint24(buf []byte, off int, v uint32) {
	buf[off] = byte(v >> 16)
	buf[off+1] = byte(v >> 8)
	buf[off+2] = byte(v)
}
