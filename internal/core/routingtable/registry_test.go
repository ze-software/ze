package routingtable

import (
	"math"
	"testing"
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
		if !tableIDFitsInInt(id) {
			continue // covered by TestRejectUnencodableTableID on this build
		}
		v, err := ValidateTableID(id)
		if err != nil {
			t.Errorf("unexpected error for table ID %d: %v", id, err)
		}
		if v != id {
			t.Errorf("ValidateTableID(%d) = %d", id, v)
		}
	}
}

// tableIDFitsInInt reports whether id survives the conversion to Go int that
// the netlink bindings perform. On the 64-bit targets Ze builds it is always
// true; it is false on a 32-bit build for ids above MaxInt32.
func tableIDFitsInInt(id uint32) bool { return uint64(id) <= uint64(math.MaxInt) }

// VALIDATES: ValidateTableID never accepts an ID that cannot be carried by the
// int-typed netlink.Rule.Table / netlink.Route.Table fields, and every accepted
// ID survives that conversion unchanged.
// PREVENTS: an out-of-range table ID silently losing its table selection. The
// netlink rule encoder emits FRA_TABLE only when Table >= 256 and the compat
// byte only when 0 <= Table < 256 (vendor/github.com/vishvananda/netlink/
// rule_linux.go:57,126), so a negative Table produces a rule with
// RT_TABLE_UNSPEC; the route encoder guards on Table > 0 (route_linux.go:1058)
// and leaves the RtMsg default RT_TABLE_MAIN (nl/route_linux.go:16), silently
// installing an isolated route in the main table.
func TestRejectUnencodableTableID(t *testing.T) {
	// The bound is passed explicitly so this exercises the 32-bit rejection on
	// a 64-bit host, where maxEncodableTableID is above every uint32.
	const maxInt32 = uint64(math.MaxInt32)

	tests := []struct {
		name       string
		id         uint32
		maxEncode  uint64
		wantReject bool
	}{
		{"below bound", 1000, maxInt32, false},
		{"at bound", math.MaxInt32, maxInt32, false},
		{"one above bound", math.MaxInt32 + 1, maxInt32, true},
		{"max uint32 on 32-bit", math.MaxUint32, maxInt32, true},
		{"max uint32 on 64-bit", math.MaxUint32, uint64(math.MaxInt), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateTableID(tt.id, tt.maxEncode)
			if tt.wantReject {
				if err == nil {
					t.Fatalf("validateTableID(%d, %d): accepted an ID that does not fit in int", tt.id, tt.maxEncode)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateTableID(%d, %d): unexpected error: %v", tt.id, tt.maxEncode, err)
			}
			if got != tt.id {
				t.Fatalf("validateTableID(%d, %d) = %d", tt.id, tt.maxEncode, got)
			}
		})
	}
}

// VALIDATES: the bound ValidateTableID applies is this build's own int limit,
// so every ID it accepts survives the netlink int conversion unchanged.
// PREVENTS: the exported entry point drifting from the tested inner bound, for
// example by being wired to a hardcoded 32-bit constant that would reject
// kernel-legal table IDs on the 64-bit targets Ze ships.
func TestValidateTableIDUsesBuildIntBound(t *testing.T) {
	for _, id := range []uint32{1, 256, 1000, 1 << 31, math.MaxUint32} {
		got, err := ValidateTableID(id)
		if fits := tableIDFitsInInt(id); fits != (err == nil) {
			t.Errorf("ValidateTableID(%d): fits in int = %v, accepted = %v", id, fits, err == nil)
			continue
		}
		if err == nil && uint32(int(got)) != got {
			t.Errorf("ValidateTableID(%d): accepted an ID that does not survive int conversion", id)
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
