package managed

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestManagedSourceAddress verifies runConnection binds the configured
// source-address as the TCP local source before the TLS upgrade: a source not
// assigned locally fails the bind rather than being ignored.
//
// VALIDATES: AC-13/AC-14 -- managed hub client binds source-address on TCP.
// PREVENTS: SourceAddress being carried on ClientConfig but never applied.
func TestManagedSourceAddress(t *testing.T) {
	cfg := &ClientConfig{
		Name:          "test",
		Server:        "192.0.2.2:12700", // RFC 5737 literal IP:port, no DNS
		Token:         "0123456789abcdef0123456789abcdef",
		SourceAddress: "192.0.2.9", // not a local address -> bind fails
	}
	backoff := newBackoff(time.Millisecond, time.Second, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := runConnection(ctx, cfg, backoff)
	if err == nil {
		t.Fatal("expected connect error for non-local source-address, got nil")
	}
	if !strings.Contains(err.Error(), "connect") {
		t.Errorf("error = %v, want a connect/bind failure", err)
	}
}

// TestManagedTLSServerName verifies the TLS ServerName is derived from the hub
// address. net.Dialer + tls.Client() (unlike tls.Dialer) does not infer it, so
// without this the certificate hostname check would be skipped (see
// serverNameFromAddr) -- a silent downgrade of certificate verification.
//
// VALIDATES: preserves hostname verification after the tls.Dialer->tls.Client split.
// PREVENTS: MITM via any CA-signed cert when TLSInsecure is false (the default).
func TestManagedTLSServerName(t *testing.T) {
	tests := []struct {
		server string
		want   string
	}{
		{"hub.example.com:12700", "hub.example.com"},
		{"127.0.0.1:12700", "127.0.0.1"},
		{"[2001:db8::1]:443", "2001:db8::1"},
		{"hostonly", "hostonly"}, // no port separator -> used verbatim
	}
	for _, tt := range tests {
		if got := serverNameFromAddr(tt.server); got != tt.want {
			t.Errorf("serverNameFromAddr(%q) = %q, want %q", tt.server, got, tt.want)
		}
	}
}

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

func mockHubFetchAndAck(t *testing.T, conn net.Conn, configData []byte, ackCh chan<- fleet.ConfigAck) {
	t.Helper()
	rc := rpc.NewConn(conn, conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	fetchReq, err := rc.ReadRequest(ctx)
	if err != nil {
		t.Logf("mockHubFetchAndAck: read fetch: %v", err)
		return
	}
	if fetchReq.Method != fleet.VerbConfigFetch {
		t.Logf("mockHubFetchAndAck: unexpected fetch method %q", fetchReq.Method)
		return
	}
	resp := fleet.ConfigFetchResponse{
		Version: fleet.VersionHash(configData),
		Config:  base64.StdEncoding.EncodeToString(configData),
	}
	if err := rc.SendResult(ctx, fetchReq.ID, resp); err != nil {
		t.Logf("mockHubFetchAndAck: send fetch response: %v", err)
		return
	}

	ackReq, err := rc.ReadRequest(ctx)
	if err != nil {
		t.Logf("mockHubFetchAndAck: read ack: %v", err)
		return
	}
	if ackReq.Method != fleet.VerbConfigAck {
		t.Logf("mockHubFetchAndAck: unexpected ack method %q", ackReq.Method)
		return
	}
	var ack fleet.ConfigAck
	if err := json.Unmarshal(ackReq.Params, &ack); err != nil {
		t.Logf("mockHubFetchAndAck: unmarshal ack: %v", err)
		return
	}
	ackCh <- ack
	_ = rc.SendResult(ctx, ackReq.ID, nil)
}

// TestManagedOnCommitNil verifies config pushes are temporarily rejected until the hub wires OnCommit.
//
// VALIDATES: AC-11 nil OnCommit returns rejection and does not advance version.
// PREVENTS: managed client ACKing config before transactional commit wiring exists.
func TestManagedOnCommitNil(t *testing.T) {
	configData := []byte("bgp { router-id 1.1.1.1; }")
	clientEnd, hubEnd := net.Pipe()
	defer clientEnd.Close() //nolint:errcheck // test cleanup
	defer hubEnd.Close()    //nolint:errcheck // test cleanup
	ackCh := make(chan fleet.ConfigAck, 1)
	go mockHubFetchAndAck(t, hubEnd, configData, ackCh)

	mc := rpc.NewMuxConn(rpc.NewConn(clientEnd, clientEnd))
	defer mc.Close() //nolint:errcheck // test cleanup
	cfg := &ClientConfig{Handler: &Handler{Validate: func([]byte) error { return nil }}}

	require.NoError(t, fetchAndProcess(context.Background(), mc, cfg))
	ack := <-ackCh
	assert.False(t, ack.OK)
	assert.Contains(t, ack.Error, "commit not ready")
	assert.Empty(t, cfg.Version)
}

// TestManagedACKDeferredUntilCommit verifies ACK is sent after OnCommit succeeds.
//
// VALIDATES: AC-9 managed push ACK OK is sent only after OnCommit returns nil.
// PREVENTS: hub seeing success before runtime transaction completes.
func TestManagedACKDeferredUntilCommit(t *testing.T) {
	configData := []byte("bgp { router-id 1.1.1.1; }")
	clientEnd, hubEnd := net.Pipe()
	defer clientEnd.Close() //nolint:errcheck // test cleanup
	defer hubEnd.Close()    //nolint:errcheck // test cleanup
	ackCh := make(chan fleet.ConfigAck, 1)
	go mockHubFetchAndAck(t, hubEnd, configData, ackCh)

	mc := rpc.NewMuxConn(rpc.NewConn(clientEnd, clientEnd))
	defer mc.Close() //nolint:errcheck // test cleanup
	committed := false
	cfg := &ClientConfig{
		Handler: &Handler{Validate: func([]byte) error { return nil }},
		OnCommit: func(data []byte) error {
			assert.Equal(t, configData, data)
			committed = true
			return nil
		},
	}

	require.NoError(t, fetchAndProcess(context.Background(), mc, cfg))
	ack := <-ackCh
	assert.True(t, committed, "OnCommit must run before ACK is sent")
	assert.True(t, ack.OK)
	assert.Equal(t, fleet.VersionHash(configData), cfg.Version)
}

// TestManagedACKErrorOnCommitFail verifies OnCommit failure is reported to the hub.
//
// VALIDATES: AC-10 managed push returns ACK error when runtime commit rejects.
// PREVENTS: failed runtime transaction being reported as accepted.
func TestManagedACKErrorOnCommitFail(t *testing.T) {
	configData := []byte("bgp { router-id 1.1.1.1; }")
	clientEnd, hubEnd := net.Pipe()
	defer clientEnd.Close() //nolint:errcheck // test cleanup
	defer hubEnd.Close()    //nolint:errcheck // test cleanup
	ackCh := make(chan fleet.ConfigAck, 1)
	go mockHubFetchAndAck(t, hubEnd, configData, ackCh)

	mc := rpc.NewMuxConn(rpc.NewConn(clientEnd, clientEnd))
	defer mc.Close() //nolint:errcheck // test cleanup
	cfg := &ClientConfig{
		Handler:  &Handler{Validate: func([]byte) error { return nil }},
		OnCommit: func([]byte) error { return fmt.Errorf("verify rejected") },
	}

	require.NoError(t, fetchAndProcess(context.Background(), mc, cfg))
	ack := <-ackCh
	assert.False(t, ack.OK)
	assert.Contains(t, ack.Error, "verify rejected")
	assert.Empty(t, cfg.Version)
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

// TestRunManagedClientStopsWhenUnmanaged verifies that RunManagedClient exits
// when CheckManaged returns false.
//
// VALIDATES: AC-17 -- meta/managed=false severs hub connection.
// PREVENTS: Managed client running indefinitely after managed flag disabled.
func TestRunManagedClientStopsWhenUnmanaged(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})

	cfg := ClientConfig{
		Name:   "edge-01",
		Server: "127.0.0.1:19999", // unreachable -- doesn't matter, check happens first
		Token:  "token",
		Handler: &Handler{
			Cache: func(data []byte) error { return nil },
		},
		CheckManaged: func() bool {
			return false // immediately unmanaged
		},
	}

	go func() {
		RunManagedClient(context.Background(), cfg)
		close(done)
	}()

	select {
	case <-done:
		// RunManagedClient returned because CheckManaged is false.
	case <-time.After(2 * time.Second):
		t.Fatal("RunManagedClient did not stop when CheckManaged returned false")
	}
}

// TestReadLine verifies byte-by-byte line reading for auth responses.
//
// VALIDATES: readLine handles normal, CRLF, oversize, and empty lines.
// PREVENTS: Auth failures from partial reads or CRLF line endings.
func TestReadLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		maxSize int
		want    string
		wantErr bool
	}{
		{name: "normal LF", input: "#0 ok\n", maxSize: 512, want: "#0 ok"},
		{name: "CRLF stripped", input: "#0 ok\r\n", maxSize: 512, want: "#0 ok"},
		{name: "empty line", input: "\n", maxSize: 512, want: ""},
		{name: "oversize", input: "aaaaaaaaaa\n", maxSize: 5, wantErr: true},
		{name: "exact at limit", input: "abcde\n", maxSize: 5, wantErr: true},
		{name: "one under limit", input: "abcd\n", maxSize: 5, want: "abcd"},
		{name: "error response", input: "#0 error {\"code\":\"auth\"}\n", maxSize: 512, want: "#0 error {\"code\":\"auth\"}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			clientEnd, serverEnd := net.Pipe()
			defer clientEnd.Close() //nolint:errcheck // test cleanup
			defer serverEnd.Close() //nolint:errcheck // test cleanup

			go func() {
				serverEnd.Write([]byte(tt.input)) //nolint:errcheck // test helper
			}()

			got, err := readLine(clientEnd, tt.maxSize)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(got))
		})
	}
}
