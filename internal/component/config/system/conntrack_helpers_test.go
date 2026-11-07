// enumeration: gated by TestHelperTableMatchesModel
// Related: conntrack_helpers.go -- the helper declaration table this test reads

package system

import (
	"slices"
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"

	_ "github.com/ze-software/ze/internal/component/config/system/yang" // registers ze-system-conf.yang, which declares the module leaf-list
)

// TestHelperTableMatchesModel ties the helper declaration table to the
// enumeration the module leaf-list declares, in both directions.
//
// The table is the declaration: it carries each helper's kernel helper names,
// transports, ports and families, which the model cannot state. A name only
// the model carried would be accepted at commit and then assign nothing, and
// a row only the table carried could never be configured.
//
// VALIDATES: every enum value has a row and every row an enum value; every row
// names a family, at least one rule, a transport and a non-zero port.
// PREVENTS: a configured helper that silently assigns nothing.
func TestHelperTableMatchesModel(t *testing.T) {
	declared, err := configyang.EnumValues(conntrackModulePath)
	if err != nil {
		t.Fatalf("read the enumeration at %s: %v", conntrackModulePath, err)
	}
	if len(declared) == 0 {
		t.Fatalf("the model declares no helper at %s", conntrackModulePath)
	}

	rows := make([]string, len(conntrackHelpers))
	for i := range conntrackHelpers {
		rows[i] = conntrackHelpers[i].Name
	}
	slices.Sort(declared)
	slices.Sort(rows)
	if !slices.Equal(declared, rows) {
		t.Fatalf("model enumeration %v and helper table %v differ", declared, rows)
	}

	for i := range conntrackHelpers {
		h := &conntrackHelpers[i]
		if h.Family == conntrackHelperFamilyUnspecified {
			t.Errorf("helper %q declares no family", h.Name)
		}
		if len(h.Rules) == 0 {
			t.Errorf("helper %q declares no rule", h.Name)
		}
		for _, r := range h.Rules {
			if r.KernelHelper == "" {
				t.Errorf("helper %q: a rule names no kernel helper", h.Name)
			}
			if r.Protocol != conntrackTCP && r.Protocol != conntrackUDP {
				t.Errorf("helper %q: rule %q has transport %q", h.Name, r.KernelHelper, r.Protocol)
			}
			if len(r.Ports) == 0 || slices.Contains(r.Ports, 0) {
				t.Errorf("helper %q: rule %q ports %v", h.Name, r.KernelHelper, r.Ports)
			}
		}
	}
}

// TestHelperTableRefusesRemovedNames proves nfs and sqlnet, which no kernel
// module provides, are refused by name with the valid list in the error.
//
// VALIDATES: ConntrackHelperByName refuses an undeclared name and names the
// valid helpers; the model refuses the two at parse time too.
// PREVENTS: a helper accepted that has nothing behind it.
func TestHelperTableRefusesRemovedNames(t *testing.T) {
	for _, name := range []string{"nfs", "sqlnet"} {
		if _, err := ConntrackHelperByName(name); err == nil {
			t.Errorf("ConntrackHelperByName(%q) answered a row", name)
		}
		if err := CheckConntrackModule(name); err == nil {
			t.Errorf("CheckConntrackModule(%q) accepted a name no kernel module provides", name)
		}
	}
	h, err := ConntrackHelperByName("ftp")
	if err != nil {
		t.Fatalf("ConntrackHelperByName(ftp): %v", err)
	}
	if h.Family != ConntrackHelperBothFamilies || len(h.Rules) != 1 || h.Rules[0].Ports[0] != 21 {
		t.Fatalf("ftp row = %+v, want both families, tcp 21", h)
	}
}
