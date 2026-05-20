package routingtable

import "testing"

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

func TestGetSetRegistry(t *testing.T) {
	old := GetRegistry()
	defer SetRegistry(old)

	r := New(map[string]uint32{"test": 42})
	SetRegistry(r)

	got := GetRegistry()
	if got != r {
		t.Error("GetRegistry did not return the registry set by SetRegistry")
	}
	id, err := got.Resolve("test")
	if err != nil {
		t.Fatal(err)
	}
	if id != 42 {
		t.Errorf("Resolve(test) = %d, want 42", id)
	}
}
