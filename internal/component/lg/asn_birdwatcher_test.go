package lg

import (
	"encoding/json"
	"testing"

	"github.com/ze-software/ze/internal/core/report"
)

// TestBirdwatcherASPathStaysNumbers proves the birdwatcher-compatible API
// answers `bgp.as_path` as an array of NUMBERS whatever notation the route row
// carried. The method transforms a dotted row and an asplain row and compares
// the marshaled array.
//
// VALIDATES: asPathNumbers puts the numbers back, so bgp/as-notation does not
// reach this API.
// PREVENTS: a display preference changing a machine contract.
// docs/architecture/api/birdwatcher-compat.md declares an array of numbers. A
// client that parses an integer would fail on a server whose operator changed
// how they read an AS number.
func TestBirdwatcherASPathStaysNumbers(t *testing.T) {
	for _, row := range []struct {
		name    string
		asPath  []any
		wantAll string
	}{
		{"asdot row", []any{"1.10", "100"}, `[65546,100]`},
		{"asplain row", []any{float64(65546), float64(100)}, `[65546,100]`},
	} {
		t.Run(row.name, func(t *testing.T) {
			ze := map[string]any{"routes": []any{map[string]any{
				"prefix":       "10.0.0.0/24",
				"next-hop":     "10.0.0.1",
				"as-path":      row.asPath,
				"peer-address": "10.0.0.1",
			}}}

			routes, ok := transformRoutes(ze, "")["routes"].([]any)
			if !ok || len(routes) != 1 {
				t.Fatalf("expected 1 route, got %v", routes)
			}
			route, _ := routes[0].(map[string]any)
			bgp, _ := route["bgp"].(map[string]any)

			encoded, err := json.Marshal(bgp["as_path"])
			if err != nil {
				t.Fatalf("marshal as_path: %v", err)
			}
			if string(encoded) != row.wantAll {
				t.Errorf("as_path = %s, want %s", encoded, row.wantAll)
			}
		})
	}
}

// TestBirdwatcherNeighborASReadsEveryNotation proves `neighbor_as` carries the
// AS number whatever notation the peer row wrote, and that the key is present
// on every protocol. The method builds one summary per spelling.
//
// The key stays present even for a row that names no AS number. birdwatcher
// declares it mandatory, and a client decoding into a struct reads an absent
// key as 0 anyway. setNeighborAS raises a warning on the report bus instead,
// and this test reads it back. A fault an operator cannot see in
// `show warnings` is a fault nobody acts on.
//
// VALIDATES: setNeighborAS reads through asn.FromJSON.
// PREVENTS: neighbor_as reporting AS 0 for every peer under a dotted notation.
// RFC 7607 Section 1 reserves AS 0, so a consumer would read a syntactically
// valid AS number that is wrong for every session.
func TestBirdwatcherNeighborASReadsEveryNotation(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	for _, row := range []struct {
		name     string
		remoteAS any
		want     any
		warned   bool
	}{
		{"asdot row", "1.10", uint32(65546), false},
		{"asplain row", float64(65546), uint32(65546), false},
		{"unreadable row", "not-an-as", uint32(0), true},
	} {
		t.Run(row.name, func(t *testing.T) {
			ze := map[string]any{"peers": []any{map[string]any{
				"name":      "transit-a",
				"address":   "10.0.0.1",
				"state":     "established",
				"remote-as": row.remoteAS,
			}}}

			protocols, ok := transformProtocols(ze)["protocols"].(map[string]any)
			if !ok {
				t.Fatalf("no protocols in the answer")
			}
			protocol, ok := protocols["transit-a"].(map[string]any)
			if !ok {
				t.Fatalf("no protocol for transit-a: %v", protocols)
			}

			got, present := protocol["neighbor_as"]
			if !present {
				t.Fatalf("neighbor_as is absent; birdwatcher declares it mandatory")
			}
			if got != row.want {
				t.Errorf("neighbor_as = %v, want %v", got, row.want)
			}
			assertUnreadableASWarning(t, "transit-a", row.warned)
		})
	}
}

// assertUnreadableASWarning reads the report bus back, and says whether the
// unreadable-AS warning is active for one peer.
func assertUnreadableASWarning(t *testing.T, subject string, want bool) {
	t.Helper()
	for _, issue := range report.Warnings() {
		if issue.Source == reportSourceLG && issue.Code == reportCodeUnreadableAS && issue.Subject == subject {
			if !want {
				t.Errorf("a readable AS number left a warning on the bus: %s", issue.Message)
			}
			return
		}
	}
	if want {
		t.Errorf("an unreadable AS number raised no warning; an operator would never see the fault")
	}
}
