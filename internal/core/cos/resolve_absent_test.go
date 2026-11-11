// Design: docs/architecture/traffic/cos-plugin.md -- a requested profile needs its resolver.

package cos

import (
	"strings"
	"testing"
)

// TestResolveAbsentProviderRefusesUnitProfile proves a unit override cannot
// disappear when the class-of-service plugin is absent.
func TestResolveAbsentProviderRefusesUnitProfile(t *testing.T) {
	mu.Lock()
	held := resolver
	resolver = nil
	mu.Unlock()
	t.Cleanup(func() { RegisterResolver(held) })
	ingress, egress, err := Resolve(nil, "none", "business", false)
	if err == nil {
		t.Fatal("unit profile accepted without its resolver")
	}
	if !strings.Contains(err.Error(), "business") {
		t.Fatalf("refusal does not name the requested profile: %v", err)
	}
	if ingress != nil || egress != nil {
		t.Fatal("refused profile returned maps")
	}
}

// TestResolveAbsentProviderPermitsNoProfile proves inline maps and explicit
// opt-outs need no profile resolver, including an override of a parent profile.
func TestResolveAbsentProviderPermitsNoProfile(t *testing.T) {
	mu.Lock()
	held := resolver
	resolver = nil
	mu.Unlock()
	t.Cleanup(func() { RegisterResolver(held) })
	for _, tc := range []struct {
		name   string
		parent string
		unit   string
	}{
		{name: "no reference"},
		{name: "parent opt-out", parent: "none"},
		{name: "unit opt-out", parent: "residential", unit: "none"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ingress, egress, err := Resolve(nil, tc.parent, tc.unit, true)
			if err != nil {
				t.Fatal(err)
			}
			if ingress != nil || egress != nil {
				t.Fatal("no profile requested, but resolver supplied maps")
			}
		})
	}
}
