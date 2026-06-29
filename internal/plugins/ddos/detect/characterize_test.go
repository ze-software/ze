package detect

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	"codeberg.org/thomas-mangin/ze/internal/core/ddosevent"
)

// VALIDATES: parseTopDestination picks the highest-byte destination as a host
// prefix and rejects empty/absent/malformed/bad-IP input (Phase 1 target fill).
// PREVENTS: a malformed or empty trafficusage response producing a bogus or
// panicking target prefix instead of a clean fallback.
func TestParseTopDestination(t *testing.T) {
	cases := []struct {
		name string
		data string
		want string // prefix string; "" means expect ok=false
	}{
		{"dominant dst", `{"egress-ips":[{"ip":"203.0.113.5","bytes":9000},{"ip":"203.0.113.6","bytes":100}]}`, "203.0.113.5/32"},
		{"single dst", `{"egress-ips":[{"ip":"198.51.100.7","bytes":1}]}`, "198.51.100.7/32"},
		{"order independent", `{"egress-ips":[{"ip":"203.0.113.6","bytes":100},{"ip":"203.0.113.5","bytes":9000}]}`, "203.0.113.5/32"},
		{"empty list", `{"egress-ips":[]}`, ""},
		{"field absent", `{"interface":"xe0","ingress-ports":[]}`, ""},
		{"not-configured", `{"status":"not-configured"}`, ""},
		{"malformed json", `not json`, ""},
		{"bad ip", `{"egress-ips":[{"ip":"not-an-ip","bytes":5}]}`, ""},
		{"blank ip skipped", `{"egress-ips":[{"ip":"","bytes":9999},{"ip":"203.0.113.8","bytes":3}]}`, "203.0.113.8/32"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseTopDestination(json.RawMessage(tc.data))
			if tc.want == "" {
				if ok {
					t.Fatalf("expected ok=false, got %v", got)
				}
				return
			}
			if !ok {
				t.Fatalf("expected ok=true with %s", tc.want)
			}
			if got.String() != tc.want {
				t.Errorf("got %s want %s", got, tc.want)
			}
		})
	}
}

func TestCharacterizeTargetQueriesAndParses(t *testing.T) {
	var gotCmd string
	d := newDetector(DefaultConfig(), newDTestBus(), func(_ context.Context, cmd string) (string, json.RawMessage, error) {
		gotCmd = cmd
		return statusDone, json.RawMessage(`{"egress-ips":[{"ip":"203.0.113.9","bytes":5000}]}`), nil
	})
	prefix, ok := d.characterizeTarget(context.Background(), "xe0")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if prefix.String() != "203.0.113.9/32" {
		t.Errorf("prefix: got %s want 203.0.113.9/32", prefix)
	}
	if gotCmd != "show traffic-usage name xe0" {
		t.Errorf("command: got %q want \"show traffic-usage name xe0\"", gotCmd)
	}
}

func TestCharacterizeTargetFallbackOnError(t *testing.T) {
	d := newDetector(DefaultConfig(), newDTestBus(), func(_ context.Context, _ string) (string, json.RawMessage, error) {
		return "", nil, errors.New("ErrUnknownCommand")
	})
	if _, ok := d.characterizeTarget(context.Background(), "xe0"); ok {
		t.Error("expected ok=false on dispatch error")
	}
}

func TestCharacterizeTargetFallbackOnNonDone(t *testing.T) {
	d := newDetector(DefaultConfig(), newDTestBus(), func(_ context.Context, _ string) (string, json.RawMessage, error) {
		return "error", nil, nil
	})
	if _, ok := d.characterizeTarget(context.Background(), "xe0"); ok {
		t.Error("expected ok=false when status != done")
	}
}

func TestCharacterizeTargetFallbackOnNilDispatch(t *testing.T) {
	d := newDetector(DefaultConfig(), newDTestBus(), nil)
	if _, ok := d.characterizeTarget(context.Background(), "xe0"); ok {
		t.Error("expected ok=false when no dispatch is wired")
	}
}

// floodInto drives the detector from idle to an active attack on iface "xe0".
func floodInto(d *detector) {
	var cumPkts uint64
	for range 20 {
		cumPkts += 50
		d.onRate([]iface.InterfaceInfo{{Name: "xe0", Stats: &iface.InterfaceStats{RxPackets: cumPkts}}})
	}
	for range 5 {
		cumPkts += 100000
		d.onRate([]iface.InterfaceInfo{{Name: "xe0", Stats: &iface.InterfaceStats{RxPackets: cumPkts}}})
	}
	d.wg.Wait()
}

func floodConfig() *Config {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.ConfirmDuration = 1
	cfg.AbsoluteFloor = 100
	cfg.BaselineWindow = 10
	cfg.StartupGrace = 0
	return cfg
}

// AC-1: a flood produces an AttackDetected carrying a valid target prefix.
func TestDetectorEmitsTargetOnTrigger(t *testing.T) {
	bus := newDTestBus()
	d := newDetector(floodConfig(), bus, func(_ context.Context, _ string) (string, json.RawMessage, error) {
		return statusDone, json.RawMessage(`{"egress-ips":[{"ip":"203.0.113.42","bytes":1000000}]}`), nil
	})

	var detected *ddosevent.AttackDetected
	ddosevent.Detected.Subscribe(bus, func(e *ddosevent.AttackDetected) { detected = e })

	floodInto(d)

	if detected == nil {
		t.Fatal("AttackDetected not emitted")
	}
	if !detected.Target.DstPrefix.IsValid() {
		t.Fatal("expected a valid target prefix")
	}
	if detected.Target.DstPrefix.String() != "203.0.113.42/32" {
		t.Errorf("target: got %s want 203.0.113.42/32", detected.Target.DstPrefix)
	}
}

// AC-10: with no reachable source the detector still emits, with an empty target
// and the generic family -- behavior no worse than before characterization.
func TestDetectorFallbackWhenSourceAbsent(t *testing.T) {
	bus := newDTestBus()
	d := newDetector(floodConfig(), bus, func(_ context.Context, _ string) (string, json.RawMessage, error) {
		return "", nil, errors.New("ErrUnknownCommand")
	})

	var detected *ddosevent.AttackDetected
	ddosevent.Detected.Subscribe(bus, func(e *ddosevent.AttackDetected) { detected = e })

	floodInto(d)

	if detected == nil {
		t.Fatal("AttackDetected must still be emitted on fallback")
	}
	if detected.Target.DstPrefix.IsValid() {
		t.Errorf("expected empty target on source-absent, got %s", detected.Target.DstPrefix)
	}
	if detected.Family != ddosevent.FamilyGenericFlood {
		t.Errorf("family: got %s want %s", detected.Family, ddosevent.FamilyGenericFlood)
	}
}

// VALIDATES: Ongoing is suppressed until the asynchronous Detected has been
// emitted, preserving Detected-before-Ongoing ordering for subscribers.
// PREVENTS: the ordering regression from making Detected async -- a slow flow
// query must not let an Ongoing reach subscribers before the attack's Detected.
func TestOngoingGatedUntilDetected(t *testing.T) {
	bus := newDTestBus()
	var mu sync.Mutex
	var order []string
	ddosevent.Detected.Subscribe(bus, func(_ *ddosevent.AttackDetected) {
		mu.Lock()
		order = append(order, "detected")
		mu.Unlock()
	})
	ddosevent.Ongoing.Subscribe(bus, func(_ *ddosevent.AttackOngoing) {
		mu.Lock()
		order = append(order, "ongoing")
		mu.Unlock()
	})

	release := make(chan struct{})
	d := newDetector(floodConfig(), bus, func(_ context.Context, _ string) (string, json.RawMessage, error) {
		<-release // simulate a slow flow query so Detected is delayed
		return statusDone, json.RawMessage(`{"egress-ips":[{"ip":"203.0.113.1","bytes":1}]}`), nil
	})

	var cum uint64
	for range 20 { // baseline
		cum += 50
		d.onRate([]iface.InterfaceInfo{{Name: "xe0", Stats: &iface.InterfaceStats{RxPackets: cum}}})
	}
	for range 5 { // flood: activate tick + ticks that would emit Ongoing
		cum += 100000
		d.onRate([]iface.InterfaceInfo{{Name: "xe0", Stats: &iface.InterfaceStats{RxPackets: cum}}})
	}

	// Detected is still blocked in the goroutine, so no event may have fired.
	mu.Lock()
	pre := append([]string(nil), order...)
	mu.Unlock()
	if len(pre) != 0 {
		t.Fatalf("no event may precede Detected, got %v", pre)
	}

	close(release)
	d.wg.Wait()

	cum += 100000 // one more active tick now that Detected has landed
	d.onRate([]iface.InterfaceInfo{{Name: "xe0", Stats: &iface.InterfaceStats{RxPackets: cum}}})

	mu.Lock()
	defer mu.Unlock()
	if len(order) == 0 || order[0] != "detected" {
		t.Fatalf("Detected must be delivered first, got %v", order)
	}
}

// VALIDATES: a characterization that completes AFTER the attack has cleared does
// not emit a stale Detected (the generation guard drops it).
// PREVENTS: a stuck local drop installed by a late Detected with no matching
// Cleared to remove it (max-mitigation-duration is not enforced in ddos-local).
func TestNoStaleDetectedAfterClear(t *testing.T) {
	bus := newDTestBus()
	var mu sync.Mutex
	var events []string
	ddosevent.Detected.Subscribe(bus, func(_ *ddosevent.AttackDetected) {
		mu.Lock()
		events = append(events, "detected")
		mu.Unlock()
	})
	ddosevent.Cleared.Subscribe(bus, func(_ *ddosevent.AttackCleared) {
		mu.Lock()
		events = append(events, "cleared")
		mu.Unlock()
	})

	release := make(chan struct{})
	cfg := floodConfig()
	cfg.ClearConsecutive = 1 // clear quickly so it races the blocked query
	d := newDetector(cfg, bus, func(_ context.Context, _ string) (string, json.RawMessage, error) {
		<-release // the attack will clear before this returns
		return statusDone, json.RawMessage(`{"egress-ips":[{"ip":"203.0.113.1","bytes":1}]}`), nil
	})

	tick := func(p uint64) {
		d.onRate([]iface.InterfaceInfo{{Name: "xe0", Stats: &iface.InterfaceStats{RxPackets: p}}})
	}

	var cum uint64
	for range 20 { // baseline
		cum += 50
		tick(cum)
	}
	cum += 100000 // flood: activate, spawns the (blocked) characterization goroutine
	tick(cum)
	tick(cum) // delta 0 -> below: active -> clearing
	tick(cum) // delta 0 -> below: clearing -> idle -> Cleared emitted

	close(release) // characterization completes now, after Cleared
	d.wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	for _, e := range events {
		if e == "detected" {
			t.Fatalf("stale Detected emitted after clear: %v", events)
		}
	}
	if len(events) == 0 || events[len(events)-1] != "cleared" {
		t.Fatalf("expected a Cleared and no Detected, got %v", events)
	}
}
