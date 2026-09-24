// Design: (none -- new TACACS+ component)
// Detail: authenticator.go -- Authenticate and handlePass, the reply readers
// Detail: authorizer.go -- authorize, the authorization decision
//
// VALIDATES: RFC 8907 Section 4.1 (a zero-length field is read as absent),
// Section 5.4.3 (RESTART is processed as FAIL by a client that does not
// implement it), Section 6.2 (FAIL denies) and Section 10.2 (the deprecated
// FOLLOW redirection is never followed).
// PREVENTS: a RESTART that lets the AAA chain fall through to another
// backend, a FAIL that consults local policy, or a client that connects to
// whatever address a FOLLOW reply names.
//
// The tests live in their own file because client_test.go carries
// `RFC requirement:` tags and is closed to edits (ai/rules/testing.md).

package tacacs

import (
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/aaa"
)

// authenReply builds an authentication REPLY body with the given status,
// server_msg and data.
func authenReply(status uint8, serverMsg string, data []byte) func(PacketHeader, []byte) []byte {
	return func(_ PacketHeader, _ []byte) []byte {
		body := make([]byte, 6+len(serverMsg)+len(data))
		body[0] = status
		binary.BigEndian.PutUint16(body[2:4], uint16(len(serverMsg)))
		binary.BigEndian.PutUint16(body[4:6], uint16(len(data)))
		copy(body[6:], serverMsg)
		copy(body[6+len(serverMsg):], data)
		return body
	}
}

var statusKey = []byte("status-test-key")

func newStatusAuthenticator(t *testing.T, replyFn func(PacketHeader, []byte) []byte, privLvlMap map[int][]string) *tacacsAuthenticator {
	t.Helper()
	srv := newTestServer(t, statusKey, replyFn)
	t.Cleanup(srv.close)
	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: srv.addr(), Key: statusKey}},
		Timeout: 5 * time.Second,
	})
	t.Cleanup(client.Close)
	return newTacacsAuthenticator(client, privLvlMap, nil)
}

var aliceRequest = aaa.AuthRequest{Username: "alice", Password: "hunter2", RemoteAddr: "192.0.2.1"}

// RFC requirement: RFC8907-4.1-1 positive — an authentication reply whose data_len is zero exposes no data bytes to the client.
func TestRFC8907ZeroLengthDataIsReadAsAbsent(t *testing.T) {
	auth := newStatusAuthenticator(t, authenReply(AuthenStatusPass, "", nil), nil)
	reply, err := auth.client.Authenticate("alice", "secret", "ssh", "192.0.2.1")
	require.NoError(t, err)
	require.Empty(t, reply.Data)
}

// RFC requirement: RFC8907-4.1-1 negative — a one-byte authentication data field containing zero is retained, not confused with an absent field.
func TestRFC8907PresentZeroByteIsNotReadAsAbsent(t *testing.T) {
	auth := newStatusAuthenticator(t, authenReply(AuthenStatusPass, "", []byte{0x00}), nil)
	reply, err := auth.client.Authenticate("alice", "secret", "ssh", "192.0.2.1")
	require.NoError(t, err)
	require.Equal(t, []byte{0x00}, reply.Data)
}

// RFC requirement: RFC8907-5.4.3-1 positive — a RESTART reply is processed as FAIL: the result is rejected with aaa.ErrAuthRejected, which stops the AAA chain.
func TestRFC8907RestartIsProcessedAsFail(t *testing.T) {
	auth := newStatusAuthenticator(t, authenReply(AuthenStatusRestart, "", nil), map[int][]string{1: {"ops"}})
	result, err := auth.Authenticate(aliceRequest)
	require.ErrorIs(t, err, aaa.ErrAuthRejected)
	require.False(t, result.Authenticated)
	require.Equal(t, backendName, result.Source)
}

// RFC requirement: RFC8907-5.4.3-1 negative — an ERROR reply is not processed as FAIL: it returns an infrastructure error that is not aaa.ErrAuthRejected, so the chain continues.
func TestRFC8907ErrorIsNotProcessedAsFail(t *testing.T) {
	auth := newStatusAuthenticator(t, authenReply(AuthenStatusError, "backend down", nil), map[int][]string{1: {"ops"}})
	result, err := auth.Authenticate(aliceRequest)
	require.Error(t, err)
	require.False(t, errors.Is(err, aaa.ErrAuthRejected), "ERROR must not stop the chain the way FAIL does")
	require.False(t, result.Authenticated)
}

// allowAll is a local authorizer that permits everything and counts calls.
type allowAll struct{ calls int }

func (a *allowAll) Authorize(_, _, _ string, _ bool) bool {
	a.calls++
	return true
}

func newStatusAuthorizer(t *testing.T, status uint8, local *allowAll) *tacacsAuthorizer {
	t.Helper()
	srv := newTestServer(t, statusKey, authorReply(status))
	t.Cleanup(srv.close)
	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: srv.addr(), Key: statusKey}},
		Timeout: 5 * time.Second,
	})
	t.Cleanup(client.Close)
	return newTacacsAuthorizer(client, local)
}

// RFC requirement: RFC8907-6.2-3 positive — a FAIL reply denies the command, and denies it without consulting the local policy that would have allowed it.
func TestRFC8907AuthorizationFailDenies(t *testing.T) {
	local := &allowAll{}
	authz := newStatusAuthorizer(t, AuthorStatusFail, local)
	require.False(t, authz.Authorize("alice", "192.0.2.1", "show version", true))
	require.Zero(t, local.calls, "a FAIL must not fall back to local policy")
}

// RFC requirement: RFC8907-6.2-3 negative — a PASS_ADD reply is not denied: the command is allowed.
func TestRFC8907AuthorizationPassAddIsNotDenied(t *testing.T) {
	authz := newStatusAuthorizer(t, AuthorStatusPassAdd, &allowAll{})
	require.True(t, authz.Authorize("alice", "192.0.2.1", "show version", true))
}

// RFC requirement: RFC8907-x-2 positive — FOLLOW is a terminal authentication rejection (aaa.ErrAuthRejected), so local fallback cannot grant the login.
func TestRFC8907FollowIsNotAuthenticated(t *testing.T) {
	auth := newStatusAuthenticator(t, authenReply(AuthenStatusFollow, "127.0.0.1:49", nil), map[int][]string{1: {"ops"}})
	result, err := auth.Authenticate(aliceRequest)
	require.ErrorIs(t, err, aaa.ErrAuthRejected)
	require.False(t, result.Authenticated)
}

// RFC requirement: RFC8907-x-2 positive — authorization FOLLOW denies the command without consulting permissive local fallback.
func TestRFC8907AuthorizationFollowDeniesWithoutFallback(t *testing.T) {
	local := &allowAll{}
	authz := newStatusAuthorizer(t, AuthorStatusFollow, local)
	require.False(t, authz.Authorize("alice", "192.0.2.1", "show version", true))
	require.Zero(t, local.calls)
}

// RFC requirement: RFC8907-x-2 negative — the redirection target a FOLLOW reply names receives no connection from the client.
func TestRFC8907FollowTargetIsNeverContacted(t *testing.T) {
	target := listenTCP(t)
	t.Cleanup(func() { closeIgnore(target) })
	contacted := make(chan struct{}, 1)
	go func() {
		conn, err := target.Accept()
		if err != nil {
			return
		}
		closeIgnore(conn)
		contacted <- struct{}{}
	}()

	auth := newStatusAuthenticator(t, authenReply(AuthenStatusFollow, target.Addr().String(), nil), map[int][]string{1: {"ops"}})
	_, err := auth.Authenticate(aliceRequest)
	require.Error(t, err)

	select {
	case <-contacted:
		t.Fatal("the client connected to the FOLLOW target")
	case <-time.After(200 * time.Millisecond):
	}
}
