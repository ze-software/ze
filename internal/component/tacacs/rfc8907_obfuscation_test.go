// Design: (none -- new TACACS+ component)
// Detail: client.go -- the obfuscation guard on the receive path
// Detail: packet.go -- MarshalInto, which obfuscates whenever a key is configured
//
// VALIDATES: RFC 8907 Section 10.5.2. A TACACS+ client never announces an
// unobfuscated body, and it refuses a reply whose obfuscation state disagrees
// with the shared-secret configuration of the server it came from.
// PREVENTS: the downgrade this file was written for. An off-path packet that
// sets TAC_PLUS_UNENCRYPTED_FLAG and carries a cleartext TAC_PLUS_AUTHEN_STATUS_PASS
// authenticates a user with no proof of the shared secret, because a client that
// only skips de-obfuscation on that flag reads the attacker's plaintext as a reply.
//
// The tests live beside client_test.go and packet_test.go rather than inside
// them: both files carry `RFC requirement:` tags, and the pretool-writeedit hook
// refuses every edit to a tagged test file, an addition included
// (ai/rules/testing.md, "RFC-Tagged Tests").

package tacacs

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// obfuscationProbe records the first request datagram a client sends and answers
// it with a reply whose header flags and body obfuscation the caller chooses.
// It is deliberately independent of testTacacsServer, which always obfuscates a
// reply when it holds a key: the downgrade under test needs a server that does
// the opposite.
type obfuscationProbe struct {
	listener net.Listener
	key      []byte
	// replyFlags is ORed into the reply header, so a caller can set
	// TAC_PLUS_UNENCRYPTED_FLAG without changing anything else.
	replyFlags uint8
	// obfuscateReply says whether the reply body leaves in obfuscated form.
	obfuscateReply bool

	requestWire chan []byte
}

func newObfuscationProbe(t *testing.T, key []byte, replyFlags uint8, obfuscateReply bool) *obfuscationProbe {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	p := &obfuscationProbe{
		listener:       ln,
		key:            key,
		replyFlags:     replyFlags,
		obfuscateReply: obfuscateReply,
		requestWire:    make(chan []byte, 1),
	}
	t.Cleanup(func() { closeIgnore(ln) })
	go p.serve()
	return p
}

func (p *obfuscationProbe) addr() string { return p.listener.Addr().String() }

func (p *obfuscationProbe) serve() {
	conn, err := p.listener.Accept()
	if err != nil {
		return // listener closed
	}
	defer func() { closeIgnore(conn) }()

	hdrBuf := make([]byte, hdrLen)
	if _, err := io.ReadFull(conn, hdrBuf); err != nil {
		return
	}
	hdr, err := UnmarshalPacketHeader(hdrBuf)
	if err != nil {
		return
	}
	body := make([]byte, hdr.Length)
	if _, err := io.ReadFull(conn, body); err != nil {
		return
	}
	wire := make([]byte, 0, hdrLen+len(body))
	wire = append(wire, hdrBuf...)
	wire = append(wire, body...)
	select {
	case p.requestWire <- wire:
	default:
	}

	replyBody := passReply()(hdr, body)
	replyHdr := PacketHeader{
		Version:   hdr.Version,
		Type:      hdr.Type,
		SeqNo:     hdr.SeqNo + 1,
		Flags:     p.replyFlags,
		SessionID: hdr.SessionID,
		Length:    uint32(len(replyBody)),
	}
	replyWire := replyHdr.MarshalBinary()
	if p.obfuscateReply {
		Encrypt(replyBody, replyHdr.SessionID, p.key, replyHdr.Version, replyHdr.SeqNo)
	}
	replyWire = append(replyWire, replyBody...)
	if _, err := conn.Write(replyWire); err != nil {
		return // probe is best-effort
	}
}

// papPlaintext rebuilds the PAP authentication START body the client is about to
// send, so the test owns a reference the client never handed it. Comparing the
// wire body against this is what tells an obfuscated body from a cleartext one.
func papPlaintext(t *testing.T, username, password, port, remAddr string) []byte {
	t.Helper()
	start := NewPAPAuthenStart(username, password, port, remAddr)
	buf := make([]byte, maxBodyLen)
	n, err := start.MarshalBinaryInto(buf)
	require.NoError(t, err)
	return buf[:n]
}

// TestRFC8907ClientNeverSendsUnobfuscatedBody reads the octets the client put on
// the TCP connection and judges the two things Section 10.5.2 forbids a client to
// do: announce TAC_PLUS_UNENCRYPTED_FLAG, and send a body anybody can read.
//
// Asserting a marshal/unmarshal round-trip is not enough on its own. A client
// that sets the flag AND skips obfuscation round-trips perfectly, because the
// unmarshal side reads the flag and skips de-obfuscation to match. Only the wire
// bytes tell the two apart, so this test reads them.
//
// RFC requirement: RFC8907-10-1 positive -- the request header's flag octet has
// TAC_PLUS_UNENCRYPTED_FLAG (0x01) clear, and the request body on the wire is not
// the plaintext the client encoded: it is the MD5 pseudo-pad XOR of it, and it
// recovers to the plaintext only with the shared secret (packet.go MarshalInto).
func TestRFC8907ClientNeverSendsUnobfuscatedBody(t *testing.T) {
	key := []byte("obfuscation-key")
	probe := newObfuscationProbe(t, key, 0x00, true)

	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: probe.addr(), Key: key}},
		Timeout: 2 * time.Second,
	})

	reply, err := client.Authenticate("admin", "secret", "ssh", "10.0.0.1")
	require.NoError(t, err)
	require.Equal(t, uint8(0x01), reply.Status)

	var wire []byte
	select {
	case wire = <-probe.requestWire:
	case <-time.After(2 * time.Second):
		t.Fatal("the probe never saw a request datagram")
	}
	require.GreaterOrEqual(t, len(wire), hdrLen+1)

	if wire[3]&FlagUnencrypted != 0 {
		t.Fatalf("request flags octet = %#02x: the client set TAC_PLUS_UNENCRYPTED_FLAG, which RFC 8907 Section 10.5.2 forbids", wire[3])
	}

	hdr, err := UnmarshalPacketHeader(wire[:hdrLen])
	require.NoError(t, err)
	onWire := wire[hdrLen:]
	want := papPlaintext(t, "admin", "secret", "ssh", "10.0.0.1")
	require.Equal(t, len(want), len(onWire), "wire body length")

	if bytes.Equal(onWire, want) {
		t.Fatal("the request body traveled in cleartext: every credential in a PAP START is readable by anyone on the path")
	}

	// The body is the obfuscation of that plaintext and nothing else: XORing the
	// pseudo-pad back recovers it byte for byte. Without this, "not equal" would
	// also be satisfied by a body that is merely corrupt.
	recovered := bytes.Clone(onWire)
	Encrypt(recovered, hdr.SessionID, key, hdr.Version, hdr.SeqNo)
	assert.Equal(t, want, recovered, "the wire body is not the MD5 pseudo-pad obfuscation of the plaintext")
}

// TestRFC8907ClientRefusesUnobfuscatedReply drives the downgrade: a server that
// holds no secret answers a keyed client with TAC_PLUS_UNENCRYPTED_FLAG set and a
// cleartext TAC_PLUS_AUTHEN_STATUS_PASS body.
//
// RFC 8907 Section 10.5.2: "the response packet was received from the server
// configured with a shared key, but the packet has TAC_PLUS_UNENCRYPTED_FLAG set
// ... the TACACS+ client MUST close the TCP session, and process the response in
// the same way that a TAC_PLUS_AUTHEN_STATUS_FAIL (authentication sessions) or
// TAC_PLUS_AUTHOR_STATUS_FAIL (authorization sessions) was received."
//
// RFC requirement: RFC8907-10.5.2-1 negative -- the client returns an error and no
// reply, so the forged PASS never reaches the caller as an authentication result.
func TestRFC8907ClientRefusesUnobfuscatedReply(t *testing.T) {
	key := []byte("obfuscation-key")
	probe := newObfuscationProbe(t, key, FlagUnencrypted, false)

	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: probe.addr(), Key: key}},
		Timeout: 2 * time.Second,
	})

	reply, err := client.Authenticate("admin", "secret", "ssh", "10.0.0.1")
	require.Error(t, err, "a cleartext PASS from a keyed server authenticated the user")
	assert.Nil(t, reply, "the forged PASS reached the caller as an authentication result")
	// The guard names the mismatch on the log line for the operator; the caller
	// sees the server drop out of the usable set, which is the "process it as a
	// FAIL" outcome Section 10.5.2 asks for.
	assert.Contains(t, err.Error(), "unreachable")
}

// TestRFC8907ClientAcceptsObfuscatedReply is the control for the refusal above.
// The same probe, the same client and the same PASS body, differing only in the
// flag and the obfuscation, must succeed.
//
// Without it the refusal proves nothing: a client that refused every reply would
// pass the negative just as well.
//
// RFC requirement: RFC8907-10.5.2-1 positive -- a reply from a keyed server that
// clears TAC_PLUS_UNENCRYPTED_FLAG and carries an obfuscated body is accepted and
// its PASS status reaches the caller (client.go trySend).
func TestRFC8907ClientAcceptsObfuscatedReply(t *testing.T) {
	key := []byte("obfuscation-key")
	probe := newObfuscationProbe(t, key, 0x00, true)

	client := NewTacacsClient(TacacsClientConfig{
		Servers: []TacacsServer{{Address: probe.addr(), Key: key}},
		Timeout: 2 * time.Second,
	})

	reply, err := client.Authenticate("admin", "secret", "ssh", "10.0.0.1")
	require.NoError(t, err)
	require.NotNil(t, reply)
	assert.Equal(t, uint8(0x01), reply.Status)
	assert.Equal(t, "Welcome", reply.ServerMsg)
}

// TestRFC8907ReplyStatusIsReadFromTheObfuscatedBody guards the control above from
// becoming vacuous: it shows the PASS the caller sees comes from de-obfuscating
// the reply, not from a default. A body obfuscated under a different secret must
// not decode to PASS.
func TestRFC8907ReplyStatusIsReadFromTheObfuscatedBody(t *testing.T) {
	body := passReply()(PacketHeader{}, nil)
	require.Equal(t, uint8(0x01), body[0])

	scrambled := bytes.Clone(body)
	Encrypt(scrambled, 0xDEADBEEF, []byte("a-different-secret"), 0xC1, 2)
	// Read it back with the secret the client would hold.
	Encrypt(scrambled, 0xDEADBEEF, []byte("obfuscation-key"), 0xC1, 2)
	if scrambled[0] == 0x01 && binary.BigEndian.Uint16(scrambled[2:4]) == binary.BigEndian.Uint16(body[2:4]) {
		t.Fatal("a body obfuscated under another secret decoded to the same PASS reply; the fixture cannot tell the secret apart")
	}
}
