package geodns

import (
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestDoctorCodesRegistered reads the doctor checks this plugin's init()
// registered and looks up every code they declare.
//
// The codes come from the registration rather than from a list here, so a check
// added, removed, or given another code is judged without an edit to this file.
//
// VALIDATES: `ze explain <code>` resolves every code this plugin's readiness
// checks report.
// PREVENTS: a diagnostic an operator cannot look up.
func TestDoctorCodesRegistered(t *testing.T) {
	diagnostic.RegisterBuiltinCodes()

	checks, codes := 0, 0
	for _, check := range registry.PluginDoctorChecks() {
		if check.PluginName != "geodns" {
			continue
		}
		checks++
		for _, code := range check.Codes {
			codes++
			if diagnostic.Lookup(code) == nil {
				t.Errorf("doctor check %q declares code %q, which the code registry does not hold", check.Name, code)
			}
		}
	}
	if checks == 0 || codes == 0 {
		t.Fatalf("the geodns registration answered %d doctor checks and %d codes, so nothing was judged", checks, codes)
	}
}
