//go:build linux

package iface

import (
	"strings"
	"testing"

	// Trigger YANG module registration so the reserved-keyword loader
	// has a populated command tree. Without these imports the YANG
	// loader has no -cmd modules and the reserved-name check is a no-op.
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/clear/schema"
	_ "codeberg.org/thomas-mangin/ze/internal/component/cmd/show/schema"
)

// TestValidateIfaceName_ReservedKeywords verifies that CLI reserved
// keywords are rejected as interface names so the grammar stays
// unambiguous.
//
// VALIDATES: user directive -- `ze clear interface counters` cannot be
// made ambiguous by an operator naming an interface `counters` (or
// any other show/clear keyword).
// PREVENTS: regression where a future CLI keyword is added to the
// show/clear interface tree without updating reservedIfaceNames.
func TestValidateIfaceName_ReservedKeywords(t *testing.T) {
	reserved := []string{"brief", "scan", "type", "errors", "counters"}
	for _, name := range reserved {
		err := ValidateIfaceName(name)
		if err == nil {
			t.Errorf("ValidateIfaceName(%q) = nil, want reserved-keyword error", name)
			continue
		}
		if !strings.Contains(err.Error(), "reserved CLI keyword") {
			t.Errorf("ValidateIfaceName(%q) = %v, want error mentioning %q", name, err, "reserved CLI keyword")
		}
	}
}

// TestValidateIfaceName_NormalNames verifies that legitimate interface
// names still pass after the reserved-keyword list was added.
//
// VALIDATES: the reservation list is a focused exclusion, not a
// blanket rejection of common-English interface names.
func TestValidateIfaceName_NormalNames(t *testing.T) {
	ok := []string{"eth0", "wan", "lan", "br0", "wg0", "vlan100", "mgmt-0"}
	for _, name := range ok {
		if err := ValidateIfaceName(name); err != nil {
			t.Errorf("ValidateIfaceName(%q) = %v, want nil", name, err)
		}
	}
}
