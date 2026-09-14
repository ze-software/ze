// Related: model.go -- the name tables this test reads

package traffic

import (
	"maps"
	"slices"
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"

	_ "github.com/ze-software/ze/internal/component/traffic/yang" // registers ze-traffic-control-conf.yang, which declares the two leaves
)

// TestParsedNamesMatchTheModel ties the names this package parses to the
// enumeration an operator writes each value into.
//
// The Go side is kept rather than derived: each name is paired with a typed
// constant the netlink and VPP backends lower to a kernel qdisc kind or a
// filter shape, which the model does not hold. That makes the model the copy,
// and this test keeps it honest in both directions. clsact and ingress are
// names the backend reads off an interface and no operator can ask for, so
// the parser leaves them out on purpose and the model must too.
//
// VALIDATES: the name set of each parse index equals the value set of the
// enumeration at its config leaf.
// PREVENTS: a qdisc or filter type the schema accepts and ParseQdiscType or
// ParseFilterType then refuses as unknown, and the reverse.
func TestParsedNamesMatchTheModel(t *testing.T) {
	cases := []struct {
		name   string
		path   string
		parsed []string
	}{
		{"qdisc type", "traffic/control/interface/qdisc/type", slices.Sorted(maps.Keys(qdiscTypeByName))},
		{"filter type", "traffic/control/interface/qdisc/class/match/type", slices.Sorted(maps.Keys(filterTypeByName))},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			declared, err := configyang.EnumValues(c.path)
			if err != nil {
				t.Fatalf("read the enumeration at %s: %v", c.path, err)
			}
			if len(declared) == 0 {
				t.Fatalf("the model declares no value at %s", c.path)
			}
			if !slices.Equal(declared, c.parsed) {
				t.Errorf("the model declares %v at %s, and this package parses %v", declared, c.path, c.parsed)
			}
		})
	}
}
