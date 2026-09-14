// Design: config.go -- validAddressFamilies is the one Go declaration of the families
//
// Goal: prove the address families an operator can write at
// service/as112/address-family are the families parseConfig accepts and the
// engine acts on, so a word cannot exist on one side alone. Method: read the
// enumeration out of the loaded model with configyang.EnumValues, which fails
// on a leaf that declares no enumeration, hold it to the table in both
// directions, and drive parseConfig with each word through the JSON the hub
// sends.

package as112

import (
	"slices"
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

const addressFamilyLeaf = "service/as112/address-family"

// TestAddressFamiliesMatchTheModel holds validAddressFamilies to the model.
func TestAddressFamiliesMatchTheModel(t *testing.T) {
	model, err := configyang.EnumValues(addressFamilyLeaf)
	if err != nil {
		t.Fatalf("read the enumeration at %s: %v", addressFamilyLeaf, err)
	}

	if !slices.Equal(model, addressFamilyNames()) {
		t.Errorf("the address families disagree: the model at %s holds %v and config.go accepts %v. "+
			"A word only the model carries is refused at parse time, and a word only Go carries is one no operator can ask for",
			addressFamilyLeaf, model, addressFamilyNames())
	}

	for _, family := range model {
		cfg, err := parseConfig(`{"service":{"as112":{"enabled":"true","address-family":"` + family + `"}}}`)
		if err != nil {
			t.Errorf("%q is offered at %s and parseConfig refuses it: %v", family, addressFamilyLeaf, err)
			continue
		}
		if cfg.AddressFamily != family {
			t.Errorf("%q parsed as %q", family, cfg.AddressFamily)
		}
		if len(hostAddresses(family)) == 0 {
			t.Errorf("%q registers no anycast address, so the word selects nothing", family)
		}
	}
}
