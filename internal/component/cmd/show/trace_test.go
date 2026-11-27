// Design: docs/architecture/diagnostics/production-diagnostics.md -- unit tests for show system trace and request system trace snapshot
// Related: trace.go -- trace inspection and snapshot handlers

package show

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/flightrec"
)

// TestHandleSystemTraceSnapshotAndFetch exercises request system trace snapshot,
// show system trace, show system trace id <id>, and operator-error refusals when
// the flight recorder is stopped or an unknown snapshot ID is requested.
//
// VALIDATES: AC-3 -- live flight-recorder capture returns snapshot metadata,
// show system trace lists metadata without raw trace bytes, show system trace
// id <id> returns go-trace-base64 payload, and stopped/unknown-ID calls return
// StatusError refusals.
// PREVENTS: broken YANG handler wiring, raw trace blobs leaking into list
// responses, or silent success on stopped recorder / missing snapshot IDs.
func TestHandleSystemTraceSnapshotAndFetch(t *testing.T) {
	flightrec.ResetForTest()
	t.Cleanup(flightrec.ResetForTest)

	stoppedResp, err := handleRequestSystemTraceSnapshot(nil, commandtest.Args())
	require.NoError(t, err)
	require.NotNil(t, stoppedResp)
	assert.Equal(t, plugin.StatusError, stoppedResp.Status)
	assert.NotEmpty(t, stoppedResp.Error)
	assert.Contains(t, stoppedResp.Error, "fix: ze show system trace")

	require.NoError(t, flightrec.Start())

	snapResp, err := handleRequestSystemTraceSnapshot(nil, commandtest.Args())
	require.NoError(t, err)
	require.NotNil(t, snapResp)
	require.Equal(t, plugin.StatusDone, snapResp.Status, "error=%s", snapResp.Error)

	snapData, ok := snapResp.Data.(plugin.Map)
	require.True(t, ok, "snapshot response data is %T, want plugin.Map", snapResp.Data)
	snapID, ok := snapData["id"].(string)
	require.True(t, ok, "snapshot id is %T, want string", snapData["id"])
	require.NotEmpty(t, snapID)
	assert.Equal(t, "operator-request", snapData["trigger"])
	assert.NotEmpty(t, snapData["captured-at"])
	snapBytes, ok := snapData["bytes"].(int)
	require.True(t, ok, "snapshot bytes is %T, want int", snapData["bytes"])
	assert.Greater(t, snapBytes, 0)

	listResp, err := handleShowSystemTrace(nil, commandtest.Args())
	require.NoError(t, err)
	require.NotNil(t, listResp)
	require.Equal(t, plugin.StatusDone, listResp.Status, "error=%s", listResp.Error)

	listData, ok := listResp.Data.(plugin.Map)
	require.True(t, ok, "list response data is %T, want plugin.Map", listResp.Data)
	assert.Equal(t, true, listData["enabled"])
	snaps, ok := listData["snapshots"].([]map[string]any)
	require.True(t, ok, "snapshots is %T, want []map[string]any", listData["snapshots"])
	found := false
	for _, row := range snaps {
		if row["id"] == snapID {
			found = true
			_, hasData := row["data"]
			assert.False(t, hasData, "list view must not include raw trace bytes")
		}
	}
	assert.True(t, found, "show system trace must list snapshot %q", snapID)

	fetchResp, err := handleShowSystemTrace(nil, commandtest.Args("id", snapID))
	require.NoError(t, err)
	require.NotNil(t, fetchResp)
	require.Equal(t, plugin.StatusDone, fetchResp.Status, "error=%s", fetchResp.Error)

	fetchData, ok := fetchResp.Data.(plugin.Map)
	require.True(t, ok, "fetch response data is %T, want plugin.Map", fetchResp.Data)
	assert.Equal(t, snapID, fetchData["id"])
	assert.Equal(t, "go-trace-base64", fetchData["format"])
	encoded, ok := fetchData["data"].(string)
	require.True(t, ok, "fetch data is %T, want string", fetchData["data"])
	rawTrace, err := base64.StdEncoding.DecodeString(encoded)
	require.NoError(t, err)
	assert.NotEmpty(t, rawTrace)

	missingResp, err := handleShowSystemTrace(nil, commandtest.Args("id", "trace-nonexistent"))
	require.NoError(t, err)
	require.NotNil(t, missingResp)
	assert.Equal(t, plugin.StatusError, missingResp.Status)
	assert.NotEmpty(t, missingResp.Error)
	assert.Contains(t, missingResp.Error, "fix: ze show system trace")
}
