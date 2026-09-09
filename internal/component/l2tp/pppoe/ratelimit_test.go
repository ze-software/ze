package pppoe_test

import (
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/pppoe"
)

func TestPADILimiterAllows(t *testing.T) {
	l := pppoe.NewPADILimiter(10)
	mac := [pppoe.EthALen]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}
	if !l.Check(mac) {
		t.Fatal("first PADI should be allowed")
	}
}

func TestPADILimiterDedups(t *testing.T) {
	l := pppoe.NewPADILimiter(10)
	mac := [pppoe.EthALen]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}
	if !l.Check(mac) {
		t.Fatal("first PADI should be allowed")
	}
	if l.Check(mac) {
		t.Fatal("duplicate PADI from same MAC should be blocked")
	}
}

func TestPADILimiterRateLimit(t *testing.T) {
	l := pppoe.NewPADILimiter(3)
	for i := range 3 {
		mac := [pppoe.EthALen]byte{0x01, 0x02, 0x03, 0x04, 0x05, byte(i)}
		if !l.Check(mac) {
			t.Fatalf("PADI %d should be allowed", i)
		}
	}
	mac := [pppoe.EthALen]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0xFF}
	if l.Check(mac) {
		t.Fatal("PADI beyond rate limit should be blocked")
	}
}

func TestPADILimiterDisabled(t *testing.T) {
	l := pppoe.NewPADILimiter(0)
	for i := range 100 {
		mac := [pppoe.EthALen]byte{0x01, 0x02, 0x03, 0x04, 0x05, byte(i)}
		if !l.Check(mac) {
			t.Fatalf("disabled limiter should allow all, blocked at %d", i)
		}
	}
}

// TestPADILimiterRefusesPADRInSameWindow records the evidence behind
// Assumption A-1 (spec-pppoe-padr-replay-allocates-unbounded-sessions): the
// PADILimiter's per-MAC arm is a one-per-second dedup, not a rate token
// bucket, so reusing one limiter instance for both a PADI and the PADR that
// legitimately follows it would refuse that PADR whenever it arrives inside
// the PADI's own one-second window. That is the reason this spec bounds the
// resource (a per-MAC session cap) rather than reusing PADILimiter for PADR
// admission (Key Design Decisions, "Bound the resource, not the packet").
//
// VALIDATES: A-1 -- Check(mac) called for a PADR immediately after Check(mac)
// was already called for that MAC's PADI, inside the same window, is refused.
func TestPADILimiterRefusesPADRInSameWindow(t *testing.T) {
	l := pppoe.NewPADILimiter(10)
	mac := [pppoe.EthALen]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}

	if !l.Check(mac) {
		t.Fatal("the PADI itself should be allowed")
	}
	if l.Check(mac) {
		t.Fatal("A-1 is broken: the PADR that legitimately follows its own PADI " +
			"in the same one-second window was allowed through a shared PADILimiter, " +
			"so reusing the PADI limiter for PADR admission would not have refused " +
			"the retransmission it is supposed to dedup")
	}
}
