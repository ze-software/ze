// Design: docs/architecture/bgp/healthcheck-plugin.md -- disable is a config reload
package healthcheck

import (
	"context"
	"encoding/json"
	"slices"
	"sync"
	"testing"
	"time"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	cmdAnnounce = "request bgp watchdog announce"
	cmdWithdraw = "request bgp watchdog withdraw"
)

// heldDispatch records every dispatch at entry and holds the first withdraw
// until release is closed (or the dispatch context ends), standing in for a
// watchdog round trip that has not completed.
type heldDispatch struct {
	mu       sync.Mutex
	calls    []dispatchCall
	holding  bool
	entered  chan struct{} // closed when the held withdraw arrives
	released chan struct{}
}

func newHeldDispatch() *heldDispatch {
	return &heldDispatch{entered: make(chan struct{}), released: make(chan struct{})}
}

func (h *heldDispatch) dispatch(ctx context.Context, command string, args []string, peer string) (string, json.RawMessage, error) {
	h.mu.Lock()
	h.calls = append(h.calls, dispatchCall{command: command, args: slices.Clone(args), peer: peer})
	hold := command == cmdWithdraw && !h.holding
	if hold {
		h.holding = true
	}
	h.mu.Unlock()
	if hold {
		close(h.entered)
		select {
		case <-h.released:
		case <-ctx.Done():
		}
	}
	return statusDone, nil, nil
}

// snapshot returns the recorded calls as "command args..." lines.
func (h *heldDispatch) snapshot() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	lines := make([]string, 0, len(h.calls))
	for _, c := range h.calls {
		line := c.command
		for _, a := range c.args {
			line += " " + a
		}
		lines = append(lines, line)
	}
	return lines
}

// newHeldManager returns a ready manager whose dispatches go to h and whose
// IP changes go to ips.
func newHeldManager(h *heldDispatch, ips *mockIPManager) *probeManager {
	mgr := newTestManager()
	mgr.dispatchFn = h.dispatch
	mgr.ipMgr = ips
	mgr.internal = true // ip-setup is accepted only in goroutine mode
	return mgr
}

// returnsWithin fails the test unless fn returns within 2 s.
func returnsWithin(t *testing.T, what string, fn func()) {
	t.Helper()
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		fn()
	}()
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatalf("%s did not return while the old probe's withdraw was held", what)
	}
}

// stopAll removes every probe and waits for each goroutine, current and
// exiting, to return, so a test leaves nothing running.
func stopAll(t *testing.T, mgr *probeManager, h *heldDispatch) {
	t.Helper()
	h.mu.Lock()
	if h.holding {
		select {
		case <-h.released:
		default:
			close(h.released)
		}
	}
	h.mu.Unlock()
	mgr.mu.Lock()
	all := slices.Clone(mgr.exiting)
	for _, rp := range mgr.probes {
		all = append(all, rp)
	}
	mgr.mu.Unlock()
	mgr.applyConfig(nil)
	for _, rp := range all {
		<-rp.done
	}
}

func hcSections(t *testing.T, probes map[string]any) []sdk.ConfigSection {
	t.Helper()
	tree := map[string]any{"bgp": map[string]any{}}
	if probes != nil {
		tree["bgp"] = map[string]any{"healthcheck": map[string]any{"probe": probes}}
	}
	return []sdk.ConfigSection{{Root: configRootBGP, Data: mustMarshalStr(t, tree)}}
}

// hcProbe is one always-up probe config in the group "hc-dns", announcing upMetric.
func hcProbe(upMetric int) map[string]any {
	return map[string]any{"command": "true", "group": "hc-dns", "interval": 1, "rise": 1, "fall": 1, "debounce": true, "up-metric": upMetric}
}

// TestHealthcheckApplyDoesNotWaitForProbeExit proves AC-16 and R-8: a reload
// whose apply removes an announced probe returns while that probe's exit
// withdraw is still held by the watchdog round trip, and the withdraw then
// lands. Method: the sdk transaction over a held dispatch stub.
// VALIDATES: AC-16, apply returns without waiting on a stopped probe's exit.
// PREVENTS: a reload stalling up to 5 s per stopped probe on a watchdog round trip.
func TestHealthcheckApplyDoesNotWaitForProbeExit(t *testing.T) {
	h := newHeldDispatch()
	mgr := newHeldManager(h, &mockIPManager{})
	defer stopAll(t, mgr, h)
	tx := newHealthcheckConfigTx(mgr)

	if err := tx.Configure(hcSections(t, map[string]any{"dns": hcProbe(100)})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	waitFor(t, "the first announce", func() bool { return len(h.snapshot()) == 1 })

	if err := tx.Verify(hcSections(t, nil)); err != nil {
		t.Fatalf("verify: %v", err)
	}
	returnsWithin(t, "apply", func() {
		if err := tx.Apply(); err != nil {
			t.Errorf("apply: %v", err)
		}
	})
	<-h.entered
	close(h.released)
	waitFor(t, "the exit withdraw", func() bool { return len(h.snapshot()) == 2 })
	want := []string{cmdAnnounce + " hc-dns med 100", cmdWithdraw + " hc-dns"}
	if got := h.snapshot(); !slices.Equal(got, want) {
		t.Fatalf("dispatches = %q, want %q", got, want)
	}
}

// TestHealthcheckChangedProbeWaitsForOldExit proves AC-16b: the probe that
// replaces a canceled one (same name, or another name on the same group, or
// the previous probe a rollback restarts) dispatches nothing while the old
// probe's withdraw is held, and announces after it. Method: a held dispatch
// stub; the recorded order is announce, withdraw, then the new announce. The
// rollback row also proves a probe canceled while it waits does nothing:
// its up-metric 200 announce and its IP add never appear. Run with -count to
// give waitReady's random select its chances.
// VALIDATES: AC-16b, a replacing probe dispatches only after its predecessor's exit.
// PREVENTS: a late withdraw landing after the new probe's announce.
func TestHealthcheckChangedProbeWaitsForOldExit(t *testing.T) {
	tests := []struct {
		name     string
		next     map[string]any
		rollback bool
		want     []string
	}{
		{
			name: "same name",
			next: map[string]any{"dns": hcProbe(200)},
			want: []string{cmdAnnounce + " hc-dns med 100", cmdWithdraw + " hc-dns", cmdAnnounce + " hc-dns med 200"},
		},
		{
			name: "other name same group",
			next: map[string]any{"web": hcProbe(200)},
			want: []string{cmdAnnounce + " hc-dns med 100", cmdWithdraw + " hc-dns", cmdAnnounce + " hc-dns med 200"},
		},
		{
			name:     "rollback restarts the previous probe after the canceled waiter",
			next:     map[string]any{"dns": withIP(hcProbe(200), "192.0.2.2/32")},
			rollback: true,
			want:     []string{cmdAnnounce + " hc-dns med 100", cmdWithdraw + " hc-dns", cmdAnnounce + " hc-dns med 100"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHeldDispatch()
			ips := &mockIPManager{}
			mgr := newHeldManager(h, ips)
			defer stopAll(t, mgr, h)
			tx := newHealthcheckConfigTx(mgr)

			first := map[string]any{"dns": withIP(hcProbe(100), "192.0.2.1/32")}
			if err := tx.Configure(hcSections(t, first)); err != nil {
				t.Fatalf("configure: %v", err)
			}
			waitFor(t, "the first announce", func() bool { return len(h.snapshot()) == 1 })

			if err := tx.Verify(hcSections(t, tt.next)); err != nil {
				t.Fatalf("verify: %v", err)
			}
			returnsWithin(t, "apply", func() {
				if err := tx.Apply(); err != nil {
					t.Errorf("apply: %v", err)
				}
			})
			<-h.entered
			if tt.rollback {
				returnsWithin(t, "rollback", func() {
					if err := tx.Rollback(); err != nil {
						t.Errorf("rollback: %v", err)
					}
				})
			}
			// A new probe that did not wait would announce at once (rise 1).
			time.Sleep(200 * time.Millisecond)
			if got := h.snapshot(); len(got) != 2 {
				t.Fatalf("dispatches while the withdraw is held = %q, want the announce and the withdraw only", got)
			}
			close(h.released)
			waitFor(t, "the new announce", func() bool { return len(h.snapshot()) >= 3 })
			// Leave a canceled waiter time to misbehave before judging.
			time.Sleep(100 * time.Millisecond)
			if got := h.snapshot(); !slices.Equal(got, tt.want) {
				t.Fatalf("dispatches = %q, want %q", got, tt.want)
			}
			if tt.rollback {
				ips.mu.Lock()
				added := slices.Clone(ips.added)
				ips.mu.Unlock()
				if slices.Contains(added, "lo:192.0.2.2/32") {
					t.Fatalf("the canceled waiter added its IP: %q", added)
				}
			}
		})
	}
}

func withIP(probe map[string]any, cidr string) map[string]any {
	probe["ip-setup"] = map[string]any{"interface": "lo", "ip": []string{cidr}}
	return probe
}
