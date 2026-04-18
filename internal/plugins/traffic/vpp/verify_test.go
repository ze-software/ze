package trafficvpp

import (
	"strings"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/traffic"
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

func TestVerifyRejectsMultiClass(t *testing.T) {
	// VALIDATES: HTB/TBF with >1 class is rejected because without
	// filter support, every policer stacks on VPP's output feature arc
	// in series. Effective rate becomes min(class_rates), which is NOT
	// the per-class shaping the operator configured.
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
		t.Fatal("multi-class HTB should be rejected under vpp")
	}
	if !strings.Contains(err.Error(), "exactly 1 class required") {
		t.Errorf("want 'exactly 1 class required' message, got %v", err)
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
	if !strings.Contains(err.Error(), "exactly 1 class required") {
		t.Errorf("want 'exactly 1 class required' message, got %v", err)
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

func TestVerifyRejectsAllFilterTypes(t *testing.T) {
	// VALIDATES: the vpp backend rejects every filter type under HTB
	// because the VPP-side classify / QoS-record pipelines are not
	// implemented; accepting them would be silent no-ops in VPP.
	for _, ft := range []traffic.FilterType{
		traffic.FilterDSCP, traffic.FilterProtocol, traffic.FilterMark,
	} {
		desired := map[string]traffic.InterfaceQoS{
			"eth0": {
				Qdisc: traffic.Qdisc{
					Type: traffic.QdiscHTB,
					Classes: []traffic.TrafficClass{
						{
							Name:    "c1",
							Rate:    1000,
							Filters: []traffic.TrafficFilter{{Type: ft, Value: 0}},
						},
					},
				},
			},
		}
		err := Verify(desired)
		if err == nil {
			t.Errorf("filter %s should be rejected under vpp", ft)
			continue
		}
		if !strings.Contains(err.Error(), "not supported by backend vpp") {
			t.Errorf("filter %s: expected 'not supported' message, got %v", ft, err)
		}
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
