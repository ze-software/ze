// Design: docs/architecture/testing/ci-format.md -- TACACS+ fixture policy

package tacacs

import (
	"encoding/base64"
	"io"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/aaa"
	protocol "github.com/ze-software/ze/internal/component/tacacs"
)

// Explicit observer grants must not turn unknown users or arbitrary observer
// commands into successful authorization, and a deny rule remains decisive.
func TestObserverAuthorizationGrantIsExact(t *testing.T) {
	principal := aaa.ReservedInternalPrefix + "plugin:tacacs-author-test"
	wireUser := "~ze~r:" + base64.RawURLEncoding.EncodeToString([]byte(principal))
	allow := []string{wireUser + "=request quiesce", wireUser + "=request shutdown"}
	for _, tc := range []struct {
		name string
		user string
		args []string
		deny []string
		want uint8
	}{
		{"quiesce", principal, []string{"cmd=request", "cmd-arg=quiesce"}, nil, protocol.AuthorStatusPassAdd},
		{"shutdown", principal, []string{"cmd=request", "cmd-arg=shutdown"}, nil, protocol.AuthorStatusPassAdd},
		{"unknown user", "unknown", []string{"cmd=request", "cmd-arg=shutdown"}, nil, protocol.AuthorStatusFail},
		{"other observer", aaa.ReservedInternalPrefix + "plugin:other-test", []string{"cmd=request", "cmd-arg=shutdown"}, nil, protocol.AuthorStatusFail},
		{"printable impersonator", wireUser, []string{"cmd=request", "cmd-arg=shutdown"}, nil, protocol.AuthorStatusFail},
		{"other command", principal, []string{"cmd=show", "cmd-arg=bgp"}, nil, protocol.AuthorStatusFail},
		{"extra argument", principal, []string{"cmd=request", "cmd-arg=shutdown", "cmd-arg=now"}, nil, protocol.AuthorStatusFail},
		{"session grant", principal, []string{"cmd="}, nil, protocol.AuthorStatusFail},
		{"explicit deny", principal, []string{"cmd=request", "cmd-arg=shutdown"}, []string{"shutdown"}, protocol.AuthorStatusFail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := &protocol.AuthorRequest{User: tc.user, Args: append([]string{"service=shell"}, tc.args...)}
			body, err := request.MarshalBinary()
			if err != nil {
				t.Fatal(err)
			}
			replyBody := mockReplyBody(t, 0x02, func(conn net.Conn, header protocol.PacketHeader, key []byte) {
				tacacsMockReplyAuthor(conn, header, body, key, tacacsUserList{{name: "admin", pass: "testpass", privLvl: 15}}, tc.deny, allow, 0, false)
			})
			reply, err := protocol.UnmarshalAuthorResponse(replyBody)
			if err != nil {
				t.Fatal(err)
			}
			if reply.Status != tc.want {
				t.Fatalf("authorization status = %#x, want %#x", reply.Status, tc.want)
			}
			if len(reply.Args) != 0 {
				t.Fatalf("command grant assigned session policy: %v", reply.Args)
			}
		})
	}
}

// A lifecycle authorization grant is not a login credential, including for a
// human who submits the printable wire spelling of the observer identity.
func TestObserverIdentityCannotAuthenticate(t *testing.T) {
	principal := aaa.ReservedInternalPrefix + "plugin:tacacs-author-test"
	wireUser := "~ze~r:" + base64.RawURLEncoding.EncodeToString([]byte(principal))
	request := protocol.NewPAPAuthenStart(wireUser, "testpass", "ssh", "")
	body, err := request.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	replyBody := mockReplyBody(t, 0x01, func(conn net.Conn, header protocol.PacketHeader, key []byte) {
		tacacsMockReplyAuthen(conn, header, body, key, tacacsUserList{{name: "admin", pass: "testpass", privLvl: 15}}, 0, false)
	})
	reply, err := protocol.UnmarshalAuthenReply(replyBody)
	if err != nil {
		t.Fatal(err)
	}
	if reply.Status != protocol.AuthenStatusFail {
		t.Fatalf("observer wire identity authenticated: status=%#x", reply.Status)
	}
}

func mockReplyBody(t *testing.T, kind uint8, respond func(net.Conn, protocol.PacketHeader, []byte)) []byte {
	t.Helper()
	client, server := net.Pipe()
	t.Cleanup(func() { _ = client.Close() })
	t.Cleanup(func() { _ = server.Close() })
	if err := client.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	key := []byte("fixture-key")
	header := protocol.PacketHeader{Version: 0xc0, Type: kind, SeqNo: 1, SessionID: 7}
	if kind == 0x01 {
		header.Version = 0xc1
	}
	go respond(server, header, key)
	var encodedHeader [12]byte
	if _, err := io.ReadFull(client, encodedHeader[:]); err != nil {
		t.Fatal(err)
	}
	replyHeader, err := protocol.UnmarshalPacketHeader(encodedHeader[:])
	if err != nil {
		t.Fatal(err)
	}
	body := make([]byte, replyHeader.Length)
	if _, err := io.ReadFull(client, body); err != nil {
		t.Fatal(err)
	}
	protocol.Encrypt(body, replyHeader.SessionID, key, replyHeader.Version, replyHeader.SeqNo)
	return body
}

// VALIDATES: a Record renders exactly the stderr line the mock has always
// printed for it, because .ci files assert those lines (tacacs-acct.ci,
// tacacs-author.ci, tacacs-singleconnect.ci) and the fixture now waits on the
// record instead of sleeping.
// PREVENTS: the record a fixture waits on and the line a .ci asserts drifting
// apart, so a wait succeeds on a record whose line the .ci no longer finds.
func TestRecordLineIsTheStderrLine(t *testing.T) {
	for _, tc := range []struct {
		record Record
		want   string
	}{
		{Record{Kind: KindConnection, Ordinal: 1, Remote: "127.0.0.1:4000"}, "tacacs-mock: connection #1 from 127.0.0.1:4000"},
		{Record{Kind: KindSingleConnect, Remote: "127.0.0.1:4000"}, "tacacs-mock: single-connect accepted on 127.0.0.1:4000"},
		{Record{Kind: KindAuthen, User: "admin", Status: 0x01, Pass: true}, `tacacs-mock: AUTHEN reply user="admin" status=0x01`},
		{Record{Kind: KindAuthor, Command: "show bgp", Pass: true, User: "admin", Privilege: ""}, `tacacs-mock: AUTHOR cmd="show bgp" reply=PASS_ADD user="admin" privilege=""`},
		{Record{Kind: KindAuthor, Command: "clear interface counters all", User: "admin"}, `tacacs-mock: AUTHOR cmd="clear interface counters all" reply=FAIL user="admin" privilege=""`},
		{Record{Kind: KindAcctStart, Seq: 1, Ordinal: 3}, "tacacs-mock: ACCT START seq=1 total=3"},
		{Record{Kind: KindAcctStop, Seq: 1, Ordinal: 4}, "tacacs-mock: ACCT STOP seq=1 total=4"},
		{Record{Kind: KindAcctWatchdog, Seq: 1, Ordinal: 5}, "tacacs-mock: ACCT WATCHDOG seq=1 total=5"},
		{Record{Kind: KindAcctOther, Seq: 1, Ordinal: 6}, "tacacs-mock: ACCT OTHER seq=1 total=6"},
	} {
		if got := tc.record.Line(); got != tc.want {
			t.Errorf("Line() = %q, want %q", got, tc.want)
		}
	}
}

// VALIDATES: every reply the mock writes leaves a Record a caller in the same
// process reads through Received, after its stderr line is written.
// PREVENTS: a fixture having nothing to wait on but a clock.
// The journal is per process and other tests in this package write to it too,
// so the records are found by a user name no other test uses.
func TestReceivedHoldsWhatTheMockTook(t *testing.T) {
	const user = "journal-check"
	before := len(Received())
	users := tacacsUserList{{name: user, pass: "testpass", privLvl: 15}}

	authen, err := protocol.NewPAPAuthenStart(user, "testpass", "ssh", "").MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	mockReplyBody(t, 0x01, func(conn net.Conn, header protocol.PacketHeader, key []byte) {
		tacacsMockReplyAuthen(conn, header, authen, key, users, 0, false)
	})
	receivedOne(t, before, func(r Record) bool { return r.Kind == KindAuthen && r.User == user && r.Pass })

	author, err := (&protocol.AuthorRequest{User: user, Args: []string{"service=shell", "cmd=show", "cmd-arg=bgp"}}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	mockReplyBody(t, 0x02, func(conn net.Conn, header protocol.PacketHeader, key []byte) {
		tacacsMockReplyAuthor(conn, header, author, key, users, nil, nil, 0, false)
	})
	receivedOne(t, before, func(r Record) bool {
		return r.Kind == KindAuthor && r.User == user && r.Command == "show bgp" && r.Pass
	})

	mockReplyBody(t, 0x03, func(conn net.Conn, header protocol.PacketHeader, key []byte) {
		tacacsMockReplyAcct(conn, header, []byte{protocol.AcctFlagStop}, key, 0, false)
	})
	receivedOne(t, before, func(r Record) bool { return r.Kind == KindAcctStop })
}

// receivedOne waits until a record past index from matches want. The mock
// keeps an AUTHEN or AUTHOR record after its reply is written, so the reply a
// test reads can arrive a moment before the record does.
func receivedOne(t *testing.T, from int, want func(Record) bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		for _, record := range Received()[from:] {
			if want(record) {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no matching record past %d in %v", from, Received()[from:])
		}
		time.Sleep(time.Millisecond) // sleep(poll-interval): the loop returns once Received holds a matching record, within a 2s deadline
	}
}
