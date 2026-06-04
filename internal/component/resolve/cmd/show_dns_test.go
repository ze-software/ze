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

func TestDNSCacheEntries_NoResolver(t *testing.T) {
	old := resolvers
	resolvers = nil
	defer func() { resolvers = old }()

	resp, err := handleDNSCache(nil, []string{dnsCacheActionList})
	if err != nil {
		t.Fatal(err)
	}
	data, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatal("expected map response")
	}
	if _, exists := data["status"]; !exists {
		t.Error("expected status field when resolver is nil")
	}
}

func TestDNSCacheEntries_WithResolver(t *testing.T) {
	withTestResolver(t, func() {
		resp, err := handleDNSCache(nil, []string{dnsCacheActionList})
		if err != nil {
			t.Fatal(err)
		}
		data, ok := resp.Data.(plugin.Map)
		if !ok {
			t.Fatal("expected map response")
		}
		if _, exists := data["count"]; !exists {
			t.Error("expected count field")
		}
		if _, exists := data["entries"]; !exists {
			t.Error("expected entries field")
		}
	})
}

func TestDNSCacheRecords_FilterByName(t *testing.T) {
	withTestResolver(t, func() {
		resp, err := handleDNSCache(nil, []string{dnsCacheActionRecord, "example.com"})
		if err != nil {
			t.Fatal(err)
		}
		data, ok := resp.Data.(plugin.Map)
		if !ok {
			t.Fatal("expected map response")
		}
		if data["filter"] != "example.com" {
			t.Errorf("filter = %v, want example.com", data["filter"])
		}
	})
}

func TestDNSCacheRecords_MissingName(t *testing.T) {
	resp, err := handleDNSCache(nil, []string{dnsCacheActionRecord})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "error" {
		t.Errorf("status = %v, want error for missing name", resp.Status)
	}
}
