package types

import (
	"bytes"
	"testing"
)

// VALIDATES: Metric accepts 0..16777215 (24-bit, TLV 22), rejects 16777216, and
// serializes to exactly 3 big-endian octets (AC-7, boundary table).
// PREVENTS: a 24-bit IS-reachability metric overflowing or using the wrong width.
func TestMetricRange(t *testing.T) {
	if _, err := NewMetric(0); err != nil {
		t.Errorf("Metric(0) should be valid: %v", err)
	}
	if _, err := NewMetric(MaxMetric); err != nil {
		t.Errorf("Metric(%d) should be valid: %v", MaxMetric, err)
	}
	if _, err := NewMetric(MaxMetric + 1); err == nil {
		t.Errorf("Metric(%d) should be rejected (above 24-bit)", MaxMetric+1)
	}

	m, _ := NewMetric(0x010203)
	var buf [8]byte
	n := m.WriteTo(buf[:], 0)
	if n != MetricLen {
		t.Fatalf("Metric.WriteTo returned %d, want %d", n, MetricLen)
	}
	want := []byte{0x01, 0x02, 0x03}
	if !bytes.Equal(buf[:n], want) {
		t.Errorf("Metric serialize = %x, want %x", buf[:n], want)
	}

	back, err := MetricFromBytes(buf[:n])
	if err != nil {
		t.Fatalf("MetricFromBytes error: %v", err)
	}
	if back.Value() != 0x010203 {
		t.Errorf("round-trip = %d, want %d", back.Value(), 0x010203)
	}
}

// VALIDATES: MetricFromBytes requires exactly 3 octets.
// PREVENTS: out-of-range slice on a short/long wire field.
func TestMetricFromBytesLength(t *testing.T) {
	for _, l := range []int{0, 2, 4} {
		if _, err := MetricFromBytes(make([]byte, l)); err == nil {
			t.Errorf("MetricFromBytes(len=%d) should error", l)
		}
	}
}

// VALIDATES: PrefixMetric accepts the full 0..4294967295 (32-bit, TLV 135/236)
// range and serializes to exactly 4 big-endian octets (AC-7, boundary table).
// PREVENTS: capping a 32-bit prefix metric at 24-bit (would mangle peer routes).
func TestPrefixMetricRange(t *testing.T) {
	for _, v := range []uint32{0, 10, MaxMetric, MaxMetric + 1, MaxPrefixMetric} {
		pm := NewPrefixMetric(v)
		if pm.Value() != v {
			t.Errorf("PrefixMetric(%d).Value() = %d", v, pm.Value())
		}
	}

	pm := NewPrefixMetric(0x01020304)
	var buf [8]byte
	n := pm.WriteTo(buf[:], 0)
	if n != PrefixMetricLen {
		t.Fatalf("PrefixMetric.WriteTo returned %d, want %d", n, PrefixMetricLen)
	}
	want := []byte{0x01, 0x02, 0x03, 0x04}
	if !bytes.Equal(buf[:n], want) {
		t.Errorf("PrefixMetric serialize = %x, want %x", buf[:n], want)
	}

	back, err := PrefixMetricFromBytes(buf[:n])
	if err != nil {
		t.Fatalf("PrefixMetricFromBytes error: %v", err)
	}
	if back.Value() != 0x01020304 {
		t.Errorf("round-trip = %d, want %d", back.Value(), 0x01020304)
	}
}

// VALIDATES: PrefixMetricFromBytes requires exactly 4 octets.
// PREVENTS: out-of-range slice on a short/long wire field.
func TestPrefixMetricFromBytesLength(t *testing.T) {
	for _, l := range []int{0, 3, 5} {
		if _, err := PrefixMetricFromBytes(make([]byte, l)); err == nil {
			t.Errorf("PrefixMetricFromBytes(len=%d) should error", l)
		}
	}
}

// VALIDATES: the two metric widths are distinct types with distinct serialize
// widths (3 vs 4 octets), per RFC 5305 TLV 22 vs TLV 135/236 (Key Design Decision).
// PREVENTS: conflating the IS-reachability and prefix metric widths.
func TestMetricWidthsDistinct(t *testing.T) {
	if MetricLen == PrefixMetricLen {
		t.Fatal("Metric (3 octets) and PrefixMetric (4 octets) must differ")
	}
	if MetricLen != 3 || PrefixMetricLen != 4 {
		t.Errorf("widths: Metric=%d PrefixMetric=%d, want 3 and 4", MetricLen, PrefixMetricLen)
	}
}
