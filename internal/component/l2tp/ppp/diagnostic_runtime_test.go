// Design: docs/research/l2tpv2-ze-integration.md -- PPP runtime diagnostic producers

package ppp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

type diagnosticDHCPv6Reader struct {
	cancel          context.CancelFunc
	payload         []byte
	length          int
	readErr         error
	writeErr        error
	cancelWithError bool
	reads           int
}

func (r *diagnosticDHCPv6Reader) ReadDHCPv6(_ context.Context, payload []byte) (int, netip.AddrPort, error) {
	r.reads++
	if r.reads > 1 {
		r.cancel()
		return 0, netip.AddrPort{}, context.Canceled
	}
	if r.cancelWithError {
		r.cancel()
	}
	copy(payload, r.payload)
	return r.length, netip.MustParseAddrPort("[fe80::2]:546"), r.readErr
}

func (r *diagnosticDHCPv6Reader) WriteDHCPv6([]byte, netip.AddrPort) (int, error) {
	return 0, r.writeErr
}

// Goal: each changed DHCPv6 problem branch records its actual cause; normal
// cancellation and an ignored message record none. Method: drive the existing
// carrier loop with one scripted outcome followed by context cancellation.
func TestDHCPv6CarrierProblemEvents(t *testing.T) {
	for _, operation := range []string{"read", "oversized payload", "parse", "handle", "write", "ignored", "canceled", "closed", "canceled with backend error", "joined cancellation and backend", "wrapped cancellation"} {
		t.Run(operation, func(t *testing.T) {
			report.ResetForTest()
			t.Cleanup(report.ResetForTest)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			reader := &diagnosticDHCPv6Reader{cancel: cancel, payload: []byte{255, 0, 0, 1}, length: 4}
			serverID := testServerID()
			svc := newIPv6Service(iPv6ServiceConfig{})
			switch operation {
			case "read", "canceled with backend error":
				reader.readErr = errors.New("carrier receive failed")
				reader.cancelWithError = operation == "canceled with backend error"
			case "oversized payload":
				reader.length = 1501
			case "parse":
				reader.payload, reader.length = []byte{1, 0, 0, 1, 0}, 5
			case "handle", "write":
				reader.payload = buildSolicit([3]byte{1, 2, 3}, testServerID(), 7)
				reader.length = len(reader.payload)
				if operation == "handle" {
					serverID.ID = make([]byte, 1024)
				} else {
					reader.writeErr = errors.New("carrier send failed")
				}
			case "canceled", "closed":
				reader.cancelWithError = true
				reader.readErr = context.Canceled
				if operation == "closed" {
					reader.readErr = net.ErrClosed
				}
			case "joined cancellation and backend":
				reader.cancelWithError = true
				reader.readErr = errors.Join(context.Canceled, errors.New("carrier receive failed"))
			case "wrapped cancellation":
				reader.cancelWithError = true
				reader.readErr = fmt.Errorf("carrier stopped: %w", context.Canceled)
			case "ignored":
			}
			dhcpv6ServerLoop(ctx, reader, svc, serverID, nil, "created-ppp", slog.New(slog.DiscardHandler))
			entries := report.Errors(0)
			if operation == "ignored" || operation == "canceled" || operation == "closed" || operation == "wrapped cancellation" {
				if len(entries) != 0 {
					t.Fatalf("normal outcome reported a problem: %+v", entries)
				}
				return
			}
			if len(entries) != 1 || entries[0].Code != "doctor-ppp-ipv6-service" {
				t.Fatalf("missing IPv6 problem event: %+v", entries)
			}
			if entries[0].Fix != "ze show errors" || entries[0].Detail["interface"] != "created-ppp" {
				t.Fatalf("event lacks owning interface or next command: %+v", entries[0])
			}
			cause, ok := entries[0].Detail["error"].(string)
			if !ok || cause == "" || !strings.Contains(entries[0].Message, cause) {
				t.Fatalf("event lacks the actual cause: %+v", entries[0])
			}
		})
	}
}

type diagnosticRSReader struct {
	cancel          context.CancelFunc
	cause           error
	cancelWithError bool
	reads           int
}

func (r *diagnosticRSReader) ReadRS(context.Context) error {
	r.reads++
	if r.reads > 1 {
		r.cancel()
		return context.Canceled
	}
	if r.cancelWithError {
		r.cancel()
	}
	return r.cause
}

// Goal: unexpected closure and receive failure remain visible, while closure
// after cancellation is normal. Method: run the actual solicitation loop.
func TestSolicitationCarrierProblemEvents(t *testing.T) {
	for _, outcome := range []string{"failure", "unexpected closure", "canceled", "closed after cancellation", "canceled with backend error", "joined cancellation and backend", "wrapped cancellation"} {
		t.Run(outcome, func(t *testing.T) {
			report.ResetForTest()
			t.Cleanup(report.ResetForTest)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			reader := &diagnosticRSReader{cancel: cancel, cause: errors.New("RS receive failed")}
			switch outcome {
			case "unexpected closure":
				reader.cause = net.ErrClosed
			case "canceled":
				reader.cancelWithError, reader.cause = true, context.Canceled
			case "closed after cancellation":
				reader.cancelWithError, reader.cause = true, net.ErrClosed
			case "canceled with backend error":
				reader.cancelWithError = true
			case "joined cancellation and backend":
				reader.cancelWithError = true
				reader.cause = errors.Join(context.Canceled, errors.New("RS receive failed"))
			case "wrapped cancellation":
				reader.cancelWithError = true
				reader.cause = fmt.Errorf("RS carrier stopped: %w", context.Canceled)
			case "failure":
			}
			rsReaderLoop(ctx, reader, make(chan struct{}, 1), "created-ppp", slog.New(slog.DiscardHandler))
			entries := report.Errors(0)
			if outcome == "canceled" || outcome == "closed after cancellation" || outcome == "wrapped cancellation" {
				if len(entries) != 0 {
					t.Fatalf("normal stop reported a problem: %+v", entries)
				}
				return
			}
			if len(entries) != 1 || entries[0].Code != "doctor-ppp-ipv6-service" || entries[0].Detail["error"] != reader.cause.Error() {
				t.Fatalf("missing actual RS failure: %+v", entries)
			}
		})
	}
}

// Goal: refusing a live ownership transfer records admission history without
// relabeling it as cleanup. Method: drive the existing rejection producer.
func TestOwnershipConflictReportsAdmission(t *testing.T) {
	for _, existingKey := range []bool{false, true} {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)
		driver := NewProductionDriver(slog.New(slog.DiscardHandler))
		start := &StartSession{TunnelID: 7, SessionID: 8, SubscriberID: "l2tp:7:8"}
		driver.rejectExistingTransfer(start, existingKey)
		entries := report.Errors(0)
		if len(entries) != 1 || entries[0].Code != "doctor-ppp-session-admission" || entries[0].Detail["error"] == "" {
			t.Fatalf("missing ownership refusal event: %+v", entries)
		}
		if len(report.Warnings()) != 0 {
			t.Fatal("ownership conflict invented a cleanup warning")
		}
	}
}

// Goal: a refused handle's actual release failure is retained as cleanup.
// Method: drive StartSession.closeFiles with both successful and failed closes.
func TestRefusedSessionReleaseReportsCleanup(t *testing.T) {
	for _, failed := range []bool{false, true} {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)
		handle := newTestSessionHandle(&fakeBackend{}, "created-ppp")
		if failed {
			handle.closeErr = errors.New("owner release failed")
		}
		start := &StartSession{TunnelID: 7, SessionID: 8, Dataplane: handle}
		start.closeFiles()
		entries := report.Warnings()
		if !failed {
			if len(entries) != 0 {
				t.Fatalf("successful close reported cleanup failure: %+v", entries)
			}
			continue
		}
		if len(entries) != 1 || entries[0].Code != diagnostic.CodeDoctorPPPCleanup || entries[0].Detail["error"] != handle.closeErr.Error() {
			t.Fatalf("missing refused-owner cleanup: %+v", entries)
		}
	}
}

// Goal: blocking replacement names the retained IPv6 lifetime rather than
// silently leaving it to a log. Method: enter the existing replacement guard.
func TestRetainedIPv6ServiceReplacementReported(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	handle := newTestSessionHandle(&fakeBackend{}, "created-ppp")
	service := newIPv6Service(iPv6ServiceConfig{Dataplane: handle})
	session := &pppSession{dataplane: handle, ipv6Svc: service, logger: slog.New(slog.DiscardHandler)}
	session.afterLCPOpenIPv6Service()
	entries := report.Errors(0)
	if len(entries) != 1 || entries[0].Code != "doctor-ppp-ipv6-service" || session.ipv6Svc != service {
		t.Fatalf("retained service refusal disappeared or replaced its owner: %+v", entries)
	}
}
