// Design: docs/architecture/doctor-and-health-checks.md -- sysctl plugin event-emit failure reporting.
// Related: register.go -- runSysctlPlugin, the EventBus subscriber and issuequeue owner.

package sysctl

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	sysctlevents "github.com/ze-software/ze/internal/component/sysctl/events"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/test/pluginengine"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

var errTestAppliedEmitRefused = errors.New("event bus emit refused")

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type failingEmitBus struct {
	mu        sync.Mutex
	handlers  map[string][]func(any)
	failType  string
	failErr   error
	emitCount int
}

func newFailingEmitBus(failType string, failErr error) *failingEmitBus {
	return &failingEmitBus{
		handlers: make(map[string][]func(any)),
		failType: failType,
		failErr:  failErr,
	}
}

func (b *failingEmitBus) Emit(_, eventType string, _ any) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.emitCount++
	if eventType == b.failType {
		return 0, b.failErr
	}
	return 1, nil
}

func (b *failingEmitBus) Subscribe(namespace, eventType string, fn func(any)) func() {
	key := namespace + ":" + eventType
	b.mu.Lock()
	b.handlers[key] = append(b.handlers[key], fn)
	idx := len(b.handlers[key]) - 1
	b.mu.Unlock()
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if idx < len(b.handlers[key]) {
			b.handlers[key][idx] = nil
		}
	}
}

func (b *failingEmitBus) deliver(namespace, eventType string, payload any) {
	key := namespace + ":" + eventType
	b.mu.Lock()
	fns := append([]func(any){}, b.handlers[key]...)
	b.mu.Unlock()
	for _, fn := range fns {
		if fn != nil {
			fn(payload)
		}
	}
}

// TestSysctlAppliedEmitFailureIsWarned verifies that a failed (sysctl, applied)
// EventBus emit logs a WARN carrying what, why, fix, and the registered doctor
// code, and raises that event through issuequeue -> SDK RaiseIssue on both
// internal (bridge) and forked (wire) transports.
//
// VALIDATES: AC-4 -- sysctl applied emit failure is warned with registered
// doctor code and raised via SDK report-issue RPC.
// PREVENTS: silently dropping EventBus emit failures at Debug after applying
// a kernel tunable, or importing daemon-local internal/core/report from
// forkable sysctl.
func TestSysctlAppliedEmitFailureIsWarned(t *testing.T) {
	require.NotNil(t, diagnostic.Lookup(codeDoctorSysctlEventEmit),
		"doctor metadata for %s must be registered", codeDoctorSysctlEventEmit)
	require.True(t, slices.Contains(diagnostic.EventCodes(), codeDoctorSysctlEventEmit),
		"event code %s must be registered", codeDoctorSysctlEventEmit)

	for _, transport := range pluginengine.Transports() {
		t.Run(transport.String(), func(t *testing.T) {
			bus := newFailingEmitBus(sysctlevents.EventApplied, errTestAppliedEmitRefused)
			var logs syncBuffer

			prevBus := getEventBusRef()
			prevLogger := logger()
			prevBackend := newSysctlBackend

			setEventBusRef(bus)
			setLogger(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn})))
			newSysctlBackend = func() backend { return newFakeBackend() }

			// Registered before pluginengine.Start so Start's cleanup stops and
			// joins runSysctlPlugin before globals are restored.
			t.Cleanup(func() {
				newSysctlBackend = prevBackend
				setLogger(prevLogger)
				setEventBusRef(prevBus)
			})

			engine := pluginengine.Start(t, transport, runSysctlPlugin)
			reported := make(chan rpc.ReportIssueInput, 4)
			engine.SetRPCHandler(func(_ context.Context, method string, params json.RawMessage) (any, error) {
				if method != rpc.MethodReportIssue {
					return nil, pluginengine.ErrUnhandledRPC
				}
				var in rpc.ReportIssueInput
				if err := json.Unmarshal(params, &in); err == nil {
					reported <- in
				}
				return json.RawMessage("{}"), nil
			})

			payload, err := json.Marshal(struct {
				Key    string `json:"key"`
				Value  string `json:"value"`
				Source string `json:"source"`
			}{
				Key:    "net.core.somaxconn",
				Value:  "128",
				Source: "test-plugin",
			})
			require.NoError(t, err)

			bus.deliver(sysctlevents.Namespace, sysctlevents.EventDefault, string(payload))

			warnOut := logs.String()
			require.Contains(t, warnOut, "code="+codeDoctorSysctlEventEmit,
				"applied emit failure must log at WARN with registered doctor code")
			require.Contains(t, warnOut, "net.core.somaxconn",
				"WARN line must name the sysctl key")
			require.Contains(t, warnOut, "ze show sysctl",
				"WARN line must name the operator fix")
			require.Contains(t, warnOut, errTestAppliedEmitRefused.Error(),
				"WARN line must carry the emit error cause")

			select {
			case got := <-reported:
				require.Equal(t, rpc.ReportRaise, got.Action)
				require.Equal(t, rpc.ReportError, got.Severity)
				require.Equal(t, codeDoctorSysctlEventEmit, got.Code)
				require.Contains(t, got.Fix, "ze show sysctl")
				require.Contains(t, got.Message, "net.core.somaxconn")
				require.Contains(t, got.Message, errTestAppliedEmitRefused.Error())
			case <-time.After(2 * time.Second):
				t.Fatalf("expected %s report-issue RPC via issuequeue after applied emit failure", codeDoctorSysctlEventEmit)
			}
		})
	}
}

type unreadableOriginalBackend struct {
	mu      sync.Mutex
	values  map[string]string
	readErr error
}

func (b *unreadableOriginalBackend) read(_ string) (string, error) {
	return "", b.readErr
}

func (b *unreadableOriginalBackend) write(key, value string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.values == nil {
		b.values = make(map[string]string)
	}
	b.values[key] = value
	return nil
}

// TestSysctlOriginalUnreadAndQueryEmitFailuresAreReported verifies that:
//   - a failed original-value read in store.saveOriginal logs WARN with
//     codeDoctorSysctlOriginalUnread and raises through issuequeue -> SDK
//     RaiseIssue (never internal/core/report), and
//   - a failed query-result EventBus emit (show-result) logs WARN with
//     codeDoctorSysctlEventEmit and raises through issuequeue -> SDK RaiseIssue.
//
// VALIDATES: AC-4 -- sysctl saveOriginal and query-result emit failure reporting.
// PREVENTS: silently dropping unreadable original sysctl values or query-result
// EventBus emit errors at Debug.
func TestSysctlOriginalUnreadAndQueryEmitFailuresAreReported(t *testing.T) {
	require.NotNil(t, diagnostic.Lookup(codeDoctorSysctlOriginalUnread))
	require.True(t, slices.Contains(diagnostic.EventCodes(), codeDoctorSysctlOriginalUnread))

	bus := newFailingEmitBus(sysctlevents.EventShowResult, errTestAppliedEmitRefused)
	var logs syncBuffer

	prevBus := getEventBusRef()
	prevLogger := logger()
	prevBackend := newSysctlBackend

	setEventBusRef(bus)
	setLogger(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn})))
	newSysctlBackend = func() backend { return &unreadableOriginalBackend{readErr: syscall.EACCES} }

	t.Cleanup(func() {
		newSysctlBackend = prevBackend
		setLogger(prevLogger)
		setEventBusRef(prevBus)
	})

	engine := pluginengine.Start(t, pluginengine.TransportBridge, runSysctlPlugin)
	reported := make(chan rpc.ReportIssueInput, 8)
	engine.SetRPCHandler(func(_ context.Context, method string, params json.RawMessage) (any, error) {
		if method != rpc.MethodReportIssue {
			return nil, pluginengine.ErrUnhandledRPC
		}
		var in rpc.ReportIssueInput
		if err := json.Unmarshal(params, &in); err == nil {
			reported <- in
		}
		return json.RawMessage("{}"), nil
	})

	// 1. Trigger store.saveOriginal read failure via EventDefault.
	payload, err := json.Marshal(struct {
		Key    string `json:"key"`
		Value  string `json:"value"`
		Source string `json:"source"`
	}{
		Key:    "net.core.somaxconn",
		Value:  "256",
		Source: "test-plugin",
	})
	require.NoError(t, err)
	bus.deliver(sysctlevents.Namespace, sysctlevents.EventDefault, string(payload))

	select {
	case got := <-reported:
		require.Equal(t, rpc.ReportRaise, got.Action)
		require.Equal(t, rpc.ReportError, got.Severity)
		require.Equal(t, codeDoctorSysctlOriginalUnread, got.Code)
		require.Contains(t, got.Message, "net.core.somaxconn")
		require.Contains(t, got.Fix, "ze show sysctl")
	case <-time.After(2 * time.Second):
		t.Fatalf("expected %s report-issue RPC after saveOriginal read failure", codeDoctorSysctlOriginalUnread)
	}

	warnOut := logs.String()
	require.Contains(t, warnOut, "code="+codeDoctorSysctlOriginalUnread)
	require.Contains(t, warnOut, "net.core.somaxconn")

	// 2. Trigger show-result EventBus emit failure.
	bus.deliver(sysctlevents.Namespace, sysctlevents.EventShowRequest, `{"request-id":"req-42"}`)

	select {
	case got := <-reported:
		require.Equal(t, rpc.ReportRaise, got.Action)
		require.Equal(t, rpc.ReportError, got.Severity)
		require.Equal(t, codeDoctorSysctlEventEmit, got.Code)
		require.Equal(t, "req-42", got.Subject)
		require.Contains(t, got.Fix, "ze show sysctl")
	case <-time.After(2 * time.Second):
		t.Fatalf("expected %s report-issue RPC after show-result emit failure", codeDoctorSysctlEventEmit)
	}

	require.Contains(t, logs.String(), "code="+codeDoctorSysctlEventEmit)
}
