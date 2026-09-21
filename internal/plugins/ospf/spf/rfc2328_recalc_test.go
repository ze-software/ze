// RFC 2328 section 13: a router-LSA or network-LSA change recalculates the entire routing
// table, starting with the shortest-path calculation of every area, not only the area whose
// database changed. The Computer's Run walks every configured area and ignores the dirty mark.

package spf

import (
	"testing"

	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// RFC requirement: RFC2328-13-6 positive — after a change marks only area 1 dirty, the next Run
// performs the shortest-path calculation of area 0 as well as area 1: both areas' run states
// advance (Computer.Run walks every configured area).
func TestSPFRecalculatesEveryAreaOnOneAreaChange(t *testing.T) {
	area0 := types.BackboneArea
	area1 := types.AreaID{0, 0, 0, 1}
	c := NewComputer(Config{Source: baseP2PSource(t, area0), Root: testRID(t, "1.1.1.1"), Areas: []types.AreaID{area0, area1}, Installer: NewInstaller(locrib.NewRIB())})
	defer c.Stop()

	c.Run()
	c.mu.Lock()
	first0, ok0 := c.state[area0]
	first1, ok1 := c.state[area1]
	c.mu.Unlock()
	if !ok0 || !ok1 {
		t.Fatalf("first Run states: area0 %v, area1 %v, want both computed", ok0, ok1)
	}

	// A change in area 1 alone.
	c.TriggerArea(area1)
	c.Run()
	c.mu.Lock()
	second0 := c.state[area0]
	second1 := c.state[area1]
	c.mu.Unlock()
	if !second1.LastRun.After(first1.LastRun) {
		t.Fatalf("area 1 was not recalculated: last run %v, previous %v", second1.LastRun, first1.LastRun)
	}
	if !second0.LastRun.After(first0.LastRun) {
		t.Fatalf("area 0 was not recalculated after an area 1 change: last run %v, previous %v", second0.LastRun, first0.LastRun)
	}
}

// RFC requirement: RFC2328-13-6 negative — the recalculation covers the configured areas and
// nothing else: an area removed from the configuration before a Run is not calculated again,
// while every remaining area is (Computer.Run walks c.areas, not the dirty set).
func TestSPFRecalculationBoundedToConfiguredAreas(t *testing.T) {
	area0 := types.BackboneArea
	area1 := types.AreaID{0, 0, 0, 1}
	c := NewComputer(Config{Source: baseP2PSource(t, area0), Root: testRID(t, "1.1.1.1"), Areas: []types.AreaID{area0, area1}, Installer: NewInstaller(locrib.NewRIB())})
	defer c.Stop()

	c.Run()
	c.mu.Lock()
	first0 := c.state[area0]
	first1 := c.state[area1]
	c.mu.Unlock()

	c.SetAreas([]types.AreaID{area0})
	c.TriggerArea(area1) // a stale mark for the removed area must not bring it back
	c.Run()
	c.mu.Lock()
	second0 := c.state[area0]
	second1 := c.state[area1]
	c.mu.Unlock()
	if !second0.LastRun.After(first0.LastRun) {
		t.Fatalf("area 0 was not recalculated: last run %v, previous %v", second0.LastRun, first0.LastRun)
	}
	if !second1.LastRun.Equal(first1.LastRun) {
		t.Fatalf("removed area 1 was recalculated: last run %v, previous %v", second1.LastRun, first1.LastRun)
	}
}
