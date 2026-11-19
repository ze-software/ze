// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- original cleanup diagnostics

package iface

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/test/pluginengine"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestPPPoECleanupDiagnosticUsesOwningSDK drives the same SDK callbacks as the
// engine on both transports. The actual cleanup callback supplies the cause;
// the doctor reader must not retry it or lose it beside a healthy client.
func TestPPPoECleanupDiagnosticUsesOwningSDK(t *testing.T) {
	for _, transport := range pluginengine.Transports() {
		t.Run(transport.String(), func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			original := NewPPPoEClient(PPPoEClientConfig{Name: "wan", SourceInterface: "eth0"}, nil, logger)
			healthy := NewPPPoEClient(PPPoEClientConfig{Name: "backup", SourceInterface: "eth1"}, nil, logger)
			set := &pppoeClientSet{mu: new(sync.Mutex), active: map[string]*PPPoEClient{"wan": original, "backup": healthy}}
			original.registry, healthy.registry = set, set
			engine := pluginengine.Start(t, transport, func(conn net.Conn) int {
				plugin := sdk.NewWithConn(componentNameInterface, conn)
				defer func() { _ = plugin.Close() }()
				configurePPPoECleanupDiagnostics(plugin, set)
				if err := plugin.Run(context.Background(), &sdk.Registration{
					DoctorChecks: []rpc.DoctorCheckDecl{pppoeCleanupDoctorDeclaration()},
				}); err != nil {
					return 1
				}
				return 0
			})
			var events []rpc.ReportIssueInput
			engine.SetRPCHandler(func(_ context.Context, method string, raw json.RawMessage) (any, error) {
				if method != rpc.MethodReportIssue {
					return nil, pluginengine.ErrUnhandledRPC
				}
				if !set.mu.TryLock() {
					t.Error("cleanup report held the registry mutex across SDK I/O")
				} else {
					set.mu.Unlock()
				}
				if !original.mu.TryLock() {
					t.Error("cleanup report held the original client mutex across SDK I/O")
				} else {
					original.mu.Unlock()
				}
				var in rpc.ReportIssueInput
				if err := json.Unmarshal(raw, &in); err != nil {
					return nil, err
				}
				events = append(events, in)
				return nil, nil //nolint:nilnil // report-issue acknowledges success without a result payload.
			})
			cause := errors.New("first original unit Close was refused")
			pending := true
			calls := 0
			original.pendingCleanup = func() error {
				calls++
				if pending {
					return errors.Join(cause, ErrNativePPPPending)
				}
				return nil
			}
			if err := original.finishCleanup(); !errors.Is(err, cause) {
				t.Fatalf("cleanup lost the original cause: %v", err)
			}
			rows, err := engine.DoctorCheck(t.Context(), pppoeCleanupDoctorName)
			if err != nil || len(rows) != 1 {
				t.Fatalf("runtime doctor rows=%+v error=%v, want the original cleanup", rows, err)
			}
			if calls != 1 || rows[0].Code != pppoeCleanupDoctorCode || !strings.Contains(rows[0].Message, cause.Error()) || !strings.Contains(rows[0].Message, "wan") || !strings.Contains(rows[0].Message, "eth0") || !strings.Contains(rows[0].Message, "unconfirmed") {
				t.Fatalf("doctor changed cleanup or lost its scope/cause: calls=%d rows=%+v", calls, rows)
			}
			if rows[0].Fix != "ze request restart" {
				t.Fatalf("cleanup fix %q does not restart the owner process", rows[0].Fix)
			}
			if len(events) != 1 || events[0].Action != rpc.ReportRaise || events[0].Severity != rpc.ReportError || events[0].Code != pppoeCleanupDoctorCode || !strings.Contains(events[0].Message, cause.Error()) || events[0].Fix != rows[0].Fix {
				t.Fatalf("owning SDK did not retain the cleanup event: %+v", events)
			}
			healthy.pendingCleanup = func() error { return nil }
			if err := healthy.finishCleanup(); err != nil {
				t.Fatal(err)
			}
			rows, err = engine.DoctorCheck(t.Context(), pppoeCleanupDoctorName)
			if err != nil || len(rows) != 1 || len(events) != 1 {
				t.Fatalf("healthy cleanup erased the original failure: rows=%+v events=%+v error=%v", rows, events, err)
			}
			pending = false
			if err := original.finishCleanup(); err != nil {
				t.Fatal(err)
			}
			rows, err = engine.DoctorCheck(t.Context(), pppoeCleanupDoctorName)
			if err != nil || len(rows) != 0 || len(events) != 1 || events[0].Action != rpc.ReportRaise {
				t.Fatalf("exact recovery did not clear current custody while retaining history: rows=%+v events=%+v error=%v", rows, events, err)
			}
		})
	}
}

// TestPPPoECleanupDiagnosticPublishesRetainedStartup proves that a failure met
// before engine calls are admitted is read from the same owner after readiness,
// without running cleanup again or issuing an SDK request before startup.
func TestPPPoECleanupDiagnosticPublishesRetainedStartup(t *testing.T) {
	for _, transport := range pluginengine.Transports() {
		t.Run(transport.String(), func(t *testing.T) {
			client := NewPPPoEClient(PPPoEClientConfig{Name: "startup-wan", SourceInterface: "eth0"}, nil,
				slog.New(slog.NewTextHandler(io.Discard, nil)))
			set := &pppoeClientSet{mu: new(sync.Mutex), active: map[string]*PPPoEClient{"startup-wan": client}}
			client.registry = set
			calls := 0
			cause := errors.New("original startup cleanup is unconfirmed")
			client.pendingCleanup = func() error {
				calls++
				return errors.Join(cause, ErrNativePPPPending)
			}
			if err := client.finishCleanup(); !errors.Is(err, cause) {
				t.Fatal(err)
			}
			engine := pluginengine.Start(t, transport, func(conn net.Conn) int {
				plugin := sdk.NewWithConn(componentNameInterface, conn)
				defer func() { _ = plugin.Close() }()
				configurePPPoECleanupDiagnostics(plugin, set)
				if err := plugin.Run(context.Background(), &sdk.Registration{
					DoctorChecks: []rpc.DoctorCheckDecl{pppoeCleanupDoctorDeclaration()},
				}); err != nil {
					return 1
				}
				return 0
			})
			raises := 0
			for _, call := range engine.RPCs() {
				if call.Method != rpc.MethodReportIssue {
					continue
				}
				var issue rpc.ReportIssueInput
				if err := json.Unmarshal(call.Params, &issue); err != nil {
					t.Fatal(err)
				}
				if issue.Code != pppoeCleanupDoctorCode || !strings.Contains(issue.Message, cause.Error()) {
					t.Fatalf("startup publication lost the original cause: %+v", issue)
				}
				raises++
			}
			if calls != 1 || raises != 1 {
				t.Fatalf("startup cleanup calls=%d reports=%d, want one original callback and one post-startup report", calls, raises)
			}
		})
	}
}

// TestPPPoECleanupDiagnosticSeparatesHistory checks the current callback's
// terminal error without claiming that physically confirmed cleanup is pending.
func TestPPPoECleanupDiagnosticSeparatesHistory(t *testing.T) {
	client := NewPPPoEClient(PPPoEClientConfig{Name: "wan"}, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	set := &pppoeClientSet{mu: new(sync.Mutex), active: map[string]*PPPoEClient{"wan": client}}
	client.registry = set
	cause := errors.New("earlier native request was refused")
	client.pendingCleanup = func() error { return cause }
	if err := client.finishCleanup(); !errors.Is(err, cause) {
		t.Fatal(err)
	}
	rows, err := set.cleanupDoctor(pppoeCleanupDoctorName)
	if err != nil || len(rows) != 1 || !strings.Contains(rows[0].Message, "completed") || strings.Contains(rows[0].Message, "unconfirmed") || !strings.Contains(rows[0].Message, cause.Error()) {
		t.Fatalf("terminal history was classified as pending custody: rows=%+v error=%v", rows, err)
	}
	if rows[0].Fix != "ze show warnings" {
		t.Fatalf("completed cleanup incorrectly requires a process restart: %+v", rows[0])
	}
	if !slices.Contains(diagnostic.EventCodes(), pppoeCleanupDoctorCode) {
		t.Fatal("cleanup event has no declaring doctor code")
	}
	declaration := pppoeCleanupDoctorDeclaration()
	if declaration.Phase != rpc.DoctorPhaseRuntime || !slices.Contains(declaration.Codes, pppoeCleanupDoctorCode) {
		t.Fatalf("cleanup declaration cannot reach the owning runtime: %+v", declaration)
	}
}
