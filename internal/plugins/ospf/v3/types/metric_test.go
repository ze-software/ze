// VALIDATES: spec-ospfv3-1-types -- Metric accepts 1..0xFFFFFF and rejects 0 and values
// above the 24-bit bound; WriteTo emits 3 big-endian octets.
// PREVENTS: a metric truncated to 16 bits or a zero/overflow accepted.
package types

import "testing"

func TestOSPFv3MetricBoundaries(t *testing.T) {
	if _, err := NewMetric(1); err != nil {
		t.Errorf("NewMetric(1): %v", err)
	}
	if _, err := NewMetric(0xffffff); err != nil {
		t.Errorf("NewMetric(max): %v", err)
	}
	if _, err := NewMetric(0); err == nil {
		t.Error("NewMetric(0) accepted")
	}
	if _, err := NewMetric(0x1000000); err == nil {
		t.Error("NewMetric(>24bit) accepted")
	}

	buf := make([]byte, 3)
	if n := Metric(0x010203).WriteTo(buf, 0); n != 3 || buf[0] != 0x01 || buf[2] != 0x03 {
		t.Errorf("WriteTo n=%d buf=%v", n, buf)
	}
}
