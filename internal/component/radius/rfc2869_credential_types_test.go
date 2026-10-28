// RFC: rfc/short/rfc2869.md -- RFC2869-5.19-1, one credential type in an Access-Request
// Related: client.go -- encodeRequest and oneCredentialType, the guard at the socket
// Related: rfc2865_wire_rules_test.go -- datagramCaptureServer

// The L2TP builder (authradius buildAuthAttrs) never produces an EAP-Message,
// so the EAP combinations of the Section 5.19 note are driven here, at the
// client every Access-Request leaves through. ARAP-Password is recognized by
// the literal type 70 (client.go credentialTypeARAPPassword) and driven beside
// EAP-Message here, while dict.go still declares no ARAP attribute
// (rfc2869_unoffered_service_attributes_test.go).
package radius

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// eapCredentialRequest is an Access-Request carrying an EAP-Message and its
// Message-Authenticator placeholder, plus any extra attributes.
func eapCredentialRequest(t *testing.T, id uint8, extra ...Attr) *Packet {
	t.Helper()
	pkt := accessRequest(t, id)
	pkt.Attrs = append(pkt.Attrs,
		Attr{Type: AttrEAPMessage, Value: []byte{2, 1, 0, 5, 1}},
	)
	pkt.Attrs = append(pkt.Attrs, extra...)
	return pkt
}

// credentialExchange sends pkt through Client.Exchange to a server that
// answers with an Access-Accept, and answers the datagrams the server read
// and the error Exchange returned.
func credentialExchange(t *testing.T, build func(id uint8) *Packet) ([][]byte, error) {
	t.Helper()
	key := []byte("testing123")
	srv := newDatagramCaptureServer(t, func(_ int, req []byte) []byte { return buildResponse(CodeAccessAccept, req, key) })
	client, err := NewClient(ClientConfig{Timeout: 200 * time.Millisecond, Retries: 1})
	require.NoError(t, err)
	defer closeSilent(client)
	_, exchangeErr := client.Exchange(context.Background(), build(client.NextID()), Server{Address: srv.addr, SharedKey: key})
	// A refused request is refused before the socket, so a short wait is enough
	// to show that nothing arrived.
	time.Sleep(50 * time.Millisecond)
	datagrams, _ := srv.snapshot()
	return datagrams, exchangeErr
}

// RFC requirement: RFC2869-5.19-1 positive -- an Access-Request carrying
// EAP-Message attributes and none of User-Password, CHAP-Password or
// ARAP-Password is sent: Client.Exchange succeeds, and the server reads one
// datagram whose EAP-Message is present and which carries none of the other
// three (client.go encodeRequest, oneCredentialType).
func TestRFC2869EAPOnlyAccessRequestIsSent(t *testing.T) {
	datagrams, err := credentialExchange(t, func(id uint8) *Packet { return eapCredentialRequest(t, id) })
	require.NoError(t, err)
	require.Len(t, datagrams, 1)
	sent, decodeErr := Decode(datagrams[0])
	require.NoError(t, decodeErr)
	assert.NotNil(t, sent.FindAttr(AttrEAPMessage))
	assert.Nil(t, sent.FindAttr(AttrUserPassword))
	assert.Nil(t, sent.FindAttr(AttrCHAPPassword))
	assert.Nil(t, sent.FindAttr(credentialTypeARAPPassword))
}

// RFC requirement: RFC2869-5.19-1 negative -- an Access-Request carrying
// EAP-Message beside a User-Password, beside a CHAP-Password, or beside an
// ARAP-Password, is refused by Client.Exchange with an error, and no datagram
// reaches the server (client.go encodeRequest, oneCredentialType).
func TestRFC2869EAPBesideAnotherCredentialIsRefused(t *testing.T) {
	others := map[string]Attr{
		"User-Password": {Type: AttrUserPassword, Value: []byte("pw")},
		"CHAP-Password": {Type: AttrCHAPPassword, Value: make([]byte, 17)},
		"ARAP-Password": {Type: credentialTypeARAPPassword, Value: make([]byte, 16)},
	}
	for name, other := range others {
		datagrams, err := credentialExchange(t, func(id uint8) *Packet { return eapCredentialRequest(t, id, other) })
		require.Error(t, err, "EAP-Message beside %s", name)
		assert.Empty(t, datagrams, "EAP-Message beside %s reached the server", name)
	}
}
