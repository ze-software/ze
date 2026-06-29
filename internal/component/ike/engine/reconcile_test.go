package engine

import (
	"log/slog"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/ike/ipsec"
)

func testIPsecConfig(peers ...ipsec.SiteToSitePeer) *ipsec.IPsecConfig {
	cfg := &ipsec.IPsecConfig{
		ESPGroups: map[string]ipsec.ESPGroup{
			"test-esp": testESPGroup(),
		},
		IKEGroups: map[string]ipsec.IKEGroup{
			"test-ike": testIKEGroup(),
		},
		Peers: make(map[string]ipsec.SiteToSitePeer),
	}
	for i := range peers {
		cfg.Peers[peers[i].Name] = peers[i]
	}
	return cfg
}

func TestReconcilePeersAdded(t *testing.T) {
	active := make(map[string]*PeerSession)
	table := NewSATable()
	log := slog.Default()

	peer := testPeer()
	cfg := testIPsecConfig(peer)

	reconcilePeers(cfg, nil, active, table, nil, nil, log)

	if len(active) != 1 {
		t.Fatalf("expected 1 active peer, got %d", len(active))
	}
	if _, ok := active["test-peer"]; !ok {
		t.Fatal("expected test-peer in active map")
	}

	// Cleanup.
	for _, ps := range active {
		ps.Stop()
	}
}

func TestReconcilePeersRemoved(t *testing.T) {
	active := make(map[string]*PeerSession)
	table := NewSATable()
	log := slog.Default()

	peer := testPeer()
	cfg := testIPsecConfig(peer)
	reconcilePeers(cfg, nil, active, table, nil, nil, log)

	if len(active) != 1 {
		t.Fatalf("setup: expected 1 active peer, got %d", len(active))
	}

	emptyCfg := testIPsecConfig()
	reconcilePeers(emptyCfg, cfg, active, table, nil, nil, log)

	if len(active) != 0 {
		t.Fatalf("expected 0 active peers after removal, got %d", len(active))
	}
}

func TestReconcilePeersChanged(t *testing.T) {
	active := make(map[string]*PeerSession)
	table := NewSATable()
	log := slog.Default()

	peer := testPeer()
	cfg := testIPsecConfig(peer)
	reconcilePeers(cfg, nil, active, table, nil, nil, log)

	oldPS := active["test-peer"]

	changedPeer := peer
	changedPeer.RemoteAddress = "198.51.100.1"
	newCfg := testIPsecConfig(changedPeer)
	reconcilePeers(newCfg, cfg, active, table, nil, nil, log)

	if len(active) != 1 {
		t.Fatalf("expected 1 active peer, got %d", len(active))
	}
	newPS := active["test-peer"]
	if newPS == oldPS {
		t.Fatal("expected new PeerSession after config change")
	}

	// Cleanup.
	for _, ps := range active {
		ps.Stop()
	}
}

func TestReconcilePeersUnchanged(t *testing.T) {
	active := make(map[string]*PeerSession)
	table := NewSATable()
	log := slog.Default()

	peer := testPeer()
	cfg := testIPsecConfig(peer)
	reconcilePeers(cfg, nil, active, table, nil, nil, log)

	oldPS := active["test-peer"]

	reconcilePeers(cfg, cfg, active, table, nil, nil, log)

	if active["test-peer"] != oldPS {
		t.Fatal("unchanged peer should keep the same PeerSession")
	}

	// Cleanup.
	for _, ps := range active {
		ps.Stop()
	}
}
