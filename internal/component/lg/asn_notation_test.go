package lg

import (
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/asn"
)

// TestExtractASPathReadsADottedRow proves the AS-path graph is built from a
// route row the daemon wrote under bgp/as-notation asdot. The method extracts
// one path of each spelling.
//
// VALIDATES: extractASPath reads the string form through asn.FromJSON.
// PREVENTS: the graph silently dropping every AS number of 65536 or more,
// which drew a path that does not exist. The walk skipped a string outright.
func TestExtractASPathReadsADottedRow(t *testing.T) {
	dotted := extractASPath(map[string]any{"as-path": []any{"1.10", "100"}})
	if len(dotted) != 2 || dotted[0] != 65546 || dotted[1] != 100 {
		t.Errorf("asdot row = %v, want [65546 100]", dotted)
	}

	plain := extractASPath(map[string]any{"as-path": []any{float64(65546), float64(100)}})
	if len(plain) != 2 || plain[0] != 65546 || plain[1] != 100 {
		t.Errorf("asplain row = %v, want [65546 100]", plain)
	}

	// A token that names no AS number is still dropped rather than read as 0.
	if got := extractASPath(map[string]any{"as-path": []any{"peer", "1.99999"}}); len(got) != 0 {
		t.Errorf("unreadable row = %v, want nothing", got)
	}
}

// TestASPathTableReadsADottedRow proves the AS path an operator reads in the
// route table and downloads as CSV is the text the daemon rendered. The method
// formats one dotted row.
//
// VALIDATES: formatASPathPlain passes the string through scalarString.
// PREVENTS: the looking glass re-deriving a spelling of its own, which would
// disagree with the daemon whenever the two read different configurations.
func TestASPathTableReadsADottedRow(t *testing.T) {
	got := formatASPathPlain(map[string]any{"as-path": []any{"1.10", "100"}})
	if got != "1.10 100" {
		t.Errorf("as-path text = %q, want %q", got, "1.10 100")
	}
}

// TestGraphLabelsFollowTheConfiguredNotation proves the AS labels a human
// reads on the topology graph carry the configured notation. The method
// renders one graph per notation.
//
// VALIDATES: renderGraphText writes each node and edge through asn.Text.
// PREVENTS: an operator reading 1.10 in the route table and 65546 on the
// graph beside it.
func TestGraphLabelsFollowTheConfiguredNotation(t *testing.T) {
	restore := asn.Configured()
	t.Cleanup(func() { configureNotation(t, restore) })

	routes := []any{map[string]any{"as-path": []any{"1.10", "100"}}}

	configureNotation(t, asn.NotationDot)
	dotted := renderGraphText(buildGraph(routes))
	if !contains(dotted, "node AS1.10") || !contains(dotted, "edge AS1.10 -> AS100") {
		t.Errorf("asdot graph = %q, want AS1.10 nodes and edges", dotted)
	}

	configureNotation(t, asn.NotationPlain)
	plain := renderGraphText(buildGraph(routes))
	if !contains(plain, "node AS65546") {
		t.Errorf("asplain graph = %q, want AS65546", plain)
	}
}

// TestResolveASNLooksUpTheDecimalForm proves the ASN-name lookup key is the
// decimal value whatever notation the row carried. The method resolves both
// spellings against a decorator that records its key.
//
// VALIDATES: LGServer.resolveASN normalizes before it calls the decorator.
// PREVENTS: every ASN-name lookup failing under asdot, because Team Cymru is
// asked for AS "1.10", a name that resolves to nothing.
func TestResolveASNLooksUpTheDecimalForm(t *testing.T) {
	var asked string
	server := &LGServer{decorateASN: func(key string) string {
		asked = key
		return "EXAMPLE"
	}}

	for _, text := range []string{"1.10", "65546"} {
		asked = ""
		if got := server.resolveASN(text); got != "EXAMPLE" {
			t.Errorf("resolveASN(%q) = %q, want EXAMPLE", text, got)
		}
		if asked != "65546" {
			t.Errorf("resolveASN(%q) asked the decorator for %q, want %q", text, asked, "65546")
		}
	}
}

// configureNotation records one notation for the rest of a test.
func configureNotation(t *testing.T, notation asn.Notation) {
	t.Helper()
	if err := asn.Configure(notation.String()); err != nil {
		t.Fatalf("asn.Configure(%s): %v", notation, err)
	}
}

// contains reports whether text holds want.
func contains(text, want string) bool {
	for i := 0; i+len(want) <= len(text); i++ {
		if text[i:i+len(want)] == want {
			return true
		}
	}
	return false
}
