package cmd

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

func TestDNSLookup_Wiring(t *testing.T) {
	resp, err := handleDNSLookup(nil, []string{"localhost"})
	if err != nil {
		t.Fatal(err)
	}
	data, ok := resp.Data.(plugin.Map)
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

	resp, err := handleDNSCache(nil, []string{dnsCacheActionList})
	if err != nil {
		t.Fatal(err)
	}
	data, ok := resp.Data.(plugin.Map)
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

	resp, err := handleDNSCache(nil, []string{dnsCacheActionList})
	if err != nil {
		t.Fatal(err)
	}
	data, ok := resp.Data.(plugin.Map)
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

func TestDNSCacheRecords_FilterByName(t *testing.T) {
	old := dnsEntriesProvider
	dnsEntriesProvider = func() []map[string]any {
		return []map[string]any{
			{"name": "example.com", "type": "A", "records": []string{"1.2.3.4"}, "ttl-seconds": 120},
			{"name": "other.com", "type": "A", "records": []string{"5.6.7.8"}, "ttl-seconds": 60},
			{"name": "example.com", "type": "AAAA", "records": []string{"::1"}, "ttl-seconds": 300},
		}
	}
	defer func() { dnsEntriesProvider = old }()

	resp, err := handleDNSCache(nil, []string{dnsCacheActionRecord, "example.com"})
	if err != nil {
		t.Fatal(err)
	}
	data, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatal("expected map response")
	}
	if data["count"] != 2 {
		t.Errorf("count = %v, want 2 (both example.com entries)", data["count"])
	}
	if data["filter"] != "example.com" {
		t.Errorf("filter = %v, want example.com", data["filter"])
	}
}

func TestDNSCacheRecords_MissingName(t *testing.T) {
	old := dnsEntriesProvider
	dnsEntriesProvider = func() []map[string]any { return nil }
	defer func() { dnsEntriesProvider = old }()

	resp, err := handleDNSCache(nil, []string{dnsCacheActionRecord})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "error" {
		t.Errorf("status = %v, want error for missing name", resp.Status)
	}
}
