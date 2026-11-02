// Design: docs/architecture/core-design.md -- Policy Filter Chain, "The BGP route view"

package bgpconfig_test

import (
	"testing"

	_ "github.com/ze-software/ze/internal/component/bgp/config"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/policy"
)

// lateFieldType is a filter type only this test declares: it reads the field
// this package registers.
const lateFieldType = "test-late-field-reader"

// lateField is registered from an init() of this external test package, which
// Go initializes after bgpconfig, the package whose init() registers the BGP
// attachments: the order a field owner bgpconfig does not import (the role or
// gr plugin) meets.
var lateField = policy.MustRegisterField("test-late-field", policy.RouteView{})

func init() {
	policyview.MustRegisterReader(lateField, func(*policyview.View) (policyview.Value, error) {
		return policyview.Value{}, policyview.ErrNoReader
	})
	policy.MustRegisterFilterType(lateFieldType, []policy.Field{lateField}, nil)
}

// TestBGPAttachmentsCarryFieldRegisteredAfter proves the carried set is
// derived when a chain is checked, not copied when bgpconfig initializes:
// a field whose reader registers from an init() that runs after bgpconfig's
// is carried at bgp import and bgp export. Method: a chain naming a filter
// that reads only that field passes the field check at both attachments.
func TestBGPAttachmentsCarryFieldRegisteredAfter(t *testing.T) {
	reg, err := policy.NewRegistry(policy.Instance{Name: "READS-LATE", Type: lateFieldType})
	if err != nil {
		t.Fatal(err)
	}
	for _, attachment := range []string{"bgp import", "bgp export"} {
		if err := policy.CheckChain(attachment, reg, []string{"READS-LATE"}); err != nil {
			t.Errorf("%s refused a field registered after bgpconfig initialized: %v", attachment, err)
		}
	}
}
