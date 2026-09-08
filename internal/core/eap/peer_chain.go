// Design: docs/architecture/ike/ipsec-11-interop-eap.md -- the peer's certificate checks
// Detail: peer.go -- startTLSClient, which installs both callbacks below
// RFC: rfc/short/rfc9190.md -- Section 5.4 (revocation), Section 5.7 (resumption)

package eap

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"time"
)

// serverChainCheck holds the checks a ze EAP-TLS peer runs on the
// authenticator's certificate chain.
//
// It spans the two crypto/tls callbacks because neither one alone sees
// everything the checks need. verifyPeerCertificate is where the chain is BUILT:
// EAP carries no server hostname, so the config sets InsecureSkipVerify and
// crypto/tls builds none of its own. verifyConnection is where the NEGOTIATED
// VERSION is known, and RFC 9190 Section 5.4's "When EAP-TLS is used with TLS
// 1.3" turns on it.
//
// crypto/tls calls the two in that order on one goroutine, a few statements
// apart (Conn.verifyServerCertificate, crypto/tls/handshake_client.go), so
// chains is written before it is read and needs no lock.
type serverChainCheck struct {
	roots *x509.CertPool
	crls  crlSet

	// chains is what verifyPeerCertificate built, leaf first and trust anchor
	// last, for verifyConnection to check the revocation status over.
	chains [][]*x509.Certificate
}

// verifyPeerCertificate validates the authenticator's presented certificate
// chain against the configured roots, without any DNS or hostname check
// (EAP-TLS has no server hostname), and keeps the verified chain for
// verifyConnection.
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
	return checkChainRevocation(c.crls, chains, cs.Version, now)
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
