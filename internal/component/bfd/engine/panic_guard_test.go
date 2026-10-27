package engine

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/packet"
	"github.com/ze-software/ze/internal/component/bfd/transport"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/taskguard"
)

// The two VRFs the isolation test runs one session each in, over one
// loopback pair: the defect hits the first one's session only.
const (
	vrfDefect  = "red"
	vrfHealthy = "blue"
)

// upPanicHook is a MetricsHook, the engine's existing observer seam, whose
// OnStateChange panics once: on the first transition to Up of a session in vrf.
// The engine calls it from the transition callback, inside the express loop
// while it processes the peer's packet, so the panic fires in the express task
// with that session selected.
type upPanicHook struct {
	vrf   string
	armed atomic.Bool
}

func (h *upPanicHook) OnStateChange(_, to packet.State, _ packet.Diag, _, vrf string) {
	if to != packet.StateUp {
		return
	}
	if vrf != h.vrf {
		return
	}
	if h.armed.CompareAndSwap(true, false) {
		panic("boom")
	}
}
func (*upPanicHook) OnTxPacket(string)               {}
func (*upPanicHook) OnRxPacket(string)               {}
func (*upPanicHook) OnAuthFailure(string)            {}
func (*upPanicHook) OnEchoTx(string)                 {}
func (*upPanicHook) OnEchoRx(string)                 {}
func (*upPanicHook) OnEchoRTT(string, time.Duration) {}

// TestBFDReadPanicContained proves the BFD express loop contains a Ze defect
// met while it processes one session's packet: the process stays up, that
// session goes Down with diagnostic 0 and comes back Up through the handshake,
// the other session on the same loop never leaves Up, and the panic is logged,
// counted and crash-recorded.
// METHOD: two Loops over one loopback pair, two sessions each (vrf red and vrf
// blue). Loop A's MetricsHook panics on red's first transition to Up, which
// happens inside handleInbound for the peer's packet. Subscribers watch both of
// A's sessions.
// PREVENTS: a peer-driven panic in the express loop killing the daemon, or
// ending the loop and with it every BFD session it runs.
func TestBFDReadPanicContained(t *testing.T) {
	restoreCrashDir := crashlog.SetCrashDirForTest(t.TempDir())
	prior := expressGuard
	var logs bytes.Buffer
	expressGuard = taskguard.New("bfd", slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() {
		expressGuard = prior
		restoreCrashDir()
	})
	guard := expressGuard

	lbA, lbB := transport.Pair(api.SingleHop, netip.MustParseAddr(addrA), netip.MustParseAddr(addrB))
	loopA := NewLoop(lbA, clock.RealClock{})
	loopB := NewLoop(lbB, clock.RealClock{})
	hook := &upPanicHook{vrf: vrfDefect}
	hook.armed.Store(true)
	loopA.SetMetricsHook(hook)
	for _, l := range []*Loop{loopA, loopB} {
		if err := l.Start(); err != nil {
			t.Fatalf("Start: %v", err)
		}
		t.Cleanup(func() {
			if err := l.Stop(); err != nil {
				t.Errorf("Stop: %v", err)
			}
		})
	}

	subs := map[string]<-chan api.StateChange{}
	for _, vrf := range []string{vrfDefect, vrfHealthy} {
		reqA := reqFor(addrB, addrA)
		reqA.VRF = vrf
		hA, err := loopA.EnsureSession(reqA)
		if err != nil {
			t.Fatalf("loopA.EnsureSession(%s): %v", vrf, err)
		}
		reqB := reqFor(addrA, addrB)
		reqB.VRF = vrf
		if _, err := loopB.EnsureSession(reqB); err != nil {
			t.Fatalf("loopB.EnsureSession(%s): %v", vrf, err)
		}
		sub := hA.Subscribe()
		t.Cleanup(func() { hA.Unsubscribe(sub) })
		subs[vrf] = sub
	}

	// Red: Down with diagnostic 0 once the guard counted the panic (the
	// subscription's first event is the initial Down, before any packet), then
	// Up again (the handshake rebuilt it). Blue: Up, and never Down after.
	var redDefectDown, redBackUp, blueUp bool
	deadline := time.After(15 * time.Second)
	for !redBackUp || !blueUp {
		select {
		case change := <-subs[vrfDefect]:
			contained, _ := guard.Panics(taskExpress)
			if change.State == packet.StateDown && contained == 1 {
				if change.Diag != packet.DiagNone {
					t.Fatalf("red went Down with diag %v, want No Diagnostic (0)", change.Diag)
				}
				redDefectDown = true
			}
			if change.State == packet.StateUp && redDefectDown {
				redBackUp = true
			}
		case change := <-subs[vrfHealthy]:
			if change.State == packet.StateDown && blueUp {
				t.Fatalf("blue went Down (diag %v): the defect in red's session reached it", change.Diag)
			}
			if change.State == packet.StateUp {
				blueUp = true
			}
		case <-deadline:
			t.Fatalf("red Down by defect %v, red back Up %v, blue Up %v: the express loop did not carry on",
				redDefectDown, redBackUp, blueUp)
		}
	}

	if count, declared := guard.Panics(taskExpress); count != 1 || !declared {
		t.Errorf("Panics(express) = %d, %v, want 1, true", count, declared)
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
		"level": "ERROR", "component": "bfd", "task": taskExpress, "panic": "boom",
		"message-type": workControl,
		"unit":         "session to " + addrB + " (single-hop, vrf red, local " + addrA + ", interface loop)",
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
