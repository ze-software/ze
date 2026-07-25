// VALIDATES: spec-ospf-13 AC-9 -- Computer.ClearSPFLog empties the per-area SPF run
// history shown by `show ospf spf` (clear ospf counters) without disturbing the
// computed routes.
// PREVENTS: a counters-clear that leaves stale run history or that drops installed routes.
package spf

import (
	"testing"

	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestComputerClearSPFLog(t *testing.T) {
	area := testArea()
	loc := locrib.NewRIB()
	db := baseP2PSource(t, area)
	c := NewComputer(Config{Source: db, Root: testRID(t, "1.1.1.1"), Areas: []types.AreaID{area}, Installer: NewInstaller(loc)})
	defer c.Stop()

	c.Run()
	if len(c.SPFSnapshot()) == 0 {
		t.Fatal("expected an SPF log entry after Run")
	}

	c.ClearSPFLog()
	if got := c.SPFSnapshot(); len(got) != 0 {
		t.Fatalf("SPFSnapshot after ClearSPFLog = %+v, want empty", got)
	}
	// The computed route survives a counters clear (the log is observational only).
	if snap := c.Snapshot(); len(snap) != 1 {
		t.Fatalf("route snapshot after ClearSPFLog = %+v, want the route preserved", snap)
	}
}
