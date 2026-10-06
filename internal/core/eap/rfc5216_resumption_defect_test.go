package eap

import (
	"bytes"
	"testing"
)

// TestRFC5216ResumingServerSendsNothingButChangeCipherSpecAndFinished is the
// failing test for a defect: it carries no RFC tag until the defect is fixed.
//
// RFC 5216 Section 2.1.2: "If the EAP server is resuming a previously
// established session, then it MUST include only a TLS change_cipher_spec
// message and a TLS finished handshake message after the server_hello
// message." crypto/tls, as newTLSMethod configures it, renews the session
// ticket on a TLS 1.2 resumption, so a NewSessionTicket follows server_hello.
func TestRFC5216ResumingServerSendsNothingButChangeCipherSpecAndFinished(t *testing.T) {
	_, resumed, _ := resumedTLS12Flights(t)
	if !bytes.Equal(resumed.types, []byte{tlsHSServerHello}) {
		t.Fatalf("the resuming server flight carries handshake types %v, want server_hello alone", resumed.types)
	}
	if !resumed.ccs || !bytes.Equal(resumed.afterCCS, []byte{tlsContentHandshake}) {
		t.Fatalf("the resuming server flight: change_cipher_spec %v, then records %v", resumed.ccs, resumed.afterCCS)
	}
}
