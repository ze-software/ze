// Design: docs/architecture/config/yang-config-design.md -- custom validator registration
//
// VALIDATES: the interface unit `vrf` leaf is refused rather than silently
// accepted. The leaf has no reader anywhere in internal/component/iface or
// internal/plugins/iface, so a config that set it used to commit and change
// nothing, and the operator kept traffic on the main routing table with no
// error. These tests drive the refusal from LoadConfig, the door both the
// daemon and `ze config validate` go through, not from the validator alone.
package config

import (
	"strings"
	"testing"
)

// TestLoadConfigRefusesTheUnimplementedVRFLeaf pins the refusal and its message.
// Remove the ze:validate binding from the leaf in ze-iface-conf.yang, or the
// registration in validators_register.go, and LoadConfig accepts the config
// again: that is the mutation this test exists to catch.
func TestLoadConfigRefusesTheUnimplementedVRFLeaf(t *testing.T) {
	const src = `interface {
	backend netlink;
	ethernet eth0 {
		unit 0 {
			vrf red;
		}
	}
}
`
	_, err := LoadConfig(src, "test.conf", nil)
	if err == nil {
		t.Fatal("LoadConfig accepted a vrf leaf no code reads: the config promises isolation ze never applies")
	}
	msg := err.Error()
	for _, want := range []string{
		"vrf",
		"not implemented",
		"main routing table",
		"not in force",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("refusal does not say %q: %s", want, msg)
		}
	}
	// The refusal must name a missing feature, never a bad value. An operator
	// told the value is wrong looks for a better value; there is none.
	for _, banned := range []string{"invalid", "is not a valid"} {
		if strings.Contains(msg, banned) {
			t.Errorf("refusal reads as a value error (%q), not as an unimplemented feature: %s", banned, msg)
		}
	}
}

// TestLoadConfigAcceptsAUnitWithoutVRF is the discrimination control: the same
// unit without the leaf still loads, so the test above fails because of the vrf
// leaf and not because the surrounding config is unacceptable.
func TestLoadConfigAcceptsAUnitWithoutVRF(t *testing.T) {
	const src = `interface {
	backend netlink;
	ethernet eth0 {
		unit 0 {
			description "no vrf here";
		}
	}
}
`
	if _, err := LoadConfig(src, "test.conf", nil); err != nil {
		t.Fatalf("LoadConfig refused a unit that sets no vrf leaf: %v", err)
	}
}
