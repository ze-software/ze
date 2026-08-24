// Design: docs/architecture/ike/ipsec-11-interop-eap.md -- EAP-TLS MSK export
// RFC: rfc/short/rfc5216.md -- Section 2.3 derives the MSK from ExportKeyingMaterial
//
// This file drives exportEAPTLSMSK over a real TLS 1.2 connection whose key
// material export crypto/tls refuses, and asserts that what ze logs tells the
// operator what to do next.
//
// VALIDATES: a refused TLS 1.2 export names the peer, the negotiated version,
// RFC 7627 and the operator's three answers, and keeps the crypto/tls sentence.
// A TLS 1.2 session that DOES export still yields a real MSK.
// PREVENTS: a return to the bare `eap-tls: export MSK (RFC 5216 Section 2.3): %w`
// wrap, which named neither the peer nor a remedy, and so sent operators to a
// GODEBUG setting Go 1.27 removed, whose old value now stops the daemon before
// main() runs.

package eap

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"strings"
	"testing"
)

// What the refuseExport argument of tls12ClientState asks for, named so a call
// site says which case it is driving.
const (
	exportRefused = true
	exportAllowed = false
)

// tls12ClientState completes a TLS 1.2 handshake over a loopback socket and
// answers the CLIENT's connection state.
//
// refuseExport asks crypto/tls to refuse every key material export on that state.
// It works by enabling renegotiation, which is the only refusal two Go endpoints
// can produce: Go's client always offers the RFC 7627 extended master secret, so
// the strongSwan 5.9.14 case (TLS 1.2, no RFC 7627) has no Go-side reproduction.
// Both refusals arrive at exportEAPTLSMSK as an error from the same call on the
// same TLS 1.2 branch, which is the code under test.
func tls12ClientState(t *testing.T, refuseExport bool) tls.ConnectionState {
	t.Helper()
	pki := newEAPTLSPKI(t)

	serverCert, err := tls.X509KeyPair(pki.serverCertPEM, pki.serverKeyPEM)
	if err != nil {
		t.Fatalf("server keypair: %v", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pki.trustedCAPEM) {
		t.Fatal("trusted CA did not parse")
	}

	// A TCP loopback socket, not net.Pipe. net.Pipe is unbuffered and synchronous,
	// so both TLS endpoints block in their own flush and the handshake deadlocks
	// (plan/journal/net-pipe-deadlock.md). Port 0 asks the kernel for a free port,
	// so parallel copies of this test do not collide.
	ctx := t.Context()
	var listenCfg net.ListenConfig
	listener, err := listenCfg.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() {
		if err := listener.Close(); err != nil {
			t.Logf("close listener: %v", err)
		}
	})

	// One accept, one handshake, then the goroutine ends. The client below is what
	// makes the accept return.
	accepted := make(chan *tls.Conn, 1)
	serverErr := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverErr <- err
			return
		}
		server := tls.Server(conn, &tls.Config{
			Certificates: []tls.Certificate{serverCert},
			MinVersion:   tls.VersionTLS12,
			MaxVersion:   tls.VersionTLS12,
		})
		accepted <- server
		serverErr <- server.HandshakeContext(ctx)
	}()

	// The same verification shape startTLSClient uses: EAP-TLS has no server
	// hostname, so the default name check is off and the chain is verified against
	// the trust anchor by the production callback.
	clientCfg := &tls.Config{
		InsecureSkipVerify:    true, //nolint:gosec // EAP has no server hostname; verifyServerChain checks the chain
		RootCAs:               roots,
		VerifyPeerCertificate: verifyServerChain(roots),
		MinVersion:            tls.VersionTLS12,
		MaxVersion:            tls.VersionTLS12,
	}
	if refuseExport {
		clientCfg.Renegotiation = tls.RenegotiateOnceAsClient
	}
	dialer := tls.Dialer{Config: clientCfg}
	dialed, err := dialer.DialContext(ctx, "tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("client handshake: %v", err)
	}
	client, ok := dialed.(*tls.Conn)
	if !ok {
		t.Fatalf("tls.Dialer answered %T, not a *tls.Conn", dialed)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Logf("close client: %v", err)
		}
	})
	if err := <-serverErr; err != nil {
		t.Fatalf("server handshake: %v", err)
	}
	server := <-accepted
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Logf("close server: %v", err)
		}
	})

	state := client.ConnectionState()
	if state.Version != tls.VersionTLS12 {
		t.Fatalf("negotiated %s, want TLS 1.2", tls.VersionName(state.Version))
	}
	return state
}

// TestEAPTLSExportRefusalNamesTheCause asserts the refused TLS 1.2 export is
// reported as something an operator can act on.
//
// VALIDATES: AC-2 of spec-fixit-eap-tls-escape-hatch-kills-the-daemon.
// PREVENTS: a raw crypto/tls string reaching the log, which names neither ze,
// nor EAP-TLS, nor the peer, nor anything the operator can change.
func TestEAPTLSExportRefusalNamesTheCause(t *testing.T) {
	state := tls12ClientState(t, exportRefused)

	msk, err := exportEAPTLSMSK(state)
	if err == nil {
		t.Fatal("export succeeded on a connection crypto/tls refuses to export from")
	}
	if msk != [64]byte{} {
		t.Fatal("a refused export answered a non-zero MSK")
	}

	got := err.Error()
	for _, want := range []string{
		"eap-tls",                          // the subsystem, so the operator knows who is speaking
		"RFC 5216 Section 2.3",             // what ze was trying to derive
		"CN=eap-tls-server",                // the peer
		"TLS 1.2",                          // the negotiated version
		"RFC 7627",                         // the missing property
		"extended master secret",           // in words, for an operator who does not know the number
		"TLS 1.3",                          // the first remedy, and the preferred one
		"RFC 9190",                         // the RFC that covers it
		"another EAP method",               // the third remedy; the second is the RFC 7627 line above
		"crypto/tls: ExportKeyingMaterial", // the wrapped cause, kept verbatim
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the refusal does not mention %q:\n%s", want, got)
		}
	}

	// Go 1.27 removed the tlsunsafeekm setting, and its old value now stops the
	// process before main() runs. An error that names it sends the operator to a
	// daemon that will not start.
	if strings.Contains(got, "tlsunsafeekm") {
		t.Errorf("the refusal names a GODEBUG key Go 1.27 removed:\n%s", got)
	}
}

// TestEAPTLSExportSucceedsOnTLS12WithExtendedMasterSecret keeps the refusal above
// from reading as "TLS 1.2 never works".
//
// VALIDATES: the TLS 1.2 branch of exportEAPTLSMSK still derives a real MSK when
// the session carries the RFC 7627 extended master secret, which Go's own client
// always offers.
// PREVENTS: a fix that turns every TLS 1.2 session into the refusal.
func TestEAPTLSExportSucceedsOnTLS12WithExtendedMasterSecret(t *testing.T) {
	state := tls12ClientState(t, exportAllowed)

	msk, err := exportEAPTLSMSK(state)
	if err != nil {
		t.Fatalf("export refused on a TLS 1.2 session that carries RFC 7627: %v", err)
	}
	if msk == [64]byte{} {
		t.Fatal("export answered 64 zero octets, which is not a key")
	}
}

// TestEAPTLSAuthenticatorKeepsItsRefusalReason asserts the AUTHENTICATOR role
// keeps the diagnosis its method produced, so the responder path can log it.
//
// The initiator role already had this: PeerSession.Process returns PeerResult.Err
// and handleEAPResponse logs it (internal/component/ike/engine/fsm.go). The
// authenticator role read MethodResult.Err as a boolean and threw it away, so
// handleResponderEAP (internal/component/ike/engine/responder_eap.go) logged
// "EAP authentication failed" with the peer name and nothing else. Interop
// scenarios responder-eap-mschapv2 and responder-eap-tls13 drive that role.
//
// VALIDATES: AC-2 holds in BOTH roles, not only when ze initiates. The message
// eapTLS12ExportRefused builds is worth nothing if the role that builds it
// discards it.
// PREVENTS: a return to a Session that produces a reason and drops it, which
// leaves an operator with a failure and no next action.
func TestEAPTLSAuthenticatorKeepsItsRefusalReason(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("rogue-client", &PeerTLSConfig{
		CertPEM:   pki.untrustedClientCertPEM,
		KeyPEM:    pki.untrustedClientKeyPEM,
		CACertPEM: pki.trustedCAPEM,
	})

	res := runEAPTLSHandshake(t, pki.serverConfig(), peer)
	if res.serverEAPSuccess {
		t.Fatal("authenticator sent EAP-Success for a client cert signed by an untrusted CA")
	}
	if !res.serverEAPFailure {
		t.Fatal("authenticator sent no EAP-Failure, so there is no failure to explain")
	}

	err := res.sess.Err()
	if err == nil {
		t.Fatal("the authenticator refused the peer and kept no reason, so the operator log carries none")
	}
	if !strings.Contains(err.Error(), "eap-tls") {
		t.Errorf("the kept reason does not name the method:\n%s", err)
	}
}
