// Design: collect_linux.go -- tableFamilyName names a kernel table family for a reader
//
// Goal: prove the family names the support bundle writes are the names an
// operator writes at firewall/table/family, so a bundle never spells a family
// the configuration cannot. Method: read the enumeration out of the loaded
// model with configyang.EnumValues, which fails on a leaf that declares no
// enumeration, and hold it to the name of each kernel family in both
// directions. The kernel constant is what Go carries and the model cannot, so
// the two are gated against each other rather than derived
// (ai/rules/principles.md).

//go:build linux

package support

import (
	"slices"
	"testing"

	"github.com/google/nftables"

	configyang "github.com/ze-software/ze/internal/component/config/yang"

	// The blank import registers ze-firewall-conf with the loader, which
	// declares the leaf read below.
	_ "github.com/ze-software/ze/internal/component/firewall/yang"
)

const tableFamilyLeaf = "firewall/table/family"

// TestTableFamilyNamesMatchTheModel holds the six kernel families to the model.
func TestTableFamilyNamesMatchTheModel(t *testing.T) {
	model, err := configyang.EnumValues(tableFamilyLeaf)
	if err != nil {
		t.Fatalf("read the enumeration at %s: %v", tableFamilyLeaf, err)
	}

	kernel := []nftables.TableFamily{
		nftables.TableFamilyINet,
		nftables.TableFamilyIPv4,
		nftables.TableFamilyIPv6,
		nftables.TableFamilyARP,
		nftables.TableFamilyBridge,
		nftables.TableFamilyNetdev,
	}
	named := make([]string, 0, len(kernel))
	for _, family := range kernel {
		named = append(named, tableFamilyName(family))
	}
	slices.Sort(named)

	if !slices.Equal(model, named) {
		t.Errorf("the table families disagree: the model at %s holds %v and tableFamilyName writes %v. "+
			"A bundle then names a family no configuration can, or a configured family reads as unknown",
			tableFamilyLeaf, model, named)
	}
}
