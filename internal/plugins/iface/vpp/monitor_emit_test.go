// Design: docs/architecture/doctor-and-health-checks.md -- VPP monitor EventBus emit failure reporting.
// Related: monitor.go -- monitor.emit, reportEmitFailed, clearEmitWarned.

package ifacevpp

import (
	"bytes"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/diagnostic"
	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
	"github.com/ze-software/ze/internal/core/report"
)

type toggleEmitBus struct {
	mu  sync.Mutex
	err error
}

func (b *toggleEmitBus) Emit(_, _ string, _ any) (int, error) {
	b.mu.Lock()
	err := b.err
	b.mu.Unlock()
	if err != nil {
		return 0, err
	}
	return 1, nil
}

func (b *toggleEmitBus) Subscribe(_, _ string, _ func(any)) func() {
	return func() {}
}

// TestVPPMonitorEmitReportsFailuresRateLimitedAndResetsOnRecovery verifies
// that VPP monitor marshal and EventBus emit failures log one WARN carrying
// what, why, fix, and codeDoctorVPPMonitorEmit while raising once on
// internal/core/report, suppress consecutive duplicates, and reset on recovery.
//
// VALIDATES: AC-4 -- VPP monitor emit and marshal failures warn and raise on
// report bus with rate limiting.
// PREVENTS: silently dropping VPP interface notifications at Debug or flooding
// logs during a burst.
func TestVPPMonitorEmitReportsFailuresRateLimitedAndResetsOnRecovery(t *testing.T) {
	require.NotNil(t, diagnostic.Lookup(codeDoctorVPPMonitorEmit))
	require.True(t, slices.Contains(diagnostic.EventCodes(), codeDoctorVPPMonitorEmit))

	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	var logs bytes.Buffer
	prevLog := loggerPtr.Load()
	loggerPtr.Store(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { loggerPtr.Store(prevLog) })

	emitErr := errors.New("vpp event bus closed")
	bus := &toggleEmitBus{err: emitErr}
	m := &monitor{eventBus: bus}

	// First emit failure logs WARN and raises on report bus.
	m.emit(ifaceevents.EventUp, stateEventPayload{Name: "xe0", Index: 1})
	// Second immediate failure on the same event type is suppressed.
	m.emit(ifaceevents.EventUp, stateEventPayload{Name: "xe0", Index: 1})

	require.Equal(t, 1, strings.Count(logs.String(), "code="+codeDoctorVPPMonitorEmit),
		"repeated emit failure must be rate-limited to one WARN")
	require.Contains(t, logs.String(), "ze show doctor")
	require.Contains(t, logs.String(), emitErr.Error())

	var emitIssues int
	for _, issue := range report.Errors(0) {
		if issue.Code == codeDoctorVPPMonitorEmit && issue.Subject == ifaceevents.EventUp+":emit" {
			emitIssues++
			require.NotEmpty(t, issue.Fix)
		}
	}
	require.Equal(t, 1, emitIssues, "expected exactly one report-bus event for emit failure")

	// Marshal failure reports under the marshal stage.
	m.emit(ifaceevents.EventDown, make(chan int))
	m.emit(ifaceevents.EventDown, make(chan int))
	require.Equal(t, 2, strings.Count(logs.String(), "code="+codeDoctorVPPMonitorEmit),
		"marshal failure must log once and suppress consecutive duplicates")

	var marshalIssues int
	for _, issue := range report.Errors(0) {
		if issue.Code == codeDoctorVPPMonitorEmit && issue.Subject == ifaceevents.EventDown+":marshal" {
			marshalIssues++
		}
	}
	require.Equal(t, 1, marshalIssues, "expected one report-bus event for marshal failure")

	// Recovery resets the warn latch so a later emit failure warns again.
	bus.mu.Lock()
	bus.err = nil
	bus.mu.Unlock()
	m.emit(ifaceevents.EventUp, stateEventPayload{Name: "xe0", Index: 1})

	bus.mu.Lock()
	bus.err = emitErr
	bus.mu.Unlock()
	m.emit(ifaceevents.EventUp, stateEventPayload{Name: "xe0", Index: 1})
	require.Equal(t, 3, strings.Count(logs.String(), "code="+codeDoctorVPPMonitorEmit),
		"failure after a successful emit must warn again")
}
