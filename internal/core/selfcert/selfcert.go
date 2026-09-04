// Design: docs/architecture/web-interface.md -- self-signed TLS certificate helpers

// Package selfcert generates and persists self-signed HTTPS certificates for
// local Ze services.
package selfcert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// CertStore abstracts certificate persistence.
//
// Implementations store PEM-encoded certificate and key data in zefs, the local
// filesystem, or another caller-owned backend.
type CertStore interface {
	// ReadCert returns the stored certificate PEM data.
	// Returns an error if no certificate has been stored.
	ReadCert() ([]byte, error)

	// ReadKey returns the stored private key PEM data.
	// Returns an error if no key has been stored.
	ReadKey() ([]byte, error)

	// WriteCert stores the certificate PEM data.
	// Permissions are handled by the store implementation.
	WriteCert(data []byte) error

	// WriteKey stores the private key PEM data.
	// The store MUST restrict read access to the owning process.
	WriteKey(data []byte) error

	// Exists returns true if both certificate and key are present in the store.
	Exists() bool
}

// certValidityDuration is the lifetime of generated self-signed certificates.
const certValidityDuration = 365 * 24 * time.Hour

var certLogger = slogutil.Logger("web.server")

// generateWebCert creates a self-signed ECDSA P-256 certificate suitable for
// local HTTPS access. The certificate includes SANs for localhost, 127.0.0.1,
// and ::1.
func generateWebCert() (certPEM, keyPEM []byte, err error) {
	return GenerateWebCertWithNames("", nil, 0)
}

// GenerateWebCertWithAddr creates a self-signed ECDSA P-256 certificate with
// SANs for localhost, 127.0.0.1, ::1, and the host portion of listenAddr (if it
// parses as a valid IP not already covered by the defaults).
func GenerateWebCertWithAddr(listenAddr string) (certPEM, keyPEM []byte, err error) {
	if listenAddr == "" {
		return generateWebCert()
	}
	return GenerateWebCertWithNames(listenAddr, nil, 0)
}

// GenerateWebCertWithNames creates a self-signed ECDSA P-256 certificate with
// SANs for localhost, 127.0.0.1, ::1, the host portion of listenAddr, and any
// extra DNS names provided. Extra names that parse as IPs are added as IP SANs
// instead of DNS SANs. A zero validity uses the default one-year lifetime.
func GenerateWebCertWithNames(listenAddr string, extraNames []string, validity time.Duration) (certPEM, keyPEM []byte, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate ECDSA key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, fmt.Errorf("generate serial number: %w", err)
	}

	if validity <= 0 {
		validity = certValidityDuration
	}

	now := time.Now()
	template := x509.Certificate{
		SerialNumber:          serialNumber,
		NotBefore:             now,
		NotAfter:              now.Add(validity),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// A host that parses as an IP address becomes an IP SAN, and every other
	// host a DNS SAN.
	for _, host := range WebCertHosts(listenAddr, extraNames) {
		if ip := net.ParseIP(host); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
			continue
		}
		template.DNSNames = append(template.DNSNames, host)
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("create certificate: %w", err)
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal private key: %w", err)
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return certPEM, keyPEM, nil
}

// WebCertHosts returns the subject alternative names a Ze web certificate
// carries: localhost and the two loopback addresses, the host part of
// listenAddr, and every extra name the caller names. It is the one declaration
// of that set. GenerateWebCertWithNames builds its own template from it, and
// the appliance build host passes it to the certificate authority, so a
// self-signed leaf and an issued one identify the same listener.
//
// Listening on the unspecified address expands to the interface addresses of
// the machine this call runs on, so a client that reaches the listener by any
// of them still verifies the name.
func WebCertHosts(listenAddr string, extraNames []string) []string {
	hosts := make([]string, 0, len(extraNames)+4)
	hosts = append(hosts, "localhost", net.IPv4(127, 0, 0, 1).String(), net.IPv6loopback.String())

	if listenAddr != "" {
		host, _, splitErr := net.SplitHostPort(listenAddr)
		if splitErr != nil {
			host = listenAddr
		}
		if ip := net.ParseIP(host); ip != nil && !ip.IsLoopback() {
			if ip.IsUnspecified() {
				hosts = append(hosts, interfaceHosts()...)
			} else {
				hosts = append(hosts, ip.String())
			}
		}
	}

	return append(hosts, extraNames...)
}

// interfaceHosts returns every non-loopback unicast IP of this machine's
// interfaces. It answers for the unspecified listen address, where the
// certificate has to be valid for whichever local IP the client connects to.
func interfaceHosts() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	var hosts []string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, addrErr := iface.Addrs()
		if addrErr != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			if ipNet.IP.IsLoopback() || ipNet.IP.IsLinkLocalUnicast() {
				continue
			}
			hosts = append(hosts, ipNet.IP.String())
		}
	}
	return hosts
}

// NewTLSConfig creates a tls.Config from PEM-encoded certificate and key data.
// The config enforces TLS 1.2 as the minimum version.
func NewTLSConfig(certPEM, keyPEM []byte) (*tls.Config, error) {
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("parse TLS key pair: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// LoadOrGenerateCert retrieves existing TLS material from the store, or
// generates a new self-signed certificate and persists it. The listenAddr is
// used to add an extra SAN for the configured listen address.
func LoadOrGenerateCert(store CertStore, listenAddr string) (certPEM, keyPEM []byte, err error) {
	if store.Exists() {
		certPEM, err = store.ReadCert()
		if err != nil {
			return nil, nil, fmt.Errorf("load certificate from store: %w", err)
		}
		keyPEM, err = store.ReadKey()
		if err != nil {
			return nil, nil, fmt.Errorf("load key from store: %w", err)
		}
		certLogger.Info("loaded TLS certificate from store")
		return certPEM, keyPEM, nil
	}

	certPEM, keyPEM, err = GenerateWebCertWithAddr(listenAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("generate self-signed certificate: %w", err)
	}

	if writeErr := store.WriteCert(certPEM); writeErr != nil {
		return nil, nil, fmt.Errorf("store certificate: %w", writeErr)
	}
	// Note: if WriteKey fails after WriteCert succeeds, the store may contain an
	// orphaned certificate. The next call to LoadOrGenerateCert will attempt to
	// load both and fail on the missing key, triggering regeneration.
	if writeErr := store.WriteKey(keyPEM); writeErr != nil {
		certLogger.Warn("WriteKey failed after WriteCert succeeded; store may have orphaned certificate", "error", writeErr)
		return nil, nil, fmt.Errorf("store private key: %w", writeErr)
	}

	certLogger.Info("generated and stored self-signed TLS certificate", "listen-addr", listenAddr)
	return certPEM, keyPEM, nil
}
