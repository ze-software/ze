// Design: plan/learned/1116-copp-firewall-shutdown-flush.md -- fakeddos synthetic
// DDoS injector: runScenario is the two-phase driver (install on detect, withdraw
// on clear) behind the ddos-local withdraw functional test.

package fakeddos

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/ddosevent"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// targetPrefix is the synthetic victim: a host /32, so the responder resolves the
// direction to local and installs an INPUT drop in table ip ze_ddos-local.
const targetPrefix = "192.0.2.66/32"

// clearTriggerFile is the driver's "clear now" handshake: fakeddos holds the
// mitigation installed until this file appears in the daemon's working directory
// (where the driver also finds daemon.pid / daemon.ready), then emits the clear.
const clearTriggerFile = "ddos-fake.clear"

var loggerPtr atomic.Pointer[slog.Logger]

func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

func logger() *slog.Logger {
	if l := loggerPtr.Load(); l != nil {
		return l
	}
	return slog.Default()
}

var eventBusPtr atomic.Pointer[ze.EventBus]

func loadBus() (ze.EventBus, error) {
	p := eventBusPtr.Load()
	if p == nil {
		return nil, fmt.Errorf("fakeddos: event bus not configured")
	}
	return *p, nil
}

// runScenario installs one synthetic mitigation and holds it until clear fires,
// then withdraws it. The two phases are decoupled from boot timing on purpose:
// the table stays installed, so the driver observes it at its leisure and dictates
// exactly when the withdraw happens -- the present->absent transition is never a
// race the poller can miss. The caller (register.go) wires clear to the driver's
// trigger file.
func runScenario(ctx context.Context, bus ze.EventBus, clear <-chan struct{}) {
	target := ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix(targetPrefix)}
	detected := &ddosevent.AttackDetected{
		Interface:  "fakeddos",
		Target:     target,
		Family:     ddosevent.FamilyGenericFlood,
		Severity:   ddosevent.SeverityHigh,
		Direction:  ddosevent.DirectionLocal,
		Observable: true,
	}

	// Re-emit Detected (idempotent: ddos-local re-installs the same table) until the
	// driver triggers the clear. Emit's return count tallies only out-of-process RPC
	// subscribers -- it stays 0 for the in-process ddos-local responder even though
	// delivery is synchronous -- so it cannot signal that the responder has
	// subscribed. The ddos-local responder subscribes in its own OnConfigure, which
	// may run after ours; re-emitting every 100ms covers that ordering. The driver
	// only creates the trigger after it has observed the ze_ddos-local table, so by
	// the time clear fires a Detected has definitely reached the responder.
	emitted := false
	for {
		if _, err := ddosevent.Detected.Emit(bus, detected); err != nil {
			logger().Error("fakeddos: emit attack-detected failed", "error", err)
		}
		if !emitted {
			logger().Info("fakeddos: attack-detected emitted", "target", targetPrefix)
			emitted = true
		}
		select {
		case <-ctx.Done():
			return
		case <-clear:
			// Trigger fired: stop re-installing and withdraw below.
		case <-time.After(100 * time.Millisecond):
			continue
		}
		break
	}

	cleared := &ddosevent.AttackCleared{Interface: "fakeddos", Target: target, Observable: true}
	n, err := ddosevent.Cleared.Emit(bus, cleared)
	if err != nil {
		logger().Error("fakeddos: emit attack-cleared failed", "error", err)
	}
	logger().Info("fakeddos: attack-cleared delivered", "subscribers", n, "target", targetPrefix)
}
