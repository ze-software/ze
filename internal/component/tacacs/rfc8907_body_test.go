// Design: (none -- new TACACS+ component)
// Detail: authen.go -- AuthenStart, the START body these tests pin
// Detail: author.go -- AuthorRequest, the authorization REQUEST body
// Detail: authorizer.go -- splitTacacsTokens, the argument builder
// Detail: packet.go -- Encrypt, the pseudo-pad the key length test pins
//
// VALIDATES: RFC 8907 body-level MUSTs a client meets by what it writes:
// the START user_len rule (Section 5.1), the PAP START shape (Section
// 5.4.2.2), the authen_service value (Section 5.4.2.6), the authorization
// user_len and argument-name rules (Section 6.1), the service and cmd
// arguments (Section 8.2) and the 32-character shared key (Section 10.5.1).
// PREVENTS: a marshaler that writes a length its field does not have, a
// builder that lets a separator into an argument name, or a pseudo-pad
// that reads only part of the key.
//
// The tests live in their own file because every sibling that carries an
// `RFC requirement:` tag is closed to edits (ai/rules/testing.md).

package tacacs

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// authenStartFields splits a marshaled START body into its four variable
// fields, reading each length octet the way a server does; rem_addr is
// skipped because no test here asserts it.
func authenStartFields(t *testing.T, body []byte) (user, port, data []byte) {
	t.Helper()
	require.GreaterOrEqual(t, len(body), 8, "START body shorter than its fixed header")
	userLen, portLen, remLen, dataLen := int(body[4]), int(body[5]), int(body[6]), int(body[7])
	require.Len(t, body, 8+userLen+portLen+remLen+dataLen, "length octets do not account for the body")
	off := 8
	user = body[off : off+userLen]
	off += userLen
	port = body[off : off+portLen]
	off += portLen
	off += remLen
	data = body[off : off+dataLen]
	return user, port, data
}

// RFC requirement: RFC8907-5.1-1 positive — a START built with no username writes user_len 0 and no user bytes, so the port field starts at offset 8.
func TestRFC8907AuthenStartAbsentUserWritesZeroLength(t *testing.T) {
	start := &AuthenStart{
		Action: authenActionLogin, AuthenType: 0x01, AuthenService: authenServiceLogin,
		Port: "ssh", RemAddr: "192.0.2.1",
	}
	body, err := start.MarshalBinary()
	require.NoError(t, err)

	require.Equal(t, uint8(0), body[4], "user_len")
	user, port, data := authenStartFields(t, body)
	require.Empty(t, user)
	require.Equal(t, []byte("ssh"), port, "port field must start right after the fixed header")
	require.Empty(t, data)
}

// RFC requirement: RFC8907-5.1-1 negative — a START built with a username never writes user_len 0: the octet is the username's byte length.
func TestRFC8907AuthenStartPresentUserWritesItsLength(t *testing.T) {
	start := NewPAPAuthenStart("alice", "secret", "ssh", "192.0.2.1")
	body, err := start.MarshalBinary()
	require.NoError(t, err)

	require.NotEqual(t, uint8(0), body[4], "a present user must not be signaled as absent")
	require.Equal(t, uint8(len("alice")), body[4], "user_len")
	user, _, _ := authenStartFields(t, body)
	require.Equal(t, []byte("alice"), user)
}

// RFC requirement: RFC8907-5.4.2.2-2 positive — the PAP START carries the username in the user field and the password bytes in the data field, with authen_type PAP and minor version 1.
func TestRFC8907PAPStartCarriesUserAndPassword(t *testing.T) {
	start := NewPAPAuthenStart("alice", "hunter2", "ssh", "192.0.2.1")
	body, err := start.MarshalBinary()
	require.NoError(t, err)

	require.Equal(t, uint8(authenActionLogin), body[0], "action")
	require.Equal(t, uint8(authenTypePAP), body[2], "authen_type")
	require.Equal(t, uint8(verMinorOne), start.Version(), "PAP uses minor_version 1")
	user, _, data := authenStartFields(t, body)
	require.Equal(t, []byte("alice"), user)
	require.Equal(t, []byte("hunter2"), data, "data field is the PAP ASCII password")
}

// RFC requirement: RFC8907-5.4.2.2-2 negative — a password the data field cannot hold whole is refused; a START whose data field holds a truncated password is never produced.
func TestRFC8907PAPStartRefusesPasswordItCannotCarry(t *testing.T) {
	start := NewPAPAuthenStart("alice", strings.Repeat("p", 256), "ssh", "192.0.2.1")
	body, err := start.MarshalBinary()
	require.Error(t, err)
	require.Nil(t, body)
}

// RFC requirement: RFC8907-5.4.2.6-2 positive — a login START carries authen_service TAC_PLUS_AUTHEN_SVC_LOGIN (0x01).
func TestRFC8907LoginStartCarriesLoginService(t *testing.T) {
	start := NewPAPAuthenStart("alice", "hunter2", "ssh", "192.0.2.1")
	body, err := start.MarshalBinary()
	require.NoError(t, err)
	require.Equal(t, uint8(authenServiceLogin), body[3], "authen_service")
}

// RFC requirement: RFC8907-5.4.2.6-2 negative — the ENABLE value (TAC_PLUS_AUTHEN_SVC_ENABLE, 0x02) never appears in the authen_service octet of a login START.
func TestRFC8907LoginStartNeverCarriesEnableService(t *testing.T) {
	const authenServiceEnable = 0x02
	start := NewPAPAuthenStart("alice", "hunter2", "ssh", "192.0.2.1")
	body, err := start.MarshalBinary()
	require.NoError(t, err)
	require.NotEqual(t, uint8(authenServiceEnable), body[3], "a login must not request ENABLE")
}

// RFC requirement: RFC8907-6.1-1 positive — an authorization REQUEST writes user_len equal to the byte length of the user field, and that many user bytes follow the arg length octets.
func TestRFC8907AuthorRequestUserLenMatchesUser(t *testing.T) {
	req := &AuthorRequest{User: "alice", Port: "ssh", RemAddr: "192.0.2.1", Args: []string{"service=shell", "cmd=show"}}
	body, err := req.MarshalBinary()
	require.NoError(t, err)

	require.Equal(t, uint8(len("alice")), body[4], "user_len")
	off := 8 + len(req.Args)
	require.Equal(t, []byte("alice"), body[off:off+int(body[4])])
}

// RFC requirement: RFC8907-6.1-1 negative — a user field longer than user_len can express is refused; a REQUEST whose user_len disagrees with its user field is never produced.
func TestRFC8907AuthorRequestRefusesUserItCannotMeasure(t *testing.T) {
	req := &AuthorRequest{User: strings.Repeat("u", 256), Port: "ssh", Args: []string{"service=shell"}}
	body, err := req.MarshalBinary()
	require.Error(t, err)
	require.Nil(t, body)
}

// argName returns the argument name: the bytes before the first separator.
func argName(arg string) string {
	if i := strings.IndexAny(arg, "=*"); i >= 0 {
		return arg[:i]
	}
	return arg
}

// RFC requirement: RFC8907-6.1-2 positive — every argument the client builds names one of service, cmd or cmd-arg before its first separator.
func TestRFC8907ArgumentNamesCarryNoSeparator(t *testing.T) {
	for _, arg := range splitTacacsArgs("show bgp neighbor 192.0.2.1") {
		name := argName(arg)
		require.Contains(t, []string{"service", "cmd", "cmd-arg"}, name, "argument %q", arg)
		require.NotContains(t, name, "=")
		require.NotContains(t, name, "*")
	}
}

// RFC requirement: RFC8907-6.1-2 negative — a command token that itself holds '=' or '*' lands after the separator as the value; the argument name never gains a separator.
func TestRFC8907ArgumentValueKeepsSeparatorOutOfName(t *testing.T) {
	args := splitTacacsArgs("set key=a*b value=*")
	require.Equal(t, []string{"service=shell", "cmd=set", "cmd-arg=key=a*b", "cmd-arg=value=*"}, args)
	for _, arg := range args[1:] {
		require.NotContains(t, argName(arg), "=", "argument %q", arg)
		require.NotContains(t, argName(arg), "*", "argument %q", arg)
	}
}

// RFC requirement: RFC8907-8.2-1 positive — the argument list for a shell command opens with service=shell.
func TestRFC8907ServiceArgumentAlwaysFirst(t *testing.T) {
	args := splitTacacsArgs("show version")
	require.Equal(t, "service=shell", args[0])
}

// RFC requirement: RFC8907-8.2-1 negative — an empty command, the one input with nothing to name, still carries service=shell; an argument list without it is never built.
func TestRFC8907ServiceArgumentPresentForEmptyCommand(t *testing.T) {
	args := splitTacacsArgs("")
	require.Contains(t, args, "service=shell")
}

// RFC requirement: RFC8907-8.2-2 positive — with service=shell the list carries cmd=<verb>, the first token of the command.
func TestRFC8907CmdArgumentFollowsShellService(t *testing.T) {
	args := splitTacacsArgs("show version")
	require.Equal(t, "service=shell", args[0])
	require.Equal(t, "cmd=show", args[1])
}

// RFC requirement: RFC8907-8.2-2 negative — an empty command still carries a cmd argument (cmd= with an empty value); a service=shell list without cmd is never built.
func TestRFC8907CmdArgumentPresentForEmptyCommand(t *testing.T) {
	args := splitTacacsArgs("")
	require.Equal(t, "service=shell", args[0])
	require.Equal(t, "cmd=", args[1])
}

// RFC requirement: RFC8907-10.5.1-2 negative — the pseudo-pad for a 32-character key differs from the pad for its first 31 characters, so the key is never truncated before use.
func TestRFC8907SharedKeyIsNotTruncated(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	require.Len(t, key, 32)

	plain := bytes.Repeat([]byte{0x5A}, 48)
	whole := bytes.Clone(plain)
	Encrypt(whole, 0x01020304, key, verMinorOne, 1)
	short := bytes.Clone(plain)
	Encrypt(short, 0x01020304, key[:31], verMinorOne, 1)

	require.NotEqual(t, plain, whole, "the pad must change the body")
	require.NotEqual(t, whole, short, "the 32nd key character must reach the pad")
}
