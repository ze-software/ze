// VALIDATES: spec-ospf-ext-5 R-8 -- the SPF post-run hook fires after the Installer
// applied the route delta, so a Segment Routing label push rides an existing IP route.
// PREVENTS: SR install racing ahead of the IP route it depends on.
package spf

import (
	"testing"

	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestSetPostRunFiresAfterInstall(t *testing.T) {
	area := testArea()
	loc := locrib.NewRIB()
	db := baseP2PSource(t, area)
	c := NewComputer(Config{Source: db, Root: testRID(t, "1.1.1.1"), Areas: []types.AreaID{area}, Installer: NewInstaller(loc)})

	fired := 0
	routesAtHook := -1
	c.SetPostRun(func() {
		fired++
		// When the hook fires the route table is already populated (install happened).
		routesAtHook = len(c.Snapshot())
	})
	c.Run()
	if fired != 1 {
		t.Fatalf("post-run hook fired %d times, want 1", fired)
	}
	if routesAtHook < 1 {
		t.Fatalf("post-run hook saw %d routes; it must fire AFTER the Installer Apply", routesAtHook)
	}
}

func TestSetPostRunNilDisables(t *testing.T) {
	area := testArea()
	loc := locrib.NewRIB()
	db := baseP2PSource(t, area)
	c := NewComputer(Config{Source: db, Root: testRID(t, "1.1.1.1"), Areas: []types.AreaID{area}, Installer: NewInstaller(loc)})
	c.SetPostRun(nil) // must not panic
	c.Run()
}
