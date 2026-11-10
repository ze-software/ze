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
