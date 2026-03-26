package peer

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// VALIDATES: HandleBgpWarnings returns stale-data warnings for peers with old PrefixUpdated.
// PREVENTS: stale peers silently ignored by warnings query.
func TestHandleBgpWarnings_Stale(t *testing.T) {
	staleDate := time.Now().Add(-200 * 24 * time.Hour).Format(time.DateOnly)
	mr := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("10.0.0.1"), PeerAS: 65001, PrefixUpdated: staleDate},
			{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 65002, PrefixUpdated: time.Now().Format(time.DateOnly)},
		},
	}

	ctx := newTestContext(mr)
	resp, err := HandleBgpWarnings(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok, "expected map[string]any data")
	assert.Equal(t, 1, data["count"])
	warnings, ok := data["warnings"].([]map[string]any)
	require.True(t, ok, "expected []map[string]any warnings")
	assert.Equal(t, "stale-data", warnings[0]["type"])
	assert.Equal(t, "10.0.0.1", warnings[0]["address"])
}

// VALIDATES: HandleBgpWarnings returns threshold-exceeded for peers with active PrefixWarnings.
// PREVENTS: runtime prefix warnings missing from warnings query.
func TestHandleBgpWarnings_ThresholdExceeded(t *testing.T) {
	mr := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("10.0.0.1"), PeerAS: 65001, PrefixWarnings: []string{"ipv4/unicast"}},
		},
	}

	ctx := newTestContext(mr)
	resp, err := HandleBgpWarnings(ctx, nil)
	require.NoError(t, err)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok, "expected map[string]any data")
	assert.Equal(t, 1, data["count"])
	warnings, ok := data["warnings"].([]map[string]any)
	require.True(t, ok, "expected []map[string]any warnings")
	assert.Equal(t, "threshold-exceeded", warnings[0]["type"])
	assert.Equal(t, "ipv4/unicast", warnings[0]["family"])
}

// VALIDATES: HandleBgpWarnings returns empty list when no warnings exist.
// PREVENTS: spurious warnings shown on healthy system.
func TestHandleBgpWarnings_None(t *testing.T) {
	mr := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("10.0.0.1"), PeerAS: 65001, PrefixUpdated: time.Now().Format(time.DateOnly)},
		},
	}

	ctx := newTestContext(mr)
	resp, err := HandleBgpWarnings(ctx, nil)
	require.NoError(t, err)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok, "expected map[string]any data")
	assert.Equal(t, 0, data["count"])
}

// VALIDATES: HandleBgpWarnings uses peer name as label when available.
// PREVENTS: unnamed peers showing IP-only labels when name is configured.
func TestHandleBgpWarnings_PeerName(t *testing.T) {
	staleDate := time.Now().Add(-200 * 24 * time.Hour).Format(time.DateOnly)
	mr := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("10.0.0.1"), PeerAS: 65001, Name: "core-router", PrefixUpdated: staleDate},
		},
	}

	ctx := newTestContext(mr)
	resp, err := HandleBgpWarnings(ctx, nil)
	require.NoError(t, err)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok, "expected map[string]any data")
	warnings, ok := data["warnings"].([]map[string]any)
	require.True(t, ok, "expected []map[string]any warnings")
	assert.Equal(t, "core-router", warnings[0]["peer"])
	assert.Equal(t, "10.0.0.1", warnings[0]["address"])
}

// VALIDATES: isPrefixStale matches reactor.IsPrefixDataStale behavior.
// PREVENTS: divergence between login warning and warnings command staleness detection.
func TestIsPrefixStale(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name    string
		updated string
		want    bool
	}{
		{"empty", "", false},
		{"recent", now.Add(-30 * 24 * time.Hour).Format(time.DateOnly), false},
		{"stale", now.Add(-200 * 24 * time.Hour).Format(time.DateOnly), true},
		{"179 days", now.Add(-179 * 24 * time.Hour).Format(time.DateOnly), false},
		{"181 days", now.Add(-181 * 24 * time.Hour).Format(time.DateOnly), true},
		{"invalid date", "not-a-date", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isPrefixStale(tt.updated, now))
		})
	}
}
