// Design: (none -- new TACACS+ component)
// Detail: client.go -- sendToServers, sendReceive, trySend, closeAndEvict,
// the connection producers these tests pin
// Detail: packet.go -- Encrypt, the pseudo-pad the key length test drives
//
// VALIDATES: RFC 8907 Section 4.1 (the first packet carries seq_no 1),
// Section 4.3 (Single Connection Mode: no second packet before the mode is
// established, the flag ignored after the second packet, a server-side
// closure accommodated), Section 4.4 (a broken pooled connection accepts
// no further session and is closed), Section 5.4.2.2 (one START, one
// REPLY) and Section 10.5.1 (a 32-character shared key).
// PREVENTS: a client that pipelines a second session onto a connection the
// server never agreed to share, keeps using a connection after a bad reply,
// or sends a CONTINUE in a PAP exchange.
//
// sessionServer differs from client_test.go's testTacacsServer in one way
// that every test here needs: it serves many packets per connection and
// many connections, and records what arrived on each.

package tacacs

import (
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// sessionConn is what the server saw on one accepted TCP connection.
type sessionConn struct {
	requests []PacketHeader
	done     chan struct{} // closed when the server's read loop on it ends
}

// sessionServer is a TACACS+ server that answers every packet on every
// connection with a PASS, echoes TAC_PLUS_SINGLE_CONNECT_FLAG on the first
// reply of a connection when echo is set, and records each request header.
type sessionServer struct {
	listener net.Listener
	key      []byte
	echo     bool
	// closeAfter closes a connection once that many packets were answered
	// on it; zero keeps it open until the client closes it.
	closeAfter int
	// dropBeforeReply closes a connection on its first packet without
	// answering, so the client sees EOF where the reply should be.
	dropBeforeReply bool
	// replyHeaderFn rewrites the reply header for the packet at index n on
	// its connection; nil keeps the header the client expects.
	replyHeaderFn func(n int, reply PacketHeader) PacketHeader
	// replyStatus is the authentication status every reply carries.
	replyStatus uint8
	// replyBodyFn can corrupt the decrypted body on a selected exchange.
	replyBodyFn func(n int, body []byte) []byte

	mu    sync.Mutex
	conns []*sessionConn
}

func newSessionServer(t *testing.T, key []byte, echo bool) *sessionServer {
	t.Helper()
	srv := &sessionServer{listener: listenTCP(t), key: key, echo: echo, replyStatus: AuthenStatusPass}
	t.Cleanup(func() { closeIgnore(srv.listener) })
	go srv.serve()
	return srv
}

func (s *sessionServer) addr() string { return s.listener.Addr().String() }

func (s *sessionServer) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		sc := &sessionConn{done: make(chan struct{})}
		s.mu.Lock()
		s.conns = append(s.conns, sc)
		s.mu.Unlock()
		go s.serveConn(conn, sc)
	}
}

func (s *sessionServer) serveConn(conn net.Conn, sc *sessionConn) {
	defer close(sc.done)
	defer closeIgnore(conn)
	var hdrBuf [hdrLen]byte
	for n := 0; ; n++ {
		if _, err := io.ReadFull(conn, hdrBuf[:]); err != nil {
			return
		}
		hdr, err := UnmarshalPacketHeader(hdrBuf[:])
		if err != nil {
			return
		}
		body := make([]byte, hdr.Length)
		if _, err := io.ReadFull(conn, body); err != nil {
			return
		}
		s.mu.Lock()
		sc.requests = append(sc.requests, hdr)
		s.mu.Unlock()
		if s.dropBeforeReply {
			return
		}

		replyBody := []byte{s.replyStatus, 0, 0, 0, 0, 0}
		if s.replyBodyFn != nil {
			replyBody = s.replyBodyFn(n, replyBody)
		}
		replyHdr := PacketHeader{
			Version:   hdr.Version,
			Type:      hdr.Type,
			SeqNo:     hdr.SeqNo + 1,
			SessionID: hdr.SessionID,
			Length:    uint32(len(replyBody)),
		}
		if s.echo && n == 0 {
			replyHdr.Flags |= FlagSingleConnect
		}
		if s.replyHeaderFn != nil {
			replyHdr = s.replyHeaderFn(n, replyHdr)
		}
		Encrypt(replyBody, replyHdr.SessionID, s.key, replyHdr.Version, replyHdr.SeqNo)
		wire := append(replyHdr.MarshalBinary(), replyBody...)
		if _, err := conn.Write(wire); err != nil {
			return
		}
		if s.closeAfter > 0 && n+1 >= s.closeAfter {
			return
		}
	}
}

// connections returns a snapshot of what the server saw so far.
func (s *sessionServer) connections() []*sessionConn {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*sessionConn, len(s.conns))
	copy(out, s.conns)
	return out
}

// requestsOn returns the request headers recorded on connection i.
func (s *sessionServer) requestsOn(i int) []PacketHeader {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]PacketHeader(nil), s.conns[i].requests...)
}

// waitClosed blocks until the server's read loop on connection i ended,
// which is how the server observes the client closing that TCP.
func waitClosed(t *testing.T, sc *sessionConn) {
	t.Helper()
	select {
	case <-sc.done:
	case <-time.After(5 * time.Second):
		t.Fatal("the client did not close the connection")
	}
}

func isClosed(sc *sessionConn) bool {
	select {
	case <-sc.done:
		return true
	default:
		return false
	}
}

var sessionKey = []byte("shared-secret-for-session-tests!")

func newSessionClient(t *testing.T, srv *sessionServer, key []byte) *TacacsClient {
	t.Helper()
	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: srv.addr(), Key: key}},
		Timeout: 5 * time.Second,
	})
	t.Cleanup(client.Close)
	return client
}

func authenticatePass(t *testing.T, client *TacacsClient) {
	t.Helper()
	reply, err := client.Authenticate("alice", "hunter2", "ssh", "192.0.2.1")
	require.NoError(t, err)
	require.Equal(t, uint8(AuthenStatusPass), reply.Status)
}

// RFC requirement: RFC8907-4.1-2 positive — the header the client writes for a new session carries seq_no 1.
func TestRFC8907FirstPacketCarriesSequenceOne(t *testing.T) {
	srv := newSessionServer(t, sessionKey, false)
	client := newSessionClient(t, srv, sessionKey)
	authenticatePass(t, client)
	authenticatePass(t, client)

	conns := srv.connections()
	require.Len(t, conns, 2)
	for i := range conns {
		requests := srv.requestsOn(i)
		require.Len(t, requests, 1)
		require.Equal(t, uint8(1), requests[0].SeqNo, "connection %d", i)
	}
}

// RFC requirement: RFC8907-4.1-2 negative — a reply that reuses seq_no 1 for the session's second packet is refused as a sequence mismatch.
func TestRFC8907ReplyReusingSequenceOneIsRefused(t *testing.T) {
	srv := newSessionServer(t, sessionKey, false)
	srv.replyHeaderFn = func(_ int, reply PacketHeader) PacketHeader {
		reply.SeqNo = 1
		return reply
	}
	client := newSessionClient(t, srv, sessionKey)
	_, err := client.Authenticate("alice", "hunter2", "ssh", "192.0.2.1")
	require.Error(t, err, "a reply carrying seq_no 1 must not be accepted")
	waitClosed(t, srv.connections()[0])

	// The end-to-end error is the failover summary; the header check names
	// the refusal.
	req := PacketHeader{Version: verMinorOne, Type: typeAuthentication, SeqNo: 1}
	resp := req
	require.ErrorContains(t, validateResponseHeader(req, resp), "sequence mismatch")
}

// RFC requirement: RFC8907-4.3-1 positive — once the server echoed the single-connect flag, the next session's packet travels on the same TCP connection.
func TestRFC8907SecondSessionReusesEstablishedConnection(t *testing.T) {
	srv := newSessionServer(t, sessionKey, true)
	client := newSessionClient(t, srv, sessionKey)
	authenticatePass(t, client)
	authenticatePass(t, client)

	require.Len(t, srv.connections(), 1, "the second session must reuse the established TCP")
	require.Len(t, srv.requestsOn(0), 2)
}

// RFC requirement: RFC8907-4.3-1 negative — when the server does not echo the flag, no second packet is ever sent on the first connection: the next session opens a new TCP.
func TestRFC8907NoSecondPacketWithoutSingleConnect(t *testing.T) {
	srv := newSessionServer(t, sessionKey, false)
	client := newSessionClient(t, srv, sessionKey)
	authenticatePass(t, client)
	waitClosed(t, srv.connections()[0])
	authenticatePass(t, client)

	conns := srv.connections()
	require.Len(t, conns, 2, "a second session must not ride a connection without single-connect")
	require.Len(t, srv.requestsOn(0), 1, "a second packet reached the first connection")
	require.Len(t, srv.requestsOn(1), 1)
}

// RFC requirement: RFC8907-4.3-2 positive — a later reply that clears the single-connect flag is ignored: the pooled connection stays in use for the sessions that follow.
func TestRFC8907FlagClearedOnLaterReplyIsIgnored(t *testing.T) {
	srv := newSessionServer(t, sessionKey, true) // echoes on the first reply only
	client := newSessionClient(t, srv, sessionKey)
	authenticatePass(t, client)
	authenticatePass(t, client)
	authenticatePass(t, client)

	require.Len(t, srv.connections(), 1, "the cleared flag on the second reply must not end single-connect")
	require.Len(t, srv.requestsOn(0), 3)
}

// RFC requirement: RFC8907-4.3-2 negative — after the connection's first exchange the client's request headers carry flags 0: it never re-signals single-connect.
func TestRFC8907ClientDoesNotResignalSingleConnect(t *testing.T) {
	srv := newSessionServer(t, sessionKey, true)
	client := newSessionClient(t, srv, sessionKey)
	authenticatePass(t, client)
	authenticatePass(t, client)
	authenticatePass(t, client)

	requests := srv.requestsOn(0)
	require.Len(t, requests, 3)
	require.Equal(t, uint8(FlagSingleConnect), requests[0].Flags, "first packet signals single-connect")
	require.Equal(t, uint8(0), requests[1].Flags, "second session must not carry the flag")
	require.Equal(t, uint8(0), requests[2].Flags, "third session must not carry the flag")
}

// RFC requirement: RFC8907-4.3-3 positive — a pooled single-connect TCP the server closed is replaced by a fresh dial and the next session completes with a PASS.
func TestRFC8907ServerClosureOfPooledConnectionIsAccommodated(t *testing.T) {
	srv := newSessionServer(t, sessionKey, true)
	srv.closeAfter = 1
	client := newSessionClient(t, srv, sessionKey)
	authenticatePass(t, client)
	waitClosed(t, srv.connections()[0])
	authenticatePass(t, client)

	require.Len(t, srv.connections(), 2, "the closed pooled TCP must be replaced by a fresh dial")
	require.Len(t, srv.requestsOn(1), 1)
}

// RFC requirement: RFC8907-4.3-3 negative — a closure on the fresh dial itself is surfaced as an error after one connection: the client does not dial a second time.
func TestRFC8907ClosureOnFreshDialIsNotRetried(t *testing.T) {
	srv := newSessionServer(t, sessionKey, true)
	srv.dropBeforeReply = true
	client := newSessionClient(t, srv, sessionKey)
	_, err := client.Authenticate("alice", "hunter2", "ssh", "192.0.2.1")
	require.Error(t, err)
	require.Len(t, srv.connections(), 1, "a fresh-dial closure must not be retried")
}

// badSessionIDOnSecondReply corrupts the session_id of the second reply on a
// connection, the incorrect-secret class of failure Section 4.4 names.
func badSessionIDOnSecondReply(n int, reply PacketHeader) PacketHeader {
	if n == 1 {
		reply.SessionID ^= 0xFFFFFFFF
	}
	return reply
}

// RFC requirement: RFC8907-4.4-2 positive — after a reply on the pooled connection fails validation, the next session is not sent on that connection: it opens a new TCP.
func TestRFC8907NoNewSessionOnBrokenPooledConnection(t *testing.T) {
	srv := newSessionServer(t, sessionKey, true)
	srv.replyHeaderFn = badSessionIDOnSecondReply
	client := newSessionClient(t, srv, sessionKey)
	authenticatePass(t, client)
	_, err := client.Authenticate("alice", "hunter2", "ssh", "192.0.2.1")
	require.Error(t, err)
	authenticatePass(t, client)

	require.Len(t, srv.connections(), 2, "the third session must not reuse the broken TCP")
	require.Len(t, srv.requestsOn(0), 2, "a third packet reached the broken connection")
	require.Len(t, srv.requestsOn(1), 1)
}

// RFC requirement: RFC8907-4.4-2 negative — a pooled connection whose replies validate keeps accepting new sessions; the client does not abandon a healthy connection.
func TestRFC8907HealthyPooledConnectionKeepsAcceptingSessions(t *testing.T) {
	srv := newSessionServer(t, sessionKey, true)
	client := newSessionClient(t, srv, sessionKey)
	for range 3 {
		authenticatePass(t, client)
	}
	require.Len(t, srv.connections(), 1)
	require.Len(t, srv.requestsOn(0), 3)
}

// RFC requirement: RFC8907-4.4-3 positive — the broken pooled connection is closed: the server observes EOF on it.
func TestRFC8907BrokenPooledConnectionIsClosed(t *testing.T) {
	srv := newSessionServer(t, sessionKey, true)
	srv.replyHeaderFn = badSessionIDOnSecondReply
	client := newSessionClient(t, srv, sessionKey)
	authenticatePass(t, client)
	_, err := client.Authenticate("alice", "hunter2", "ssh", "192.0.2.1")
	require.Error(t, err)

	waitClosed(t, srv.connections()[0])
}

// RFC requirement: RFC8907-4.4-3 negative — a healthy pooled connection is not closed between sessions: the server still holds it open after three sessions.
func TestRFC8907HealthyPooledConnectionStaysOpen(t *testing.T) {
	srv := newSessionServer(t, sessionKey, true)
	client := newSessionClient(t, srv, sessionKey)
	for range 3 {
		authenticatePass(t, client)
	}
	require.False(t, isClosed(srv.connections()[0]), "a healthy pooled TCP must stay open")
}

// A malformed decrypted body invalidates the pooled stream just as a header
// failure does; the next request must establish a fresh connection.
func TestRFC8907MalformedBodyClosesPooledConnection(t *testing.T) {
	srv := newSessionServer(t, sessionKey, true)
	srv.replyBodyFn = func(n int, body []byte) []byte {
		if n == 1 {
			return append(body, 0)
		}
		return body
	}
	client := newSessionClient(t, srv, sessionKey)
	authenticatePass(t, client)
	_, err := client.Authenticate("alice", "hunter2", "ssh", "192.0.2.1")
	require.Error(t, err)
	waitClosed(t, srv.connections()[0])
	authenticatePass(t, client)
	require.Len(t, srv.connections(), 2)
	require.Len(t, srv.requestsOn(0), 2)
	require.Len(t, srv.requestsOn(1), 1)
}

// RFC requirement: RFC8907-5.4.2.2-1 positive — a PAP authentication puts exactly one START on the wire and returns on the single REPLY.
func TestRFC8907PAPExchangeIsOneStartOneReply(t *testing.T) {
	srv := newSessionServer(t, sessionKey, false)
	client := newSessionClient(t, srv, sessionKey)
	authenticatePass(t, client)
	waitClosed(t, srv.connections()[0])

	requests := srv.requestsOn(0)
	require.Len(t, requests, 1, "a PAP exchange is a single START")
	require.Equal(t, uint8(typeAuthentication), requests[0].Type)
}

// RFC requirement: RFC8907-5.4.2.2-1 negative — a GETPASS reply to the PAP START provokes no second client packet: the client sends no CONTINUE.
func TestRFC8907PAPClientSendsNoContinue(t *testing.T) {
	srv := newSessionServer(t, sessionKey, false)
	srv.replyStatus = AuthenStatusGetPass
	client := newSessionClient(t, srv, sessionKey)
	reply, err := client.Authenticate("alice", "hunter2", "ssh", "192.0.2.1")
	require.Error(t, err)
	require.Nil(t, reply)
	waitClosed(t, srv.connections()[0])

	require.Len(t, srv.requestsOn(0), 1, "a CONTINUE followed the START")
}

// RFC requirement: RFC8907-10.5.1-2 positive — a 32-character shared key and a 300-character one each complete an exchange whose reply the client de-obfuscates to a PASS.
func TestRFC8907LongSharedKeysAuthenticate(t *testing.T) {
	for _, key := range [][]byte{
		[]byte(strings.Repeat("k", 32)),
		[]byte(strings.Repeat("k", 300)),
	} {
		srv := newSessionServer(t, key, false)
		client := newSessionClient(t, srv, key)
		authenticatePass(t, client)
		require.Len(t, srv.requestsOn(0), 1)
	}
}
