// Design: docs/architecture/aaa-tacacs.md
// Related: client.go, reply.go, text.go -- live request and reply validation.
// RFC 8907 Sections 3.7, 4.1, 4.4 and 4.5 -- see rfc/short/rfc8907.md.
package tacacs

import (
	"bytes"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func validationReply(kind, status uint8) []byte {
	if kind == typeAccounting {
		return []byte{0, 0, 0, 0, status}
	}
	return []byte{status, 0, 0, 0, 0, 0}
}

func validationExchange(client *TacacsClient, kind uint8, username string) (uint8, error) {
	switch kind {
	case typeAuthentication:
		reply, err := client.Authenticate(username, "secret", "ssh", "192.0.2.1")
		if err != nil {
			return 0, err
		}
		return reply.Status, nil
	case typeAuthorization:
		reply, err := client.SendAuthorization(&AuthorRequest{
			User: username, Port: "ssh", RemAddr: "192.0.2.1",
			Args: []string{"service=shell", "cmd=show"},
		})
		if err != nil {
			return 0, err
		}
		return reply.Status, nil
	default:
		reply, err := client.SendAccounting(&AcctRequest{
			Flags: AcctFlagStart, User: username, Port: "ssh", RemAddr: "192.0.2.1",
			Args: []string{"task_id=1", "service=shell", "cmd=show"},
		})
		if err != nil {
			return 0, err
		}
		return reply.Status, nil
	}
}

func validationClient(t *testing.T, servers ...*testTacacsServer) *TacacsClient {
	t.Helper()
	cfg := TacacsClientConfig{Timeout: time.Second}
	for _, server := range servers {
		cfg.Servers = append(cfg.Servers, TacacsServer{Address: server.addr(), Key: sessionKey})
		t.Cleanup(server.close)
	}
	client := NewTacacsClient(cfg)
	t.Cleanup(client.Close)
	return client
}

// RFC requirement: RFC8907-3.7-1 positive -- authentication, authorization and accounting requests transmit width-mapped NFC usernames while preserving case.
func TestRFC8907UsernameProfileOnWire(t *testing.T) {
	for _, kind := range []uint8{typeAuthentication, typeAuthorization, typeAccounting} {
		seen := make(chan []byte, 1)
		server := newTestServer(t, sessionKey, func(_ PacketHeader, body []byte) []byte {
			seen <- body
			return validationReply(kind, 1)
		})
		client := validationClient(t, server)
		status, err := validationExchange(client, kind, "Ａlice\u0301")
		if err != nil || status != 1 {
			t.Fatalf("kind %d: status %d, error %v", kind, status, err)
		}
		body := <-seen
		offset, length := 8, int(body[4])
		if kind == typeAuthorization {
			offset += int(body[7])
		}
		if kind == typeAccounting {
			offset, length = 9+int(body[8]), int(body[5])
		}
		if got := string(body[offset : offset+length]); got != "Alicé" {
			t.Fatalf("kind %d username %q, want case-preserved Alicé", kind, got)
		}
	}
}

// RFC requirement: RFC8907-3.7-1 negative -- a username forbidden by PRECIS is refused before any server request for every client service.
func TestRFC8907ForbiddenUsernameNeverSent(t *testing.T) {
	for _, username := range []string{"alice\x00", "alice\ue000", "a\u200db", "\xff", "\u05d0a"} {
		for _, kind := range []uint8{typeAuthentication, typeAuthorization, typeAccounting} {
			var contacted atomic.Bool
			server := newTestServer(t, sessionKey, func(_ PacketHeader, _ []byte) []byte {
				contacted.Store(true)
				return validationReply(kind, 1)
			})
			client := validationClient(t, server)
			_, err := validationExchange(client, kind, username)
			if !errors.Is(err, errRequestInvalid) || contacted.Load() {
				t.Fatalf("kind %d accepted forbidden username: error %v, contacted %v", kind, err, contacted.Load())
			}
		}
	}
}

// RFC requirement: RFC8907-3.7-2 positive -- printable ASCII messages and PAP password punctuation survive a real obfuscated exchange unchanged.
func TestRFC8907PrintableTextRoundTrip(t *testing.T) {
	const password = " !\"#$%&'()*+,-./0123456789:;<=>?@AZ[\\]^_`az{|}~"
	seen := make(chan []byte, 1)
	server := newTestServer(t, sessionKey, func(hdr PacketHeader, body []byte) []byte {
		seen <- body
		return authenReply(AuthenStatusPass, password, []byte{0, 255})(hdr, body)
	})
	client := validationClient(t, server)
	reply, err := client.Authenticate("alice", password, "ssh", "192.0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	body := <-seen
	if got := string(body[8+int(body[4])+int(body[5])+int(body[6]):]); got != password {
		t.Fatalf("PAP password changed: %q", got)
	}
	if reply.ServerMsg != password || !bytes.Equal(reply.Data, []byte{0, 255}) {
		t.Fatalf("printable message or binary authentication data changed: %#v", reply)
	}
}

// RFC requirement: RFC8907-3.7-2 negative -- control or non-ASCII text in outgoing fields and incoming display strings is rejected rather than displayed or sent.
func TestRFC8907NonPrintableTextRejected(t *testing.T) {
	for _, value := range []string{"control\n", "delete\x7f", "café"} {
		start := NewPAPAuthenStart("alice", value, "ssh", "192.0.2.1")
		if _, err := start.MarshalBinary(); err == nil {
			t.Fatal("non-ASCII PAP password accepted")
		}
		request := &AuthorRequest{User: "alice", Args: []string{"cmd=" + value}}
		if _, err := request.MarshalBinary(); err == nil {
			t.Fatal("non-printable authorization argument accepted")
		}
		account := &AcctRequest{Flags: AcctFlagStart, User: "alice", Port: value}
		if _, err := account.MarshalBinary(); err == nil {
			t.Fatal("non-printable accounting port accepted")
		}
		server := newTestServer(t, sessionKey, authenReply(AuthenStatusPass, value, nil))
		client := validationClient(t, server)
		if reply, err := client.Authenticate("alice", "secret", "ssh", "192.0.2.1"); err == nil || reply != nil {
			t.Fatalf("invalid display message reached caller: %#v, %v", reply, err)
		}
	}
}

// RFC requirement: RFC8907-4.1-3 positive -- unknown header flags are ignored, and a valid PASS still reaches the client.
func TestRFC8907UnknownHeaderFlagsIgnored(t *testing.T) {
	server := newTestServerWithHeader(t, sessionKey, passReply(), func(_ PacketHeader, reply PacketHeader) PacketHeader {
		reply.Flags = 0xf2
		return reply
	})
	client := validationClient(t, server)
	if status, err := validationExchange(client, typeAuthentication, "alice"); err != nil || status != AuthenStatusPass {
		t.Fatalf("unknown flag bits refused valid reply: %d, %v", status, err)
	}
}

// RFC requirement: RFC8907-4.1-3 negative -- ignoring unknown bits does not ignore the defined unencrypted flag, which turns an apparent PASS into FAIL.
func TestRFC8907UnknownFlagsDoNotMaskDowngrade(t *testing.T) {
	server := newTestServerWithHeader(t, sessionKey, passReply(), func(_ PacketHeader, reply PacketHeader) PacketHeader {
		reply.Flags = 0x80 | FlagUnencrypted
		return reply
	})
	client := validationClient(t, server)
	if status, err := validationExchange(client, typeAuthentication, "alice"); err != nil || status != AuthenStatusFail {
		t.Fatalf("downgrade with unknown bits: %d, %v", status, err)
	}
}

// RFC requirement: RFC8907-4.4-1 positive -- ERROR from the first server falls through to a backup for authentication, authorization and accounting.
func TestRFC8907ErrorUsesBackupServer(t *testing.T) {
	for _, kind := range []uint8{typeAuthentication, typeAuthorization, typeAccounting} {
		errorStatus := uint8(AuthenStatusError)
		if kind == typeAuthorization {
			errorStatus = AuthorStatusError
		}
		if kind == typeAccounting {
			errorStatus = AcctStatusError
		}
		primary := newTestServer(t, sessionKey, func(_ PacketHeader, _ []byte) []byte {
			return validationReply(kind, errorStatus)
		})
		var contacted atomic.Bool
		backup := newTestServer(t, sessionKey, func(_ PacketHeader, _ []byte) []byte {
			contacted.Store(true)
			return validationReply(kind, 1)
		})
		client := validationClient(t, primary, backup)
		status, err := validationExchange(client, kind, "alice")
		if err != nil || status != 1 || !contacted.Load() {
			t.Fatalf("kind %d failed to use backup: status %d, error %v, contacted %v", kind, status, err, contacted.Load())
		}
	}
}

// RFC requirement: RFC8907-4.4-1 negative -- an explicit FAIL is terminal and cannot be replaced by a backup PASS for authentication or authorization.
func TestRFC8907FailDoesNotUseBackupServer(t *testing.T) {
	for _, kind := range []uint8{typeAuthentication, typeAuthorization} {
		failStatus := uint8(AuthenStatusFail)
		if kind == typeAuthorization {
			failStatus = AuthorStatusFail
		}
		primary := newTestServer(t, sessionKey, func(_ PacketHeader, _ []byte) []byte {
			return validationReply(kind, failStatus)
		})
		var contacted atomic.Bool
		backup := newTestServer(t, sessionKey, func(_ PacketHeader, _ []byte) []byte {
			contacted.Store(true)
			return validationReply(kind, 1)
		})
		client := validationClient(t, primary, backup)
		status, err := validationExchange(client, kind, "alice")
		if err != nil || status != failStatus || contacted.Load() {
			t.Fatalf("kind %d overrode rejection: status %d, error %v, backup contacted %v", kind, status, err, contacted.Load())
		}
	}
}

// RFC requirement: RFC8907-4.6-1 positive -- replies whose component lengths exactly consume the header body length are accepted by each live client service.
func TestRFC8907ExactReplyLengthsAccepted(t *testing.T) {
	for _, kind := range []uint8{typeAuthentication, typeAuthorization, typeAccounting} {
		server := newTestServer(t, sessionKey, func(_ PacketHeader, _ []byte) []byte {
			return validationReply(kind, 1)
		})
		client := validationClient(t, server)
		if status, err := validationExchange(client, kind, "alice"); err != nil || status != 1 {
			t.Fatalf("kind %d exact length rejected: %d, %v", kind, status, err)
		}
	}
}

// RFC requirement: RFC8907-4.6-1 negative -- trailing decrypted bytes outside the component lengths invalidate a PASS for every service.
func TestRFC8907TrailingReplyBytesRejected(t *testing.T) {
	for _, kind := range []uint8{typeAuthentication, typeAuthorization, typeAccounting} {
		server := newTestServer(t, sessionKey, func(_ PacketHeader, _ []byte) []byte {
			return append(validationReply(kind, 1), 'x')
		})
		client := validationClient(t, server)
		if status, err := validationExchange(client, kind, "alice"); err == nil || status != 0 {
			t.Fatalf("kind %d ignored trailing body: %d, %v", kind, status, err)
		}
	}
}

// Reject an absent PAP username and overlong normalized names before writing.
func TestRFC8907PAPRequiresPresentBoundedUsername(t *testing.T) {
	for _, username := range []string{"", strings.Repeat("a", 256)} {
		server := newTestServer(t, sessionKey, passReply())
		client := validationClient(t, server)
		_, err := client.Authenticate(username, "secret", "ssh", "192.0.2.1")
		if !errors.Is(err, errRequestInvalid) {
			t.Fatalf("username length %d produced %v", len(username), err)
		}
	}
}

// Fuzz all reply decoders with arbitrary body lengths, including five-byte
// accounting replies that are shorter than the other services' fixed headers.
func FuzzTACACSReplyBodies(f *testing.F) {
	f.Add([]byte{0, 0, 0, 0, AcctStatusSuccess})
	f.Add([]byte{AuthenStatusPass, 0, 0, 0, 0, 0})
	f.Add([]byte{AuthorStatusPassAdd, 1, 0, 0, 0, 0, 1, 'x'})
	f.Fuzz(func(_ *testing.T, body []byte) {
		_, _ = UnmarshalAuthenReply(body)
		_, _ = UnmarshalAuthorResponse(body)
		_, _ = UnmarshalAcctReply(body)
	})
}

// RFC requirement: RFC8907-7-1 negative -- SendAccounting refuses a START with the deprecated MORE bit before the request reaches a server.
func TestRFC8907AccountingMoreFlagRefused(t *testing.T) {
	client := validationClient(t, newTestServer(t, sessionKey, func(_ PacketHeader, _ []byte) []byte {
		return validationReply(typeAccounting, AcctStatusSuccess)
	}))
	_, err := client.SendAccounting(&AcctRequest{Flags: AcctFlagStart | 0x01, User: "alice"})
	if !errors.Is(err, errRequestInvalid) {
		t.Fatalf("MORE flag was not refused locally: %v", err)
	}
}

// RFC requirement: RFC8907-7-2 negative -- a combined START and STOP request is refused rather than encoded as an ambiguous accounting event.
func TestRFC8907AccountingStartStopCombinationRefused(t *testing.T) {
	client := validationClient(t, newTestServer(t, sessionKey, func(_ PacketHeader, _ []byte) []byte {
		return validationReply(typeAccounting, AcctStatusSuccess)
	}))
	_, err := client.SendAccounting(&AcctRequest{Flags: AcctFlagStart | AcctFlagStop, User: "alice"})
	if !errors.Is(err, errRequestInvalid) {
		t.Fatalf("START plus STOP was not refused locally: %v", err)
	}
}

// RFC requirement: RFC8907-7.2-1 negative -- a STOP carrying WATCHDOG cannot reach the accounting server.
func TestRFC8907AccountingStopWatchdogCombinationRefused(t *testing.T) {
	client := validationClient(t, newTestServer(t, sessionKey, func(_ PacketHeader, _ []byte) []byte {
		return validationReply(typeAccounting, AcctStatusSuccess)
	}))
	_, err := client.SendAccounting(&AcctRequest{Flags: AcctFlagStop | AcctFlagWatchdog, User: "alice"})
	if !errors.Is(err, errRequestInvalid) {
		t.Fatalf("STOP plus WATCHDOG was not refused locally: %v", err)
	}
}
