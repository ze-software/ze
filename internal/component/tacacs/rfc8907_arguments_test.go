// Design: docs/architecture/aaa-tacacs.md
// Detail: authen.go, author.go, acct.go -- the body codecs and their text checks
// Detail: authenticator.go -- handlePass, the session authorization request
// Detail: authorizer.go -- the command authorization request and policy
// Detail: accounting.go -- CommandStart and CommandStop
//
// VALIDATES: RFC 8907 Section 3.7 (every text field is printable US-ASCII),
// Section 5.4.2.2 (a PAP START carries the username and the password),
// Section 6.1 (user_len counts bytes), Section 7.1 (the accounting flag
// values), Section 8 (the argument dictionary for every request kind Ze
// sends), Sections 8.1 and 8.2 (UTC epoch times, service and cmd), and
// Section 10.5.4 (shared argument values keep their defined meaning).
// PREVENTS: a text check that guards four fields while the others pass
// control bytes, a byte length computed as a rune count, and a request kind
// whose arguments no test reads.
//
// The servers here decode the request the client wrote, so each assertion
// reads the octets that reach the wire.

package tacacs

import (
	"bytes"
	"errors"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/aaa"
)

// printableASCII holds every US-ASCII character outside the RFC 20 Section 5.2
// control characters.
func printableASCII() string {
	var text strings.Builder
	for c := byte(0x20); c < 0x7f; c++ {
		text.WriteByte(c)
	}
	return text.String()
}

var nonPrintableText = []string{"a\x00b", "a\x1fb", "a\x7fb", "aéb"}

// textReplyBodies builds an authorization and an accounting reply whose
// server_msg, data and one argument all hold text.
func textReplyBodies(text string) (author, acct []byte) {
	author = []byte{AuthorStatusPassAdd, 1, 0, byte(len(text)), 0, byte(len(text)), byte(len(text) + 2)}
	author = append(author, text...)
	author = append(author, text...)
	author = append(author, "a="...)
	author = append(author, text...)
	acct = []byte{0, byte(len(text)), 0, byte(len(text)), AcctStatusSuccess}
	acct = append(acct, text...)
	acct = append(acct, text...)
	return author, acct
}

// RFC requirement: RFC8907-3.7-2 positive -- every printable US-ASCII character survives unchanged in the authentication port and rem_addr, the authorization port, rem_addr and argument, the accounting rem_addr and argument, and the authorization and accounting reply server_msg, data and argument.
func TestRFC8907PrintableTextInEveryField(t *testing.T) {
	text := printableASCII()

	authen, err := NewPAPAuthenStart("alice", "secret", text, text).MarshalBinary()
	require.NoError(t, err)
	require.Equal(t, 2, bytes.Count(authen, []byte(text)), "authentication port and rem_addr")

	author, err := (&AuthorRequest{User: "alice", Port: text, RemAddr: text, Args: []string{"cmd=" + text}}).MarshalBinary()
	require.NoError(t, err)
	require.Equal(t, 3, bytes.Count(author, []byte(text)), "authorization port, rem_addr and argument")

	acct, err := (&AcctRequest{Flags: AcctFlagStart, User: "alice", Port: "ssh", RemAddr: text, Args: []string{"cmd=" + text}}).MarshalBinary()
	require.NoError(t, err)
	require.Equal(t, 2, bytes.Count(acct, []byte(text)), "accounting rem_addr and argument")

	authorBody, acctBody := textReplyBodies(text)
	authorReply, err := UnmarshalAuthorResponse(authorBody)
	require.NoError(t, err)
	require.Equal(t, text, authorReply.ServerMsg)
	require.Equal(t, []byte(text), authorReply.Data)
	require.Equal(t, []string{"a=" + text}, authorReply.Args)
	acctReply, err := UnmarshalAcctReply(acctBody)
	require.NoError(t, err)
	require.Equal(t, text, acctReply.ServerMsg)
	require.Equal(t, []byte(text), acctReply.Data)
}

// RFC requirement: RFC8907-3.7-2 negative -- a NUL, a control character, DEL or a non-ASCII byte is refused in the authentication port and rem_addr, the authorization port and rem_addr, the accounting rem_addr and argument, and the authorization and accounting reply server_msg, data and argument.
func TestRFC8907ControlCharactersRefusedInEveryField(t *testing.T) {
	for _, bad := range nonPrintableText {
		requests := map[string]func() error{
			"authentication port": func() error {
				_, err := NewPAPAuthenStart("alice", "secret", bad, "ok").MarshalBinary()
				return err
			},
			"authentication rem_addr": func() error {
				_, err := NewPAPAuthenStart("alice", "secret", "ok", bad).MarshalBinary()
				return err
			},
			"authorization port": func() error {
				_, err := (&AuthorRequest{User: "alice", Port: bad, Args: []string{"service=shell"}}).MarshalBinary()
				return err
			},
			"authorization rem_addr": func() error {
				_, err := (&AuthorRequest{User: "alice", RemAddr: bad, Args: []string{"service=shell"}}).MarshalBinary()
				return err
			},
			"accounting rem_addr": func() error {
				_, err := (&AcctRequest{Flags: AcctFlagStart, User: "alice", RemAddr: bad}).MarshalBinary()
				return err
			},
			"accounting argument": func() error {
				_, err := (&AcctRequest{Flags: AcctFlagStart, User: "alice", Args: []string{"cmd=" + bad}}).MarshalBinary()
				return err
			},
		}
		for field, marshal := range requests {
			require.Error(t, marshal(), "%s accepted %q", field, bad)
		}

		clean, _ := textReplyBodies("ok")
		_, err := UnmarshalAuthorResponse(clean)
		require.NoError(t, err, "the clean control reply")
		author := []byte{AuthorStatusPassAdd, 0, 0, byte(len(bad)), 0, 0}
		_, err = UnmarshalAuthorResponse(append(author, bad...))
		require.Error(t, err, "authorization server_msg accepted %q", bad)
		author = []byte{AuthorStatusPassAdd, 0, 0, 0, 0, byte(len(bad))}
		_, err = UnmarshalAuthorResponse(append(author, bad...))
		require.Error(t, err, "authorization data accepted %q", bad)
		author = []byte{AuthorStatusPassAdd, 1, 0, 0, 0, 0, byte(len(bad) + 2), 'a', '='}
		_, err = UnmarshalAuthorResponse(append(author, bad...))
		require.Error(t, err, "authorization argument accepted %q", bad)
		acct := []byte{0, byte(len(bad)), 0, 0, AcctStatusSuccess}
		_, err = UnmarshalAcctReply(append(acct, bad...))
		require.Error(t, err, "accounting server_msg accepted %q", bad)
		acct = []byte{0, 0, 0, byte(len(bad)), AcctStatusSuccess}
		_, err = UnmarshalAcctReply(append(acct, bad...))
		require.Error(t, err, "accounting data accepted %q", bad)
	}
}

// papCaptureServer answers a PAP START with PASS and records the decrypted body.
func papCaptureServer(t *testing.T) (*testTacacsServer, <-chan []byte, *atomic.Bool) {
	t.Helper()
	seen := make(chan []byte, 1)
	contacted := &atomic.Bool{}
	srv := newTestServer(t, sessionKey, func(hdr PacketHeader, body []byte) []byte {
		contacted.Store(true)
		seen <- bytes.Clone(body)
		return passReply()(hdr, body)
	})
	return srv, seen, contacted
}

// RFC requirement: RFC8907-5.4.2.2-2 positive -- the PAP START the client puts on the wire carries the username in the user field and the password in the data field.
func TestRFC8907PAPStartOnWireCarriesUserAndPassword(t *testing.T) {
	srv, seen, _ := papCaptureServer(t)
	client := validationClient(t, srv)
	reply, err := client.Authenticate("alice", "pass word!", "ssh", "192.0.2.1")
	require.NoError(t, err)
	require.Equal(t, uint8(AuthenStatusPass), reply.Status)
	user, _, data := authenStartFields(t, <-seen)
	require.Equal(t, []byte("alice"), user)
	require.Equal(t, []byte("pass word!"), data)
}

// RFC requirement: RFC8907-5.4.2.2-2 negative -- a PAP login with no username is refused as an invalid request before any START reaches the server.
func TestRFC8907PAPStartWithoutUsernameNeverSent(t *testing.T) {
	srv, _, contacted := papCaptureServer(t)
	client := validationClient(t, srv)
	_, err := client.Authenticate("", "secret", "ssh", "192.0.2.1")
	require.ErrorIs(t, err, errRequestInvalid)
	require.False(t, contacted.Load(), "a START with no username reached the server")
}

// RFC requirement: RFC8907-6.1-1 positive -- user_len is the byte count of a multi-byte user: "Alicé" writes 6, and a 255-byte user of two-byte characters writes 255.
func TestRFC8907AuthorUserLenCountsBytes(t *testing.T) {
	for _, user := range []string{"Alicé", strings.Repeat("é", 127) + "a"} {
		body, err := (&AuthorRequest{User: user, Port: "ssh", Args: []string{"service=shell"}}).MarshalBinary()
		require.NoError(t, err)
		require.Equal(t, uint8(len(user)), body[4], "user_len for %d runes", len([]rune(user)))
		off := 8 + 1
		require.Equal(t, []byte(user), body[off:off+int(body[4])])
	}
}

// RFC requirement: RFC8907-6.1-1 negative -- a user of 128 two-byte characters, 128 runes but 256 bytes, is refused rather than measured by its rune count.
func TestRFC8907AuthorUserLenRefusesRuneCount(t *testing.T) {
	user := strings.Repeat("é", 128)
	_, err := (&AuthorRequest{User: user, Port: "ssh", Args: []string{"service=shell"}}).MarshalBinary()
	require.Error(t, err)
}

var validAcctFlags = map[uint8]bool{
	AcctFlagStart: true, AcctFlagStop: true, AcctFlagWatchdog: true, AcctFlagWatchdog | AcctFlagStart: true,
}

// RFC requirement: RFC8907-7-1 positive -- START (0x02), STOP (0x04), WATCHDOG (0x08) and WATCHDOG with START (0x0a) are written to the flags octet unchanged.
func TestRFC8907AccountingValidFlagsWritten(t *testing.T) {
	for flags := range validAcctFlags {
		body, err := (&AcctRequest{Flags: flags, User: "alice", Port: "ssh"}).MarshalBinary()
		require.NoError(t, err, "flags %#x", flags)
		require.Equal(t, flags, body[0])
	}
}

// RFC requirement: RFC8907-7-1 negative -- every one of the other 252 flags values, including each of the undefined bits 0x01 and 0x10 to 0x80, is refused, and SendAccounting refuses one before any request is sent.
func TestRFC8907AccountingOtherFlagsRefused(t *testing.T) {
	for value := range 256 {
		flags := uint8(value)
		if validAcctFlags[flags] {
			continue
		}
		_, err := (&AcctRequest{Flags: flags, User: "alice", Port: "ssh"}).MarshalBinary()
		require.Error(t, err, "flags %#x", flags)
	}
	contacted := &atomic.Bool{}
	client := validationClient(t, newTestServer(t, sessionKey, func(_ PacketHeader, _ []byte) []byte {
		contacted.Store(true)
		return validationReply(typeAccounting, AcctStatusSuccess)
	}))
	_, err := client.SendAccounting(&AcctRequest{Flags: AcctFlagStart | 0x80, User: "alice"})
	require.ErrorIs(t, err, errRequestInvalid)
	require.False(t, contacted.Load())
}

// authorRequests captures the arguments of the command authorization request
// for "show bgp summary" and of the session authorization request a PAP login
// sends.
func authorRequests(t *testing.T) (command, session []string) {
	t.Helper()
	commands := make(chan []string, 1)
	srv := newTestServer(t, sessionKey, func(hdr PacketHeader, body []byte) []byte {
		args, err := decodeAuthorRequestArgs(body)
		require.NoError(t, err)
		commands <- args
		return authorReply(AuthorStatusPassAdd)(hdr, body)
	})
	client := validationClient(t, srv)
	require.True(t, newTacacsAuthorizer(client, &fakeLocalAuthz{allow: true}).Authorize("alice", "192.0.2.1", "show bgp summary", true))

	sessions := make(chan []string, 1)
	login := newProfileServer(t, sessionKey, func(hdr PacketHeader, body []byte) []byte {
		if hdr.Type == typeAuthorization {
			args, err := decodeAuthorRequestArgs(body)
			require.NoError(t, err)
			sessions <- args
			return authorReplyArgs(AuthorStatusPassAdd, []string{"priv-lvl=15"})(hdr, body)
		}
		return passReply()(hdr, body)
	})
	loginClient := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: login.addr(), Key: sessionKey}},
		Timeout: 2 * time.Second,
	})
	t.Cleanup(loginClient.Close)
	result, err := newTacacsAuthenticator(loginClient, map[int][]string{15: {"admin"}}, nil).Authenticate(aliceRequest)
	require.NoError(t, err)
	require.True(t, result.Authenticated)
	return <-commands, <-sessions
}

// accountingRecords returns the START and STOP records for one command.
func accountingRecords(t *testing.T, command string) (start, stop *AcctRequest) {
	t.Helper()
	acct := newQueuedAccountant()
	id := acct.CommandStart("alice", "192.0.2.1", command)
	start = dequeue(t, acct)
	acct.CommandStop(id, "alice", "192.0.2.1", command)
	return start, dequeue(t, acct)
}

var authorizationNames = map[string]bool{"service": true, "cmd": true, "cmd-arg": true}

// RFC requirement: RFC8907-8-1 positive -- the command authorization request expresses the command with service, cmd and cmd-arg, and the session authorization request a login sends expresses the shell session with service and cmd.
func TestRFC8907AuthorizationRequestsUseDictionaryArguments(t *testing.T) {
	command, session := authorRequests(t)
	require.Equal(t, []string{"service=shell", "cmd=show", "cmd-arg=bgp", "cmd-arg=summary"}, command)
	require.Equal(t, []string{"service=shell", "cmd="}, session)
}

// RFC requirement: RFC8907-8-1 negative -- neither authorization request Ze sends carries an argument name outside the Section 8.2 dictionary.
func TestRFC8907AuthorizationRequestsCarryNoForeignArgument(t *testing.T) {
	command, session := authorRequests(t)
	for _, arg := range slices.Concat(command, session) {
		require.True(t, authorizationNames[argName(arg)], "argument %q is not in the Section 8.2 dictionary", arg)
	}
}

// RFC requirement: RFC8907-8.2-1 positive -- the service argument is the first argument of the command authorization request, the session authorization request, and the accounting START and STOP records.
func TestRFC8907EveryRequestKindLeadsWithService(t *testing.T) {
	command, session := authorRequests(t)
	start, stop := accountingRecords(t, "show bgp summary")
	require.Equal(t, "service=shell", command[0])
	require.Equal(t, "service=shell", session[0])
	for _, record := range []*AcctRequest{start, stop} {
		value, ok := argValue(record.Args, "service")
		require.True(t, ok, "accounting record lacks service")
		require.Equal(t, "shell", value)
	}
}

// RFC requirement: RFC8907-8.2-1 negative -- no request kind drops service, including the session request whose cmd is empty and records for a command with no arguments, and none carries a second service argument.
func TestRFC8907NoRequestKindOmitsService(t *testing.T) {
	command, session := authorRequests(t)
	start, stop := accountingRecords(t, "show")
	for _, args := range [][]string{command, session, start.Args, stop.Args} {
		count := 0
		for _, arg := range args {
			if argName(arg) == "service" {
				count++
			}
		}
		require.Equal(t, 1, count, "service count in %q", args)
	}
}

// RFC requirement: RFC8907-8.2-2 positive -- every request with service=shell carries cmd: the command request with cmd=show, the session request with an empty cmd, and the accounting START and STOP records.
func TestRFC8907EveryShellRequestCarriesCmd(t *testing.T) {
	command, session := authorRequests(t)
	start, stop := accountingRecords(t, "show bgp summary")
	require.Contains(t, command, "cmd=show")
	require.Contains(t, session, "cmd=")
	for _, record := range []*AcctRequest{start, stop} {
		value, ok := argValue(record.Args, "cmd")
		require.True(t, ok, "accounting record lacks cmd")
		require.Equal(t, "show", value)
	}
}

// RFC requirement: RFC8907-8.2-2 negative -- a shell request whose command is empty or has no arguments still carries cmd: the session request keeps cmd= and the records for "show" keep cmd=show.
func TestRFC8907ShellRequestNeverLacksCmd(t *testing.T) {
	_, session := authorRequests(t)
	start, stop := accountingRecords(t, "show")
	require.NotEqual(t, -1, argIndex(session, "cmd"), "the session request lacks cmd")
	for _, record := range []*AcctRequest{start, stop} {
		require.NotEqual(t, -1, argIndex(record.Args, "cmd"), "the accounting record lacks cmd")
	}
}

// RFC requirement: RFC8907-8.1-2 positive -- stop_time is the number of seconds since the epoch, the UTC reading of the clock.
func TestRFC8907StopTimeIsEpochSeconds(t *testing.T) {
	acct := newQueuedAccountant()
	before := time.Now().Unix()
	acct.CommandStop("7", "alice", "192.0.2.1", "show version")
	after := time.Now().Unix()
	value, ok := argValue(dequeue(t, acct).Args, "stop_time")
	require.True(t, ok, "stop_time missing")
	seconds, err := strconv.ParseInt(value, 10, 64)
	require.NoError(t, err)
	require.GreaterOrEqual(t, seconds, before)
	require.LessOrEqual(t, seconds, after)
}

// RFC requirement: RFC8907-8.1-2 negative -- with the process zone set five hours east of UTC the stop_time does not move by that offset, and no timezone argument is emitted.
func TestRFC8907StopTimeIgnoresLocalZone(t *testing.T) {
	saved := time.Local
	t.Cleanup(func() { time.Local = saved })
	time.Local = time.FixedZone("east", 5*60*60)

	acct := newQueuedAccountant()
	before := time.Now().UTC().Unix()
	acct.CommandStop("7", "alice", "192.0.2.1", "show version")
	after := time.Now().UTC().Unix()
	stop := dequeue(t, acct)
	value, ok := argValue(stop.Args, "stop_time")
	require.True(t, ok, "stop_time missing")
	seconds, err := strconv.ParseInt(value, 10, 64)
	require.NoError(t, err)
	require.GreaterOrEqual(t, seconds, before)
	require.LessOrEqual(t, seconds, after)
	_, hasZone := argValue(stop.Args, "timezone")
	require.False(t, hasZone, "no timezone argument is emitted, so the value must be UTC")
}

// RFC requirement: RFC8907-10.5.4-1 positive -- the shared values Ze consumes keep their Section 8.2 meaning: priv-lvl 0, 1 and 15 select the profiles of levels 0, 1 and 15, and a replacement naming the same shell command with the same cmd-arg values in order authorizes that command.
func TestRFC8907SharedArgumentsKeepDefinedMeaning(t *testing.T) {
	for level, profile := range map[string]string{"0": "guest", "1": "ops", "15": "admin"} {
		auth := sessionPolicyAuthenticator(t, AuthorStatusPassAdd, []string{"priv-lvl=" + level}, nil)
		result, err := auth.Authenticate(aliceRequest)
		require.NoError(t, err, "priv-lvl=%s", level)
		require.Equal(t, []string{profile}, result.Profiles, "priv-lvl=%s", level)
	}
	authz, _ := commandPolicyAuthorizer(t, AuthorStatusPassRepl, []string{
		"service=shell", "cmd=show", "cmd-arg=bgp", "cmd-arg=summary",
	})
	require.True(t, authz.AuthorizeCommandArgs("alice", "192.0.2.1", "show", []string{"bgp", "summary"}, "", true))
}

// RFC requirement: RFC8907-10.5.4-1 negative -- a shared value outside its Section 8.2 definition is never given a nearby meaning: priv-lvl 16, " 15", "0x0f", "1.5" and a full-width "15" grant no profile, and a replacement whose cmd or cmd-arg differs in case or order does not authorize the command.
func TestRFC8907SharedArgumentsNeverReinterpreted(t *testing.T) {
	for _, level := range []string{"16", " 15", "0x0f", "1.5"} {
		auth := sessionPolicyAuthenticator(t, AuthorStatusPassAdd, []string{"priv-lvl=" + level}, nil)
		result, err := auth.Authenticate(aliceRequest)
		require.True(t, errors.Is(err, aaa.ErrAuthRejected), "priv-lvl=%q: %v", level, err)
		require.Empty(t, result.Profiles, "priv-lvl=%q", level)
	}
	// Full-width digits are not US-ASCII, so the reply carrying them is refused
	// as a whole (Section 3.7) before any argument is read: still no profile.
	auth := sessionPolicyAuthenticator(t, AuthorStatusPassAdd, []string{"priv-lvl=１５"}, nil)
	result, err := auth.Authenticate(aliceRequest)
	require.Error(t, err)
	require.False(t, result.Authenticated)
	require.Empty(t, result.Profiles)
	for _, replacement := range [][]string{
		{"service=shell", "cmd=SHOW", "cmd-arg=bgp", "cmd-arg=summary"},
		{"service=shell", "cmd=show", "cmd-arg=BGP", "cmd-arg=summary"},
		{"service=shell", "cmd=show", "cmd-arg=summary", "cmd-arg=bgp"},
	} {
		authz, _ := commandPolicyAuthorizer(t, AuthorStatusPassRepl, replacement)
		require.False(t, authz.AuthorizeCommandArgs("alice", "192.0.2.1", "show", []string{"bgp", "summary"}, "", true), "%q", replacement)
	}
}

// requestServices returns the authen_service octet of every request Ze sends
// for one login and one command: the PAP START on the wire, the session and
// command authorization requests on the wire, and the accounting START and STOP.
func requestServices(t *testing.T) map[string]uint8 {
	t.Helper()
	services := make(chan uint8, 3)
	login := newProfileServer(t, sessionKey, func(hdr PacketHeader, body []byte) []byte {
		services <- body[3]
		if hdr.Type == typeAuthorization {
			return authorReplyArgs(AuthorStatusPassAdd, []string{"priv-lvl=15"})(hdr, body)
		}
		return passReply()(hdr, body)
	})
	loginClient := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: login.addr(), Key: sessionKey}},
		Timeout: 2 * time.Second,
	})
	t.Cleanup(loginClient.Close)
	_, err := newTacacsAuthenticator(loginClient, map[int][]string{15: {"admin"}}, nil).Authenticate(aliceRequest)
	require.NoError(t, err)
	found := map[string]uint8{"authentication START": <-services, "session authorization": <-services}

	command := newTestServer(t, sessionKey, func(hdr PacketHeader, body []byte) []byte {
		services <- body[3]
		return authorReply(AuthorStatusPassAdd)(hdr, body)
	})
	require.True(t, newTacacsAuthorizer(validationClient(t, command), &fakeLocalAuthz{allow: true}).Authorize("alice", "192.0.2.1", "show version", true))
	found["command authorization"] = <-services

	start, stop := accountingRecords(t, "show version")
	found["accounting START"] = start.AuthenService
	found["accounting STOP"] = stop.AuthenService
	return found
}

// RFC requirement: RFC8907-5.4.2.6-2 positive -- the PAP login, which is not an ENABLE request, puts TAC_PLUS_AUTHEN_SVC_LOGIN (0x01) in the authen_service octet of the START on the wire.
func TestRFC8907PAPLoginOnWireCarriesLoginService(t *testing.T) {
	require.Equal(t, uint8(authenServiceLogin), requestServices(t)["authentication START"])
}

// RFC requirement: RFC8907-5.4.2.6-2 negative -- no request Ze sends, none of which is an ENABLE request, carries TAC_PLUS_AUTHEN_SVC_ENABLE (0x02): not the PAP START, not the session or command authorization request, not the accounting START or STOP.
func TestRFC8907NoRequestCarriesEnableService(t *testing.T) {
	services := requestServices(t)
	require.Len(t, services, 5)
	for request, service := range services {
		require.NotEqual(t, uint8(0x02), service, "%s carries TAC_PLUS_AUTHEN_SVC_ENABLE", request)
	}
}
