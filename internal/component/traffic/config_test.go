package traffic

import (
	"testing"
)

// VALIDATES: AC-12 "Config with HTB qdisc, classes, mark match parsed correctly".
// PREVENTS: traffic config parsing broken.
func TestParseTrafficHTB(t *testing.T) {
	data := `{"traffic":{"control":{"interface":{"eth0":{"qdisc":{"type":"htb","default-class":"bulk","class":{"voip":{"rate":"10mbit","ceil":"100mbit","priority":"0","match":{"mark":{"value":"0x10"}}},"bulk":{"rate":"85mbit","ceil":"100mbit","priority":"2"}}}}}}}}`
	qosMap, err := parseTrafficConfig(data)
	if err != nil {
		t.Fatalf("ParseTrafficConfig: %v", err)
	}
	if len(qosMap) != 1 {
		t.Fatalf("got %d interfaces, want 1", len(qosMap))
	}
	qos, ok := qosMap["eth0"]
	if !ok {
		t.Fatal("missing eth0")
	}
	if qos.Interface != "eth0" {
		t.Errorf("Interface = %q, want %q", qos.Interface, "eth0")
	}
	if qos.Qdisc.Type != QdiscHTB {
		t.Errorf("Qdisc.Type = %v, want htb", qos.Qdisc.Type)
	}
	if qos.Qdisc.DefaultClass != "bulk" {
		t.Errorf("DefaultClass = %q, want %q", qos.Qdisc.DefaultClass, "bulk")
	}
	if len(qos.Qdisc.Classes) != 2 {
		t.Fatalf("got %d classes, want 2", len(qos.Qdisc.Classes))
	}
}

func TestParseTrafficNoSection(t *testing.T) {
	data := `{}`
	qosMap, err := parseTrafficConfig(data)
	if err != nil {
		t.Fatalf("ParseTrafficConfig: %v", err)
	}
	if len(qosMap) != 0 {
		t.Errorf("got %d entries, want 0", len(qosMap))
	}
}

func TestParseTrafficInvalidQdisc(t *testing.T) {
	data := `{"traffic":{"control":{"interface":{"eth0":{"qdisc":{"type":"invalid"}}}}}}`
	_, err := parseTrafficConfig(data)
	if err == nil {
		t.Fatal("expected error for invalid qdisc type")
	}
}

func TestParseTrafficRateFormats(t *testing.T) {
	tests := []struct {
		name string
		rate string
		want uint64
	}{
		{"mbit", "10mbit", 10_000_000},
		{"kbit", "100kbit", 100_000},
		{"gbit", "1gbit", 1_000_000_000},
		{"bit", "500bit", 500},
		{"mbps", "10mbps", 80_000_000},
		{"kbps", "100kbps", 800_000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRateBps(tt.rate)
			if err != nil {
				t.Fatalf("ParseRateBps(%q): %v", tt.rate, err)
			}
			if got != tt.want {
				t.Errorf("ParseRateBps(%q) = %d, want %d", tt.rate, got, tt.want)
			}
		})
	}
}

func TestParseTrafficInvalidRate(t *testing.T) {
	_, err := ParseRateBps("notarate")
	if err == nil {
		t.Fatal("expected error for invalid rate")
	}
}

func TestParseTrafficNamedDSCP(t *testing.T) {
	data := `{"traffic":{"control":{"interface":{"eth0":{"qdisc":{"type":"htb","default-class":"default","class":{"control":{"rate":"10mbit","ceil":"100mbit","priority":"0","match":{"dscp":{"value":"cs6"}}},"default":{"rate":"90mbit","ceil":"100mbit","priority":"1"}}}}}}}}`
	qosMap, err := parseTrafficConfig(data)
	if err != nil {
		t.Fatalf("ParseTrafficConfig: %v", err)
	}
	qos := qosMap["eth0"]
	for _, cls := range qos.Qdisc.Classes {
		if cls.Name != "control" {
			continue
		}
		if len(cls.Filters) != 1 {
			t.Fatalf("control class: got %d filters, want 1", len(cls.Filters))
		}
		f := cls.Filters[0]
		if f.Type != FilterDSCP {
			t.Fatalf("filter type = %v, want FilterDSCP", f.Type)
		}
		if f.Value != 48 {
			t.Errorf("filter value = %d, want 48 (cs6)", f.Value)
		}
		return
	}
	t.Fatal("control class not found")
}

func TestParseTrafficDSCPRejectsOutOfRange(t *testing.T) {
	data := `{"traffic":{"control":{"interface":{"eth0":{"qdisc":{"type":"htb","class":{"c":{"rate":"10mbit","match":{"dscp":{"value":"64"}}}}}}}}}}`
	_, err := parseTrafficConfig(data)
	if err == nil {
		t.Fatal("expected error for dscp value 64")
	}
}

func TestParseTrafficProtocolRejectsOutOfRange(t *testing.T) {
	data := `{"traffic":{"control":{"interface":{"eth0":{"qdisc":{"type":"htb","class":{"c":{"rate":"10mbit","match":{"protocol":{"value":"256"}}}}}}}}}}`
	_, err := parseTrafficConfig(data)
	if err == nil {
		t.Fatal("expected error for protocol value 256")
	}
}
