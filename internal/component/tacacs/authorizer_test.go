package tacacs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// fakeLocalAuthz is a test double for local RBAC (aaa.Authorizer shape).
type fakeLocalAuthz struct {
	allow bool
	calls int
}

func (f *fakeLocalAuthz) Authorize(_, _, _ string, _ bool) bool {
	f.calls++
	return f.allow
}

func decodeAuthorRequestArgs(body []byte) ([]string, error) {
	if len(body) < 8 {
		return nil, assert.AnError
	}
	argCount := int(body[7])
	if len(body) < 8+argCount {
		return nil, assert.AnError
	}
	argLens := body[8 : 8+argCount]
	offset := 8 + argCount + int(body[4]) + int(body[5]) + int(body[6])
	if offset > len(body) {
		return nil, assert.AnError
	}
	args := make([]string, 0, argCount)
	for _, argLen := range argLens {
		next := offset + int(argLen)
		if next > len(body) {
			return nil, assert.AnError
		}
		args = append(args, string(body[offset:next]))
		offset = next
	}
	return args, nil
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
	authorizer := newTacacsAuthorizer(client, local)
	result := authorizer.Authorize("admin", "10.0.0.1:22", "show version", true)
	assert.True(t, result, "PASS_ADD should allow")
}

// VALIDATES: AuthorizeCommandArgs preserves exact cmd-arg boundaries for TACACS+.
// PREVENTS: typed inter-plugin args with spaces or odd characters being re-split before authorization.
func TestTacacsAuthorizerAuthorizeCommandArgsPreservesOddArgs(t *testing.T) {
	key := []byte("secret")
	var gotArgs []string
	var gotErr error
	srv := newTestServer(t, key, func(_ PacketHeader, body []byte) []byte {
		gotArgs, gotErr = decodeAuthorRequestArgs(body)
		return authorReply(AuthorStatusPassAdd)(PacketHeader{}, nil)
	})
	defer srv.close()

	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: srv.addr(), Key: key}},
		Timeout: 2 * time.Second,
	})
	local := &fakeLocalAuthz{allow: false}
	authorizer := newTacacsAuthorizer(client, local)

	result := authorizer.AuthorizeCommandArgs(
		"admin",
		"10.0.0.1:22",
		"request bgp adj-rib-in accept-routes",
		[]string{"peer key with spaces", `quote"inside`, `slash\inside`},
		"",
		true,
	)
	assert.True(t, result, "PASS_ADD should allow")
	assert.NoError(t, gotErr)
	assert.Equal(t, []string{
		"service=shell",
		"cmd=request",
		"cmd-arg=bgp",
		"cmd-arg=adj-rib-in",
		"cmd-arg=accept-routes",
		"cmd-arg=peer key with spaces",
		`cmd-arg=quote"inside`,
		`cmd-arg=slash\inside`,
	}, gotArgs)
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
	authorizer := newTacacsAuthorizer(client, local)

	result := authorizer.Authorize("admin", "10.0.0.1:22", "restart", true)
	assert.False(t, result, "FAIL should deny")
}

// VALIDATES: replacing TACACS+ local fallback keeps a reachable remote denial
// authoritative over permissive accepted-generation policy.
// PREVENTS: API fallback rebinding bypassing TACACS+ command authorization.
func TestTacacsAuthorizerBoundLocalFallbackPreservesRemoteDenial(t *testing.T) {
	key := []byte("secret")
	srv := newTestServer(t, key, authorReply(AuthorStatusFail))
	defer srv.close()

	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: srv.addr(), Key: key}},
		Timeout: 2 * time.Second,
	})
	authorizer := newTacacsAuthorizer(client, &fakeLocalAuthz{allow: false})
	permissive := &fakeLocalAuthz{allow: true}
	bound := authorizer.BindLocalFallback(permissive)

	assert.False(t, bound.Authorize("admin", "10.0.0.1:22", "restart", false))
	assert.Equal(t, 0, permissive.calls, "reachable TACACS+ denial must not consult local fallback")
}

// VALIDATES: replacing TACACS+ local fallback preserves server-unavailable
// fallback semantics.
// PREVENTS: accepted-generation rebinding turning TACACS+ outages into denial.
func TestTacacsAuthorizerBoundLocalFallbackUsedWhenUnavailable(t *testing.T) {
	client := NewTacacsClient(TacacsClientConfig{Timeout: 200 * time.Millisecond})
	authorizer := newTacacsAuthorizer(client, &fakeLocalAuthz{allow: false})
	permissive := &fakeLocalAuthz{allow: true}
	bound := authorizer.BindLocalFallback(permissive)

	assert.True(t, bound.Authorize("admin", "10.0.0.1:22", "show version", true))
	assert.Equal(t, 1, permissive.calls)
}

// VALIDATES: AC-9/AC-10 -- TACACS+ server unreachable falls back to local.
// PREVENTS: commands blocked when TACACS+ authorization server is down.
func TestTacacsAuthorizerFallbackToLocal(t *testing.T) {
	client := NewTacacsClient(TacacsClientConfig{
		Timeout: 200 * time.Millisecond,
	})
	local := &fakeLocalAuthz{allow: true}
	authorizer := newTacacsAuthorizer(client, local)

	result := authorizer.Authorize("admin", "10.0.0.1:22", "show version", true)
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
	authorizer := newTacacsAuthorizer(client, local)

	result := authorizer.Authorize("admin", "10.0.0.1:22", "show version", true)
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
	authorizer := newTacacsAuthorizer(client, local)

	result := authorizer.Authorize("admin", "10.0.0.1:22", "show version", true)
	assert.True(t, result, "ERROR should fall back to local allow")
}

// VALIDATES: strict TACACS+ fallback denies when authorization is unavailable.
// PREVENTS: local RBAC fail-open when operator requested strict TACACS+ authorization.
func TestTacacsAuthorizerStrictFallbackDeniesUnreachable(t *testing.T) {
	client := NewTacacsClient(TacacsClientConfig{
		Timeout: 200 * time.Millisecond,
	})
	local := &fakeLocalAuthz{allow: true}
	authorizer := newTacacsAuthorizerWithFallback(client, local, nil, true)

	result := authorizer.Authorize("admin", "10.0.0.1:22", "show version", true)
	assert.False(t, result, "strict fallback should deny")
	assert.Equal(t, 0, local.calls, "strict fallback must not call local RBAC")
}
