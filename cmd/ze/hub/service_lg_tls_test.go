//go:build ze_lg

// Design: docs/architecture/pki/tls-listeners.md -- hub looking-glass TLS material selection tests

package hub

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"log/slog"
	"math/big"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	zepki "github.com/ze-software/ze/internal/component/pki"
)

const lgLeafCommonName = "looking glass leaf"

// loadLGPKIStore installs a store holding a root CA, an intermediate, a
// chain-bearing device certificate named "lg-cert", and a keyless entry.
func loadLGPKIStore(t *testing.T) {
	t.Helper()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "lg root"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}

	interKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	interTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(2),
		Subject:               pkix.Name{CommonName: "lg intermediate"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	interDER, err := x509.CreateCertificate(rand.Reader, interTmpl, caCert, &interKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	interCert, err := x509.ParseCertificate(interDER)
	if err != nil {
		t.Fatal(err)
	}

	devKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	devTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: lgLeafCommonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
	}
	devDER, err := x509.CreateCertificate(rand.Reader, devTmpl, interCert, &devKey.PublicKey, interKey)
	if err != nil {
		t.Fatal(err)
	}
	devCert, err := x509.ParseCertificate(devDER)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &zepki.PKIConfig{
		CACerts: map[string]*zepki.CACertEntry{
			"lg-ca": {Name: "lg-ca", Certificate: caCert, Raw: caDER},
		},
		Certificates: map[string]*zepki.CertificateEntry{
			"lg-cert": {
				Name: "lg-cert", Certificate: devCert, Raw: devDER, PrivateKey: devKey,
				Intermediates: []*x509.Certificate{interCert}, RawIntermediates: [][]byte{interDER},
			},
			"lg-keyless": {
				Name: "lg-keyless", Certificate: devCert, Raw: devDER,
				Intermediates: []*x509.Certificate{interCert}, RawIntermediates: [][]byte{interDER},
			},
		},
	}
	if err := zepki.Load(cfg); err != nil {
		t.Fatalf("pki Load: %v", err)
	}
	t.Cleanup(func() { _ = zepki.Load(nil) })
}

// lgBlobStorage returns blob-backed storage, which is what the self-signed path
// needs and what the named-certificate path must not need.
func lgBlobStorage(t *testing.T) storage.Storage {
	t.Helper()
	dir := t.TempDir()
	store, err := storage.NewBlob(filepath.Join(dir, "database.zefs"), dir)
	if err != nil {
		t.Fatalf("storage.NewBlob: %v", err)
	}
	return store
}

// lgServedChain handshakes with the running looking glass and returns the
// certificates it presented, leaf first. The handshake is the only place the
// served material is observable from outside the server.
func lgServedChain(t *testing.T, svc Service) []*x509.Certificate {
	t.Helper()
	addressed, ok := svc.(interface{ Addresses() []string })
	if !ok {
		t.Fatalf("looking-glass service %T does not report its addresses", svc)
	}
	addrs := addressed.Addresses()
	if len(addrs) == 0 {
		t.Fatal("the looking glass bound no address")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// The test reads the presented chain rather than trusting it, so
	// verification is off here and the assertions below carry the check.
	dialer := &tls.Dialer{Config: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec // the test inspects the chain rather than trusting it
	conn, err := dialer.DialContext(ctx, "tcp", addrs[0])
	if err != nil {
		t.Fatalf("TLS handshake with %s: %v", addrs[0], err)
	}
	defer conn.Close() //nolint:errcheck // test client teardown
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		t.Fatalf("dialed connection is %T, not TLS", conn)
	}
	return tlsConn.ConnectionState().PeerCertificates
}

func TestBuildLGServiceResolvesNamedCertificate(t *testing.T) {
	// VALIDATES: AC-1, AC-3 and AC-4 -- a configured name makes the looking
	// glass serve the PKI store chain, and a name that does not resolve is an
	// error with no service rather than a quiet self-signed listener.
	// PREVENTS: the operator trap this spec exists to close -- a public looking
	// glass presenting a certificate the visitor's browser refuses, while the
	// config names a real one.
	loadLGPKIStore(t)

	t.Run("a named certificate is served with its chain", func(t *testing.T) {
		svc, err := buildLGService(&serviceDeps{
			Dispatch:      lgTestDispatch(),
			LGAddrs:       []string{"127.0.0.1:0"},
			LGTLS:         true,
			LGTLSExplicit: true,
			LGCertificate: "lg-cert",
			Store:         lgBlobStorage(t),
		})
		if err != nil {
			t.Fatalf("buildLGService: %v", err)
		}
		if svc == nil {
			t.Fatal("want a running looking glass, got nil")
		}
		t.Cleanup(func() { _ = svc.Shutdown(context.Background()) })

		chain := lgServedChain(t, svc)
		if len(chain) != 2 {
			t.Fatalf("served chain length = %d, want 2 (leaf + intermediate)", len(chain))
		}
		if chain[0].Subject.CommonName != lgLeafCommonName {
			t.Fatalf("leaf CN = %q, want the PKI store leaf", chain[0].Subject.CommonName)
		}
	})

	for _, name := range []string{"typo-cert", "lg-keyless"} {
		t.Run("an unresolvable name refuses the listener: "+name, func(t *testing.T) {
			svc, err := buildLGService(&serviceDeps{
				Dispatch:      lgTestDispatch(),
				LGAddrs:       []string{"127.0.0.1:0"},
				LGTLS:         true,
				LGTLSExplicit: true,
				LGCertificate: name,
				Store:         lgBlobStorage(t),
			})
			if err == nil {
				t.Fatal("a configured name that does not resolve must be an error")
			}
			if svc != nil {
				t.Fatal("a failed reference must leave no looking-glass service")
			}
			if !strings.Contains(err.Error(), name) {
				t.Fatalf("error %q does not name the reference", err)
			}
		})
	}
}

func TestBuildLGServiceNamedCertificateWithoutBlobStorage(t *testing.T) {
	// VALIDATES: AC-7 and R-3 -- the blob-storage precondition guards the
	// self-signed path only. The PKI store holds the named material already, so
	// a deployment with no blob store serves that chain over TLS.
	// PREVENTS: refusing a valid deployment for a reason that does not apply to
	// it, which is what leaving the guard ahead of both branches would do.
	loadLGPKIStore(t)

	svc, err := buildLGService(&serviceDeps{
		Dispatch:      lgTestDispatch(),
		LGAddrs:       []string{"127.0.0.1:0"},
		LGTLS:         true,
		LGTLSExplicit: true,
		LGCertificate: "lg-cert",
		Store:         storage.NewFilesystem(),
	})
	if err != nil {
		t.Fatalf("a named certificate must not need blob storage: %v", err)
	}
	if svc == nil {
		t.Fatal("want a running looking glass, got nil")
	}
	t.Cleanup(func() { _ = svc.Shutdown(context.Background()) })

	chain := lgServedChain(t, svc)
	if len(chain) != 2 {
		t.Fatalf("served chain length = %d, want 2 (leaf + intermediate)", len(chain))
	}
	if chain[0].Subject.CommonName != lgLeafCommonName {
		t.Fatalf("leaf CN = %q, want the PKI store leaf", chain[0].Subject.CommonName)
	}
}

func TestBuildLGServiceEmptyNameKeepsStorageRules(t *testing.T) {
	// VALIDATES: AC-8 -- with no certificate named, the two established
	// storage rules are unchanged. An operator who wrote `tls true` and has no
	// blob store gets an error; one who only inherited the default gets
	// plaintext and a warning.
	// PREVENTS: the guard move for the named path widening into a change of the
	// path every existing deployment takes.
	loadLGPKIStore(t)

	t.Run("explicit TLS without blob storage is an error", func(t *testing.T) {
		svc, err := buildLGService(&serviceDeps{
			Dispatch:      lgTestDispatch(),
			LGAddrs:       []string{"127.0.0.1:0"},
			LGTLS:         true,
			LGTLSExplicit: true,
			LGCertificate: "",
			Store:         storage.NewFilesystem(),
		})
		if err == nil {
			t.Fatal("explicit TLS with no blob storage must fail, not fall back to plaintext")
		}
		if svc != nil {
			t.Fatal("a refused looking glass must leave no service")
		}
		if !strings.Contains(err.Error(), "blob storage") {
			t.Fatalf("error must name the missing certificate store, got %v", err)
		}
	})

	t.Run("defaulted TLS without blob storage serves plaintext", func(t *testing.T) {
		svc, err := buildLGService(&serviceDeps{
			Dispatch:      lgTestDispatch(),
			LGAddrs:       []string{"127.0.0.1:0"},
			LGTLS:         true,
			LGTLSExplicit: false,
			LGCertificate: "",
			Store:         storage.NewFilesystem(),
		})
		if err != nil {
			t.Fatalf("defaulted TLS with no blob storage must still start: %v", err)
		}
		if svc == nil {
			t.Fatal("want a running looking glass, got nil")
		}
		t.Cleanup(func() { _ = svc.Shutdown(context.Background()) })
	})
}

// lgRegistration returns the looking-glass entry the ze_lg init() recorded, so
// a test drives the real registration hook (register_lg.go) rather than a copy
// of it. The hook is where the rotation handle is installed or withheld.
func lgRegistration(t *testing.T) namedFactory {
	t.Helper()
	for _, nf := range serviceFactories {
		if nf.name == "looking-glass" {
			return nf
		}
	}
	t.Fatal("the looking-glass factory is not registered")
	return namedFactory{}
}

// lgMigratorFor builds one looking glass from deps and wires it into a
// migrator, which is the sequence the daemon runs at startup.
func lgMigratorFor(t *testing.T, deps *serviceDeps) *listenerMigrator {
	t.Helper()
	nf := lgRegistration(t)
	svc, err := nf.factory(deps)
	if err != nil {
		t.Fatalf("buildLGService: %v", err)
	}
	if svc == nil {
		t.Fatal("want a running looking glass, got nil")
	}
	t.Cleanup(func() { _ = svc.Shutdown(context.Background()) })

	lm := &listenerMigrator{}
	registerBuiltService(lm, builtService{Service: svc, wireMigrator: nf.wireMigrator})
	return lm
}

// captureMigratorLog points the migrator at a buffer and returns what it wrote.
// The returned function reads everything logged up to the moment it is called.
func captureMigratorLog(lm *listenerMigrator) func() string {
	var sink strings.Builder
	lm.logger = slog.New(slog.NewTextHandler(&sink, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return sink.String
}

func TestPlaintextLGHoldsNoRotationHandle(t *testing.T) {
	// VALIDATES: the certificate is inert on every path a looking glass serves
	// plaintext. A deployment Ze dropped to plaintext for want of a blob store
	// still reads `tls true` from its config, so lgCertificateName reports the
	// name and the reload resolves it; the handle is what withholds the
	// rotation.
	// PREVENTS: the operator adding `certificate x` to that deployment and
	// having the WHOLE reload rejected with "lg server: TLS is not enabled",
	// which refuses unrelated config over a leaf that listener never reads.
	loadLGPKIStore(t)

	t.Run("a downgraded looking glass takes no rotation", func(t *testing.T) {
		// The config asks for TLS by inheriting the default, names no
		// certificate, and has no blob store: buildLGService serves plaintext.
		lm := lgMigratorFor(t, &serviceDeps{
			Dispatch:      lgTestDispatch(),
			LGAddrs:       []string{"127.0.0.1:0"},
			LGTLS:         true,
			LGTLSExplicit: false,
			Store:         storage.NewFilesystem(),
		})
		if lm.lg == nil {
			t.Fatal("the looking glass must still be wired for listener migration")
		}
		if lm.lgTLS != nil {
			t.Fatal("a plaintext looking glass must hold no certificate-rotation handle")
		}

		// The operator now writes `certificate lg-cert` and reloads. The reload
		// is accepted and rotates nothing, so the log line is the ONLY thing
		// that tells the operator their certificate reached no listener. An
		// accepted change that takes no effect in silence is the failure this
		// assertion exists to prevent (ai/rules/principles.md).
		logged := captureMigratorLog(lm)
		if err := lm.updateLGCertificate("lg-cert"); err != nil {
			t.Fatalf("naming a certificate must not refuse the reload: %v", err)
		}
		line := logged()
		for _, want := range []string{
			"the listener serves plaintext",
			"environment.looking-glass.certificate",
			"lg-cert",
			"restart ze",
		} {
			if !strings.Contains(line, want) {
				t.Fatalf("the accepted reload must say why the certificate is inert: %q missing from %q", want, line)
			}
		}
	})

	t.Run("a TLS looking glass takes the rotation", func(t *testing.T) {
		lm := lgMigratorFor(t, &serviceDeps{
			Dispatch:      lgTestDispatch(),
			LGAddrs:       []string{"127.0.0.1:0"},
			LGTLS:         true,
			LGTLSExplicit: true,
			LGCertificate: "lg-cert",
			Store:         storage.NewFilesystem(),
		})
		if lm.lgTLS == nil {
			t.Fatal("a looking glass serving TLS must hold the certificate-rotation handle")
		}
		if err := lm.updateLGCertificate("lg-cert"); err != nil {
			t.Fatalf("rotating onto a TLS looking glass: %v", err)
		}
	})
}
