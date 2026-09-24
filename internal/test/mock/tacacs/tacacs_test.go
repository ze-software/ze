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
