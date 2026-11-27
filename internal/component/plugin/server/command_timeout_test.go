package server

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/flightrec"
	"github.com/ze-software/ze/internal/core/report"
)

type captureAuditRecorder struct {
	mu      sync.Mutex
	entries []audit.Entry
}

func (c *captureAuditRecorder) Record(entry audit.Entry) error {
	c.mu.Lock()
	c.entries = append(c.entries, entry)
	c.mu.Unlock()
	return nil
}

func (c *captureAuditRecorder) last() (audit.Entry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) == 0 {
		return audit.Entry{}, false
	}
	return c.entries[len(c.entries)-1], true
}

func TestDispatcherHandlerTimeoutFastAndAuditOpID(t *testing.T) {
	s := &Server{}
	d := NewDispatcher()
	rec := &captureAuditRecorder{}
	d.SetAuditRecorder(rec)

	var seenOpID string
	if err := d.RegisterWithOptions("show fast test", func(ctx *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		seenOpID = ctx.OpID()
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "fast test", RegisterOptions{AuditAction: "test-fast"}); err != nil {
		t.Fatalf("RegisterWithOptions: %v", err)
	}

	ctx := &CommandContext{Server: s, Username: "alice", Surface: audit.CLI}
	resp, err := d.Dispatch(ctx, "show fast test")
	if err != nil {
		t.Fatalf("Dispatch error: %v", err)
	}
	if resp == nil || resp.Status != plugin.StatusDone {
		t.Fatalf("resp = %+v, want StatusDone", resp)
	}
	if seenOpID == "" {
		t.Fatal("handler saw empty OpID")
	}
	if ctx.OpID() != seenOpID {
		t.Fatalf("ctx.OpID() = %q, want %q", ctx.OpID(), seenOpID)
	}
	entry, ok := rec.last()
	if !ok {
		t.Fatal("expected audit entry")
	}
	if entry.OpID != seenOpID {
		t.Fatalf("audit entry OpID = %q, want %q", entry.OpID, seenOpID)
	}
	if s.HungHandlerCount() != 0 {
		t.Fatalf("HungHandlerCount = %d, want 0", s.HungHandlerCount())
	}
}

func TestDispatcherHandlerTimeoutWedgedOperatorHandler(t *testing.T) {
	if err := flightrec.Start(); err == nil {
		t.Cleanup(flightrec.Stop)
	}

	s := &Server{}
	d := NewDispatcher()
	d.setHandlerTimeout(25 * time.Millisecond)

	unblock := make(chan struct{})
	t.Cleanup(func() { close(unblock) })

	if err := d.Register("show wedged test", func(_ *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		<-unblock
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "wedged test"); err != nil {
		t.Fatalf("Register: %v", err)
	}

	ctx := &CommandContext{Server: s, Username: "alice", Surface: audit.MCP}
	resp, err := d.Dispatch(ctx, "show wedged test")
	if err != nil {
		t.Fatalf("Dispatch returned err = %v, want nil with OperatorErrorResponse", err)
	}
	if resp == nil || resp.Status != plugin.StatusError {
		t.Fatalf("resp = %+v, want StatusError", resp)
	}
	opID := ctx.OpID()
	if opID == "" {
		t.Fatal("expected non-empty OpID")
	}
	if !strings.Contains(resp.Error, "show wedged test") {
		t.Errorf("error %q missing command name", resp.Error)
	}
	if !strings.Contains(resp.Error, "operation-id: "+opID) {
		t.Errorf("error %q missing operation-id %q", resp.Error, opID)
	}
	if !strings.Contains(resp.Error, "ze show system goroutines mode blocked") {
		t.Errorf("error %q missing goroutines fix", resp.Error)
	}

	if got := s.HungHandlerCount(); got != 1 {
		t.Fatalf("HungHandlerCount = %d, want 1", got)
	}
	hung := s.HungHandlers()
	if len(hung) != 1 {
		t.Fatalf("HungHandlers len = %d, want 1", len(hung))
	}
	if hung[0].Command != "show wedged test" || hung[0].OpID != opID || hung[0].Actor != "alice" || hung[0].Surface != audit.MCP {
		t.Fatalf("HungHandlers[0] = %+v, want matching command/opID/actor/surface", hung[0])
	}

	// Verify report bus recorded CodeDoctorCommandHandlerHung with matching OpID.
	issues := report.Errors(0)
	foundReport := false
	for i := range issues {
		if issues[i].Code == CodeDoctorCommandHandlerHung && issues[i].Subject == "show wedged test" && issues[i].OpID == opID {
			foundReport = true
			break
		}
	}
	if !foundReport {
		t.Fatalf("expected report error %q for command with OpID %q", CodeDoctorCommandHandlerHung, opID)
	}
}

func TestDispatcherHandlerTimeoutRingBound(t *testing.T) {
	s := &Server{}
	for i := range maxHungHandlerRecords + 4 {
		s.hungHandlers.record(HungHandlerRecord{
			Command: "cmd",
			OpID:    "op-" + string(rune('a'+i)),
		})
	}
	hung := s.HungHandlers()
	if len(hung) != maxHungHandlerRecords {
		t.Fatalf("HungHandlers len = %d, want %d", len(hung), maxHungHandlerRecords)
	}
}

func TestDispatcherHandlerTimeoutCallerCancelNoFalseHung(t *testing.T) {
	s := &Server{}
	d := NewDispatcher()
	d.setHandlerTimeout(2 * time.Second)

	started := make(chan struct{})
	if err := d.Register("show cancel test", func(ctx *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		close(started)
		<-ctx.Context().Done()
		return nil, ctx.Context().Err()
	}, "cancel test"); err != nil {
		t.Fatalf("Register: %v", err)
	}

	parentCtx, cancel := context.WithCancel(context.Background())
	ctx := &CommandContext{Server: s, RequestContext: parentCtx}

	go func() {
		<-started
		cancel()
	}()

	_, err := d.Dispatch(ctx, "show cancel test")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Dispatch err = %v, want context.Canceled", err)
	}
	if got := s.HungHandlerCount(); got != 0 {
		t.Fatalf("HungHandlerCount = %d, want 0 after caller cancel", got)
	}
	if hung := s.HungHandlers(); len(hung) != 0 {
		t.Fatalf("HungHandlers = %+v, want empty after caller cancel", hung)
	}
}

func TestDispatcherHandlerTimeoutPluginOriginNotCutOff(t *testing.T) {
	s := &Server{}
	d := NewDispatcher()
	d.setHandlerTimeout(15 * time.Millisecond)

	if err := d.Register("show plugin internal", func(_ *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		time.Sleep(50 * time.Millisecond)
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "plugin internal"); err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Non-nil Process represents a plugin-originated internal RPC dispatch.
	ctx := &CommandContext{Server: s, Process: &process.Process{}}
	resp, err := d.Dispatch(ctx, "show plugin internal")
	if err != nil {
		t.Fatalf("plugin-originated Dispatch err = %v, want nil", err)
	}
	if resp == nil || resp.Status != plugin.StatusDone {
		t.Fatalf("plugin-originated resp = %+v, want StatusDone", resp)
	}
	if s.HungHandlerCount() != 0 {
		t.Fatalf("HungHandlerCount = %d, want 0", s.HungHandlerCount())
	}
}

func TestServerWatchdogProbes(t *testing.T) {
	s := &Server{}
	s.setTxLockStallThreshold(15 * time.Millisecond)

	if stalled, _, _ := s.txLockWatchdogProbe(); stalled {
		t.Fatal("txLockWatchdogProbe reported stalled while unlocked")
	}
	if stalled, _, _ := s.hungHandlerWatchdogProbe(); stalled {
		t.Fatal("hungHandlerWatchdogProbe reported stalled with 0 hung handlers")
	}

	if !s.txLock.tryAcquire() {
		t.Fatal("txLock.tryAcquire failed")
	}
	t.Cleanup(s.txLock.release)

	if stalled, _, _ := s.txLockWatchdogProbe(); stalled {
		t.Fatal("txLockWatchdogProbe reported stalled immediately after acquire")
	}

	time.Sleep(25 * time.Millisecond)
	stalled, detail, held := s.txLockWatchdogProbe()
	if !stalled {
		t.Fatal("txLockWatchdogProbe did not report stall past threshold")
	}
	if detail == "" || held <= 15*time.Millisecond {
		t.Fatalf("txLockWatchdogProbe detail=%q held=%v, want non-empty detail and held > 15ms", detail, held)
	}

	s.hungHandlers.active.Add(1)
	defer s.hungHandlers.active.Add(-1)
	if hungStalled, hungDetail, _ := s.hungHandlerWatchdogProbe(); !hungStalled || !strings.Contains(hungDetail, "1 hung command handler") {
		t.Fatalf("hungHandlerWatchdogProbe = (%v, %q), want stalled with count", hungStalled, hungDetail)
	}
}

func TestDispatcherHungCapDoesNotLockOutHealthyDiagnostics(t *testing.T) {
	srv := &Server{}
	d := NewDispatcher()
	d.setHandlerTimeout(15 * time.Millisecond)

	release := make(chan struct{})
	defer close(release)

	err := d.RegisterWithOptions("show wedged", func(_ *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		<-release
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "wedged command", RegisterOptions{ReadOnly: true})
	if err != nil {
		t.Fatalf("RegisterWithOptions wedged: %v", err)
	}

	err = d.RegisterWithOptions("show system goroutines", func(_ *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"ok": true}}, nil
	}, "healthy diagnostic", RegisterOptions{ReadOnly: true})
	if err != nil {
		t.Fatalf("RegisterWithOptions healthy: %v", err)
	}

	// Wedge "show wedged" until it reaches its per-command active hung cap.
	for range maxActiveHungPerCommand {
		resp, err := d.Dispatch(&CommandContext{Server: srv, RequestContext: context.Background()}, "show wedged")
		if err != nil {
			t.Fatalf("Dispatch wedged err = %v", err)
		}
		if resp == nil || resp.Status != plugin.StatusError {
			t.Fatalf("Dispatch wedged resp = %+v, want StatusError", resp)
		}
	}

	// Next dispatch of the already-wedged command is refused immediately.
	start := time.Now()
	refused, err := d.Dispatch(&CommandContext{Server: srv, RequestContext: context.Background()}, "show wedged")
	if err != nil {
		t.Fatalf("Dispatch wedged capped err = %v", err)
	}
	if refused == nil || refused.Status != plugin.StatusError || !strings.Contains(refused.Error, "ze show system goroutines mode blocked") {
		t.Fatalf("capped refusal = %+v, want operator error naming fix", refused)
	}
	if time.Since(start) > 100*time.Millisecond {
		t.Errorf("capped refusal took %v, expected immediate refusal", time.Since(start))
	}

	// Healthy diagnostic command MUST still execute cleanly while "show wedged" is capped.
	diagResp, err := d.Dispatch(&CommandContext{Server: srv, RequestContext: context.Background()}, "show system goroutines")
	if err != nil {
		t.Fatalf("Dispatch healthy diagnostic err = %v", err)
	}
	if diagResp == nil || diagResp.Status != plugin.StatusDone {
		t.Fatalf("Dispatch healthy diagnostic resp = %+v, want StatusDone (diagnostics must not be locked out)", diagResp)
	}
}
