package fsm

import (
	"testing"
	"time"
)

// TestSkewTime and TestMasterDownInterval assert the exact literal nanosecond
// values from the spec Boundary Tests table (multiplication before division,
// division last, computed on time.Duration).
//
// VALIDATES: AC-14 (timer math boundaries equal the literal ns values).
// PREVENTS: uvrrpd v2 skew truncated to whole seconds and holo v3 skew int-cast
// to zero (the truncate-to-zero bug class this child exists to kill).

type timerBoundary struct {
	version    uint8
	intervalMs int
	priority   uint8
	skewNs     int64
	masterNs   int64
}

// boundaryTable mirrors the spec's "Exact expected timer values" table verbatim.
var boundaryTable = []timerBoundary{
	{3, 10, 1, 9_960_937, 39_960_937},
	{3, 10, 100, 6_093_750, 36_093_750},
	{3, 10, 254, 78_125, 30_078_125},
	{3, 10, 255, 39_062, 30_039_062},
	{3, 1000, 1, 996_093_750, 3_996_093_750},
	{3, 1000, 100, 609_375_000, 3_609_375_000},
	{3, 1000, 254, 7_812_500, 3_007_812_500},
	{3, 1000, 255, 3_906_250, 3_003_906_250},
	{3, 40950, 1, 40_790_039_062, 163_640_039_062},
	{3, 40950, 100, 24_953_906_250, 147_803_906_250},
	{3, 40950, 254, 319_921_875, 123_169_921_875},
	{3, 40950, 255, 159_960_937, 123_009_960_937},
	{2, 1000, 1, 996_093_750, 3_996_093_750},
	{2, 1000, 100, 609_375_000, 3_609_375_000},
	{2, 1000, 254, 7_812_500, 3_007_812_500},
	{2, 1000, 255, 3_906_250, 3_003_906_250},
	{2, 255000, 1, 996_093_750, 765_996_093_750},
	{2, 255000, 254, 7_812_500, 765_007_812_500},
}

func TestSkewTime(t *testing.T) {
	for _, tc := range boundaryTable {
		name := skewCaseName(tc)
		t.Run(name, func(t *testing.T) {
			got := skewTime(tc.version, tc.priority, tc.intervalMs)
			if int64(got) != tc.skewNs {
				t.Fatalf("skewTime(v%d, prio %d, %dms) = %d ns, want %d ns",
					tc.version, tc.priority, tc.intervalMs, int64(got), tc.skewNs)
			}
		})
	}
}

func TestMasterDownInterval(t *testing.T) {
	for _, tc := range boundaryTable {
		name := skewCaseName(tc)
		t.Run(name, func(t *testing.T) {
			got := masterDownInterval(tc.version, tc.priority, tc.intervalMs)
			if int64(got) != tc.masterNs {
				t.Fatalf("masterDownInterval(v%d, prio %d, %dms) = %d ns, want %d ns",
					tc.version, tc.priority, tc.intervalMs, int64(got), tc.masterNs)
			}
		})
	}
}

func skewCaseName(tc timerBoundary) string {
	// Avoid fmt here to keep the file honest about allocation discipline is
	// unnecessary in tests; use a readable name.
	return "v" + itoa(int(tc.version)) + "_int" + itoa(tc.intervalMs) + "_prio" + itoa(int(tc.priority))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// TestSkewNeverZero asserts skew > 0 for every (priority) x (valid interval)
// corner in both versions. The v3 prio-254/10ms case (78,125 ns) is exactly the
// value an integer-millisecond representation truncates to zero.
//
// VALIDATES: AC-14 "skew is never zero for any valid input".
// PREVENTS: uvrrpd/holo skew truncation bug.
func TestSkewNeverZero(t *testing.T) {
	cases := []struct {
		version   uint8
		intervals []int
	}{
		{3, []int{10, 100, 1000, 40950}},
		{2, []int{1000, 60000, 255000}},
	}
	for _, c := range cases {
		for _, iv := range c.intervals {
			for p := 1; p <= 255; p++ {
				skew := skewTime(c.version, uint8(p), iv)
				if skew <= 0 {
					t.Fatalf("skewTime(v%d, prio %d, %dms) = %v; must be > 0",
						c.version, p, iv, skew)
				}
				md := masterDownInterval(c.version, uint8(p), iv)
				if md <= skew {
					t.Fatalf("masterDownInterval(v%d, prio %d, %dms) = %v; must exceed skew %v",
						c.version, p, iv, md, skew)
				}
			}
		}
	}
}

// TestSkewV2IntervalIndependent proves the v2 skew ignores the advertisement
// interval (RFC 3768 Algorithms), unlike v3.
//
// VALIDATES: Version Behavior table "Skew_Time (256-priority)/256 seconds,
// interval-independent" for v2.
func TestSkewV2IntervalIndependent(t *testing.T) {
	for p := 1; p <= 255; p++ {
		a := skewTime(2, uint8(p), 1000)
		b := skewTime(2, uint8(p), 255000)
		if a != b {
			t.Fatalf("v2 skew prio %d differs by interval: %v vs %v", p, a, b)
		}
	}
	// And v3 does scale with the interval.
	if skewTime(3, 100, 10) == skewTime(3, 100, 1000) {
		t.Fatal("v3 skew must scale with the advertisement interval")
	}
}

var _ = time.Nanosecond
