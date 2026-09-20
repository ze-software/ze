// Design: docs/architecture/isis/isis-4-component-config.md -- reconcile applies
// a committed circuit parameter to the running circuit.
//
// VALIDATES: a commit that changes hello-interval, hold-multiplier or priority
// on a circuit that is ALREADY UP reaches that circuit, so the new period drives
// the Hello sender and the new holding time and priority ride the next IIH; and
// a change the running circuit cannot absorb (circuit-type, level, address
// family) closes and reopens it instead of being dropped.
// PREVENTS: the defect recorded in plan/journal/unwired-feature.md on 2026-09-06
// -- reconcile stored the new InterfaceConfig in e.running, reported the circuit
// changed, and left the live circuit.Circuit holding every value it was built
// with. Every assertion below reads the LIVE circuit or the wire, never
// e.running: asserting on e.running is what the defect already did.

package isis

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
)

// runningEngine starts an engine over a capturing backend with the circuits data
// describes, so a test can reconcile against a circuit whose socket is open and
// whose hello+sweep worker is running.
func runningEngine(t *testing.T, data string) (*engine, *capturingBackend) {
	t.Helper()
	cfg, err := parseISISConfig(sec(data))
	if err != nil {
		t.Fatalf("parseISISConfig: %v", err)
	}
	fb := &capturingBackend{}
	eng := newEngine(transport.New(fb))
	eng.setConfig(cfg)
	if err := eng.openCircuits(); err != nil {
		t.Fatalf("openCircuits: %v", err)
	}
	t.Cleanup(eng.shutdown)
	return eng, fb
}

// reconcileTo applies data to a running engine and returns the journal.
func reconcileTo(t *testing.T, eng *engine, data string) reconcileResult {
	t.Helper()
	cfg, err := parseISISConfig(sec(data))
	if err != nil {
		t.Fatalf("parseISISConfig(reload): %v", err)
	}
	return eng.reconcile(cfg)
}

// liveCircuitPeriods reports the Hello period the running circuit publishes for
// each level. It reads the circuit the engine is sending from, which is the only
// thing that answers whether the operator's value reached the wire.
func liveCircuitPeriods(t *testing.T, eng *engine, name string) map[adjacency.Level]time.Duration {
	t.Helper()
	eng.circuitsMu.RLock()
	c := eng.circuitByName[name]
	eng.circuitsMu.RUnlock()
	if c == nil {
		t.Fatalf("no live circuit for %s", name)
	}
	out := make(map[adjacency.Level]time.Duration)
	for _, s := range c.HelloSchedules() {
		out[s.Level] = s.Period
	}
	return out
}

// awaitLANHello waits for a captured LAN IIH carrying hold and priority, and
// reports whether one arrived. The reconcile hands the change to the running
// hello worker through a channel, so the IIH is produced by another goroutine;
// the deadline bounds the wait and a miss fails the caller rather than hanging.
func awaitLANHello(fb *capturingBackend, hold uint16, priority uint8) bool {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		fb.mu.Lock()
		c := fb.circuit
		fb.mu.Unlock()
		if c != nil {
			c.mu.Lock()
			frames := append([][]byte(nil), c.sent...)
			c.mu.Unlock()
			for _, pdu := range frames {
				p, err := packet.DecodePDU(pdu)
				if err != nil || p.LANHello == nil {
					continue
				}
				if uint16(p.LANHello.HoldingTime) == hold && p.LANHello.Priority == priority {
					return true
				}
			}
		}
		time.Sleep(2 * time.Millisecond)
	}
	return false
}

// TestISISReconcileAppliesTimerChangeToTheRunningCircuit: an operator commits a
// new hello-interval, hold-multiplier and priority on a circuit that is already
// up. The live circuit must adopt all three, and an IIH carrying the new holding
// time must go out AT ONCE rather than after the new (longer) period: a neighbor
// holds the adjacency only for the holding time the last IIH told it (ISO/IEC
// 10589 clause 8.2), so a silent 30-second gap after a 9-second holding time
// would drop the adjacency the change was meant to keep.
func TestISISReconcileAppliesTimerChangeToTheRunningCircuit(t *testing.T) {
	const before = `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l1-l2","interfaces":{"interface":{"eth0":` +
		`{"level":"l1-l2","circuit-type":"broadcast","hello-interval":"3","hold-multiplier":"3","priority":"10"}}}}}`
	const after = `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l1-l2","interfaces":{"interface":{"eth0":` +
		`{"level":"l1-l2","circuit-type":"broadcast","hello-interval":"30","hold-multiplier":"2","priority":"100"}}}}}`

	eng, fb := runningEngine(t, before)

	got := liveCircuitPeriods(t, eng, "eth0")
	for _, level := range []adjacency.Level{adjacency.Level1, adjacency.Level2} {
		if got[level] != 3*time.Second {
			t.Fatalf("before reconcile: %v period = %v, want 3s", level, got[level])
		}
	}

	res := reconcileTo(t, eng, after)
	if !res.changed["eth0"] {
		t.Fatalf("reconcile did not mark eth0 changed: %+v", res)
	}
	if len(res.rebuilt) != 0 {
		t.Errorf("a timer change rebuilt the circuit (%v), which flaps every adjacency on it", res.rebuilt)
	}

	got = liveCircuitPeriods(t, eng, "eth0")
	for _, level := range []adjacency.Level{adjacency.Level1, adjacency.Level2} {
		if got[level] != 30*time.Second {
			t.Errorf("after reconcile: live circuit %v period = %v, want 30s", level, got[level])
		}
	}

	// hold-multiplier 2 over a 30s interval is a 60s holding time, and the
	// priority rides the same IIH. Neither value can come from a ticker inside
	// this test's lifetime, so a frame carrying both proves the reconcile woke
	// the sender.
	if !awaitLANHello(fb, 60, 100) {
		t.Error("no IIH carrying the committed holding time 60 and priority 100 was sent after the reconcile")
	}
}

// TestISISReconcileRebuildsOnCircuitTypeChange: broadcast and point-to-point are
// different PDU types and different DIS behavior, both fixed when the circuit is
// constructed, so the reconcile closes the circuit and opens it again. The new
// circuit must be a different object publishing the single P2P Hello schedule.
func TestISISReconcileRebuildsOnCircuitTypeChange(t *testing.T) {
	const before = `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l1-l2","interfaces":{"interface":{"eth0":` +
		`{"level":"l1-l2","circuit-type":"broadcast"}}}}}`
	const after = `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l1-l2","interfaces":{"interface":{"eth0":` +
		`{"level":"l1-l2","circuit-type":"point-to-point"}}}}}`

	eng, _ := runningEngine(t, before)
	if n := len(liveCircuitPeriods(t, eng, "eth0")); n != 2 {
		t.Fatalf("before reconcile: %d hello schedules, want 2 (a broadcast L1L2 circuit)", n)
	}

	res := reconcileTo(t, eng, after)
	if len(res.rebuilt) != 1 || res.rebuilt[0] != "eth0" {
		t.Fatalf("reconcile rebuilt = %v, want [eth0]", res.rebuilt)
	}
	if n := len(liveCircuitPeriods(t, eng, "eth0")); n != 1 {
		t.Errorf("after reconcile: %d hello schedules, want 1 (a point-to-point circuit sends one IIH)", n)
	}
}

// TestISISReconcileSeesAnAddressFamilyChange: the address-family list decides
// whether the circuit advertises IPv6, and the circuit reads it once at
// construction. A diff that called the change equal would leave the commit
// invisible, and one that called it an in-place change would leave the running
// circuit answering for the old set.
func TestISISReconcileSeesAnAddressFamilyChange(t *testing.T) {
	v4 := InterfaceConfig{Name: "eth0", Enabled: true, AddressFamily: []string{"ipv4-unicast"}}
	both := InterfaceConfig{Name: "eth0", Enabled: true, AddressFamily: []string{"ipv4-unicast", "ipv6-unicast"}}

	if circuitParamsEqual(v4, both) {
		t.Error("circuitParamsEqual called an address-family change unchanged")
	}
	if !circuitNeedsRebuild(v4, both) {
		t.Error("circuitNeedsRebuild said an address-family change can be written into a running circuit")
	}
	if circuitNeedsRebuild(v4, v4) {
		t.Error("circuitNeedsRebuild asked for a rebuild on an unedited config")
	}
}

// TestISISReconcileAdoptsTheWholeConfig: a reload changes more than the
// interface list, and reconcile is the only callback that fires on one
// (OnConfigApply). Adding a key chain and pointing a circuit's level-1 at it
// must reach the key store, so the running circuit signs its IIH and the verify
// path knows the chain. Before the fix reconcile stored e.cfg directly and never
// called setConfig, so the key store kept the config the daemon started with and
// the hitless rotation setKeyStore documents could not happen on a reload.
func TestISISReconcileAdoptsTheWholeConfig(t *testing.T) {
	const chains = `"key-chains":{"iih-key":{"name":"iih-key","key":{"1":` +
		`{"key-id":"1","algorithm":"hmac-sha-256","secret":"s3cr3t"}}}},`
	const before = `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l1-l2",` +
		`"interfaces":{"interface":{"eth0":{"level":"l1-l2","circuit-type":"broadcast"}}}}}`
	const after = `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l1-l2",` + chains +
		`"interfaces":{"interface":{"eth0":{"level":"l1-l2","circuit-type":"broadcast",` +
		`"level-1":{"auth-key-chain":"iih-key"}}}}}}`

	eng, _ := runningEngine(t, before)
	eng.ksMu.RLock()
	configured := eng.keystore.configured()
	eng.ksMu.RUnlock()
	if configured {
		t.Fatal("the startup config declares no key chain, so the key store must hold none")
	}

	if res := reconcileTo(t, eng, after); !res.changed["eth0"] {
		t.Fatalf("reconcile did not mark eth0 changed: %+v", res)
	}

	eng.ksMu.RLock()
	configured = eng.keystore.configured()
	chain := eng.keystore.helloChain("eth0", levelOne) != nil
	eng.ksMu.RUnlock()
	if !configured {
		t.Error("the reloaded key chain never reached the key store: reconcile did not adopt the whole config")
	}
	if !chain {
		t.Error("the reloaded level-1 auth-key-chain resolved to no IIH chain for eth0")
	}
}
