package managed

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/pkg/fleet"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// mockHub simulates a hub that responds to config-fetch RPCs.
func mockHub(t *testing.T, conn net.Conn, configData []byte) {
	t.Helper()
	rc := rpc.NewConn(conn, conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := rc.ReadRequest(ctx)
	if err != nil {
		t.Logf("mockHub: read error: %v", err)
		return
	}

	if req.Method != fleet.VerbConfigFetch {
		t.Logf("mockHub: unexpected method %q", req.Method)
		return
	}

	var fetchReq fleet.ConfigFetchRequest
	if err := json.Unmarshal(req.Params, &fetchReq); err != nil {
		t.Logf("mockHub: unmarshal error: %v", err)
		return
	}

	version := fleet.VersionHash(configData)
	var resp fleet.ConfigFetchResponse
	if fetchReq.Version == version {
		resp = fleet.ConfigFetchResponse{Status: "current"}
	} else {
		resp = fleet.ConfigFetchResponse{
			Version: version,
			Config:  base64.StdEncoding.EncodeToString(configData),
		}
	}

	if err := rc.SendResult(ctx, req.ID, resp); err != nil {
		t.Logf("mockHub: send error: %v", err)
	}
}

// TestClientFetchConfig verifies that the client fetches config from hub.
//
// VALIDATES: Client sends config-fetch, receives and processes response (AC-1).
// PREVENTS: Client unable to fetch config from hub.
func TestClientFetchConfig(t *testing.T) {
	t.Parallel()

	configData := []byte("bgp { peer 10.0.0.1 { peer-as 65001; } }")
	clientEnd, hubEnd := net.Pipe()
	defer clientEnd.Close() //nolint:errcheck // test cleanup
	defer hubEnd.Close()    //nolint:errcheck // test cleanup

	go mockHub(t, hubEnd, configData)

	mc := rpc.NewMuxConn(rpc.NewConn(clientEnd, clientEnd))
	defer mc.Close() //nolint:errcheck // test cleanup

	resp, err := FetchConfig(context.Background(), mc, "")
	require.NoError(t, err)
	assert.Equal(t, fleet.VersionHash(configData), resp.Version)
	assert.NotEmpty(t, resp.Config)
}

// TestClientFetchConfigCurrent verifies that matching version returns "current".
//
// VALIDATES: Matching version gets status=current (AC-13).
// PREVENTS: Unnecessary config transfer.
func TestClientFetchConfigCurrent(t *testing.T) {
	t.Parallel()

	configData := []byte("bgp { peer 10.0.0.1 { peer-as 65001; } }")
	currentVersion := fleet.VersionHash(configData)

	clientEnd, hubEnd := net.Pipe()
	defer clientEnd.Close() //nolint:errcheck // test cleanup
	defer hubEnd.Close()    //nolint:errcheck // test cleanup

	go mockHub(t, hubEnd, configData)

	mc := rpc.NewMuxConn(rpc.NewConn(clientEnd, clientEnd))
	defer mc.Close() //nolint:errcheck // test cleanup

	resp, err := FetchConfig(context.Background(), mc, currentVersion)
	require.NoError(t, err)
	assert.Equal(t, "current", resp.Status)
	assert.Empty(t, resp.Config)
}

// TestClientFetchConfigTimeout verifies that fetch respects context timeout.
//
// VALIDATES: Fetch respects context deadline.
// PREVENTS: Client hanging indefinitely on unresponsive hub.
func TestClientFetchConfigTimeout(t *testing.T) {
	t.Parallel()

	clientEnd, hubEnd := net.Pipe()
	defer clientEnd.Close() //nolint:errcheck // test cleanup
	defer hubEnd.Close()    //nolint:errcheck // test cleanup

	// Hub never responds.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	mc := rpc.NewMuxConn(rpc.NewConn(clientEnd, clientEnd))
	defer mc.Close() //nolint:errcheck // test cleanup

	_, err := FetchConfig(ctx, mc, "")
	require.Error(t, err)
}
