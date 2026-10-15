// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config

package watchdog

import (
	"encoding/json"
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestConfigTxUnchangedSubtreeIsNoop proves AC-5 for bgp-watchdog: a reload
// that edits bgp outside the watchdog pools sends no route command and keeps
// every entry announced, and the rollback of that transaction does the same.
// Method: a command recorder sees every route command; pool dnsr is announced
// to the peer, then a verify, apply and rollback run over a candidate that
// differs from the running config only in another peer's description.
// VALIDATES: spec-plugin-reload-applies-or-refuses AC-5 for bgp-watchdog.
// PREVENTS: a shared-root edit re-announcing or withdrawing the pools' routes.
// MUTATION: make carryEntryState send the announce of every kept announced
// entry (drop its AnnounceCmd comparison) and the apply check fails.
func TestConfigTxUnchangedSubtreeIsNoop(t *testing.T) {
	prefixes := []string{"10.0.0.0/24", "10.0.1.0/24"}
	recorder := &commandRecorder{}
	srv := newWatchdogServer(recorder.send, nil)
	tx := &watchdogConfigTx{srv: srv}
	if err := tx.configure(withOtherPeer(t, poolSection("", prefixes...), "first")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	srv.mu.Lock()
	srv.peerUp[reloadPeer] = true
	srv.mu.Unlock()
	if _, _, err := srv.handleCommand("request bgp watchdog announce", []string{"dnsr"}, reloadPeer); err != nil {
		t.Fatalf("announce: %v", err)
	}
	if got := len(recorder.take()); got != len(prefixes) {
		t.Fatalf("announce sent %d commands, want %d", got, len(prefixes))
	}

	if err := tx.verify(withOtherPeer(t, poolSection("", prefixes...), "second")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := tx.apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if sent := recorder.take(); len(sent) != 0 {
		t.Fatalf("an edit outside the watchdog pools sent %q, want nothing", sent)
	}
	if got := announcedPrefixes(srv); len(got) != len(prefixes) {
		t.Fatalf("after an edit outside the pools announced %v, want both prefixes", got)
	}
	if err := tx.rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if sent := recorder.take(); len(sent) != 0 {
		t.Fatalf("the rollback of an edit outside the pools sent %q, want nothing", sent)
	}
	if got := announcedPrefixes(srv); len(got) != len(prefixes) {
		t.Fatalf("after the rollback announced %v, want both prefixes", got)
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
