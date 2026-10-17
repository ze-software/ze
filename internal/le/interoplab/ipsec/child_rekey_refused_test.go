// Design: checkers.go -- the refused Child SA rekey gap is read from charon's own timestamps.
package ipsec

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

// swanRefusal is one charon line answering a Child SA rekey N(NO_PROP), stamped as the
// scenario's strongswan.conf configures (time_format %H:%M:%S, time_add_ms).
func swanRefusal(stamp string, msgID int) string {
	return stamp + " 13[ENC] generating CREATE_CHILD_SA response " + strconv.Itoa(msgID) + " [ N(NO_PROP) ]"
}

// VALIDATES: childRekeyRefusalGap measures the gap between strongSwan's first two
// refusals from the timestamps on the lines, whatever surrounds them, and the verdict
// bounds then pass a 14.2 s resend and refuse two refusals stamped together.
// PREVENTS: the read-time measurement that failed the nightly with a 14.858 ms gap. A
// checker that times lines by when it reads them sees two lines from one buffered flush
// as a resend inside one poll, so it cannot tell an observation defect from a real one.
func TestChildRekeyRefusalGap(t *testing.T) {
	cases := []struct {
		name   string
		lines  []string
		want   time.Duration
		inside bool
	}{
		{"resent after the wait", []string{
			"12:00:00.100 00[DMN] Starting IKE charon daemon",
			swanRefusal("12:00:39.250", 2),
			"12:00:40.000 09[NET] sending packet",
			swanRefusal("12:00:53.450", 3),
			swanRefusal("12:01:08.000", 4),
		}, 14200 * time.Millisecond, true},
		{"resent at once", []string{
			swanRefusal("12:00:39.250", 2),
			swanRefusal("12:00:39.250", 3),
		}, 0, false},
		{"resent on the next tick", []string{
			swanRefusal("12:00:39.250", 2),
			swanRefusal("12:00:40.249", 3),
		}, 999 * time.Millisecond, false},
		{"across midnight", []string{
			swanRefusal("23:59:50.000", 2),
			swanRefusal("00:00:04.000", 3),
		}, 14 * time.Second, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gap, err := childRekeyRefusalGap(strings.Join(c.lines, "\n"))
			if err != nil {
				t.Fatalf("childRekeyRefusalGap: %v", err)
			}
			if gap != c.want {
				t.Fatalf("gap = %v, want %v", gap, c.want)
			}
			inside := gap >= childRekeyRefusedGapMin && gap <= childRekeyRefusedGapMax
			if inside != c.inside {
				t.Fatalf("gap %v inside the owner's window = %v, want %v", gap, inside, c.inside)
			}
		})
	}
}

// VALIDATES: a log with fewer than two timestamped refusals measures no gap, including
// one whose charon writes no timestamp. PREVENTS: a zero gap answered as a measurement.
func TestChildRekeyRefusalGapNeedsTwoStampedLines(t *testing.T) {
	logs := []string{
		swanRefusal("12:00:39.250", 2),
		"13[ENC] generating CREATE_CHILD_SA response 2 [ N(NO_PROP) ]\n" +
			"13[ENC] generating CREATE_CHILD_SA response 3 [ N(NO_PROP) ]",
	}
	for _, log := range logs {
		if gap, err := childRekeyRefusalGap(log); err == nil {
			t.Fatalf("a log with fewer than two stamped refusals measured a gap of %v", gap)
		}
	}
}
