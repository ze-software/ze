// Design: docs/architecture/plugin/plugin-system.md -- Registration fields, the runtime list equals the declared list
// Related: registered.go -- ParseRows, Compare

package registered

import (
	"strings"
	"testing"

	repoinventory "github.com/ze-software/ze/internal/le/repo/inventory"
)

// TestParseRowsRefusesWhatIsNotAPluginList proves an empty or foreign payload
// never becomes an empty runtime list.
//
// VALIDATES: ParseRows refuses a payload that is not a JSON array, an empty
// array, the `{plugins: [...]}` envelope, a row with no name or no outcome,
// and a name listed twice; it accepts a well-formed list.
// PREVENTS: two empty lists comparing equal.
func TestParseRowsRefusesWhatIsNotAPluginList(t *testing.T) {
	refused := map[string]string{
		"not JSON":    `name description`,
		"empty array": `[]`,
		"envelope":    `{"plugins":[{"name":"a","outcome":"unknown"}]}`,
		"no name":     `[{"description":"d","outcome":"unknown"}]`,
		"no outcome":  `[{"name":"a"}]`,
		"duplicate":   `[{"name":"a","outcome":"unknown"},{"name":"a","outcome":"unknown"}]`,
	}
	for label, payload := range refused {
		if _, err := ParseRows([]byte(payload)); err == nil {
			t.Errorf("%s: ParseRows accepted %s", label, payload)
		}
	}

	rows, err := ParseRows([]byte(`[{"name":"a","description":"d","capabilities":[1,2],"outcome":"succeeded"}]`))
	if err != nil {
		t.Fatalf("ParseRows refused a plugin list: %v", err)
	}
	if len(rows) != 1 || rows[0].Name != "a" || len(rows[0].Capabilities) != 2 {
		t.Fatalf("ParseRows = %+v, want one row a with two capability codes", rows)
	}
}

// TestCompareNamesEveryDifference proves each kind of disagreement is named.
//
// VALIDATES: Compare accepts equal lists; it names a plugin registered and
// not declared, one declared and not registered, and a field that differs;
// it refuses an empty list on either side.
// PREVENTS: a difference passing because only the count was compared.
func TestCompareNamesEveryDifference(t *testing.T) {
	declared := []repoinventory.Plugin{
		{Name: "a", Description: "A", RFCs: []string{"4271"}, Capabilities: []uint8{1}, Families: []string{"ipv4/unicast"}},
		{Name: "b", Description: "B"},
	}
	equal := []Row{
		{Name: "a", Description: "A", RFCs: []string{"4271"}, Capabilities: []int{1}, Families: []string{"ipv4/unicast"}, Outcome: "unknown"},
		{Name: "b", Description: "B", Outcome: "unknown"},
	}
	if err := Compare(declared, equal); err != nil {
		t.Fatalf("Compare refused equal lists: %v", err)
	}

	differing := []Row{
		{Name: "a", Description: "A", RFCs: []string{"4271"}, Capabilities: []int{2}, Families: []string{"ipv4/unicast"}, Outcome: "unknown"},
		{Name: "c", Description: "C", Outcome: "unknown"},
	}
	err := Compare(declared, differing)
	if err == nil {
		t.Fatal("Compare accepted lists that differ")
	}
	for _, want := range []string{
		`plugin "a" capabilities: declared [1], registered [2]`,
		`plugin "b" is in the declared list and not registered at run time`,
		`plugin "c" is registered at run time and absent from the declared list`,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Compare error lacks %q:\n%v", want, err)
		}
	}

	if Compare(nil, equal) == nil {
		t.Error("Compare accepted an empty declared list")
	}
	if Compare(declared, nil) == nil {
		t.Error("Compare accepted an empty runtime list")
	}
}

// TestCompareNamesEachRegistrationField proves every field the two sides share
// is compared, one field at a time.
//
// VALIDATES: for each of config roots, dependencies, optional dependencies,
// start-after, features, yang, nlri-decoder and nlri-encoder, Compare accepts
// a row equal to the declared plugin and names that field, with both values,
// when only it differs.
// PREVENTS: registration metadata written by code the declared walk reads as
// opaque, or a field the runtime row drops, passing as equal.
func TestCompareNamesEachRegistrationField(t *testing.T) {
	declared := []repoinventory.Plugin{{
		Name: "a", Description: "A",
		ConfigRoots: []string{"bgp"}, Dependencies: []string{"b"},
		OptionalDependencies: []string{"c"}, StartAfter: []string{"d"},
		Features: "nlri yang", HasYANG: true, HasDecoder: true,
	}}
	base := Row{
		Name: "a", Description: "A",
		ConfigRoots: []string{"bgp"}, Dependencies: []string{"b"},
		OptionalDependencies: []string{"c"}, StartAfter: []string{"d"},
		Features: "nlri yang", YANG: true, NLRIDecoder: true, Outcome: "unknown",
	}
	if err := Compare(declared, []Row{base}); err != nil {
		t.Fatalf("Compare refused an equal row: %v", err)
	}

	cases := []struct {
		field string
		edit  func(*Row)
		want  string
	}{
		{"config-roots", func(r *Row) { r.ConfigRoots = nil }, `plugin "a" config-roots: declared [bgp], registered []`},
		{"dependencies", func(r *Row) { r.Dependencies = []string{"b", "e"} }, `plugin "a" dependencies: declared [b], registered [b e]`},
		{"optional-dependencies", func(r *Row) { r.OptionalDependencies = []string{"x"} }, `plugin "a" optional-dependencies: declared [c], registered [x]`},
		{"start-after", func(r *Row) { r.StartAfter = nil }, `plugin "a" start-after: declared [d], registered []`},
		{"features", func(r *Row) { r.Features = "nlri" }, `plugin "a" features: declared "nlri yang", registered "nlri"`},
		{"yang", func(r *Row) { r.YANG = false }, `plugin "a" yang: declared true, registered false`},
		{"nlri-decoder", func(r *Row) { r.NLRIDecoder = false }, `plugin "a" nlri-decoder: declared true, registered false`},
		{"nlri-encoder", func(r *Row) { r.NLRIEncoder = true }, `plugin "a" nlri-encoder: declared false, registered true`},
	}
	for _, tc := range cases {
		row := base
		tc.edit(&row)
		err := Compare(declared, []Row{row})
		if err == nil {
			t.Errorf("%s: Compare accepted a row whose %s differs", tc.field, tc.field)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: Compare error lacks %q:\n%v", tc.field, tc.want, err)
		}
		if strings.Count(err.Error(), "\n  ") != 1 {
			t.Errorf("%s: Compare named more than the one differing field:\n%v", tc.field, err)
		}
	}
}

// TestParseRowsReadsEveryRegistrationKey proves the row keys ParseRows reads
// are the keys `show plugin list` writes, so a renamed key reaches Compare as
// a difference rather than as a silent zero on both sides.
//
// VALIDATES: each kebab-case key PluginInfo writes lands in its Row field.
// PREVENTS: a misspelled JSON tag making a field compare empty with empty.
func TestParseRowsReadsEveryRegistrationKey(t *testing.T) {
	rows, err := ParseRows([]byte(`[{"name":"a","outcome":"unknown","config-roots":["bgp"],` +
		`"dependencies":["b"],"optional-dependencies":["c"],"start-after":["d"],` +
		`"features":"nlri","yang":true,"nlri-decoder":true,"nlri-encoder":true}]`))
	if err != nil {
		t.Fatalf("ParseRows refused a plugin list: %v", err)
	}
	row := rows[0]
	if len(row.ConfigRoots) != 1 || len(row.Dependencies) != 1 || len(row.OptionalDependencies) != 1 ||
		len(row.StartAfter) != 1 || row.Features != "nlri" || !row.YANG || !row.NLRIDecoder || !row.NLRIEncoder {
		t.Fatalf("ParseRows = %+v, want every registration key read", row)
	}
}
