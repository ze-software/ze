// Design: docs/architecture/fleet-config.md -- how a managed client authenticates its hub
// Related: client.go -- runConnection dials with the tls.Config this file builds
// Related: ../pki/store.go -- GetCA turns the configured ca name into the trust anchor
// Related: ../plugin/server/managed_serve.go -- the hub half, which serves the leaf this validates

package managed

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/pki"
)

// errCANotConfigured refuses a client whose configured trust anchor does not
// exist in the pki store. It is an error and not a fall-through to the system
// pool: an operator who named a CA asked for that CA, and a client that quietly
// tried another anchor would either fail far from the cause or, worse, succeed
// against one the operator never chose.
var errCANotConfigured = errors.New("managed client: pki ca is not configured")

// clientTLSConfig builds the TLS config for the hub connection. The client
// sends its token immediately after the handshake. What this returns therefore
// decides whether that token can reach an impostor.
//
// Four ways to authenticate the hub, in this order:
//
//  1. The pki ca entry the client names (bus/hub/client/ca). That entry
//     holds the hub's certificate authority root, which the operator exported
//     from the hub. The chain is validated against it and against nothing else,
//     so a hub that reissues its leaf stays reachable with no client change.
//     A name that resolves to nothing is an error, never a fallback.
//  2. HubRoot, the root ze init stored (--hub-ca) or the first start kept
//     (--hub-trust-first-use). It anchors the hub for a client whose config
//     names no ca, the first start included, the same way a named entry does.
//  3. TLSInsecure. The operator gave up server authentication, and the log line
//     says so on every connection.
//  4. The system CA pool, with the server name taken from the hub address.
//     This is the default, and it fails closed. An unverifiable certificate
//     ends the handshake before the token is written.
func clientTLSConfig(cfg *ClientConfig) (*tls.Config, error) {
	if cfg.CA != "" {
		entry := pki.GetCA(cfg.CA)
		if entry == nil {
			return nil, fmt.Errorf("%w: %s", errCANotConfigured, cfg.CA)
		}
		pool := x509.NewCertPool()
		pool.AddCert(entry.Certificate)
		return &tls.Config{
			ServerName: serverNameFromAddr(cfg.Server),
			RootCAs:    pool,
			MinVersion: tls.VersionTLS13,
		}, nil
	}

	if cfg.HubRoot != nil {
		pool := x509.NewCertPool()
		pool.AddCert(cfg.HubRoot)
		return &tls.Config{
			ServerName: serverNameFromAddr(cfg.Server),
			RootCAs:    pool,
			MinVersion: tls.VersionTLS13,
		}, nil
	}

	if cfg.TLSInsecure {
		logger().Warn("managed TLS: certificate verification disabled (insecure)")
		return &tls.Config{
			ServerName:         serverNameFromAddr(cfg.Server),
			InsecureSkipVerify: true, //nolint:gosec // opt-in via explicit config flag
			MinVersion:         tls.VersionTLS13,
		}, nil
	}

	return &tls.Config{
		ServerName: serverNameFromAddr(cfg.Server),
		MinVersion: tls.VersionTLS13,
	}, nil
}

// ClientTLSConfig answers the same rules for a caller outside this package.
//
// It exists for FIRST BOOT. A managed client that has no config yet fetches one
// before RunManagedClient ever runs (fetchInitialConfig, cmd/ze), and that path
// built its own tls.Config: the trust anchor could not reach it, so the very
// first exchange, the one that carries the token to a hub the client has never
// spoken to, was the least authenticated of them all. One rule for every
// connection is the point; a second spelling of it is how they diverge.
//
// It takes the whole ClientConfig rather than the three values it reads, so a
// fourth trust input reaches both callers the moment clientTLSConfig reads it.
// An argument list is a second spelling of the struct, and the narrower one is
// the one a later field is forgotten in.
//
// Reads Server, TLSInsecure, CA and HubRoot. A caller that has only those may
// leave the rest zero.
func ClientTLSConfig(cfg *ClientConfig) (*tls.Config, error) { return clientTLSConfig(cfg) }

// ErrHubPresentedNoAuthority refuses a first-use handshake with a hub that
// presented no certificate authority to keep: its chain is a leaf alone, or
// ends in a certificate that may not sign, or that did not issue the leaf.
var ErrHubPresentedNoAuthority = errors.New("the hub presented no certificate authority that issued its certificate")

// FirstUseTLSConfig builds the TLS config for the ONE connection of a client
// told to trust on first use (ze init --hub-trust-first-use): it has no root
// for its hub yet and takes the one the hub presents. The handshake completes
// only when the last certificate the hub presents is a certificate authority
// and the hub's own certificate verifies against it for the hub's name. The
// second return value answers that authority after a completed handshake, and
// nil before one.
//
// Nothing here says the hub is the right one: whoever answers on the hub
// address is believed once. The caller MUST store the authority it returns
// and use it as ClientConfig.HubRoot for every later connection, so that the
// belief is never extended to a second answerer.
func FirstUseTLSConfig(server string) (*tls.Config, func() *x509.Certificate) {
	var kept *x509.Certificate
	serverName := serverNameFromAddr(server)
	conf := &tls.Config{
		ServerName: serverName,
		MinVersion: tls.VersionTLS13,
		// The chain is verified below against the authority it carries.
		// The library's own verification has no root to verify against yet.
		InsecureSkipVerify: true, //nolint:gosec // replaced by VerifyConnection, opt-in through ze init --hub-trust-first-use
		VerifyConnection: func(state tls.ConnectionState) error {
			authority, err := presentedAuthority(state.PeerCertificates, serverName)
			if err != nil {
				return err
			}
			kept = authority
			return nil
		},
	}
	return conf, func() *x509.Certificate { return kept }
}

// presentedAuthority returns the certificate authority at the end of chain
// once the first certificate of chain verifies against it for serverName.
func presentedAuthority(chain []*x509.Certificate, serverName string) (*x509.Certificate, error) {
	if len(chain) < 2 {
		return nil, ErrHubPresentedNoAuthority
	}
	authority := chain[len(chain)-1]
	if !authority.IsCA {
		return nil, ErrHubPresentedNoAuthority
	}
	roots := x509.NewCertPool()
	roots.AddCert(authority)
	intermediates := x509.NewCertPool()
	for _, cert := range chain[1 : len(chain)-1] {
		intermediates.AddCert(cert)
	}
	if _, err := chain[0].Verify(x509.VerifyOptions{
		Roots:         roots,
		Intermediates: intermediates,
		DNSName:       serverName,
	}); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrHubPresentedNoAuthority, err)
	}
	return authority, nil
}
