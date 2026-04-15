package tacacs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// fakeLocalAuthz is a test double for local RBAC (aaa.Authorizer shape).
type fakeLocalAuthz struct {
	allow bool
}

func (f *fakeLocalAuthz) Authorize(_, _ string, _ bool) bool {
	return f.allow
}

// authorReply returns a replyFn that produces an AuthorResponse with the given status.
func authorReply(status uint8) func(PacketHeader, []byte) []byte {
	return func(_ PacketHeader, _ []byte) []byte {
		// RFC 8907 Section 6.2: status(1) + arg_cnt(1) + server_msg_len(2) + data_len(2).
		return []byte{status, 0, 0, 0, 0, 0}
	}
}

// VALIDATES: AC-9 -- TACACS+ authorization PASS_ADD allows the command.
// PREVENTS: authorized commands being blocked.
func TestTacacsAuthorizerPassAdd(t *testing.T) {
	key := []byte("secret")
	srv := newTestServer(t, key, authorReply(AuthorStatusPassAdd))
	defer srv.close()

	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: srv.addr(), Key: key}},
		Timeout: 2 * time.Second,
	})
	local := &fakeLocalAuthz{allow: false}
	authorizer := NewTacacsAuthorizer(client, local, nil)

	result := authorizer.Authorize("admin", "show version", true)
	assert.True(t, result, "PASS_ADD should allow")
}

// VALIDATES: AC-10 -- TACACS+ authorization FAIL blocks the command.
// PREVENTS: denied commands proceeding.
func TestTacacsAuthorizerFail(t *testing.T) {
	key := []byte("secret")
	srv := newTestServer(t, key, authorReply(AuthorStatusFail))
	defer srv.close()

	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: srv.addr(), Key: key}},
		Timeout: 2 * time.Second,
	})
	local := &fakeLocalAuthz{allow: true}
	authorizer := NewTacacsAuthorizer(client, local, nil)

	result := authorizer.Authorize("admin", "restart", true)
	assert.False(t, result, "FAIL should deny")
}

// VALIDATES: AC-9/AC-10 -- TACACS+ server unreachable falls back to local.
// PREVENTS: commands blocked when TACACS+ authorization server is down.
func TestTacacsAuthorizerFallbackToLocal(t *testing.T) {
	client := NewTacacsClient(TacacsClientConfig{
		Timeout: 200 * time.Millisecond,
	})
	local := &fakeLocalAuthz{allow: true}
	authorizer := NewTacacsAuthorizer(client, local, nil)

	result := authorizer.Authorize("admin", "show version", true)
	assert.True(t, result, "should fall back to local allow")
}

// VALIDATES: PASS_REPL is also treated as Allow.
// PREVENTS: PASS_REPL incorrectly denied.
func TestTacacsAuthorizerPassRepl(t *testing.T) {
	key := []byte("secret")
	srv := newTestServer(t, key, authorReply(AuthorStatusPassRepl))
	defer srv.close()

	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: srv.addr(), Key: key}},
		Timeout: 2 * time.Second,
	})
	local := &fakeLocalAuthz{allow: false}
	authorizer := NewTacacsAuthorizer(client, local, nil)

	result := authorizer.Authorize("admin", "show version", true)
	assert.True(t, result, "PASS_REPL should allow")
}

// VALIDATES: ERROR status falls back to local.
// PREVENTS: ERROR treated as deny instead of fallback.
func TestTacacsAuthorizerErrorFallback(t *testing.T) {
	key := []byte("secret")
	srv := newTestServer(t, key, authorReply(AuthorStatusError))
	defer srv.close()

	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: srv.addr(), Key: key}},
		Timeout: 2 * time.Second,
	})
	local := &fakeLocalAuthz{allow: true}
	authorizer := NewTacacsAuthorizer(client, local, nil)

	result := authorizer.Authorize("admin", "show version", true)
	assert.True(t, result, "ERROR should fall back to local allow")
}
