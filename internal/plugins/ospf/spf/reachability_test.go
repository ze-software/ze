// Design: docs/architecture/ospf/ospf-8-spf-rib.md -- immutable completed SPF views.
package spf

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestReachabilitySnapshotRemainsCoherentAcrossRuns(t *testing.T) {
	area := testArea()
	root, peer := testRID(t, "1.1.1.1"), testRID(t, "2.2.2.2")
	db := testSource(t, area,
		routerLSA(t, "1.1.1.1", p2pLink(t, "2.2.2.2", "10.0.0.1", 10)),
		routerLSA(t, "2.2.2.2", p2pLink(t, "1.1.1.1", "10.0.0.2", 10)),
	)
	computer := NewComputer(Config{Source: db, Root: root, Areas: []types.AreaID{area}})
	t.Cleanup(computer.Stop)
	if computer.Reachability().Ready() {
		t.Fatal("reachability claimed a completed run before SPF")
	}
	computer.Run()
	before := computer.Reachability()
	if !before.Ready() || !before.RouterReachable(area, peer) || !before.RouterReachableAny(peer) {
		t.Fatal("native SPF lost a reached router that advertises no IP prefix")
	}
	if !before.RouterKnown(area, peer) || !before.RouterKnownAny(peer) {
		t.Fatal("completed native input forgot its live router")
	}
	computer.SetRoot(root)
	computer.SetAreas([]types.AreaID{area})
	if !computer.Reachability().Ready() {
		t.Fatal("unchanged configuration invalidated the completed view")
	}
	if !db.Delete(area, types.LSAKey{Type: types.LSTypeRouter, LinkStateID: types.LinkStateID(peer), AdvertisingRouter: peer}) {
		t.Fatal("peer Router-LSA missing")
	}
	computer.Run()
	after := computer.Reachability()
	if !after.Ready() || after.RouterReachable(area, peer) || after.RouterReachableAny(peer) {
		t.Fatal("new completed view retained withdrawn native reachability")
	}
	if after.RouterKnown(area, peer) || after.RouterKnownAny(peer) {
		t.Fatal("withdrawn origin remained in the newer native input")
	}
	if !before.RouterReachable(area, peer) {
		t.Fatal("later SPF mutated an already captured view")
	}
	computer.SetRoot(testRID(t, "3.3.3.3"))
	if computer.Reachability().Ready() {
		t.Fatal("old-root SPF remained ready after Router ID change")
	}
	if !before.RouterReachable(area, peer) {
		t.Fatal("configuration change mutated an already captured view")
	}
}

func TestReachabilityRetainsUnresolvedInterAreaASBRInput(t *testing.T) {
	area := testArea()
	root, asbr := testRID(t, "1.1.1.1"), testRID(t, "4.4.4.4")
	summary := summaryASBRLSA(t, "4.4.4.4", "7.7.7.7", 10)
	db := testSource(t, area,
		routerLSA(t, "1.1.1.1", p2pLink(t, "2.2.2.2", "10.0.0.1", 10)),
		routerLSA(t, "2.2.2.2", p2pLink(t, "1.1.1.1", "10.0.0.2", 10)),
		summary,
	)
	computer := NewComputer(Config{Source: db, Root: root, Areas: []types.AreaID{area}})
	t.Cleanup(computer.Stop)
	computer.Run()
	before := computer.Reachability()
	if !before.RouterKnownAny(asbr) || before.RouterReachableAny(asbr) {
		t.Fatal("unresolved native Type-4 target was not known and unreachable")
	}
	if before.RouterKnown(area, asbr) {
		t.Fatal("inter-area ASBR target leaked into area-local origin membership")
	}
	if !db.Delete(area, summary.Header.Key()) {
		t.Fatal("ASBR summary missing")
	}
	computer.Run()
	if computer.Reachability().RouterKnownAny(asbr) {
		t.Fatal("withdrawn Type-4 target remained in the next native input")
	}
	if !before.RouterKnownAny(asbr) {
		t.Fatal("later calculation mutated the captured known-input set")
	}
}

type heldReachabilityStrategy struct {
	v4Strategy
	first    atomic.Bool
	captured chan struct{}
	release  chan struct{}
}

func (s *heldReachabilityStrategy) BuildGraph(source Source, area types.AreaID) *Graph {
	graph := s.v4Strategy.BuildGraph(source, area)
	if s.first.CompareAndSwap(false, true) {
		close(s.captured)
		<-s.release
	}
	return graph
}

func TestOlderSPFRunCannotReplaceNewerReachability(t *testing.T) {
	area := testArea()
	root, peer := testRID(t, "1.1.1.1"), testRID(t, "2.2.2.2")
	db := baseP2PSource(t, area)
	strategy := &heldReachabilityStrategy{captured: make(chan struct{}), release: make(chan struct{})}
	computer := NewComputer(Config{Source: db, Root: root, Areas: []types.AreaID{area}, Strategy: strategy})
	t.Cleanup(computer.Stop)
	t.Cleanup(func() {
		select {
		case <-strategy.release:
		default:
			close(strategy.release)
		}
	})
	done := make(chan struct{})
	go func() {
		computer.Run()
		close(done)
	}()
	select {
	case <-strategy.captured:
	case <-time.After(5 * time.Second):
		t.Fatal("first SPF did not capture its native graph")
	}
	if !db.Delete(area, types.LSAKey{Type: types.LSTypeRouter, LinkStateID: types.LinkStateID(peer), AdvertisingRouter: peer}) {
		t.Fatal("peer Router-LSA missing")
	}
	computer.Run()
	if view := computer.Reachability(); !view.Ready() || view.RouterReachable(area, peer) {
		t.Fatal("newer SPF did not publish the disconnected graph")
	}
	close(strategy.release)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("older SPF did not finish")
	}
	if computer.Reachability().RouterReachable(area, peer) || len(computer.Routes()) != 0 {
		t.Fatal("older SPF overwrote newer reachability or installed its stale route")
	}
}
