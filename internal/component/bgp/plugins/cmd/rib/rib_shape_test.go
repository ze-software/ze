// Design: docs/architecture/api/commands.md — per-command answer shape
// Related: rib.go — the declarations this file pins

package rib

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"

	// The BGP peer command plugin declares an EMPTY shape and an EMPTY column
	// order for every direct child of `show bgp`, `show bgp rib` among them.
	// Importing it is what makes this test a test: the two packages declare one
	// path, and this file asserts which declaration the path answers with.
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/peer"
)

// VALIDATES: AC-4. In a process that imports both command plugins,
// `show bgp rib` resolves to the `tab` shape and the column order the rib
// command plugin declares, and it does so whatever order the two packages
// registered in.
// PREVENTS: the empty declaration the peer command plugin puts on every child
// of `show bgp` erasing the rib plugin's declaration, which happened whenever
// package initialization ran the peer plugin second and which nothing reported.
func TestShowBgpRibResolvesToTab(t *testing.T) {
	shape, declared := command.ShapeForCommand(cmdRibShow)
	if !declared {
		t.Fatalf("%s declares no shape, want %s", cmdRibShow, command.ShapeTab)
	}
	if shape != command.ShapeTab {
		t.Errorf("%s resolves to shape %s, want %s", cmdRibShow, shape, command.ShapeTab)
	}
	if orders := command.ColumnsForCommand(cmdRibShow); len(orders) == 0 {
		t.Errorf("%s resolves to no column order, want the one rib.go declares", cmdRibShow)
	}

	// The peer command plugin's empty declaration, made again with the rib
	// plugin's already in the registry. It is the adverse initialization order,
	// and it leaves the registry holding what it held, so no cleanup is owed.
	command.RegisterShape([]string{cmdRibShow})
	command.RegisterColumns([]string{cmdRibShow})

	shape, declared = command.ShapeForCommand(cmdRibShow)
	if !declared || shape != command.ShapeTab {
		t.Errorf("after an empty declaration, %s resolves to shape %s (declared=%v), want %s",
			cmdRibShow, shape, declared, command.ShapeTab)
	}
	if orders := command.ColumnsForCommand(cmdRibShow); len(orders) == 0 {
		t.Errorf("after an empty declaration, %s resolves to no column order", cmdRibShow)
	}
}

// TestRibScalarPathsDeclareForThemselves holds the four rib commands that
// answer a document, not routes, to declaring their own shape.
//
// VALIDATES: AC-16.
// PREVENTS: the inheritance this file's own Phase 1 declaration created. A
// shape and a column order resolve by the longest registered command path that
// is a prefix of the command (commandRegistry.lookup in
// internal/component/command/column_order.go), so `show bgp rib` declaring
// `tab` reached `show bgp rib status`, `... best status` and `... rpf` too.
// Each was published as supporting `| count`, `| first` and `| display` over an
// answer holding no rows, was offered eleven route columns it never writes, and
// was accepted for `| resolve` on the route row's "peer" field. Nothing
// reported any of it: the operator was refused after dispatch by the answer's
// own shape, or not refused at all.
//
// `show bgp rib status` is the one whose old refusal was not even reliable. Its
// answer holds "route-counts" keyed by peer address and, only while a peer is
// in graceful restart, "gr-state" keyed the same way (RIBManager.status,
// internal/component/bgp/plugins/rib/rib_commands.go). One row set is rows and
// two is the ambiguous case rowsInKeyed refuses, so `| count` answered the peer
// count on a router with no GR peer and was refused on the same router a
// restart later. The declaration makes the refusal the answer in both.
func TestRibScalarPathsDeclareForThemselves(t *testing.T) {
	for _, path := range []string{cmdRibStatus, cmdRibBestStatus, cmdRibRPF} {
		t.Run(path, func(t *testing.T) {
			shape, declared := command.ShapeForCommand(path)
			if !declared {
				t.Fatalf("%s declares no shape of its own", path)
			}
			if shape != command.ShapeDoc {
				t.Errorf("%s resolves to %s, want %s: its answer holds no rows", path, shape, command.ShapeDoc)
			}

			// The refusal an operator reaches, not the registry reading alone.
			_, _, errMsg := command.ProcessPipesDefaultFormatChecked(path+" | count", "")
			if errMsg == "" {
				t.Errorf("%s | count was accepted; the answer holds no rows", path)
			} else if !strings.Contains(errMsg, "count") || !strings.Contains(errMsg, "one document") {
				t.Errorf("%s | count refusal = %q, want it to name count and say the answer is one document", path, errMsg)
			}

			// The route column order must not reach a document that carries no
			// route key. Each declares its own, naming the keys its producer
			// writes, so nothing is inherited.
			for _, order := range command.ColumnsForCommand(path) {
				for _, name := range order {
					if name == "direction" || name == "path-id" || name == "communities" {
						t.Errorf("%s resolves the route column order (%v); it answers a document", path, order)
						break
					}
				}
			}
		})
	}
}

// TestRibBestDeclaresItsOwnRows holds `show bgp rib best` to the row it
// actually answers rather than to the route row it inherited.
//
// VALIDATES: AC-14 in the form the payload supports, and AC-15.
// PREVENTS: eleven route column names ordering a best-path row that carries
// five keys. bestResult (internal/component/bgp/plugins/rib/rib_pipeline_best.go)
// writes family, prefix, best-peer, multipath-peers and attributes, and shares
// exactly two names with the route row `show bgp rib` declares.
//
// AC-14 names the chain `| display prefix next-hop`, and that chain cannot
// answer next-hop for THIS command: the best-path row carries next-hop inside
// "attributes", and a record naming at least one displayed field is cut to the
// displayed ones (tableStyle.orderKeys and selectRecord in
// internal/component/command/pipe_columns.go, documented in
// docs/architecture/api/commands.md). So "attributes" goes, and next-hop with
// it. `| display prefix best-peer` names two keys of the same row and proves
// the same property: the operator's own order reaches the renderer.
func TestRibBestDeclaresItsOwnRows(t *testing.T) {
	shape, declared := command.ShapeForCommand(cmdRibBest)
	if !declared || shape != command.ShapeTab {
		t.Fatalf("%s = %v/%v, want tab/declared", cmdRibBest, shape, declared)
	}

	orders := command.ColumnsForCommand(cmdRibBest)
	if len(orders) != 1 {
		t.Fatalf("%s declares %d column orders, want 1", cmdRibBest, len(orders))
	}
	for _, name := range orders[0] {
		if _, written := bestPathRow(t)[name]; !written {
			t.Errorf("%s declares column %q, which bestResult does not write", cmdRibBest, name)
		}
	}

	_, format, errMsg := command.ProcessPipesDefaultFormatChecked(cmdRibBest+" | display prefix best-peer", "")
	if errMsg != "" {
		t.Fatalf("%s | display prefix best-peer was refused: %s", cmdRibBest, errMsg)
	}
	answer := format(bestPathFixture)
	prefixAt := strings.Index(answer, "prefix")
	peerAt := strings.Index(answer, "best-peer")
	if prefixAt < 0 || peerAt < 0 {
		t.Fatalf("%s | display prefix best-peer answered %q, want both fields", cmdRibBest, answer)
	}
	if prefixAt > peerAt {
		t.Errorf("%s | display prefix best-peer put best-peer before prefix: %q", cmdRibBest, answer)
	}
	if strings.Contains(answer, "ipv4") {
		t.Errorf("%s | display kept the family column: %q", cmdRibBest, answer)
	}

	// `| origin` needs a declared address field. Two of the row's values hold a
	// bare address: "best-peer", and "next-hop" inside "attributes". "prefix"
	// and "multipath-peers" hold neither, and declaring either would publish an
	// operator that decorates nothing: originJSON decorates a map value that
	// parses as an address (internal/component/command/pipe_origin.go), and a
	// prefix string fails netip.ParseAddr while an array element is walked past.
	fields := command.AddressFieldsForCommand(cmdRibBest)
	want := []string{"best-peer", "next-hop"}
	if len(fields) != len(want) {
		t.Fatalf("%s address fields = %v, want %v", cmdRibBest, fields, want)
	}
	for i, name := range want {
		if fields[i] != name {
			t.Errorf("%s address fields = %v, want %v", cmdRibBest, fields, want)
		}
	}
	if _, _, errMsg := command.ProcessPipesDefaultFormatChecked(cmdRibBest+" | origin", ""); errMsg != "" {
		t.Errorf("%s | origin was refused: %s", cmdRibBest, errMsg)
	}
}

// TestRibBestFiltersSurviveDeclaration holds the twelve pipe filters
// `show bgp rib best` registers to folding as they did before it declared a
// shape.
//
// VALIDATES: AC-22.
// PREVENTS: a declared shape changing which side of the boundary an operator is
// answered on. `| count` and `| graph` are the command's OWN filters
// (registerPipeFilters, rib.go), so foldFilters rewrites each into a server-side
// argument and the producer answers it; the row operator never runs. A
// declaration is read by validateDeclaredShape, at a different step and from a
// different registry, and neither reads the other.
func TestRibBestFiltersSurviveDeclaration(t *testing.T) {
	for _, filter := range []string{"count", "graph", "histogram", "reason"} {
		t.Run(filter, func(t *testing.T) {
			folded, format, errMsg := command.ProcessPipesDefaultFormatChecked(cmdRibBest+" | "+filter, "")
			if errMsg != "" {
				t.Fatalf("%s | %s was refused: %s", cmdRibBest, filter, errMsg)
			}
			if !strings.Contains(folded, filter) {
				t.Errorf("%s | %s folded to %q, want the filter carried into the command", cmdRibBest, filter, folded)
			}

			// The producer answers it, so the formatter must not fold the rows
			// into a number of its own. Feeding it a two-row answer and getting
			// "2" back would mean the row operator ran after all.
			answer := format(bestPathFixture)
			if answer == "2" || strings.Contains(answer, `"count": 2`) {
				t.Errorf("%s | %s was answered by the row operator: %q", cmdRibBest, filter, answer)
			}
		})
	}
}

// TestPeerRibDeclaresTheRouteShape holds `show bgp peer rib` to the same
// declaration as `show bgp rib`.
//
// VALIDATES: AC-20 for the one path the spec's Current Behavior table missed.
// PREVENTS: a command that forwards to the SAME plugin command declaring
// nothing. forwardRibRoutes sends both paths to cmdRibShow with a peer selector
// (rib.go), so both answer route rows, and only `show bgp rib` said so. The
// path is not covered by inheritance either way: it sits under `show bgp peer`,
// whose empty declaration is what it resolved.
func TestPeerRibDeclaresTheRouteShape(t *testing.T) {
	shape, declared := command.ShapeForCommand(cmdBgpPeerRib)
	if !declared || shape != command.ShapeTab {
		t.Fatalf("%s = %v/%v, want tab/declared", cmdBgpPeerRib, shape, declared)
	}

	peerRibOrders := command.ColumnsForCommand(cmdBgpPeerRib)
	ribOrders := command.ColumnsForCommand(cmdRibShow)
	if len(peerRibOrders) != len(ribOrders) {
		t.Fatalf("%s declares %d orders, %s declares %d; both answer the same rows",
			cmdBgpPeerRib, len(peerRibOrders), cmdRibShow, len(ribOrders))
	}
	for i := range ribOrders {
		if strings.Join(peerRibOrders[i], ",") != strings.Join(ribOrders[i], ",") {
			t.Errorf("%s order %v differs from %s order %v", cmdBgpPeerRib, peerRibOrders[i], cmdRibShow, ribOrders[i])
		}
	}
	if got, want := strings.Join(command.AddressFieldsForCommand(cmdBgpPeerRib), ","),
		strings.Join(command.AddressFieldsForCommand(cmdRibShow), ","); got != want {
		t.Errorf("%s address fields = %q, want %q", cmdBgpPeerRib, got, want)
	}
}

// TestDeclaredColumnsExistInPayload holds every column name this package
// declares to being a key its producer writes.
//
// VALIDATES: AC-21 for the rib paths.
// PREVENTS: the one failure with no signal. A declared name the payload never
// carries orders nothing and publishes a field that does not exist.
//
// The producers run in the bgp-rib plugin PROCESS, in
// internal/component/bgp/plugins/rib, so each answer is a fixture here rather
// than a handler call. Each fixture names the function that writes it, and the
// names were read from that function.
func TestDeclaredColumnsExistInPayload(t *testing.T) {
	cases := []struct {
		path   string
		record map[string]any
	}{
		// RIBManager.status, rib_commands.go. "gr-state" is written only while
		// a peer is in graceful restart, and it is in the fixture because a
		// declared name has to exist in the branch that writes it.
		{cmdRibStatus, decodeFixture(t, `{"running":true,"peers":2,"routes-in":10,"routes-out":4,`+
			`"stale-routes":0,"route-counts":{"192.0.2.1":{"in":10,"out":4}},`+
			`"gr-state":{"192.0.2.1":{"stale-at":"2026-08-24T00:00:00Z","restart-time":120,"expires-at":"2026-08-24T00:02:00Z"}}}`)},
		// RIBManager.bestPathStatus, rib_commands.go.
		{cmdRibBestStatus, decodeFixture(t, `{"running":true,"peers-with-rib":2,"total-routes":10}`)},
		// RIBManager.rpfLookup, rib_commands.go, the FOUND branch. The
		// not-found branch writes source, family and found alone, and an order
		// never hides or invents a key, so naming all seven is correct for both.
		{cmdRibRPF, decodeFixture(t, `{"source":"192.0.2.9","family":"ipv4 unicast","found":true,`+
			`"matched-prefix":"192.0.2.0/24","next-hop":"198.51.100.1","distance":20,"metric":0}`)},
		{cmdRibBest, bestPathRow(t)},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			orders := command.ColumnsForCommand(tc.path)
			if len(orders) == 0 {
				t.Fatalf("%s declares no column order", tc.path)
			}
			for _, order := range orders {
				for _, name := range order {
					if _, written := tc.record[name]; !written {
						t.Errorf("%s declares column %q; the producer writes %v", tc.path, name, sortedRecordKeys(tc.record))
					}
				}
			}
		})
	}
}

// bestPathFixture is one `show bgp rib best` answer, in the spelling
// bestResult and bestPathEnvelopeKey produce
// (internal/component/bgp/plugins/rib/rib_pipeline_best.go). "attributes"
// carries what enrichRouteMapFromEntry renders, next-hop among it
// (rib_attr_format.go).
const bestPathFixture = `{"best-path":[` +
	`{"family":"ipv4 unicast","prefix":"192.0.2.0/24","best-peer":"198.51.100.1",` +
	`"multipath-peers":["198.51.100.2"],"attributes":{"next-hop":"198.51.100.1","origin":"igp"}},` +
	`{"family":"ipv4 unicast","prefix":"203.0.113.0/24","best-peer":"198.51.100.2",` +
	`"multipath-peers":[],"attributes":{"next-hop":"198.51.100.2","origin":"igp"}}]}`

// bestPathRow answers the first row of bestPathFixture.
func bestPathRow(t *testing.T) map[string]any {
	t.Helper()

	envelope := decodeFixture(t, bestPathFixture)
	rows, isRows := envelope["best-path"].([]any)
	if !isRows || len(rows) == 0 {
		t.Fatalf("the best-path fixture carries no rows")
	}
	row, isRecord := rows[0].(map[string]any)
	if !isRecord {
		t.Fatalf("the first best-path row is %T, want a record", rows[0])
	}
	return row
}

// decodeFixture reads a fixture as an operator's chain reads an answer.
func decodeFixture(t *testing.T, payload string) map[string]any {
	t.Helper()

	var decoded map[string]any
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("fixture does not decode: %v", err)
	}
	return decoded
}

// sortedRecordKeys names a record's keys for a failure message.
func sortedRecordKeys(record map[string]any) []string {
	names := make([]string, 0, len(record))
	for name := range record {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
