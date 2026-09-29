// Design: docs/architecture/aaa-tacacs.md
// Detail: client.go -- trySend and validateResponseHeader, the header checks
// Detail: reply.go -- validateReplyBody, the Section 4.5 length sum
//
// VALIDATES: RFC 8907 Section 4.1 (major version, sequence parity, session_id,
// network byte order, zero-length fields), Section 4.3 (a closed single-connect
// TCP is accommodated), Section 4.5 (the length sum in both directions) and
// Section 10.5.2 (an obfuscation mismatch closes the TCP and reads as FAIL).
// PREVENTS: a header check that passes only because the test server echoes
// the request header back, and a mismatch that is refused but leaves the TCP
// open for the next session.
//
// Each server here captures what the client wrote before it answers, so the
// assertions read the client's own octets rather than an echo of them.

package tacacs

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var requestKinds = []uint8{typeAuthentication, typeAuthorization, typeAccounting}

// headerCaptureServer answers one request of any kind with a valid status and
// hands the request header the client wrote to the test. mutate, when set,
// rewrites the reply header after the capture.
func headerCaptureServer(t *testing.T, mutate func(PacketHeader) PacketHeader) (*testTacacsServer, <-chan PacketHeader) {
	t.Helper()
	seen := make(chan PacketHeader, 1)
	srv := newTestServerWithHeader(t, sessionKey, func(hdr PacketHeader, _ []byte) []byte {
		return validationReply(hdr.Type, 1)
	}, func(req, reply PacketHeader) PacketHeader {
		seen <- req
		if mutate != nil {
			reply = mutate(reply)
		}
		return reply
	})
	return srv, seen
}

func receiveHeader(t *testing.T, seen <-chan PacketHeader) PacketHeader {
	t.Helper()
	select {
	case hdr := <-seen:
		return hdr
	case <-time.After(2 * time.Second):
		t.Fatal("the server never saw a request header")
		return PacketHeader{}
	}
}

// RFC requirement: RFC8907-4-1 positive -- the authentication, authorization and accounting request headers the client writes carry major version 0xc, and the reply that carries it back is accepted.
func TestRFC8907EveryRequestCarriesMajorVersion12(t *testing.T) {
	for _, kind := range requestKinds {
		srv, seen := headerCaptureServer(t, nil)
		client := validationClient(t, srv)
		status, err := validationExchange(client, kind, "alice")
		require.NoError(t, err, "kind %d", kind)
		require.Equal(t, uint8(1), status, "kind %d", kind)
		require.Equal(t, uint8(0xc), receiveHeader(t, seen).Version>>4, "kind %d request major version", kind)
	}
}

// RFC requirement: RFC8907-4-1 negative -- a reply whose major version is any of the fifteen values other than 0xc is refused for every request kind.
func TestRFC8907ReplyWithOtherMajorVersionRefused(t *testing.T) {
	for _, kind := range requestKinds {
		for major := range uint8(16) {
			if major == 0xc {
				continue
			}
			srv, _ := headerCaptureServer(t, func(reply PacketHeader) PacketHeader {
				reply.Version = major<<4 | reply.Version&0x0f
				return reply
			})
			client := validationClient(t, srv)
			status, err := validationExchange(client, kind, "alice")
			require.Error(t, err, "kind %d accepted major version %#x", kind, major)
			require.Zero(t, status)
		}
	}
}

// RFC requirement: RFC8907-4-2 positive -- every request kind leaves the client with an odd seq_no, and the even reply that follows it is accepted.
func TestRFC8907ClientSendsOddSequenceNumbers(t *testing.T) {
	for _, kind := range requestKinds {
		srv, seen := headerCaptureServer(t, nil)
		client := validationClient(t, srv)
		status, err := validationExchange(client, kind, "alice")
		require.NoError(t, err, "kind %d", kind)
		require.Equal(t, uint8(1), status, "kind %d", kind)
		require.Equal(t, uint8(1), receiveHeader(t, seen).SeqNo%2, "kind %d request seq_no is even", kind)
	}
}

// RFC requirement: RFC8907-4-2 negative -- a reply carrying any of the 128 odd seq_no values, which only a client may send, is refused for every request kind.
func TestRFC8907ReplyWithOddSequenceNumberRefused(t *testing.T) {
	for _, kind := range requestKinds {
		for seq := 1; seq < 256; seq += 2 {
			srv, _ := headerCaptureServer(t, func(reply PacketHeader) PacketHeader {
				reply.SeqNo = uint8(seq)
				return reply
			})
			client := validationClient(t, srv)
			status, err := validationExchange(client, kind, "alice")
			require.Error(t, err, "kind %d accepted odd server seq_no %d", kind, seq)
			require.Zero(t, status)
		}
	}
}

// RFC requirement: RFC8907-4-4 positive -- the reply that carries the session_id the client wrote in its request is accepted for every request kind.
func TestRFC8907ReplyKeepingSessionIDAccepted(t *testing.T) {
	for _, kind := range requestKinds {
		srv, seen := headerCaptureServer(t, nil)
		client := validationClient(t, srv)
		status, err := validationExchange(client, kind, "alice")
		require.NoError(t, err, "kind %d", kind)
		require.Equal(t, uint8(1), status, "kind %d", kind)
		receiveHeader(t, seen)
	}
}

// RFC requirement: RFC8907-4-4 negative -- a reply whose session_id differs from the request's, by its lowest bit, its highest bit or all bits, is refused for every request kind.
func TestRFC8907ReplyChangingSessionIDRefused(t *testing.T) {
	for _, kind := range requestKinds {
		for _, flip := range []uint32{0x00000001, 0x80000000, 0xFFFFFFFF} {
			srv, _ := headerCaptureServer(t, func(reply PacketHeader) PacketHeader {
				reply.SessionID ^= flip
				return reply
			})
			client := validationClient(t, srv)
			status, err := validationExchange(client, kind, "alice")
			require.Error(t, err, "kind %d accepted a session_id changed by %#x", kind, flip)
			require.Zero(t, status)
		}
	}
}

// RFC requirement: RFC8907-4-5 positive -- the header session_id and length are written and read as big-endian unsigned octets, and every two-octet reply length is read big-endian and unsigned, including values at or above 0x8000.
func TestRFC8907LengthsAreUnsignedNetworkOrder(t *testing.T) {
	hdr := PacketHeader{Version: 0xC1, Type: typeAuthentication, SeqNo: 1, SessionID: 0xA1B2C3D4, Length: 0x01020304}
	wire := hdr.MarshalBinary()
	require.Equal(t, []byte{0xA1, 0xB2, 0xC3, 0xD4}, wire[4:8], "session_id octets")
	require.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, wire[8:12], "length octets")

	read, err := UnmarshalPacketHeader([]byte{0xC1, typeAuthentication, 2, 0, 0, 0, 0, 1, 0xFF, 0xFF, 0xFF, 0xFE})
	require.NoError(t, err)
	require.Equal(t, uint32(1), read.SessionID)
	require.Equal(t, uint32(0xFFFFFFFE), read.Length, "the header length is unsigned")

	for _, length := range []int{0x0100, 0x8001} {
		message := strings.Repeat("m", length)
		authen := make([]byte, 6, 6+length)
		authen[0] = AuthenStatusPass
		binary.BigEndian.PutUint16(authen[2:4], uint16(length))
		authen = append(authen, message...)
		authenReply, err := UnmarshalAuthenReply(authen)
		require.NoError(t, err)
		require.Len(t, authenReply.ServerMsg, length, "authentication server_msg_len")

		author := make([]byte, 6, 6+length)
		author[0] = AuthorStatusPassAdd
		binary.BigEndian.PutUint16(author[4:6], uint16(length))
		author = append(author, message...)
		authorReply, err := UnmarshalAuthorResponse(author)
		require.NoError(t, err)
		require.Len(t, authorReply.Data, length, "authorization data_len")

		acct := make([]byte, 5, 5+length)
		binary.BigEndian.PutUint16(acct[0:2], uint16(length))
		acct[4] = AcctStatusSuccess
		acct = append(acct, message...)
		acctReply, err := UnmarshalAcctReply(acct)
		require.NoError(t, err)
		require.Len(t, acctReply.ServerMsg, length, "accounting server_msg_len")
	}
}

// RFC requirement: RFC8907-4.6-1 negative -- a reply whose component lengths sum to more than the header datalength, through server_msg_len, data_len or an argument length, is discarded with an error for every request kind.
func TestRFC8907OverlongReplyComponentsRejected(t *testing.T) {
	cases := []struct {
		kind uint8
		body []byte
	}{
		{typeAuthentication, []byte{AuthenStatusPass, 0, 0, 5, 0, 0, 'a', 'b', 'c'}},
		{typeAuthentication, []byte{AuthenStatusPass, 0, 0, 0, 0, 4, 'a'}},
		{typeAuthorization, []byte{AuthorStatusPassAdd, 1, 0, 0, 0, 0, 10, 'x', '=', 'y'}},
		{typeAuthorization, []byte{AuthorStatusPassAdd, 0, 0, 3, 0, 0, 'o', 'k'}},
		{typeAccounting, []byte{0, 5, 0, 0, AcctStatusSuccess, 'a', 'b'}},
		{typeAccounting, []byte{0, 0, 0, 2, AcctStatusSuccess, 'a'}},
	}
	for _, tc := range cases {
		server := newTestServer(t, sessionKey, func(_ PacketHeader, _ []byte) []byte {
			return tc.body
		})
		client := validationClient(t, server)
		status, err := validationExchange(client, tc.kind, "alice")
		require.Error(t, err, "kind %d accepted body %x whose lengths exceed it", tc.kind, tc.body)
		require.Zero(t, status)
	}
}

// RFC requirement: RFC8907-4.1-1 positive -- fields whose length is not zero are read at the offsets the zero-length fields before them leave, in authentication, authorization and accounting replies.
func TestRFC8907PresentFieldsFollowZeroLengthOnes(t *testing.T) {
	authen, err := UnmarshalAuthenReply([]byte{AuthenStatusPass, 0, 0, 0, 0, 2, 0x00, 0xFF})
	require.NoError(t, err)
	require.Equal(t, []byte{0x00, 0xFF}, authen.Data)

	author, err := UnmarshalAuthorResponse([]byte{AuthorStatusPassAdd, 3, 0, 0, 0, 2, 0, 3, 0, 'o', 'k', 'a', '=', 'b'})
	require.NoError(t, err)
	require.Equal(t, []byte("ok"), author.Data)
	require.Equal(t, []string{"a=b"}, author.Args)

	acct, err := UnmarshalAcctReply([]byte{0, 0, 0, 2, AcctStatusSuccess, 'o', 'k'})
	require.NoError(t, err)
	require.Equal(t, []byte("ok"), acct.Data)
}

// RFC requirement: RFC8907-4.1-1 negative -- a zero-length server_msg, data or argument is not present in the decoded reply: no empty argument, no message and no data are produced for it.
func TestRFC8907ZeroLengthFieldsAreNotPresent(t *testing.T) {
	authen, err := UnmarshalAuthenReply([]byte{AuthenStatusPass, 0, 0, 0, 0, 0})
	require.NoError(t, err)
	require.Empty(t, authen.ServerMsg)
	require.Empty(t, authen.Data)

	author, err := UnmarshalAuthorResponse([]byte{AuthorStatusPassAdd, 3, 0, 0, 0, 0, 0, 3, 0, 'a', '=', 'b'})
	require.NoError(t, err)
	require.Empty(t, author.ServerMsg)
	require.Empty(t, author.Data)
	require.Equal(t, []string{"a=b"}, author.Args, "a zero-length argument became an entry")

	acct, err := UnmarshalAcctReply([]byte{0, 0, 0, 0, AcctStatusSuccess})
	require.NoError(t, err)
	require.Empty(t, acct.ServerMsg)
	require.Empty(t, acct.Data)
}

// midExchangeCloser keeps every TCP open as a single-connect server would, and
// closes the connection that carries request number dropAt, counted across all
// connections, after reading it and without answering it.
type midExchangeCloser struct {
	listener net.Listener
	dropAt   int

	mu       sync.Mutex
	requests int
	perConn  []int
}

func newMidExchangeCloser(t *testing.T, dropAt int) *midExchangeCloser {
	t.Helper()
	s := &midExchangeCloser{listener: listenTCP(t), dropAt: dropAt}
	t.Cleanup(func() { closeIgnore(s.listener) })
	go s.serve()
	return s
}

func (s *midExchangeCloser) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		s.mu.Lock()
		index := len(s.perConn)
		s.perConn = append(s.perConn, 0)
		s.mu.Unlock()
		go s.serveConn(conn, index)
	}
}

func (s *midExchangeCloser) serveConn(conn net.Conn, index int) {
	defer closeIgnore(conn)
	var hdrBuf [hdrLen]byte
	for {
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
		s.perConn[index]++
		s.requests++
		drop := s.requests == s.dropAt
		s.mu.Unlock()
		if drop {
			return
		}
		reply := passReply()(hdr, body)
		replyHdr := PacketHeader{
			Version: hdr.Version, Type: hdr.Type, SeqNo: hdr.SeqNo + 1,
			Flags: FlagSingleConnect, SessionID: hdr.SessionID, Length: uint32(len(reply)),
		}
		Encrypt(reply, replyHdr.SessionID, sessionKey, replyHdr.Version, replyHdr.SeqNo)
		if _, err := conn.Write(append(replyHdr.MarshalBinary(), reply...)); err != nil {
			return
		}
	}
}

func (s *midExchangeCloser) counts() []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]int(nil), s.perConn...)
}

// RFC requirement: RFC8907-4.3-3 negative -- when the server closes an established single-connect TCP after reading a later session's request and without answering it, that session is not failed: it completes with a PASS on a fresh TCP.
func TestRFC8907ClosureMidSessionOnPooledConnectionIsAccommodated(t *testing.T) {
	srv := newMidExchangeCloser(t, 3)
	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: srv.listener.Addr().String(), Key: sessionKey}},
		Timeout: 5 * time.Second,
	})
	t.Cleanup(client.Close)

	authenticatePass(t, client)
	authenticatePass(t, client)
	require.Equal(t, []int{2}, srv.counts(), "two sessions share the established TCP")

	authenticatePass(t, client)
	require.Equal(t, []int{3, 1}, srv.counts(), "the unanswered session must be carried to completion on a fresh TCP")
}

// mismatchProbe answers one request with a PASS whose obfuscation flag and body
// are chosen by the test, echoing TAC_PLUS_SINGLE_CONNECT_FLAG so that a client
// ignoring the mismatch would keep the TCP. It then reports whether the client
// closed the TCP within wait.
type mismatchProbe struct {
	listener  net.Listener
	flags     uint8
	obfuscate bool
	wait      time.Duration

	requestSeen  chan bool
	clientClosed chan bool
}

func newMismatchProbe(t *testing.T, flags uint8, obfuscate bool, wait time.Duration) *mismatchProbe {
	t.Helper()
	p := &mismatchProbe{
		listener: listenTCP(t), flags: flags, obfuscate: obfuscate, wait: wait,
		requestSeen: make(chan bool, 1), clientClosed: make(chan bool, 1),
	}
	t.Cleanup(func() { closeIgnore(p.listener) })
	go p.serve()
	return p
}

func (p *mismatchProbe) serve() {
	conn, err := p.listener.Accept()
	if err != nil {
		return
	}
	defer closeIgnore(conn)
	var hdrBuf [hdrLen]byte
	if _, err := io.ReadFull(conn, hdrBuf[:]); err != nil {
		p.requestSeen <- false
		return
	}
	p.requestSeen <- true
	hdr, err := UnmarshalPacketHeader(hdrBuf[:])
	if err != nil {
		return
	}
	body := make([]byte, hdr.Length)
	if _, err := io.ReadFull(conn, body); err != nil {
		return
	}
	reply := passReply()(hdr, body)
	if hdr.Type == typeAuthorization {
		reply = authorReplyArgs(AuthorStatusPassAdd, nil)(hdr, body)
	}
	replyHdr := PacketHeader{
		Version: hdr.Version, Type: hdr.Type, SeqNo: hdr.SeqNo + 1,
		Flags: FlagSingleConnect | p.flags, SessionID: hdr.SessionID, Length: uint32(len(reply)),
	}
	if p.obfuscate {
		Encrypt(reply, replyHdr.SessionID, sessionKey, replyHdr.Version, replyHdr.SeqNo)
	}
	if _, err := conn.Write(append(replyHdr.MarshalBinary(), reply...)); err != nil {
		return
	}
	if err := conn.SetReadDeadline(time.Now().Add(p.wait)); err != nil {
		return
	}
	// A client that closes before reading the reply body closes with unread
	// data, which the kernel signals with a reset rather than a FIN.
	var one [1]byte
	_, err = conn.Read(one[:])
	p.clientClosed <- errors.Is(err, io.EOF) || errors.Is(err, syscall.ECONNRESET)
}

func (p *mismatchProbe) closed(t *testing.T) bool {
	t.Helper()
	select {
	case closed := <-p.clientClosed:
		return closed
	case <-time.After(p.wait + 2*time.Second):
		t.Fatal("the probe never reported on the TCP")
		return false
	}
}

func mismatchClient(t *testing.T, p *mismatchProbe, key []byte) *TacacsClient {
	t.Helper()
	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: p.listener.Addr().String(), Key: key}},
		Timeout: 2 * time.Second,
	})
	t.Cleanup(client.Close)
	return client
}

func mismatchAuthorRequest() *AuthorRequest {
	return &AuthorRequest{User: "alice", Port: "ssh", RemAddr: "192.0.2.1", Args: []string{"service=shell", "cmd=show"}}
}

// RFC requirement: RFC8907-10.5.2-1 negative -- a reply from a server configured with a shared key that sets TAC_PLUS_UNENCRYPTED_FLAG makes the client close the TCP and return TAC_PLUS_AUTHEN_STATUS_FAIL for an authentication session and TAC_PLUS_AUTHOR_STATUS_FAIL for an authorization session; a server configured without a key never receives a request, so no reply from it is ever processed.
func TestRFC8907ObfuscationMismatchClosesAndFails(t *testing.T) {
	authenProbe := newMismatchProbe(t, FlagUnencrypted, false, 2*time.Second)
	authen, err := mismatchClient(t, authenProbe, sessionKey).Authenticate("alice", "secret", "ssh", "192.0.2.1")
	require.NoError(t, err)
	require.Equal(t, uint8(AuthenStatusFail), authen.Status)
	require.True(t, authenProbe.closed(t), "the client kept the TCP after an unobfuscated authentication reply")

	authorProbe := newMismatchProbe(t, FlagUnencrypted, false, 2*time.Second)
	author, err := mismatchClient(t, authorProbe, sessionKey).SendAuthorization(mismatchAuthorRequest())
	require.NoError(t, err)
	require.Equal(t, uint8(AuthorStatusFail), author.Status)
	require.True(t, authorProbe.closed(t), "the client kept the TCP after an unobfuscated authorization reply")

	keyless := newMismatchProbe(t, 0, false, 2*time.Second)
	_, err = mismatchClient(t, keyless, nil).Authenticate("alice", "secret", "ssh", "192.0.2.1")
	require.Error(t, err)
	select {
	case seen := <-keyless.requestSeen:
		require.False(t, seen, "a request reached a server configured without a key")
	case <-time.After(4 * time.Second):
		t.Fatal("the keyless probe never reported")
	}
}

// RFC requirement: RFC8907-10.5.2-1 positive -- a reply from a keyed server that clears TAC_PLUS_UNENCRYPTED_FLAG and is obfuscated keeps its PASS or PASS_ADD status for both session kinds, and the single-connect TCP stays open.
func TestRFC8907MatchingObfuscationKeepsStatusAndConnection(t *testing.T) {
	authenProbe := newMismatchProbe(t, 0, true, 300*time.Millisecond)
	authen, err := mismatchClient(t, authenProbe, sessionKey).Authenticate("alice", "secret", "ssh", "192.0.2.1")
	require.NoError(t, err)
	require.Equal(t, uint8(AuthenStatusPass), authen.Status)
	require.False(t, authenProbe.closed(t), "the client closed a TCP whose reply matched its configuration")

	authorProbe := newMismatchProbe(t, 0, true, 300*time.Millisecond)
	author, err := mismatchClient(t, authorProbe, sessionKey).SendAuthorization(mismatchAuthorRequest())
	require.NoError(t, err)
	require.Equal(t, uint8(AuthorStatusPassAdd), author.Status)
	require.False(t, authorProbe.closed(t), "the client closed a TCP whose reply matched its configuration")
}
