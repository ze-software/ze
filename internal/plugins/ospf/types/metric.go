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

// DefaultMetric derives an interface output cost from the auto-cost convention: the
// reference bandwidth divided by the speed of the link, both in Mbit/s. The division
// truncates towards zero, so a link at or above the reference bandwidth reaches the
// floor. The quotient is clamped into the range a Router-LSA link metric can carry:
// MetricMin at the bottom, because RFC 2328 Appendix C.3 states the interface output
// cost "must always be greater than 0", and MetricMax at the top, because the wire
// field is two octets wide.
//
// A zero operand has no quotient and returns ErrOutOfRange. The kernel reports no speed
// for a virtual device or a down link, so that case is normal rather than exceptional,
// and the caller decides what an interface of unknown speed costs.
func DefaultMetric(referenceBandwidthMbps, linkSpeedMbps uint64) (Metric, error) {
	if referenceBandwidthMbps == 0 || linkSpeedMbps == 0 {
		return 0, ErrOutOfRange
	}
	cost := min(max(referenceBandwidthMbps/linkSpeedMbps, uint64(MetricMin)), uint64(MetricMax))
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
