package trafficvpp

import (
	"strings"
	"testing"

	"go.fd.io/govpp/binapi/policer_types"

	"codeberg.org/thomas-mangin/ze/internal/component/traffic"
)

func TestRateToKbpsRounding(t *testing.T) {
	cases := []struct {
		name string
		bps  uint64
		want uint32
	}{
		{"exact 1kbps", 1000, 1},
		{"round up from 1", 1, 1},
		{"round up from 999", 999, 1},
		{"round up from 1001", 1001, 2},
		{"round up from 1500", 1500, 2},
		{"1Gbps", 1_000_000_000, 1_000_000},
		{"max valid", uint64(^uint32(0)) * 1000, ^uint32(0)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := rateToKbps(c.bps)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Fatalf("bps=%d: want %d kbps, got %d", c.bps, c.want, got)
			}
		})
	}
}

func TestRateToKbpsErrors(t *testing.T) {
	if _, err := rateToKbps(0); err == nil {
		t.Fatal("expected error for 0 bps")
	}
	overflow := uint64(^uint32(0))*1000 + 1001
	if _, err := rateToKbps(overflow); err == nil {
		t.Fatalf("expected overflow error for %d bps", overflow)
	}
}

func TestBurstBytesFloor(t *testing.T) {
	// VALIDATES: very low rates still return at least minBurstBytes so a
	// 1kbps policer admits one MTU of burst, not 12 bytes.
	if got := burstBytes(1); got < minBurstBytes {
		t.Errorf("burst(1 kbps): want >= %d, got %d", minBurstBytes, got)
	}
}

func TestBurstBytesScalesWithRate(t *testing.T) {
	// VALIDATES: above the floor, burst grows with rate.
	low := burstBytes(100_000)  // 100 Mbps -> 1.25 MB
	high := burstBytes(500_000) // 500 Mbps -> 6.25 MB
	if high <= low {
		t.Errorf("burst must scale above the floor; low=%d high=%d", low, high)
	}
}

func TestPolicerFromClassHTB(t *testing.T) {
	cls := traffic.TrafficClass{Name: "premium", Rate: 10_000_000, Ceil: 20_000_000}
	p, err := policerFromClass(cls, traffic.QdiscHTB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.IsAdd {
		t.Error("IsAdd must be true")
	}
	if p.Name != "premium" {
		t.Errorf("Name: want premium, got %q", p.Name)
	}
	if p.Cir != 10_000 {
		t.Errorf("Cir: want 10000 kbps, got %d", p.Cir)
	}
	if p.Eir != 20_000 {
		t.Errorf("Eir: want 20000 kbps, got %d", p.Eir)
	}
	if p.Type != policer_types.SSE2_QOS_POLICER_TYPE_API_2R3C_RFC_2698 {
		t.Errorf("Type: want 2R3C_RFC_2698, got %v", p.Type)
	}
	if p.RateType != policer_types.SSE2_QOS_RATE_API_KBPS {
		t.Errorf("RateType: want KBPS, got %v", p.RateType)
	}
	if p.RoundType != policer_types.SSE2_QOS_ROUND_API_TO_UP {
		t.Errorf("RoundType: want TO_UP, got %v", p.RoundType)
	}
	if p.ColorAware {
		t.Error("HTB translation must be color-blind")
	}
	if p.ConformAction.Type != policer_types.SSE2_QOS_ACTION_API_TRANSMIT {
		t.Errorf("ConformAction: want TRANSMIT, got %v", p.ConformAction.Type)
	}
	if p.ExceedAction.Type != policer_types.SSE2_QOS_ACTION_API_TRANSMIT {
		t.Errorf("ExceedAction: want TRANSMIT (HTB), got %v", p.ExceedAction.Type)
	}
	if p.ViolateAction.Type != policer_types.SSE2_QOS_ACTION_API_DROP {
		t.Errorf("ViolateAction: want DROP, got %v", p.ViolateAction.Type)
	}
	if p.Cb == 0 || p.Eb == 0 {
		t.Error("Cb and Eb must be non-zero")
	}
}

func TestPolicerFromClassHTBNoCeil(t *testing.T) {
	cls := traffic.TrafficClass{Name: "basic", Rate: 5_000_000}
	p, err := policerFromClass(cls, traffic.QdiscHTB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Cir != 5_000 || p.Eir != 5_000 {
		t.Errorf("Cir=Eir=5000 expected when Ceil is zero, got Cir=%d Eir=%d", p.Cir, p.Eir)
	}
}

func TestPolicerFromClassTBF(t *testing.T) {
	cls := traffic.TrafficClass{Name: "shaped", Rate: 100_000_000}
	p, err := policerFromClass(cls, traffic.QdiscTBF)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Cir != p.Eir {
		t.Errorf("TBF must have Cir == Eir, got Cir=%d Eir=%d", p.Cir, p.Eir)
	}
	if p.Type != policer_types.SSE2_QOS_POLICER_TYPE_API_1R2C {
		t.Errorf("Type: want 1R2C, got %v", p.Type)
	}
	if p.ExceedAction.Type != policer_types.SSE2_QOS_ACTION_API_DROP {
		t.Errorf("TBF ExceedAction: want DROP, got %v", p.ExceedAction.Type)
	}
}

func TestPolicerFromClassRejectsOtherQdisc(t *testing.T) {
	cls := traffic.TrafficClass{Name: "anon", Rate: 1000}
	for _, q := range []traffic.QdiscType{traffic.QdiscHFSC, traffic.QdiscFQ, traffic.QdiscSFQ, traffic.QdiscNetem, traffic.QdiscPrio} {
		t.Run(q.String(), func(t *testing.T) {
			if _, err := policerFromClass(cls, q); err == nil {
				t.Fatalf("qdisc %s: expected error, got nil", q)
			}
		})
	}
}

func TestPolicerFromClassOverflow(t *testing.T) {
	cls := traffic.TrafficClass{Name: "too-big", Rate: uint64(^uint32(0))*1000 + 2000}
	_, err := policerFromClass(cls, traffic.QdiscHTB)
	if err == nil {
		t.Fatal("expected overflow error")
	}
	if !strings.Contains(err.Error(), "overflow") && !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("expected overflow message, got %v", err)
	}
}

func TestPolicerFromClassNameIsPassthrough(t *testing.T) {
	// VALIDATES: policerFromClass does NOT truncate or rewrite the class
	// name. The backend overwrites PolicerAddDel.Name with the composed
	// "ze/<iface>/<class>" form before sending, so truncation here would
	// be dead code. The verifier enforces the 64-byte limit on the
	// composed name at verify time instead.
	longName := strings.Repeat("a", 100)
	cls := traffic.TrafficClass{Name: longName, Rate: 1000}
	p, err := policerFromClass(cls, traffic.QdiscHTB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name != longName {
		t.Errorf("Name: want passthrough %q, got %q", longName, p.Name)
	}
}
