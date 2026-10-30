package bgpconfig

import (
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/configorder"
)

const restoreActiveConfig = `bgp {
	router-id 10.0.0.1;
	session { asn { local 65000; } }
	peer accepted {
		connection {
			remote { ip 192.0.2.1; }
			local { ip 192.168.1.1; }
		}
		session { asn { remote 65001; } }
	}
}`

const restoreCandidateConfig = `bgp {
	router-id 10.0.0.1;
	session { asn { local 65000; } }
	peer rejected {
		connection {
			remote { ip 192.0.2.2; }
			local { ip 192.168.1.1; }
		}
		session { asn { remote 65002; } }
	}
}`

// TestApplyConfigDiffAppliesTheTreeNotTheStagedCandidate proves that applying
// a diff applies the tree it is handed, on the reactor CreateReactor builds for
// a file-configured daemon.
//
// An outer compensation (restoreReload, plugin/server/reload_compensation.go)
// hands ApplyConfigDiff the prior tree while the hub still holds the rejected
// candidate staged: the hub clears it only after rollbackReload. A loader that
// re-reads the file answers that candidate (storage.ReadReloadConfig prefers
// it), so the restore re-applied the very config it was undoing.
//
// Method: boot the reactor from the active config, stage a candidate that
// replaces its peer, then hand ApplyConfigDiff the active tree, as the restore
// does. The running peer must be the active one.
//
// VALIDATES: ApplyConfigDiff(before) leaves the peers of before running.
// PREVENTS: an outer compensation re-applying the rejected candidate.
func TestApplyConfigDiffAppliesTheTreeNotTheStagedCandidate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "router.conf")
	store := newReloadFileStore(t, path)
	if _, _, err := storage.EnsureActiveVersion(store, path, []byte(restoreActiveConfig), time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(restoreActiveConfig), 0o600); err != nil {
		t.Fatal(err)
	}

	loaded, err := config.LoadConfig(restoreActiveConfig, path, nil)
	if err != nil {
		t.Fatalf("parse the active config: %v", err)
	}
	before := loaded.Tree.ToPluginMap()
	r, err := CreateReactor(loaded, path, store, true)
	if err != nil {
		t.Fatalf("create reactor: %v", err)
	}
	// Not started: the reconcile adds and removes peers in the reactor's table
	// either way, and a started reactor would listen on the peers' local
	// address, which a test host does not hold.

	if _, err := storage.WriteCandidateVersion(store, path, []byte(restoreCandidateConfig), time.Now()); err != nil {
		t.Fatal(err)
	}

	api, ok := r.ReactorLifecycleAdapter().(interface {
		ApplyConfigDiff(map[string]any) error
	})
	if !ok {
		t.Fatal("the reactor lifecycle adapter does not apply a config diff")
	}
	if err := api.ApplyConfigDiff(before); err != nil {
		t.Fatalf("applying the prior tree failed: %v", err)
	}

	peers := r.Peers()
	if len(peers) != 1 {
		t.Fatalf("running peers = %d, want the 1 peer of the applied tree", len(peers))
	}
	settings := peers[0].Settings()
	want := netip.MustParseAddr("192.0.2.1")
	if settings.Address != want {
		t.Fatalf("running peer = %s AS %d, want %s AS 65001: the staged candidate was applied instead of the tree handed over",
			settings.Address, settings.PeerAS, want)
	}
	if settings.PeerAS != 65001 {
		t.Fatalf("running peer AS = %d, want 65001", settings.PeerAS)
	}
}

// TestRestoreLeavesAnUntouchedCreatedPeerAsItWas proves that an outer
// compensation, which now applies the prior tree through the full pipeline,
// does not swap or restart a peer `create bgp peer` built that the rejected
// reload never touched.
//
// The prior tree (ReloadRunning of the candidate) keeps a created peer's entry
// when the candidate declares its name, and the reconcile then compares the
// pipeline's settings for that entry with the create builder's. Here the
// candidate declares the created peer's name at another address, so the
// forward reload leaves the created peer running as it was and adds a second
// peer; the restore must remove that peer and leave the created one alone.
//
// Method: boot from the active config, create a peer, reload a candidate that
// declares the created peer's name at another address, then undo it the way
// restoreReload does: RestoreCreatedPeers, ApplyConfigDiff and SetConfigTree of
// the prior tree, RestoreCreatedPeers again.
//
// VALIDATES: the created peer is the same *Peer, with the same settings, after
// the restore, and the candidate's peer is gone.
// PREVENTS: a restore that bounces or rewrites a created peer because the
// pipeline and the create builder disagree about one entry.
func TestRestoreLeavesAnUntouchedCreatedPeerAsItWas(t *testing.T) {
	path := filepath.Join(t.TempDir(), "router.conf")
	store := newReloadFileStore(t, path)
	if err := os.WriteFile(path, []byte(restoreActiveConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.LoadConfig(restoreActiveConfig, path, nil)
	if err != nil {
		t.Fatalf("parse the active config: %v", err)
	}
	r, err := CreateReactor(loaded, path, store, true)
	if err != nil {
		t.Fatalf("create reactor: %v", err)
	}
	api, ok := r.ReactorLifecycleAdapter().(interface {
		AddDynamicPeer(netip.Addr, map[string]any) error
		CreatedPeers() map[netip.Addr]string
		GetConfigTree() map[string]any
		ReloadRunning(map[string]any) map[string]any
		ApplyConfigDiff(map[string]any) error
		SetConfigTree(map[string]any)
		RestoreCreatedPeers(map[string]any, map[netip.Addr]string) error
	})
	if !ok {
		t.Fatal("the reactor lifecycle adapter lacks the reload and create surface")
	}
	api.SetConfigTree(loaded.Tree.ToPluginMap())
	// CreateReactor adds no peer until the reactor starts: apply the boot
	// tree, so the configured peer runs as it does in a booted daemon.
	if err := api.ApplyConfigDiff(loaded.Tree.ToPluginMap()); err != nil {
		t.Fatal(err)
	}

	createdAddr := netip.MustParseAddr("192.0.2.7")
	if err := api.AddDynamicPeer(createdAddr, map[string]any{
		"connection": map[string]any{"local": map[string]any{"ip": "192.168.1.1"}},
		"session":    map[string]any{"asn": map[string]any{"remote": "65007"}},
	}); err != nil {
		t.Fatalf("create the peer: %v", err)
	}
	created := api.CreatedPeers()
	name, held := created[createdAddr]
	if !held {
		t.Fatalf("created peers = %v, want %s", created, createdAddr)
	}
	peerAt := func(addr netip.Addr) *reactor.Peer {
		for _, peer := range r.Peers() {
			if peer.Settings().Address == addr {
				return peer
			}
		}
		return nil
	}
	createdPeer := peerAt(createdAddr)
	if createdPeer == nil {
		t.Fatal("the created peer is not running")
	}
	createdSettings := *createdPeer.Settings()

	candidate, err := config.LoadConfig(restoreActiveConfig, path, nil)
	if err != nil {
		t.Fatal(err)
	}
	declaring := candidate.Tree.ToPluginMap()
	bgp, _ := declaring["bgp"].(map[string]any)
	peers, _ := bgp["peer"].(map[string]any)
	peers[name] = map[string]any{
		"connection": map[string]any{
			"remote": map[string]any{"ip": "192.0.2.8"},
			"local":  map[string]any{"ip": "192.168.1.1"},
		},
		"session": map[string]any{"asn": map[string]any{"remote": "65008"}},
	}
	// Two entries carry their order, as Tree.ToPluginMap lowers them.
	bgp[configorder.OrderKey("peer")] = []string{"accepted", name}
	before := api.ReloadRunning(declaring)
	if err := api.ApplyConfigDiff(declaring); err != nil {
		t.Fatalf("forward reload: %v", err)
	}
	api.SetConfigTree(declaring)
	if peerAt(netip.MustParseAddr("192.0.2.8")) == nil {
		t.Fatal("the forward reload did not add the candidate's peer")
	}
	if peerAt(createdAddr) != createdPeer {
		t.Fatal("the forward reload replaced the created peer it does not declare")
	}

	if err := api.RestoreCreatedPeers(before, created); err != nil {
		t.Fatal(err)
	}
	if err := api.ApplyConfigDiff(before); err != nil {
		t.Fatalf("restore: %v", err)
	}
	api.SetConfigTree(before)
	if err := api.RestoreCreatedPeers(before, created); err != nil {
		t.Fatal(err)
	}

	if peerAt(netip.MustParseAddr("192.0.2.8")) != nil {
		t.Fatal("the restore left the rejected candidate's peer running")
	}
	if peerAt(createdAddr) != createdPeer {
		t.Fatal("the restore restarted the created peer")
	}
	if !reflect.DeepEqual(*createdPeer.Settings(), createdSettings) {
		t.Fatalf("the restore rewrote the created peer's settings:\n got  %+v\n want %+v", *createdPeer.Settings(), createdSettings)
	}
	if got := api.CreatedPeers()[createdAddr]; got != name {
		t.Fatalf("created mark after restore = %q, want %q", got, name)
	}
}
