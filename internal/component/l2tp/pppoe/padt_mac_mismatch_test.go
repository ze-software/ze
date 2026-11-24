// Design: docs/architecture/l2tp/bng-5-pppoe.md -- per-interface PPPoE server discovery handling.
// Related: server.go -- InterfaceServer.handlePADT and padtMismatchLogLimiter.

package pppoe

import (
	"context"
	"log/slog"
	"net"
	"testing"
)

type capturedLogRecord struct {
	level slog.Level
	msg   string
	attrs map[string]slog.Value
}

// captureLogHandler records every slog record at or above Debug so tests can
// verify log level, reason attribute, and rate-limiting behavior.
type captureLogHandler struct {
	records []capturedLogRecord
}

func (*captureLogHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

//nolint:gocritic // hugeParam: slog.Handler interface requires slog.Record by value.
func (h *captureLogHandler) Handle(_ context.Context, r slog.Record) error {
	attrs := make(map[string]slog.Value, r.NumAttrs())
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value
		return true
	})
	h.records = append(h.records, capturedLogRecord{
		level: r.Level,
		msg:   r.Message,
		attrs: attrs,
	})
	return nil
}

func (h *captureLogHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *captureLogHandler) WithGroup(string) slog.Handler      { return h }

// TestHandlePADTMACMismatchLogsDebugRateLimited exercises AC-11.
//
// VALIDATES: handlePADT logs a PADT MAC mismatch at Debug with reason
// "padt-mac-mismatch", never at Warn, rate-limits repeated mismatched frames
// within a one-second window, and resumes logging with the suppressed count
// once the window advances.
// PREVENTS: a host on the access segment forging PADTs at line rate from
// flooding Warn logs.
func TestHandlePADTMACMismatchLogsDebugRateLimited(t *testing.T) {
	now := int64(1_700_000_000)
	restore := SetNowUnix(func() int64 { return now })
	t.Cleanup(restore)

	ownerMAC := net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0x01}
	localMAC := [EthALen]byte{0x10, 0x20, 0x30, 0x40, 0x50, 0x60}
	forgerMAC := [EthALen]byte{0xde, 0xad, 0xbe, 0xef, 0x00, 0x01}

	st := newSessionTable("eth0", 100)
	sid, err := st.AllocSID()
	if err != nil {
		t.Fatalf("AllocSID: %v", err)
	}
	if err := st.Add(&Session{SID: sid, MAC: ownerMAC, IfName: "eth0", State: StateSession}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	handler := &captureLogHandler{}
	s := &InterfaceServer{
		ifName:   "eth0",
		hwAddr:   localMAC,
		sessions: st,
		logger:   slog.New(handler),
	}

	const burstTotal = 20
	for i := range burstTotal {
		src := forgerMAC
		src[5] = byte(i + 1)
		s.handlePADT(&Packet{Code: CodePADT, SID: sid, SrcMAC: src, DstMAC: localMAC})
	}

	if s.sessions.Lookup(sid) == nil {
		t.Fatal("mismatched PADTs removed the live session")
	}
	if len(handler.records) != padtMismatchLogBurst {
		t.Fatalf("logged %d records for %d mismatched PADTs in one second, want %d",
			len(handler.records), burstTotal, padtMismatchLogBurst)
	}
	for i, rec := range handler.records {
		if rec.level != slog.LevelDebug {
			t.Fatalf("record %d level = %v, want Debug", i, rec.level)
		}
		if got := rec.attrs["reason"].String(); got != reasonPADTMACMismatch {
			t.Fatalf("record %d reason = %q, want %q", i, got, reasonPADTMACMismatch)
		}
		if _, ok := rec.attrs["sid"]; !ok {
			t.Fatalf("record %d missing sid attribute", i)
		}
	}

	// Advance past the one-second window: logging resumes and reports how many
	// frames were suppressed during the burst.
	now += padtMismatchLogWindowSec
	s.handlePADT(&Packet{Code: CodePADT, SID: sid, SrcMAC: forgerMAC, DstMAC: localMAC})

	if len(handler.records) != padtMismatchLogBurst+1 {
		t.Fatalf("logged %d records after window advance, want %d", len(handler.records), padtMismatchLogBurst+1)
	}
	last := handler.records[len(handler.records)-1]
	if last.level != slog.LevelDebug {
		t.Fatalf("post-window record level = %v, want Debug", last.level)
	}
	if got := last.attrs["reason"].String(); got != reasonPADTMACMismatch {
		t.Fatalf("post-window record reason = %q, want %q", got, reasonPADTMACMismatch)
	}
	wantSuppressed := int64(burstTotal - padtMismatchLogBurst)
	if got := last.attrs["suppressed"].Int64(); got != wantSuppressed {
		t.Fatalf("post-window suppressed = %d, want %d", got, wantSuppressed)
	}
}
