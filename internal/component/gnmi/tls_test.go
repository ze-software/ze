// Design: docs/guide/gnmi.md -- TLS: the transport serves TLS or nothing
// Related: tls.go -- loadTLSCredentials, the refusal these tests drive
// Related: server.go -- Serve, which loads the material before it listens

package gnmi

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	gpb "github.com/openconfig/gnmi/proto/gnmi"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	yangloader "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/selfcert"
)

// writeTestKeyPair writes a fresh self-signed certificate and its key into
// dir under the given base name, and returns the two paths.
func writeTestKeyPair(t *testing.T, dir, name string) (certFile, keyFile string) {
	t.Helper()
	certPEM, keyPEM, err := selfcert.GenerateWebCertWithAddr("127.0.0.1:0")
	if err != nil {
		t.Fatalf("generate certificate: %v", err)
	}
	certFile = filepath.Join(dir, name+".pem")
	keyFile = filepath.Join(dir, name+".key")
	if err := os.WriteFile(certFile, certPEM, 0o600); err != nil {
		t.Fatalf("write certificate: %v", err)
	}
	if err := os.WriteFile(keyFile, keyPEM, 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return certFile, keyFile
}

// VALIDATES: a config that names TLS material gNMI cannot load starts no
// listener at all. Serve returns an operator error naming the file and the
// setting that fixes it, and binds nothing.
// PREVENTS: the clear-text fallback. The hub read the files and, on a read
// error, left the PEM empty; the server then served gRPC without TLS, so the
// bearer token and the config crossed the network in clear over a config that
// asked for TLS.
// Method: each case configures Serve on 127.0.0.1:0 with broken material and
// waits for it to return. A Serve that serves instead never returns, and the
// test fails on the timeout.
func TestServeRefusesUnloadableTLSMaterial(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := writeTestKeyPair(t, dir, "server")
	_, otherKeyFile := writeTestKeyPair(t, dir, "other")
	missing := filepath.Join(dir, "missing.pem")

	tests := []struct {
		name        string
		cert        string
		key         string
		wantFile    string
		wantSetting string
		wantMissing bool
	}{
		{"unreadable certificate", missing, keyFile, missing, "environment gnmi tls cert", true},
		{"unreadable key", certFile, missing, missing, "environment gnmi tls key", true},
		{"key without certificate", "", keyFile, keyFile, "environment gnmi tls cert", false},
		{"certificate without key", certFile, "", certFile, "environment gnmi tls key", false},
		{"mismatched pair", certFile, otherKeyFile, otherKeyFile, "environment gnmi tls cert", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := NewServer(Config{ListenAddr: "127.0.0.1:0", CertFile: tt.cert, KeyFile: tt.key},
				treeFunc(zeconfig.NewTree()), nil, yangloader.DefaultLoader, nil)

			serveErr := make(chan error, 1)
			go func() { serveErr <- srv.Serve(t.Context()) }()
			var err error
			select {
			case err = <-serveErr:
			case <-time.After(2 * time.Second):
				t.Fatalf("Serve is serving on %q over TLS material it cannot load", srv.Address())
			}

			if addr := srv.Address(); addr != "" {
				t.Errorf("Serve bound %s; a refused TLS config must bind nothing", addr)
			}
			var refusal *opererr.Error
			if !errors.As(err, &refusal) {
				t.Fatalf("Serve error = %v (%T), want an operator error", err, err)
			}
			if !strings.Contains(refusal.Problem(), tt.wantFile) {
				t.Errorf("problem %q does not name the file %s", refusal.Problem(), tt.wantFile)
			}
			if !strings.Contains(refusal.Fix(), tt.wantSetting) {
				t.Errorf("fix %q does not name the setting %q", refusal.Fix(), tt.wantSetting)
			}
			if got := errors.Is(err, fs.ErrNotExist); got != tt.wantMissing {
				t.Errorf("errors.Is(err, fs.ErrNotExist) = %v, want %v: the read error must stay the cause", got, tt.wantMissing)
			}
		})
	}
}

// VALIDATES: the control case. Readable, matching material serves TLS: a TLS
// client completes Capabilities and is shown the configured certificate, a
// clear-text client is refused, and Status reports TLS configured.
// PREVENTS: a fix that refuses good material too, or that loads it and still
// serves clear text.
func TestServeTLSWithValidMaterial(t *testing.T) {
	certFile, keyFile := writeTestKeyPair(t, t.TempDir(), "server")
	srv := NewServer(Config{ListenAddr: "127.0.0.1:0", CertFile: certFile, KeyFile: keyFile},
		treeFunc(zeconfig.NewTree()), nil, yangloader.DefaultLoader, nil)
	startTestServer(t, srv)

	if !srv.Status().TLSConfigured {
		t.Error("Status().TLSConfigured = false for a server serving TLS")
	}

	var presented []byte
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		// The certificate is self-signed, so the chain is not verified;
		// VerifyConnection checks the server presented the configured one.
		InsecureSkipVerify: true, //nolint:gosec // VerifyConnection pins the presented certificate
		VerifyConnection: func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) == 0 {
				return errors.New("server presented no certificate")
			}
			presented = state.PeerCertificates[0].Raw
			return nil
		},
	}
	conn, err := grpc.NewClient(srv.Address(), grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	if err != nil {
		t.Fatalf("dial TLS: %v", err)
	}
	t.Cleanup(func() { conn.Close() }) //nolint:errcheck // test cleanup

	ctx := t.Context()
	if _, err := gpb.NewGNMIClient(conn).Capabilities(ctx, &gpb.CapabilityRequest{}); err != nil {
		t.Fatalf("Capabilities over TLS: %v", err)
	}
	want, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		t.Fatalf("reload pair: %v", err)
	}
	if !bytes.Equal(presented, want.Certificate[0]) {
		t.Error("the server presented a certificate other than the configured one")
	}

	clearCtx, clearCancel := context.WithTimeout(ctx, 5*time.Second)
	defer clearCancel()
	if _, err := dialTestServer(t, srv.Address()).Capabilities(clearCtx, &gpb.CapabilityRequest{}); err == nil {
		t.Error("a clear-text client completed Capabilities against a TLS listener")
	}
}
