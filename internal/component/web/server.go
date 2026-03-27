// Design: docs/architecture/web-interface.md -- Web server infrastructure

// Package web provides the HTTPS web interface for ze.
//
// The server uses self-signed TLS certificates (ECDSA P-256) that are
// generated on first start and stored via a CertStore interface. Callers
// can also supply pre-existing PEM-encoded certificate and key material
// to skip generation entirely.
//
// Route handlers are registered externally via HandleFunc; the web package
// owns transport (TLS, listen, serve) but not application logic.
package web

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	stdlog "log"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// serverLogger is the structured logger for the web server subsystem.
// The auth logger is declared separately in auth.go as "web.auth".
var serverLogger = slogutil.Logger("web.server")

// certValidityDuration is the lifetime of generated self-signed certificates.
const certValidityDuration = 365 * 24 * time.Hour

// CertStore abstracts certificate persistence.
// The web server reads and writes PEM-encoded certificate and key data
// through this interface. Implementations may use zefs blob storage,
// the local filesystem, or any other backend.
type CertStore interface {
	// ReadCert returns the stored certificate PEM data.
	// Returns an error if no certificate has been stored.
	ReadCert() ([]byte, error)

	// ReadKey returns the stored private key PEM data.
	// Returns an error if no key has been stored.
	ReadKey() ([]byte, error)

	// WriteCert stores the certificate PEM data.
	// Permissions are handled by the store implementation (e.g., zefs
	// manages access control internally rather than using filesystem modes).
	WriteCert(data []byte) error

	// WriteKey stores the private key PEM data.
	// The store MUST restrict read access to the owning process.
	WriteKey(data []byte) error

	// Exists returns true if both certificate and key are present in the store.
	Exists() bool
}

// WebConfig holds the configuration for creating a WebServer.
type WebConfig struct {
	// ListenAddr is the address to bind (e.g., "127.0.0.1:8443").
	ListenAddr string

	// CertPEM is optional PEM-encoded certificate data.
	// When set together with KeyPEM, certificate generation is skipped.
	CertPEM []byte

	// KeyPEM is optional PEM-encoded private key data.
	// When set together with CertPEM, certificate generation is skipped.
	KeyPEM []byte

	// Logger is the structured logger for the web server.
	// If nil, the package-level web logger is used.
	Logger *slog.Logger
}

// WebServer is the HTTPS web server.
// Routes are registered via HandleFunc before calling ListenAndServe.
// Callers MUST call Shutdown to release resources when the server is no longer needed.
type WebServer struct {
	mux       *http.ServeMux
	tlsConfig *tls.Config
	addr      string
	mu        sync.RWMutex  // protects addr after ListenAndServe updates it
	ready     chan struct{} // closed when the listener is bound
	logger    *slog.Logger
	server    *http.Server
}

// NewWebServer creates a new WebServer from the given configuration.
// It requires TLS material (CertPEM and KeyPEM) to be present in cfg.
// Use LoadOrGenerateCert to obtain PEM data from a CertStore before
// calling NewWebServer.
func NewWebServer(cfg WebConfig) (*WebServer, error) {
	if cfg.ListenAddr == "" {
		return nil, fmt.Errorf("web server: listen address is required")
	}

	log := cfg.Logger
	if log == nil {
		log = serverLogger
	}

	if len(cfg.CertPEM) == 0 || len(cfg.KeyPEM) == 0 {
		return nil, fmt.Errorf("web server: certificate and key PEM data are required")
	}

	tlsCfg, err := NewTLSConfig(cfg.CertPEM, cfg.KeyPEM)
	if err != nil {
		return nil, fmt.Errorf("web server: %w", err)
	}

	mux := http.NewServeMux()

	return &WebServer{
		mux:       mux,
		tlsConfig: tlsCfg,
		addr:      cfg.ListenAddr,
		ready:     make(chan struct{}),
		logger:    log,
		server: &http.Server{
			Addr:      cfg.ListenAddr,
			Handler:   mux,
			TLSConfig: tlsCfg,
			// Timeouts prevent slow clients from holding connections indefinitely.
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       120 * time.Second,
			// Suppress TLS handshake errors from browsers rejecting self-signed certs.
			ErrorLog: stdlog.New(io.Discard, "", 0),
		},
	}, nil
}

// HandleFunc registers a handler function for the given pattern on the server's mux.
// Patterns follow net/http ServeMux conventions (e.g., "GET /api/status").
func (s *WebServer) HandleFunc(pattern string, handler http.HandlerFunc) {
	s.mux.HandleFunc(pattern, handler)
}

// Handle registers a handler for the given pattern on the server's mux.
func (s *WebServer) Handle(pattern string, handler http.Handler) {
	s.mux.Handle(pattern, handler)
}

// ListenAndServe starts the HTTPS server on the configured address.
// It blocks until the server is shut down or encounters a fatal error.
// The context is used for the initial listener bind; use Shutdown for
// graceful termination of the running server.
func (s *WebServer) ListenAndServe(ctx context.Context) error {
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", s.addr)
	if err != nil {
		return fmt.Errorf("web server bind: %w", err)
	}

	// Update address to reflect the actual bound address (e.g., when port is 0).
	s.mu.Lock()
	s.addr = ln.Addr().String()
	s.mu.Unlock()

	close(s.ready)

	s.logger.Info("web server listening", "address", s.addr)

	tlsLn := tls.NewListener(ln, s.tlsConfig)

	// Serve is used instead of ServeTLS because the listener is already TLS-wrapped.
	return s.server.Serve(tlsLn)
}

// Address returns the configured listen address.
// After ListenAndServe is called, this reflects the actual bound address
// (which may differ if the configured port was 0).
func (s *WebServer) Address() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.addr
}

// WaitReady blocks until the server has bound its listener and is ready
// to accept connections, or until ctx is canceled.
func (s *WebServer) WaitReady(ctx context.Context) error {
	select {
	case <-s.ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Shutdown gracefully shuts down the server without interrupting active connections.
// It waits for active requests to complete or until the context deadline is reached.
func (s *WebServer) Shutdown(ctx context.Context) error {
	s.logger.Info("web server shutting down")
	return s.server.Shutdown(ctx)
}

// GenerateWebCert creates a self-signed ECDSA P-256 certificate
// suitable for local HTTPS access. The certificate includes SANs for
// localhost, 127.0.0.1, and ::1.
//
// The returned PEM data can be passed directly to NewTLSConfig or stored
// via a CertStore for reuse across restarts.
func GenerateWebCert() (certPEM, keyPEM []byte, err error) {
	return GenerateWebCertWithAddr("")
}

// GenerateWebCertWithAddr creates a self-signed ECDSA P-256 certificate
// with SANs for localhost, 127.0.0.1, ::1, and the host portion of listenAddr
// (if it parses as a valid IP not already covered by the defaults).
func GenerateWebCertWithAddr(listenAddr string) (certPEM, keyPEM []byte, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate ECDSA key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, fmt.Errorf("generate serial number: %w", err)
	}

	now := time.Now()
	template := x509.Certificate{
		SerialNumber:          serialNumber,
		NotBefore:             now,
		NotAfter:              now.Add(certValidityDuration),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses: []net.IP{
			net.IPv4(127, 0, 0, 1),
			net.IPv6loopback,
		},
	}

	// Add SANs for the listen address. When listening on the unspecified
	// address (0.0.0.0 or ::), add all non-loopback interface IPs so the
	// certificate is valid regardless of which IP the client connects to.
	if listenAddr != "" {
		host, _, splitErr := net.SplitHostPort(listenAddr)
		if splitErr != nil {
			host = listenAddr
		}
		if ip := net.ParseIP(host); ip != nil && !ip.IsLoopback() {
			if ip.IsUnspecified() {
				addInterfaceIPs(&template)
			} else {
				template.IPAddresses = append(template.IPAddresses, ip)
			}
		}
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

// addInterfaceIPs appends all non-loopback unicast IPs from network interfaces
// to the certificate template's IPAddresses list. Used when listening on 0.0.0.0
// so the cert is valid for any local IP the client connects to.
func addInterfaceIPs(tmpl *x509.Certificate) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return
	}
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
			tmpl.IPAddresses = append(tmpl.IPAddresses, ipNet.IP)
		}
	}
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
// generates a new self-signed certificate and persists it. The listenAddr
// is used to add an extra SAN for the configured listen address.
//
// Returns PEM-encoded certificate and key data ready for NewWebServer.
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
		serverLogger.Info("loaded TLS certificate from store")
		return certPEM, keyPEM, nil
	}

	certPEM, keyPEM, err = GenerateWebCertWithAddr(listenAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("generate self-signed certificate: %w", err)
	}

	if writeErr := store.WriteCert(certPEM); writeErr != nil {
		return nil, nil, fmt.Errorf("store certificate: %w", writeErr)
	}
	// Note: if WriteKey fails after WriteCert succeeds, the store may contain
	// an orphaned certificate. The next call to LoadOrGenerateCert will attempt
	// to load both and fail on the missing key, triggering regeneration.
	if writeErr := store.WriteKey(keyPEM); writeErr != nil {
		serverLogger.Warn("WriteKey failed after WriteCert succeeded; store may have orphaned certificate", "error", writeErr)
		return nil, nil, fmt.Errorf("store private key: %w", writeErr)
	}

	serverLogger.Info("generated and stored self-signed TLS certificate", "listen-addr", listenAddr)
	return certPEM, keyPEM, nil
}
