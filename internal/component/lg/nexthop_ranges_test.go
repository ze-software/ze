package lg

import (
	"errors"
	"net/netip"
	"strconv"
	"strings"
	"testing"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/opererr"
)

// namedRangesServer returns a server that holds the given named ranges, as
// prefix then name.
func namedRangesServer(t *testing.T, prefixThenName ...string) *LGServer {
	t.Helper()
	ranges := make(map[netip.Prefix]string, len(prefixThenName)/2)
	for i := 0; i+1 < len(prefixThenName); i += 2 {
		ranges[netip.MustParsePrefix(prefixThenName[i])] = prefixThenName[i+1]
	}
	srv := testServer()
	if _, err := srv.UpdateNextHopRanges(ranges); err != nil {
		t.Fatalf("store the named ranges: %v", err)
	}
	return srv
}

// TestNewNextHopRangesBounds asks what list the server takes: 256 ranges are
// the last valid count and 257 are refused, and a range with host bits or with
// no name is refused. These repeat, on the server's side, what the config
// reader guarantees.
func TestNewNextHopRangesBounds(t *testing.T) {
	build := func(count int) map[netip.Prefix]string {
		ranges := make(map[netip.Prefix]string, count)
		for i := range count {
			ranges[netip.MustParsePrefix("10."+strconv.Itoa(i/256)+"."+strconv.Itoa(i%256)+".0/24")] = "range"
		}
		return ranges
	}
	if _, err := newNextHopRanges(build(zeconfig.LGNextHopRangesMax)); err != nil {
		t.Errorf("%d ranges were refused: %v", zeconfig.LGNextHopRangesMax, err)
	}
	if _, err := newNextHopRanges(build(zeconfig.LGNextHopRangesMax + 1)); !errors.Is(err, errNextHopRangesTooMany) {
		t.Errorf("%d ranges: error %v, want errNextHopRangesTooMany", zeconfig.LGNextHopRangesMax+1, err)
	}

	hostBits := map[netip.Prefix]string{netip.MustParsePrefix("198.51.100.1/24"): "ixp-lan"}
	if _, err := newNextHopRanges(hostBits); !errors.Is(err, errNextHopRangesPrefix) {
		t.Errorf("a range with host bits: error %v, want errNextHopRangesPrefix", err)
	}
	invalid := map[netip.Prefix]string{{}: "ixp-lan"}
	if _, err := newNextHopRanges(invalid); !errors.Is(err, errNextHopRangesPrefix) {
		t.Errorf("the zero prefix: error %v, want errNextHopRangesPrefix", err)
	}
	mapped := map[netip.Prefix]string{netip.MustParsePrefix("::ffff:198.51.100.0/120"): "ixp-lan"}
	if _, err := newNextHopRanges(mapped); !errors.Is(err, errNextHopRangesPrefix) {
		t.Errorf("an IPv4-mapped range: error %v, want errNextHopRangesPrefix", err)
	}
	nameless := map[netip.Prefix]string{netip.MustParsePrefix("198.51.100.0/24"): ""}
	if _, err := newNextHopRanges(nameless); !errors.Is(err, errNextHopRangesNameless) {
		t.Errorf("a range with no name: error %v, want errNextHopRangesNameless", err)
	}
}

// TestUpdateNextHopRangesReplacesTheList asks whether the reload seam swaps
// the list whole on a running server (AC-13): the old name is gone, the new
// one is shown, and a list the server refuses leaves the installed one in
// place.
func TestUpdateNextHopRangesReplacesTheList(t *testing.T) {
	srv := testServer()
	if got := srv.nextHopName("198.51.100.7"); got != "" {
		t.Fatalf("a server with no list names 198.51.100.7 %q", got)
	}

	lan := netip.MustParsePrefix("198.51.100.0/24")
	if _, err := srv.UpdateNextHopRanges(map[netip.Prefix]string{lan: "ixp-lan-a"}); err != nil {
		t.Fatalf("first list: %v", err)
	}
	if got := srv.nextHopName("198.51.100.7"); got != "ixp-lan-a" {
		t.Errorf("name = %q, want ixp-lan-a", got)
	}

	if _, err := srv.UpdateNextHopRanges(map[netip.Prefix]string{lan: "ixp-lan-b"}); err != nil {
		t.Fatalf("second list: %v", err)
	}
	if got := srv.nextHopName("198.51.100.7"); got != "ixp-lan-b" {
		t.Errorf("name after the swap = %q, want ixp-lan-b", got)
	}

	refused := map[netip.Prefix]string{lan: ""}
	if _, err := srv.UpdateNextHopRanges(refused); err == nil {
		t.Fatal("a list with a nameless range was accepted")
	}
	if got := srv.nextHopName("198.51.100.7"); got != "ixp-lan-b" {
		t.Errorf("name after a refused list = %q, want ixp-lan-b kept", got)
	}

	if _, err := srv.UpdateNextHopRanges(nil); err != nil {
		t.Fatalf("empty list: %v", err)
	}
	if got := srv.nextHopName("198.51.100.7"); got != "" {
		t.Errorf("name after the list was emptied = %q, want none", got)
	}
}

// TestUpdateNextHopRangesRestoresTheReplacedList asks whether the call a
// reload gets back puts the list it replaced back, for a first list and for a
// list that replaced another.
//
// PREVENTS: a reload that is rolled back after the names changed leaving the
// page with the names of a configuration that is not in force.
func TestUpdateNextHopRangesRestoresTheReplacedList(t *testing.T) {
	srv := testServer()
	lan := netip.MustParsePrefix("198.51.100.0/24")

	restore, err := srv.UpdateNextHopRanges(map[netip.Prefix]string{lan: "ixp-lan-a"})
	if err != nil {
		t.Fatalf("first list: %v", err)
	}
	restore()
	if got := srv.nextHopName("198.51.100.7"); got != "" {
		t.Errorf("name after the first list was rolled back = %q, want none", got)
	}

	if _, err = srv.UpdateNextHopRanges(map[netip.Prefix]string{lan: "ixp-lan-a"}); err != nil {
		t.Fatalf("accepted list: %v", err)
	}
	restore, err = srv.UpdateNextHopRanges(map[netip.Prefix]string{lan: "ixp-lan-b"})
	if err != nil {
		t.Fatalf("second list: %v", err)
	}
	if got := srv.nextHopName("198.51.100.7"); got != "ixp-lan-b" {
		t.Fatalf("name before the rollback = %q, want ixp-lan-b", got)
	}
	restore()
	if got := srv.nextHopName("198.51.100.7"); got != "ixp-lan-a" {
		t.Errorf("name after the rollback = %q, want ixp-lan-a", got)
	}

	restore, err = srv.UpdateNextHopRanges(map[netip.Prefix]string{lan: ""})
	if err == nil {
		t.Error("a list with a nameless range was accepted")
	}
	if restore != nil {
		t.Error("a refused list answered a restore call, want none")
	}
}

// TestRouteTableShowsNextHopName asks whether the name reaches the markup of
// the three places a next hop is shown (AC-10): the search table, a peer's
// route table and the route detail. A next hop in no range shows its address
// alone, and operator text is escaped.
func TestRouteTableShowsNextHopName(t *testing.T) {
	srv := namedRangesServer(t, "198.51.100.0/24", "ixp-lan-a")
	rows := srv.routeRowsWithNextHopNames([]any{
		map[string]any{"prefix": "10.60.1.0/24", "next-hop": "198.51.100.7"},
		map[string]any{"prefix": "10.60.2.0/24", "next-hop": "203.0.113.9"},
	})
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	const named = `198.51.100.7 <span class="next-hop-name">ixp-lan-a</span>`

	search, err := renderToString(routeResults(searchView{Routes: rows, Count: 2}))
	if err != nil {
		t.Fatalf("render the search table: %v", err)
	}
	peer, err := renderToString(peerRoutesContent(peerRoutesView{Address: "192.0.2.1", Routes: rows}))
	if err != nil {
		t.Fatalf("render the peer table: %v", err)
	}
	detail, err := renderToString(routeDetail(routeDetailView{Prefix: "10.60.1.0/24", Route: &rows[0]}))
	if err != nil {
		t.Fatalf("render the route detail: %v", err)
	}
	for where, body := range map[string]string{"search table": search, "peer table": peer, "route detail": detail} {
		if !strings.Contains(body, named) {
			t.Errorf("%s does not show the name beside the next hop: %s", where, body)
		}
	}
	for where, body := range map[string]string{"search table": search, "peer table": peer} {
		if !strings.Contains(body, "203.0.113.9") {
			t.Errorf("%s does not show the unnamed next hop: %s", where, body)
		}
		if strings.Count(body, "next-hop-name") != 1 {
			t.Errorf("%s names %d next hops, want only the one in a named range: %s",
				where, strings.Count(body, "next-hop-name"), body)
		}
	}

	hostile := routeRow{Prefix: "10.60.3.0/24", NextHop: "198.51.100.8", NextHopName: `<script>x</script>`}
	escaped, err := renderToString(routeResults(searchView{Routes: []routeRow{hostile}, Count: 1}))
	if err != nil {
		t.Fatalf("render the hostile name: %v", err)
	}
	if strings.Contains(escaped, "<script>") {
		t.Errorf("a name reached the page unescaped: %s", escaped)
	}
}

// TestPeersPageShowsDescriptionAndGroup asks whether the peers page shows the
// peer's own description and keeps its group apart (AC-14): the description is
// the cell, the group is that cell's tooltip and a field of the peer header. A
// peer with no description shows an empty cell and never its group.
func TestPeersPageShowsDescriptionAndGroup(t *testing.T) {
	summary := map[string]any{fieldPeers: []any{
		map[string]any{"address": "192.0.2.1", "remote-as": "65001", "description": "transit to example", "group": "upstreams"},
		map[string]any{"address": "192.0.2.2", "remote-as": "65002", "description": "", "group": "members"},
		map[string]any{"address": "192.0.2.3", "remote-as": "65003", "description": "", "group": ""},
	}}
	peers := testServer().extractPeers(summary)
	if len(peers) != 3 {
		t.Fatalf("peers = %d, want 3", len(peers))
	}
	if peers[0].Description != "transit to example" {
		t.Errorf("description = %q, want the peer's own label", peers[0].Description)
	}
	if peers[0].Group != "upstreams" {
		t.Errorf("group = %q, want upstreams", peers[0].Group)
	}
	if peers[1].Description != "" {
		t.Errorf("a peer with no description shows %q, want an empty cell", peers[1].Description)
	}

	body, err := renderToString(peersTableBody(peers))
	if err != nil {
		t.Fatalf("render the peer rows: %v", err)
	}
	for _, want := range []string{`title="Group upstreams"`, `>transit to example</td>`, `title="Group members"`} {
		if !strings.Contains(body, want) {
			t.Errorf("peer rows do not hold %q: %s", want, body)
		}
	}
	if strings.Contains(body, ">members</td>") {
		t.Errorf("the group stands in for a missing description: %s", body)
	}
	if strings.Count(body, `title="Group `) != 2 {
		t.Errorf("%d rows carry a group tooltip, want 2: the third peer has no group: %s",
			strings.Count(body, `title="Group `), body)
	}

	header, err := renderToString(peerRoutesContent(peerRoutesView{
		Address: "192.0.2.1",
		Peer:    peerInfoFrom(map[string]any{"description": "transit to example", "group": "upstreams"}),
	}))
	if err != nil {
		t.Fatalf("render the peer header: %v", err)
	}
	for _, want := range []string{`<span class="peer-info-desc">transit to example</span>`, `<span class="peer-info-group">Group upstreams</span>`} {
		if !strings.Contains(header, want) {
			t.Errorf("peer header does not hold %q: %s", want, header)
		}
	}
	noGroup, err := renderToString(peerRoutesContent(peerRoutesView{
		Address: "192.0.2.3", Peer: peerInfoFrom(map[string]any{"description": "", "group": ""}),
	}))
	if err != nil {
		t.Fatalf("render the peer header with no group: %v", err)
	}
	if strings.Contains(noGroup, "peer-info-group") {
		t.Errorf("a peer in no group shows a group field: %s", noGroup)
	}
}

// nextHopRangesTree builds a tree whose looking glass block holds the given
// next-hop-range entries, as key then name.
func nextHopRangesTree(keyThenName ...string) *zeconfig.Tree {
	tree := zeconfig.NewTree()
	block := tree.GetOrCreateContainer("environment").GetOrCreateContainer("looking-glass")
	for i := 0; i+1 < len(keyThenName); i += 2 {
		entry := zeconfig.NewTree()
		entry.Set("name", keyThenName[i+1])
		block.AddListEntry("next-hop-range", keyThenName[i], entry)
	}
	return tree
}

// TestNextHopRangeValidatorRefusesOneRangeTwice asks whether the looking
// glass's tree validator refuses two entries for one range, naming both
// entries and ending with the line to remove (AC-12), and whether nested
// ranges pass.
func TestNextHopRangeValidatorRefusesOneRangeTwice(t *testing.T) {
	err := validateNextHopRanges(nextHopRangesTree("198.51.100.0/24", "ixp-lan-a", "198.51.100.1/24", "ixp-lan-b"))
	if err == nil {
		t.Fatal("two entries for 198.51.100.0/24 passed the validator")
	}
	for _, entry := range []string{"next-hop-range 198.51.100.0/24", "next-hop-range 198.51.100.1/24"} {
		if !strings.Contains(err.Error(), entry) {
			t.Errorf("refusal does not name the entry %q: %v", entry, err)
		}
	}
	var operator *opererr.Error
	if !errors.As(err, &operator) {
		t.Fatalf("refusal is %T, want an operator error", err)
	}
	if want := "delete environment looking-glass next-hop-range 198.51.100.1/24"; operator.Fix() != want {
		t.Errorf("fix = %q, want %q", operator.Fix(), want)
	}

	if err := validateNextHopRanges(nextHopRangesTree("198.51.100.0/24", "ixp-lan", "198.51.100.0/25", "ixp-lan-low")); err != nil {
		t.Errorf("nested ranges were refused: %v", err)
	}
}

// TestNextHopRangeValidatorIsRegistered asks whether the refusal is reached
// through the walk every commit, `ze config validate`, start and reload run,
// which is what makes it a commit-time refusal: the validator is registered by
// this package's init (register.go).
func TestNextHopRangeValidatorIsRegistered(t *testing.T) {
	tree := nextHopRangesTree("198.51.100.0/24", "ixp-lan-a", "198.51.100.1/24", "ixp-lan-b")
	_, err := zeconfig.ValidateCustomSections(tree)
	if !errors.Is(err, zeconfig.ErrCustomValidation) {
		t.Fatalf("the config walk answered %v, want the validator's refusal", err)
	}
	if !strings.Contains(err.Error(), "name the same range") {
		t.Errorf("the walk refused for another reason: %v", err)
	}
}
