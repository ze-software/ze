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

// VALIDATES: AC-8 - default cost uses reference bandwidth divided by interface bandwidth, floored at 1.
// PREVENTS: high-speed interfaces deriving an invalid zero metric.
func TestDefaultMetric(t *testing.T) {
	fast, err := DefaultMetric(DefaultReferenceBandwidth, 1000000000)
	if err != nil {
		t.Fatalf("DefaultMetric fast returned error: %v", err)
	}
	if fast != Metric(1) {
		t.Fatalf("fast DefaultMetric = %d, want 1", fast)
	}
	ten, err := DefaultMetric(DefaultReferenceBandwidth, 10000000)
	if err != nil {
		t.Fatalf("DefaultMetric 10M returned error: %v", err)
	}
	if ten != Metric(10) {
		t.Fatalf("10M DefaultMetric = %d, want 10", ten)
	}
	if _, err := DefaultMetric(DefaultReferenceBandwidth, 0); err == nil {
		t.Fatalf("DefaultMetric with zero bandwidth succeeded")
	}
}
