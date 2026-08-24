package command

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// VALIDATES: a registered column order comes back for the exact command path.
// PREVENTS: a declaration that is stored but never resolved, which would leave
// every table alphabetical while the registration call looks correct.
func TestRegisterColumnsRoundTrip(t *testing.T) {
	ResetColumnsForTest()
	t.Cleanup(ResetColumnsForTest)

	RegisterColumns([]string{"show bgp"}, ColumnOrder{"address", "state", "uptime"})

	orders := ColumnsForCommand("show bgp")
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

	RegisterColumns([]string{"  Show   BGP  Health "}, ColumnOrder{" Address ", "", "STATE"})

	orders := ColumnsForCommand("show bgp health")
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
		{command: "show bgp health", want: []string{"parent"}},
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
	if orders := ColumnsForCommand("show bgp health"); len(orders) != 1 {
		t.Errorf("orders = %v, want the parent's order for a sibling that declares nothing", orders)
	}
}

// VALIDATES: a command declares one order per record shape.
// PREVENTS: the outer record and the peer rows of `show bgp` having to
// share one flat list, which no single list can express because both carry
// "uptime" in a different position.
func TestRegisterColumnsMultipleShapes(t *testing.T) {
	ResetColumnsForTest()
	t.Cleanup(ResetColumnsForTest)

	RegisterColumns([]string{"show bgp"},
		ColumnOrder{"address", "state", "uptime"},
		ColumnOrder{"router-id", "uptime", "peers"},
	)

	orders := ColumnsForCommand("show bgp")
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

// declarationCase is one of the registries that hold a single declaration for
// each command path. Every case declares on declaredPath alone, so the three
// tests below read the same rule through four different value types.
//
// The four public registrars are the entry points a package declares through,
// so the tests drive them rather than the registry underneath.
type declarationCase struct {
	// name is what the conflict report calls this registry.
	name string
	// reset clears the registry before and after a case.
	reset func()
	// none declares NOTHING on declaredPath: the floor.
	none func()
	// one and other declare two DIFFERENT non-empty values on declaredPath.
	one   func()
	other func()
	// oneText and otherText are how a conflict report renders those two values.
	oneText   string
	otherText string
	// resolved renders what declaredPath resolves to, so a test compares the
	// answer without knowing the value type. It answers resolvedNothing for a
	// path that resolves to nothing at all.
	resolved func() string
}

const (
	// declaredPath is the command path every declaration case declares on. It
	// names no real command, so a case never reads a declaration an init() left
	// behind.
	declaredPath = "show test rows"

	// resolvedNothing is what a case's resolved() answers for a path that
	// resolves to no declaration at all.
	resolvedNothing = "none"
)

func declarationCases() []declarationCase {
	return []declarationCase{
		{
			name:      "answer shape",
			reset:     ResetShapesForTest,
			none:      func() { RegisterShape([]string{declaredPath}) },
			one:       func() { RegisterShape([]string{declaredPath}, ShapeTab) },
			other:     func() { RegisterShape([]string{declaredPath}, ShapeDoc) },
			oneText:   "tab",
			otherText: "doc",
			resolved: func() string {
				shape, declared := ShapeForCommand(declaredPath)
				if !declared {
					return resolvedNothing
				}
				return shape.String()
			},
		},
		{
			name:      "column order",
			reset:     ResetColumnsForTest,
			none:      func() { RegisterColumns([]string{declaredPath}) },
			one:       func() { RegisterColumns([]string{declaredPath}, ColumnOrder{"address", "state"}) },
			other:     func() { RegisterColumns([]string{declaredPath}, ColumnOrder{"state", "address"}) },
			oneText:   "[address state]",
			otherText: "[state address]",
			resolved: func() string {
				orders := ColumnsForCommand(declaredPath)
				if len(orders) == 0 {
					return resolvedNothing
				}
				return fmt.Sprint(orders[0])
			},
		},
		{
			name:      "address field list",
			reset:     ResetAddressFieldsForTest,
			none:      func() { RegisterAddressFields([]string{declaredPath}) },
			one:       func() { RegisterAddressFields([]string{declaredPath}, "peer") },
			other:     func() { RegisterAddressFields([]string{declaredPath}, "next-hop") },
			oneText:   "peer",
			otherText: "next-hop",
			resolved: func() string {
				fields := AddressFieldsForCommand(declaredPath)
				if len(fields) == 0 {
					return resolvedNothing
				}
				return strings.Join(fields, " ")
			},
		},
		{
			name:  "pipe filter set",
			reset: ResetPipeFiltersForTest,
			none:  func() { RegisterPipeFilters([]string{declaredPath}) },
			one: func() {
				RegisterPipeFilters([]string{declaredPath}, PipeFilter{Name: "peer", Description: "Filter by peer", TakesArg: true})
			},
			other: func() {
				RegisterPipeFilters([]string{declaredPath}, PipeFilter{Name: "family", Description: "Filter by family", TakesArg: true})
			},
			oneText:   "peer",
			otherText: "family",
			resolved: func() string {
				filters := PipeFiltersForCommand(declaredPath)
				if len(filters) == 0 {
					return resolvedNothing
				}
				return filters[0].Name
			},
		},
	}
}

// VALIDATES: AC-2. Two packages declaring two DIFFERENT non-empty values for
// one command path panic, and the report names the registry, the path and both
// values.
// PREVENTS: the last package to run its init() deciding what a command's answer
// holds, which is how `show bgp rib` came to resolve to a shape neither package
// can predict.
func TestRegisterConflictPanics(t *testing.T) {
	for _, tt := range declarationCases() {
		t.Run(tt.name, func(t *testing.T) {
			tt.reset()
			t.Cleanup(tt.reset)

			tt.one()

			defer func() {
				raised := recover()
				if raised == nil {
					t.Fatalf("%s: a second, different declaration for %q did not panic", tt.name, declaredPath)
				}
				report, isText := raised.(string)
				if !isText {
					t.Fatalf("%s: panicked with %T, want the BUG report as a string", tt.name, raised)
				}
				for _, want := range []string{"BUG:", tt.name, declaredPath, tt.oneText, tt.otherText} {
					if !strings.Contains(report, want) {
						t.Errorf("%s: report %q does not name %q", tt.name, report, want)
					}
				}
			}()

			tt.other()
		})
	}
}

// VALIDATES: AC-1. An EMPTY declaration is a floor rather than a claim, so a
// non-empty declaration wins whatever the order the two are registered in.
// PREVENTS: the empty declaration the BGP peer command plugin puts on every
// child of `show bgp` erasing the `tab` the rib command plugin declares for
// `show bgp rib`, which package initialization order alone decided before.
func TestRegisterEmptyThenNonEmpty(t *testing.T) {
	for _, tt := range declarationCases() {
		t.Run(tt.name, func(t *testing.T) {
			tt.reset()
			t.Cleanup(tt.reset)

			tt.none()
			tt.one()
			if got := tt.resolved(); got != tt.oneText {
				t.Errorf("%s: empty then non-empty resolved to %q, want %q", tt.name, got, tt.oneText)
			}

			tt.reset()

			tt.one()
			tt.none()
			if got := tt.resolved(); got != tt.oneText {
				t.Errorf("%s: non-empty then empty resolved to %q, want %q", tt.name, got, tt.oneText)
			}
		})
	}
}

// VALIDATES: AC-3. One declaration made twice is a no-op, never a conflict.
// PREVENTS: the conflict guard firing on a package that restates its own
// declaration, which would make a correct registration stop the daemon.
func TestRegisterIdenticalIsNoOp(t *testing.T) {
	for _, tt := range declarationCases() {
		t.Run(tt.name, func(t *testing.T) {
			tt.reset()
			t.Cleanup(tt.reset)

			defer func() {
				if raised := recover(); raised != nil {
					t.Fatalf("%s: an identical second declaration panicked: %v", tt.name, raised)
				}
			}()

			tt.one()
			tt.one()
			if got := tt.resolved(); got != tt.oneText {
				t.Errorf("%s: resolved to %q, want %q", tt.name, got, tt.oneText)
			}
		})
	}
}
