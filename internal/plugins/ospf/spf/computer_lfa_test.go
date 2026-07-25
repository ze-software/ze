// VALIDATES: story 5 (fast-reroute disabled -> no backups, route set unchanged)
// and the wiring (config enables the LFA pass in Run and the backup reaches the
// locrib.Path).
// PREVENTS: the LFA pass running when disabled, or the backup not reaching install.
package spf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestFastRerouteDisabledNoBackup(t *testing.T) {
	c := NewComputer(Config{
		Source: triangleAltSource(t),
		Root:   testRID(t, "1.1.1.1"),
		Areas:  []types.AreaID{testArea()},
	})
	c.Run()
	routes := c.Routes()
	if len(routes) == 0 {
		t.Fatal("no routes computed")
	}
	for _, r := range routes {
		if len(r.Backups) != 0 {
			t.Fatalf("fast-reroute disabled but backups present: %+v", r)
		}
	}
	// attachAllBackups returns early when disabled, so `selected` is never mutated:
	// the route set is byte-for-byte a router without fast-reroute.
}

func TestFastRerouteConfigEnablesLFAPass(t *testing.T) {
	loc := locrib.NewRIB()
	c := NewComputer(Config{
		Source:    triangleAltSource(t),
		Root:      testRID(t, "1.1.1.1"),
		Areas:     []types.AreaID{testArea()},
		Installer: NewInstaller(loc),
	})
	c.SetFastReroute(FastRerouteConfig{Enabled: true, NodeProtection: true})
	c.Run()

	r, ok := backupFor(c.Routes(), "192.0.2.0/24")
	if !ok || len(r.Backups) == 0 || !r.Backups[0].Valid() {
		t.Fatalf("config did not enable the LFA pass: %+v", c.Routes())
	}
	if r.Backups[0].NextHop != netip.MustParseAddr("10.0.13.3") {
		t.Fatalf("backup next-hop = %v, want 10.0.13.3", r.Backups[0].NextHop)
	}
	// The backup reached the locrib.Path.
	paths := lookupPaths(t, loc, netip.MustParsePrefix("192.0.2.0/24"))
	if len(paths) == 0 || paths[0].BackupNextHop != netip.MustParseAddr("10.0.13.3") {
		t.Fatalf("backup not installed to locrib: %+v", paths)
	}

	// FastRerouteSnapshot surfaces the protected prefix.
	var protected bool
	for _, e := range c.FastRerouteSnapshot() {
		if e.Prefix == "192.0.2.0/24" {
			for _, h := range e.NextHops {
				if h.Protected {
					protected = true
				}
			}
		}
	}
	if !protected {
		t.Fatal("FastRerouteSnapshot did not surface the protected prefix")
	}
}
