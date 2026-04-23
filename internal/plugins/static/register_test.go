package static

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

func TestStaticRouteRegistration(t *testing.T) {
	reg := registry.Lookup("static")
	if reg == nil {
		t.Fatal("static plugin not registered")
	}
	if reg.Name != "static" {
		t.Errorf("name = %q, want %q", reg.Name, "static")
	}
	if len(reg.ConfigRoots) != 1 || reg.ConfigRoots[0] != "static" {
		t.Errorf("config roots = %v, want [static]", reg.ConfigRoots)
	}
	if reg.YANG == "" {
		t.Error("YANG schema is empty")
	}
}
