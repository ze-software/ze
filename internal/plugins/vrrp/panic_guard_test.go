// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- panic containment in the receive and advertisement tasks
// Related: panic_guard.go -- serveAdvertisements, the boundary under test

package vrrp

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/taskguard"
	"github.com/ze-software/ze/internal/plugins/vrrp/fsm"
	"github.com/ze-software/ze/internal/plugins/vrrp/packet"
	"github.com/ze-software/ze/internal/plugins/vrrp/transport"
	"github.com/ze-software/ze/internal/test/sim"
)

// The two virtual routers of the isolation test: the defect hits the first.
const (
	vridDefect  = 10
	vridHealthy = 20
)

// defectRecorder wraps the fake deps' receive-error and state seams. Its
// recordRxError panics once, for the defect VRID, the first time a discarded
// advertisement is recorded: onPacket calls it under the instance's mu, inside
// the advertisement task, so the panic fires there with that advertisement
// being dispatched. Every other call is recorded by VRID.
type defectRecorder struct {
	armed atomic.Bool

	mu          sync.Mutex
	rxErrors    []uint8
	transitions map[uint8][]string
}

func (d *defectRecorder) recordRxError(key transport.InstanceKey, _ string) {
	if key.VRID == vridDefect && d.armed.CompareAndSwap(true, false) {
		panic("boom")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.rxErrors = append(d.rxErrors, key.VRID)
}

func (d *defectRecorder) emitState(spec GroupSpec, from, to fsm.State, _ string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.transitions[spec.VRID] = append(d.transitions[spec.VRID], from.String()+"->"+to.String())
}

func (d *defectRecorder) snapshot() (rxErrors []uint8, transitions map[uint8][]string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	transitions = map[uint8][]string{}
	for vrid, list := range d.transitions {
		transitions[vrid] = append([]string(nil), list...)
	}
	return append([]uint8(nil), d.rxErrors...), transitions
}

// startedKey waits until the instance for vrid has run its readiness-driven
// Startup, and answers its transport key.
func startedKey(t *testing.T, eng *engine, vrid uint8) transport.InstanceKey {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		eng.mu.Lock()
		var target *instance
		for _, in := range eng.instances {
			if in.key.VRID == vrid {
				target = in
			}
		}
		eng.mu.Unlock()
		if target != nil {
			target.mu.Lock()
			started, key := target.started, target.key
			target.mu.Unlock()
			if started {
				return key
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("vrid %d never started", vrid)
	return transport.InstanceKey{}
}

// malformed is an advertisement every instance discards (one byte, short of
// the fixed fields), so its only effect is the receive-error record.
func malformed(key transport.InstanceKey) transport.RxItem {
	return transport.RxItem{
		Key:     key,
		Meta:    packet.RxMeta{TTL: 255, Family: packet.V4, Src: netip.MustParseAddr("192.0.2.9"), Dst: packet.MulticastV4},
		Payload: []byte{0x01},
	}
}

// TestVRRPReadPanicContained proves the advertisement dispatcher contains a Ze
// defect met while it handles one virtual router's advertisement: the process
// stays up, the dispatcher keeps delivering, that virtual router restarts
// through Shutdown and Startup into Backup, the other is untouched, and the
// panic is logged, counted and crash-recorded.
// METHOD: an engine over the fake platform runs two groups, VRID 10 and 20, on
// one interface. The receive-error seam panics once, for VRID 10. The
// dispatcher (rxLoop) gets a malformed advertisement for 10, which panics, then
// a malformed one for 20 and another for 10: both must be recorded, which
// proves the dispatcher carried on after the panic and after the restart.
// PREVENTS: a peer-driven panic in onPacket killing the daemon, or ending the
// one dispatcher every virtual router depends on.
func TestVRRPReadPanicContained(t *testing.T) {
	restoreCrashDir := crashlog.SetCrashDirForTest(t.TempDir())
	prior := advertGuard
	var logs bytes.Buffer
	advertGuard = taskguard.New("vrrp", slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() {
		advertGuard = prior
		restoreCrashDir()
	})
	guard := advertGuard

	rec := &defectRecorder{transitions: map[uint8][]string{}}
	rec.armed.Store(true)
	deps := (&fakeDeps{}).deps()
	deps.recordRxError = rec.recordRxError
	deps.emitState = rec.emitState
	eng := newEngine(sim.NewFakeClock(time.Unix(0, 0).UTC()), newFakePlatform().platform(), deps)
	t.Cleanup(eng.stopAll)

	defect, healthy := testSpec(), testSpec()
	defect.VRID, defect.Name = vridDefect, "defect"
	healthy.VRID, healthy.Name = vridHealthy, "healthy"
	healthy.VIPs = []netip.Addr{netip.MustParseAddr("192.0.2.2")}
	eng.apply([]GroupSpec{defect, healthy})
	defectKey := startedKey(t, eng, vridDefect)
	healthyKey := startedKey(t, eng, vridHealthy)

	items := make(chan transport.RxItem)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		rxLoop(eng, items)
	}()
	for _, item := range []transport.RxItem{malformed(defectKey), malformed(healthyKey), malformed(defectKey)} {
		select {
		case items <- item:
		case <-time.After(5 * time.Second):
			t.Fatal("the advertisement dispatcher stopped taking advertisements")
		}
	}
	close(items)
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("the advertisement dispatcher did not return when its channel closed")
	}

	rxErrors, transitions := rec.snapshot()
	if want := []uint8{vridHealthy, vridDefect}; !slices.Equal(rxErrors, want) {
		t.Errorf("recorded receive errors for VRIDs %v, want %v: the dispatcher did not carry on", rxErrors, want)
	}
	wantDefect := "Initialize->Backup Backup->Initialize Initialize->Backup"
	if got := strings.Join(transitions[vridDefect], " "); got != wantDefect {
		t.Errorf("vrid %d transitions %q, want %q (restart into Backup)", vridDefect, got, wantDefect)
	}
	if got := strings.Join(transitions[vridHealthy], " "); got != "Initialize->Backup" {
		t.Errorf("vrid %d transitions %q, want only its own Startup: the defect reached it", vridHealthy, got)
	}

	if count, declared := guard.Panics(taskAdvertisement); count != 1 || !declared {
		t.Errorf("Panics(advertisement) = %d, %v, want 1, true", count, declared)
	}
	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("got %d log lines, want 1: %q", len(lines), logs.String())
	}
	var line map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &line); err != nil {
		t.Fatalf("log line is not JSON: %v: %q", err, lines[0])
	}
	want := map[string]string{
		"level": "ERROR", "component": "vrrp", "task": taskAdvertisement, "panic": "boom",
		"unit": "vrid 10 ipv4 on eth0, advertisement from 192.0.2.9",
	}
	for key, value := range want {
		if line[key] != value {
			t.Errorf("log field %s = %v, want %q", key, line[key], value)
		}
	}
	crashes := crashlog.ListCrashes()
	if len(crashes) != 1 {
		t.Fatalf("ListCrashes returned %d records, want 1", len(crashes))
	}
	if crashes[0].Kind != crashlog.KindRecovered {
		t.Errorf("crash record kind %q, want %q", crashes[0].Kind, crashlog.KindRecovered)
	}
}
