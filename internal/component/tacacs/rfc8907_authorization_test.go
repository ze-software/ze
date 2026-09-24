// Design: docs/architecture/aaa-tacacs.md
// Detail: authorizer.go -- effective command policy and mandatory AV handling.
// Detail: authenticator.go -- session authorization and privilege assignment.
package tacacs

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/aaa"
)

// authorReplyArgs encodes the RFC 8907 Section 6.2 reply layout:
// status[0], arg_cnt[1], message/data lengths[2:6], AV lengths[6:6+count], AVs.
func authorReplyArgs(status uint8, args []string) func(PacketHeader, []byte) []byte {
	return func(_ PacketHeader, _ []byte) []byte {
		size := 6 + len(args)
		for _, arg := range args {
			size += len(arg)
		}
		body := make([]byte, size)
		body[0], body[1] = status, uint8(len(args))
		offset := 6 + len(args)
		for i, arg := range args {
			body[6+i] = uint8(len(arg))
			copy(body[offset:], arg)
			offset += len(arg)
		}
		return body
	}
}

func commandPolicyAuthorizer(t *testing.T, status uint8, args []string) (*tacacsAuthorizer, *fakeLocalAuthz) {
	t.Helper()
	key := []byte("command-policy-key")
	srv := newTestServer(t, key, authorReplyArgs(status, args))
	t.Cleanup(srv.close)
	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: srv.addr(), Key: key}},
		Timeout: 2 * time.Second,
	})
	t.Cleanup(client.Close)
	local := &fakeLocalAuthz{allow: true}
	return newTacacsAuthorizer(client, local), local
}

// RFC requirement: RFC8907-6-1 positive — recognized mandatory shell and command values preserve authorization, and separators inside command values remain part of the value.
// RFC requirement: RFC8907-10.5.4-1 positive — a mandatory policy with the same shell command and exact argument values is enacted by approving that command.
func TestRFC8907MandatoryCommandPolicyAccepted(t *testing.T) {
	authz, _ := commandPolicyAuthorizer(t, AuthorStatusPassRepl, []string{
		"service=shell", "cmd=show", "cmd-arg=name=a*b",
	})
	if !authz.AuthorizeCommandArgs("alice", "192.0.2.1", "show", []string{"name=a*b"}, "", true) {
		t.Fatal("identical command with separators inside its value was denied")
	}
}

// RFC requirement: RFC8907-6-1 negative — unsupported mandatory arguments and unsupported mandatory values deny the command without permissive local fallback.
// RFC requirement: RFC8907-10.5.4-1 negative — unrecognized mandatory arguments are terminal denials, not successful authorization or local fallback.
func TestRFC8907MandatoryCommandPolicyDenied(t *testing.T) {
	for _, arg := range []string{
		"vendor-policy=permit", "timeout=5", "priv-lvl=15", "service=connection", "cmd=delete",
	} {
		t.Run(arg, func(t *testing.T) {
			authz, local := commandPolicyAuthorizer(t, AuthorStatusPassAdd, []string{arg})
			if authz.Authorize("alice", "192.0.2.1", "show version", true) {
				t.Fatal("mandatory policy which the command consumer cannot enact was granted")
			}
			if local.calls != 0 {
				t.Fatal("mandatory policy denial consulted permissive local fallback")
			}
		})
	}
}

// RFC requirement: RFC8907-6-1 positive — unsupported optional attributes can be disregarded; an equals sign after the first optional separator does not make the attribute mandatory.
func TestRFC8907OptionalCommandPolicyIgnored(t *testing.T) {
	authz, _ := commandPolicyAuthorizer(t, AuthorStatusPassAdd, []string{
		"vendor-policy*value=with*separators", "timeout*5", "priv-lvl*15",
	})
	if !authz.Authorize("alice", "192.0.2.1", "show version", true) {
		t.Fatal("unsupported optional policy denied an otherwise authorized command")
	}
}

// RFC requirement: RFC8907-6.2-1 positive — PASS_ADD retains the requested command while applying supported mandatory service and command confirmations.
func TestRFC8907PassAddRetainsCommand(t *testing.T) {
	authz, _ := commandPolicyAuthorizer(t, AuthorStatusPassAdd, []string{"service=shell", "cmd=show"})
	if !authz.Authorize("alice", "192.0.2.1", "show version", true) {
		t.Fatal("PASS_ADD lost the request arguments")
	}
}

// RFC requirement: RFC8907-6.2-1 negative — an added mandatory cmd-arg cannot be ignored to authorize the unchanged original command.
func TestRFC8907PassAddCannotIgnoreCommandExtension(t *testing.T) {
	authz, local := commandPolicyAuthorizer(t, AuthorStatusPassAdd, []string{"cmd-arg=detail"})
	if authz.Authorize("alice", "192.0.2.1", "show version", true) {
		t.Fatal("command was granted without the required added argument")
	}
	if local.calls != 0 {
		t.Fatal("unsupported command extension fell back to local policy")
	}
}

// RFC requirement: RFC8907-6.2-2 positive — PASS_REPL authorizes when its complete replacement describes exactly the original command and ordered argument boundaries.
func TestRFC8907PassReplPreservesExactCommand(t *testing.T) {
	authz, _ := commandPolicyAuthorizer(t, AuthorStatusPassRepl, []string{
		"service=shell", "cmd=show", "cmd-arg=peer one", "cmd-arg=detail", "vendor-note*ignored",
	})
	if !authz.AuthorizeCommandArgs("alice", "192.0.2.1", "show", []string{"peer one", "detail"}, "", true) {
		t.Fatal("identical replacement with preserved argument boundaries was denied")
	}
}

// RFC requirement: RFC8907-6.2-2 negative — empty, incomplete, changed, reordered and re-split replacements cannot authorize the immutable original command.
func TestRFC8907PassReplCannotAuthorizeOriginalCommand(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"empty", nil},
		{"missing service", []string{"cmd=show", "cmd-arg=peer one", "cmd-arg=detail"}},
		{"missing command", []string{"service=shell", "cmd-arg=peer one", "cmd-arg=detail"}},
		{"changed command", []string{"service=shell", "cmd=delete", "cmd-arg=peer one", "cmd-arg=detail"}},
		{"removed argument", []string{"service=shell", "cmd=show", "cmd-arg=peer one"}},
		{"reordered arguments", []string{"service=shell", "cmd=show", "cmd-arg=detail", "cmd-arg=peer one"}},
		{"split argument", []string{"service=shell", "cmd=show", "cmd-arg=peer", "cmd-arg=one", "cmd-arg=detail"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authz, local := commandPolicyAuthorizer(t, AuthorStatusPassRepl, tt.args)
			if authz.AuthorizeCommandArgs("alice", "192.0.2.1", "show", []string{"peer one", "detail"}, "", true) {
				t.Fatal("replacement authorized a different original command")
			}
			if local.calls != 0 {
				t.Fatal("replacement refusal fell back to local policy")
			}
		})
	}
}

func sessionPolicyAuthenticator(t *testing.T, status uint8, args []string, data []byte) *tacacsAuthenticator {
	t.Helper()
	key := []byte("session-policy-key")
	srv := newProfileServer(t, key, func(header PacketHeader, body []byte) []byte {
		if header.Type == 0x01 {
			return authenReply(AuthenStatusPass, "", data)(header, body)
		}
		requested, err := decodeAuthorRequestArgs(body)
		if err != nil || !slices.Equal(requested, []string{"service=shell", "cmd="}) {
			return authorReply(AuthorStatusFail)(header, body)
		}
		return authorReplyArgs(status, args)(header, body)
	})
	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: srv.addr(), Key: key}},
		Timeout: 2 * time.Second,
	})
	t.Cleanup(client.Close)
	return newTacacsAuthenticator(client, map[int][]string{
		0: {"guest"}, 1: {"ops"}, 15: {"admin"}, 16: {"invalid-admin"},
	}, nil)
}

// RFC requirement: RFC8907-8.1-1 positive — a two-digit decimal privilege within the supported range is handled by session authorization and maps to the configured profile.
// RFC requirement: RFC8907-6.2-1 positive — PASS_ADD session authorization applies the returned priv-lvl rather than retaining the default privilege.
func TestRFC8907SessionPrivilegeApplied(t *testing.T) {
	auth := sessionPolicyAuthenticator(t, AuthorStatusPassAdd, []string{"priv-lvl=15"}, nil)
	result, err := auth.Authenticate(aliceRequest)
	if err != nil || !result.Authenticated || !slices.Equal(result.Profiles, []string{"admin"}) {
		t.Fatalf("session privilege was not applied: result=%+v err=%v", result, err)
	}
}

// RFC requirement: RFC8907-8.1-1 negative — overlong mandatory privilege numbers are refused before conversion; out-of-range or nondecimal privileges cannot select a privileged profile.
func TestRFC8907InvalidMandatoryPrivilegeDenied(t *testing.T) {
	for _, value := range []string{"18446744073709551631", strings.Repeat("0", 240) + "15", "16", "-1", "+1", "", "1x"} {
		t.Run(value, func(t *testing.T) {
			auth := sessionPolicyAuthenticator(t, AuthorStatusPassAdd, []string{"priv-lvl=" + value}, nil)
			result, err := auth.Authenticate(aliceRequest)
			if !errors.Is(err, aaa.ErrAuthRejected) || result.Authenticated || len(result.Profiles) != 0 {
				t.Fatalf("invalid mandatory privilege granted profiles: result=%+v err=%v", result, err)
			}
		})
	}
}

// RFC requirement: RFC8907-8.1-1 positive — an unhandled overlong optional numeric attribute is ignored under Section 6.1 rather than converted into privilege.
func TestRFC8907OverlongOptionalPrivilegeCannotElevate(t *testing.T) {
	auth := sessionPolicyAuthenticator(t, AuthorStatusPassAdd, []string{"priv-lvl*18446744073709551631"}, []byte{15})
	result, err := auth.Authenticate(aliceRequest)
	if err != nil || !result.Authenticated || !slices.Equal(result.Profiles, []string{"ops"}) {
		t.Fatalf("unhandled optional privilege or PAP data affected the default profile: result=%+v err=%v", result, err)
	}
}

// RFC requirement: RFC8907-6.2-2 positive — complete session replacement with priv-lvl=0 maps level zero without restoring a default or taking privilege from PAP data.
func TestRFC8907SessionReplacementMapsZero(t *testing.T) {
	auth := sessionPolicyAuthenticator(t, AuthorStatusPassRepl, []string{"service=shell", "cmd=", "priv-lvl=0"}, []byte{15})
	result, err := auth.Authenticate(aliceRequest)
	if err != nil || !result.Authenticated || !slices.Equal(result.Profiles, []string{"guest"}) {
		t.Fatalf("session replacement did not apply level zero: result=%+v err=%v", result, err)
	}
}

// RFC requirement: RFC8907-6-1 negative — session login rejects mandatory restrictions its profile consumer cannot enact, even when priv-lvl=15 is otherwise mapped.
// RFC requirement: RFC8907-10.5.4-1 negative — an unknown mandatory session argument makes Authenticate return terminal rejection with no granted profiles.
func TestRFC8907MandatorySessionPolicyDenied(t *testing.T) {
	for _, arg := range []string{"vendor-policy=admin", "timeout=5", "autocmd=show version", "cmd-arg=extra"} {
		t.Run(arg, func(t *testing.T) {
			auth := sessionPolicyAuthenticator(t, AuthorStatusPassAdd, []string{"priv-lvl=15", arg}, nil)
			result, err := auth.Authenticate(aliceRequest)
			if !errors.Is(err, aaa.ErrAuthRejected) || result.Authenticated || len(result.Profiles) != 0 {
				t.Fatalf("unenforceable session policy granted profiles: result=%+v err=%v", result, err)
			}
		})
	}
}

// RFC requirement: RFC8907-6.2-2 negative — session replacement without service and cmd does not inherit those request attributes and cannot grant a profile.
func TestRFC8907IncompleteSessionReplacementDenied(t *testing.T) {
	auth := sessionPolicyAuthenticator(t, AuthorStatusPassRepl, []string{"priv-lvl=15"}, nil)
	result, err := auth.Authenticate(aliceRequest)
	if !errors.Is(err, aaa.ErrAuthRejected) || result.Authenticated || len(result.Profiles) != 0 {
		t.Fatalf("incomplete replacement inherited requested session: result=%+v err=%v", result, err)
	}
}

// Authentication success alone cannot assign a profile: session authorization
// denial remains authoritative even when the PAP data resembles level 15.
func TestTacacsAuthenticatorPAPDataCannotOverrideSessionDenial(t *testing.T) {
	auth := sessionPolicyAuthenticator(t, AuthorStatusFail, nil, []byte{15})
	result, err := auth.Authenticate(aliceRequest)
	if !errors.Is(err, aaa.ErrAuthRejected) || result.Authenticated || len(result.Profiles) != 0 {
		t.Fatalf("PAP data overrode session authorization denial: result=%+v err=%v", result, err)
	}
}

// Invalid local request text is a terminal rejection, never an infrastructure
// failure which permits the AAA chain or local authorization fallback.
func TestTacacsInvalidRequestCannotFallBack(t *testing.T) {
	client := NewTacacsClient(TacacsClientConfig{Timeout: time.Second})
	t.Cleanup(client.Close)
	auth := newTacacsAuthenticator(client, map[int][]string{1: {"admin"}}, nil)
	result, err := auth.Authenticate(aaa.AuthRequest{Username: "alice", Password: "bad\npassword"})
	if !errors.Is(err, aaa.ErrAuthRejected) || result.Authenticated {
		t.Fatalf("invalid PAP request did not reject: result=%+v err=%v", result, err)
	}
	local := &fakeLocalAuthz{allow: true}
	authz := newTacacsAuthorizer(client, local)
	if authz.AuthorizeCommandArgs("alice", "", "show", []string{strings.Repeat("x", 256)}, "", true) {
		t.Fatal("invalid command request was granted")
	}
	if local.calls != 0 {
		t.Fatal("invalid command request consulted local fallback")
	}
}
