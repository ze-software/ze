package trafficvpp

import (
	"strings"
	"testing"

	"go.fd.io/govpp/binapi/policer_types"

	"github.com/ze-software/ze/internal/component/traffic"
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

// VALIDATES: AC-1 golden vectors (R-3 "matched wrong offset"). The IPv4
// protocol byte sits at absolute offset 23 (Ethernet 14 + IPv4 protocol 9)
// in a two-vector (32-byte) mask with skip=0 -- the window VPP's own CLI
// emits for `classify table mask l3 ip4 proto`, verified on VPP v25.10.
func TestClassifyMaskMatchIPv4Protocol(t *testing.T) {
	mask, match := protocolClassifyVectors(classifyIPv4, 6)
	if len(mask) != classifyMaskLen || len(match) != classifyMaskLen {
		t.Fatalf("vector len = (%d,%d), want %d", len(mask), len(match), classifyMaskLen)
	}
	assertOnlyByteSet(t, mask, ipv4ProtocolByte, 0xff)
	assertOnlyByteSet(t, match, ipv4ProtocolByte, 6)
}

// VALIDATES: AC-1/A-4 golden vectors. IPv6 next-header sits at absolute offset
// 20 (Ethernet 14 + IPv6 next-header 6); confirmed expressible on VPP v25.10.
func TestClassifyMaskMatchIPv6NextHeader(t *testing.T) {
	mask, match := protocolClassifyVectors(classifyIPv6, 17)
	assertOnlyByteSet(t, mask, ipv6NextHeaderByte, 0xff)
	assertOnlyByteSet(t, match, ipv6NextHeaderByte, 17)
}

// VALIDATES: boundary -- protocol 255 (max) encodes in the single match byte.
func TestClassifyMaskMatchMaxProtocol(t *testing.T) {
	_, match := protocolClassifyVectors(classifyIPv4, 255)
	assertOnlyByteSet(t, match, ipv4ProtocolByte, 0xff)
}

// assertOnlyByteSet fails unless buf[idx]==want and every other byte is zero.
func assertOnlyByteSet(t *testing.T, buf []byte, idx int, want byte) {
	t.Helper()
	for i, b := range buf {
		exp := byte(0)
		if i == idx {
			exp = want
		}
		if b != exp {
			t.Fatalf("byte[%d] = 0x%02x, want 0x%02x (only byte %d should be 0x%02x)", i, b, exp, idx, want)
		}
	}
}

// assertBytesSet fails unless buf equals a zero buffer with exactly the given
// {index: value} entries set.
func assertBytesSet(t *testing.T, buf []byte, want map[int]byte) {
	t.Helper()
	for i, b := range buf {
		exp := want[i]
		if b != exp {
			t.Fatalf("byte[%d] = 0x%02x, want 0x%02x (set=%v)", i, b, exp, want)
		}
	}
}

// VALIDATES: AC-2 dscp golden vectors (police-by-dscp). The IPv4 TOS byte sits
// at absolute offset 15 (Ethernet 14 + IPv4 TOS 1); DSCP is the top 6 bits so
// mask=0xFC and match=dscp<<2. Confirmed accepted by real VPP v25.10.
func TestClassifyMaskMatchIPv4Dscp(t *testing.T) {
	mask, match := dscpClassifyVectors(classifyIPv4, 48) // cs6
	if len(mask) != classifyMaskLen || len(match) != classifyMaskLen {
		t.Fatalf("vector len = (%d,%d), want %d", len(mask), len(match), classifyMaskLen)
	}
	assertOnlyByteSet(t, mask, ipv4TosByte, 0xFC)
	assertOnlyByteSet(t, match, ipv4TosByte, 48<<2) // 0xC0
}

// VALIDATES: AC-2 dscp golden vectors for IPv6. The 8-bit traffic-class field
// straddles bytes 14/15 (not byte-aligned in the first IPv6 word); DSCP =
// byte14 low nibble (0x0F) + byte15 top two bits (0xC0). dscp=48 -> byte14=0x0C,
// byte15=0x00. Confirmed accepted by real VPP v25.10.
func TestClassifyMaskMatchIPv6Dscp(t *testing.T) {
	mask, match := dscpClassifyVectors(classifyIPv6, 48)
	assertBytesSet(t, mask, map[int]byte{ipv6TrafficClassHiByte: 0x0F, ipv6TrafficClassLoByte: 0xC0})
	assertBytesSet(t, match, map[int]byte{ipv6TrafficClassHiByte: 0x0C, ipv6TrafficClassLoByte: 0x00})
}

// VALIDATES: dscp boundary (0-63). dscp=63 packs to the full 6-bit field:
// IPv4 match byte = 63<<2 = 0xFC; IPv6 byte14 = 0x0F, byte15 = 0xC0. dscp=0 is
// all-zero match (only the mask bytes are set).
func TestClassifyMaskMatchDscpBoundary(t *testing.T) {
	_, m4hi := dscpClassifyVectors(classifyIPv4, 63)
	assertOnlyByteSet(t, m4hi, ipv4TosByte, 0xFC)
	_, m6hi := dscpClassifyVectors(classifyIPv6, 63)
	assertBytesSet(t, m6hi, map[int]byte{ipv6TrafficClassHiByte: 0x0F, ipv6TrafficClassLoByte: 0xC0})

	_, m4lo := dscpClassifyVectors(classifyIPv4, 0)
	assertBytesSet(t, m4lo, map[int]byte{}) // dscp 0: no match byte set
	_, m6lo := dscpClassifyVectors(classifyIPv6, 0)
	assertBytesSet(t, m6lo, map[int]byte{}) // dscp 0: no match byte set
}

// VALIDATES: filterClassifyVectors dispatches protocol->protocol offsets,
// dscp->dscp offsets, and reports mark as non-steering (ok=false, never
// reaches the apply path).
func TestFilterClassifyVectorsDispatch(t *testing.T) {
	_, protoMatch, ok := filterClassifyVectors(classifyIPv4, traffic.TrafficFilter{Type: traffic.FilterProtocol, Value: 6})
	if !ok {
		t.Fatal("protocol filter must be steerable")
	}
	assertOnlyByteSet(t, protoMatch, ipv4ProtocolByte, 6)

	_, dscpMatch, ok := filterClassifyVectors(classifyIPv4, traffic.TrafficFilter{Type: traffic.FilterDSCP, Value: 48})
	if !ok {
		t.Fatal("dscp filter must be steerable")
	}
	assertOnlyByteSet(t, dscpMatch, ipv4TosByte, 48<<2)

	if _, _, ok := filterClassifyVectors(classifyIPv4, traffic.TrafficFilter{Type: traffic.FilterMark, Value: 7}); ok {
		t.Fatal("mark filter must NOT be steerable (ok=false)")
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
