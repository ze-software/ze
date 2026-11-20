package managed

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
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

// TestManagedCommitOverNil verifies config pushes are temporarily rejected until the hub wires CommitOver.
//
// VALIDATES: AC-11 nil CommitOver returns rejection and leaves the device's
// active configuration and baseline as they were.
// PREVENTS: managed client ACKing config before transactional commit wiring exists.
func TestManagedCommitOverNil(t *testing.T) {
	configData := []byte("bgp { router-id 1.1.1.1; }")
	clientEnd, hubEnd := net.Pipe()
	defer clientEnd.Close() //nolint:errcheck // test cleanup
	defer hubEnd.Close()    //nolint:errcheck // test cleanup
	ackCh := make(chan fleet.ConfigAck, 1)
	go mockHubFetchAndAck(t, hubEnd, configData, ackCh)

	mc := rpc.NewMuxConn(rpc.NewConn(clientEnd, clientEnd))
	defer mc.Close() //nolint:errcheck // test cleanup
	access := appliedDevice(t)
	cfg := &ClientConfig{Handler: &Handler{Validate: func([]byte) error { return nil }}, Baseline: access}

	require.NoError(t, fetchAndProcess(context.Background(), mc, cfg))
	ack := <-ackCh
	assert.False(t, ack.OK)
	assert.Contains(t, ack.Error, "commit not ready")
	requireDeviceUnchanged(t, access)
}

// TestManagedACKDeferredUntilCommit verifies ACK is sent after CommitOver succeeds.
//
// VALIDATES: AC-9 managed push ACK OK is sent only after CommitOver returns
// nil, and names the version of the configuration committed.
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
	access := appliedDevice(t)
	cfg := &ClientConfig{
		Handler: &Handler{Validate: func([]byte) error { return nil }},
		CommitOver: stagedOver(access, func(data []byte) error {
			assert.Equal(t, configData, data)
			committed = true
			return nil
		}),
		Baseline: access,
	}

	require.NoError(t, fetchAndProcess(context.Background(), mc, cfg))
	ack := <-ackCh
	assert.True(t, committed, "CommitOver must run before ACK is sent")
	assert.True(t, ack.OK)
	assert.Equal(t, fleet.VersionHash(configData), ack.Version)
}

// TestManagedACKErrorOnCommitFail verifies CommitOver failure is reported to the hub.
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
	access := appliedDevice(t)
	cfg := &ClientConfig{
		Handler:    &Handler{Validate: func([]byte) error { return nil }},
		CommitOver: stagedOver(access, func([]byte) error { return fmt.Errorf("verify rejected") }),
		Baseline:   access,
	}

	require.NoError(t, fetchAndProcess(context.Background(), mc, cfg))
	ack := <-ackCh
	assert.False(t, ack.OK)
	assert.Contains(t, ack.Error, "verify rejected")
	requireDeviceUnchanged(t, access)
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
		Baseline: NewBaselineAccess(holdDevice(t, holdEarlierConfig, holdEarlierConfig)),
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

// TestRunManagedClientRefusesNoBaseline proves a client built without the
// device's baseline access never runs: it could compare nothing, so every
// push would overwrite the device's configuration unchecked.
//
// VALIDATES: D-3 (OD-5). RunManagedClient returns at once with a context
// that is never canceled, dials nothing, never consults CheckManaged, and
// logs one Error naming the missing baseline access.
// PREVENTS: a new caller that leaves Baseline nil and so brings back the
// unguarded push this spec deleted.
//
// MUTATION: drop the entry check, and the client dials its hub and never
// returns.
func TestRunManagedClientRefusesNoBaseline(t *testing.T) {
	// Not parallel: it replaces the package's logger for the length of its run.
	lines := captureLog(t)
	var lc net.ListenConfig
	listener, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	hub, isTCP := listener.(*net.TCPListener)
	require.True(t, isTCP, "a tcp listener")
	t.Cleanup(func() { hub.Close() }) //nolint:errcheck // test cleanup
	var consulted atomic.Bool
	done := make(chan struct{})
	cfg := ClientConfig{
		Name: "edge-01", Server: hub.Addr().String(), Token: testHubToken, TLSInsecure: true,
		Handler: &Handler{Validate: func([]byte) error { return nil }},
		CheckManaged: func() bool {
			consulted.Store(true)
			return true
		},
	}

	go func() {
		defer close(done)
		RunManagedClient(context.Background(), cfg)
	}()

	select {
	case <-done:
	case <-time.After(holdWait):
		t.Fatal("RunManagedClient ran a client that has no baseline access")
	}
	assert.False(t, consulted.Load(), "the client went on to its connection loop")
	require.NoError(t, hub.SetDeadline(time.Now().Add(100*time.Millisecond)))
	if conn, acceptErr := hub.Accept(); acceptErr == nil {
		conn.Close() //nolint:errcheck // test cleanup
		t.Fatal("the client dialed its hub")
	}
	assert.Equal(t, 1, strings.Count(lines.String(), "level=ERROR"), "log: %s", lines.String())
	assert.Contains(t, lines.String(), "baseline")
	assert.Contains(t, lines.String(), "code="+diagnostic.CodeDoctorInternalDefect,
		"the refusal is not logged as the defect it is")
}

// capturedLog is the text a captureLog logger wrote. Safe for concurrent
// use: a client goroutine may log while the test reads.
type capturedLog struct {
	mu    sync.Mutex
	lines bytes.Buffer
}

// Write appends one handler record.
func (c *capturedLog) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lines.Write(p)
}

// String answers every line written so far.
func (c *capturedLog) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lines.String()
}

// captureLog replaces the package's logger with one that writes every level,
// as text, into the log it answers, until the test ends. A test that calls
// it MUST NOT be parallel: the logger is package state.
func captureLog(t *testing.T) *capturedLog {
	t.Helper()
	lines := &capturedLog{}
	captured := slog.New(slog.NewTextHandler(lines, &slog.HandlerOptions{Level: slog.LevelDebug}))
	previous := logger
	logger = func() *slog.Logger { return captured }
	t.Cleanup(func() { logger = previous })
	return lines
}

// TestRunManagedClientRaisesNoBaselineDefect proves the refusal of a client
// built without the device's baseline access reaches `ze doctor`, not only
// the log: only a defect in ze builds such a client, and the
// doctor-internal-defect code promises an error event on the report bus.
//
// VALIDATES: D-3 (OD-5), the report-bus half. After RunManagedClient returns,
// the bus holds an error from source managed, code doctor-internal-defect,
// subject the client, with ze show errors as its fix.
// PREVENTS: a defect only a log reader finds, so the recent-events doctor
// check never shows it.
//
// MUTATION: drop the report.RaiseErrorFix call in RunManagedClient, and no
// such event is on the bus.
func TestRunManagedClientRaisesNoBaselineDefect(t *testing.T) {
	// Not parallel: it replaces the package's logger and resets the shared
	// report bus for the length of its run.
	captureLog(t)
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	const name = "edge-no-baseline"

	RunManagedClient(context.Background(), ClientConfig{Name: name})

	var raised []report.Issue
	for _, issue := range report.Errors(0) {
		if issue.Source == doctorComponent && issue.Subject == "hub client "+name {
			raised = append(raised, issue)
		}
	}
	require.Len(t, raised, 1, "the refusal raised no single error event on the report bus")
	assert.Equal(t, diagnostic.CodeDoctorInternalDefect, raised[0].Code)
	assert.Equal(t, "ze show errors", raised[0].Fix)
	assert.Contains(t, raised[0].Message, "baseline")
}

// TestConfigChangedLocalCommitTwiceLogsDecision proves a config-changed whose
// fetch a local commit ended on both of its fetches is logged as the
// device's decision, not as a hub it could not reach: the hub answered both
// fetches. The setup is TestPushFetchesAgainOnlyOnce's, driven through
// handleConfigChangedRequest.
//
// VALIDATES: the errActiveChangedTwice branch of handleConfigChangedRequest.
// It answers errActiveChangedTwice, so the loop ends the connection and the
// reconnect's fetch decides again; the log carries one Info line saying the
// device keeps its local edit, and no Warn line, so nothing is logged under
// doctor-hub-unreachable.
// PREVENTS: a local-commit race reported to the operator as a hub outage,
// whose fix (ze doctor) then finds a hub that answers.
//
// MUTATION: drop the errActiveChangedTwice branch in
// handleConfigChangedRequest, and the race is logged at Warn under
// doctor-hub-unreachable.
func TestConfigChangedLocalCommitTwiceLogsDecision(t *testing.T) {
	// Not parallel: it replaces the package's logger for the length of its run.
	lines := captureLog(t)
	store := holdDevice(t, holdEarlierConfig, holdEarlierConfig)
	cfg, _, applied := holdClient("192.0.2.1:1791", NewBaselineAccess(store))
	stage := cfg.CommitOver
	cfg.CommitOver = func(data []byte, check func(active []byte) error) error {
		err := stage(data, check)
		if errors.Is(err, errActiveChanged) {
			// A second local commit restores the baseline's content, so the
			// fetch made again would apply and its stage refuses again.
			require.NoError(t, store.WriteFile(fleetStatusConfigName, []byte(holdEarlierConfig), 0))
		}
		return err
	}
	written := make(chan error, 4) // Room past two, so a third fetch never blocks the hub.
	var fetches int
	pushing := hubPushing(func() {
		fetches++
		written <- store.WriteFile(fleetStatusConfigName, fmt.Appendf(nil, "device { local edit %d; }\n", fetches), 0)
	})
	mc, _ := heldPipe(t, func(req *rpc.Request) any {
		if req.Method == rpc.StatusOK {
			return heldAnswer{silent: true} // The device's answer to the config-changed.
		}
		return pushing(req)
	})
	req := &rpc.Request{ID: 900, Method: fleet.VerbConfigChanged, Params: json.RawMessage(`{"version":"changed"}`)}

	require.ErrorIs(t, handleConfigChangedRequest(context.Background(), mc, req, &cfg), errActiveChangedTwice,
		"the race did not end the connection")

	for range 2 {
		select {
		case err := <-written:
			require.NoError(t, err, "the hub could not land its local commit")
		default:
			t.Fatal("the device made fewer than two fetches, so the race was not reached")
		}
	}
	assert.Empty(t, applied, "the push committed the hub's configuration over a local commit")
	assert.Zero(t, strings.Count(lines.String(), "level=WARN"), "log: %s", lines.String())
	var decision []string
	for line := range strings.SplitSeq(lines.String(), "\n") {
		if strings.Contains(line, "changed during both fetches") && strings.Contains(line, "keeps its local edit") {
			decision = append(decision, line)
		}
	}
	require.Len(t, decision, 1, "log: %s", lines.String())
	assert.Contains(t, decision[0], "level=INFO")
}

// TestFetchMadeAgainReportsHoldWhenDiverged is the holding half of the hold
// TestPushFetchesAgainOnlyOnce reports: the same two refused pushes, but the
// stage wrapper restores the baseline after the first refusal only, so the
// second local commit stays active and the device differs from its baseline.
//
// VALIDATES: refetchOverLocalCommit on the fetch made again reports the hold
// from the store it reads again: here holding, once.
// PREVENTS: a device that runs a local edit its hub never took and reports
// not holding.
//
// MUTATION: report not holding unconditionally in refetchOverLocalCommit on
// refetchNone, and the hold reads false.
func TestFetchMadeAgainReportsHoldWhenDiverged(t *testing.T) {
	store := holdDevice(t, holdEarlierConfig, holdEarlierConfig)
	cfg, _, applied := holdClient("192.0.2.1:1791", NewBaselineAccess(store))
	var holding []bool
	cfg.OnHold = func(held bool) { holding = append(holding, held) }
	stage := cfg.CommitOver
	var refusals int
	cfg.CommitOver = func(data []byte, check func(active []byte) error) error {
		err := stage(data, check)
		if errors.Is(err, errActiveChanged) {
			refusals++
			if refusals == 1 {
				// Back on the baseline, so the fetch made again would apply.
				require.NoError(t, store.WriteFile(fleetStatusConfigName, []byte(holdEarlierConfig), 0))
			}
		}
		return err
	}
	written := make(chan error, 4) // Room past two, so a third fetch never blocks the hub.
	var fetches int
	mc, _ := heldPipe(t, hubPushing(func() {
		fetches++
		written <- store.WriteFile(fleetStatusConfigName, fmt.Appendf(nil, "device { local edit %d; }\n", fetches), 0)
	}))

	err := fetchAndProcess(context.Background(), mc, &cfg)

	for range 2 {
		select {
		case writeErr := <-written:
			require.NoError(t, writeErr, "the hub could not land a local commit")
		default:
			t.Fatal("the hub landed fewer than two local commits")
		}
	}
	require.ErrorIs(t, err, errActiveChangedTwice)
	assert.Equal(t, 2, refusals, "the stage did not refuse both pushes")
	assert.Empty(t, applied, "the push committed the hub's configuration over a local commit")
	assert.Equal(t, []bool{true}, holding, "a device running a local edit did not report holding")
}

// TestLocalCommitTwiceReconnectsQuietly proves a connect fetch that a local
// commit ended on both of its fetches ends the connection quietly: the device
// logs its decision once, at Info, and RunManagedClient dials again with no
// "connection lost" Warn. The reconnect's fetch then decides over what the
// device runs: here its baseline, so the hub's answer that it is current
// leaves it connected.
//
// VALIDATES: the errActiveChangedTwice branches of runConnection and
// RunManagedClient. The hub pushes on both fetches of the first connection
// and lands a local commit during each; the stage wrapper restores the
// baseline after each refusal. The first connection closes, a second one
// arrives and fetches, and the log carries the Info decision and no Warn.
// PREVENTS: a local-commit race kept on a connection nothing fetches over
// again, and one logged as a lost connection.
//
// MUTATION: return errConnectionClosed in place of err in runConnection's
// errActiveChangedTwice branch, and the reconnect logs the Warn "connection
// lost, reconnecting".
func TestLocalCommitTwiceReconnectsQuietly(t *testing.T) {
	// Not parallel: it replaces the package's logger for the length of its run.
	lines := captureLog(t)
	addr, admitted := fetchHub(t)
	store := holdDevice(t, holdEarlierConfig, holdEarlierConfig)
	cfg, _, applied := holdClient(addr, NewBaselineAccess(store))
	cfg.fetchTimeout = fetchBoundShort
	stage := cfg.CommitOver
	cfg.CommitOver = func(data []byte, check func(active []byte) error) error {
		err := stage(data, check)
		if errors.Is(err, errActiveChanged) {
			// A second local commit restores the baseline's content. This runs
			// on the client's goroutine, so it reports with assert.
			assert.NoError(t, store.WriteFile(fleetStatusConfigName, []byte(holdEarlierConfig), 0))
		}
		return err
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { // The device's client, until the test ends.
		defer close(done)
		RunManagedClient(ctx, cfg)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(holdWait):
			t.Error("the client did not return after the test canceled it")
		}
	})
	first := nextSession(t, admitted)

	for edit := range 2 {
		fetch := hubRequest(t, first)
		require.Equal(t, fleet.VerbConfigFetch, fetch.Method)
		// The local commit lands after the device read the hash it sent.
		require.NoError(t, store.WriteFile(fleetStatusConfigName, fmt.Appendf(nil, "device { connect edit %d; }\n", edit), 0))
		require.NoError(t, first.SendResult(t.Context(), fetch.ID, fleet.ConfigFetchResponse{
			Version: fleet.VersionHash([]byte(holdHubConfig)),
			Config:  base64.StdEncoding.EncodeToString([]byte(holdHubConfig)),
		}))
	}
	select {
	case <-first.Done():
	case <-time.After(holdWait):
		t.Fatal("the client kept a connection nothing fetches over again")
	}
	second := nextSession(t, admitted)
	reconnect := hubRequest(t, second)
	// The wrapper put the baseline back, so the device runs holdEarlierConfig.
	assert.Equal(t, versionOf([]byte(holdEarlierConfig)).Hash(), fetchedVersion(t, reconnect),
		"the reconnect's fetch does not decide over what the device runs")
	require.NoError(t, second.SendResult(t.Context(), reconnect.ID, fleet.ConfigFetchResponse{Status: fetchStatusCurrent}))

	assert.Empty(t, applied, "the push committed the hub's configuration over a local commit")
	assert.Zero(t, strings.Count(lines.String(), "level=WARN"), "log: %s", lines.String())
	assert.Equal(t, 1, strings.Count(lines.String(), "keeps its local edit"), "log: %s", lines.String())
}
