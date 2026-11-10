// Design: ai/rules/plugins.md -- ze_gnmi reload seam validation
//
//go:build ze_gnmi

package hub

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	zegnmi "github.com/ze-software/ze/internal/component/gnmi"
	"github.com/ze-software/ze/internal/core/selfcert"
)

func TestGNMIReloadNotify(t *testing.T) {
	if gnmiReloadNotify == nil {
		t.Fatal("ze_gnmi build: gNMI reload hook not installed")
	}

	old := activeGNMINotifier
	defer func() { activeGNMINotifier = old }()

	notifier := zegnmi.NewChangeNotifier()
	ch := notifier.Subscribe()
	if ch == nil {
		t.Fatal("expected gNMI subscription channel")
	}
	defer notifier.Unsubscribe(ch)
	activeGNMINotifier = notifier

	gnmiReloadNotify()

	select {
	case got := <-ch:
		updates := got.GetUpdate()
		if len(updates) != 1 || updates[0].GetVal().GetStringVal() != "config-reload" {
			t.Fatalf("reload notification = %#v, want config-reload update", got)
		}
	case <-time.After(time.Second):
		t.Fatal("gNMI reload hook did not notify subscribers")
	}
}

// writeGNMITLSPair writes a fresh certificate pair into dir and returns the
// two paths.
func writeGNMITLSPair(t *testing.T, dir string) (certFile, keyFile string) {
	t.Helper()
	certPEM, keyPEM, err := selfcert.GenerateWebCertWithAddr("127.0.0.1:0")
	if err != nil {
		t.Fatalf("generate certificate: %v", err)
	}
	certFile = filepath.Join(dir, "gnmi.pem")
	keyFile = filepath.Join(dir, "gnmi.key")
	if err := os.WriteFile(certFile, certPEM, 0o600); err != nil {
		t.Fatalf("write certificate: %v", err)
	}
	if err := os.WriteFile(keyFile, keyPEM, 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return certFile, keyFile
}

// startGNMIWithTLS runs the gated builder with gNMI started from the
// environment on an ephemeral loopback port and the given TLS files, and
// returns its handle. The registered server is put back afterwards.
func startGNMIWithTLS(t *testing.T, certFile, keyFile string) gnmiServer {
	t.Helper()
	setGNMIEnv(t, "ze.gnmi.enabled", "true")
	setGNMIEnv(t, "ze.gnmi.listen", "127.0.0.1:0")
	setGNMIEnv(t, "ze.gnmi.token", "gnmi-s3cret")
	setGNMIEnv(t, "ze.gnmi.tls.cert", certFile)
	setGNMIEnv(t, "ze.gnmi.tls.key", keyFile)
	oldNotifier := activeGNMINotifier
	t.Cleanup(func() { activeGNMINotifier = oldNotifier })

	return gnmiBuildImpl(&gnmiBuildInputs{Tree: zeconfig.NewTree()})
}

// VALIDATES: AC-7 of spec-doctor-coverage-11-hub-engine. When the configured
// gNMI certificate cannot be read, the builder starts no gNMI server: it
// answers no handle and registers no server, so nothing listens.
// PREVENTS: the clear-text fallback, where the read error printed a warning,
// left the PEM empty, and gNMI served its token and config without TLS.
func TestGNMIUnreadableCertNotStarted(t *testing.T) {
	dir := t.TempDir()
	_, keyFile := writeGNMITLSPair(t, dir)
	missing := filepath.Join(dir, "missing.pem")

	handle := startGNMIWithTLS(t, missing, keyFile)
	if handle != nil {
		handle.Stop()
		t.Fatal("gnmiBuild started gNMI over a certificate file it cannot read")
	}
	if srv := zegnmi.LookupServer(); srv != nil {
		t.Fatalf("a gNMI server is registered on %q after the TLS refusal", srv.Address())
	}
}

// VALIDATES: the control case. Readable material starts gNMI, and the
// registered server reports TLS configured on a bound address.
func TestGNMIReadableCertStarted(t *testing.T) {
	certFile, keyFile := writeGNMITLSPair(t, t.TempDir())

	handle := startGNMIWithTLS(t, certFile, keyFile)
	if handle == nil {
		t.Fatal("gnmiBuild did not start gNMI over readable TLS material")
	}
	t.Cleanup(handle.Stop)
	srv := zegnmi.LookupServer()
	if srv == nil {
		t.Fatal("no gNMI server registered after a TLS start")
	}
	status := srv.Status()
	if status.ListenAddress == "" {
		t.Error("the registered gNMI server is not bound")
	}
	if !status.TLSConfigured {
		t.Error("the registered gNMI server does not report TLS configured")
	}
}
