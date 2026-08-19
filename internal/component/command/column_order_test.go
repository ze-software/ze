package command

import (
	"slices"
	"testing"
)

// VALIDATES: a registered column order comes back for the exact command path.
// PREVENTS: a declaration that is stored but never resolved, which would leave
// every table alphabetical while the registration call looks correct.
func TestRegisterColumnsRoundTrip(t *testing.T) {
	ResetColumnsForTest()
	t.Cleanup(ResetColumnsForTest)

	RegisterColumns([]string{"show bgp summary"}, ColumnOrder{"address", "state", "uptime"})

	orders := ColumnsForCommand("show bgp summary")
	if len(orders) != 1 {
		t.Fatalf("orders = %v, want one order", orders)
	}
	want := []string{"address", "state", "uptime"}
	if !slices.Equal(orders[0], want) {
		t.Errorf("order = %v, want %v", orders[0], want)
	}
	if got := ColumnsForCommand("show version"); got != nil {
		t.Errorf("undeclared command returned %v, want nil", got)
	}
}

// VALIDATES: registration normalizes whitespace, case, and empty names.
// PREVENTS: a declaration typed with an upper-case key or a stray space
// matching nothing, which would be silently inert (A-2).
func TestRegisterColumnsNormalizes(t *testing.T) {
	ResetColumnsForTest()
	t.Cleanup(ResetColumnsForTest)

	RegisterColumns([]string{"  Show   BGP  Summary "}, ColumnOrder{" Address ", "", "STATE"})

	orders := ColumnsForCommand("show bgp summary")
	if len(orders) != 1 {
		t.Fatalf("orders = %v, want one order", orders)
	}
	want := []string{"address", "state"}
	if !slices.Equal(orders[0], want) {
		t.Errorf("order = %v, want %v", orders[0], want)
	}
}

// VALIDATES: the longest registered command path wins, and a command with no
// declaration of its own inherits the nearest registered ancestor's.
// PREVENTS: a child command rendering with a parent's order when it declares
// its own (AC-7).
func TestColumnsForCommandLongestPrefix(t *testing.T) {
	ResetColumnsForTest()
	t.Cleanup(ResetColumnsForTest)

	RegisterColumns([]string{"show bgp"}, ColumnOrder{"parent"})
	RegisterColumns([]string{"show bgp peer list"}, ColumnOrder{"child"})

	tests := []struct {
		command string
		want    []string
	}{
		{command: "show bgp", want: []string{"parent"}},
		{command: "show bgp summary", want: []string{"parent"}},
		{command: "show bgp peer list", want: []string{"child"}},
		{command: "show bgp peer list detail", want: []string{"child"}},
		{command: "show bgp peer listing", want: []string{"parent"}},
		{command: "show version", want: nil},
	}
	for _, tt := range tests {
		orders := ColumnsForCommand(tt.command)
		if tt.want == nil {
			if orders != nil {
				t.Errorf("%q: orders = %v, want nil", tt.command, orders)
			}
			continue
		}
		if len(orders) != 1 || !slices.Equal(orders[0], tt.want) {
			t.Errorf("%q: orders = %v, want %v", tt.command, orders, tt.want)
		}
	}
}

// VALIDATES: an empty declaration on a child blocks the parent's order.
// PREVENTS: a command with unrelated keys inheriting an order it never asked
// for, which R-1 names as the trap the pipe-filter registry already carries.
func TestColumnsForCommandEmptyBlocksInheritance(t *testing.T) {
	ResetColumnsForTest()
	t.Cleanup(ResetColumnsForTest)

	RegisterColumns([]string{"show bgp"}, ColumnOrder{"parent"})
	RegisterColumns([]string{"show bgp statistics"})

	if orders := ColumnsForCommand("show bgp statistics"); len(orders) != 0 {
		t.Errorf("orders = %v, want none: an empty declaration must block inheritance", orders)
	}
	if orders := ColumnsForCommand("show bgp statistics family"); len(orders) != 0 {
		t.Errorf("orders = %v, want none: the block must reach the child's own children", orders)
	}
	if orders := ColumnsForCommand("show bgp summary"); len(orders) != 1 {
		t.Errorf("orders = %v, want the parent's order for a sibling that declares nothing", orders)
	}
}

// VALIDATES: a command declares one order per record shape.
// PREVENTS: the outer record and the peer rows of `show bgp summary` having to
// share one flat list, which no single list can express because both carry
// "uptime" in a different position.
func TestRegisterColumnsMultipleShapes(t *testing.T) {
	ResetColumnsForTest()
	t.Cleanup(ResetColumnsForTest)

	RegisterColumns([]string{"show bgp summary"},
		ColumnOrder{"address", "state", "uptime"},
		ColumnOrder{"router-id", "uptime", "peers"},
	)

	orders := ColumnsForCommand("show bgp summary")
	if len(orders) != 2 {
		t.Fatalf("orders = %v, want two orders", orders)
	}
	if !slices.Equal(orders[0], []string{"address", "state", "uptime"}) {
		t.Errorf("first order = %v", orders[0])
	}
	if !slices.Equal(orders[1], []string{"router-id", "uptime", "peers"}) {
		t.Errorf("second order = %v", orders[1])
	}
}
