// Design: docs/architecture/api/process-protocol.md -- final process cleanup evidence
// Related: exit_cleanup_external_linux_test.go -- actual TLS startup and child joins
//go:build linux

package server

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// This actual fork makes its final report after SDK Run closes. Its mode is
// an input from this test's owned launch, never a fabricated ProcessState.
func TestTerminalCleanupEvidenceChild(t *testing.T) {
	mode := os.Getenv(exitCleanupExternalMode)
	if !strings.HasPrefix(mode, "evidence-") {
		return
	}
	client, err := sdk.NewFromEnv(os.Getenv("ZE_PLUGIN_NAME"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = client.Close() //nolint:errcheck // the parent closed this RPC connection; its actual child Wait still decides final release
	}()
	client.OnStarted(func(context.Context) error {
		return os.WriteFile(os.Getenv(exitCleanupExternalMarker), []byte("native-start"), 0o600)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	declaration := &sdk.Registration{ExitConfirmsCleanup: true}
	if mode == "evidence-ordinary-truncated-stderr" {
		declaration.ExitConfirmsCleanup = false
	}
	if err := client.Run(ctx, declaration); err != nil {
		t.Fatal(err)
	}
	switch mode {
	case "evidence-zero-failure", "evidence-disabled-relay":
		if _, err := fmt.Fprintf(os.Stderr, "level=ERROR msg=%q code=%s error=%q\n",
			"plugin cleanup failed", exitCleanupExternalCode, exitCleanupExternalCause); err != nil {
			t.Fatal(err)
		}
	case "evidence-unreported-failure":
		t.Error("original fork failed without a structured cleanup cause")
	case "evidence-truncated-stderr", "evidence-ordinary-truncated-stderr":
		// This exceeds the real relay scanner's single-line limit. Its write
		// fits within one consumed line plus a bounded remainder, so it does
		// not require the stopped scanner to consume another whole line.
		if _, err := fmt.Fprintln(os.Stderr, strings.Repeat("x", 64*1024+1)); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatalf("unknown owned cleanup evidence mode %q", mode)
	}
}

// An actual zero child exit cannot erase the original failure it reported
// after SDK closure. The exact native ProcessState must first confirm zero.
func TestExitCleanupExternalZeroExitKeepsFailure(t *testing.T) {
	exitCleanupExternalReset(t)
	handler := exitCleanupEvidenceHandler(t, "terminal-zero-cause", "evidence-zero-failure")
	if err := handler.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	handler.Stop()
	exitCleanupExternalJoin(t, handler)
	exitCleanupEvidenceZero(t, handler)
	exitCleanupExternalIssue(t, exitCleanupExternalCause)
}

// A declared fork's actual failure without a structured cause stays unknown,
// with its native exit status. Plain test output supplies no cleanup receipt.
func TestExitCleanupExternalUnreportedFailureRemainsUnknown(t *testing.T) {
	exitCleanupExternalReset(t)
	handler := exitCleanupEvidenceHandler(t, "terminal-no-cause", "evidence-unreported-failure")
	if err := handler.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	handler.Stop()
	exitCleanupExternalJoin(t, handler)
	command := handler.proc.Cmd()
	if command.ProcessState == nil {
		t.Fatal("unreported failure did not reap its actual child")
	}
	if command.ProcessState.ExitCode() != 1 {
		t.Fatalf("unreported-failure fixture changed its actual native status: %v", command.ProcessState)
	}
	issue := exitCleanupExternalIssue(t, "exit status 1")
	if !strings.Contains(issue.Message, "no cleanup failure cause") {
		t.Fatalf("missing cause became confirmed cleanup: %+v", issue)
	}
}

// The real log relay is disabled before launch, but the daemon still owns
// final failure reporting. The child exits zero after its structured cause.
func TestExitCleanupExternalDisabledRelayKeepsFailure(t *testing.T) {
	exitCleanupExternalReset(t)
	exitCleanupEvidenceDisableRelay(t)
	handler := exitCleanupEvidenceHandler(t, "terminal-disabled-relay", "evidence-disabled-relay")
	if err := handler.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	handler.Stop()
	exitCleanupExternalJoin(t, handler)
	exitCleanupEvidenceZero(t, handler)
	exitCleanupExternalIssue(t, exitCleanupExternalCause)
}

// A genuine zero exit with truncated final stderr cannot confirm cleanup.
// The original warning retains the missing evidence and actual zero status.
func TestExitCleanupExternalTruncatedStderrRemainsUnknown(t *testing.T) {
	exitCleanupExternalReset(t)
	handler := exitCleanupEvidenceHandler(t, "terminal-truncated-stderr", "evidence-truncated-stderr")
	if err := handler.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	handler.Stop()
	exitCleanupExternalJoin(t, handler)
	exitCleanupEvidenceZero(t, handler)
	issue := exitCleanupExternalIssue(t, "stderr")
	if !strings.Contains(issue.Message, "exit status 0") {
		t.Fatalf("unknown stderr lost its original actual native status: %+v", issue)
	}
}

// Declared incomplete stderr stays unknown even when relay output is disabled.
// The ordinary relay's discard policy supplies no native cleanup evidence.
func TestExitCleanupExternalDisabledTruncatedStderrRemainsUnknown(t *testing.T) {
	exitCleanupExternalReset(t)
	exitCleanupEvidenceDisableRelay(t)
	handler := exitCleanupEvidenceHandler(t, "terminal-disabled-truncation", "evidence-truncated-stderr")
	if err := handler.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	handler.Stop()
	exitCleanupExternalJoin(t, handler)
	exitCleanupEvidenceZero(t, handler)
	issue := exitCleanupExternalIssue(t, "stderr")
	if !strings.Contains(issue.Message, "exit status 0") {
		t.Fatalf("disabled truncated stderr lost actual zero exit: %+v", issue)
	}
}

// An undeclared process retains the disabled relay's existing discard policy.
// Its actual zero exit and oversized stderr invent no new diagnostic event.
func TestExitCleanupOrdinaryDisabledRelayKeepsDiscardPolicy(t *testing.T) {
	exitCleanupExternalReset(t)
	exitCleanupEvidenceDisableRelay(t)
	handler := exitCleanupEvidenceHandler(t, "terminal-ordinary-discard", "evidence-ordinary-truncated-stderr")
	if err := handler.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	handler.Stop()
	exitCleanupExternalJoin(t, handler)
	exitCleanupEvidenceZero(t, handler)
	for _, issue := range report.Errors(0) {
		if issue.Code == process.CodeDoctorPluginStderrRelayStopped {
			t.Fatalf("disabled ordinary relay gained an error event: %+v", issue)
		}
	}
	for _, issue := range report.Warnings() {
		if issue.Code == exitCleanupExternalCode {
			t.Fatalf("ordinary process gained a cleanup guarantee: %+v", issue)
		}
	}
}

func exitCleanupEvidenceDisableRelay(t *testing.T) {
	t.Helper()
	t.Setenv("ze.log.relay", "disabled")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	if _, enabled := slogutil.RelayLevel(); enabled {
		t.Fatal("cleanup fixture did not disable the actual log relay")
	}
}

func exitCleanupEvidenceHandler(t *testing.T, name, mode string) *SubsystemHandler {
	t.Helper()
	handler, _ := exitCleanupExternalHandler(t, name, mode)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	quoted := "'" + strings.ReplaceAll(executable, "'", "'\"'\"'") + "'"
	// This is the same owned TLS harness with a different real child helper.
	// Configuration changes before Start, never after publication.
	handler.config.Binary = "exec " + quoted + " -test.run='^TestTerminalCleanupEvidenceChild$'"
	return handler
}

func exitCleanupEvidenceZero(t *testing.T, handler *SubsystemHandler) {
	t.Helper()
	command := handler.proc.Cmd()
	if command.ProcessState == nil {
		t.Fatal("cleanup evidence did not reap its actual child")
	}
	if !command.ProcessState.Success() {
		t.Fatalf("cleanup evidence did not preserve its genuine zero exit: %v", command.ProcessState)
	}
}
