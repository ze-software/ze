// Design: docs/architecture/ospf/ospf-2-wire.md -- AS-External and NSSA LSA body codec
// RFC 2328 Appendix A.4.5: AS-External-LSA. RFC 3101: Type 7 NSSA uses the same body.

package packet

const (
	ExternalMetricMax = 0x00ffffff
	externalLSALen    = 16
)

// ExternalLSA is the Type 5 AS-External or Type 7 NSSA body.
type ExternalLSA struct {
	NetworkMask      [4]byte
	ExternalType2    bool // E bit: true means Type 2 external metric
	Metric           uint32
	ForwardingAddr   [4]byte
	ExternalRouteTag uint32
}

// DecodeExternalLSA parses an AS-External/NSSA body.
func DecodeExternalLSA(body []byte) (ExternalLSA, error) {
	if len(body) < externalLSALen {
		return ExternalLSA{}, ErrTruncated
	}
	if len(body) != externalLSALen {
		return ExternalLSA{}, ErrLength
	}
	return ExternalLSA{
		NetworkMask:      readIPv4(body, 0),
		ExternalType2:    body[4]&0x80 != 0,
		Metric:           readUint24(body, 5),
		ForwardingAddr:   readIPv4(body, 8),
		ExternalRouteTag: readUint32(body, 12),
	}, nil
}

// EncodedLen returns the External-LSA body length.
func (l ExternalLSA) EncodedLen() int { return externalLSALen }

// WriteTo serializes the External/NSSA-LSA body into buf at off.
func (l ExternalLSA) WriteTo(buf []byte, off int) int {
	off += writeIPv4(buf, off, l.NetworkMask)
	metric := l.Metric & ExternalMetricMax
	buf[off] = 0
	if l.ExternalType2 {
		buf[off] = 0x80
	}
	off++
	off += writeUint24(buf, off, metric)
	off += writeIPv4(buf, off, l.ForwardingAddr)
	off += writeUint32(buf, off, l.ExternalRouteTag)
	return off
}
