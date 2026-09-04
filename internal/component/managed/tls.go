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
// Three ways to authenticate the hub, in this order:
//
//  1. The pki ca entry the client names (plugin/hub/client/ca). That entry
//     holds the hub's certificate authority root, which the operator exported
//     from the hub. The chain is validated against it and against nothing else,
//     so a hub that reissues its leaf stays reachable with no client change.
//     A name that resolves to nothing is an error, never a fallback.
//  2. TLSInsecure. The operator gave up server authentication, and the log line
//     says so on every connection.
//  3. The system CA pool, with the server name taken from the hub address.
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
// Reads Server, TLSInsecure and CA. A caller that has only those may leave the
// rest zero.
func ClientTLSConfig(cfg *ClientConfig) (*tls.Config, error) { return clientTLSConfig(cfg) }
