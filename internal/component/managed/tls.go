// Design: docs/architecture/fleet-config.md -- how a managed client authenticates its hub
// Related: client.go -- runConnection dials with the tls.Config this file builds
// Related: ../plugin/ipc/tls.go -- TLSConfigWithFingerprint, the pinning the plugin rail uses

package managed

import (
	"crypto/tls"
	"strings"

	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/core/env"
)

// envCertificateFingerprint carries the hub certificate fingerprint before any
// config exists. A managed client's first boot has only environment variables
// (ze.managed.server, ze.managed.name, ze.managed.token), so the trust anchor
// has to be reachable the same way. It also overrides the configured leaf,
// which is the precedence ai/rules/config.md states for a setting that lives in
// both places.
const envCertificateFingerprint = "ze.managed.tls.certificate-fingerprint"

var _ = env.MustRegister(env.EnvEntry{
	Key:  envCertificateFingerprint,
	Type: "string",
	Description: "SHA-256 fingerprint (hex) of the hub certificate the managed client pins. " +
		"Overrides plugin/hub/client/certificate-fingerprint.",
})

// certificateFingerprint returns the fingerprint this client pins, lowercased.
// pluginipc.TLSConfigWithFingerprint compares the hex TEXT, and crypto/hex
// writes lowercase. Without this, an uppercase digest an operator pasted would
// match nothing.
func certificateFingerprint(cfg *ClientConfig) string {
	if v := strings.TrimSpace(env.Get(envCertificateFingerprint)); v != "" {
		return strings.ToLower(v)
	}
	return strings.ToLower(strings.TrimSpace(cfg.CertificateFingerprint))
}

// clientTLSConfig builds the TLS config for the hub connection. The client
// sends its token immediately after the handshake. What this returns therefore
// decides whether that token can reach an impostor.
//
// Three ways to authenticate the hub, in this order:
//
//  1. A pinned certificate fingerprint. The handshake fails on any other
//     certificate, and no CA is needed. A pin beats TLSInsecure, because an
//     operator who supplied a fingerprint asked for a check.
//  2. TLSInsecure. The operator gave up server authentication.
//  3. The system CA pool, with the server name taken from the hub address.
//     This is the default, and it fails closed. An unverifiable certificate
//     ends the handshake before the token is written.
func clientTLSConfig(cfg *ClientConfig) *tls.Config {
	if fp := certificateFingerprint(cfg); fp != "" {
		return pluginipc.TLSConfigWithFingerprint(fp)
	}
	if cfg.TLSInsecure {
		logger().Warn("managed TLS: certificate verification disabled (insecure)")
		return &tls.Config{
			ServerName:         serverNameFromAddr(cfg.Server),
			InsecureSkipVerify: true, //nolint:gosec // opt-in via explicit config flag
			MinVersion:         tls.VersionTLS13,
		}
	}
	return &tls.Config{
		ServerName: serverNameFromAddr(cfg.Server),
		MinVersion: tls.VersionTLS13,
	}
}

// ClientTLSConfig answers the same rules for a caller outside this package.
//
// It exists for FIRST BOOT. A managed client that has no config yet fetches one
// before RunManagedClient ever runs (fetchInitialConfig, cmd/ze), and that path
// built its own tls.Config: the fingerprint could not reach it, so the very
// first exchange, the one that carries the token to a hub the client has never
// spoken to, was the least authenticated of them all. One rule for every
// connection is the point; a second spelling of it is how they diverge.
func ClientTLSConfig(server string, insecure bool, fingerprint string) *tls.Config {
	return clientTLSConfig(&ClientConfig{
		Server:                 server,
		TLSInsecure:            insecure,
		CertificateFingerprint: fingerprint,
	})
}
