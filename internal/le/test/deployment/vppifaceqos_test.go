// Design: docs/architecture/testing/interop.md -- real VPP VLAN QoS reload proof

package testdeployment

import (
	"strings"
	"testing"
)

// A retained key 5 must differ from the replacement row; malformed CLI output
// can never stand in for an empty map.
func TestVLANQoSRowRequiresCompleteReadback(t *testing.T) {
	values := strings.Split(strings.Repeat("0,", 255)+"0", ",")
	values[3], values[5] = "3", "5"
	initial := " Map-ID:7\n vlan:[" + strings.Join(values, ",") + "]\n"
	row, err := qosVLANRow(initial)
	if err != nil {
		t.Fatal(err)
	}
	if row[3] != 3 || row[5] != 5 {
		t.Fatalf("wrong installed row: %v", row)
	}
	values[5] = "0"
	replaced := " vlan:[" + strings.Join(values, ",") + "]\n"
	changed, err := qosVLANRow(replaced)
	if err != nil {
		t.Fatal(err)
	}
	if changed == row {
		t.Fatal("stale key 5 indistinguishable from replacement")
	}
	var expected [256]byte
	expected[3] = 3
	if changed != expected {
		t.Fatal("replacement contains an unexpected mapping")
	}
	for _, bad := range []string{"", "unknown input", "vlan:[0,3]", "vlan:" + strings.Join(values, ","), initial + initial, strings.Replace(initial, "3,", "256,", 1)} {
		if _, err := qosVLANRow(bad); err == nil {
			t.Errorf("accepted malformed row %q", bad)
		}
	}
}

// A missing or ambiguous interface cannot prove mark removal, and an unrelated
// source's mark cannot prove that VLAN rewriting is installed.
func TestVLANQoSInterfaceAndMarkReadback(t *testing.T) {
	name, index, err := qosVLANInterface("Name Idx State\nloop2.100 7 up\nloop2 6 up\n")
	if err != nil {
		t.Fatal(err)
	}
	if name != "loop2.100" || index != 7 {
		t.Fatalf("got %s/%d", name, index)
	}
	for _, bad := range []string{"", "loop2 6 up", "loop2.100 nope up", "loop2.100 7 up\nloop3.100 8 up"} {
		if _, _, err := qosVLANInterface(bad); err == nil {
			t.Errorf("accepted interface table %q", bad)
		}
	}
	if !qosVLANMarked(" loop2.100:\n  vlan: map:0\n", name) {
		t.Fatal("valid VLAN mark rejected")
	}
	for _, bad := range []string{"", "unknown input", "loop3.100:\n vlan: map:0", "loop2.100:\n ip: map:0", "loop2.100:\n vlan: map:unknown", "loop2.100:\n vlan: map:1"} {
		if qosVLANMarked(bad, name) {
			t.Errorf("accepted wrong mark %q", bad)
		}
	}
}

// The external row and internal mark must describe the same sole allocation.
func TestVLANQoSRequiresTheOnlyMap(t *testing.T) {
	if err := qosOnlyMap(" Map-ID:7\n vlan:[0]\n", 7); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", "Map-ID:8", "Map-ID:7\nMap-ID:8", "Map-ID:7\nMap-ID:7", "Map-ID:unknown"} {
		if err := qosOnlyMap(bad, 7); err == nil {
			t.Errorf("accepted ambiguous map binding %q", bad)
		}
	}
}
