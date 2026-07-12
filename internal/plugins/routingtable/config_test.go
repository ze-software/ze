package routingtable

import "testing"

// TestParseConfig_StringValuedDelivery pins the ACTUAL config format the plugin
// config framework delivers: YANG leaf values arrive as JSON strings ("100"),
// not native JSON numbers. The prior parser asserted entry["id"].(float64), so
// a string-valued id was rejected as "missing or invalid" and the routing table
// failed to configure. This test would have caught that.
func TestParseConfig_StringValuedDelivery(t *testing.T) {
	data := `{"routing-table":{"table":{"red":{"id":"100"},"blue":{"id":"50000"}}}}`
	tables, err := parseRoutingTableConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if tables["red"] != 100 {
		t.Errorf("red id = %d, want 100 (string-valued id)", tables["red"])
	}
	if tables["blue"] != 50000 {
		t.Errorf("blue id = %d, want 50000 (string-valued id)", tables["blue"])
	}
}
