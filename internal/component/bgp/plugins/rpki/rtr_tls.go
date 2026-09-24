// Design: docs/guide/rpki.md -- mutual TLS cache configuration and authentication
// RFC: rfc/short/rfc8210.md -- Sections 3 and 9.2, authenticated RTR transport
package rpki

import (
	"context"
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"time"

	"golang.org/x/net/idna"

	"github.com/ze-software/ze/internal/component/pki"
)

type rtrTLSSettings struct {
	ServerName    string
	Certificate   string
	CACertificate string
}

// rtrServerName is a DNS reference identifier, never the endpoint's IP address.
// RFC 8210 Section 9.2: "The client router MUST set its 'reference identifier'
// to the DNS name of the rpki-rtr cache.".
func rtrServerName(name string) (string, error) {
	name = strings.TrimSuffix(name, ".")
	ascii, err := idna.Lookup.ToASCII(name)
	if err != nil {
		return "", fmt.Errorf("rtr: invalid cache DNS reference name: %w", err)
	}
	if ascii == "" || strings.ContainsAny(ascii, "*:/ \t\r\n") {
		return "", errors.New("rtr: TLS requires a DNS cache reference name, not an IP address or wildcard")
	}
	if _, err := netip.ParseAddr(ascii); err == nil {
		return "", errors.New("rtr: TLS requires a DNS cache reference name, not an IP address")
	}
	if len(ascii) > 253 {
		return "", errors.New("rtr: cache DNS reference name exceeds 253 bytes")
	}
	for label := range strings.SplitSeq(ascii, ".") {
		if label == "" || len(label) > 63 {
			return "", errors.New("rtr: cache DNS reference labels must contain 1 to 63 bytes")
		}
	}
	return strings.ToLower(ascii), nil
}

// buildRTRTLSConfig resolves the named trust anchor and router identity against
// one immutable PKI snapshot. It neither changes that store nor falls back to
// system roots, another certificate, CN-ID authentication, or unverified TLS.
func buildRTRTLSConfig(settings *rtrTLSSettings, store *pki.PKIConfig) (*tls.Config, error) {
	if settings == nil || settings.Certificate == "" || settings.CACertificate == "" {
		return nil, errors.New("rtr: TLS requires a named PKI certificate and ca-certificate")
	}
	name, err := rtrServerName(settings.ServerName)
	if err != nil {
		return nil, err
	}
	if store == nil {
		return nil, errors.New("rtr: TLS PKI store is unavailable")
	}
	ca := store.CACerts[settings.CACertificate]
	if ca == nil || ca.Certificate == nil {
		return nil, fmt.Errorf("rtr: PKI CA %q is not configured", settings.CACertificate)
	}
	entry := store.Certificates[settings.Certificate]
	if entry == nil || entry.Certificate == nil || entry.PrivateKey == nil {
		return nil, fmt.Errorf("rtr: PKI client certificate %q is absent or has no private key", settings.Certificate)
	}
	leaf := entry.Certificate
	// Section 9.2: router certificates "MUST include a subjectAltName
	// extension ... containing one or more iPAddress identities".
	if len(leaf.IPAddresses) == 0 {
		return nil, fmt.Errorf("rtr: client certificate %q has no iPAddress subjectAltName", settings.Certificate)
	}
	now := time.Now()
	if now.Before(leaf.NotBefore) || !now.Before(leaf.NotAfter) {
		return nil, fmt.Errorf("rtr: client certificate %q is not currently valid", settings.Certificate)
	}
	if leaf.KeyUsage != 0 && leaf.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		return nil, fmt.Errorf("rtr: certificate %q does not permit digital signatures", settings.Certificate)
	}
	clientUsage := len(leaf.ExtKeyUsage) == 0 && len(leaf.UnknownExtKeyUsage) == 0
	for _, usage := range leaf.ExtKeyUsage {
		if usage == x509.ExtKeyUsageAny || usage == x509.ExtKeyUsageClientAuth {
			clientUsage = true
			break
		}
	}
	if !clientUsage {
		return nil, fmt.Errorf("rtr: certificate %q does not permit TLS client authentication", settings.Certificate)
	}
	signer, ok := entry.PrivateKey.(crypto.Signer)
	if !ok {
		return nil, fmt.Errorf("rtr: certificate %q has an unsupported signing key", settings.Certificate)
	}
	public, ok := leaf.PublicKey.(interface{ Equal(crypto.PublicKey) bool })
	if !ok || !public.Equal(signer.Public()) {
		return nil, fmt.Errorf("rtr: certificate %q does not match its private key", settings.Certificate)
	}
	chain := make([][]byte, 1, 1+len(entry.RawIntermediates))
	chain[0] = entry.Raw
	chain = append(chain, entry.RawIntermediates...)
	roots := x509.NewCertPool()
	roots.AddCert(ca.Certificate)
	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		ServerName:   name,
		RootCAs:      roots,
		Certificates: []tls.Certificate{{Certificate: chain, PrivateKey: entry.PrivateKey, Leaf: leaf}},
	}, nil
}

// startTLS authenticates both roles before connectAndSync writes its RTR query.
func (s *RTRSession) startTLS(ctx context.Context, conn net.Conn) (net.Conn, error) {
	cfg, err := buildRTRTLSConfig(s.tlsSettings, s.pkiConfig)
	if err != nil {
		return nil, err
	}
	certificate := &cfg.Certificates[0]
	// Section 9.2 assigns source-IP matching to the cache. The router cannot
	// infer the address that cache sees across NAT from conn.LocalAddr().
	requested := false
	cfg.GetClientCertificate = func(request *tls.CertificateRequestInfo) (*tls.Certificate, error) {
		if err := request.SupportsCertificate(certificate); err != nil {
			return nil, fmt.Errorf("rtr: cache cannot authenticate configured client certificate: %w", err)
		}
		requested = true
		return certificate, nil
	}
	secure := tls.Client(conn, cfg)
	if err := secure.HandshakeContext(ctx); err != nil {
		return nil, fmt.Errorf("rtr: TLS authentication: %w", err)
	}
	// Section 9.2 requires client-side authentication. A server that never
	// requested the router's certificate cannot receive an RTR query.
	if !requested {
		return nil, errors.New("rtr: cache did not request TLS client authentication")
	}
	return secure, nil
}
