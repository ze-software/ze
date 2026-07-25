package copp

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

func TestDoctorCheckCoppNilTree(t *testing.T) {
	ctx := registry.DoctorCheckContext{Tree: nil}
	diags := checkCoppInputChain(ctx)
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics for nil tree, got %d", len(diags))
	}
}

func TestDoctorCheckCoppNoConfig(t *testing.T) {
	tree := config.NewTree()
	ctx := registry.DoctorCheckContext{Tree: tree}
	diags := checkCoppInputChain(ctx)
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics without copp config, got %d", len(diags))
	}
}

func TestDoctorCheckCoppWithConfig(t *testing.T) {
	tree := config.NewTree()
	cpp := tree.GetOrCreateContainer("control-plane-protection")
	bgp := cpp.GetOrCreateContainer("bgp")
	bgp.Set("rate", "100/second")

	ctx := registry.DoctorCheckContext{Tree: tree}
	diags := checkCoppInputChain(ctx)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if diags[0].Code != "doctor-copp-missing" {
		t.Errorf("code = %q, want doctor-copp-missing", diags[0].Code)
	}
}
