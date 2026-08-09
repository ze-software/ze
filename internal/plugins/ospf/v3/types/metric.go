// Design: docs/architecture/ospf/ospfv3-1-types.md -- Metric 24-bit OSPFv3 route metric.
// RFC: rfc/short/rfc5340.md (§A.4.4 Inter-Area-Prefix / §A.4.7 AS-External metric)
//
// OSPFv3 carries a 24-bit metric in the prefix and external LSAs (interface cost users may
// narrow to 16 bits). A valid route metric is 1..0xFFFFFF; 0 is not a usable cost.

package types

// Metric is a 24-bit OSPFv3 route metric.
type Metric uint32

// metricMax is the largest valid 24-bit metric.
const metricMax = 0xffffff

// NewMetric validates v as a 24-bit metric in 1..0xFFFFFF.
func NewMetric(v uint32) (Metric, error) {
	if v == 0 || v > metricMax {
		return 0, ErrOutOfRange
	}
	return Metric(v), nil
}

// WriteTo writes the 3 big-endian octets into buf at off and returns 3.
func (m Metric) WriteTo(buf []byte, off int) int {
	buf[off] = byte(m >> 16)
	buf[off+1] = byte(m >> 8)
	buf[off+2] = byte(m)
	return 3
}
