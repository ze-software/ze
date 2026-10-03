// Design: docs/architecture/config/yang-config-design.md -- Validation
// Related: validate_sections_test.go -- the custom-validator walk LoadConfig runs
//
// RFC 7950 Section 8 binds the running datastore: it MUST always be valid, and
// the constraints MUST be enforced in each window the section names. Ze holds
// one route into the running configuration, LoadConfig, which the daemon takes
// at start and again on every reload (cmd/ze/hub/main_reload.go::runReload).
// The cases here drive that route: a valid config becomes the tree the daemon
// serves, and a constraint violation in the parse window or in the whole-tree
// window is refused before any tree exists to serve.
package config

import (
	"errors"
	"strings"
	"testing"
)

// validRunningConfig passes every YANG type and every registered ze:validate
// rule. The interface section is in validatedSections and its mac leaf binds a
// custom validator, so the input goes through both windows, not around them.
const validRunningConfig = `interface {
	backend netlink;
	ethernet eth0 {
		mtu 1500;
		mac {
			address 02:00:00:00:00:02;
		}
	}
}
`

// RFC requirement: RFC7950-8.1-2 positive — a config every constraint accepts loads to the tree the daemon runs, so the running datastore receives a valid config
// RFC requirement: RFC7950-8.3-1 positive — a config valid in the parse window and in the whole-tree window loads; neither window refuses a conforming value
// RFC requirement: RFC7950-8.3.3-1 positive — when LoadConfig completes, the final tree carries the section it was given and obeys every constraint both windows checked.
func TestRFC7950RunningDatastoreAcceptsAValidConfig(t *testing.T) {
	result, err := LoadConfig(validRunningConfig, "test.conf", nil)
	if err != nil {
		t.Fatalf("LoadConfig refused a valid config: %v", err)
	}
	if result == nil || result.Tree == nil {
		t.Fatal("LoadConfig returned no tree for a valid config")
	}
	iface := result.Tree.GetContainer("interface")
	if iface == nil {
		t.Fatal("the interface section did not reach the running tree")
	}
	if got := iface.ToMap(); got["ethernet"] == nil {
		t.Errorf("the ethernet list did not reach the running tree: %v", got)
	}
}

// RFC requirement: RFC7950-8.3-1 negative — a value outside its YANG range (mtu 20 against "68..16000") is refused in the parse window: LoadConfig returns no tree and the error names the range.
func TestRFC7950RunningDatastoreRefusesAViolationAtParse(t *testing.T) {
	const src = `interface {
	backend netlink;
	ethernet eth0 {
		mtu 20;
	}
}
`
	result, err := LoadConfig(src, "test.conf", nil)
	if err == nil {
		t.Fatal("LoadConfig accepted an mtu below the YANG range")
	}
	if result != nil {
		t.Errorf("LoadConfig returned a tree beside the refusal: %+v", result)
	}
	msg := err.Error()
	for _, want := range []string{"mtu", "20"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error does not name %q: %s", want, msg)
		}
	}
}

// RFC requirement: RFC7950-8.1-2 negative — a config a registered ze:validate rule refuses never becomes the running tree: LoadConfig returns ErrCustomValidation and no tree
// RFC requirement: RFC7950-8.3-1 negative — a value the parser accepts but a ze:validate rule refuses is caught in the whole-tree window after parsing, so a violation the first window cannot see is still refused
// RFC requirement: RFC7950-8.3.3-1 negative — a tree that violates a constraint at the end of processing is refused as a whole; no partial tree is returned beside the error.
func TestRFC7950RunningDatastoreRefusesAViolationAfterParse(t *testing.T) {
	// `plugin/internal/use` is `type string { length "1..64" }`, so this name
	// passes the parse window and only InternalPluginNameValidator refuses it.
	const src = `plugin {
	internal p1 {
		use no-such-plugin-here
	}
}
`
	result, err := LoadConfig(src, "test.conf", nil)
	if err == nil {
		t.Fatal("LoadConfig accepted a config the internal-plugin-name validator refuses")
	}
	if !errors.Is(err, ErrCustomValidation) {
		t.Errorf("refusal is not ErrCustomValidation: %v", err)
	}
	if result != nil {
		t.Errorf("LoadConfig returned a tree beside the refusal: %+v", result)
	}
	if !strings.Contains(err.Error(), "no-such-plugin-here") {
		t.Errorf("error does not name the refused value: %v", err)
	}
}
