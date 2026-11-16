// Design: docs/architecture/api/process-protocol.md -- initialized subsystem startup.

package server

import (
	"context"
	"crypto/tls"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/core/selfcert"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const subsystemInitializerChild = "ZE_TEST_SUBSYSTEM_INITIALIZER_CHILD"
const subsystemInitializerMarker = "ZE_TEST_SUBSYSTEM_INITIALIZER_MARKER"

// This child runs the actual external SDK over the inherited TLS transport.
// Separate 16s callbacks each fit their accepted 20s allowance; together they
// deliberately cross the legacy Start wrapper's unrelated 30s deadline.
func TestSubsystemInitializerExternalChild(t *testing.T) {
	if os.Getenv(subsystemInitializerChild) != "1" {
		return
	}
	p, err := sdk.NewFromEnv(os.Getenv("ZE_PLUGIN_NAME"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	p.OnConfigure(func([]sdk.ConfigSection) error { time.Sleep(16 * time.Second); return nil })
	p.OnStarted(func(ctx context.Context) error {
		timer := time.NewTimer(16 * time.Second)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return ctx.Err()
		}
		return os.WriteFile(os.Getenv(subsystemInitializerMarker), []byte("initialized"), 0o600)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := p.Run(ctx, &sdk.Registration{ApplyBudget: 20}); err != nil {
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	}
}

// MUTATION: restoring Start's total 30s timeout refuses this otherwise valid
// external initializer. Omitting the receipt instead returns without its marker.
func TestSubsystemStartJoinsAcceptedExternalInitializer(t *testing.T) {
	setStartupStageTimeout(t, "5s")
	certPEM, keyPEM, err := selfcert.GenerateWebCertWithNames("127.0.0.1:0", nil, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS13})
	if err != nil {
		t.Fatal(err)
	}
	acceptor := ipc.NewPluginAcceptor(listener, "subsystem-initializer-test", certPEM)
	acceptor.Start()
	t.Cleanup(acceptor.Stop)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(t.TempDir(), "initialized")
	t.Setenv(subsystemInitializerChild, "1")
	t.Setenv(subsystemInitializerMarker, marker)
	t.Setenv("GORACE", "atexit_sleep_ms=0")
	quoted := "'" + strings.ReplaceAll(executable, "'", "'\"'\"'") + "'"
	handler := NewSubsystemHandler(SubsystemConfig{Name: "initialized-subsystem", Binary: "exec " + quoted + " -test.run='^TestSubsystemInitializerExternalChild$'"})
	handler.SetAcceptor(acceptor)
	t.Cleanup(handler.Stop)
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	if err := handler.Start(ctx); err != nil {
		t.Fatalf("accepted external initialization failed: %v", err)
	}
	initialized, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("Start admitted the subsystem before its initializer completed: %v", err)
	}
	if string(initialized) != "initialized" {
		t.Fatalf("wrong initialization marker: %q", initialized)
	}
	if !handler.Running() {
		t.Fatal("initialized subsystem is not running")
	}
}
