package tacacs

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/authz"
)

// replyWithPrivLvl returns a PASS reply with the given priv-lvl in the data field.
func replyWithPrivLvl(privLvl uint8) func(PacketHeader, []byte) []byte {
	return func(_ PacketHeader, _ []byte) []byte {
		body := make([]byte, 7)
		body[0] = AuthenStatusPass // PASS
		body[1] = 0x00
		binary.BigEndian.PutUint16(body[2:4], 0) // no server_msg
		binary.BigEndian.PutUint16(body[4:6], 1) // data_len = 1
		body[6] = privLvl
		return body
	}
}

// VALIDATES: AC-1 + AC-6 -- TACACS+ PASS with priv-lvl 15 maps to admin profile.
// PREVENTS: successful auth not producing correct profiles.
func TestTacacsAuthenticatorPass(t *testing.T) {
	key := []byte("test-key")
	srv := newTestServer(t, key, replyWithPrivLvl(15))
	defer srv.close()

	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: srv.addr(), Key: key}},
		Timeout: 2 * time.Second,
	})

	privMap := map[int][]string{
		15: {"admin"},
		1:  {"read-only"},
	}

	auth := NewTacacsAuthenticator(client, privMap, nil)
	result, err := auth.Authenticate(authz.AuthRequest{Username: "admin", Password: "secret"})

	require.NoError(t, err)
	assert.True(t, result.Authenticated)
	assert.Equal(t, "tacacs", result.Source)
	assert.Equal(t, []string{"admin"}, result.Profiles)
}

// VALIDATES: AC-7 -- priv-lvl 1 maps to read-only profile.
// PREVENTS: wrong priv-lvl mapping.
func TestTacacsAuthenticatorPrivLvl1(t *testing.T) {
	key := []byte("test-key")
	srv := newTestServer(t, key, replyWithPrivLvl(1))
	defer srv.close()

	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: srv.addr(), Key: key}},
		Timeout: 2 * time.Second,
	})

	privMap := map[int][]string{
		15: {"admin"},
		1:  {"read-only"},
	}

	auth := NewTacacsAuthenticator(client, privMap, nil)
	result, err := auth.Authenticate(authz.AuthRequest{Username: "user", Password: "pass"})

	require.NoError(t, err)
	assert.True(t, result.Authenticated)
	assert.Equal(t, []string{"read-only"}, result.Profiles)
}

// VALIDATES: AC-2 -- TACACS+ FAIL returns ErrAuthRejected (chain stops).
// PREVENTS: FAIL falling through to local auth.
func TestTacacsAuthenticatorFail(t *testing.T) {
	key := []byte("test-key")
	srv := newTestServer(t, key, failReply())
	defer srv.close()

	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: srv.addr(), Key: key}},
		Timeout: 2 * time.Second,
	})

	auth := NewTacacsAuthenticator(client, map[int][]string{15: {"admin"}}, nil)
	result, err := auth.Authenticate(authz.AuthRequest{Username: "admin", Password: "wrong"})

	assert.ErrorIs(t, err, authz.ErrAuthRejected)
	assert.False(t, result.Authenticated)
	assert.Equal(t, "tacacs", result.Source)
}

// VALIDATES: AC-18 -- PASS with unmapped priv-lvl rejects.
// PREVENTS: unmapped priv-lvl granting admin access.
func TestTacacsAuthenticatorUnmappedPrivLvl(t *testing.T) {
	key := []byte("test-key")
	srv := newTestServer(t, key, replyWithPrivLvl(5)) // priv-lvl 5 not in map
	defer srv.close()

	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: srv.addr(), Key: key}},
		Timeout: 2 * time.Second,
	})

	privMap := map[int][]string{
		15: {"admin"},
		1:  {"read-only"},
		// 5 intentionally missing
	}

	auth := NewTacacsAuthenticator(client, privMap, nil)
	result, err := auth.Authenticate(authz.AuthRequest{Username: "user", Password: "pass"})

	assert.ErrorIs(t, err, authz.ErrAuthRejected)
	assert.False(t, result.Authenticated)
}

// VALIDATES: AC-15 -- ERROR status treated as infrastructure failure.
// PREVENTS: ERROR status blocking auth chain.
func TestTacacsAuthenticatorErrorStatus(t *testing.T) {
	key := []byte("test-key")
	srv := newTestServer(t, key, func(_ PacketHeader, _ []byte) []byte {
		msg := "internal error"
		body := make([]byte, 6+len(msg))
		body[0] = AuthenStatusError
		body[1] = 0x00
		binary.BigEndian.PutUint16(body[2:4], uint16(len(msg)))
		binary.BigEndian.PutUint16(body[4:6], 0)
		copy(body[6:], msg)
		return body
	})
	defer srv.close()

	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: srv.addr(), Key: key}},
		Timeout: 2 * time.Second,
	})

	auth := NewTacacsAuthenticator(client, map[int][]string{15: {"admin"}}, nil)
	_, err := auth.Authenticate(authz.AuthRequest{Username: "admin", Password: "pass"})

	// ERROR should be a non-ErrAuthRejected error (chain tries next backend).
	assert.Error(t, err)
	assert.NotErrorIs(t, err, authz.ErrAuthRejected)
	assert.Contains(t, err.Error(), "internal error")
}

// VALIDATES: AC-3 -- all servers unreachable returns non-rejection error.
// PREVENTS: connection failure stopping auth chain.
func TestTacacsAuthenticatorConnectionFailure(t *testing.T) {
	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: "127.0.0.1:1", Key: []byte("key")}},
		Timeout: 200 * time.Millisecond,
	})

	auth := NewTacacsAuthenticator(client, map[int][]string{15: {"admin"}}, nil)
	_, err := auth.Authenticate(authz.AuthRequest{Username: "admin", Password: "pass"})

	// Connection failure should be a non-ErrAuthRejected error.
	assert.Error(t, err)
	assert.NotErrorIs(t, err, authz.ErrAuthRejected)
}

// VALIDATES: TACACS auth forwards the SSH remote address into the authen START packet.
// PREVENTS: rem_addr staying empty even after the AAA request object is widened.
func TestTacacsAuthenticatorUsesRemoteAddr(t *testing.T) {
	key := []byte("test-key")
	var seenRemoteAddr string

	srv := newTestServer(t, key, func(_ PacketHeader, body []byte) []byte {
		off := 8 + int(body[4]) + int(body[5])
		seenRemoteAddr = string(body[off : off+int(body[6])])
		return replyWithPrivLvl(15)(PacketHeader{}, nil)
	})
	defer srv.close()

	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: srv.addr(), Key: key}},
		Timeout: 2 * time.Second,
	})

	auth := NewTacacsAuthenticator(client, map[int][]string{15: {"admin"}}, nil)
	result, err := auth.Authenticate(authz.AuthRequest{
		Username:   "admin",
		Password:   "secret",
		RemoteAddr: "203.0.113.5:2222",
		Service:    "ssh",
	})

	require.NoError(t, err)
	assert.True(t, result.Authenticated)
	assert.Equal(t, "203.0.113.5:2222", seenRemoteAddr)
}
