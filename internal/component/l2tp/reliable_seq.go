// Design: docs/architecture/wire/l2tp.md — L2TP reliable delivery engine
// RFC: rfc/short/rfc2661.md — RFC 2661 Section 5.8, Section 26.2
// Related: reliable.go — engine core consuming seqBefore and retentionDuration

package l2tp

import "time"

// Default engine parameters. These are protocol recommendations from
// RFC 2661 Section 5.8 and empirical defaults inherited from accel-ppp.
const (
	// DefaultRTimeout is the initial retransmission timeout. RFC 2661
	// Section 5.8: "a recommended default is 1 second".
	DefaultRTimeout = time.Second

	// DefaultRTimeoutCap caps the exponential backoff. RFC 2661 Section
	// 5.8: "This cap MUST be no less than 8 seconds". 16s matches the
	// accel-ppp default (DEFAULT_RTIMEOUT_CAP).
	DefaultRTimeoutCap = 16 * time.Second

	// DefaultMaxRetransmit is the maximum retransmission count before
	// declaring the tunnel dead. RFC 2661 Section 5.8: "a recommended
	// default is 5, but SHOULD be configurable".
	DefaultMaxRetransmit = 5

	// DefaultPeerRcvWindow is the initial assumption about the peer's
	// receive window before the peer's Receive Window Size AVP arrives.
	// RFC 2661 Section 5.8 line 2614-2615: "MUST accept a window of up to
	// 4 from its peer". Accel-ppp uses the same default
	// (DEFAULT_PEER_RECV_WINDOW_SIZE).
	DefaultPeerRcvWindow uint16 = 4

	// DefaultRecvWindow is the value we advertise to peers via the
	// Receive Window Size AVP. It also sizes our reorder ring buffer;
	// messages whose Ns exceeds nextRecvSeq by more than this value are
	// dropped as out-of-range. Accel-ppp default (DEFAULT_RECV_WINDOW).
	DefaultRecvWindow uint16 = 16

	// RecvWindowMax is the largest legal Receive Window Size AVP value.
	// The Ns comparison window is half the 16-bit sequence space; any
	// window larger than 32768 would admit messages the classifier cannot
	// distinguish from duplicates.
	RecvWindowMax uint16 = 32768
)

// seqBefore reports whether Ns a comes before Ns b in the 16-bit sequence
// space. RFC 2661 Section 5.8: "if its value lies in the range of the
// last received number and the preceding 32767 values, inclusive" -- so a
// is before b iff the unsigned distance (b - a) is in [1, 32767].
//
// The naive int16 signed-subtraction form (`int16(a-b) < 0`) gets the
// exact half-space boundary wrong: it would classify diff=32768 as
// "before", but the RFC treats that as undefined (exactly 32768 apart is
// neither before nor after -- we return false to match accel-ppp's
// nsnr_cmp at l2tp.c:242-264).
func seqBefore(a, b uint16) bool {
	diff := b - a
	return diff > 0 && diff <= 32767
}

// retentionDuration computes the post-teardown state retention period,
// which RFC 2661 Section 5.8 defines as "the full retransmission interval
// after the final message exchange has occurred". That is the cumulative
// time that max_retransmit attempts would take with exponential backoff.
//
// With defaults (rtimeout=1s, cap=16s, maxRetransmit=5) the schedule is
// 1+2+4+8+16 = 31 seconds. With a tighter cap or fewer retries, retention
// shrinks accordingly. Calling with maxRetransmit <= 0 returns 0.
func retentionDuration(rtimeout, rtimeoutCap time.Duration, maxRetransmit int) time.Duration {
	var total time.Duration
	current := rtimeout
	for range maxRetransmit {
		total += current
		current *= 2
		if current > rtimeoutCap {
			current = rtimeoutCap
		}
	}
	return total
}
