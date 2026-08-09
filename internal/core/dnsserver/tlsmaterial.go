// Design: docs/architecture/dns/secure-transports.md -- shared certificate loading for
// the DoT/DoH listeners, so as112 and geodns do not each re-implement it.

package dnsserver

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"os"

	"github.com/ze-software/ze/internal/core/selfcert"
)

// LoadTLSMaterial builds the tls.Config the DoT/DoH listeners serve. When the
// operator sets both certFile and keyFile it loads that PEM material; when both
// are empty it falls back to an ephemeral self-signed certificate (the same
// selfcert helper the web UI uses, web/server.go:111), logging a warning because
// a strict client cannot validate it. A half-configured pair (only one of the
// two) is an error, so a mistyped config fails loudly rather than silently
// serving a self-signed cert an operator did not intend.
//
// selfSignedSANs are extra SAN entries (IPs or DNS names) added to the fallback
// certificate -- the listener addresses, so a lenient client can still match the
// name.
func LoadTLSMaterial(certFile, keyFile string, selfSignedSANs []string, log *slog.Logger) (*tls.Config, error) {
	if certFile != "" || keyFile != "" {
		if certFile == "" || keyFile == "" {
			return nil, fmt.Errorf("dnsserver: tls needs both cert-file and key-file (got cert-file=%q key-file=%q)", certFile, keyFile)
		}
		certPEM, err := os.ReadFile(certFile) //nolint:gosec // cert path from parsed operator config
		if err != nil {
			return nil, fmt.Errorf("dnsserver: read cert-file %q: %w", certFile, err)
		}
		keyPEM, err := os.ReadFile(keyFile) //nolint:gosec // key path from parsed operator config
		if err != nil {
			return nil, fmt.Errorf("dnsserver: read key-file %q: %w", keyFile, err)
		}
		cfg, err := selfcert.NewTLSConfig(certPEM, keyPEM)
		if err != nil {
			return nil, fmt.Errorf("dnsserver: tls material: %w", err)
		}
		return cfg, nil
	}

	// No operator material: ephemeral self-signed fallback.
	certPEM, keyPEM, err := selfcert.GenerateWebCertWithNames("", selfSignedSANs, 0)
	if err != nil {
		return nil, fmt.Errorf("dnsserver: generate self-signed certificate: %w", err)
	}
	cfg, err := selfcert.NewTLSConfig(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("dnsserver: self-signed tls material: %w", err)
	}
	if log != nil {
		log.Warn("dnsserver: no tls cert-file/key-file configured; serving an ephemeral self-signed certificate (strict clients cannot validate it)")
	}
	return cfg, nil
}
