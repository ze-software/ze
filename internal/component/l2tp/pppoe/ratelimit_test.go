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
