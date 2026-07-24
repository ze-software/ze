// VALIDATES: the set-format loader fails closed on unknown fields that no
// migration heals: a set/set-meta config carrying a key this build's schema
// does not know (a feature compiled out, or an unmigratable rename) is
// REJECTED with the dropped keys named, never silently pruned.
// PREVENTS: the feature-gate-12 review BLOCKER -- a stripped build booting a
// full build's committed (set-meta) config would silently drop the gated
// blocks (tacacs/radius auth, l2tp, ike, ...) and fall back to permissive
// behavior (local-only auth) with no error, no warning: fail-open on the
// daemon's own persisted format, while the brace format rejected the same
// keys. Boot (LoadConfig) and validate (ParseTreeForValidation) share this
// producer (parseSetWithMigration), so both are covered.
package config

import (
	"strings"
	"testing"
)

func TestSetFormatRejectsUnknownFieldFailClosed(t *testing.T) {
	cases := map[string]string{
		"set-plain":               "set no-such-feature enabled true\n",
		"set-meta":                "#admin @cli set no-such-feature enabled true\n",
		"set-nested-known-parent": "set system authentication no-such-method server 192.0.2.1\n",
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ParseTreeForValidation(input)
			if err == nil {
				t.Fatal("set-format config with an unknown field parsed clean (silently pruned): want fail-closed rejection")
			}
			if !strings.Contains(err.Error(), "unknown field") {
				t.Fatalf("rejection = %v, want an error naming the unknown field", err)
			}
		})
	}
}

// TestSetFormatKnownConfigStillParses guards the other direction: the
// fail-closed check must not reject a valid set-format config.
func TestSetFormatKnownConfigStillParses(t *testing.T) {
	if _, err := ParseTreeForValidation("set environment log level warn\n"); err != nil {
		t.Fatalf("valid set-format config rejected: %v", err)
	}
}
