package traffic

import (
	"testing"
)

// VALIDATES: AC-5 "InterfaceQoS with HTB: Qdisc, classes, filters all representable".
// PREVENTS: missing traffic control model fields.
func TestInterfaceQoSConstruction(t *testing.T) {
	qos := InterfaceQoS{
		Interface: "eth0",
		Qdisc: Qdisc{
			Type:         QdiscHTB,
			DefaultClass: "bulk",
			Classes: []TrafficClass{
				{
					Name:     "voip",
					Rate:     10_000_000,
					Ceil:     100_000_000,
					Priority: 0,
					Filters: []TrafficFilter{
						{Type: FilterMark, Value: 0x10},
					},
				},
				{
					Name:     "interactive",
					Rate:     5_000_000,
					Ceil:     100_000_000,
					Priority: 1,
					Filters: []TrafficFilter{
						{Type: FilterMark, Value: 0x20},
					},
				},
				{
					Name:     "bulk",
					Rate:     85_000_000,
					Ceil:     100_000_000,
					Priority: 2,
				},
			},
		},
	}
	if qos.Interface != "eth0" {
		t.Errorf("Interface = %q, want %q", qos.Interface, "eth0")
	}
	if qos.Qdisc.Type != QdiscHTB {
		t.Errorf("Qdisc.Type = %v, want %v", qos.Qdisc.Type, QdiscHTB)
	}
	if len(qos.Qdisc.Classes) != 3 {
		t.Fatalf("Qdisc.Classes len = %d, want 3", len(qos.Qdisc.Classes))
	}
	if qos.Qdisc.Classes[0].Name != "voip" {
		t.Errorf("Classes[0].Name = %q, want %q", qos.Qdisc.Classes[0].Name, "voip")
	}
	if qos.Qdisc.Classes[0].Rate != 10_000_000 {
		t.Errorf("Classes[0].Rate = %d, want 10000000", qos.Qdisc.Classes[0].Rate)
	}
	if len(qos.Qdisc.Classes[0].Filters) != 1 {
		t.Fatalf("Classes[0].Filters len = %d, want 1", len(qos.Qdisc.Classes[0].Filters))
	}
	if qos.Qdisc.DefaultClass != "bulk" {
		t.Errorf("DefaultClass = %q, want %q", qos.Qdisc.DefaultClass, "bulk")
	}
}

func TestQdiscType(t *testing.T) {
	tests := []struct {
		name  string
		qt    QdiscType
		str   string
		valid bool
	}{
		{"htb", QdiscHTB, "htb", true},
		{"hfsc", QdiscHFSC, "hfsc", true},
		{"fq", QdiscFQ, "fq", true},
		{"fq_codel", QdiscFQCodel, "fq_codel", true},
		{"sfq", QdiscSFQ, "sfq", true},
		{"tbf", QdiscTBF, "tbf", true},
		{"netem", QdiscNetem, "netem", true},
		{"prio", QdiscPrio, "prio", true},
		{"clsact", QdiscClsact, "clsact", true},
		{"ingress", QdiscIngress, "ingress", true},
		{"unknown zero", QdiscType(0), "unknown", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.qt.String(); got != tt.str {
				t.Errorf("String() = %q, want %q", got, tt.str)
			}
			if got := tt.qt.Valid(); got != tt.valid {
				t.Errorf("Valid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

// TestParseQdiscTypeRefusesIngressHookKinds
// VALIDATES: the two directions disagree on purpose. String and Valid still
// name clsact and ingress, because the backend reads them off an interface;
// ParseQdiscType refuses them, because no operator can ask for one. Method:
// parse each name and require the refusal, then parse a real discipline.
// PREVENTS: a second enumeration that outlives the YANG one. The schema stopped
// offering both kinds, and a config path that reaches ParseQdiscType without
// the schema validator would otherwise still produce a qdisc no backend builds.
func TestParseQdiscTypeRefusesIngressHookKinds(t *testing.T) {
	for _, name := range []string{"clsact", "ingress"} {
		if qt, ok := ParseQdiscType(name); ok {
			t.Errorf("ParseQdiscType(%q) = %v, want refused", name, qt)
		}
	}
	if qt, ok := ParseQdiscType("htb"); !ok || qt != QdiscHTB {
		t.Errorf("ParseQdiscType(\"htb\") = %v, %v, want QdiscHTB, true", qt, ok)
	}
	if !QdiscClsact.Valid() || QdiscClsact.String() != "clsact" {
		t.Error("readback must still name clsact")
	}
}

func TestFilterType(t *testing.T) {
	tests := []struct {
		name  string
		ft    FilterType
		str   string
		valid bool
	}{
		{"mark", FilterMark, "mark", true},
		{"dscp", FilterDSCP, "dscp", true},
		{"protocol", FilterProtocol, "protocol", true},
		{"unknown zero", FilterType(0), "unknown", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ft.String(); got != tt.str {
				t.Errorf("String() = %q, want %q", got, tt.str)
			}
			if got := tt.ft.Valid(); got != tt.valid {
				t.Errorf("Valid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

// Boundary tests.

func TestHTBRateBoundary(t *testing.T) {
	tests := []struct {
		name    string
		rate    uint64
		wantErr bool
	}{
		{"valid min", 1, false},
		{"invalid zero", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRate(tt.rate)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRate(%d) error = %v, wantErr %v", tt.rate, err, tt.wantErr)
			}
		})
	}
}

func TestHTBCeilBoundary(t *testing.T) {
	// Ceil must be >= rate.
	tests := []struct {
		name    string
		rate    uint64
		ceil    uint64
		wantErr bool
	}{
		{"ceil equals rate", 100, 100, false},
		{"ceil above rate", 100, 200, false},
		{"ceil below rate", 100, 50, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCeil(tt.rate, tt.ceil)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCeil(%d, %d) error = %v, wantErr %v", tt.rate, tt.ceil, err, tt.wantErr)
			}
		})
	}
}

// TestPolicerSetDistinguishesAbsentFromConfigured checks the guard that tells
// "this interface asks for no ingress enforcement" apart from "this interface
// asks for a rate". The zero rate is not a rate ValidateRate accepts, so the
// two are distinguishable, and Set is the name that says so.
func TestPolicerSetDistinguishesAbsentFromConfigured(t *testing.T) {
	var absent Policer
	if absent.Set() {
		t.Fatal("zero Policer reports Set")
	}
	if got := NewPolicer(5_000_000); !got.Set() {
		t.Fatal("NewPolicer(5mbit) reports not Set")
	}
}

// TestNewPolicerFillsBurst checks that a policer built from a rate carries a
// burst, because a token bucket with no depth drops every packet.
func TestNewPolicerFillsBurst(t *testing.T) {
	p := NewPolicer(80_000_000)
	if p.RateBps != 80_000_000 {
		t.Fatalf("RateBps = %d, want 80000000", p.RateBps)
	}
	// 100ms of 80 Mbit/s is 1_000_000 bytes.
	if p.BurstBytes != 1_000_000 {
		t.Fatalf("BurstBytes = %d, want 1000000", p.BurstBytes)
	}
}

// TestPolicerBurstBytesFloor checks the floor: at a low rate the 100ms window
// yields less than one MTU, and a bucket that cannot hold one packet drops
// every packet.
func TestPolicerBurstBytesFloor(t *testing.T) {
	if got := PolicerBurstBytes(1_000); got != 2048 {
		t.Fatalf("PolicerBurstBytes(1kbit) = %d, want the 2048 floor", got)
	}
	if got := PolicerBurstBytes(0); got != 2048 {
		t.Fatalf("PolicerBurstBytes(0) = %d, want the 2048 floor", got)
	}
}

// TestPolicerValidateRejectsBurstlessRate checks that a policer carrying a rate
// and no burst is refused rather than programmed, because the kernel would
// accept it and drop every packet.
func TestPolicerValidateRejectsBurstlessRate(t *testing.T) {
	if err := (Policer{RateBps: 1_000_000}).Validate(); err == nil {
		t.Fatal("Validate accepted a rate with no burst")
	}
	if err := (Policer{}).Validate(); err != nil {
		t.Fatalf("Validate rejected an absent policer: %v", err)
	}
	if err := NewPolicer(1_000_000).Validate(); err != nil {
		t.Fatalf("Validate rejected NewPolicer output: %v", err)
	}
}

// TestInterfaceQoSCarriesIngressPolicer checks that the data model can express
// the upload direction at all. Before this field the model held one root Qdisc,
// which is egress only, so no upload enforcement was reachable through Backend.
func TestInterfaceQoSCarriesIngressPolicer(t *testing.T) {
	qos := InterfaceQoS{
		Interface: "ppp0",
		Ingress:   NewPolicer(5_000_000),
	}
	if !qos.Ingress.Set() {
		t.Fatal("InterfaceQoS.Ingress does not carry the policer")
	}
}
