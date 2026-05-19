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

func TestDNSCacheEntries_NoProvider(t *testing.T) {
	old := dnsEntriesProvider
	dnsEntriesProvider = nil
	defer func() { dnsEntriesProvider = old }()

	resp, err := handleDNSCache(nil, []string{dnsCacheActionEntries})
	if err != nil {
		t.Fatal(err)
	}
	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatal("expected map response")
	}
	if _, exists := data["status"]; !exists {
		t.Error("expected status field when provider is nil")
	}
}

func TestDNSCacheEntries_WithProvider(t *testing.T) {
	old := dnsEntriesProvider
	dnsEntriesProvider = func() []map[string]any {
		return []map[string]any{
			{"name": "example.com", "type": "A", "records": []string{"1.2.3.4"}, "ttl-seconds": 120},
		}
	}
	defer func() { dnsEntriesProvider = old }()

	resp, err := handleDNSCache(nil, []string{dnsCacheActionEntries})
	if err != nil {
		t.Fatal(err)
	}
	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatal("expected map response")
	}
	if data["count"] != 1 {
		t.Errorf("count = %v, want 1", data["count"])
	}
	entries, ok := data["entries"].([]map[string]any)
	if !ok {
		t.Fatal("expected entries array")
	}
	if entries[0]["name"] != "example.com" {
		t.Errorf("name = %v, want example.com", entries[0]["name"])
	}
}
