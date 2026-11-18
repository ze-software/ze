// Design: docs/architecture/api/process-protocol.md -- forked engine cleanup outcome
// Related: subsystem.go -- actual five-stage SDK startup and original Process stop
//go:build linux

package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/selfcert"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const exitCleanupExternalMode = "ZE_TEST_TERMINAL_CLEANUP_CHILD"
const exitCleanupExternalMarker = "ZE_TEST_TERMINAL_CLEANUP_MARKER"
const exitCleanupExternalCause = "original forked engine cleanup failed"
const exitCleanupExternalCode = "doctor-plugin-cleanup"

// This helper runs only as the parent's actual fork. Its cleanup failure
// occurs after SDK Run ends, when report-issue RPC cannot be relied upon.
func TestTerminalCleanupExternalChild(t *testing.T) {
	mode := os.Getenv(exitCleanupExternalMode)
	if mode == "" {
		return
	}
	client, err := sdk.NewFromEnv(os.Getenv("ZE_PLUGIN_NAME"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = client.Close() //nolint:errcheck // the parent closes this RPC connection; actual child reaping confirms final descriptor release
	}()
	client.OnStarted(func(context.Context) error {
		return os.WriteFile(os.Getenv(exitCleanupExternalMarker), []byte("native-start"), 0o600)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	declaration := &sdk.Registration{ExitConfirmsCleanup: mode != "ordinary"}
	if err := client.Run(ctx, declaration); err != nil {
		if mode == "capacity" {
			return
		}
		t.Fatal(err)
	}
	if mode == "failure" {
		// Existing structured stderr supplies cause only. It supplies no
		// successful cleanup receipt and cannot select a different Process.
		if _, err := fmt.Fprintf(os.Stderr, "level=ERROR msg=%q code=%s error=%q\n",
			"plugin cleanup failed", exitCleanupExternalCode, exitCleanupExternalCause); err != nil {
			t.Error(err)
		}
		t.Error(exitCleanupExternalCause)
	}
}

// Expected Stop must retain the original fork's actual cleanup error after
// the real SDK connection closes, the child exits, and stderr is joined.
func TestExitCleanupExternalStopRetainsOriginalFailure(t *testing.T) {
	exitCleanupExternalReset(t)
	handler, _ := exitCleanupExternalHandler(t, "terminal-failed-fork", "failure")
	if err := handler.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	handler.Stop()
	exitCleanupExternalJoin(t, handler)
	before := exitCleanupExternalIssue(t, exitCleanupExternalCause)
	report.ClearSource(handler.proc.Name())
	report.ClearSource(before.Source)
	if after := exitCleanupExternalIssue(t, exitCleanupExternalCause); !reflect.DeepEqual(after, before) {
		t.Fatalf("fork cleanup replaced its original failure: before=%+v after=%+v", before, after)
	}
}

// A killed declared fork has no successful terminal cleanup result. Its
// original warning must name the actual signal instead of claiming release.
func TestExitCleanupKilledExternalOwnerRemainsUnknown(t *testing.T) {
	exitCleanupExternalReset(t)
	handler, _ := exitCleanupExternalHandler(t, "terminal-killed-fork", "success")
	if err := handler.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := handler.Signal(os.Kill); err != nil {
		t.Fatal(err)
	}
	exitCleanupExternalJoin(t, handler)
	exitCleanupExternalIssue(t, "signal")
}

// A full reservation set refuses the external opt-in at accepted Stage 1,
// before OnStarted writes its native-start marker. No compiled row is borrowed.
func TestExitCleanupExternalReservationPrecedesNativeStart(t *testing.T) {
	exitCleanupExternalReset(t)
	exitCleanupExternalFill(t)
	handler, marker := exitCleanupExternalHandler(t, "terminal-full-fork", "capacity")
	startErr := handler.Start(t.Context())
	handler.Stop()
	exitCleanupExternalJoin(t, handler)
	if startErr == nil {
		t.Fatal("external native owner started without a cleanup reservation")
	}
	if !strings.Contains(startErr.Error(), "resource warning reservation is full") {
		t.Fatalf("reservation refusal returned another failure: %v", startErr)
	}
	if _, err := os.ReadFile(marker); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("reservation refusal reached native startup: marker error=%v", err)
	}
	for _, issue := range report.Warnings() {
		if issue.Code == exitCleanupExternalCode {
			t.Fatalf("refused native admission invented cleanup: %+v", issue)
		}
	}
}

// An undeclared external program keeps its existing zero-exit semantics and
// can start while cleanup reservation capacity is full.
func TestExitCleanupOrdinaryExternalStartDoesNotReserve(t *testing.T) {
	exitCleanupExternalReset(t)
	exitCleanupExternalFill(t)
	handler, marker := exitCleanupExternalHandler(t, "terminal-ordinary-fork", "ordinary")
	if err := handler.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadFile(marker); err != nil {
		t.Fatal(err)
	}
	handler.Stop()
	exitCleanupExternalJoin(t, handler)
	for _, issue := range report.Warnings() {
		if issue.Code == exitCleanupExternalCode {
			t.Fatalf("ordinary exit gained a cleanup guarantee: %+v", issue)
		}
	}
}

func exitCleanupExternalReset(t *testing.T) {
	t.Helper()
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
}

func exitCleanupExternalFill(t *testing.T) {
	t.Helper()
	for index := range report.Losses().WarningCap {
		owner, err := report.NewResourceWarning("terminal-external-capacity", fmt.Sprintf("occupied-%d", index))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := owner.Close(); err != nil {
				t.Error(err)
			}
		})
	}
}

func exitCleanupExternalHandler(t *testing.T, name, mode string) (*SubsystemHandler, string) {
	t.Helper()
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
	acceptor := ipc.NewPluginAcceptor(listener, "terminal-cleanup-test-secret-at-least-32-bytes", certPEM)
	acceptor.Start()
	t.Cleanup(acceptor.Stop)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(t.TempDir(), "native-started")
	t.Setenv(exitCleanupExternalMode, mode)
	t.Setenv(exitCleanupExternalMarker, marker)
	t.Setenv("GORACE", "atexit_sleep_ms=0")
	quoted := "'" + strings.ReplaceAll(executable, "'", "'\"'\"'") + "'"
	handler := NewSubsystemHandler(SubsystemConfig{Name: name, Binary: "exec " + quoted + " -test.run='^TestTerminalCleanupExternalChild$'"})
	handler.SetAcceptor(acceptor)
	t.Cleanup(func() { handler.Stop(); exitCleanupExternalJoin(t, handler) })
	return handler, marker
}

func exitCleanupExternalJoin(t *testing.T, handler *SubsystemHandler) {
	t.Helper()
	if handler.proc == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := handler.proc.Wait(ctx); err != nil {
		t.Errorf("original fork did not join: %v", err)
	}
}

func exitCleanupExternalIssue(t *testing.T, cause string) report.Issue {
	t.Helper()
	for _, issue := range report.Warnings() {
		if issue.Code != exitCleanupExternalCode {
			continue
		}
		if !strings.Contains(issue.Message, cause) {
			continue
		}
		if issue.Subject == "" {
			t.Fatal("original fork cleanup has no subject")
		}
		if issue.Fix != "ze show warnings" {
			t.Fatalf("cleanup next command=%q", issue.Fix)
		}
		return issue
	}
	t.Fatalf("no retained original fork cleanup contains %q: %+v", cause, report.Warnings())
	return report.Issue{}
}
