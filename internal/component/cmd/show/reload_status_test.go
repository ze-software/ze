package show

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestHandleShowReloadStatusNilServer verifies the handler degrades to an error
// response rather than panicking when no daemon is wired.
//
// VALIDATES: `show reload-status` against a context with no server returns a
// StatusError response, exit 0 for the handler itself.
// PREVENTS: a nil-pointer panic taking down the command dispatcher when the
// command is invoked offline, the same guard handleShowUptime carries.
func TestHandleShowReloadStatusNilServer(t *testing.T) {
	resp, err := handleShowReloadStatus(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
}

// TestHandleShowReloadStatusBeforeReload verifies the pre-reload zero state is
// reported distinguishably.
//
// VALIDATES: AC-2 -- an observer reads the generation BEFORE triggering a
// reload, so the pre-reload read must be well-defined and must not look like a
// completed reload.
// PREVENTS: an observer fencing against a garbage baseline and either passing
// vacuously or hanging.
func TestHandleShowReloadStatusBeforeReload(t *testing.T) {
	srv := &pluginserver.Server{}
	ctx := &pluginserver.CommandContext{Server: srv}

	resp, err := handleShowReloadStatus(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok, "Data should be plugin.Map, got %T", resp.Data)
	assert.Equal(t, uint64(0), data["generation"])
	assert.Equal(t, pluginserver.ReloadOutcomeNone, data["last-outcome"])
}

// TestHandleShowReloadStatusReportsRejectedReload is the AC-1 surface
// assertion: the counter a rejected reload advanced must be READABLE through
// the show command, which is the only way an observer can see it.
//
// VALIDATES: AC-1 -- "a reload (applied OR rejected) advances a
// plugin-QUERYABLE generation counter", and AC-2 -- the observer waits on that
// query. Asserts the JSON contract the .ci observer depends on:
// generation/last-outcome keys.
// PREVENTS: the counter advancing internally while the show surface reports a
// stale or differently-keyed value, which would silently break every observer
// polling it and send the next session hunting a daemon bug that is not there.
func TestHandleShowReloadStatusReportsRejectedReload(t *testing.T) {
	tests := []struct {
		name        string
		applied     bool
		wantOutcome string
	}{
		{name: "applied reload", applied: true, wantOutcome: pluginserver.ReloadOutcomeApplied},
		{name: "rejected reload", applied: false, wantOutcome: pluginserver.ReloadOutcomeFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := &pluginserver.Server{}
			ctx := &pluginserver.CommandContext{Server: srv}

			srv.MarkReloadProcessed(tt.applied)

			resp, err := handleShowReloadStatus(ctx, nil)
			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, plugin.StatusDone, resp.Status)

			data, ok := resp.Data.(plugin.Map)
			require.True(t, ok, "Data should be plugin.Map, got %T", resp.Data)
			assert.Equal(t, uint64(1), data["generation"], "a processed reload must be visible through show")
			assert.Equal(t, tt.wantOutcome, data["last-outcome"])
			assert.NotEmpty(t, data["last-reload-at"], "a processed reload must report when it finished")
		})
	}
}

// TestHandleShowReloadStatusTracksMultipleReloads verifies the show surface
// keeps reporting the live counter, not a first-read snapshot.
//
// VALIDATES: AC-2 -- the observer polls this command repeatedly and waits for
// the value to CHANGE. A cached response would never change.
// PREVENTS: an observer polling forever against a value frozen at its first
// read.
func TestHandleShowReloadStatusTracksMultipleReloads(t *testing.T) {
	srv := &pluginserver.Server{}
	ctx := &pluginserver.CommandContext{Server: srv}

	for i := 1; i <= 3; i++ {
		srv.MarkReloadProcessed(false)

		resp, err := handleShowReloadStatus(ctx, nil)
		require.NoError(t, err)
		data, ok := resp.Data.(plugin.Map)
		require.True(t, ok)
		assert.Equal(t, uint64(i), data["generation"], "generation must track each processed reload")
	}
}
