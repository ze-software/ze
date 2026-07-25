package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
)

func dnsCacheMap(t *testing.T, resp *plugin.Response) plugin.Map {
	t.Helper()
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok, "expected plugin.Map response data")
	return data
}

func assertDNSCacheKeys(t *testing.T, data plugin.Map, want ...string) {
	t.Helper()
	got := make([]string, 0, len(data))
	for key := range data {
		got = append(got, key)
	}
	assert.ElementsMatch(t, want, got)
}

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

func TestDNSCacheStats_WithResolver(t *testing.T) {
	withTestResolver(t, func() {
		resp, err := handleDNSCacheStats(nil, nil)
		require.NoError(t, err)
		assert.Equal(t, plugin.StatusDone, resp.Status)
		data := dnsCacheMap(t, resp)
		assertDNSCacheKeys(t, data,
			"capacity",
			"entries",
			"evictions",
			"expired",
			"hit-rate",
			"hits",
			"miss-rate",
			"misses",
		)
	})
}

func TestDNSCacheStats_RejectsActionLikeArgs(t *testing.T) {
	withTestResolver(t, func() {
		resp, err := handleDNSCacheStats(nil, []string{"record", "example.com"})
		require.NoError(t, err)
		assert.Equal(t, plugin.StatusError, resp.Status)
		assert.Contains(t, resp.Error, "unexpected arguments")
		assert.Nil(t, resp.Data)
	})
}

func TestDNSCacheEntries_NoResolver(t *testing.T) {
	old := resolvers
	resolvers = nil
	defer func() { resolvers = old }()

	resp, err := handleDNSCacheList(nil, nil)
	require.NoError(t, err)
	data := dnsCacheMap(t, resp)
	assert.Equal(t, plugin.Map{"status": "DNS cache not available"}, data)
}

func TestDNSCacheList_WithResolver(t *testing.T) {
	withTestResolver(t, func() {
		resp, err := handleDNSCacheList(nil, nil)
		require.NoError(t, err)
		assert.Equal(t, plugin.StatusDone, resp.Status)
		data := dnsCacheMap(t, resp)
		assertDNSCacheKeys(t, data, "count", "entries")
		assert.Equal(t, 0, data["count"])
		entries, ok := data["entries"].([]map[string]any)
		require.True(t, ok, "entries should be a list of DNS cache entry maps")
		assert.Empty(t, entries)
	})
}

func TestDNSCacheList_RejectsActionLikeArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "stats keyword", args: []string{"stats"}},
		{name: "record keyword with hostname", args: []string{"record", "example.com"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withTestResolver(t, func() {
				resp, err := handleDNSCacheList(nil, tt.args)
				require.NoError(t, err)
				assert.Equal(t, plugin.StatusError, resp.Status)
				assert.Contains(t, resp.Error, "unexpected arguments")
				assert.Nil(t, resp.Data)
			})
		})
	}
}

func TestDNSCacheRecords_FilterByName(t *testing.T) {
	withTestResolver(t, func() {
		resp, err := handleDNSCacheRecord(nil, []string{"example.com"})
		require.NoError(t, err)
		assert.Equal(t, plugin.StatusDone, resp.Status)
		data := dnsCacheMap(t, resp)
		assertDNSCacheKeys(t, data, "count", "entries", "filter")
		assert.Equal(t, "example.com", data["filter"])
	})
}

func TestDNSCacheRecords_MissingName(t *testing.T) {
	resp, err := handleDNSCacheRecord(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "error" {
		t.Errorf("status = %v, want error for missing name", resp.Status)
	}
}
