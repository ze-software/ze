// VALIDATES: gap #55 (RFC2328-8.1-1) next hop across an unnumbered point-to-point
// link. The root's Router-LSA link carries an ifIndex as Link Data (RFC 2328
// Section 12.4.1.1); SPF resolves the next hop from the Full adjacency on that
// ifIndex (the neighbor's Hello source on the named device), marks it on-link,
// and the same source feeds the LFA alternate and the Loc-RIB path.
// PREVENTS: an unnumbered neighbor's prefixes being unreachable, or installed
// with a gateway the kernel refuses because it lies off the device's /32.
package spf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

type unnumberedAdjacency struct {
	key  adjacencyKey
	addr netip.Addr
	name string
}

type adjacencyKey struct {
	localID uint32
	router  types.RouterID
}

// fakeLinkAdjacency answers NextHopOnLink from a fixed table, and records the
// area and root it was asked for so the test can prove they are passed through.
type fakeLinkAdjacency struct {
	rows  []unnumberedAdjacency
	areas []types.AreaID
	roots []types.RouterID
}

func (f *fakeLinkAdjacency) NextHopOnLink(area types.AreaID, root types.RouterID, localID uint32, id types.RouterID) (netip.Addr, string, bool) {
	f.areas = append(f.areas, area)
	f.roots = append(f.roots, root)
	for _, r := range f.rows {
		if r.key.localID == localID && r.key.router == id {
			return r.addr, r.name, true
		}
	}
	return netip.Addr{}, "", false
}

// unnumberedLink is an unnumbered point-to-point link of cost 10 toward router
// on the interface ifIndex.
func unnumberedLink(t *testing.T, router string, ifIndex byte) packet.RouterLink {
	t.Helper()
	return packet.RouterLink{LinkID: testLSID(t, router), LinkData: [4]byte{0, 0, 0, ifIndex}, Type: packet.RouterLinkTypeP2P, Metric: metric(t, 10)}
}

// unnumberedTriangleSource is the LFA triangle with both of the root's links
// unnumbered: 1.1.1.1 reaches 2.2.2.2 on ifIndex 12 and 3.3.3.3 on ifIndex 13.
// 192.0.2.0/24 sits behind 2.2.2.2, and 3.3.3.3 is its loop-free alternate.
func unnumberedTriangleSource(t *testing.T) Source {
	t.Helper()
	return testSource(t, testArea(),
		routerLSA(t, "1.1.1.1", unnumberedLink(t, "2.2.2.2", 12), unnumberedLink(t, "3.3.3.3", 13)),
		routerLSA(t, "2.2.2.2", unnumberedLink(t, "1.1.1.1", 21), p2pLink(t, "3.3.3.3", "10.0.23.2", 10), stubLink(t, "192.0.2.0", 1)),
		routerLSA(t, "3.3.3.3", unnumberedLink(t, "1.1.1.1", 31), p2pLink(t, "2.2.2.2", "10.0.23.3", 10)),
	)
}

func unnumberedAdjacencies(t *testing.T) *fakeLinkAdjacency {
	t.Helper()
	return &fakeLinkAdjacency{rows: []unnumberedAdjacency{
		{key: adjacencyKey{localID: 12, router: testRID(t, "2.2.2.2")}, addr: netip.MustParseAddr("198.51.100.2"), name: "eth12"},
		{key: adjacencyKey{localID: 13, router: testRID(t, "3.3.3.3")}, addr: netip.MustParseAddr("198.51.100.3"), name: "eth13"},
	}}
}

func TestUnnumberedP2PNextHopAndLFA(t *testing.T) {
	adj := unnumberedAdjacencies(t)
	loc := locrib.NewRIB()
	c := NewComputer(Config{
		Source:    unnumberedTriangleSource(t),
		Root:      testRID(t, "1.1.1.1"),
		Areas:     []types.AreaID{testArea()},
		Installer: NewInstaller(loc),
		Strategy:  NewV4Strategy(adj),
	})
	c.SetFastReroute(FastRerouteConfig{Enabled: true, NodeProtection: true})
	c.Run()

	r, ok := backupFor(c.Routes(), "192.0.2.0/24")
	if !ok {
		t.Fatalf("192.0.2.0/24 not computed through the unnumbered neighbor: %+v", c.Routes())
	}
	want := NextHop{Addr: netip.MustParseAddr("198.51.100.2"), Interface: "eth12", Router: testRID(t, "2.2.2.2"), OnLink: true}
	if len(r.NextHops) != 1 || r.NextHops[0] != want {
		t.Fatalf("next hops = %+v, want [%+v]", r.NextHops, want)
	}
	if len(r.Backups) == 0 || !r.Backups[0].Valid() {
		t.Fatalf("no LFA alternate across the second unnumbered link: %+v", r)
	}
	if r.Backups[0].NextHop != netip.MustParseAddr("198.51.100.3") || r.Backups[0].Interface != "eth13" {
		t.Fatalf("backup = %+v, want 198.51.100.3 on eth13", r.Backups[0])
	}
	// The LFA pass re-roots SPF at each neighbor for distances only; the
	// adjacency answers solely for this router's own root, so a re-rooted query
	// finds nothing. At least one query MUST carry the area and this root.
	var asked bool
	for i := range adj.areas {
		if adj.areas[i] == testArea() && adj.roots[i] == testRID(t, "1.1.1.1") {
			asked = true
		}
	}
	if !asked {
		t.Fatalf("adjacency never asked for area %v root 1.1.1.1: areas %v roots %v", testArea(), adj.areas, adj.roots)
	}

	paths := lookupPaths(t, loc, netip.MustParsePrefix("192.0.2.0/24"))
	if len(paths) != 1 {
		t.Fatalf("Loc-RIB paths = %+v, want one", paths)
	}
	p := paths[0]
	if p.NextHop != want.Addr || p.Interface != "eth12" || !p.OnLink {
		t.Fatalf("primary path = %+v, want 198.51.100.2 on eth12 on-link", p)
	}
	if p.BackupNextHop != netip.MustParseAddr("198.51.100.3") || p.BackupInterface != "eth13" || !p.BackupOnLink {
		t.Fatalf("backup path = %+v, want 198.51.100.3 on eth13 on-link", p)
	}
}

// TestUnnumberedP2PNoAdjacencyNoNextHop proves the ifIndex is never used as an
// address: with no Full adjacency on the ifIndex, the neighbor gets no next hop.
func TestUnnumberedP2PNoAdjacencyNoNextHop(t *testing.T) {
	for _, strategy := range []AFPrefixStrategy{nil, NewV4Strategy(&fakeLinkAdjacency{})} {
		c := NewComputer(Config{
			Source:   unnumberedTriangleSource(t),
			Root:     testRID(t, "1.1.1.1"),
			Areas:    []types.AreaID{testArea()},
			Strategy: strategy,
		})
		c.Run()
		for _, r := range c.Routes() {
			for _, nh := range r.NextHops {
				if nh.Addr.IsValid() {
					t.Fatalf("route %s got next hop %+v with no adjacency on the unnumbered link", r.Prefix, nh)
				}
			}
		}
	}
}
