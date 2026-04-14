package l2tp

import (
	"testing"
	"time"
)

// VALIDATES: AC-1 sequence comparison in modulo-65536 space works across
// wraparound. PREVENTS: the integer-overflow trap (RFC 2661 Section 5.8
// line 2538-2547 and Section 26.2) that would compare sequence numbers as
// raw uint16 values and lose ordering near 0/65535.
func TestSeqBefore(t *testing.T) {
	tests := []struct {
		name string
		a, b uint16
		want bool
	}{
		{"simple less", 1, 2, true},
		{"simple greater", 2, 1, false},
		{"equal", 100, 100, false},
		{"wrap: 65535 before 0", 65535, 0, true},
		{"wrap: 0 not before 65535", 0, 65535, false},
		{"wrap: 65530 before 5", 65530, 5, true},
		{"wrap: 5 not before 65530", 5, 65530, false},
		{"half-space: 0 before 32767 (diff=32767, inclusive)", 0, 32767, true},
		{"half-space: 32767 not before 0 (would need diff<=32767 via b-a=-32767)", 32767, 0, false},
		{"half-space boundary: 32768 not before 0 (diff=32768, exceeds 32767)", 32768, 0, false},
		{"half-space boundary: 0 not before 32768 (diff=32768, exceeds 32767)", 0, 32768, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := seqBefore(tc.a, tc.b)
			if got != tc.want {
				t.Errorf("seqBefore(%d, %d) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

// VALIDATES: AC-25 retention-duration computation flows from configured
// schedule. Hardcoding 31s would break when the schedule is changed (RFC
// 2661 Section 5.8 line 2602-2605 says "full retransmission interval",
// not a fixed duration).
func TestRetentionDuration(t *testing.T) {
	tests := []struct {
		name          string
		rtimeout      time.Duration
		rtimeoutCap   time.Duration
		maxRetransmit int
		want          time.Duration
	}{
		{"defaults: 1s/16s/5 -> 31s", time.Second, 16 * time.Second, 5, 31 * time.Second},
		{"short schedule: 1s/16s/3 -> 7s (1+2+4)", time.Second, 16 * time.Second, 3, 7 * time.Second},
		{"capped schedule: 1s/4s/5 -> 15s (1+2+4+4+4)", time.Second, 4 * time.Second, 5, 15 * time.Second},
		{"no cap hit: 2s/100s/4 -> 30s (2+4+8+16)", 2 * time.Second, 100 * time.Second, 4, 30 * time.Second},
		{"single retry: 1s/16s/1 -> 1s", time.Second, 16 * time.Second, 1, time.Second},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := retentionDuration(tc.rtimeout, tc.rtimeoutCap, tc.maxRetransmit)
			if got != tc.want {
				t.Errorf("retentionDuration(%v, %v, %d) = %v, want %v",
					tc.rtimeout, tc.rtimeoutCap, tc.maxRetransmit, got, tc.want)
			}
		})
	}
}

// VALIDATES: constants match RFC 2661 recommendations and accel-ppp
// defaults. PREVENTS: drift from the protocol spec over time.
func TestConstants(t *testing.T) {
	if DefaultRTimeout != time.Second {
		t.Errorf("DefaultRTimeout = %v, want 1s (RFC 2661 S5.8 line 2587)", DefaultRTimeout)
	}
	if DefaultRTimeoutCap != 16*time.Second {
		t.Errorf("DefaultRTimeoutCap = %v, want 16s (accel-ppp DEFAULT_RTIMEOUT_CAP)", DefaultRTimeoutCap)
	}
	if DefaultMaxRetransmit != 5 {
		t.Errorf("DefaultMaxRetransmit = %d, want 5 (RFC 2661 S5.8 line 2599)", DefaultMaxRetransmit)
	}
	if DefaultPeerRcvWindow != 4 {
		t.Errorf("DefaultPeerRcvWindow = %d, want 4 (RFC 2661 S5.8 line 2614-2615)", DefaultPeerRcvWindow)
	}
	if DefaultRecvWindow != 16 {
		t.Errorf("DefaultRecvWindow = %d, want 16 (accel-ppp DEFAULT_RECV_WINDOW)", DefaultRecvWindow)
	}
	if RecvWindowMax != 32768 {
		t.Errorf("RecvWindowMax = %d, want 32768 (half of 16-bit Ns space)", RecvWindowMax)
	}
}
