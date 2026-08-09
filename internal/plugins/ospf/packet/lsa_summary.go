// Design: docs/architecture/ospf/ospf-2-wire.md -- Summary-LSA body codec
// RFC 2328 Appendix A.4.4: Summary-LSA types 3 and 4 use a 24-bit metric.

package packet

const (
	SummaryMetricMax = 0x00ffffff
	summaryLSALen    = 8
)

// SummaryLSA is the Type 3/4 Summary-LSA body.
type SummaryLSA struct {
	NetworkMask [4]byte
	TOS         uint8
	Metric      uint32 // 24-bit metric, 0..0x00ffffff
}

// DecodeSummaryLSA parses a Summary-LSA body. Only the base TOS 0 block is
// surfaced; obsolete additional TOS blocks are rejected for v1 simplicity.
func DecodeSummaryLSA(body []byte) (SummaryLSA, error) {
	if len(body) < summaryLSALen {
		return SummaryLSA{}, ErrTruncated
	}
	if len(body) != summaryLSALen {
		return SummaryLSA{}, ErrLength
	}
	return SummaryLSA{NetworkMask: readIPv4(body, 0), TOS: body[4], Metric: readUint24(body, 5)}, nil
}

// EncodedLen returns the Summary-LSA body length.
func (l SummaryLSA) EncodedLen() int { return summaryLSALen }

// WriteTo serializes the Summary-LSA body into buf at off.
func (l SummaryLSA) WriteTo(buf []byte, off int) int {
	off += writeIPv4(buf, off, l.NetworkMask)
	buf[off] = l.TOS
	off++
	off += writeUint24(buf, off, l.Metric&SummaryMetricMax)
	return off
}
