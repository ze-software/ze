package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/resolve"
	"github.com/ze-software/ze/internal/component/resolve/dns"
)

func withTestResolver(t *testing.T, fn func()) {
	t.Helper()
	old := resolvers
	resolvers = &resolve.Resolvers{
		DNS: dns.NewResolver(dns.ResolverConfig{CacheSize: 100}),
	}
	defer func() {
		resolvers.DNS.Close()
		resolvers = old
	}()
	fn()
}

func clearData(t *testing.T, resp *plugin.Response) plugin.Map {
	t.Helper()
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok, "expected plugin.Map response data")
	return data
}

func TestClearDNSCache_NoResolver(t *testing.T) {
	old := resolvers
	resolvers = nil
	defer func() { resolvers = old }()

	resp, err := handleClearDNSCache(nil, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
}

func TestClearDNSCache_All(t *testing.T) {
	withTestResolver(t, func() {
		resp, err := handleClearDNSCache(nil, nil)
		require.NoError(t, err)
		assert.Equal(t, plugin.StatusDone, resp.Status)
		data := clearData(t, resp)
		assert.Equal(t, "clear-all", data["action"])
	})
}

func TestClearDNSCache_Stats(t *testing.T) {
	withTestResolver(t, func() {
		resp, err := handleClearDNSCacheStats(nil, nil)
		require.NoError(t, err)
		assert.Equal(t, plugin.StatusDone, resp.Status)
		data := clearData(t, resp)
		assert.Equal(t, "reset-stats", data["action"])
	})
}

func TestClearDNSCacheStats_RejectsActionLikeArgs(t *testing.T) {
	withTestResolver(t, func() {
		resp, err := handleClearDNSCacheStats(nil, []string{"record", "example.com", "type", "AAAA"})
		require.NoError(t, err)
		assert.Equal(t, plugin.StatusError, resp.Status)
		assert.Contains(t, resp.Error, "unexpected arguments")
		assert.Nil(t, resp.Data)
	})
}

func TestClearDNSCache_EntryWithType(t *testing.T) {
	withTestResolver(t, func() {
		resp, err := handleClearDNSCacheRecord(nil, []string{"example.com", "type", "AAAA"})
		require.NoError(t, err)
		assert.Equal(t, plugin.StatusDone, resp.Status)
		data := clearData(t, resp)
		assert.Equal(t, "delete-entry", data["action"])
		assert.Equal(t, "example.com", data["name"])
		assert.Equal(t, "AAAA", data["type"])
	})
}

func TestClearDNSCache_EntryNoType(t *testing.T) {
	withTestResolver(t, func() {
		resp, err := handleClearDNSCacheRecord(nil, []string{"example.com"})
		require.NoError(t, err)
		assert.Equal(t, plugin.StatusDone, resp.Status)
		data := clearData(t, resp)
		assert.Equal(t, "delete-entry", data["action"])
		assert.Equal(t, "example.com", data["name"])
	})
}

func TestClearDNSCache_EntryMissingName(t *testing.T) {
	withTestResolver(t, func() {
		resp, err := handleClearDNSCacheRecord(nil, nil)
		require.NoError(t, err)
		assert.Equal(t, plugin.StatusError, resp.Status)
	})
}

func TestClearDNSCache_UnknownType(t *testing.T) {
	withTestResolver(t, func() {
		resp, err := handleClearDNSCacheRecord(nil, []string{"example.com", "type", "BOGUS"})
		require.NoError(t, err)
		assert.Equal(t, plugin.StatusDone, resp.Status)
		data := clearData(t, resp)
		assert.Equal(t, "delete-entry", data["action"])
		assert.Contains(t, data["error"], "unknown type")
	})
}
