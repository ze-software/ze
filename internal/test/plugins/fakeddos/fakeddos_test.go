// VALIDATES: the fakeddos injector's two-phase contract -- AttackDetected is
//            delivered up front (installing the ze_ddos-local mitigation) and
//            AttackCleared only after the clear channel fires (withdrawing it) --
//            plus the /32 IPv4 target the .ci driver's `nft list table ip
//            ze_ddos-local` grep depends on.
// PREVENTS: a regression where the injector clears before the driver observes the
//           table (making the withdraw test a false pass), or emits a target whose
//           family/hook no longer matches what the driver checks.

package fakeddos

import (
	"context"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/ddosevent"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

// countingBus is a minimal in-memory ze.EventBus that delivers synchronously to
// registered handlers (the shared ddosevent testBus is unexported, so this package
// needs its own). Its Emit return count is not load-bearing: runScenario ignores
// it and re-emits until the clear channel fires, exactly because the real bus does
// not count in-process subscribers.
type countingBus struct {
	mu   sync.Mutex
	subs map[string][]func(any)
}

func newCountingBus() *countingBus {
	return &countingBus{subs: make(map[string][]func(any))}
}

func (b *countingBus) Emit(namespace, eventType string, payload any) (int, error) {
	key := namespace + "/" + eventType
	b.mu.Lock()
	handlers := make([]func(any), len(b.subs[key]))
	copy(handlers, b.subs[key])
	b.mu.Unlock()
	for _, h := range handlers {
		h(payload)
	}
	return len(handlers), nil
}

func (b *countingBus) Subscribe(namespace, eventType string, handler func(any)) func() {
	key := namespace + "/" + eventType
	b.mu.Lock()
	b.subs[key] = append(b.subs[key], handler)
	b.mu.Unlock()
	return func() {}
}

var _ ze.EventBus = (*countingBus)(nil)

// TestTargetPrefixIsHostV4 locks the contract the .ci driver relies on: an IPv4
// /32 victim, so the responder builds an `ip`-family table and resolves the
// direction as local (INPUT drop). The driver greps `nft list table ip
// ze_ddos-local`; a non-v4 or non-host prefix would change the family/hook.
// TestActivationFromSections locks the `enabled` leaf gate: the injector starts
// only when the ddos/fake section sets enabled=true. The "enabled false stays
// inert" case is the regression for a review finding where enabled was parsed by
// YANG but never read, so `fake { enabled false; }` still activated the injector.
func TestActivationFromSections(t *testing.T) {
	tests := []struct {
		name     string
		sections []sdk.ConfigSection
		want     bool
		wantErr  bool
	}{
		{"enabled true (framework string form)", []sdk.ConfigSection{{Root: "ddos/fake", Data: `{"ddos":{"fake":{"enabled":"true"}}}`}}, true, false},
		{"enabled false stays inert", []sdk.ConfigSection{{Root: "ddos/fake", Data: `{"ddos":{"fake":{"enabled":"false"}}}`}}, false, false},
		{"enabled native bool true", []sdk.ConfigSection{{Root: "ddos/fake", Data: `{"ddos":{"fake":{"enabled":true}}}`}}, true, false},
		{"enabled absent defaults inert", []sdk.ConfigSection{{Root: "ddos/fake", Data: `{"ddos":{"fake":{}}}`}}, false, false},
		{"our section absent", []sdk.ConfigSection{{Root: "bgp", Data: `{}`}}, false, false},
		{"no sections", nil, false, false},
		{"malformed json errors", []sdk.ConfigSection{{Root: "ddos/fake", Data: `{not json`}}, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := activationFromSections(tt.sections)
			if (err != nil) != tt.wantErr {
				t.Fatalf("activationFromSections err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("activationFromSections = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTargetPrefixIsHostV4(t *testing.T) {
	p := netip.MustParsePrefix(targetPrefix)
	if !p.Addr().Is4() {
		t.Fatalf("targetPrefix %s must be IPv4 so the responder builds an `ip`-family table", targetPrefix)
	}
	if p.Bits() != 32 {
		t.Fatalf("targetPrefix %s must be a /32 host route so the victim resolves as local", targetPrefix)
	}
}

// TestRunScenarioEmitsDetectedThenClearedOnSignal proves the two-phase contract:
// AttackDetected is delivered up front (installing the mitigation) and
// AttackCleared is delivered only after the clear channel fires (withdrawing it),
// never before.
func TestRunScenarioEmitsDetectedThenClearedOnSignal(t *testing.T) {
	bus := newCountingBus()

	var detected, cleared atomic.Int32
	var gotTarget atomic.Pointer[netip.Prefix]
	ddosevent.Detected.Subscribe(bus, func(e *ddosevent.AttackDetected) {
		p := e.Target.DstPrefix
		gotTarget.Store(&p)
		detected.Add(1)
	})
	ddosevent.Cleared.Subscribe(bus, func(*ddosevent.AttackCleared) {
		cleared.Add(1)
	})

	ctx := t.Context()
	clear := make(chan struct{}, 1)

	done := make(chan struct{})
	go func() {
		runScenario(ctx, bus, clear)
		close(done)
	}()

	waitFor(t, func() bool { return detected.Load() >= 1 }, "attack-detected emitted")
	if cleared.Load() != 0 {
		t.Fatal("attack-cleared emitted before the clear signal")
	}
	if p := gotTarget.Load(); p == nil || p.String() != targetPrefix {
		t.Fatalf("detected target = %v, want %s", p, targetPrefix)
	}

	clear <- struct{}{}
	waitFor(t, func() bool { return cleared.Load() >= 1 }, "attack-cleared emitted after clear")

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runScenario did not return after clear")
	}
}

// TestRunScenarioStopsOnContextCancel proves the emit loop respects cancellation
// when no responder ever subscribes (n stays 0), rather than spinning forever.
func TestRunScenarioStopsOnContextCancel(t *testing.T) {
	bus := newCountingBus() // no subscribers: n>0 is never reached
	ctx, cancel := context.WithCancel(t.Context())
	clear := make(chan struct{})
	done := make(chan struct{})
	go func() {
		runScenario(ctx, bus, clear)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runScenario ignored context cancellation while waiting for a subscriber")
	}
}

func waitFor(t *testing.T, cond func() bool, what string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for: %s", what)
}
