// Design: docs/architecture/ike/ipsec-11-interop-eap.md -- EAP-TLS in-memory handshake harness
// RFC: rfc/short/rfc5216.md -- EAP-TLS MSK derivation
// RFC: rfc/short/rfc9190.md -- the TLS 1.3 replacement for that derivation
//
// VALIDATES: the EAP-TLS MSK is the TLS export the RFC names, under the exporter
// label the RFC spells, on both the TLS 1.2 and the TLS 1.3 branch.
// PREVENTS: the failure this file was written for. The tagged assertions in
// eap_tls_handshake_test.go check that the two MSKs are non-zero, 64 octets and
// EQUAL. Both sides call one producer, exportEAPTLSMSK, so replacing either label
// constant with a made-up string leaves all three true and every test green,
// while every real supplicant derives a different key and authentication fails.
// The TLS 1.2 branch was not reached at all: crypto/tls negotiates TLS 1.3 by
// default, so no test executed the RFC 5216 label it was tagged for.
//
// The tests live beside eap_tls_handshake_test.go rather than inside it: that
// file carries `RFC requirement:` tags and the pretool-writeedit hook refuses
// every edit to a tagged test file, an addition included (ai/rules/testing.md).

package eap

import (
	"crypto/tls"
	"testing"
)

// exportFrom runs the TLS exporter over a connection state. ConnectionState is
// returned by value and ExportKeyingMaterial has a pointer receiver, so the value
// needs a name before it can be exported from.
func exportFrom(cs tls.ConnectionState) func(label string, context []byte, length int) ([]byte, error) {
	return cs.ExportKeyingMaterial
}

// mskLabelHandshake drives a full EAP-TLS exchange with the authenticator's TLS
// version pinned to one value, and returns the completed method and peer so a
// caller can run the TLS exporter itself over the same connections.
//
// The pin is applied to the method's own tls.Config before Begin, which is the
// call that starts the TLS engine goroutine.
func mskLabelHandshake(t *testing.T, version uint16) (*tlsMethod, *PeerSession, [64]byte, [64]byte) {
	t.Helper()

	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("eap-tls-client", &PeerTLSConfig{
		CertPEM:   pki.clientCertPEM,
		KeyPEM:    pki.clientKeyPEM,
		CACertPEM: pki.trustedCAPEM,
	})

	sess, err := NewSession(TypeTLS, pki.serverConfig())
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(func() {
		sess.Close()
		peer.Close()
	})

	method, ok := sess.method.(*tlsMethod)
	if !ok {
		t.Fatalf("authenticator method is %T, want *tlsMethod", sess.method)
	}
	method.tlsConfig.MinVersion = version
	method.tlsConfig.MaxVersion = version

	var peerMSK, serverMSK [64]byte
	var done bool
	req := sess.Begin()
	for range 60 {
		pres := peer.Process(req)
		if pres.Err != nil {
			t.Fatalf("the peer failed a handshake both sides should accept: %v", pres.Err)
		}
		if pres.Done {
			peerMSK = pres.MSK
			done = true
			break
		}
		if pres.Response == nil {
			t.Fatal("the peer stopped answering before the handshake completed")
		}
		next := sess.Process(pres.Response)
		if next == nil {
			t.Fatal("the authenticator stopped answering before the handshake completed")
		}
		if next.Code == CodeSuccess {
			serverMSK = sess.MSK()
		}
		if next.Code == CodeFailure {
			t.Fatal("the authenticator refused a handshake it should accept")
		}
		req = next
	}
	if !done {
		t.Fatal("the handshake did not complete")
	}

	serverState := method.conn.ConnectionState()
	if serverState.Version != version {
		t.Fatalf("authenticator negotiated TLS %#04x, want the pinned %#04x", serverState.Version, version)
	}
	return method, peer, serverMSK, peerMSK
}

// TestRFC5216MSKIsTheExportUnderTheRFCLabel pins the TLS 1.2 derivation to the
// exporter label RFC 5216 spells, computed here from the literal string rather
// than from the constant the producer reads.
//
// RFC 5216 Section 2.3: "Key_Material = TLS-PRF-128(master_secret, "client EAP
// encryption", client.random || server.random)" and "MSK = Key_Material(0,63)".
//
// RFC requirement: RFC5216-2.3-1 positive -- over a TLS 1.2 session both sides'
// MSK is the first 64 octets the TLS exporter yields for the label "client EAP
// encryption" with no context, so the derivation uses that label and no other
// (eap_tls.go exportEAPTLSMSK).
func TestRFC5216MSKIsTheExportUnderTheRFCLabel(t *testing.T) {
	method, peer, serverMSK, peerMSK := mskLabelHandshake(t, tls.VersionTLS12)

	// The literal the RFC spells, written out here so the assertion does not read
	// the same constant the producer reads.
	const rfc5216Label = "client EAP encryption"

	want, err := exportFrom(method.conn.ConnectionState())(rfc5216Label, nil, 64)
	if err != nil {
		t.Fatalf("the TLS 1.2 session refused the RFC 5216 export, so this test cannot judge the label: %v", err)
	}
	if [64]byte(want) != serverMSK {
		t.Fatalf("authenticator MSK = %x,\n                want %x (TLS export under %q)", serverMSK, want, rfc5216Label)
	}

	peerWant, err := exportFrom(peer.tlsConn.ConnectionState())(rfc5216Label, nil, 64)
	if err != nil {
		t.Fatalf("the peer's TLS 1.2 session refused the RFC 5216 export: %v", err)
	}
	if [64]byte(peerWant) != peerMSK {
		t.Fatalf("peer MSK = %x,\n     want %x (TLS export under %q)", peerMSK, peerWant, rfc5216Label)
	}

	// A different label yields different octets on this connection, so the
	// equality above is a statement about the label and not about any export.
	other, err := exportFrom(method.conn.ConnectionState())("some other label", nil, 64)
	if err != nil {
		t.Fatalf("export under a control label: %v", err)
	}
	if [64]byte(other) == serverMSK {
		t.Fatal("the TLS exporter returned the same octets for a different label; this fixture cannot see a wrong label")
	}
}

// TestRFC9190MSKIsTheExportUnderTheRFCLabel pins the TLS 1.3 derivation, which
// RFC 9190 replaces because TLS 1.3 has no master_secret for the RFC 5216 PRF.
//
// RFC 9190 Section 2.3: "For EAP-TLS, the Type field has value 0x0D", then
// "Type = 0x0D" and "Key_Material = TLS-Exporter("EXPORTER_EAP_TLS_Key_Material",
// Type, 128)". The same section: "The MSK and EMSK are derived from the
// Key_Material in the same manner as with EAP-TLS [RFC5216], Section 2.3."
//
// RFC requirement: RFC9190-2.3-1 positive -- over a TLS 1.3 session both sides'
// MSK is the first 64 octets of the 128-octet TLS export for the label
// "EXPORTER_EAP_TLS_Key_Material" with the EAP Type octet as context
// (eap_tls.go exportEAPTLSMSK).
func TestRFC9190MSKIsTheExportUnderTheRFCLabel(t *testing.T) {
	method, peer, serverMSK, peerMSK := mskLabelHandshake(t, tls.VersionTLS13)

	const rfc9190Label = "EXPORTER_EAP_TLS_Key_Material"
	typeCode := []byte{TypeTLS}

	want, err := exportFrom(method.conn.ConnectionState())(rfc9190Label, typeCode, 128)
	if err != nil {
		t.Fatalf("the TLS 1.3 session refused the RFC 9190 export: %v", err)
	}
	if [64]byte(want[:64]) != serverMSK {
		t.Fatalf("authenticator MSK = %x,\n                want %x (first 64 octets of the export under %q)", serverMSK, want[:64], rfc9190Label)
	}

	peerWant, err := exportFrom(peer.tlsConn.ConnectionState())(rfc9190Label, typeCode, 128)
	if err != nil {
		t.Fatalf("the peer's TLS 1.3 session refused the RFC 9190 export: %v", err)
	}
	if [64]byte(peerWant[:64]) != peerMSK {
		t.Fatalf("peer MSK = %x,\n     want %x", peerMSK, peerWant[:64])
	}

	// The Type octet is part of the derivation, not decoration: the same label with
	// a different context yields different octets.
	other, err := exportFrom(method.conn.ConnectionState())(rfc9190Label, []byte{TypeTLS + 1}, 128)
	if err != nil {
		t.Fatalf("export under a control context: %v", err)
	}
	if [64]byte(other[:64]) == serverMSK {
		t.Fatal("the TLS exporter ignored the context; this fixture cannot see a wrong Type octet")
	}
}
