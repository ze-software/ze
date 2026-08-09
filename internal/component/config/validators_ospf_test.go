// Design: docs/architecture/ospf/ospf-4-component-config.md -- OSPF custom validators
//
// VALIDATES: ospf-router-id accepts only dotted-quad IPv4 router IDs, and
// ospf-area-id accepts dotted-quad or decimal uint32 area identifiers.
package config

import "testing"

func TestOSPFRouterIDValidator(t *testing.T) {
	v := ospfRouterIDValidator()
	if v.ValidateFn == nil {
		t.Fatal("OSPFRouterIDValidator has no ValidateFn")
	}
	for _, s := range []string{"10.0.0.1", "255.255.255.255"} {
		if err := v.ValidateFn("ospf/router-id", s); err != nil {
			t.Errorf("OSPFRouterIDValidator(%q) = %v, want nil", s, err)
		}
	}
	for name, value := range map[string]any{
		"not dotted": "router-1",
		"zero":       "0.0.0.0",
		"bad octet":  "256.0.0.1",
		"ipv6":       "2001:db8::1",
		"non-string": 42,
	} {
		t.Run(name, func(t *testing.T) {
			if err := v.ValidateFn("ospf/router-id", value); err == nil {
				t.Fatalf("OSPFRouterIDValidator(%v) = nil, want error", value)
			}
		})
	}
}

func TestOSPFAreaIDValidator(t *testing.T) {
	v := ospfAreaIDValidator()
	if v.ValidateFn == nil {
		t.Fatal("OSPFAreaIDValidator has no ValidateFn")
	}
	for _, s := range []string{"0", "1", "4294967295", "0.0.0.0", "255.255.255.255"} {
		if err := v.ValidateFn("ospf/areas/area/area-id", s); err != nil {
			t.Errorf("OSPFAreaIDValidator(%q) = %v, want nil", s, err)
		}
	}
	for name, value := range map[string]any{
		"empty":      "",
		"overflow":   "4294967296",
		"negative":   "-1",
		"bad dotted": "300.0.0.1",
		"ipv6":       "2001:db8::1",
		"non-string": 42,
	} {
		t.Run(name, func(t *testing.T) {
			if err := v.ValidateFn("ospf/areas/area/area-id", value); err == nil {
				t.Fatalf("OSPFAreaIDValidator(%v) = nil, want error", value)
			}
		})
	}
}
