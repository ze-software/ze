// Design: docs/architecture/isis/isis-1-types.md -- Metric (24-bit TLV 22) and PrefixMetric (32-bit TLV 135/236)
// RFC: rfc/short/rfc5305.md -- wide IS-reachability metric (TLV 22, 24-bit) and IP prefix metric (TLV 135, 32-bit)
// RFC: rfc/short/rfc5308.md -- IPv6 reachability prefix metric (TLV 236, 32-bit)

package types

// Metric / PrefixMetric widths and bounds.
const (
	// MetricLen is the IS-reachability metric width on the wire (3 octets).
	MetricLen = 3
	// MaxMetric is the maximum IS-reachability metric value.
	//
	// RFC 5305 section 3 (Extended IS Reachability, TLV 22): the default metric
	// is a 24-bit unsigned value, range 0..16777215. The default link metric is
	// 10. Only wide metrics are originated by Ze (umbrella decision); the narrow
	// 6-bit metric (TLV 2) is decode-only and not modeled as a type here.
	MaxMetric = 1<<24 - 1 // 16777215

	// PrefixMetricLen is the IP/IPv6 prefix metric width on the wire (4 octets).
	PrefixMetricLen = 4
	// MaxPrefixMetric is the maximum IP/IPv6 prefix metric value.
	//
	// RFC 5305 section 4 (Extended IP Reachability, TLV 135) and RFC 5308
	// (IPv6 Reachability, TLV 236): the prefix metric is a 32-bit unsigned
	// value, range 0..4294967295. It is a SEPARATE width from the 24-bit IS
	// reachability metric; capping it at 24-bit would reject or mangle valid
	// peer routes, so the two must not be conflated.
	MaxPrefixMetric = 1<<32 - 1 // 4294967295
)

// Metric is the IS-reachability link cost carried in TLV 22 (RFC 5305). It is a
// 24-bit unsigned value (0..16777215). NewMetric range-checks the input;
// MetricFromBytes decodes exactly 3 big-endian octets.
type Metric struct {
	v uint32 // always <= MaxMetric
}

// NewMetric constructs a Metric, rejecting values above the 24-bit maximum
// (RFC 5305: the IS-reachability metric is 24-bit).
func NewMetric(v uint32) (Metric, error) {
	if v > MaxMetric {
		return Metric{}, ErrWrongLength
	}
	return Metric{v: v}, nil
}

// MetricFromBytes decodes a 3-octet big-endian IS-reachability metric. A length
// other than 3 returns ErrWrongLength. The 24-bit value cannot exceed MaxMetric
// by construction (3 octets), so no range error is possible here.
func MetricFromBytes(b []byte) (Metric, error) {
	if len(b) != MetricLen {
		return Metric{}, ErrWrongLength
	}
	v := uint32(b[0])<<16 | uint32(b[1])<<8 | uint32(b[2])
	return Metric{v: v}, nil
}

// Value returns the metric as a uint32 (always <= MaxMetric).
func (m Metric) Value() uint32 { return m.v }

// WriteTo writes the 3 big-endian octets into buf at off; returns MetricLen.
// Buffer-first, no allocation.
func (m Metric) WriteTo(buf []byte, off int) int {
	buf[off] = byte(m.v >> 16)
	buf[off+1] = byte(m.v >> 8)
	buf[off+2] = byte(m.v)
	return MetricLen
}

// PrefixMetric is the IPv4/IPv6 prefix cost carried in TLV 135 (RFC 5305) and
// TLV 236 (RFC 5308). It is a 32-bit unsigned value (0..4294967295); the full
// uint32 range is valid, so there is no NewPrefixMetric error return.
type PrefixMetric struct {
	v uint32
}

// NewPrefixMetric constructs a PrefixMetric. Every uint32 is in range (the
// prefix metric is a full 32-bit value), so this cannot fail.
func NewPrefixMetric(v uint32) PrefixMetric { return PrefixMetric{v: v} }

// PrefixMetricFromBytes decodes a 4-octet big-endian prefix metric. A length
// other than 4 returns ErrWrongLength.
func PrefixMetricFromBytes(b []byte) (PrefixMetric, error) {
	if len(b) != PrefixMetricLen {
		return PrefixMetric{}, ErrWrongLength
	}
	v := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
	return PrefixMetric{v: v}, nil
}

// Value returns the prefix metric as a uint32.
func (m PrefixMetric) Value() uint32 { return m.v }

// WriteTo writes the 4 big-endian octets into buf at off; returns
// PrefixMetricLen. Buffer-first, no allocation.
func (m PrefixMetric) WriteTo(buf []byte, off int) int {
	buf[off] = byte(m.v >> 24)
	buf[off+1] = byte(m.v >> 16)
	buf[off+2] = byte(m.v >> 8)
	buf[off+3] = byte(m.v)
	return PrefixMetricLen
}
