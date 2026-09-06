// Design: docs/architecture/ospf/ospf-1-types.md -- Metric range and default-cost derivation

package types

import "testing"

// VALIDATES: AC-8 - Metric accepts 1..65535, rejects 0, and serializes as two bytes.
// PREVENTS: invalid zero link costs and endian drift in Router-LSA link records.
func TestMetricRangeAndCost(t *testing.T) {
	for _, cost := range []uint32{MetricMin, MetricMax} {
		metric, err := NewMetric(cost)
		if err != nil {
			t.Fatalf("NewMetric(%d) returned error: %v", cost, err)
		}
		if uint32(metric) != cost {
			t.Fatalf("NewMetric(%d) = %d", cost, metric)
		}
	}
	if _, err := NewMetric(0); err == nil {
		t.Fatalf("NewMetric(0) succeeded, want error")
	}
	metric, err := MetricFromBytes([]byte{0xff, 0xff})
	if err != nil {
		t.Fatalf("MetricFromBytes returned error: %v", err)
	}
	if uint32(metric) != MetricMax {
		t.Fatalf("MetricFromBytes = %d, want %d", metric, MetricMax)
	}
	var buf [2]byte
	if n := metric.WriteTo(buf[:], 0); n != MetricLen || buf != [2]byte{0xff, 0xff} {
		t.Fatalf("Metric.WriteTo n=%d bytes=%v", n, buf)
	}
	if _, err := MetricFromBytes([]byte{0x01}); err == nil {
		t.Fatalf("short metric parse succeeded")
	}
}

// VALIDATES: AC-8 - Metric.String renders the decimal cost across the valid range.
// PREVENTS: metric display drifting from the numeric wire value.
func TestMetricString(t *testing.T) {
	cases := []struct {
		m    Metric
		want string
	}{
		{Metric(MetricMin), "1"},
		{Metric(10), "10"},
		{Metric(MetricMax), "65535"},
	}
	for _, tc := range cases {
		if got := tc.m.String(); got != tc.want {
			t.Errorf("Metric(%d).String() = %q, want %q", uint16(tc.m), got, tc.want)
		}
	}
}

// VALIDATES: AC-8 - default cost uses reference bandwidth divided by link speed, clamped
// into the Router-LSA metric range.
// PREVENTS: a high-speed interface deriving an invalid zero metric, and a slow interface
// under a large reference bandwidth deriving a cost the two-octet wire field cannot carry.
func TestDefaultMetric(t *testing.T) {
	// Both operands are Mbit/s. The reference bandwidth is the ze-ospf-conf.yang default.
	const referenceMbps = 100000
	cases := []struct {
		name          string
		linkSpeedMbps uint64
		want          Metric
	}{
		{name: "100G reaches the floor", linkSpeedMbps: 100000, want: Metric(MetricMin)},
		{name: "400G stays at the floor", linkSpeedMbps: 400000, want: Metric(MetricMin)},
		{name: "10G", linkSpeedMbps: 10000, want: 10},
		{name: "1G", linkSpeedMbps: 1000, want: 100},
		{name: "100M", linkSpeedMbps: 100, want: 1000},
		{name: "10M", linkSpeedMbps: 10, want: 10000},
		{name: "1M reaches the ceiling", linkSpeedMbps: 1, want: Metric(MetricMax)},
	}
	for _, tc := range cases {
		got, err := DefaultMetric(referenceMbps, tc.linkSpeedMbps)
		if err != nil {
			t.Fatalf("%s: DefaultMetric(%d, %d) returned error: %v", tc.name, referenceMbps, tc.linkSpeedMbps, err)
		}
		if got != tc.want {
			t.Errorf("%s: DefaultMetric(%d, %d) = %d, want %d", tc.name, referenceMbps, tc.linkSpeedMbps, got, tc.want)
		}
	}
	if _, err := DefaultMetric(referenceMbps, 0); err == nil {
		t.Fatalf("DefaultMetric with an unknown link speed succeeded, want an error")
	}
	if _, err := DefaultMetric(0, 1000); err == nil {
		t.Fatalf("DefaultMetric with a zero reference bandwidth succeeded, want an error")
	}
}
