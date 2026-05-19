package clear

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

func TestClearDNSCache_NoProvider(t *testing.T) {
	dnsCacheClearProvider = nil
	resp, err := handleClearDNSCache(nil, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
}

func TestClearDNSCache_All(t *testing.T) {
	var gotAction string
	dnsCacheClearProvider = func(action, name, typeName string) map[string]any {
		gotAction = action
		return map[string]any{"action": "clear-all"}
	}
	defer func() { dnsCacheClearProvider = nil }()

	resp, err := handleClearDNSCache(nil, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.Equal(t, "all", gotAction)
}

func TestClearDNSCache_Stats(t *testing.T) {
	var gotAction string
	dnsCacheClearProvider = func(action, name, typeName string) map[string]any {
		gotAction = action
		return map[string]any{"action": "reset-stats"}
	}
	defer func() { dnsCacheClearProvider = nil }()

	resp, err := handleClearDNSCache(nil, []string{"stats"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.Equal(t, "stats", gotAction)
}

func TestClearDNSCache_EntryWithType(t *testing.T) {
	var gotAction, gotName, gotType string
	dnsCacheClearProvider = func(action, name, typeName string) map[string]any {
		gotAction = action
		gotName = name
		gotType = typeName
		return map[string]any{"action": "delete-entry"}
	}
	defer func() { dnsCacheClearProvider = nil }()

	resp, err := handleClearDNSCache(nil, []string{"record", "example.com", "type", "AAAA"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.Equal(t, "record", gotAction)
	assert.Equal(t, "example.com", gotName)
	assert.Equal(t, "AAAA", gotType)
}

func TestClearDNSCache_EntryNoType(t *testing.T) {
	var gotAction, gotName, gotType string
	dnsCacheClearProvider = func(action, name, typeName string) map[string]any {
		gotAction = action
		gotName = name
		gotType = typeName
		return map[string]any{"action": "delete-entry"}
	}
	defer func() { dnsCacheClearProvider = nil }()

	resp, err := handleClearDNSCache(nil, []string{"record", "example.com"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.Equal(t, "record", gotAction)
	assert.Equal(t, "example.com", gotName)
	assert.Equal(t, "", gotType, "no type specified means delete all types")
}

func TestClearDNSCache_EntryMissingName(t *testing.T) {
	dnsCacheClearProvider = func(action, name, typeName string) map[string]any {
		return nil
	}
	defer func() { dnsCacheClearProvider = nil }()

	resp, err := handleClearDNSCache(nil, []string{"record"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
}
