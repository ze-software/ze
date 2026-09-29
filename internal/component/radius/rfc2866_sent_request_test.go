// RFC: rfc/short/rfc2866.md -- the Request Authenticator and Identifier rules, read off the wire
// Related: client.go -- Exchange, encodeRequest and the Acct-Delay-Time retransmit
// Related: rfc2865_wire_rules_test.go -- datagramCaptureServer

// These tests read the Accounting-Request datagrams ze really sends, rather than
// the value a helper returns, so a client that stopped applying the Section 3
// formula, or applied it before the final octets were in place, goes red.
package radius

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec // RFC 2866 Section 3 mandates MD5
	"encoding/binary"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// acctResponse answers an Accounting-Request with a valid Accounting-Response.
func acctResponse(req, secret []byte) []byte {
	var requestAuth [AuthenticatorLen]byte
	copy(requestAuth[:], req[4:4+AuthenticatorLen])
	resp := make([]byte, HeaderLen)
	resp[0] = CodeAccountingResp
	resp[1] = req[1]
	binary.BigEndian.PutUint16(resp[2:4], HeaderLen)
	auth := ResponseAuthenticator(CodeAccountingResp, req[1], HeaderLen, requestAuth, nil, secret)
	copy(resp[4:4+AuthenticatorLen], auth[:])
	return resp
}

// acctSection3Auth is an independent reference for the RFC 2866 Section 3
// formula: MD5(Code + Identifier + Length + 16 zero octets + request attributes
// + shared secret), over the datagram as it was sent.
func acctSection3Auth(datagram, secret []byte) []byte {
	length := binary.BigEndian.Uint16(datagram[2:4])
	h := md5.New() //nolint:gosec // RFC 2866 Section 3 mandates MD5
	h.Write(datagram[:4])
	h.Write(make([]byte, AuthenticatorLen))
	h.Write(datagram[HeaderLen:length])
	h.Write(secret)
	return h.Sum(nil)
}

// acctDelayTime returns the Acct-Delay-Time value of a datagram.
func acctDelayTime(t *testing.T, datagram []byte) []byte {
	t.Helper()
	pkt, err := Decode(datagram)
	require.NoError(t, err)
	value := pkt.FindAttr(AttrAcctDelayTime)
	require.Len(t, value, 4, "every Accounting-Request ze stamps carries Acct-Delay-Time")
	return value
}

// acctRecord is the Accounting-Request every test here sends.
func acctRecord() *Packet {
	return &Packet{
		Code: CodeAccountingReq,
		Attrs: []Attr{
			{Type: AttrAcctStatusType, Value: AttrUint32(AcctStatusStart)},
			{Type: AttrAcctSessionID, Value: AttrString("1-2-3")},
			{Type: AttrUserName, Value: AttrString("alice")},
		},
	}
}

// VALIDATES: the Request Authenticator of the Accounting-Request on the wire is
// the Section 3 hash of that datagram's own octets.
// PREVENTS: a client that sends the random authenticator the caller placed in
// the packet, or that hashes before the last attribute is written.
//
// RFC requirement: RFC2866-3-2 positive -- the 16 authenticator octets of the
// datagram SendToServers put on the wire equal an independent MD5 over its
// Code, Identifier and Length, 16 zero octets, its attributes and the shared
// secret, and differ from the random authenticator SendToServers placed in the
// packet (client.go Exchange, encodeRequest).
func TestRFC2866SentAccountingRequestCarriesTheSection3Authenticator(t *testing.T) {
	secret := []byte("acct-secret")
	srv := newDatagramCaptureServer(t, func(_ int, req []byte) []byte { return acctResponse(req, secret) })

	client, err := NewClient(ClientConfig{Servers: []Server{{Address: srv.addr, SharedKey: secret}}, Timeout: time.Second, Retries: 1})
	require.NoError(t, err)
	defer closeSilent(client)

	pkt := acctRecord()
	resp, err := client.SendToServers(context.Background(), pkt)
	require.NoError(t, err)
	require.Equal(t, uint8(CodeAccountingResp), resp.Code)

	datagrams, _ := srv.snapshot()
	require.Len(t, datagrams, 1)
	sent := datagrams[0]
	assert.Equal(t, acctSection3Auth(sent, secret), sent[4:4+AuthenticatorLen])
	assert.NotEqual(t, pkt.Authenticator[:], sent[4:4+AuthenticatorLen], "the random authenticator is not what goes out")
}

// VALIDATES: a retransmission that updates Acct-Delay-Time takes a new
// Identifier and a new Request Authenticator, and each datagram's authenticator
// is the Section 3 hash of its own octets.
// PREVENTS: a retransmit that keeps the old authenticator bytes, which the
// server cannot verify against the new attributes.
// The timeout is just over one second, so the retransmission's Acct-Delay-Time
// is 1 where the first is 0: the attributes really change, and no reply has
// been received, so the attribute trigger of Section 4.1 is the only one.
//
// RFC requirement: RFC2866-3-3 positive -- the server drops the first datagram;
// the retransmission carries a different Acct-Delay-Time value, a different
// Identifier and a different Request Authenticator, and each of the two
// authenticators equals the Section 3 MD5 of its own datagram (client.go
// Exchange).
// RFC requirement: RFC2866-4.1-4 positive -- with no reply received for the
// first datagram, a change of the Attributes field alone gives the
// retransmission a new Identifier (client.go Exchange).
func TestRFC2866AccountingRetransmitTakesANewIdentifierAndRequestAuthenticator(t *testing.T) {
	secret := []byte("acct-secret")
	srv := newDatagramCaptureServer(t, func(index int, req []byte) []byte {
		if index == 0 {
			return nil
		}
		return acctResponse(req, secret)
	})

	client, err := NewClient(ClientConfig{Servers: []Server{{Address: srv.addr, SharedKey: secret}}, Timeout: 1100 * time.Millisecond, Retries: 2})
	require.NoError(t, err)
	defer closeSilent(client)

	_, err = client.SendToServers(context.Background(), acctRecord())
	require.NoError(t, err)

	datagrams, _ := srv.snapshot()
	require.Len(t, datagrams, 2, "the first datagram went unanswered and was resent once")
	first, again := datagrams[0], datagrams[1]
	require.NotEqual(t, acctDelayTime(t, first), acctDelayTime(t, again), "the attributes changed")
	assert.NotEqual(t, first[1], again[1], "a new Identifier")
	assert.NotEqual(t, first[4:4+AuthenticatorLen], again[4:4+AuthenticatorLen], "a new Request Authenticator")
	assert.Equal(t, acctSection3Auth(first, secret), first[4:4+AuthenticatorLen])
	assert.Equal(t, acctSection3Auth(again, secret), again[4:4+AuthenticatorLen])
}

// VALIDATES: a record sent after a valid reply to the previous one takes a new
// Identifier even when its Attributes field is identical.
// PREVENTS: a client that changes the Identifier only when the attributes
// change, which lets the server read a new record as a duplicate.
//
// RFC requirement: RFC2866-4.1-4 positive -- two records whose attribute octets
// are identical on the wire, the second sent after the first was answered,
// carry different Identifiers (client.go SendToServers).
func TestRFC2866IdentifierChangesAfterAValidReplyWithUnchangedAttributes(t *testing.T) {
	secret := []byte("acct-secret")
	srv := newDatagramCaptureServer(t, func(_ int, req []byte) []byte { return acctResponse(req, secret) })

	client, err := NewClient(ClientConfig{Servers: []Server{{Address: srv.addr, SharedKey: secret}}, Timeout: time.Second, Retries: 1})
	require.NoError(t, err)
	defer closeSilent(client)

	for range 2 {
		_, err = client.SendToServers(context.Background(), acctRecord())
		require.NoError(t, err)
	}

	datagrams, _ := srv.snapshot()
	require.Len(t, datagrams, 2)
	require.True(t, bytes.Equal(datagrams[0][HeaderLen:], datagrams[1][HeaderLen:]), "the Attributes fields are identical")
	assert.NotEqual(t, datagrams[0][1], datagrams[1][1])
}
