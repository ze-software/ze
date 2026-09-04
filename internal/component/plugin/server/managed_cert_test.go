// VALIDATES: the managed listener serves the pki certificate the config names, fails
// closed when that name resolves to nothing, and otherwise serves a leaf the daemon's
// certificate authority issued (spec-managed-server-hardening AC-1, spec-local-ca AC-4/AC-6).
// PREVENTS: returning to the ephemeral self-signed certificate no client could validate,
// and a configured certificate name quietly becoming a self-issued one.

package server

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"testing"
	"time"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/selfcert"
	"github.com/ze-software/ze/internal/test/sim"
)

// testCA is a certificate authority for these tests. It is a local copy rather
// than pki.Root because internal/component/pki imports this package
// (pki/show.go -> plugin/server), so no test file here can import it.
//
// issued counts calls, which is what proves a configured certificate name is
// answered before issuance is ever considered.
type testCA struct {
	cert   *x509.Certificate
	key    *ecdsa.PrivateKey
	issued int
}

func newTestCA(t *testing.T) *testCA {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("test authority key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test Local CA"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("test authority certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse test authority certificate: %v", err)
	}
	return &testCA{cert: cert, key: key}
}

func (c *testCA) IssueLeaf(commonName string, hosts []string) (tls.Certificate, error) {
	c.issued++
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(int64(c.issued) + 1),
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	for _, host := range hosts {
		template.IPAddresses = append(template.IPAddresses, net.ParseIP(host))
	}
	der, err := x509.CreateCertificate(rand.Reader, template, c.cert, &key.PublicKey, c.key)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, nil
}

func (c *testCA) CertificatePEM() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.cert.Raw})
}

var _ plugin.Authority = (*testCA)(nil)

// hubCertMaterial returns a certificate/key PEM pair, standing in for what
// pki.ServerTLSMaterial returns for a named store certificate.
func hubCertMaterial(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()
	certPEM, keyPEM, err := selfcert.GenerateWebCertWithNames("127.0.0.1:0", nil, time.Hour)
	if err != nil {
		t.Fatalf("generate certificate: %v", err)
	}
	return certPEM, keyPEM
}

// anchoredConfig verifies the server certificate against anchorPEM and nothing
// else, which is how every managed client authenticates its hub.
func anchoredConfig(t *testing.T, anchorPEM []byte) *tls.Config {
	t.Helper()
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(anchorPEM) {
		t.Fatal("the anchor PEM is not usable as a trust anchor")
	}
	return &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS13}
}

// dialAnchored completes a handshake against srv with anchorPEM as the only
// trust anchor, and returns the leaf the listener presented.
func dialAnchored(t *testing.T, srv *ManagedServer, anchorPEM []byte) *x509.Certificate {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	raw, err := (&tls.Dialer{Config: anchoredConfig(t, anchorPEM)}).DialContext(ctx, "tcp", srv.Addrs()[0].String())
	if err != nil {
		t.Fatalf("dial the managed listener: %v", err)
	}
	defer raw.Close() //nolint:errcheck // test cleanup

	conn, ok := raw.(*tls.Conn)
	if !ok {
		t.Fatalf("tls.Dialer returned a %T, want a *tls.Conn", raw)
	}
	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		t.Fatal("the managed listener presented no certificate")
	}
	return state.PeerCertificates[0]
}

// TestManagedServerServesConfiguredCertificate: the listener presents the named
// certificate, and a client anchored on that certificate completes the
// handshake.
//
// MUTATION: ignore cfg.Certificate in managedCertificate and issue instead --
// the served leaf no longer chains to the resolved material and the dial fails.
func TestManagedServerServesConfiguredCertificate(t *testing.T) {
	certPEM, keyPEM := hubCertMaterial(t)

	srv, err := NewManagedServer(ManagedServerConfig{
		Addrs:         []string{"127.0.0.1:0"},
		ClientSecrets: map[string]string{testClientName: testClientSecret},
		ReadConfig:    func(string) ([]byte, error) { return []byte(testClientConfig), nil },
		Certificate:   "fleet-hub",
		TLSMaterialResolver: func(name string) ([]byte, []byte, error) {
			if name != "fleet-hub" {
				return nil, nil, errors.New("unexpected certificate name " + name)
			}
			return certPEM, keyPEM, nil
		},
	})
	if err != nil {
		t.Fatalf("NewManagedServer: %v", err)
	}
	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(srv.Stop)

	dialAnchored(t, srv, certPEM)
}

// TestNamedCertificateOutranksIssuance: AC-6 and R-4 -- a configured name is
// resolved first and issuance is never reached, whether the name resolves or
// not. A fail-closed reference must not become a working self-issued listener.
//
// MUTATION: move the cfg.Certificate branch below the issuance branch in
// managedCertificate and both cases fail on the issuance count.
func TestNamedCertificateOutranksIssuance(t *testing.T) {
	certPEM, keyPEM := hubCertMaterial(t)

	t.Run("named-certificate-is-served", func(t *testing.T) {
		ca := newTestCA(t)
		srv, err := NewManagedServer(ManagedServerConfig{
			Addrs:         []string{"127.0.0.1:0"},
			ClientSecrets: map[string]string{testClientName: testClientSecret},
			ReadConfig:    func(string) ([]byte, error) { return []byte(testClientConfig), nil },
			Certificate:   "fleet-hub",
			TLSMaterialResolver: func(string) ([]byte, []byte, error) {
				return certPEM, keyPEM, nil
			},
			Authority: ca,
		})
		if err != nil {
			t.Fatalf("NewManagedServer: %v", err)
		}
		if startErr := srv.Start(context.Background()); startErr != nil {
			t.Fatalf("Start: %v", startErr)
		}
		t.Cleanup(srv.Stop)

		if ca.issued != 0 {
			t.Fatalf("the authority issued %d certificates while a name was configured", ca.issued)
		}
		dialAnchored(t, srv, certPEM)
	})

	t.Run("a-named-certificate-is-never-reissued", func(t *testing.T) {
		// The operator owns that material, so Ze answers it unchanged for the
		// life of the listener and renewing it stays a pki store operation.
		// Only an ISSUED leaf is Ze's to replace.
		clk := sim.NewFakeClock(time.Now())
		ca := newTestCA(t)
		srv, err := NewManagedServer(ManagedServerConfig{
			Addrs:         []string{"127.0.0.1:0"},
			ClientSecrets: map[string]string{testClientName: testClientSecret},
			ReadConfig:    func(string) ([]byte, error) { return []byte(testClientConfig), nil },
			Certificate:   "fleet-hub",
			TLSMaterialResolver: func(string) ([]byte, []byte, error) {
				return certPEM, keyPEM, nil
			},
			Authority: ca,
			Clock:     clk,
		})
		if err != nil {
			t.Fatalf("NewManagedServer: %v", err)
		}

		before, err := srv.getCertificate(nil)
		if err != nil {
			t.Fatalf("the named certificate did not answer: %v", err)
		}
		clk.Add(400 * 24 * time.Hour)
		after, err := srv.getCertificate(nil)
		if err != nil {
			t.Fatalf("the named certificate stopped answering after the clock moved: %v", err)
		}
		if !bytes.Equal(before.Certificate[0], after.Certificate[0]) {
			t.Fatal("Ze replaced the certificate the operator named")
		}
		if ca.issued != 0 {
			t.Fatalf("the authority issued %d certificates while a name was configured", ca.issued)
		}
	})

	t.Run("an-issued-leaf-is-reissued-as-it-ages", func(t *testing.T) {
		clk := sim.NewFakeClock(time.Now())
		ca := newTestCA(t)
		srv, err := NewManagedServer(ManagedServerConfig{
			Addrs:         []string{"127.0.0.1:0"},
			ClientSecrets: map[string]string{testClientName: testClientSecret},
			ReadConfig:    func(string) ([]byte, error) { return []byte(testClientConfig), nil },
			Authority:     ca,
			Clock:         clk,
		})
		if err != nil {
			t.Fatalf("NewManagedServer: %v", err)
		}

		before, err := srv.getCertificate(nil)
		if err != nil {
			t.Fatalf("the issued leaf did not answer: %v", err)
		}
		clk.Add(2 * time.Hour)
		after, err := srv.getCertificate(nil)
		if err != nil {
			t.Fatalf("the issued leaf did not renew: %v", err)
		}
		if bytes.Equal(before.Certificate[0], after.Certificate[0]) {
			t.Fatal("the managed listener served the same leaf past its life rather than reissuing")
		}
		if ca.issued != 2 {
			t.Fatalf("the authority issued %d certificates, want 2: one at start and one renewal", ca.issued)
		}
	})

	t.Run("broken-name-errors-rather-than-issuing", func(t *testing.T) {
		ca := newTestCA(t)
		srv, err := NewManagedServer(ManagedServerConfig{
			Addrs:         []string{"127.0.0.1:0"},
			ClientSecrets: map[string]string{testClientName: testClientSecret},
			ReadConfig:    func(string) ([]byte, error) { return []byte(testClientConfig), nil },
			Certificate:   "missing",
			TLSMaterialResolver: func(string) ([]byte, []byte, error) {
				return nil, nil, errors.New("certificate missing not found")
			},
			Authority: ca,
		})
		if err == nil {
			srv.Stop()
			t.Fatal("a certificate name that does not resolve must be an error, not an issued leaf")
		}
		if ca.issued != 0 {
			t.Fatalf("the authority issued %d certificates for a name that did not resolve", ca.issued)
		}
	})
}

// TestManagedServerIssuesFromTheAuthority: with no certificate name the
// listener serves a leaf the daemon root issued, so a client that holds the
// exported root validates the chain.
//
// MUTATION: self-sign in managedCertificate instead of calling the authority
// and this fails: the presented leaf does not chain to the root.
func TestManagedServerIssuesFromTheAuthority(t *testing.T) {
	ca := newTestCA(t)

	srv, err := NewManagedServer(ManagedServerConfig{
		Addrs:         []string{"127.0.0.1:0"},
		ClientSecrets: map[string]string{testClientName: testClientSecret},
		ReadConfig:    func(string) ([]byte, error) { return []byte(testClientConfig), nil },
		Authority:     ca,
	})
	if err != nil {
		t.Fatalf("NewManagedServer: %v", err)
	}
	if startErr := srv.Start(context.Background()); startErr != nil {
		t.Fatalf("Start: %v", startErr)
	}
	t.Cleanup(srv.Stop)

	leaf := dialAnchored(t, srv, ca.CertificatePEM())
	if leaf.IsCA {
		t.Fatal("the listener presented a CA certificate, not a leaf issued from one")
	}
	if err := leaf.CheckSignatureFrom(ca.cert); err != nil {
		t.Fatalf("the presented leaf was not signed by the authority root: %v", err)
	}
}

// TestManagedServerFailsClosedOnCertificate: a configured name that cannot be
// resolved refuses to build the server, and so does a hub with neither a name
// nor an authority. Neither falls back to a certificate nothing issued: such a
// listener looks healthy until a client refuses the handshake.
//
// MUTATION: self-sign on a resolver error, or when Authority is nil, and each
// case fails.
func TestManagedServerFailsClosedOnCertificate(t *testing.T) {
	base := ManagedServerConfig{
		Addrs:         []string{"127.0.0.1:0"},
		ClientSecrets: map[string]string{testClientName: testClientSecret},
		ReadConfig:    func(string) ([]byte, error) { return []byte(testClientConfig), nil },
		Certificate:   "missing",
	}

	t.Run("resolver-error", func(t *testing.T) {
		cfg := base
		cfg.TLSMaterialResolver = func(string) ([]byte, []byte, error) {
			return nil, nil, errors.New("certificate missing not found")
		}
		srv, err := NewManagedServer(cfg)
		if err == nil {
			srv.Stop()
			t.Fatal("NewManagedServer accepted an unresolvable certificate name")
		}
	})

	t.Run("no-resolver", func(t *testing.T) {
		cfg := base
		srv, err := NewManagedServer(cfg)
		if err == nil {
			srv.Stop()
			t.Fatal("NewManagedServer accepted a certificate name with no resolver")
		}
	})

	t.Run("no-name-and-no-authority", func(t *testing.T) {
		cfg := base
		cfg.Certificate = ""
		srv, err := NewManagedServer(cfg)
		if err == nil {
			srv.Stop()
			t.Fatal("NewManagedServer built a listener with no certificate authority and no name")
		}
	})
}

// TestManagedLeafHostsSkipUnverifiableAddresses: an unspecified listen address
// names no host a peer can check, so it contributes no SAN and the loopback
// stands for it. A SAN of 0.0.0.0 would be a name no client ever dials.
func TestManagedLeafHostsSkipUnverifiableAddresses(t *testing.T) {
	got := managedLeafHosts([]string{"0.0.0.0:1790", "[::]:1790", "10.0.0.1:1790", "127.0.0.1:1790"})
	want := []string{"127.0.0.1", "10.0.0.1"}

	if len(got) != len(want) {
		t.Fatalf("hosts = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("hosts = %v, want %v", got, want)
		}
	}
}
