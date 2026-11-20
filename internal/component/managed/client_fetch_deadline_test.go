// VALIDATES: AC-6 of spec-managed-server-hardening. Every config-fetch a
// managed client makes is bounded (configFetchTimeout, shortened here through
// ClientConfig.fetchTimeout): at connect the fetch ends on its deadline and
// the connection closes; on the notification loop the device logs the failed
// fetch, keeps its connection and answers the next config-changed; a hub
// answer that arrives after the bound is dropped; a connection the hub closes
// during a fetch is followed by a fetch on the next connection.
// PREVENTS: a hub that keeps the connection open but never answers a fetch
// holding the device at connect, or on its notification loop, for ever.

package managed

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// fetchBoundShort is the fetch bound these tests give the client in place of
// configFetchTimeout, so no test waits 30 seconds.
const fetchBoundShort = 200 * time.Millisecond

// fetchAuthLineMax bounds the auth line the fetch hub reads from a client.
const fetchAuthLineMax = 4096

// fetchSession is one connection the fetch hub admitted. The test reads the
// client's requests from it, answers them, and calls config-changed over it.
type fetchSession struct {
	*rpc.MuxConn
	conn net.Conn
}

// drop closes the connection, as the hub's push worker does when a
// config-changed fails (pluginserver notifyDropped).
func (s fetchSession) drop() {
	_ = s.Close()
	_ = s.conn.Close()
}

// fetchHub is a TLS hub whose leaf a fresh authority issues, installed as the
// client's pki ca "fleet-hub". It admits every client that sends an auth line
// and hands each admitted connection to the returned channel; it answers
// nothing else itself, the test does. Its cleanup MUST close the listener,
// join the accept worker, then close every admitted connection.
func fetchHub(t *testing.T) (string, <-chan fetchSession) {
	t.Helper()
	root := testRoot(t)
	installCA(t, root)
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		MinVersion: tls.VersionTLS13,
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			cert, issueErr := root.IssueLeaf("ze-managed-hub", []string{"127.0.0.1"})
			return &cert, issueErr
		},
	})
	require.NoError(t, err)
	admitted := make(chan fetchSession, 4)
	var opened []fetchSession // Written by the accept worker only; read after it joined.
	done := make(chan struct{})
	go func() { // The accept worker: it ends when the cleanup closes the listener.
		defer close(done)
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			session, admitErr := admitFetchClient(conn)
			if admitErr != nil {
				_ = conn.Close()
				continue
			}
			opened = append(opened, session)
			select {
			case admitted <- session:
			default:
				session.drop() // More connections than any test reads.
			}
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		select {
		case <-done:
		case <-time.After(holdWait):
			t.Error("the fetch hub's accept worker did not join after its listener closed")
			return
		}
		for _, session := range opened {
			session.drop()
		}
	})
	return listener.Addr().String(), admitted
}

// admitFetchClient reads the client's auth line from conn and answers ok, as
// a hub that knows the client does, then carries the RPCs that follow.
func admitFetchClient(conn net.Conn) (fetchSession, error) {
	if err := conn.SetDeadline(time.Now().Add(holdWait)); err != nil {
		return fetchSession{}, err
	}
	if _, err := readLine(conn, fetchAuthLineMax); err != nil {
		return fetchSession{}, err
	}
	if _, err := conn.Write([]byte("#0 ok\n")); err != nil {
		return fetchSession{}, err
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		return fetchSession{}, err
	}
	return fetchSession{MuxConn: rpc.NewMuxConn(rpc.NewConn(conn, conn)), conn: conn}, nil
}

// nextSession answers the next connection the fetch hub admitted.
func nextSession(t *testing.T, admitted <-chan fetchSession) fetchSession {
	t.Helper()
	select {
	case session := <-admitted:
		return session
	case <-time.After(holdWait):
		t.Fatal("the client did not connect to the fetch hub")
		return fetchSession{}
	}
}

// hubRequest answers the next request the client sent over session.
func hubRequest(t *testing.T, session fetchSession) *rpc.Request {
	t.Helper()
	select {
	case req, ok := <-session.Requests():
		require.True(t, ok, "the client closed the connection before its next request")
		return req
	case <-time.After(holdWait):
		t.Fatal("the client sent no request to the fetch hub")
		return nil
	}
}

// answerFetchCurrent takes the client's next request, which MUST be a
// config-fetch, and answers that the hub holds the version it names.
func answerFetchCurrent(t *testing.T, session fetchSession) {
	t.Helper()
	req := hubRequest(t, session)
	require.Equal(t, fleet.VerbConfigFetch, req.Method)
	require.NoError(t, session.SendResult(t.Context(), req.ID, fleet.ConfigFetchResponse{Status: fetchStatusCurrent}))
}

// pushChanged calls config-changed on the client over session under holdWait,
// as the hub's push worker does under pluginserver.ManagedPushTimeout.
func pushChanged(t *testing.T, session fetchSession) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), holdWait)
	defer cancel()
	_, err := session.CallRPC(ctx, fleet.VerbConfigChanged, fleet.ConfigChanged{Version: fleet.VersionHash([]byte(holdHubConfig))})
	return err
}

// fetchClient is a client of the fetch hub at addr, for a device that runs
// its baseline, whose fetch bound is fetchBoundShort.
func fetchClient(t *testing.T, addr string) (ClientConfig, *BaselineAccess, chan []byte) {
	t.Helper()
	access := appliedDevice(t)
	cfg, _, applied := holdClient(addr, access)
	cfg.fetchTimeout = fetchBoundShort
	return cfg, access, applied
}

// runFetchConnection runs one connection of cfg (runConnection) and answers
// what it returns. Its cleanup MUST cancel the connection and join it.
func runFetchConnection(t *testing.T, cfg *ClientConfig) <-chan error {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	returned := make(chan error, 1)
	done := make(chan struct{})
	go func() { // The device's connection, until it returns or the test ends.
		defer close(done)
		returned <- runConnection(ctx, cfg, newBackoff(time.Millisecond, time.Second, 0))
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(holdWait):
			t.Error("the client's connection did not return after the test canceled it")
		}
	})
	return returned
}

// requireStillConnected asserts that the connection runConnection holds has
// not returned.
func requireStillConnected(t *testing.T, returned <-chan error) {
	t.Helper()
	select {
	case err := <-returned:
		t.Fatalf("the client ended its connection: %v", err)
	default:
	}
}

// TestFetchDeadlineAtConnect proves a hub that authenticates the device and
// never answers its connect fetch does not hold it: runConnection returns the
// fetch's deadline error after the bound and closes the connection, so
// RunManagedClient dials again with backoff.
//
// VALIDATES: AC-6 at connect.
// PREVENTS: a device stuck for ever on its first fetch of a connection.
//
// MUTATION: call CallRPC with ctx instead of the deadline context in
// fetchConfigWithin: runConnection never returns.
func TestFetchDeadlineAtConnect(t *testing.T) {
	addr, admitted := fetchHub(t)
	cfg, access, applied := fetchClient(t, addr)
	returned := runFetchConnection(t, &cfg)
	hub := nextSession(t, admitted)

	require.Equal(t, fleet.VerbConfigFetch, hubRequest(t, hub).Method)
	select {
	case err := <-returned:
		// Only the fetch's own deadline answers DeadlineExceeded, so the error
		// says the bound ended the fetch.
		require.ErrorIs(t, err, context.DeadlineExceeded)
	case <-time.After(fetchBoundShort + holdWait):
		t.Fatal("a connect fetch the hub never answered did not end at its bound")
	}
	select {
	case <-hub.Done():
	case <-time.After(holdWait):
		t.Fatal("the client kept the connection after its connect fetch failed")
	}
	assert.Empty(t, applied, "the device applied a configuration no hub sent")
	requireDeviceUnchanged(t, access)
}

// TestFetchDeadlineOnNotificationLoop proves a fetch the hub never answers
// after a config-changed ends at its bound and frees the loop: a second
// config-changed, called under a push bound longer than the fetch bound, is
// answered, and the device fetches again after it on the same connection.
//
// VALIDATES: AC-6 on the notification loop.
// PREVENTS: a device that answers no config-changed after one unanswered
// fetch, so the hub closes the connection at every change.
//
// MUTATION: call CallRPC with ctx instead of the deadline context in
// fetchConfigWithin: the second config-changed fails on its bound.
func TestFetchDeadlineOnNotificationLoop(t *testing.T) {
	addr, admitted := fetchHub(t)
	cfg, access, applied := fetchClient(t, addr)
	returned := runFetchConnection(t, &cfg)
	hub := nextSession(t, admitted)
	answerFetchCurrent(t, hub)

	require.NoError(t, pushChanged(t, hub), "the device did not answer the first config-changed")
	require.Equal(t, fleet.VerbConfigFetch, hubRequest(t, hub).Method, "the device did not fetch after config-changed")
	require.NoError(t, pushChanged(t, hub),
		"the device did not answer a config-changed sent while its unanswered fetch waited")
	require.Equal(t, fleet.VerbConfigFetch, hubRequest(t, hub).Method, "the device sent no fetch after the second config-changed")

	requireStillConnected(t, returned)
	assert.Empty(t, applied, "the device applied a configuration no hub sent")
	requireDeviceUnchanged(t, access)
}

// TestFetchDeadlineDropsLateAnswer proves an answer the hub sends after the
// device gave up on its fetch is dropped: it carries a configuration the
// device would apply, and nothing is committed, the connection stays open and
// the next config-changed is answered.
//
// VALIDATES: R-6. The late answer finds no pending call (rpc MuxConn
// discardOrphanResponse) and never reaches the commit.
// PREVENTS: a late answer applied over a decision the device no longer holds,
// or a late answer that ends the connection.
//
// MUTATION: in rpc MuxConn routeResponse, hand a response whose id has no
// pending entry to another pending CallRPC entry: the late answer reaches the
// fetch the device still waits on, and the device commits it.
func TestFetchDeadlineDropsLateAnswer(t *testing.T) {
	addr, admitted := fetchHub(t)
	cfg, access, applied := fetchClient(t, addr)
	returned := runFetchConnection(t, &cfg)
	hub := nextSession(t, admitted)
	answerFetchCurrent(t, hub)

	require.NoError(t, pushChanged(t, hub))
	late := hubRequest(t, hub)
	require.Equal(t, fleet.VerbConfigFetch, late.Method)
	// The device answers this second config-changed only once its fetch ended.
	require.NoError(t, pushChanged(t, hub))
	next := hubRequest(t, hub)
	require.Equal(t, fleet.VerbConfigFetch, next.Method)
	require.NoError(t, hub.SendResult(t.Context(), late.ID, fleet.ConfigFetchResponse{
		Version: fleet.VersionHash([]byte(holdHubConfig)),
		Config:  base64.StdEncoding.EncodeToString([]byte(holdHubConfig)),
	}))
	require.NoError(t, hub.SendResult(t.Context(), next.ID, fleet.ConfigFetchResponse{Status: fetchStatusCurrent}))
	require.NoError(t, pushChanged(t, hub), "the device did not answer a config-changed after the late answer")
	require.Equal(t, fleet.VerbConfigFetch, hubRequest(t, hub).Method)

	requireStillConnected(t, returned)
	assert.Empty(t, applied, "the device committed the answer it had given up on")
	requireDeviceUnchanged(t, access)
}

// TestFetchAtConnectAfterHubDropsPush proves the device half of the
// self-healing OD-3 accepts: when the hub closes the connection while the
// device's loop fetch waits (its config-changed failed on the push bound),
// the client reconnects and its first request on the new connection is a
// config-fetch naming the configuration it runs, so the change the lost push
// announced is delivered.
//
// VALIDATES: R-7, the device half (the hub half is
// TestManagedServeUnansweredPushDisconnects).
// PREVENTS: a device that reconnects and waits for a notification the hub
// will not send again.
//
// MUTATION: skip fetchAndProcess in runConnection: the new connection carries
// no fetch.
func TestFetchAtConnectAfterHubDropsPush(t *testing.T) {
	addr, admitted := fetchHub(t)
	cfg, _, _ := fetchClient(t, addr)
	cfg.fetchTimeout = 0 // configFetchTimeout: only the hub's close ends the fetch.
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
	answerFetchCurrent(t, first)
	require.NoError(t, pushChanged(t, first))
	require.Equal(t, fleet.VerbConfigFetch, hubRequest(t, first).Method)

	first.drop()

	second := nextSession(t, admitted)
	assert.Equal(t, versionOf([]byte(holdEarlierConfig)).Hash(), fetchedVersion(t, hubRequest(t, second)),
		"the first request on the new connection is not a fetch of the configuration the device runs")
}

// TestFirstBootFetchEndsAtFetchDeadline proves the bound lives in the fetch
// itself, so a caller whose context carries no deadline, as first boot's
// fetch now does (cmd/ze fetchInitialConfig), is still bounded: the bounded
// fetch FetchConfig delegates to, run with a short bound against a hub that
// never answers, ends with the deadline error after that bound.
//
// VALIDATES: OD-6, with TestFirstBootFetchOutlivesConnectTimeout (cmd/ze) and
// TestFetchTimeoutDocumentsPushBoundTradeOff (the constant's value).
// PREVENTS: a first boot that waits for ever on a silent hub.
//
// MUTATION: call CallRPC with ctx instead of the deadline context in
// fetchConfigWithin: the fetch does not return within its bound.
func TestFirstBootFetchEndsAtFetchDeadline(t *testing.T) {
	mc, seen := heldPipe(t, func(*rpc.Request) any { return heldAnswer{silent: true} })
	returned := make(chan error, 1)
	go func() { // Ends when the pipe closes, if the bound is missing.
		_, err := fetchConfigWithin(context.Background(), mc, "", fetchBoundShort)
		returned <- err
	}()

	require.Equal(t, fleet.VerbConfigFetch, nextRequest(t, seen).Method)
	select {
	case err := <-returned:
		// Only the fetch's own deadline answers DeadlineExceeded, so the error
		// says the bound ended the fetch.
		require.ErrorIs(t, err, context.DeadlineExceeded)
	case <-time.After(fetchBoundShort + holdWait):
		t.Fatal("a fetch the hub never answered did not return within its bound")
	}
}
