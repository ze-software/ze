package cli

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"

	// The plugin transport env keys (ze.plugin.hub.token, ze.plugin.ca.pem) are
	// registered in the SDK, and env.Get aborts the process on an unregistered
	// key. A binary that links this package without the SDK therefore cannot
	// reach engine mode at all; that placement is recorded in
	// plan/journal/env-var-double-registration.md.
	_ "github.com/ze-software/ze/pkg/plugin/sdk"
)

// VALIDATES: spec-local-ca AC-4 on the plugin framework's engine mode, the
// caller the daemon uses for every system plugin it launches out of process
// (internal/plugins/*/register.go all end in cli.RunPlugin).
// PREVENTS: the state this replaced, where connFromEnv set InsecureSkipVerify
// and the plugin wrote its own token to whatever answered on the address.

// TestEngineModeRefusesWithoutATrustAnchor drives RunPlugin's engine mode with
// the hub token set and no certificate authority root.
//
// The assertion is that RunEngine is NEVER reached: no connection was made, so
// no plugin registered and no token left this process. An assertion on the exit
// code alone would also pass when the dial merely failed to connect, which is
// why the second half supplies a real engine and requires the same call to
// succeed.
func TestEngineModeRefusesWithoutATrustAnchor(t *testing.T) {
	rootPEM, leaf := engineAnchorAuthority(t)
	addr := engineAnchorServer(t, leaf)

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split %s: %v", addr, err)
	}

	cases := []struct {
		name    string
		caPEM   string
		reached bool
	}{
		{name: "no-anchor", caPEM: "", reached: false},
		{name: "not-a-certificate", caPEM: "-----BEGIN CERTIFICATE-----\nnot base64\n-----END CERTIFICATE-----\n", reached: false},
		{name: "the-engine-root", caPEM: string(rootPEM), reached: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ZE_PLUGIN_HUB_HOST", host)
			t.Setenv("ZE_PLUGIN_HUB_PORT", port)
			t.Setenv("ZE_PLUGIN_HUB_TOKEN", "engine-anchor-token-of-at-least-32-chars")
			t.Setenv("ZE_PLUGIN_CA_PEM", tc.caPEM)
			t.Setenv("ZE_PLUGIN_NAME", "engine-anchor-probe")
			env.ResetCache()
			t.Cleanup(env.ResetCache)

			reached := false
			code := RunPlugin(PluginConfig{
				Name: "engine-anchor-probe",
				RunEngine: func(conn net.Conn) int {
					reached = true
					conn.Close() //nolint:errcheck,gosec // the assertion is that this ran at all
					return 0
				},
			}, nil)

			if reached != tc.reached {
				t.Fatalf("RunEngine reached = %t, want %t", reached, tc.reached)
			}
			wantCode := 1
			if tc.reached {
				wantCode = 0
			}
			if code != wantCode {
				t.Fatalf("RunPlugin exit = %d, want %d", code, wantCode)
			}
		})
	}
}

// engineAnchorAuthority mints a root and one leaf for 127.0.0.1, and returns the
// root in PEM plus the leaf as a serving pair.
//
// crypto/x509 is used directly rather than internal/component/pki: pki reaches
// this package through plugin/server, so importing it here is an import cycle.
func engineAnchorAuthority(t *testing.T) ([]byte, tls.Certificate) {
	t.Helper()

	rootKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("root key: %v", err)
	}
	rootTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "engine anchor test root"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTemplate, rootTemplate, &rootKey.PublicKey, rootKey)
	if err != nil {
		t.Fatalf("root certificate: %v", err)
	}
	root, err := x509.ParseCertificate(rootDER)
	if err != nil {
		t.Fatalf("parse root: %v", err)
	}

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("leaf key: %v", err)
	}
	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "ze-plugin-hub"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, root, &leafKey.PublicKey, rootKey)
	if err != nil {
		t.Fatalf("leaf certificate: %v", err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rootDER}),
		tls.Certificate{Certificate: [][]byte{leafDER}, PrivateKey: leafKey}
}

// engineAnchorServer answers one plugin connection the way the hub acceptor
// does: TLS with the issued leaf, then an `ok` to the auth line. It returns the
// address it bound.
func engineAnchorServer(t *testing.T, leaf tls.Certificate) string {
	t.Helper()

	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{leaf},
		MinVersion:   tls.VersionTLS13,
	})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { listener.Close() }) //nolint:errcheck // test teardown

	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			go func() {
				defer conn.Close() //nolint:errcheck // test server teardown
				if _, readErr := bufio.NewReader(conn).ReadString('\n'); readErr != nil {
					return
				}
				conn.Write([]byte("#0 ok\n")) //nolint:errcheck,gosec // the client's read decides the result
				// The client keeps the connection; hold it until the test ends.
				<-time.After(5 * time.Second)
			}()
		}
	}()

	return listener.Addr().String()
}
