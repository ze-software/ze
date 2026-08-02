// Design: plan/learned/744-ipsec-9-ikev2-eap-nat.md -- EAP-TLS authenticator failure reporting
// RFC: rfc/short/rfc5216.md -- EAP-TLS: a fragment ACK answers the M flag, it is not a resting state
//
// The authenticator's tlsMethod.Process waits for its TLS engine to settle, but a
// REJECTED handshake settles exactly like a finished flight: both can leave
// waitServerData with nothing to return. These tests pin the two consequences of
// failing to tell them apart.
//
// VALIDATES: a peer whose client certificate the authenticator's
// RequireAndVerifyClientCert + ClientCAs pool rejects makes tlsMethod.Process
// return MethodResult.Err naming the certificate failure, and makes the EAP
// session send EAP-Failure, instead of answering bare fragment ACKs forever; and
// feedPeerData refuses octets that would grow the TLS engine's unread backlog
// past eapTLSMaxPeerBuffered, reporting the refusal instead of buffering them.
// PREVENTS: the regression where the transport's recorded handshake error was
// never read, so a refused peer was ACKed until the 30s stale-handshake reaper
// while feedPeerData accumulated everything an unauthenticated party sent.

package eap

import (
	"crypto/x509"
	"encoding/binary"
	"strings"
	"testing"
)

// authDrive is what driving an EAP-TLS exchange from the authenticator side
// revealed: the authenticator's first reported error, the peer's first reported
// error, and how many bare fragment ACKs (TypeData {0}) the authenticator sent.
type authDrive struct {
	methodErr error
	peerErr   error
	ackRounds int
	rounds    int
	done      bool
}

// driveTLSMethod runs the real authenticator method against the real peer,
// message by message, stopping at the first error, at completion, or at the cap.
//
// It drives tlsMethod directly rather than through Session so the test can see
// MethodResult.Err itself: Session.handleMethod collapses every error into one
// EAP-Failure packet, which cannot show whether the cause reached the caller.
func driveTLSMethod(t *testing.T, method *tlsMethod, peer *PeerSession, maxRounds int) authDrive {
	t.Helper()

	var d authDrive
	req := method.Start(1)

	for i := range maxRounds {
		d.rounds = i + 1

		pres := peer.Process(req)
		if pres.Err != nil {
			d.peerErr = pres.Err
			return d
		}
		if pres.Done || pres.Response == nil {
			return d
		}

		mres := method.Process(pres.Response)
		if mres.Err != nil {
			d.methodErr = mres.Err
			return d
		}
		if mres.Done {
			d.done = true
			return d
		}
		if mres.Response == nil {
			return d
		}
		// A bare fragment ACK: one octet of flags, all clear. RFC 5216 Section
		// 2.1.5 permits it only as the answer to a message carrying the M flag.
		if len(mres.Response.TypeData) == 1 && mres.Response.TypeData[0] == 0 {
			d.ackRounds++
		}
		req = mres.Response
	}
	return d
}

// impostorPKI is a PKI whose second CA carries the SAME Subject DN as the
// trusted one but a different key.
//
// That detail is what forces real path validation. Go's TLS client offers a
// client certificate only when its issuer matches one of the acceptable CAs in
// the server's CertificateRequest, so a plainly untrusted chain makes the client
// send an EMPTY certificate and the server report "client didn't provide a
// certificate". Matching the DN makes the client send the certificate, so the
// server verifies it against ClientCAs and rejects it on the chain itself.
type impostorPKI struct {
	caPEM         []byte
	serverCertPEM []byte
	serverKeyPEM  []byte
	clientCertPEM []byte
	clientKeyPEM  []byte
}

func newImpostorPKI(t *testing.T) *impostorPKI {
	t.Helper()
	const sharedCN = "eap-tls-shared-name-ca"

	trustedCA, trustedKey, trustedPEM := newCA(t, sharedCN, 300)
	impostorCA, impostorKey, _ := newCA(t, sharedCN, 301)

	p := &impostorPKI{caPEM: trustedPEM}
	p.serverCertPEM, p.serverKeyPEM = newLeaf(t, trustedCA, trustedKey, "impostor-pki-server", 302,
		[]x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth})
	p.clientCertPEM, p.clientKeyPEM = newLeaf(t, impostorCA, impostorKey, "impostor-pki-client", 303,
		[]x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})
	return p
}

func (p *impostorPKI) serverConfig() MethodConfig {
	return MethodConfig{
		ServerCertPEM: p.serverCertPEM,
		ServerKeyPEM:  p.serverKeyPEM,
		CACertPEM:     p.caPEM,
	}
}

func (p *impostorPKI) peerConfig() *PeerTLSConfig {
	return &PeerTLSConfig{
		CertPEM:   p.clientCertPEM,
		KeyPEM:    p.clientKeyPEM,
		CACertPEM: p.caPEM,
	}
}

// TestEAPTLSAuthenticatorReportsRejectedClientCertificate drives a full EAP-TLS
// exchange in which the authenticator's TLS engine rejects the peer's client
// certificate, and asserts the authenticator REPORTS that rejection.
//
// VALIDATES: tlsMethod.Process returns a non-nil MethodResult.Err carrying the
// TLS engine's own reason once runTLSServer has recorded it, and the exchange
// stops within a couple of rounds instead of emitting bare TypeData {0} ACKs.
// PREVENTS: the endless-ACK regression -- Process read only waitServerData's
// return value, which is empty both while the engine builds a flight and after
// it has failed, so a refused peer was ACKed until the stale-handshake reaper.
func TestEAPTLSAuthenticatorReportsRejectedClientCertificate(t *testing.T) {
	trusted := newEAPTLSPKI(t)
	impostor := newImpostorPKI(t)

	cases := []struct {
		name      string
		serverCfg MethodConfig
		peerCfg   *PeerTLSConfig
		// wantReason is the TLS engine's own wording, which must survive into the
		// reported error: the cause, not the "no MSK" consequence.
		wantReason string
	}{
		{
			// The client's issuer is not among the CertificateRequest's acceptable
			// CAs, so it sends an empty certificate and RequireAndVerifyClientCert
			// refuses the connection.
			name:      "peer offers no acceptable certificate",
			serverCfg: trusted.serverConfig(),
			peerCfg: &PeerTLSConfig{
				CertPEM: trusted.untrustedClientCertPEM,
				KeyPEM:  trusted.untrustedClientKeyPEM,
				// The peer trusts the authenticator, so it does not fail first: the
				// rejection under test is the authenticator's.
				CACertPEM: trusted.trustedCAPEM,
			},
			wantReason: "certificate",
		},
		{
			name:       "peer certificate fails path validation",
			serverCfg:  impostor.serverConfig(),
			peerCfg:    impostor.peerConfig(),
			wantReason: "unknown authority",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			method, err := newTLSMethod(tc.serverCfg)
			if err != nil {
				t.Fatalf("newTLSMethod: %v", err)
			}
			peer := NewPeerSessionTLS("rejected-client", tc.peerCfg)
			t.Cleanup(func() {
				method.Close()
				peer.Close()
			})

			// Forty rounds is far more than a handshake needs, so exhausting them
			// is itself the endless-ACK symptom.
			const maxRounds = 40
			d := driveTLSMethod(t, method, peer, maxRounds)

			if d.methodErr == nil {
				t.Fatalf("authenticator reported no error for a rejected client certificate "+
					"(rounds=%d, bare ACKs=%d, peerErr=%v, done=%v): the exchange must fail, not ACK forever",
					d.rounds, d.ackRounds, d.peerErr, d.done)
			}
			if d.done {
				t.Fatal("authenticator completed EAP-TLS with a rejected client certificate")
			}

			msg := d.methodErr.Error()
			if !strings.Contains(msg, tc.wantReason) {
				t.Errorf("error %q does not carry the TLS engine's own reason (want it to mention %q)", msg, tc.wantReason)
			}
			// The consequence must not stand in for the cause.
			if strings.Contains(msg, "no MSK") {
				t.Errorf("error %q reports the missing MSK rather than the certificate failure that caused it", msg)
			}

			// A handshake this short cannot legitimately need many bare ACKs; an
			// unreported failure produces one per round until the cap.
			if d.ackRounds > 3 {
				t.Errorf("authenticator sent %d bare fragment ACKs before failing (rounds=%d): "+
					"the rejection was not reported promptly", d.ackRounds, d.rounds)
			}
			if d.rounds >= maxRounds {
				t.Errorf("exchange ran the full %d rounds: it never terminated on the rejection", maxRounds)
			}
		})
	}
}

// TestEAPTLSSessionSendsFailureForRejectedClientCertificate asserts the reported
// error reaches the wire as EAP-Failure, which is what actually ends the peer's
// exchange.
//
// VALIDATES: with a client certificate the authenticator cannot verify, the EAP
// Session emits an EAP-Failure packet (RFC 3748 Section 4.2) rather than
// continuing the method.
// PREVENTS: a fix that reports the error inside tlsMethod without
// Session.handleMethod turning it into the peer-visible failure.
func TestEAPTLSSessionSendsFailureForRejectedClientCertificate(t *testing.T) {
	impostor := newImpostorPKI(t)
	peer := NewPeerSessionTLS("impostor-pki-client", impostor.peerConfig())

	res := runEAPTLSHandshake(t, impostor.serverConfig(), peer)

	if res.serverEAPSuccess {
		t.Fatal("authenticator sent EAP-Success for a client certificate it could not verify")
	}
	if !res.serverEAPFailure {
		t.Fatalf("authenticator never sent EAP-Failure (rounds=%d, peerErr=%v): "+
			"a rejected peer must be told, not ACKed until the reaper fires", res.rounds, res.peerErr)
	}
}

// bufferedPeerLen reads the transport's unread backlog under its own mutex.
func bufferedPeerLen(tr *eapTLSTransport) int {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return len(tr.peerBuf)
}

// TestEAPTLSProcessRefusesUnboundedPeerBuffer drives the backlog ceiling through
// tlsMethod.Process, the entry point an unauthenticated peer actually reaches
// (ai/rules/fail-closed-guards.md).
//
// The transport is marked finished with NO error, which is exactly the state
// runTLSServer leaves behind once its goroutine has returned: nothing drains
// peerBuf again, and waitServerData settles at once. The handshake-error report
// is therefore not involved, which isolates the ceiling.
//
// VALIDATES: Process accepts messages up to eapTLSMaxPeerBuffered of unread
// backlog and returns MethodResult.Err for the message that would exceed it,
// leaving the buffer at the ceiling.
// PREVENTS: the unbounded growth measured at 98,304 bytes, where feedPeerData
// appended every message an unauthenticated peer sent after the TLS engine had
// stopped reading.
func TestEAPTLSProcessRefusesUnboundedPeerBuffer(t *testing.T) {
	pki := newEAPTLSPKI(t)
	method, err := newTLSMethod(pki.serverConfig())
	if err != nil {
		t.Fatalf("newTLSMethod: %v", err)
	}

	tr := newEAPTLSTransport()
	tr.mu.Lock()
	tr.finished = true // the TLS engine goroutine has returned; nothing drains
	tr.mu.Unlock()
	method.transport = tr
	method.started.Store(true)
	method.state = tlsStateHandshake
	t.Cleanup(method.Close)

	// One full-size EAP-TLS message: L flag, declared length, then the payload.
	fullMessage := func() []byte {
		td := make([]byte, 5+eapTLSMaxReassembly)
		td[0] = eapTLSFlagL
		binary.BigEndian.PutUint32(td[1:5], uint32(eapTLSMaxReassembly))
		return td
	}

	// eapTLSMaxPeerBuffered is two messages' worth, so exactly two are accepted.
	accepted := eapTLSMaxPeerBuffered / eapTLSMaxReassembly
	for i := range accepted {
		res := method.Process(&Packet{Code: CodeResponse, Type: TypeTLS, TypeData: fullMessage()})
		if res.Err != nil {
			t.Fatalf("message %d of %d refused below the ceiling: %v", i+1, accepted, res.Err)
		}
	}
	if got := bufferedPeerLen(tr); got != eapTLSMaxPeerBuffered {
		t.Fatalf("backlog is %d bytes after %d full messages, want %d", got, accepted, eapTLSMaxPeerBuffered)
	}

	// The next message crosses the ceiling: refused, reported, and not buffered.
	res := method.Process(&Packet{Code: CodeResponse, Type: TypeTLS, TypeData: fullMessage()})
	if res.Err == nil {
		t.Fatalf("Process accepted a message past the %d byte backlog ceiling (backlog now %d bytes): "+
			"an unauthenticated peer can grow it without limit", eapTLSMaxPeerBuffered, bufferedPeerLen(tr))
	}
	if got := bufferedPeerLen(tr); got > eapTLSMaxPeerBuffered {
		t.Errorf("refused message was buffered anyway: backlog %d bytes, ceiling %d", got, eapTLSMaxPeerBuffered)
	}
	if msg := res.Err.Error(); !strings.Contains(msg, "unread") {
		t.Errorf("error %q does not say the TLS engine had not read the backlog", msg)
	}
}

// TestEAPTLSFeedPeerDataCeilingDoesNotFireOnDrainedTraffic asserts the ceiling
// cannot fire on a live handshake, where the engine reads each flight in the
// round that delivers it.
//
// VALIDATES: once the engine drains the backlog, feedPeerData accepts full-size
// messages again, so the ceiling bounds only an engine that stopped reading.
// PREVENTS: choosing a ceiling that rejects a conformant exchange -- a
// fail-closed guard that fires on valid traffic is an outage, not a guard.
func TestEAPTLSFeedPeerDataCeilingDoesNotFireOnDrainedTraffic(t *testing.T) {
	tr := newEAPTLSTransport()
	chunk := make([]byte, eapTLSMaxReassembly)

	for round := range 6 {
		if err := tr.feedPeerData(chunk); err != nil {
			t.Fatalf("round %d: feedPeerData refused a full message on a drained transport: %v", round, err)
		}
		// Drain it the way the TLS engine does.
		read := 0
		buf := make([]byte, 4096)
		for read < len(chunk) {
			n, err := tr.Read(buf)
			if err != nil {
				t.Fatalf("round %d: Read: %v", round, err)
			}
			read += n
		}
		if got := bufferedPeerLen(tr); got != 0 {
			t.Fatalf("round %d: %d bytes left after draining", round, got)
		}
	}
}
