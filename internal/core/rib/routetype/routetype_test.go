// VALIDATES: the forwarding-action vocabulary the FIB backends map to kernel
// and VPP actions, and the unset/unicast distinction the carry-through relies on.
// PREVENTS: a renumbering that silently installs the wrong forwarding action,
// and a zero value that reads as a deliberate unicast stamp.

package routetype

import "testing"

// TestValuesMatchLinuxRTN pins the numeric values. The kernel FIB backend maps
// them to RTN_ constants without a table, so a renumbering here would silently
// install the wrong forwarding action.
func TestValuesMatchLinuxRTN(t *testing.T) {
	cases := []struct {
		typ  Type
		want uint8
	}{
		{Unicast, 1},
		{Blackhole, 6},
		{Unreachable, 7},
		{Prohibit, 8},
	}
	for _, c := range cases {
		if uint8(c.typ) != c.want {
			t.Errorf("%v = %d, want %d", c.typ, uint8(c.typ), c.want)
		}
	}
}

// TestZeroIsUnset proves the zero value is distinguishable from Unicast. A
// producer that says nothing must not be read as "this is a unicast route",
// because that reading would make an unstamped route indistinguishable from a
// deliberately stamped one.
func TestZeroIsUnset(t *testing.T) {
	var unset Type
	if unset == Unicast {
		t.Fatal("zero Type equals Unicast: an unstamped route is indistinguishable from a stamped one")
	}
	if got := unset.String(); got != "unset" {
		t.Errorf("zero String() = %q, want %q", got, "unset")
	}
	if unset.Discards() {
		t.Error("zero Type discards; an unstamped route must forward")
	}
}

func TestDiscards(t *testing.T) {
	discarding := []Type{Blackhole, Unreachable, Prohibit}
	for _, typ := range discarding {
		if !typ.Discards() {
			t.Errorf("%v.Discards() = false, want true", typ)
		}
	}
	if Unicast.Discards() {
		t.Error("Unicast.Discards() = true, want false")
	}
}

func TestString(t *testing.T) {
	cases := map[Type]string{
		Unicast:     "unicast",
		Blackhole:   "blackhole",
		Unreachable: "unreachable",
		Prohibit:    "prohibit",
		Type(200):   "unset",
	}
	for typ, want := range cases {
		if got := typ.String(); got != want {
			t.Errorf("Type(%d).String() = %q, want %q", uint8(typ), got, want)
		}
	}
}
