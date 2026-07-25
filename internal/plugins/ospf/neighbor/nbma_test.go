// VALIDATES: shouldAdj is true for point-to-multipoint (every neighbor adjacent) and
// DR/BDR-gated for NBMA; a PtMP neighbor reaches Full without a DR while an NBMA
// DROther pair stays at 2-Way. The predicate is address-family-neutral, so the v3
// variants exercise the same code path.
// PREVENTS: the shouldAdj default branch leaving PtMP/NBMA neighbors stuck at 2-Way.
package neighbor

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestOSPFShouldAdjPtMP(t *testing.T) {
	tbl, cfg := testTable(t, NetworkPointToMultipoint)
	peer := rid(t, "10.0.0.2")
	if reason := tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0))); reason != "" {
		t.Fatalf("Hello: %s", reason)
	}
	// Point-to-multipoint forms an adjacency with every neighbor, no DR gating.
	snap, _ := tbl.Lookup(cfg.Name, peer)
	if snap.State != stateNameExStart {
		t.Fatalf("PtMP peer state = %s, want exstart", snap.State)
	}
}

func TestOSPFv3ShouldAdjPtMP(t *testing.T) {
	// The predicate keys on NetworkType only, so the OSPFv3 family shares it.
	TestOSPFShouldAdjPtMP(t)
}

func TestOSPFShouldAdjNBMA(t *testing.T) {
	tbl, cfg := testTable(t, NetworkNBMA)
	peer := rid(t, "10.0.0.2")
	if reason := tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0))); reason != "" {
		t.Fatalf("Hello: %s", reason)
	}
	// NBMA is DR/BDR-gated exactly like broadcast: a DROther pair stays at 2-Way.
	snap, _ := tbl.Lookup(cfg.Name, peer)
	if snap.State != stateNameTwoWay {
		t.Fatalf("NBMA DROther peer state = %s, want 2-way", snap.State)
	}
	// Once this router is DR, the adjacency advances.
	tbl.AdjOK(cfg.Name, cfg.RouterID, types.RouterID{})
	snap, _ = tbl.Lookup(cfg.Name, peer)
	if snap.State != stateNameExStart {
		t.Fatalf("NBMA peer state after local DR = %s, want exstart", snap.State)
	}
}

func TestOSPFv3ShouldAdjNBMA(t *testing.T) {
	TestOSPFShouldAdjNBMA(t)
}

func TestOSPFPtMPAdjacency(t *testing.T) {
	tbl, cfg := testTable(t, NetworkPointToMultipoint)
	peer := rid(t, "10.0.0.2")
	if reason := tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0))); reason != "" {
		t.Fatalf("Hello: %s", reason)
	}
	driveFull(t, tbl, cfg, peer)
	snap, _ := tbl.Lookup(cfg.Name, peer)
	if snap.State != stateNameFull {
		t.Fatalf("PtMP adjacency state = %s, want full", snap.State)
	}
}

func TestOSPFv3PtMPAdjacency(t *testing.T) {
	TestOSPFPtMPAdjacency(t)
}

func TestOSPFNBMAAdjacency(t *testing.T) {
	tbl, cfg := testTable(t, NetworkNBMA)
	peer := rid(t, "10.0.0.2")
	if reason := tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0))); reason != "" {
		t.Fatalf("Hello: %s", reason)
	}
	// Without a DR the NBMA neighbor stays at 2-Way.
	snap, _ := tbl.Lookup(cfg.Name, peer)
	if snap.State != stateNameTwoWay {
		t.Fatalf("NBMA state without DR = %s, want 2-way", snap.State)
	}
	// Elect this router DR and drive the adjacency to Full.
	tbl.AdjOK(cfg.Name, cfg.RouterID, types.RouterID{})
	driveFull(t, tbl, cfg, peer)
	snap, _ = tbl.Lookup(cfg.Name, peer)
	if snap.State != stateNameFull {
		t.Fatalf("NBMA adjacency state = %s, want full", snap.State)
	}
}

func TestOSPFv3NBMAAdjacency(t *testing.T) {
	TestOSPFNBMAAdjacency(t)
}
