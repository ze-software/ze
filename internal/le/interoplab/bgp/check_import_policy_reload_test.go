// Design: docs/architecture/testing/interop.md -- the IGP import-policy scenarios.
// Related: check_import_policy_reload.go -- the predicates these tests judge.

package bgp

import (
	"strings"
	"testing"
)

// isisReloadDatabase is a `show isis database` answer in the shape
// databaseSnapshot writes: Ze's own LSP and FRR's, at both levels.
const isisReloadDatabase = `[
{"level":"l2","lsp-id":"0000.0000.0003.00-00","sequence":4,"lifetime":1100,"checksum":4660,"overload":false,"own":false},
{"level":"l1","lsp-id":"0000.0000.0002.00-00","sequence":3,"lifetime":1150,"checksum":291,"overload":false,"own":true},
{"level":"l1","lsp-id":"0000.0000.0003.00-00","sequence":4,"lifetime":1100,"checksum":4661,"overload":false,"own":false},
{"level":"l2","lsp-id":"0000.0000.0002.00-00","sequence":3,"lifetime":1150,"checksum":292,"overload":false,"own":true}
]`

// isisReloadNeighbors is a `show isis neighbor` answer in the shape
// neighborSnapshot writes: one point-to-point adjacency at both levels.
const isisReloadNeighbors = `[
{"interface":"eth0","system-id":"0000.0000.0003","level":"l1","state":"up","hold-time":30,"hold-expiry-unix":1000},
{"interface":"eth0","system-id":"0000.0000.0003","level":"l2","state":"up","hold-time":30,"hold-expiry-unix":1000}
]`

// TestISISRouterStateIgnoresWhatTimeMoves proves two readings of an unchanged
// router compare equal although the remaining lifetimes and hold expiries
// moved, and whatever order Ze listed the rows in. Method: the same rows with
// later timers, reversed.
func TestISISRouterStateIgnoresWhatTimeMoves(t *testing.T) {
	before, err := parseISISRouterState(isisReloadDatabase, isisReloadNeighbors)
	if err != nil {
		t.Fatalf("parse first reading: %v", err)
	}
	later := `[
{"level":"l2","lsp-id":"0000.0000.0002.00-00","sequence":3,"lifetime":1120,"checksum":292,"overload":false,"own":true},
{"level":"l1","lsp-id":"0000.0000.0003.00-00","sequence":4,"lifetime":1070,"checksum":4661,"overload":false,"own":false},
{"level":"l1","lsp-id":"0000.0000.0002.00-00","sequence":3,"lifetime":1120,"checksum":291,"overload":false,"own":true},
{"level":"l2","lsp-id":"0000.0000.0003.00-00","sequence":4,"lifetime":1070,"checksum":4660,"overload":false,"own":false}
]`
	neighbors := strings.ReplaceAll(isisReloadNeighbors, `"hold-expiry-unix":1000`, `"hold-expiry-unix":1030`)
	after, err := parseISISRouterState(later, neighbors)
	if err != nil {
		t.Fatalf("parse second reading: %v", err)
	}
	if err := isisRouterStateMoved(before, after); err != nil {
		t.Fatalf("an unchanged router reads as moved: %v", err)
	}
}

// TestISISRouterStateNamesANewLSPVersion proves a re-originated LSP, the
// trace of a fresh advertisement or of an adjacency that flapped, is refused.
// Method: one row at a higher sequence and new checksum, everything else kept.
func TestISISRouterStateNamesANewLSPVersion(t *testing.T) {
	before, err := parseISISRouterState(isisReloadDatabase, isisReloadNeighbors)
	if err != nil {
		t.Fatalf("parse first reading: %v", err)
	}
	for _, edit := range []struct{ name, from, to string }{
		{"FRR's LSP", `"sequence":4,"lifetime":1100,"checksum":4661`, `"sequence":5,"lifetime":1199,"checksum":17`},
		{"Ze's own LSP", `"sequence":3,"lifetime":1150,"checksum":291`, `"sequence":4,"lifetime":1199,"checksum":18`},
	} {
		after, err := parseISISRouterState(strings.Replace(isisReloadDatabase, edit.from, edit.to, 1), isisReloadNeighbors)
		if err != nil {
			t.Fatalf("%s: parse second reading: %v", edit.name, err)
		}
		moved := isisRouterStateMoved(before, after)
		if moved == nil {
			t.Fatalf("%s: a new LSP version reads as unchanged", edit.name)
		}
		if !strings.Contains(moved.Error(), "LSDB moved") {
			t.Fatalf("%s: refusal names the wrong reading: %v", edit.name, moved)
		}
	}
}

// TestISISRouterStateNamesAMovedAdjacency proves an adjacency that is gone or
// was replaced is refused even when the LSDB still reads the same.
func TestISISRouterStateNamesAMovedAdjacency(t *testing.T) {
	before, err := parseISISRouterState(isisReloadDatabase, isisReloadNeighbors)
	if err != nil {
		t.Fatalf("parse first reading: %v", err)
	}
	one := `[{"interface":"eth0","system-id":"0000.0000.0003","level":"l2","state":"up","hold-time":30,"hold-expiry-unix":1000}]`
	after, err := parseISISRouterState(isisReloadDatabase, one)
	if err != nil {
		t.Fatalf("parse second reading: %v", err)
	}
	moved := isisRouterStateMoved(before, after)
	if moved == nil {
		t.Fatal("a lost level-1 adjacency reads as unchanged")
	}
	if !strings.Contains(moved.Error(), "adjacencies moved") {
		t.Fatalf("refusal names the wrong reading: %v", moved)
	}
}

// TestISISRouterStateRefusesAReadingThatCannotMove proves a reading with no
// evidence in it never becomes a baseline: equality between two such readings
// would prove nothing. Method: each missing or down piece on its own.
func TestISISRouterStateRefusesAReadingThatCannotMove(t *testing.T) {
	ownOnly := `[{"level":"l1","lsp-id":"0000.0000.0002.00-00","sequence":3,"lifetime":1150,"checksum":291,"overload":false,"own":true}]`
	foreignOnly := `[{"level":"l1","lsp-id":"0000.0000.0003.00-00","sequence":4,"lifetime":1100,"checksum":4661,"overload":false,"own":false}]`
	purgedOwn := strings.ReplaceAll(isisReloadDatabase, `"own":true}`, `"own":true,"purged":true}`)
	for _, test := range []struct {
		name, database, neighbors, want string
	}{
		{"no adjacency", isisReloadDatabase, `[]`, "no IS-IS adjacency"},
		{"adjacency initializing", isisReloadDatabase, strings.Replace(isisReloadNeighbors, `"state":"up"`, `"state":"initializing"`, 1), `"initializing"`},
		{"no LSP of Ze's own", foreignOnly, isisReloadNeighbors, "no live LSP of its own"},
		{"only purged LSPs of Ze's own", purgedOwn, isisReloadNeighbors, "no live LSP of its own"},
		{"no LSP from a neighbor", ownOnly, isisReloadNeighbors, "no live LSP from a neighbor"},
		{"database not JSON", "show isis database: unknown command", isisReloadNeighbors, "decode Ze's show isis database"},
		{"neighbors not JSON", isisReloadDatabase, "error", "decode Ze's show isis neighbor"},
	} {
		_, err := parseISISRouterState(test.database, test.neighbors)
		if err == nil {
			t.Errorf("%s: accepted", test.name)
			continue
		}
		if !strings.Contains(err.Error(), test.want) {
			t.Errorf("%s: refusal %q does not name %q", test.name, err, test.want)
		}
	}
}
