// Design: plan/learned/768-doctor-health-checks.md -- interface error counter monitoring
// Related: backend.go -- GetStats for counter reads
// Related: iface.go -- InterfaceStats with RxErrors/TxErrors fields

package iface

import (
	"context"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/health"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const reportSourceIface = "iface"
const reportCodeIfaceErrors = "iface-errors"

// ifaceErrorTracker tracks previous error counter values per interface
// to detect increases between polls.
type ifaceErrorTracker struct {
	mu   sync.Mutex
	prev map[string]errorSnapshot
}

type errorSnapshot struct {
	rxErrors uint64
	txErrors uint64
}

var errorTracker = &ifaceErrorTracker{prev: make(map[string]errorSnapshot)}

// CheckAllInterfaceErrors discovers all interfaces via the backend and checks
// their error counters. Called from the health registry on /health and show health.
func CheckAllInterfaceErrors() int {
	backend := GetBackend()
	if backend == nil {
		return 0
	}
	ifaces, err := backend.ListInterfaces()
	if err != nil {
		return 0
	}
	names := make([]string, len(ifaces))
	for i := range ifaces {
		names[i] = ifaces[i].Name
	}
	return CheckInterfaceErrors(names)
}

// CheckInterfaceErrors polls error counters for the given interfaces
// and raises iface-errors warnings on the report bus when counters increase.
// Returns the number of interfaces with increasing errors.
func CheckInterfaceErrors(names []string) int {
	backend := GetBackend()
	if backend == nil {
		return 0
	}

	statsMap := make(map[string]InterfaceStats, len(names))
	for _, name := range names {
		s, err := backend.GetStats(name)
		if err != nil || s == nil {
			continue
		}
		statsMap[name] = *s
	}
	return checkErrorsFromStats(statsMap)
}

// checkErrorsFromStats is the testable core: compares current stats against
// previous snapshots and raises/clears warnings. Separated from
// CheckInterfaceErrors so tests can inject stats without a full Backend.
func checkErrorsFromStats(current map[string]InterfaceStats) int {
	errorTracker.mu.Lock()
	defer errorTracker.mu.Unlock()

	findings := 0
	for name, stats := range current {
		prev, hasPrev := errorTracker.prev[name]
		errorTracker.prev[name] = errorSnapshot{rxErrors: stats.RxErrors, txErrors: stats.TxErrors}

		if !hasPrev {
			continue
		}

		rxDelta := stats.RxErrors - prev.rxErrors
		txDelta := stats.TxErrors - prev.txErrors

		if rxDelta > 0 || txDelta > 0 {
			var b textbuf.Buffer
			msg := b.Reset().Str("interface ").Str(name).Str(": rx-errors +").Uint(rxDelta).Str(", tx-errors +").Uint(txDelta).String()
			report.RaiseWarning(reportSourceIface, reportCodeIfaceErrors, name, msg,
				map[string]any{"rx_errors_delta": rxDelta, "tx_errors_delta": txDelta})
			findings++
		} else {
			report.ClearWarning(reportSourceIface, reportCodeIfaceErrors, name)
		}
	}
	return findings
}

// ResetErrorTracker clears stored snapshots. Test-only.
func ResetErrorTracker() {
	errorTracker.mu.Lock()
	defer errorTracker.mu.Unlock()
	errorTracker.prev = make(map[string]errorSnapshot)
}

// healthErrorProbe reports degraded while iface-errors warnings are active.
var healthErrorProbe = report.HealthProbeDegraded(reportCodeIfaceErrors)

// checkHealth sweeps interface error counters with a one-second guard, then
// reports the iface-errors warning state. Registered from register.go so
// deleting this component removes its health row.
func checkHealth() (health.Status, string) {
	done := make(chan struct{})
	go func() {
		CheckAllInterfaceErrors()
		close(done)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	select {
	case <-done:
	case <-ctx.Done():
		return health.StatusDegraded, "interface error check timed out"
	}
	return healthErrorProbe()
}
