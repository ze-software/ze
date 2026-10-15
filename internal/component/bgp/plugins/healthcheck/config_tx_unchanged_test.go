// Design: docs/architecture/bgp/healthcheck-plugin.md -- disable is a config reload

package healthcheck

import (
	"encoding/json"
	"slices"
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// runningProbeNamed returns the probe running under name, nil when none.
func runningProbeNamed(mgr *probeManager, name string) *runningProbe {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	return mgr.probes[name]
}

// TestConfigTxUnchangedSubtreeIsNoop proves AC-5 for bgp-healthcheck: a
// reload that edits bgp outside the healthcheck probes restarts no probe and
// dispatches nothing, and the rollback of that transaction does neither.
// Method: a dispatch stub, released from the start, records every watchdog
// command; the running probe is compared across a verify, apply and
// rollback whose candidate differs from the running config only in another
// peer's description.
// VALIDATES: spec-plugin-reload-applies-or-refuses AC-5 for bgp-healthcheck.
// PREVENTS: a shared-root edit restarting a probe, which withdraws its route
// and walks the FSM again from INIT.
// MUTATION: delete the reflect.DeepEqual short-circuit in sdk.ConfigTx.Apply
// AND make applyConfig treat every probe as changed (newCfg.equal) and the
// apply check fails; either guard alone keeps the probe running.
func TestConfigTxUnchangedSubtreeIsNoop(t *testing.T) {
	h := newHeldDispatch()
	close(h.released) // Nothing here is held: the exit withdraw of stopAll returns at once.
	mgr := newHeldManager(h, &mockIPManager{})
	defer stopAll(t, mgr, h)
	tx := newHealthcheckConfigTx(mgr)
	probes := map[string]any{"dns": hcProbe("hc-dns", 100)}

	if err := tx.Configure(withOtherPeer(t, hcSections(t, probes), "first")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	waitFor(t, "the first announce", func() bool { return len(h.snapshot()) == 1 })
	running := runningProbeNamed(mgr, "dns")
	if running == nil {
		t.Fatal("configure started no probe dns")
	}

	if err := tx.Verify(withOtherPeer(t, hcSections(t, probes), "second")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if runningProbeNamed(mgr, "dns") != running {
		t.Fatal("an edit outside the healthcheck probes restarted probe dns")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if runningProbeNamed(mgr, "dns") != running {
		t.Fatal("the rollback of an edit outside the healthcheck probes restarted probe dns")
	}
	want := []string{cmdAnnounce + " hc-dns med 100"}
	if got := h.snapshot(); !slices.Equal(got, want) {
		t.Fatalf("dispatches = %q, want only the first announce %q", got, want)
	}
}

// withOtherPeer returns sections with peer "other" added to the bgp subtree:
// a peer no filter names, carrying description, so two calls that differ only
// in description differ only outside the plugin's subtree.
func withOtherPeer(t *testing.T, sections []sdk.ConfigSection, description string) []sdk.ConfigSection {
	t.Helper()
	var tree map[string]any
	if err := json.Unmarshal([]byte(sections[0].Data), &tree); err != nil {
		t.Fatalf("section data: %v", err)
	}
	bgpCfg, ok := tree["bgp"].(map[string]any)
	if !ok {
		t.Fatal("section data holds no bgp object")
	}
	peers, ok := bgpCfg["peer"].(map[string]any)
	if !ok {
		peers = map[string]any{}
		bgpCfg["peer"] = peers
	}
	peers["other"] = map[string]any{
		"connection":  map[string]any{"remote": map[string]any{"ip": "192.0.2.99"}},
		"description": description,
	}
	data, err := json.Marshal(tree)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return []sdk.ConfigSection{{Root: sections[0].Root, Data: string(data)}}
}
