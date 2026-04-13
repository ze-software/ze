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
			err := ValidateCeil(tt.rate, tt.ceil)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCeil(%d, %d) error = %v, wantErr %v", tt.rate, tt.ceil, err, tt.wantErr)
			}
		})
	}
}
