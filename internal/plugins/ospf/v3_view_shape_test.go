// Design: docs/architecture/api/commands.md -- a plugin declares its own answer shape
// Related: register.go -- commandDecls, the declarations under test
// Related: debug_wiring.go -- v3Views, the producers whose answers are rendered

package ospf

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestOSPFv3ViewDeclsMatchV3Views checks that the per-address-family views commandDecls
// lists by hand are exactly the views v3Views serves, each declared as a table led by the
// instance identity.
//
// Method: commandDecls names each view a second time, as a literal `./le cli dispatch
// check` can read, so v3Views (debug_wiring.go) is the source and this compares the copy
// with it in both directions. Every v3Views key must have exactly one declaration, of shape
// table, whose columns start `address-family`, `instance-id`. Every declaration whose
// columns name both identity keys must be a v3Views key or one of the two Router
// Information views, which declare `af` first. No two declarations may share a columns
// slice, because the engine keeps the slice each declaration hands it.
func TestOSPFv3ViewDeclsMatchV3Views(t *testing.T) {
	decls := commandDecls()
	byName := make(map[string]rpc.CommandDecl, len(decls))
	owners := map[*string]string{}
	for _, decl := range decls {
		if _, seen := byName[decl.Name]; seen {
			t.Errorf("%q is declared twice", decl.Name)
		}
		byName[decl.Name] = decl
		if len(decl.Columns) == 0 {
			continue
		}
		if other, shared := owners[&decl.Columns[0]]; shared {
			t.Errorf("%q and %q share one columns slice", decl.Name, other)
		}
		owners[&decl.Columns[0]] = decl.Name
	}

	lead := []string{keywordAddressFamily, fieldInstanceID}
	for name := range v3Views {
		decl, declared := byName[name]
		if !declared {
			t.Errorf("v3Views serves %q, which commandDecls does not declare", name)
			continue
		}
		if decl.Shape != rpc.AnswerTypeTable {
			t.Errorf("%q declares shape %q, want %q", name, decl.Shape, rpc.AnswerTypeTable)
		}
		if len(decl.Columns) < len(lead) || !slices.Equal(decl.Columns[:len(lead)], lead) {
			t.Errorf("%q declares columns %v, want them to start %v", name, decl.Columns, lead)
		}
	}

	riViews := []string{cmdShowDatabaseRI, cmdShowIPv6DatabaseRI}
	riColumns := []string{"af", keywordAddressFamily, fieldInstanceID}
	for _, name := range riViews {
		decl, declared := byName[name]
		if !declared {
			t.Errorf("commandDecls does not declare the Router Information view %q", name)
			continue
		}
		if decl.Shape != rpc.AnswerTypeTable {
			t.Errorf("%q declares shape %q, want %q", name, decl.Shape, rpc.AnswerTypeTable)
		}
		if !slices.Equal(decl.Columns, riColumns) {
			t.Errorf("%q declares columns %v, want %v", name, decl.Columns, riColumns)
		}
	}

	for _, decl := range decls {
		if !slices.Contains(decl.Columns, keywordAddressFamily) {
			continue
		}
		if !slices.Contains(decl.Columns, fieldInstanceID) {
			continue
		}
		if _, served := v3Views[decl.Name]; served {
			continue
		}
		if slices.Contains(riViews, decl.Name) {
			continue
		}
		t.Errorf("%q declares the instance columns %v but is not a v3Views view", decl.Name, decl.Columns)
	}
}

// ospfShapeOwner is the owner the test declares this plugin's answer shapes under.
const ospfShapeOwner = "ospf-shape-test"

// declareOSPFShapes registers this plugin's answer shapes the way Stage 1 does
// (registerPluginShapes, internal/component/plugin/server/startup.go), so a chain renders
// against what the plugin itself declares, and withdraws them when the test ends.
func declareOSPFShapes(t *testing.T) {
	t.Helper()
	decls := commandDecls()
	declared := make([]command.PluginShape, 0, len(decls))
	for i := range decls {
		if decls[i].Shape == "" {
			continue
		}
		shape, known := command.ParseAnswerShape(decls[i].Shape)
		if !known {
			t.Fatalf("%s declares shape %q, which Stage 1 refuses", decls[i].Name, decls[i].Shape)
		}
		declared = append(declared, command.PluginShape{
			Command:       decls[i].Name,
			Shape:         shape,
			Columns:       command.ColumnOrder(decls[i].Columns),
			AddressFields: decls[i].AddressFields,
		})
	}
	if err := command.RegisterPluginShapes(ospfShapeOwner, declared); err != nil {
		t.Fatalf("register the OSPF answer shapes: %v", err)
	}
	t.Cleanup(func() { command.UnregisterPluginShapes(ospfShapeOwner) })
}

// renderV3View runs chain over payload the way the CLI runs it: the declaration decides
// whether the chain is admitted, and the renderer reads the declared columns.
func renderV3View(t *testing.T, chain string, payload []byte) string {
	t.Helper()
	_, format, refusal := command.ProcessPipesDefaultFormatChecked(chain, "")
	if refusal != "" {
		t.Fatalf("%s was refused: %s", chain, refusal)
	}
	return format(string(payload))
}

// tableRowCells answers the cells of the header and of each row of a `| table` rendering,
// header first. A row whose cell holds a nested table spans several lines, and only its
// first line names the row's leading cell, so a line whose first cell is blank continues the
// row above it and is skipped.
func tableRowCells(table string) [][]string {
	var rows [][]string
	for line := range strings.SplitSeq(table, "\n") {
		if !strings.HasPrefix(line, "│") {
			continue
		}
		parts := strings.Split(line, "│")
		cells := make([]string, 0, len(parts))
		for _, part := range parts[1 : len(parts)-1] {
			cells = append(cells, strings.TrimSpace(part))
		}
		if len(cells) == 0 {
			continue
		}
		if cells[0] == "" {
			continue
		}
		rows = append(rows, cells)
	}
	return rows
}

// TestOSPFv3ViewTableLeadsWithTheInstance checks that `| table` over a two-family answer
// puts `address-family` and `instance-id` in the first two columns and renders the same rows
// as `| json` (AC-10).
//
// Method: two AF engines (ipv6-unicast at Instance ID 0, ipv4-unicast at Instance ID 64)
// compute a route each, and `show ospf ipv6 route` is answered through its v3Views producer
// and rendered by the CLI's pipe chain under the plugin's own declarations. The route row's
// keys are area, metric, next-hops, origin, prefix and type, so a table rendered without the
// declared columns sorts `area` between the two identity keys; the header must read
// address-family, instance-id, area. Each table row must then carry, in order, the family,
// Instance ID and prefix of the matching `| json` row, so neither rendering lost or
// reordered a row of either family.
func TestOSPFv3ViewTableLeadsWithTheInstance(t *testing.T) {
	declareOSPFShapes(t)
	synctest.Test(t, func(t *testing.T) {
		set := twoAFViewSet(t)
		answer, err := v3Views[cmdShowIPv6Route](set, nil, cmdShowIPv6Route, nil)
		if err != nil {
			t.Fatalf("%s: %v", cmdShowIPv6Route, err)
		}
		payload, err := json.Marshal(answer)
		if err != nil {
			t.Fatalf("%s: marshal: %v", cmdShowIPv6Route, err)
		}

		rendered := renderV3View(t, cmdShowIPv6Route+" | json", payload)
		dec := json.NewDecoder(strings.NewReader(rendered))
		dec.UseNumber()
		var jsonRows []map[string]any
		if err := dec.Decode(&jsonRows); err != nil {
			t.Fatalf("`| json` is not a list of rows: %v\n%s", err, rendered)
		}
		families := map[any]bool{}
		for _, row := range jsonRows {
			families[row[keywordAddressFamily]] = true
		}
		if !families[afNameIPv6Unicast] || !families[afNameIPv4Unicast] {
			t.Fatalf("`| json` holds rows of %v, want rows of both running families:\n%s", families, rendered)
		}

		table := renderV3View(t, cmdShowIPv6Route+" | table", payload)
		rows := tableRowCells(table)
		if len(rows) == 0 {
			t.Fatalf("`| table` rendered no header:\n%s", table)
		}
		header := rows[0]
		lead := []string{keywordAddressFamily, fieldInstanceID, "area"}
		if len(header) < len(lead) {
			t.Fatalf("`| table` header %v, want it to start %v:\n%s", header, lead, table)
		}
		for i, name := range lead {
			if header[i] != name {
				t.Fatalf("`| table` header %v, want it to start %v:\n%s", header, lead, table)
			}
		}
		prefixColumn := -1
		for i, name := range header {
			if name == "prefix" {
				prefixColumn = i
			}
		}
		if prefixColumn < 0 {
			t.Fatalf("`| table` header %v has no prefix column:\n%s", header, table)
		}

		data := rows[1:]
		if len(data) != len(jsonRows) {
			t.Fatalf("`| table` renders %d rows and `| json` %d:\n%s\n%s", len(data), len(jsonRows), table, rendered)
		}
		for i, row := range data {
			want := []string{
				jsonString(jsonRows[i][keywordAddressFamily]),
				jsonString(jsonRows[i][fieldInstanceID]),
				jsonString(jsonRows[i]["prefix"]),
			}
			got := []string{row[0], row[1], row[prefixColumn]}
			for j := range want {
				if got[j] != want[j] {
					t.Fatalf("table row %d is %v, want the json row's %v:\n%s\n%s", i, got, want, table, rendered)
				}
			}
		}
	})
}

// jsonString spells a decoded JSON scalar as a table cell spells it.
func jsonString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	default:
		return ""
	}
}
