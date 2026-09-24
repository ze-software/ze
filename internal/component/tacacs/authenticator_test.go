package tacacs

import (
	"encoding/binary"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
)

// profileReplies separates PAP authentication from the shell privilege policy.
func profileReplies(privLvl uint8) func(PacketHeader, []byte) []byte {
	return func(header PacketHeader, body []byte) []byte {
		if header.Type == 0x02 {
			return authorReplyArgs(AuthorStatusPassAdd, []string{
				"priv-lvl=" + strconv.Itoa(int(privLvl)),
			})(header, body)
		}
		return passReply()(header, body)
	}
}

// newProfileServer serves the two independent sessions needed for login.
// Cleanup closes the listener and MUST wait for its bounded worker to exit.
func newProfileServer(t *testing.T, key []byte, replyFn func(PacketHeader, []byte) []byte) *testTacacsServer {
	t.Helper()
	srv := &testTacacsServer{listener: listenTCP(t), key: key, replyFn: replyFn}
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		for range 2 {
			srv.serve()
		}
	}()
	t.Cleanup(func() {
		srv.close()
		// MUST join the worker after closing its listener.
		<-stopped
	})
	return srv
}

// VALIDATES: AC-1 + AC-6 -- TACACS+ PASS with priv-lvl 15 maps to admin profile.
// PREVENTS: successful auth not producing correct profiles.
func TestTacacsAuthenticatorPass(t *testing.T) {
	key := []byte("test-key")
	srv := newProfileServer(t, key, profileReplies(15))
	defer srv.close()

	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: srv.addr(), Key: key}},
		Timeout: 2 * time.Second,
	})

	privMap := map[int][]string{
		15: {"admin"},
		1:  {"read-only"},
	}

	auth := newTacacsAuthenticator(client, privMap, nil)
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
	srv := newProfileServer(t, key, profileReplies(1))
	defer srv.close()

	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: srv.addr(), Key: key}},
		Timeout: 2 * time.Second,
	})

	privMap := map[int][]string{
		15: {"admin"},
		1:  {"read-only"},
	}

	auth := newTacacsAuthenticator(client, privMap, nil)
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

	auth := newTacacsAuthenticator(client, map[int][]string{15: {"admin"}}, nil)
	result, err := auth.Authenticate(authz.AuthRequest{Username: "admin", Password: "wrong"})

	assert.ErrorIs(t, err, authz.ErrAuthRejected)
	assert.False(t, result.Authenticated)
	assert.Equal(t, "tacacs", result.Source)
}

// VALIDATES: AC-18 -- PASS with unmapped priv-lvl rejects.
// PREVENTS: unmapped priv-lvl granting admin access.
func TestTacacsAuthenticatorUnmappedPrivLvl(t *testing.T) {
	key := []byte("test-key")
	srv := newProfileServer(t, key, profileReplies(5)) // priv-lvl 5 not in map
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

	auth := newTacacsAuthenticator(client, privMap, nil)
	result, err := auth.Authenticate(authz.AuthRequest{Username: "user", Password: "pass"})

	assert.ErrorIs(t, err, authz.ErrAuthRejected)
	assert.False(t, result.Authenticated)
}

// TestTacacsAuthenticatorProfileMappingShapes covers the three shapes a priv-lvl
// mapping can take, so the deny cases are read against the allow case rather
// than in isolation. The empty-list row is the regression: a level present in
// the map with no profile names must deny exactly like an absent level.
//
// VALIDATES: AC-1 -- a priv-lvl mapped to an EMPTY profile list denies access,
// and an authenticated result never carries an empty profile set.
// PREVENTS: `tacacs-profile { level 15; }` with no profile leaf-list entries
//
//	authenticating successfully with zero profiles. The result-scoped authorizer
//	would fail closed, but this test enforces the primary guard: authentication
//	rejects the login before authorization runs. Before that guard, an empty
//	mapping granted admin, the opposite of the operator's intent.
func TestTacacsAuthenticatorProfileMappingShapes(t *testing.T) {
	privMap := map[int][]string{
		15: {"admin"},
		1:  {"read-only"},
		9:  {},  // present but empty: operator wrote `tacacs-profile { level 9; }`
		7:  nil, // present but nil: every member deactivated (Tree.GetSlice)
		// 5 intentionally absent: unmapped.
	}

	tests := []struct {
		name         string
		privLvl      uint8
		wantProfiles []string // non-nil => expect success with these profiles
	}{
		{"non-empty list authenticates", 15, []string{"admin"}},
		{"non-empty list authenticates (read-only)", 1, []string{"read-only"}},
		{"empty list denies", 9, nil},
		{"nil list denies", 7, nil},
		{"unmapped level denies", 5, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := []byte("test-key")
			srv := newProfileServer(t, key, profileReplies(tt.privLvl))
			defer srv.close()

			client := NewTacacsClient(TacacsClientConfig{
				Servers: []TacacsServer{{Address: srv.addr(), Key: key}},
				Timeout: 2 * time.Second,
			})

			auth := newTacacsAuthenticator(client, privMap, nil)
			result, err := auth.Authenticate(authz.AuthRequest{Username: "user", Password: "pass"})

			if tt.wantProfiles != nil {
				require.NoError(t, err)
				assert.True(t, result.Authenticated)
				assert.Equal(t, tt.wantProfiles, result.Profiles)
				return
			}

			assert.ErrorIs(t, err, authz.ErrAuthRejected,
				"priv-lvl %d resolves to no profiles and must be rejected", tt.privLvl)
			assert.False(t, result.Authenticated,
				"priv-lvl %d must not authenticate with an empty profile set", tt.privLvl)
			assert.Empty(t, result.Profiles)
		})
	}
}

// TestTacacsAuthenticatorDropsReservedProfile is defense-in-depth for the
// spec-fixit-authz-admin-fallthrough review (finding 1): even though the priv-lvl
// map is config-derived and ValidateAuthzConfig rejects reserved references, the
// authenticator strips any reserved name so a priv-level can never resolve to a
// reserved identity. A level mapped only to the reserved name resolves to nothing
// and is rejected; a mixed mapping keeps only the real name.
//
// VALIDATES: a reserved profile name in the priv-lvl map is dropped, never emitted.
// PREVENTS: a reserved-identity spoof surviving to AuthResult.Profiles.
func TestTacacsAuthenticatorDropsReservedProfile(t *testing.T) {
	privMap := map[int][]string{
		15: {aaa.ReservedRecoveryProfile},              // reserved-only -> denied
		14: {aaa.ReservedRecoveryProfile, "read-only"}, // mixed -> only read-only survives
	}
	key := []byte("test-key")

	// Reserved-only level: stripped to nothing, so the login is rejected.
	srv := newProfileServer(t, key, profileReplies(15))
	defer srv.close()
	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: srv.addr(), Key: key}},
		Timeout: 2 * time.Second,
	})
	res, err := newTacacsAuthenticator(client, privMap, nil).
		Authenticate(authz.AuthRequest{Username: "u", Password: "p"})
	assert.ErrorIs(t, err, authz.ErrAuthRejected, "reserved-only priv-lvl must be rejected")
	assert.False(t, res.Authenticated)
	assert.NotContains(t, res.Profiles, aaa.ReservedRecoveryProfile)

	// Mixed level: the real profile survives, the reserved name is dropped.
	srv2 := newProfileServer(t, key, profileReplies(14))
	defer srv2.close()
	client2 := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: srv2.addr(), Key: key}},
		Timeout: 2 * time.Second,
	})
	res2, err2 := newTacacsAuthenticator(client2, privMap, nil).
		Authenticate(authz.AuthRequest{Username: "u2", Password: "p"})
	require.NoError(t, err2)
	assert.True(t, res2.Authenticated)
	assert.Equal(t, []string{"read-only"}, res2.Profiles)
	assert.NotContains(t, res2.Profiles, aaa.ReservedRecoveryProfile)
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

	auth := newTacacsAuthenticator(client, map[int][]string{15: {"admin"}}, nil)
	_, err := auth.Authenticate(authz.AuthRequest{Username: "admin", Password: "pass"})

	// ERROR should be a non-ErrAuthRejected error (chain tries next backend).
	assert.Error(t, err)
	assert.NotErrorIs(t, err, authz.ErrAuthRejected)
}

// VALIDATES: AC-3 -- all servers unreachable returns non-rejection error.
// PREVENTS: connection failure stopping auth chain.
func TestTacacsAuthenticatorConnectionFailure(t *testing.T) {
	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: "127.0.0.1:1", Key: []byte("key")}},
		Timeout: 200 * time.Millisecond,
	})

	auth := newTacacsAuthenticator(client, map[int][]string{15: {"admin"}}, nil)
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

	srv := newProfileServer(t, key, func(header PacketHeader, body []byte) []byte {
		if header.Type == 0x01 {
			off := 8 + int(body[4]) + int(body[5])
			seenRemoteAddr = string(body[off : off+int(body[6])])
		}
		return profileReplies(15)(header, body)
	})
	defer srv.close()

	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: srv.addr(), Key: key}},
		Timeout: 2 * time.Second,
	})

	auth := newTacacsAuthenticator(client, map[int][]string{15: {"admin"}}, nil)
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
