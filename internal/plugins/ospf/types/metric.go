// Design: docs/architecture/ospf/ospf-1-types.md -- OSPF interface output metric
// Related: lsakey.go -- metrics are LSA version data, not identity

package types

import "strconv"

// MetricLen is the interface metric wire width in Router-LSA link records.
const MetricLen = 2

const (
	// MetricMin is the lowest valid OSPF interface output cost.
	MetricMin uint32 = 1
	// MetricMax is the highest valid 16-bit OSPF interface output cost.
	MetricMax uint32 = 65535
	// DefaultReferenceBandwidth is the common 100 Mbps reference bandwidth in bits per second.
	DefaultReferenceBandwidth uint64 = 100000000
)

// Metric is the 16-bit OSPF interface output cost.
type Metric uint16

// NewMetric validates and constructs an interface metric.
func NewMetric(cost uint32) (Metric, error) {
	if cost < MetricMin || cost > MetricMax {
		return 0, ErrOutOfRange
	}
	return Metric(cost), nil
}

// MetricFromBytes decodes a two-octet big-endian Router-LSA link metric. Unlike
// NewMetric (which validates a configured interface output cost as 1..65535), the wire
// metric spans the full 16-bit range: a stub/host-route link legitimately carries cost
// 0 (RFC 2328 sec 12.4.1.4 point-to-multipoint host route), and FRR emits such links, so
// the decoder must accept 0 rather than reject the whole LSA.
func MetricFromBytes(b []byte) (Metric, error) {
	if len(b) != MetricLen {
		return 0, ErrWrongLength
	}
	return Metric(uint32(b[0])<<8 | uint32(b[1])), nil
}

// DefaultMetric derives the default cost as referenceBandwidth/interfaceBandwidth, floored at 1.
func DefaultMetric(referenceBandwidth, interfaceBandwidth uint64) (Metric, error) {
	if referenceBandwidth == 0 || interfaceBandwidth == 0 {
		return 0, ErrOutOfRange
	}
	cost := max(referenceBandwidth/interfaceBandwidth, uint64(MetricMin))
	if cost > uint64(MetricMax) {
		return 0, ErrOutOfRange
	}
	return Metric(cost), nil
}

// WriteTo writes the two big-endian metric octets into buf at off.
func (m Metric) WriteTo(buf []byte, off int) int {
	return writeUint16(buf, off, uint16(m))
}

// String returns the decimal metric.
func (m Metric) String() string {
	return strconv.FormatUint(uint64(m), 10)
}
