// Design: docs/guide/ipsec.md -- complete-path rate and burst bounds

package dataplane

import (
	"testing"
	"time"
)

// TestESPFormLimiterRefillRate counts admitted datagrams at fixed synthetic times
// after spending the initial burst, proving the documented refill rate.
func TestESPFormLimiterRefillRate(t *testing.T) {
	for _, tc := range []struct {
		name    string
		elapsed time.Duration
		want    int
	}{
		{name: "one second", elapsed: time.Second, want: 5000},
		{name: "one and a half seconds", elapsed: 1500 * time.Millisecond, want: 7500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Unix(1700000000, 0)
			lim := newESPFormLimiter(start)
			for range 10000 {
				if !lim.allow(start) {
					t.Fatal("refused a datagram within the initial burst")
				}
			}

			admitted := 0
			for range 10001 {
				if lim.allow(start.Add(tc.elapsed)) {
					admitted++
				}
			}
			if admitted != tc.want {
				t.Errorf("admitted %d datagrams after %s, want %d", admitted, tc.elapsed, tc.want)
			}
		})
	}
}

// TestESPFormLimiterIdleBurstCeiling counts admissions after ordinary and very
// long idle periods, proving that idle time cannot grow the documented burst.
func TestESPFormLimiterIdleBurstCeiling(t *testing.T) {
	for _, tc := range []struct {
		name string
		idle time.Duration
	}{
		{name: "two seconds", idle: 2 * time.Second},
		{name: "maximum duration", idle: time.Duration(1<<63 - 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Unix(1700000000, 0)
			lim := newESPFormLimiter(start)
			for range 10000 {
				if !lim.allow(start) {
					t.Fatal("refused a datagram within the initial burst")
				}
			}

			admitted := 0
			for range 10001 {
				if lim.allow(start.Add(tc.idle)) {
					admitted++
				}
			}
			if admitted != 10000 {
				t.Errorf("admitted %d datagrams after %s idle, want burst of 10000", admitted, tc.idle)
			}
		})
	}
}
