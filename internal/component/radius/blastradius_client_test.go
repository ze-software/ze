// Design: docs/guide/radius.md -- BlastRADIUS: Message-Authenticator on every Access-Request, unsigned answers discarded
// Related: client.go -- encodeRequest, dispatchResponse, discardUnsignedResponse

package radius

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/md5" //nolint:gosec // RFC 2869 Section 5.14 mandates HMAC-MD5.
	"encoding/binary"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/metrics"
)

// codeProtocolError is the RFC 7930 Protocol-Error code, which
// draft-ietf-radext-deprecating-radius Section 4.6 lists among the answers to
// an Access-Request. Ze sends nothing that solicits it, so dict.go names none.
const codeProtocolError = 52

// lockedLog is a log sink the client's read goroutine and the test share.
type lockedLog struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *lockedLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *lockedLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

// scriptedServer answers each request with the datagrams script returns, and
// records every request it received.
type scriptedServer struct {
	conn     *net.UDPConn
	addr     string
	mu       sync.Mutex
	requests [][]byte
	done     chan struct{}
}

func newScriptedServer(t *testing.T, script func(req []byte) [][]byte) *scriptedServer {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	s := &scriptedServer{conn: conn, addr: conn.LocalAddr().String(), done: make(chan struct{})}
	go s.serve(script)
	t.Cleanup(func() {
		_ = conn.Close()
		<-s.done
	})
	return s
}

func (s *scriptedServer) serve(script func(req []byte) [][]byte) {
	defer close(s.done)
	buf := make([]byte, MaxPacketLen)
	for {
		n, from, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		req := append([]byte{}, buf[:n]...)
		s.mu.Lock()
		s.requests = append(s.requests, req)
		s.mu.Unlock()
		for _, answer := range script(req) {
			if answer == nil {
				time.Sleep(100 * time.Millisecond) // a nil answer is a pause in the script
				continue
			}
			_, _ = s.conn.WriteToUDP(answer, from)
		}
	}
}

func (s *scriptedServer) received() [][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([][]byte{}, s.requests...)
}

// unsignedAnswer is an answer with a correct Response Authenticator and no
// Message-Authenticator: what a pre-2024 server sends, and what a BlastRADIUS
// forgery looks like.
func unsignedAnswer(code uint8, req, secret []byte) []byte {
	resp := make([]byte, HeaderLen)
	resp[0], resp[1] = code, req[1]
	binary.BigEndian.PutUint16(resp[2:4], HeaderLen)
	var reqAuth [AuthenticatorLen]byte
	copy(reqAuth[:], req[4:HeaderLen])
	auth := ResponseAuthenticator(code, req[1], HeaderLen, reqAuth, nil, secret)
	copy(resp[4:HeaderLen], auth[:])
	return resp
}

// signedAnswerMALast is a correctly signed answer whose Message-Authenticator
// is the LAST attribute.
func signedAnswerMALast(code uint8, req, secret []byte) []byte {
	body := []byte{AttrFilterID, 7, 'a', 'd', 'm', 'i', 'n', AttrMessageAuthenticator, 2 + AuthenticatorLen}
	body = append(body, make([]byte, AuthenticatorLen)...)
	total := HeaderLen + len(body)
	resp := make([]byte, total)
	resp[0], resp[1] = code, req[1]
	binary.BigEndian.PutUint16(resp[2:4], uint16(total))
	copy(resp[4:HeaderLen], req[4:HeaderLen])
	copy(resp[HeaderLen:], body)
	mac := hmac.New(md5.New, secret)
	mac.Write(resp)
	copy(resp[total-AuthenticatorLen:], mac.Sum(nil))
	var reqAuth [AuthenticatorLen]byte
	copy(reqAuth[:], req[4:HeaderLen])
	auth := ResponseAuthenticator(code, req[1], uint16(total), reqAuth, resp[HeaderLen:], secret)
	copy(resp[4:HeaderLen], auth[:])
	return resp
}

// requestMAVerifies reports whether the Message-Authenticator at octets 22-37
// of an encoded Access-Request is the HMAC-MD5 under secret.
func requestMAVerifies(t *testing.T, raw, secret []byte) bool {
	t.Helper()
	require.GreaterOrEqual(t, len(raw), HeaderLen+2+AuthenticatorLen)
	require.Equal(t, uint8(AttrMessageAuthenticator), raw[HeaderLen], "octet 20 is type 80")
	require.Equal(t, uint8(2+AuthenticatorLen), raw[HeaderLen+1], "octet 21 is length 18")
	zeroed := append([]byte{}, raw...)
	clear(zeroed[HeaderLen+2 : HeaderLen+2+AuthenticatorLen])
	mac := hmac.New(md5.New, secret)
	mac.Write(zeroed)
	return hmac.Equal(mac.Sum(nil), raw[HeaderLen+2:HeaderLen+2+AuthenticatorLen])
}

func blastClient(t *testing.T, servers []Server, timeout time.Duration, retries int) (*Client, *lockedLog) {
	t.Helper()
	sink := &lockedLog{}
	client, err := NewClient(ClientConfig{
		Servers: servers,
		Timeout: timeout,
		Retries: retries,
		Logger:  slog.New(slog.NewTextHandler(sink, nil)),
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	return client, sink
}

// scrapeUnsignedResponseDiscards returns the value of the unsigned response
// discard series for server, or -1 when the series is absent.
func scrapeUnsignedResponseDiscards(t *testing.T, reg *metrics.PrometheusRegistry, server string) float64 {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", http.NoBody)
	reg.Handler().ServeHTTP(recorder, request)
	prefix := `ze_radius_unsigned_response_discards_total{server="` + server + `"} `
	for line := range strings.SplitSeq(recorder.Body.String(), "\n") {
		value, found := strings.CutPrefix(line, prefix)
		if !found {
			continue
		}
		parsed, err := strconv.ParseFloat(value, 64)
		require.NoError(t, err)
		return parsed
	}
	return -1
}

// TestAccessRequestCarriesMessageAuthenticatorFirst proves AC-1.
//
// VALIDATES: an Access-Request of every credential type leaves encodeRequest
// with octets 20-21 = type 80, length 18, and octets 22-37 the HMAC-MD5 under
// the server's secret.
// PREVENTS: a PAP, CHAP or MS-CHAP request going out unsigned, which is the
// injection point of BlastRADIUS (draft-ietf-radext-deprecating-radius
// Section 4.2).
func TestAccessRequestCarriesMessageAuthenticatorFirst(t *testing.T) {
	secret := []byte("blast-secret")
	cases := map[string]Attr{
		"PAP":     {Type: AttrUserPassword, Value: []byte("password")},
		"CHAP":    {Type: AttrCHAPPassword, Value: append([]byte{1}, make([]byte, 16)...)},
		"MS-CHAP": {Type: AttrVendorSpecific, Value: []byte{0, 0, 1, 55, 25, 8, 0, 0, 0, 0, 0, 0}},
		"EAP":     {Type: AttrEAPMessage, Value: []byte{2, 1, 0, 5, 1}},
	}
	for name, credential := range cases {
		t.Run(name, func(t *testing.T) {
			buf := make([]byte, MaxPacketLen)
			pkt := &Packet{Code: CodeAccessRequest, Identifier: 9, Attrs: []Attr{
				{Type: AttrUserName, Value: AttrString("alice")}, credential,
			}}
			n, _, err := encodeRequest(pkt, secret, buf)
			require.NoError(t, err)
			assert.Equal(t, n, int(binary.BigEndian.Uint16(buf[2:4])), "Length counts the added attribute")
			assert.True(t, requestMAVerifies(t, buf[:n], secret))
			assert.False(t, requestMAVerifies(t, buf[:n], []byte("other-secret")))
			decoded, err := Decode(buf[:n])
			require.NoError(t, err)
			assert.Len(t, decoded.FindAllAttr(AttrMessageAuthenticator), 1, "exactly one Message-Authenticator")
		})
	}
}

// TestAccessRequestRefusesCallerMessageAuthenticator proves AC-2.
//
// VALIDATES: an Access-Request handed to Exchange with a Message-Authenticator
// already in it is refused, naming RFC 2869 Section 5.14, and nothing is sent.
// PREVENTS: a packet with two Message-Authenticators.
func TestAccessRequestRefusesCallerMessageAuthenticator(t *testing.T) {
	secret := []byte("blast-secret")
	srv := newScriptedServer(t, func(req []byte) [][]byte { return [][]byte{buildReplyResponse(CodeAccessAccept, req, secret, nil)} })
	client, _ := blastClient(t, nil, 200*time.Millisecond, 1)

	pkt := accessRequest(t, client.NextID())
	pkt.Attrs = append(pkt.Attrs, Attr{Type: AttrMessageAuthenticator, Value: make([]byte, AuthenticatorLen)})
	_, err := client.Exchange(context.Background(), pkt, Server{Address: srv.addr, SharedKey: secret})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RFC 2869 Section 5.14")
	assert.Contains(t, err.Error(), "more than one")
	time.Sleep(50 * time.Millisecond)
	assert.Empty(t, srv.received(), "nothing is sent")
}

// TestFailoverSignsWithEachServerSecret proves AC-4.
//
// VALIDATES: server A's retransmissions are byte-identical and signed under
// A's secret; server B gets a Message-Authenticator that verifies under B's
// secret and not A's.
// PREVENTS: accel-ppp's failover defect (09d7b71), a request re-sent to the
// next server still signed with the first server's secret.
func TestFailoverSignsWithEachServerSecret(t *testing.T) {
	secretA, secretB := []byte("secret-of-a"), []byte("secret-of-b")
	serverA := newScriptedServer(t, func([]byte) [][]byte { return nil })
	serverB := newScriptedServer(t, func(req []byte) [][]byte {
		return [][]byte{buildReplyResponse(CodeAccessAccept, req, secretB, nil)}
	})
	client, _ := blastClient(t, []Server{
		{Address: serverA.addr, SharedKey: secretA},
		{Address: serverB.addr, SharedKey: secretB},
	}, 100*time.Millisecond, 2)

	resp, err := client.SendToServers(context.Background(), accessRequest(t, 0))
	require.NoError(t, err)
	assert.Equal(t, uint8(CodeAccessAccept), resp.Code)

	fromA := serverA.received()
	require.Len(t, fromA, 2)
	assert.Equal(t, fromA[0], fromA[1], "a retransmission to the same server is byte-identical")
	assert.True(t, requestMAVerifies(t, fromA[0], secretA))
	fromB := serverB.received()
	require.Len(t, fromB, 1)
	assert.True(t, requestMAVerifies(t, fromB[0], secretB))
	assert.False(t, requestMAVerifies(t, fromB[0], secretA))
}

// TestUnsignedAcceptDiscardedByDefault proves AC-5 and AC-14.
//
// VALIDATES: a forged Access-Accept with a correct Response Authenticator and
// no Message-Authenticator is discarded when the server requires one: the
// exchange fails with ErrUnsignedResponse, the counter rises, one warning
// names the server and the leaf.
// PREVENTS: BlastRADIUS (CVE-2024-3596), the forged Accept being taken as a
// login.
func TestUnsignedAcceptDiscardedByDefault(t *testing.T) {
	reg := bindTestRadiusMetrics(t)
	secret := []byte("blast-secret")
	srv := newScriptedServer(t, func(req []byte) [][]byte { return [][]byte{unsignedAnswer(CodeAccessAccept, req, secret)} })
	client, sink := blastClient(t, nil, 100*time.Millisecond, 1)

	_, err := client.Exchange(context.Background(), accessRequest(t, client.NextID()), Server{
		Address: srv.addr, SharedKey: secret,
		ConfigPath: "system authentication radius server 127.0.0.1",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnsignedResponse))
	assert.Contains(t, err.Error(), srv.addr)
	assert.Contains(t, err.Error(), "no Message-Authenticator")
	assert.Contains(t, err.Error(), "'set system authentication radius server 127.0.0.1 require-message-authenticator false'")

	assert.Equal(t, float64(1), scrapeUnsignedResponseDiscards(t, reg, srv.addr))
	log := sink.String()
	assert.Equal(t, 1, strings.Count(log, "without Message-Authenticator"))
	assert.Contains(t, log, "server="+srv.addr)
	assert.Contains(t, log, "require-message-authenticator")
}

// TestUnsignedResponseCodesDiscarded proves AC-6.
//
// VALIDATES: an unsigned Access-Reject, Access-Challenge or Protocol-Error is
// discarded the same way an unsigned Accept is.
// PREVENTS: a check that judges only the Accept.
func TestUnsignedResponseCodesDiscarded(t *testing.T) {
	secret := []byte("blast-secret")
	for name, code := range map[string]uint8{
		"Access-Reject": CodeAccessReject, "Access-Challenge": CodeAccessChallenge, "Protocol-Error": codeProtocolError,
	} {
		t.Run(name, func(t *testing.T) {
			srv := newScriptedServer(t, func(req []byte) [][]byte { return [][]byte{unsignedAnswer(code, req, secret)} })
			client, _ := blastClient(t, nil, 100*time.Millisecond, 1)
			_, err := client.Exchange(context.Background(), accessRequest(t, client.NextID()),
				Server{Address: srv.addr, SharedKey: secret})
			assert.True(t, errors.Is(err, ErrUnsignedResponse), "got %v", err)
		})
	}
}

// TestPresenceCheckRunsBeforeResponseAuth proves the AC-5 ordering.
//
// VALIDATES: an unsigned datagram with a WRONG Response Authenticator is
// judged unsigned, so the presence check ran before VerifyResponseAuth.
// PREVENTS: the order draft-ietf-radext-deprecating-radius Section 4.6
// forbids, verifying first.
func TestPresenceCheckRunsBeforeResponseAuth(t *testing.T) {
	secret := []byte("blast-secret")
	srv := newScriptedServer(t, func(req []byte) [][]byte {
		answer := unsignedAnswer(CodeAccessAccept, req, secret)
		answer[4] ^= 0xff
		return [][]byte{answer}
	})
	client, sink := blastClient(t, nil, 100*time.Millisecond, 1)
	_, err := client.Exchange(context.Background(), accessRequest(t, client.NextID()),
		Server{Address: srv.addr, SharedKey: secret})
	assert.True(t, errors.Is(err, ErrUnsignedResponse), "got %v", err)
	assert.NotContains(t, sink.String(), "bad response authenticator")
}

// TestDiscardDoesNotMoveRetransmitTimer proves AC-7.
//
// VALIDATES: a forged unsigned Accept followed by the real signed answer
// inside the timeout ends in the real answer, with no retransmission.
// PREVENTS: a discard that restarts or ends the wait.
func TestDiscardDoesNotMoveRetransmitTimer(t *testing.T) {
	secret := []byte("blast-secret")
	srv := newScriptedServer(t, func(req []byte) [][]byte {
		return [][]byte{
			unsignedAnswer(CodeAccessAccept, req, secret),
			nil, // 100 ms pause
			buildReplyResponse(CodeAccessReject, req, secret, nil),
		}
	})
	client, _ := blastClient(t, nil, 400*time.Millisecond, 2)
	start := time.Now()
	resp, err := client.Exchange(context.Background(), accessRequest(t, client.NextID()),
		Server{Address: srv.addr, SharedKey: secret})
	require.NoError(t, err)
	assert.Equal(t, uint8(CodeAccessReject), resp.Code, "the real answer, not the forgery")
	assert.GreaterOrEqual(t, time.Since(start), 100*time.Millisecond)
	assert.Len(t, srv.received(), 1, "the discard caused no early retransmission")
}

// TestUnsignedAcceptAcceptedWhenNotRequired proves AC-8.
//
// VALIDATES: with the flag false an unsigned non-EAP Accept is accepted.
// PREVENTS: a legacy server becoming unusable for an operator who opted in.
func TestUnsignedAcceptAcceptedWhenNotRequired(t *testing.T) {
	secret := []byte("blast-secret")
	srv := newScriptedServer(t, func(req []byte) [][]byte { return [][]byte{unsignedAnswer(CodeAccessAccept, req, secret)} })
	client, _ := blastClient(t, nil, 200*time.Millisecond, 1)
	resp, err := client.Exchange(context.Background(), accessRequest(t, client.NextID()),
		Server{Address: srv.addr, SharedKey: secret, AllowUnsignedResponse: true})
	require.NoError(t, err)
	assert.Equal(t, uint8(CodeAccessAccept), resp.Code)
}

// TestMessageAuthenticatorLocationIgnored proves AC-10.
//
// VALIDATES: a signed answer whose Message-Authenticator is the last attribute
// is accepted.
// PREVENTS: a discard on location, which draft-ietf-radext-deprecating-radius
// Section 4.6 forbids.
func TestMessageAuthenticatorLocationIgnored(t *testing.T) {
	secret := []byte("blast-secret")
	srv := newScriptedServer(t, func(req []byte) [][]byte { return [][]byte{signedAnswerMALast(CodeAccessAccept, req, secret)} })
	client, _ := blastClient(t, nil, 200*time.Millisecond, 1)
	resp, err := client.Exchange(context.Background(), accessRequest(t, client.NextID()),
		Server{Address: srv.addr, SharedKey: secret})
	require.NoError(t, err)
	assert.Equal(t, uint8(CodeAccessAccept), resp.Code)
}

// TestAccountingResponseNotJudgedByRequireFlag proves AC-12.
//
// VALIDATES: an Accounting-Response with no Message-Authenticator is accepted
// even when the server's flag is true.
// PREVENTS: the flag being consulted for another request type, which
// draft-ietf-radext-deprecating-radius Section 4.6 forbids.
func TestAccountingResponseNotJudgedByRequireFlag(t *testing.T) {
	secret := []byte("blast-secret")
	srv := newScriptedServer(t, func(req []byte) [][]byte { return [][]byte{acctResponse(req, secret)} })
	client, _ := blastClient(t, nil, 200*time.Millisecond, 1)
	pkt := &Packet{Code: CodeAccountingReq, Identifier: client.NextID(), Attrs: []Attr{
		{Type: AttrUserName, Value: AttrString("alice")},
		{Type: AttrAcctStatusType, Value: AttrUint32(1)},
	}}
	resp, err := client.Exchange(context.Background(), pkt,
		Server{Address: srv.addr, SharedKey: secret})
	require.NoError(t, err)
	assert.Equal(t, uint8(CodeAccountingResp), resp.Code)
}

// TestUnsignedDiscardLogIsRateLimited proves AC-13.
//
// VALIDATES: 1000 unsigned answers from one server in under 60 s write one
// warning and count 1000.
// PREVENTS: a log flood under attack, and a counter that follows the limiter.
func TestUnsignedDiscardLogIsRateLimited(t *testing.T) {
	reg := bindTestRadiusMetrics(t)
	secret := []byte("blast-secret")
	client, sink := blastClient(t, nil, time.Second, 1)
	key := responseKey{server: "192.0.2.7:1812", id: 3}
	var reqAuth [AuthenticatorLen]byte
	waiter, err := client.registerWaiter(key, reqAuth, secret, true)
	require.NoError(t, err)
	defer client.unregisterWaiter(key, waiter)

	req := make([]byte, HeaderLen)
	req[1] = 3
	answer := unsignedAnswer(CodeAccessAccept, req, secret)
	for range 1000 {
		client.dispatchResponse(key, answer)
	}
	assert.Equal(t, 1, strings.Count(sink.String(), "without Message-Authenticator"))
	assert.Equal(t, float64(1000), scrapeUnsignedResponseDiscards(t, reg, key.server))
	assert.True(t, waiter.unsignedDiscarded.Load())
	assert.Empty(t, waiter.ch, "no discarded answer reached the waiter")
}
