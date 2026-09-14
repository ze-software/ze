// Related: config.go -- Validate, which reads the unit from the firewall's table
// Related: match.go -- where the unit becomes a firewall.Limit

package shape

import (
	"slices"
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/firewall"

	// The blank imports register ze-anomaly-shape-conf.yang and the
	// ze-anomaly-detect-conf.yang it augments, so the limit-unit leaf resolves.
	_ "github.com/ze-software/ze/internal/plugins/anomaly/detect/yang"
	_ "github.com/ze-software/ze/internal/plugins/anomaly/shape/yang"
)

// TestLimitUnitsMatchTheModel ties the units the limit-unit leaf offers to the
// firewall's rate-unit table, which is what Validate reads and what every
// backend divides by.
//
// The firewall table is the declaration: it pairs each unit with its seconds.
// The model is the copy, and this test keeps it honest in both directions,
// through Validate so the surface an operator reaches is the one under test.
//
// VALIDATES: the enumeration at anomaly/shape/limit-unit and
// firewall.RateUnitNames are one set, and Validate accepts each declared unit.
// PREVENTS: a unit the schema accepts that the responder refuses at load, and a
// unit the firewall lowers that no operator can write.
func TestLimitUnitsMatchTheModel(t *testing.T) {
	const path = "anomaly/shape/limit-unit"

	declared, err := configyang.EnumValues(path)
	if err != nil {
		t.Fatalf("read the enumeration at %s: %v", path, err)
	}
	if len(declared) == 0 {
		t.Fatalf("the model declares no unit at %s", path)
	}
	if units := firewall.RateUnitNames(); !slices.Equal(declared, units) {
		t.Errorf("the model declares %v at %s, and the firewall lowers %v", declared, path, units)
	}

	for _, unit := range declared {
		cfg := DefaultConfig()
		cfg.LimitUnit = unit
		if err := cfg.Validate(); err != nil {
			t.Errorf("the model declares limit-unit %q and Validate refuses it: %v", unit, err)
		}
	}
}
