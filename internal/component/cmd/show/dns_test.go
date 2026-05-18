package show

import "testing"

func TestDNSLookup_Wiring(t *testing.T) {
	resp, err := handleDNSLookup(nil, []string{"localhost"})
	if err != nil {
		t.Fatal(err)
	}
	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatal("expected map response")
	}
	if _, exists := data["name"]; !exists {
		t.Error("missing name field")
	}
	if _, exists := data["type"]; !exists {
		t.Error("missing type field")
	}
}

func TestDNSCache_Wiring(t *testing.T) {
	resp, err := handleDNSCache(nil, []string{dnsCacheActionStats})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Data == nil {
		t.Error("expected non-nil data")
	}
}
