package trafficvpp

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/traffic"
)

// singleClassHTB returns a minimal accept-case config: HTB with exactly
// one class, no filters, for a given interface name.
func singleClassHTB(iface string) map[string]traffic.InterfaceQoS {
	return map[string]traffic.InterfaceQoS{
		iface: {
			Qdisc: traffic.Qdisc{
				Type:    traffic.QdiscHTB,
				Classes: []traffic.TrafficClass{{Name: "c1", Rate: 1000}},
			},
		},
	}
}

func TestVerifyAcceptsHTBOneClass(t *testing.T) {
	if err := Verify(singleClassHTB("eth0")); err != nil {
		t.Fatalf("HTB with one class should be accepted, got %v", err)
	}
}

func TestVerifyAcceptsTBFOneClass(t *testing.T) {
	desired := map[string]traffic.InterfaceQoS{
		"eth0": {
			Qdisc: traffic.Qdisc{
				Type:    traffic.QdiscTBF,
				Classes: []traffic.TrafficClass{{Name: "c1", Rate: 1000}},
			},
		},
	}
	if err := Verify(desired); err != nil {
		t.Errorf("TBF with one class should be accepted, got %v", err)
	}
}

func TestVerifyRejectsMultiClassWithoutFilters(t *testing.T) {
	// VALIDATES: AC-5/AC-6 -- HTB/TBF with >1 class is rejected when any class
	// lacks a steering filter, because such a class falls back to VPP's egress
	// output arc and stacks IN SERIES with the others (effective rate =
	// min(rates)), NOT the per-class shaping the operator configured. Multi-class
	// with a filter on every class is accepted (see
	// TestVerifyAcceptsMultiClassWithFilters).
	desired := map[string]traffic.InterfaceQoS{
		"eth0": {
			Qdisc: traffic.Qdisc{
				Type: traffic.QdiscHTB,
				Classes: []traffic.TrafficClass{
					{Name: "fast", Rate: 10_000_000},
					{Name: "slow", Rate: 1_000_000},
				},
			},
		},
	}
	err := Verify(desired)
	if err == nil {
		t.Fatal("multi-class HTB without steering filters should be rejected under vpp")
	}
	if !strings.Contains(err.Error(), "requires every class to carry a steering filter") {
		t.Errorf("want 'requires every class to carry a steering filter' message, got %v", err)
	}
}

func TestVerifyAcceptsMultiClassWithFilters(t *testing.T) {
	// VALIDATES: AC-5 -- HTB with 2+ classes is accepted when EVERY class carries
	// a steering filter (protocol/dscp): classify steers each class's traffic to
	// its own policer, so per-class shaping is faithful.
	desired := map[string]traffic.InterfaceQoS{
		"eth0": {
			Qdisc: traffic.Qdisc{
				Type: traffic.QdiscHTB,
				Classes: []traffic.TrafficClass{
					{Name: "fast", Rate: 10_000_000, Filters: []traffic.TrafficFilter{{Type: traffic.FilterProtocol, Value: 6}}},
					{Name: "slow", Rate: 1_000_000, Filters: []traffic.TrafficFilter{{Type: traffic.FilterDSCP, Value: 48}}},
				},
			},
		},
	}
	if err := Verify(desired); err != nil {
		t.Fatalf("multi-class HTB with a filter on every class should be accepted, got %v", err)
	}
}

func TestVerifyRejectsDuplicateSteeringAcrossClasses(t *testing.T) {
	// VALIDATES: exact-or-reject -- two classes selecting the IDENTICAL steering
	// match (both protocol tcp) collide on one classify session; VPP keeps only
	// the last, silently dropping a class's policing. Reject at verify.
	desired := map[string]traffic.InterfaceQoS{
		"eth0": {
			Qdisc: traffic.Qdisc{
				Type: traffic.QdiscHTB,
				Classes: []traffic.TrafficClass{
					{Name: "a", Rate: 1000, Filters: []traffic.TrafficFilter{{Type: traffic.FilterProtocol, Value: 6}}},
					{Name: "b", Rate: 2000, Filters: []traffic.TrafficFilter{{Type: traffic.FilterProtocol, Value: 6}}},
				},
			},
		},
	}
	err := Verify(desired)
	if err == nil {
		t.Fatal("duplicate protocol steering across classes should be rejected")
	}
	if !strings.Contains(err.Error(), "used by both class") {
		t.Errorf("want 'used by both class' message, got %v", err)
	}
	// Distinct values on the two classes are fine.
	desired["eth0"].Qdisc.Classes[1].Filters[0].Value = 17
	if err := Verify(desired); err != nil {
		t.Fatalf("distinct steering values should be accepted, got %v", err)
	}
}

func TestVerifyRejectsZeroClasses(t *testing.T) {
	// VALIDATES: qdisc with no classes has no rate to program and is
	// rejected so the operator sees an explicit error instead of an
	// apply that silently programs nothing.
	desired := map[string]traffic.InterfaceQoS{
		"eth0": {Qdisc: traffic.Qdisc{Type: traffic.QdiscHTB}},
	}
	err := Verify(desired)
	if err == nil {
		t.Fatal("zero-class HTB should be rejected under vpp")
	}
	if !strings.Contains(err.Error(), "at least 1 class required") {
		t.Errorf("want 'at least 1 class required' message, got %v", err)
	}
}

func TestVerifyRejectsUnsupportedQdiscs(t *testing.T) {
	for _, q := range []traffic.QdiscType{
		traffic.QdiscHFSC, traffic.QdiscFQ, traffic.QdiscSFQ,
		traffic.QdiscFQCodel, traffic.QdiscNetem,
		traffic.QdiscPrio, traffic.QdiscClsact, traffic.QdiscIngress,
	} {
		desired := map[string]traffic.InterfaceQoS{
			"eth0": {Qdisc: traffic.Qdisc{Type: q}},
		}
		err := Verify(desired)
		if err == nil {
			t.Errorf("%s should be rejected", q)
			continue
		}
		if !strings.Contains(err.Error(), "not supported by backend vpp") {
			t.Errorf("%s: expected 'not supported' message, got %v", q, err)
		}
	}
}

func TestVerifyRejectsMarkFilter(t *testing.T) {
	// VALIDATES: AC-3/AC-6 -- mark filters stay rejected under vpp (Linux SKB
	// fwmark has no faithful VPP equivalent). DSCP is now ACCEPTED (see
	// TestVerifyAcceptsDscpFilter); only mark and prio remain rejected.
	desired := map[string]traffic.InterfaceQoS{
		"eth0": {
			Qdisc: traffic.Qdisc{
				Type: traffic.QdiscHTB,
				Classes: []traffic.TrafficClass{
					{
						Name:    "c1",
						Rate:    1000,
						Filters: []traffic.TrafficFilter{{Type: traffic.FilterMark, Value: 0}},
					},
				},
			},
		},
	}
	err := Verify(desired)
	if err == nil {
		t.Fatal("filter mark should be rejected under vpp")
	}
	if !strings.Contains(err.Error(), "not supported by backend vpp") {
		t.Errorf("filter mark: expected 'not supported' message, got %v", err)
	}
}

func TestVerifyAcceptsDscpFilter(t *testing.T) {
	// VALIDATES: AC-2 (POLICE-BY-DSCP, USER decision 2026-07-10) -- a dscp filter
	// on the single class is accepted; the backend classifies the DSCP bits and
	// steers matching traffic to the class policer (same pipeline as protocol).
	desired := map[string]traffic.InterfaceQoS{
		"eth0": {
			Qdisc: traffic.Qdisc{
				Type: traffic.QdiscHTB,
				Classes: []traffic.TrafficClass{
					{
						Name:    "c1",
						Rate:    1000,
						Filters: []traffic.TrafficFilter{{Type: traffic.FilterDSCP, Value: 48}},
					},
				},
			},
		},
	}
	if err := Verify(desired); err != nil {
		t.Fatalf("dscp filter should be accepted under vpp, got %v", err)
	}
}

func TestVerifyDscpBoundary(t *testing.T) {
	// VALIDATES: boundary (dscp 0-63) -- 0 and 63 accepted, 64 rejected. The
	// classify TOS/TC mask covers exactly the 6-bit DSCP field.
	mk := func(v uint32) map[string]traffic.InterfaceQoS {
		return map[string]traffic.InterfaceQoS{
			"eth0": {
				Qdisc: traffic.Qdisc{
					Type: traffic.QdiscHTB,
					Classes: []traffic.TrafficClass{
						{Name: "c1", Rate: 1000, Filters: []traffic.TrafficFilter{{Type: traffic.FilterDSCP, Value: v}}},
					},
				},
			},
		}
	}
	for _, v := range []uint32{0, 63} {
		if err := Verify(mk(v)); err != nil {
			t.Errorf("dscp %d should be accepted, got %v", v, err)
		}
	}
	err := Verify(mk(64))
	if err == nil {
		t.Fatal("dscp 64 should be rejected (out of range 0-63)")
	}
	if !strings.Contains(err.Error(), "out of range") {
		t.Errorf("want 'out of range' message for dscp 64, got %v", err)
	}
}

func TestVerifyRejectsPrioWithActionableError(t *testing.T) {
	// VALIDATES: AC-4 (RESOLVED: rejection-retained, USER decision 2026-07-10) --
	// qdisc prio is rejected with an actionable error naming the scheduler gap.
	desired := map[string]traffic.InterfaceQoS{
		"eth0": {Qdisc: traffic.Qdisc{Type: traffic.QdiscPrio}},
	}
	err := Verify(desired)
	if err == nil {
		t.Fatal("qdisc prio should be rejected under vpp")
	}
	if !strings.Contains(err.Error(), "not supported by backend vpp") {
		t.Errorf("want 'not supported by backend vpp', got %v", err)
	}
	if !strings.Contains(err.Error(), "priority scheduler") {
		t.Errorf("want actionable 'priority scheduler' explanation, got %v", err)
	}
}

func TestVerifyAcceptsProtocolFilter(t *testing.T) {
	// VALIDATES: AC-1 -- a protocol filter on the single class is accepted;
	// the backend programs the classify + policer-classify pipeline.
	desired := map[string]traffic.InterfaceQoS{
		"eth0": {
			Qdisc: traffic.Qdisc{
				Type: traffic.QdiscHTB,
				Classes: []traffic.TrafficClass{
					{
						Name:    "c1",
						Rate:    1000,
						Filters: []traffic.TrafficFilter{{Type: traffic.FilterProtocol, Value: 6}},
					},
				},
			},
		},
	}
	if err := Verify(desired); err != nil {
		t.Fatalf("protocol filter should be accepted under vpp, got %v", err)
	}
}

func TestVerifyRejectsProtocolOutOfRange(t *testing.T) {
	// VALIDATES: boundary -- a protocol value above 255 does not fit the
	// single classify byte and is rejected at verify.
	desired := map[string]traffic.InterfaceQoS{
		"eth0": {
			Qdisc: traffic.Qdisc{
				Type: traffic.QdiscHTB,
				Classes: []traffic.TrafficClass{
					{
						Name:    "c1",
						Rate:    1000,
						Filters: []traffic.TrafficFilter{{Type: traffic.FilterProtocol, Value: 256}},
					},
				},
			},
		},
	}
	err := Verify(desired)
	if err == nil {
		t.Fatal("protocol value 256 should be rejected (out of range)")
	}
	if !strings.Contains(err.Error(), "out of range") {
		t.Errorf("want 'out of range' message, got %v", err)
	}
}

func TestVerifyReportsInterfaceName(t *testing.T) {
	desired := map[string]traffic.InterfaceQoS{
		"wan0": {Qdisc: traffic.Qdisc{Type: traffic.QdiscHFSC}},
	}
	err := Verify(desired)
	if err == nil {
		t.Fatal("expected rejection")
	}
	if !strings.Contains(err.Error(), `"wan0"`) {
		t.Errorf("error should name the offending interface, got %v", err)
	}
}

func TestVerifyReportsAllBadInterfaces(t *testing.T) {
	desired := map[string]traffic.InterfaceQoS{
		"eth0": {Qdisc: traffic.Qdisc{Type: traffic.QdiscHFSC}},
		"eth1": {Qdisc: traffic.Qdisc{Type: traffic.QdiscFQ}},
	}
	err := Verify(desired)
	if err == nil {
		t.Fatal("expected rejection")
	}
	msg := err.Error()
	if !strings.Contains(msg, `"eth0"`) || !strings.Contains(msg, `"eth1"`) {
		t.Errorf("error should name both offending interfaces, got %v", err)
	}
}

func TestVerifyRejectsSeparatorInIfaceName(t *testing.T) {
	desired := map[string]traffic.InterfaceQoS{
		"eth/0": {
			Qdisc: traffic.Qdisc{
				Type:    traffic.QdiscHTB,
				Classes: []traffic.TrafficClass{{Name: "c1", Rate: 1000}},
			},
		},
	}
	err := Verify(desired)
	if err == nil {
		t.Fatal("expected rejection for interface name containing /")
	}
	if !strings.Contains(err.Error(), "reserved as policer-name separator") {
		t.Errorf("want separator-reserved message, got %v", err)
	}
}

func TestVerifyRejectsSeparatorInClassName(t *testing.T) {
	desired := map[string]traffic.InterfaceQoS{
		"eth0": {
			Qdisc: traffic.Qdisc{
				Type:    traffic.QdiscHTB,
				Classes: []traffic.TrafficClass{{Name: "sub/class", Rate: 1000}},
			},
		},
	}
	err := Verify(desired)
	if err == nil {
		t.Fatal("expected rejection for class name containing /")
	}
	if !strings.Contains(err.Error(), "reserved as policer-name separator") {
		t.Errorf("want separator-reserved message, got %v", err)
	}
}

func TestVerifyRejectsZeroRate(t *testing.T) {
	desired := map[string]traffic.InterfaceQoS{
		"eth0": {
			Qdisc: traffic.Qdisc{
				Type:    traffic.QdiscHTB,
				Classes: []traffic.TrafficClass{{Name: "c1", Rate: 0}},
			},
		},
	}
	err := Verify(desired)
	if err == nil {
		t.Fatal("expected rejection for zero Rate")
	}
	if !strings.Contains(err.Error(), "rate must be >= 1") {
		t.Errorf("want ValidateRate message, got %v", err)
	}
}

func TestVerifyRejectsCeilBelowRate(t *testing.T) {
	desired := map[string]traffic.InterfaceQoS{
		"eth0": {
			Qdisc: traffic.Qdisc{
				Type: traffic.QdiscHTB,
				Classes: []traffic.TrafficClass{
					{Name: "c1", Rate: 10_000_000, Ceil: 5_000_000},
				},
			},
		},
	}
	err := Verify(desired)
	if err == nil {
		t.Fatal("expected rejection for Ceil < Rate")
	}
	if !strings.Contains(err.Error(), "ceil") {
		t.Errorf("want ValidateCeil message, got %v", err)
	}
}

func TestVerifyRejectsDanglingDefaultClass(t *testing.T) {
	desired := map[string]traffic.InterfaceQoS{
		"eth0": {
			Qdisc: traffic.Qdisc{
				Type:         traffic.QdiscHTB,
				DefaultClass: "nonexistent",
				Classes:      []traffic.TrafficClass{{Name: "c1", Rate: 1000}},
			},
		},
	}
	err := Verify(desired)
	if err == nil {
		t.Fatal("expected rejection for default-class naming a nonexistent class")
	}
	if !strings.Contains(err.Error(), "default-class") {
		t.Errorf("want default-class message, got %v", err)
	}
}

func TestVerifyAcceptsDefaultClassMatchingSingleClass(t *testing.T) {
	desired := map[string]traffic.InterfaceQoS{
		"eth0": {
			Qdisc: traffic.Qdisc{
				Type:         traffic.QdiscHTB,
				DefaultClass: "c1",
				Classes:      []traffic.TrafficClass{{Name: "c1", Rate: 1000}},
			},
		},
	}
	if err := Verify(desired); err != nil {
		t.Fatalf("default-class matching the single class should be accepted, got %v", err)
	}
}

func TestPolicerNameBoundaryMultiClass(t *testing.T) {
	// VALIDATES: R-5 boundary -- in a multi-class config the 64-byte VPP policer
	// name limit is enforced per class. "ze/eth0/" is 8 bytes, so a 56-char
	// class name yields exactly 64 (valid) and 57 yields 65 (rejected). Both
	// classes carry a filter (multi-class requirement).
	mk := func(longLen int) map[string]traffic.InterfaceQoS {
		return map[string]traffic.InterfaceQoS{
			"eth0": {
				Qdisc: traffic.Qdisc{
					Type: traffic.QdiscHTB,
					Classes: []traffic.TrafficClass{
						{Name: "web", Rate: 1000, Filters: []traffic.TrafficFilter{{Type: traffic.FilterProtocol, Value: 6}}},
						{Name: strings.Repeat("x", longLen), Rate: 1000, Filters: []traffic.TrafficFilter{{Type: traffic.FilterProtocol, Value: 17}}},
					},
				},
			},
		}
	}
	if err := Verify(mk(56)); err != nil { // "ze/eth0/" (8) + 56 = 64, at the limit
		t.Fatalf("64-byte policer name should be accepted, got %v", err)
	}
	err := Verify(mk(57)) // 65 bytes, over the limit
	if err == nil {
		t.Fatal("65-byte policer name should be rejected in multi-class")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("want 'exceeds' message, got %v", err)
	}
}

func TestVerifyRejectsLongPolicerName(t *testing.T) {
	longName := strings.Repeat("x", 200)
	desired := map[string]traffic.InterfaceQoS{
		"eth0": {
			Qdisc: traffic.Qdisc{
				Type: traffic.QdiscHTB,
				Classes: []traffic.TrafficClass{
					{Name: longName, Rate: 1000},
				},
			},
		},
	}
	err := Verify(desired)
	if err == nil {
		t.Fatal("expected rejection for over-long policer name")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("want 'exceeds' message, got %v", err)
	}
}
