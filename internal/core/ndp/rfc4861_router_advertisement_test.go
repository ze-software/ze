// RFC 4861 sender obligations on the Router Advertisement this package encodes
// and on the schedule it computes.
//
// VALIDATES: the Reserved fields and the prefix bits past the prefix length
// are written as zero over a buffer that held ones (Sections 4.2 and 4.6.2);
// a solicited answer waits 0 to MAX_RA_DELAY_TIME and respects
// MIN_DELAY_BETWEEN_RAS (Section 6.2.6); an unsolicited interval past the
// initial burst is never shorter than MinRtrAdvInterval (Section 6.2.6).
// PREVENTS: stale buffer content leaking into a Reserved field, and a
// schedule that floods the link.

package ndp

import (
	"net/netip"
	"testing"
	"time"
)

// rfc4861DirtyBuffer returns a buffer of n octets that all hold 0xff, so a
// field the encoder forgets to write reads as non-zero.
func rfc4861DirtyBuffer(n int) []byte {
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = 0xff
	}
	return buf
}

// RFC requirement: RFC4861-4.2-2 positive -- BuildRA, writing over a buffer of
// 0xff octets with both the M and O flags set, leaves the six Reserved bits of
// the flags octet zero.
// RFC requirement: RFC4861-4.6.2-3 positive -- BuildRA, writing a Prefix
// Information option with both the L and A flags set over a buffer of 0xff
// octets, leaves the six Reserved1 bits zero.
// RFC requirement: RFC4861-4.6.2-5 positive -- the four Reserved2 octets of that
// Prefix Information option are written as zero over a buffer of 0xff octets.
// RFC requirement: RFC4861-4.6.2-6 positive -- for a configured prefix that
// carries bits past its /48 length, the Prefix field holds those bits as zero.
// RFC requirement: RFC4861-4.2-2 negative -- the M and O flags in the same octet are still written as set, so the zero Reserved bits are not a blanket clear of the octet.
// RFC requirement: RFC4861-4.6.2-3 negative -- the L and A flags in the same octet are still written as set, so the zero Reserved1 bits are not a blanket clear of the octet.
// RFC requirement: RFC4861-4.6.2-6 negative -- the 48 prefix bits inside the prefix length are kept (2001:db8:1::), so only the bits past the length are cleared.
func TestRFC4861RouterAdvertisementReservedFieldsAreZero(t *testing.T) {
	cfg := RAConfig{
		Managed:     true,
		OtherConfig: true,
		Prefixes: []PrefixInfo{{
			Prefix:     netip.MustParsePrefix("2001:db8:1:2:3:4:5:6/48"),
			OnLink:     true,
			Autonomous: true,
		}},
	}
	buf := rfc4861DirtyBuffer(RALen(cfg))
	if n := BuildRA(buf, 0, cfg); n != RALen(cfg) {
		t.Fatalf("BuildRA wrote %d octets, want %d", n, RALen(cfg))
	}

	if got := buf[5]; got != 0xc0 {
		t.Errorf("flags octet = %#02x, want 0xc0: M and O set, six Reserved bits zero", got)
	}

	option := buf[raHeaderLen:]
	if option[0] != OptPrefixInformation {
		t.Fatalf("option type = %d, want %d", option[0], OptPrefixInformation)
	}
	if got := option[3]; got != 0xc0 {
		t.Errorf("prefix flags octet = %#02x, want 0xc0: L and A set, six Reserved1 bits zero", got)
	}
	for i, b := range option[12:16] {
		if b != 0 {
			t.Errorf("Reserved2 octet %d = %#02x, want 0", i, b)
		}
	}
	want := netip.MustParseAddr("2001:db8:1::").As16()
	if got := [16]byte(option[16:32]); got != want {
		t.Errorf("Prefix field = % x, want % x: bits past the prefix length are zero", got, want)
	}
}

// RFC requirement: RFC4861-6.2.6-2 positive -- over 1000 draws, the delay before
// answering a Router Solicitation is never below 0 nor above MAX_RA_DELAY_TIME
// (0.5 seconds), and it takes more than one value.
func TestRFC4861SolicitedAnswerDelayIsRandomUpToMaxRADelayTime(t *testing.T) {
	r := newTestRand()
	seen := make(map[time.Duration]struct{})
	for range 1000 {
		got := SolicitedDelay(r)
		if got < 0 {
			t.Fatalf("delay %v is negative", got)
		}
		if got > 500*time.Millisecond {
			t.Fatalf("delay %v is above MAX_RA_DELAY_TIME (0.5 seconds)", got)
		}
		seen[got] = struct{}{}
	}
	if len(seen) < 2 {
		t.Error("the delay is constant, not random")
	}
}

// RFC requirement: RFC4861-6.2.6-3 positive -- an answer to a solicitation that
// arrives 0.5 seconds after the last multicast advertisement is scheduled no
// earlier than MIN_DELAY_BETWEEN_RAS (3 seconds) after that advertisement.
func TestRFC4861SolicitedAnswerIsRateLimited(t *testing.T) {
	last := time.Unix(1000, 0)
	now := last.Add(500 * time.Millisecond)
	got := SolicitedSendTime(last, now, 100*time.Millisecond)
	if got.Before(last.Add(3 * time.Second)) {
		t.Errorf("answer scheduled %v after the last advertisement, want at least 3s", got.Sub(last))
	}
}

// RFC requirement: RFC4861-6.2.6-4 negative -- past the initial burst, none of
// 1000 unsolicited intervals drawn for a 200 to 600 second configuration is
// shorter than the 200 second minimum-interval.
func TestRFC4861UnsolicitedIntervalIsNeverBelowMinRtrAdvInterval(t *testing.T) {
	r := newTestRand()
	minimum := 200 * time.Second
	for i := range 1000 {
		if got := UnsolicitedInterval(minimum, 600*time.Second, MaxInitialAdvertisements+i, r); got < minimum {
			t.Fatalf("interval %v is shorter than MinRtrAdvInterval %v", got, minimum)
		}
	}
}
