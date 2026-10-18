// Design: docs/architecture/ike/ipsec-11-interop-eap.md -- EAP-TLS configured intermediate certificates
// Detail: eap_tls.go -- the authenticator's tls.Config; revocation.go -- the chain check that follows
// RFC: rfc/short/rfc5216.md -- Section 5.3

package eap

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"time"
)

// clientChainVerifier path-validates the client chain on the authenticator
// when the operator configured intermediate certificates.
//
// crypto/tls cannot do it: with ClientAuth RequireAndVerifyClientCert it builds
// the intermediate pool from the client's certificate message alone
// (Conn.processCertsFromClient, crypto/tls/handshake_server.go), and tls.Config
// carries no field for more. So the authenticator takes the chain over itself,
// with ClientAuth RequireAnyClientCert, which still refuses a client that sends
// no certificate, and runs the verification here.
//
// It is immutable after construction and safe for concurrent use.
type clientChainVerifier struct {
	roots      *x509.CertPool
	configured []*x509.Certificate
}

// parseIntermediateCAs parses concatenated PEM CERTIFICATE blocks, each of
// which MUST be a CA certificate. An empty input answers an empty list, and
// newTLSMethod then leaves the chain to crypto/tls.
//
// A certificate that cannot sign another has no place on a path, and accepting
// one would read as a working configuration while completing nothing. The
// commit-time check refuses the same config (ValidateIntermediateRefs,
// ipsec/validate.go); this is the second check of the pair.
func parseIntermediateCAs(intermediatePEM []byte) ([]*x509.Certificate, error) {
	var certs []*x509.Certificate
	rest := intermediatePEM
	for len(rest) > 0 {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			return nil, fmt.Errorf("eap-tls: intermediate certificate list holds a %s PEM block, not a CERTIFICATE", block.Type)
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("eap-tls: parse intermediate certificate: %w", err)
		}
		if !cert.BasicConstraintsValid {
			return nil, fmt.Errorf("eap-tls: intermediate certificate %q is not a CA certificate (no basic constraints)", cert.Subject.String())
		}
		if !cert.IsCA {
			return nil, fmt.Errorf("eap-tls: intermediate certificate %q is not a CA certificate (basic constraints cA is false)", cert.Subject.String())
		}
		certs = append(certs, cert)
	}
	if len(certs) == 0 && len(intermediatePEM) > 0 {
		return nil, errors.New("eap-tls: the intermediate certificate list holds no PEM CERTIFICATE block")
	}
	return certs, nil
}

// verify path-validates the client chain and answers every path it built, for
// the revocation check that follows.
//
// RFC 5216 Section 5.3: "it SHOULD support validating the peer certificate
// using RFC 3280 [RFC3280] compliant path validation, including the ability to
// retrieve intermediate certificates that may be necessary to validate the peer
// certificate." Ze retrieves nothing over the network. Its intermediates are
// the ones the same section names for a server that does not fetch: "it will
// need to be pre-configured with the necessary intermediate certificates", so
// the path is built from the configured ones and the ones the client sent,
// anchored at the roots alone.
//
// It reads peerCerts on a resumed session too, where crypto/tls restores the
// client's certificates from the ticket, so a ticket never outlives the chain
// it was minted for: the current time is checked again.
func (v *clientChainVerifier) verify(peerCerts []*x509.Certificate, now time.Time) ([][]*x509.Certificate, error) {
	if len(peerCerts) == 0 {
		return nil, errors.New("eap-tls: the client sent no certificate to validate")
	}
	opts := x509.VerifyOptions{
		Roots:         v.roots,
		Intermediates: x509.NewCertPool(),
		CurrentTime:   now,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	for _, cert := range v.configured {
		opts.Intermediates.AddCert(cert)
	}
	for _, cert := range peerCerts[1:] {
		opts.Intermediates.AddCert(cert)
	}
	chains, err := peerCerts[0].Verify(opts)
	if err != nil {
		return nil, fmt.Errorf("eap-tls: client certificate %q has no path to the ca-certificate through the intermediates it sent or the configured intermediate-certificate list: %w",
			peerCerts[0].Subject.String(), err)
	}
	return chains, nil
}
