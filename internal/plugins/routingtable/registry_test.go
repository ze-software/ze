package routingtable

import (
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

func TestResolveDefault(t *testing.T) {
	r := New(nil)
	id, err := r.Resolve("default")
	if err != nil {
		t.Fatal(err)
	}
	if id != 0 {
		t.Errorf("Resolve(default) = %d, want 0", id)
	}
}

func TestResolveEmpty(t *testing.T) {
	r := New(nil)
	id, err := r.Resolve("")
	if err != nil {
		t.Fatal(err)
	}
	if id != 0 {
		t.Errorf("Resolve(\"\") = %d, want 0", id)
	}
}

func TestResolveNamed(t *testing.T) {
	r := New(map[string]uint32{"surfprotect": 100, "lns": 200})
	id, err := r.Resolve("surfprotect")
	if err != nil {
		t.Fatal(err)
	}
	if id != 100 {
		t.Errorf("Resolve(surfprotect) = %d, want 100", id)
	}

	id, err = r.Resolve("lns")
	if err != nil {
		t.Fatal(err)
	}
	if id != 200 {
		t.Errorf("Resolve(lns) = %d, want 200", id)
	}
}

func TestRejectUnknownName(t *testing.T) {
	r := New(map[string]uint32{"surfprotect": 100})
	_, err := r.Resolve("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown table name")
	}
}

func TestRejectReservedTableID(t *testing.T) {
	for _, id := range []uint32{0, 253, 254, 255} {
		_, err := ValidateTableID(id)
		if err == nil {
			t.Errorf("expected error for reserved table ID %d", id)
		}
	}
}

func TestAcceptValidTableID(t *testing.T) {
	for _, id := range []uint32{1, 100, 252, 256, 4294967295} {
		v, err := ValidateTableID(id)
		if err != nil {
			t.Errorf("unexpected error for table ID %d: %v", id, err)
		}
		if v != id {
			t.Errorf("ValidateTableID(%d) = %d", id, v)
		}
	}
}

func TestParseRoutingTableConfig(t *testing.T) {
	input := `{"routing-table":{"table":{"lns":{"id":100},"surfprotect":{"id":200}}}}`
	tables, err := parseRoutingTableConfig(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 2 {
		t.Fatalf("got %d tables, want 2", len(tables))
	}
	if tables["lns"] != 100 {
		t.Errorf("lns = %d, want 100", tables["lns"])
	}
	if tables["surfprotect"] != 200 {
		t.Errorf("surfprotect = %d, want 200", tables["surfprotect"])
	}
}

func TestParseRoutingTableConfigRejectsReserved(t *testing.T) {
	input := `{"routing-table":{"table":{"bad":{"id":254}}}}`
	_, err := parseRoutingTableConfig(input)
	if err == nil {
		t.Fatal("expected error for reserved table ID 254")
	}
}

func TestParseRoutingTableConfigRejectsDefault(t *testing.T) {
	input := `{"routing-table":{"table":{"default":{"id":100}}}}`
	_, err := parseRoutingTableConfig(input)
	if err == nil {
		t.Fatal("expected error for name 'default' (built-in)")
	}
}

func TestParseRoutingTableConfigEmpty(t *testing.T) {
	input := `{"routing-table":{}}`
	tables, err := parseRoutingTableConfig(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 0 {
		t.Errorf("got %d tables, want 0", len(tables))
	}
}

func TestParseRoutingTableConfigMissingID(t *testing.T) {
	input := `{"routing-table":{"table":{"lns":{}}}}`
	_, err := parseRoutingTableConfig(input)
	if err == nil {
		t.Fatal("expected error for missing id")
	}
}

func TestNilRegistryResolvesDefault(t *testing.T) {
	var r *Registry
	id, err := r.Resolve("default")
	if err != nil {
		t.Fatal(err)
	}
	if id != 0 {
		t.Errorf("nil registry Resolve(default) = %d, want 0", id)
	}
}

func TestNilRegistryRejectsNamed(t *testing.T) {
	var r *Registry
	_, err := r.Resolve("surfprotect")
	if err == nil {
		t.Fatal("nil registry should reject named table")
	}
}

func TestRoutingTableRegistration(t *testing.T) {
	reg := registry.Lookup("routing-table")
	if reg == nil {
		t.Fatal("routing-table plugin not registered")
	}
	if reg.Name != "routing-table" {
		t.Errorf("name = %q, want %q", reg.Name, "routing-table")
	}
	if reg.YANG == "" {
		t.Error("YANG schema not set")
	}
}
