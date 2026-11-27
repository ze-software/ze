// Design: docs/architecture/ike/ipsec-11-interop-eap.md -- the peer's certificate checks
// Detail: peer.go -- startTLSClient, which installs both callbacks below
// RFC: rfc/short/rfc9190.md -- Section 5.4 (revocation), Section 5.7 (resumption)

package eap

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"slices"
	"sync/atomic"
	"time"
)

// serverChainCheck holds the checks a ze EAP-TLS peer runs on the
// authenticator's certificate chain.
//
// It spans the two crypto/tls callbacks because neither one alone sees
// everything the checks need. verifyPeerCertificate is where the chain is BUILT:
// the config sets InsecureSkipVerify so RFC 2818 Section 3.1 identity checks
// (including CN fallback when SAN is absent and rfc822Name SANs) run in
// verifyConnection without emitting cleartext SNI, and crypto/tls builds no
// chain of its own. verifyConnection is where the NEGOTIATED VERSION is known
// (RFC 9190 Section 5.4) and where both full and resumed sessions are checked
// for revocation, OCSP stapling, and expected server identity.
//
// crypto/tls calls the two in that order on one goroutine, a few statements
// apart (Conn.verifyServerCertificate, crypto/tls/handshake_client.go), so
// chains is written before it is read and needs no lock.
type serverChainCheck struct {
	roots *x509.CertPool
	crls  crlSet

	// requireStatus is the operator's certificate-status-request answer. It is
	// what makes this peer one that "uses Certificate Status Requests to check
	// the revocation status of the EAP-TLS server's certificate chain" in RFC
	// 9190 Section 5.4's sense, and so the antecedent of that sentence's MUST
	// (checkStapledChainStatus, ocsp.go).
	requireStatus bool

	// expectedServer is the compiled expected server identity checked under
	// RFC 5216 Section 5.3 and RFC 2818 Section 3.1.
	expectedServer ServerIdentity

	// chains is what verifyPeerCertificate built, leaf first and trust anchor
	// last, for verifyConnection to check the revocation status over.
	chains [][]*x509.Certificate

	// verified publishes the chain this session finally accepted, for the
	// post-authentication revocation check to re-read once the tunnel is up
	// (RFC 9190 Section 5.4, engine/postauth_revocation.go).
	//
	// It is a separate field from chains, and atomic, because the two are read
	// from different goroutines: chains is written and read inside the TLS
	// handshake, and this one is read by the IKE session goroutine after the
	// handshake has finished. It carries the chain a RESUMED session rebuilt as
	// well, which chains does not hold.
	verified atomic.Pointer[[][]*x509.Certificate]

	// serverIDs publishes the verified server certificate identities extracted in
	// certificate wire order (RFC 5216 Section 5.2), written once all checks pass.
	serverIDs atomic.Pointer[[]string]
}

func (c *serverChainCheck) primaryServerID() string {
	ids := c.serverIDs.Load()
	if ids == nil {
		return ""
	}
	if len(*ids) == 0 {
		return ""
	}
	return (*ids)[0]
}

func (c *serverChainCheck) verifiedServerIDs() []string {
	ids := c.serverIDs.Load()
	if ids == nil {
		return nil
	}
	if len(*ids) == 0 {
		return nil
	}
	return slices.Clone(*ids)
}

// ServerChains answers the authenticator certificate chains this EAP-TLS peer
// verified and accepted, leaf first and trust anchor last, and nil when the
// session ran no EAP-TLS handshake or the handshake refused the chain.
//
// RFC 9190 Section 5.4: "EAP-TLS peer implementations MUST also support checking
// for certificate revocation after authentication completes and network
// connectivity is available." That later check needs the chain this session
// accepted, and the IKE engine is what knows when connectivity exists
// (startServerCertRecheck, internal/component/ike/engine/postauth_revocation.go).
//
// Safe for concurrent use.
func (ps *PeerSession) ServerChains() [][]*x509.Certificate {
	if ps.serverCheck == nil {
		return nil
	}
	chains := ps.serverCheck.verified.Load()
	if chains == nil {
		return nil
	}
	return *chains
}

// verifyPeerCertificate validates the authenticator's presented certificate
// chain against the configured roots and keeps the verified chain for
// verifyConnection, which enforces revocation, OCSP stapling, and RFC 2818
// Section 3.1 server-identity verification.
//
// RFC 5216 Section 5.3: the peer validates the authenticator's certificate.
func (c *serverChainCheck) verifyPeerCertificate(rawCerts [][]byte, _ [][]*x509.Certificate) error {
	if len(rawCerts) == 0 {
		return fmt.Errorf("eap-tls: authenticator presented no certificate")
	}
	certs := make([]*x509.Certificate, 0, len(rawCerts))
	for _, raw := range rawCerts {
		cert, err := x509.ParseCertificate(raw)
		if err != nil {
			return fmt.Errorf("eap-tls: parse authenticator certificate: %w", err)
		}
		certs = append(certs, cert)
	}
	opts := x509.VerifyOptions{Roots: c.roots, Intermediates: x509.NewCertPool()}
	for _, cert := range certs[1:] {
		opts.Intermediates.AddCert(cert)
	}
	chains, err := certs[0].Verify(opts)
	if err != nil {
		return fmt.Errorf("eap-tls: authenticator certificate chain verification failed: %w", err)
	}
	c.chains = chains
	return nil
}

// verifyConnection checks the revocation status of the chain
// verifyPeerCertificate built.
//
// RFC 9190 Section 5.4: "When EAP-TLS is used with TLS 1.3, the revocation
// status of all the certificates in the certificate chains MUST be checked
// (except the trust anchor)."
//
// crypto/tls sends a fatal bad_certificate alert for a non-nil return
// (Conn.verifyServerCertificate, crypto/tls/handshake_client.go), which is the
// abort Section 5.4 asks for.
//
// ON A RESUMED SESSION IT REBUILDS THE CHAIN FIRST, because nothing else on this
// role validates the cached one. Go skips verifyPeerCertificate entirely when it
// resumes -- readServerCertificate (crypto/tls/handshake_client_tls13.go)
// returns from its hs.usingPSK branch after calling VerifyConnection and nothing
// else -- so c.chains is empty and checkChainRevocation would refuse every
// resumed session. That refusal is fail-closed and correct, and it is also the
// thing that makes peer-side resumption impossible, so the chain is rebuilt
// rather than the check dropped.
func (c *serverChainCheck) verifyConnection(cs tls.ConnectionState) error {
	now := time.Now()
	chains := c.chains
	if len(chains) == 0 && cs.DidResume {
		rebuilt, err := c.rebuildResumedChains(cs, now)
		if err != nil {
			return err
		}
		chains = rebuilt
	}
	if err := checkChainRevocation(c.crls, chains, cs.Version, now); err != nil {
		return err
	}

	// RFC 9190 Section 5.4 binds this check to a peer that USES Certificate
	// Status Requests, and the operator's certificate-status-request leaf is what
	// says whether this peer is one (checkStapledChainStatus, ocsp.go).
	//
	// It runs AFTER the revocation walk above, so a peer with both configured
	// reports the revoked certificate rather than the missing staple: a chain a
	// CRL already refuses is refused for the reason the operator can act on.
	if c.requireStatus {
		if err := checkStapledChainStatus(chains, cs.OCSPResponse, now); err != nil {
			return err
		}
	}

	// RFC 5216 Section 5.3 / RFC 2818 Section 3.1 / RFC 9190 Section 5.7:
	// verify the authenticator leaf certificate identity on both full and resumed
	// sessions after chain, CRL, and OCSP checks have succeeded.
	if len(chains) == 0 {
		return fmt.Errorf("eap-tls: no verified authenticator certificate chain available for server-identity verification")
	}
	if len(chains[0]) == 0 {
		return fmt.Errorf("eap-tls: verified authenticator certificate chain holds no leaf certificate")
	}
	ids, err := verifyServerCertificate(chains[0][0], c.expectedServer)
	if err != nil {
		return err
	}
	c.serverIDs.Store(&ids)

	// Published for the post-authentication check, and only once every check
	// above accepted the chain. A chain this session refused is one no later
	// check should re-read (ai/rules/principles.md).
	c.verified.Store(&chains)
	return nil
}

// rebuildResumedChains path-validates the certificates a resumed session
// restored from its ticket, so verifyConnection has the chain the revocation
// check is about.
//
// IT READS cs.PeerCertificates AND NOT cs.VerifiedChains, and the difference is
// load-bearing. Conn.verifyServerCertificate (crypto/tls/handshake_client.go)
// assigns c.peerCertificates unconditionally, but assigns c.verifiedChains only
// under `else if !c.config.InsecureSkipVerify`. This peer sets
// InsecureSkipVerify, because EAP-TLS carries no server hostname (startTLSClient
// below), so c.verifiedChains is empty on a FULL ze handshake too.
// Conn.sessionState (crypto/tls/ticket.go) then caches that empty value and the
// resumption restores it, which is why cs.VerifiedChains answers nothing here
// and reading it would leave every resumed session refused.
//
// IT IS THE ONLY THING REVALIDATING THE CACHED CHAIN ON THIS ROLE. Conn.
// loadSession runs its own anyValidVerifiedChain sweep over the cached chains
// before it offers a ticket, and skips that sweep under InsecureSkipVerify for
// the same reason. So without this rebuild a peer would resume against a
// certificate that has since expired or left its CA, and RFC 9190 Section 5.7's
// requirements on cached data would hold on the authenticator alone.
//
// The current time is used deliberately: a chain that verified when the ticket
// was minted says nothing about now, and expiry is exactly what a week-old
// ticket can outlive.
func (c *serverChainCheck) rebuildResumedChains(cs tls.ConnectionState, now time.Time) ([][]*x509.Certificate, error) {
	if len(cs.PeerCertificates) == 0 {
		return nil, fmt.Errorf("eap-tls: the resumed session carries no authenticator certificate to check")
	}
	opts := x509.VerifyOptions{Roots: c.roots, Intermediates: x509.NewCertPool(), CurrentTime: now}
	for _, cert := range cs.PeerCertificates[1:] {
		opts.Intermediates.AddCert(cert)
	}
	chains, err := cs.PeerCertificates[0].Verify(opts)
	if err != nil {
		return nil, fmt.Errorf("eap-tls: the resumed session's cached authenticator chain no longer verifies: %w", err)
	}
	if len(chains) == 0 {
		// Verify answers an error rather than an empty result, so this is
		// unreachable. It is written because an empty chain set is what the
		// caller would read as "nothing to check" (ai/rules/principles.md).
		return nil, fmt.Errorf("eap-tls: the resumed session's cached authenticator chain built no path to a trust anchor")
	}
	return chains, nil
}
