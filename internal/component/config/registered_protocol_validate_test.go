// VALIDATES: spec-per-protocol-fib-import AC-5 -- `rib { fib-withhold [ ... ] }`
// refuses a name nothing registered, and the refusal NAMES the protocols that
// are registered, so the operator reads the vocabulary out of the error.
// PREVENTS: a withhold list that accepts a typo, which withholds nothing and
// says nothing, so the operator believes a protocol is out of the FIB while
// every one of its routes is still programmed.
package config

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/redistevents"

	// Register ze-rib-conf so ValidateTreeAllModules finds the section. The
	// live binary registers it through the generated all.go blank import.
	_ "github.com/ze-software/ze/internal/component/sysrib/yang"
)

// validateFIBWithhold walks the `rib` section the way `ze config validate` and
// LoadConfig both walk it, and answers what the walk found.
func validateFIBWithhold(t *testing.T, names ...string) []yang.ValidationError {
	t.Helper()
	loader := newTestLoader(t)
	reg := yang.NewValidatorRegistry()
	RegisterValidators(reg)
	reg.MergeGlobalCompletions()
	v := yang.NewValidator(loader)
	v.SetRegistry(reg)

	items := make([]any, 0, len(names))
	for _, name := range names {
		items = append(items, name)
	}
	return v.ValidateTreeAllModules("rib", map[string]any{"fib-withhold": items})
}

func TestRegisteredProtocolValidatorRefusesAnUnknownName(t *testing.T) {
	// Registration is idempotent on the name, so this is the protocol the valid
	// half of the test names and the one the error message must offer.
	redistevents.RegisterProtocol("bgp")

	if errs := validateFIBWithhold(t, "bgp"); len(errs) != 0 {
		t.Fatalf("a registered protocol was refused: %v", errs)
	}

	errs := validateFIBWithhold(t, "no-such-protocol")
	if len(errs) == 0 {
		t.Fatal("fib-withhold accepted a protocol nothing registered")
	}
	message := errs[0].Message
	if !strings.Contains(message, "no-such-protocol") {
		t.Errorf("the refusal does not name the value refused: %s", message)
	}
	// AC-5 is the second half: the operator learns the vocabulary from the
	// refusal rather than from a page.
	if !strings.Contains(message, "bgp") {
		t.Errorf("the refusal does not name the registered protocols: %s", message)
	}
}

// TestLoadConfigRefusesAnUnregisteredFIBWithhold drives AC-5 through the door an
// operator uses. The walk above proves the validator; this proves the walk
// REACHES it, which is a separate fact: ValidateCustomSections iterates
// validatedSections, and a section absent from that list carries annotations
// that never run and refuse nothing. Drop "rib" from validatedSections and this
// goes red while the test above stays green.
func TestLoadConfigRefusesAnUnregisteredFIBWithhold(t *testing.T) {
	redistevents.RegisterProtocol("bgp")

	const src = `rib {
	fib-withhold [ no-such-protocol ]
}
`
	_, err := LoadConfig(src, "test.conf", nil)
	if err == nil {
		t.Fatal("LoadConfig accepted a fib-withhold naming a protocol nothing registered")
	}
	message := err.Error()
	for _, want := range []string{"config validation failed", "rib", "no-such-protocol", "bgp"} {
		if !strings.Contains(message, want) {
			t.Errorf("the refusal does not name %q: %s", want, message)
		}
	}
}
