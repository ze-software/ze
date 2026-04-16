// VALIDATES: typed event handles bind T to (namespace, eventType),
// SetLogger swaps the diagnostic logger atomically, and the AsString
// shim logs once per distinct wrong type.
// PREVENTS: silent type-mismatch drops, SetLogger race regressions,
// AsString suppression masking cascading bugs.
//
// INVARIANT: Tests in this file MUST NOT call t.Parallel(). They mutate
// the package-global loggerPtr via SetLogger; running them concurrently
// makes the "which logger is captured by my buffer" question
// nondeterministic even though atomic.Pointer keeps the swap race-free.
package events

import (
	"bytes"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// captureLogger returns a logger writing to buf and a getter func suitable
// for SetLogger.
func captureLogger(buf *bytes.Buffer) func() *slog.Logger {
	h := slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	l := slog.New(h)
	return func() *slog.Logger { return l }
}

// TestSetLoggerSwapsLogger verifies SetLogger replaces the diagnostic
// sink, and that calls before SetLogger fall back to slog.Default().
func TestSetLoggerSwapsLogger(t *testing.T) {
	var buf bytes.Buffer
	SetLogger(captureLogger(&buf))
	t.Cleanup(func() { SetLogger(slog.Default) })

	logger().Warn("test-msg", "k", "v")
	if !strings.Contains(buf.String(), "test-msg") {
		t.Errorf("warn did not land in captured logger; got %q", buf.String())
	}
}

// TestSetLoggerNilNoop verifies SetLogger(nil) does not clobber the
// installed logger.
func TestSetLoggerNilNoop(t *testing.T) {
	var buf bytes.Buffer
	SetLogger(captureLogger(&buf))
	t.Cleanup(func() { SetLogger(slog.Default) })

	SetLogger(nil) // must not replace the captured logger
	logger().Warn("after-nil")
	if !strings.Contains(buf.String(), "after-nil") {
		t.Errorf("nil SetLogger clobbered the prior logger; got %q", buf.String())
	}
}

// TestSetLoggerRaceFree exercises concurrent SetLogger / logger() calls
// to verify the atomic.Pointer swap is race-free under -race.
func TestSetLoggerRaceFree(t *testing.T) {
	var buf bytes.Buffer
	getter := captureLogger(&buf)
	SetLogger(getter)
	t.Cleanup(func() { SetLogger(slog.Default) })

	var wg sync.WaitGroup
	stop := make(chan struct{})
	for range 4 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					SetLogger(getter)
				}
			}
		}()
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					logger().Warn("race")
				}
			}
		}()
	}
	// Run for a short, deterministic burst.
	for range 1000 {
		logger().Warn("burst")
	}
	close(stop)
	wg.Wait()
}

// TestPayloadInfo verifies the combined-lookup contract: registered
// typed events return (typ, false), registered signal events return
// (signalType, true), and unregistered pairs return (nil, false).
// PREVENTS: a future refactor breaking the (nil, false) for-unregistered
// contract that callers like tryDecodeTypedPayload rely on as their
// "no typed contract here" signal.
func TestPayloadInfo(t *testing.T) {
	type testStruct struct{ X int }

	// Use private (ns, et) to avoid leaking into other test ordering.
	const ns = "events-test-payloadinfo"
	_ = Register[*testStruct](ns, "typed-event")
	_ = RegisterSignal(ns, "signal-event")

	t.Run("typed", func(t *testing.T) {
		typ, isSig := PayloadInfo(ns, "typed-event")
		if typ == nil {
			t.Fatalf("typ = nil; want *testStruct")
		}
		if isSig {
			t.Errorf("isSignal = true; want false for typed event")
		}
	})
	t.Run("signal", func(t *testing.T) {
		typ, isSig := PayloadInfo(ns, "signal-event")
		if typ == nil {
			t.Fatalf("typ = nil; want signalType sentinel")
		}
		if !isSig {
			t.Errorf("isSignal = false; want true for signal event")
		}
	})
	t.Run("unregistered", func(t *testing.T) {
		typ, isSig := PayloadInfo(ns, "never-registered")
		if typ != nil {
			t.Errorf("typ = %v; want nil for unregistered event", typ)
		}
		if isSig {
			t.Errorf("isSignal = true; want false for unregistered event")
		}
	})
}

// asStringPayload and asStringOther are sentinel types used to verify
// AsString logs once per distinct dynamic payload type.
type asStringPayload struct{ N int }
type asStringOther struct{ S string }

// TestAsStringLogsOncePerWrongType verifies AsString logs the first
// non-string drop for each distinct dynamic type (so a cascade of
// different wrong types each surfaces once) and suppresses repeated
// drops of the SAME type on the same wrapper to avoid log floods.
func TestAsStringLogsOncePerWrongType(t *testing.T) {
	var buf bytes.Buffer
	SetLogger(captureLogger(&buf))
	t.Cleanup(func() { SetLogger(slog.Default) })

	var stringCalls int32
	wrap := AsString(func(_ string) { atomic.AddInt32(&stringCalls, 1) })

	wrap("ok")                   // string path; no log
	wrap(&asStringPayload{N: 1}) // first wrong type; logs
	wrap(&asStringPayload{N: 2}) // same type; suppressed
	wrap(&asStringOther{S: "x"}) // different type; logs again

	if got := atomic.LoadInt32(&stringCalls); got != 1 {
		t.Errorf("stringCalls = %d, want 1", got)
	}
	logs := buf.String()
	count := strings.Count(logs, "AsString wrapper received non-string payload")
	if count != 2 {
		t.Errorf("expected exactly 2 warns (one per distinct wrong type), got %d in %q",
			count, logs)
	}
}
