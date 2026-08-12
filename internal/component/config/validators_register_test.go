// Design: docs/architecture/config/yang-config-design.md -- custom validator registration
//
// VALIDATES: every name a YANG leaf reaches through ze:validate is present in
// the central registry. An unregistered name is not an error at validation time.
// applyCustomValidators (config/yang/validator.go) skips a name the registry
// does not hold, and it reports no error. The leaf would then accept everything.
// CheckAllValidatorsRegistered catches that at startup. This test catches it at
// build time.
package config

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config/yang"
)

func TestISISHostnameValidatorRegistered(t *testing.T) {
	reg := yang.NewValidatorRegistry()
	RegisterValidators(reg)

	// The name the isis hostname leaf carries in ze-isis-conf.yang.
	for _, name := range []string{"isis-hostname", "isis-net", "isis-system-id"} {
		cv := reg.Get(name)
		if cv == nil {
			t.Fatalf("validator %q is not registered: a ze:validate leaf naming it accepts every value", name)
		}
		if cv.ValidateFn == nil {
			t.Errorf("validator %q has a nil ValidateFn: it cannot refuse anything", name)
		}
	}
}
