// Design: docs/architecture/rib-transition.md -- watchdog initial announcement state
// Related: config.go -- collectWatchdogRoutes reads the configured boolean
// Related: config_tx.go -- reload retains runtime state for existing routes

package watchdog

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestParseConfigWithdrawValues checks the initial flag itself, because an
// entry's per-peer map is empty until peer-up for both held and announced routes.
func TestParseConfigWithdrawValues(t *testing.T) {
	for _, scope := range []string{"peer", "group"} {
		for _, tc := range []struct {
			name      string
			watchdog  map[string]any
			announced bool
		}{
			{"omitted", map[string]any{"name": "dnsr"}, true},
			{"string-false", map[string]any{"name": "dnsr", "withdraw": "false"}, true},
			{"native-false", map[string]any{"name": "dnsr", "withdraw": false}, true},
			{"string-true", map[string]any{"name": "dnsr", "withdraw": "true"}, false},
			{"native-true", map[string]any{"name": "dnsr", "withdraw": true}, false},
		} {
			t.Run(scope+"/"+tc.name, func(t *testing.T) {
				sections := withdrawValueSection(t, scope, map[string]any{
					"route": withdrawValueUpdate("192.0.2.0/24", tc.watchdog),
				})
				pools, present, err := parseWatchdogSections(sections)
				if err != nil {
					t.Fatalf("parse watchdog section: %v", err)
				}
				if !present {
					t.Fatal("BGP section was not read")
				}
				entry := withdrawValueEntry(t, pools)
				if entry.initiallyAnnounced != tc.announced {
					t.Fatalf("initiallyAnnounced = %v, want %v", entry.initiallyAnnounced, tc.announced)
				}
			})
		}
	}
}

// TestParseConfigWithdrawRejectsInvalid keeps a malformed delivered setting
// distinct from an omitted setting; the parser must never announce by default.
func TestParseConfigWithdrawRejectsInvalid(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value any
	}{
		{"invalid-string", "invalid"},
		{"empty-string", ""},
		{"number", 1},
		{"null", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pools, _, err := parseWatchdogSections(withdrawValueSection(t, "peer", map[string]any{
				"route": withdrawValueUpdate("192.0.2.0/24", map[string]any{
					"name": "dnsr", "withdraw": tc.value,
				}),
			}))
			if err == nil {
				t.Fatalf("accepted malformed withdraw value %v", tc.value)
			}
			if pools != nil {
				t.Fatal("malformed setting returned route pools")
			}
			for _, want := range []string{reloadPeer, "dnsr", "invalid withdraw", "fix: withdraw false"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %q", err, want)
				}
			}
		})
	}
}

// TestConfigureWithdrawRejectsInvalid proves that startup's actual configure
// callback refuses a malformed value before it installs any route pool.
func TestConfigureWithdrawRejectsInvalid(t *testing.T) {
	recorder := &commandRecorder{}
	srv := newWatchdogServer(recorder.send, nil)
	tx := &watchdogConfigTx{srv: srv}
	err := tx.configure(withdrawValueSection(t, "peer", map[string]any{
		"route": withdrawValueUpdate("192.0.2.0/24", map[string]any{
			"name": "dnsr", "withdraw": "invalid",
		}),
	}))
	if err == nil {
		t.Fatal("configure accepted malformed withdraw")
	}
	if len(srv.peerPools) != 0 {
		t.Fatalf("configure installed %d peer pools after refusing the value", len(srv.peerPools))
	}
	if sent := recorder.take(); len(sent) != 0 {
		t.Fatalf("refused configure sent %q", sent)
	}
}

// TestVerifyWithdrawRejectsInvalid proves that failed verification clears an
// older candidate, preserves the running pool, and leaves apply unverified.
func TestVerifyWithdrawRejectsInvalid(t *testing.T) {
	recorder := &commandRecorder{}
	srv := newWatchdogServer(recorder.send, nil)
	tx := &watchdogConfigTx{srv: srv}
	if err := tx.configure(withdrawValueSection(t, "peer", map[string]any{
		"route": withdrawValueUpdate("192.0.2.0/24", map[string]any{"name": "dnsr"}),
	})); err != nil {
		t.Fatalf("configure running route: %v", err)
	}
	srv.handleStateUp(reloadPeer, 0)
	withdrawValueAnnounced(t, recorder.take(), "192.0.2.0/24")
	running := srv.peerPools[reloadPeer]
	if err := tx.verify(withdrawValueSection(t, "peer", map[string]any{
		"candidate": withdrawValueUpdate("198.51.100.0/24", map[string]any{"name": "dnsr"}),
	})); err != nil {
		t.Fatalf("verify older candidate: %v", err)
	}
	if err := tx.verify(withdrawValueSection(t, "peer", map[string]any{
		"invalid": withdrawValueUpdate("203.0.113.0/24", map[string]any{
			"name": "dnsr", "withdraw": "invalid",
		}),
	})); err == nil {
		t.Fatal("verify accepted malformed withdraw")
	}
	if tx.pending != nil {
		t.Fatal("failed verify retained a pending candidate")
	}
	if err := tx.apply(); !errors.Is(err, sdk.ErrConfigApplyUnverified) {
		t.Fatalf("apply after refused verify = %v, want %v", err, sdk.ErrConfigApplyUnverified)
	}
	if srv.peerPools[reloadPeer] != running {
		t.Fatal("failed verify or unverified apply replaced the running pool")
	}
	entry := withdrawValueEntry(t, srv.peerPools)
	if entry.Route.Prefix != "192.0.2.0/24" {
		t.Fatalf("running route = %s, want 192.0.2.0/24", entry.Route.Prefix)
	}
	if !entry.isAnnounced(reloadPeer) {
		t.Fatal("failed verify changed the running route's announced state")
	}
	if sent := recorder.take(); len(sent) != 0 {
		t.Fatalf("failed verify or unverified apply sent %q", sent)
	}
}

// TestParsedWithdrawValuesControlPeerUp drives configure and peer-up, then
// releases a held route through the command handler and observes its send.
func TestParsedWithdrawValuesControlPeerUp(t *testing.T) {
	for _, tc := range []struct {
		name      string
		watchdog  map[string]any
		announced bool
	}{
		{"omitted", map[string]any{"name": "dnsr"}, true},
		{"false", map[string]any{"name": "dnsr", "withdraw": "false"}, true},
		{"true", map[string]any{"name": "dnsr", "withdraw": "true"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := &commandRecorder{}
			srv := newWatchdogServer(recorder.send, nil)
			tx := &watchdogConfigTx{srv: srv}
			if err := tx.configure(withdrawValueSection(t, "peer", map[string]any{
				"route": withdrawValueUpdate("192.0.2.0/24", tc.watchdog),
			})); err != nil {
				t.Fatalf("configure: %v", err)
			}
			srv.handleStateUp(reloadPeer, 0)
			sent := recorder.take()
			if tc.announced {
				withdrawValueAnnounced(t, sent, "192.0.2.0/24")
				return
			}
			if len(sent) != 0 {
				t.Fatalf("held route sent on peer-up: %q", sent)
			}
			status, _, err := srv.handleCommand(commandRequestWatchdogAnnounce, []string{"dnsr"}, reloadPeer)
			if err != nil {
				t.Fatalf("announce held route: %v", err)
			}
			if status != statusDone {
				t.Fatalf("announce status = %q, want %q", status, statusDone)
			}
			withdrawValueAnnounced(t, recorder.take(), "192.0.2.0/24")
		})
	}
}

// TestNewWithdrawValuesOnReload checks that a new false route announces on
// apply, while an existing route is not sent again merely because config reloads.
func TestNewWithdrawValuesOnReload(t *testing.T) {
	recorder := &commandRecorder{}
	srv := newWatchdogServer(recorder.send, nil)
	tx := &watchdogConfigTx{srv: srv}
	kept := withdrawValueUpdate("192.0.2.0/24", map[string]any{"name": "dnsr"})
	if err := tx.configure(withdrawValueSection(t, "peer", map[string]any{"kept": kept})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	srv.handleStateUp(reloadPeer, 0)
	withdrawValueAnnounced(t, recorder.take(), "192.0.2.0/24")
	if err := tx.verify(withdrawValueSection(t, "peer", map[string]any{
		"kept": kept,
		"new": withdrawValueUpdate("198.51.100.0/24", map[string]any{
			"name": "dnsr", "withdraw": "false",
		}),
	})); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := tx.apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	withdrawValueAnnounced(t, recorder.take(), "198.51.100.0/24")
}

// TestReloadWithdrawValueRetainsRuntimeState holds a route by command, changes
// its initial setting to false, and checks that apply preserves the held state.
func TestReloadWithdrawValueRetainsRuntimeState(t *testing.T) {
	recorder := &commandRecorder{}
	srv := newWatchdogServer(recorder.send, nil)
	tx := &watchdogConfigTx{srv: srv}
	if err := tx.configure(withdrawValueSection(t, "peer", map[string]any{
		"route": withdrawValueUpdate("192.0.2.0/24", map[string]any{"name": "dnsr"}),
	})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	srv.handleStateUp(reloadPeer, 0)
	withdrawValueAnnounced(t, recorder.take(), "192.0.2.0/24")
	status, _, err := srv.handleCommand(commandRequestWatchdogWithdraw, []string{"dnsr"}, reloadPeer)
	if err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	if status != statusDone {
		t.Fatalf("withdraw status = %q, want %q", status, statusDone)
	}
	if sent := recorder.take(); len(sent) != 1 {
		t.Fatalf("withdraw sent %q, want one command", sent)
	}
	if err := tx.verify(withdrawValueSection(t, "peer", map[string]any{
		"route": withdrawValueUpdate("192.0.2.0/24", map[string]any{
			"name": "dnsr", "withdraw": "false",
		}),
	})); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := tx.apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if sent := recorder.take(); len(sent) != 0 {
		t.Fatalf("reload changed runtime held state by sending %q", sent)
	}
	entry := withdrawValueEntry(t, srv.peerPools)
	if entry.isAnnounced(reloadPeer) {
		t.Fatal("reload changed runtime held state to announced")
	}
}

func withdrawValueUpdate(prefix string, watchdog map[string]any) map[string]any {
	return map[string]any{
		"attribute": map[string]any{"origin": "igp", "next-hop": "1.2.3.4"},
		"nlri":      map[string]any{"ipv4/unicast": map[string]any{"content": "add " + prefix}},
		"watchdog":  watchdog,
	}
}

func withdrawValueSection(t *testing.T, scope string, updates map[string]any) []sdk.ConfigSection {
	t.Helper()
	peer := map[string]any{
		"connection": map[string]any{"remote": map[string]any{"ip": reloadPeer}},
	}
	peers := map[string]any{"peer1": peer}
	var bgp map[string]any
	if scope == "group" {
		bgp = map[string]any{"group": map[string]any{
			"group1": map[string]any{"peer": peers, "update": updates},
		}}
	} else {
		peer["update"] = updates
		bgp = map[string]any{"peer": peers}
	}
	data, err := json.Marshal(map[string]any{"bgp": bgp})
	if err != nil {
		t.Fatalf("encode watchdog config: %v", err)
	}
	return []sdk.ConfigSection{{Root: configRootBGP, Data: string(data)}}
}

func withdrawValueEntry(t *testing.T, peers map[string]*poolSet) *PoolEntry {
	t.Helper()
	pools := peers[reloadPeer]
	if pools == nil {
		t.Fatal("peer has no watchdog pools")
	}
	pool := pools.getPool("dnsr")
	if pool == nil {
		t.Fatal("peer has no dnsr pool")
	}
	entries := pool.Routes()
	if len(entries) != 1 {
		t.Fatalf("pool has %d entries, want one", len(entries))
	}
	return entries[0]
}

func withdrawValueAnnounced(t *testing.T, sent []string, prefix string) {
	t.Helper()
	if len(sent) != 1 {
		t.Fatalf("sent %q, want one announcement of %s", sent, prefix)
	}
	if !strings.Contains(sent[0], "nlri add "+prefix) {
		t.Fatalf("sent %q, want an announcement of %s", sent[0], prefix)
	}
}
