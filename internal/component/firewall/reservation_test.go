package firewall

import (
	"strings"
	"testing"
)

// testReservationOwner is the owner every reserveForTest reservation is made for,
// the "test-holder" the tests name when they register or expect that owner.
const testReservationOwner = "test-holder"

// reserveForTest reserves names for testReservationOwner and removes the
// reservation when the test ends, so no other test in the package meets it.
func reserveForTest(t *testing.T, names ...string) {
	t.Helper()
	MustReserveTableNames(testReservationOwner, testTableControl, names...)
	t.Cleanup(func() {
		tableReservations.mu.Lock()
		defer tableReservations.mu.Unlock()
		for _, name := range names {
			delete(tableReservations.owners, name)
		}
	})
}

// VALIDATES: AC-22 -- RegisterTables refuses a table name another owner
// reserved, naming both owners, and leaves the reserving owner's registration
// as it was. The reserving owner itself still registers the name.
// PREVENTS: an owner's table merging into another owner's table at ApplyAll.
func TestRegisterTablesRefusesAReservedName(t *testing.T) {
	reserveForTest(t, "ze_test_reserved")
	t.Cleanup(func() {
		_ = RegisterTables("test-holder", nil)
		_ = RegisterTables("test-intruder", nil)
	})

	held := []Table{{Name: "ze_test_reserved", Family: FamilyARP}}
	if err := RegisterTables("test-holder", held); err != nil {
		t.Fatalf("the reserving owner was refused its own name: %v", err)
	}
	err := RegisterTables("test-intruder", []Table{{Name: "ze_test_reserved", Family: FamilyIP6}})
	if err == nil {
		t.Fatal("expected a refusal for a name another owner reserved, got nil")
	}
	for _, want := range []string{`"test-intruder"`, `"test-holder"`, `"ze_test_reserved"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %s", err, want)
		}
	}

	tableRegistry.mu.Lock()
	defer tableRegistry.mu.Unlock()
	if _, stored := tableRegistry.owners["test-intruder"]; stored {
		t.Error("the refused owner was stored in the registry")
	}
	got := tableRegistry.owners["test-holder"]
	if len(got) != 1 || got[0].Name != "ze_test_reserved" || got[0].Family != FamilyARP {
		t.Errorf("the reserving owner's tables changed: %+v", got)
	}
}

// VALIDATES: AC-22 -- an unreserved name stays shareable, so firewall-irr and
// firewall-domain still register sets into an operator table's name.
// PREVENTS: the reservation check refusing the set contributions ApplyAll
// merges into operator tables.
func TestRegisterTablesAcceptsSetsIntoAnUnreservedName(t *testing.T) {
	t.Cleanup(func() {
		_ = RegisterTables("firewall", nil)
		_ = RegisterTables("firewall-irr", nil)
	})
	if err := RegisterTables("firewall", []Table{{Name: "ze_wan", Family: FamilyInet}}); err != nil {
		t.Fatalf("operator table refused: %v", err)
	}
	sets := []Table{{Name: "ze_wan", Family: FamilyInet, Sets: []Set{{Name: "irr_v4_AS65000", Type: SetTypeIPv4}}}}
	if err := RegisterTables("firewall-irr", sets); err != nil {
		t.Fatalf("set contribution into an unreserved operator table refused: %v", err)
	}
}

// VALIDATES: AC-15 -- an operator table whose kernel name a producer
// reserved is refused at verify, naming the table, its kernel name and the
// owner; an unreserved operator table passes.
// PREVENTS: an operator table named like a VRRP table merging into it.
func TestOperatorTableRefusedOnAReservedName(t *testing.T) {
	reserveForTest(t, "ze_test_reserved")
	err := refuseReservedTableNames([]Table{
		{Name: "ze_wan", Family: FamilyInet},
		{Name: "ze_test_reserved", Family: FamilyIP},
	})
	if err == nil {
		t.Fatal("expected a refusal for a reserved operator table name, got nil")
	}
	for _, want := range []string{`firewall table "test_reserved"`, `kernel name "ze_test_reserved"`, `owner "test-holder"`, "rename the table"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
	if err := refuseReservedTableNames([]Table{{Name: "ze_wan", Family: FamilyInet}}); err != nil {
		t.Errorf("unreserved operator table refused: %v", err)
	}
}

// VALIDATES: AC-15 -- the conntrack-helper tables, which this package
// registers itself, are reserved, so an operator table cannot take them.
func TestConntrackHelperTablesAreReserved(t *testing.T) {
	for _, name := range []string{conntrackHelperTableInet, conntrackHelperTableIP} {
		owner, held := tableReservedBy(name)
		if !held || owner != conntrackHelperOwner {
			t.Errorf("table %q reserved by %q (held %v), want %q", name, owner, held, conntrackHelperOwner)
		}
	}
}

// VALIDATES: a second owner reserving a held name, or a name without the ze_
// prefix, is a defect in the calling package and panics at init.
func TestMustReserveTableNamesPanicsOnASecondOwner(t *testing.T) {
	reserveForTest(t, "ze_test_reserved")
	for _, tc := range []struct {
		name  string
		owner string
		table string
	}{
		{"second owner", "test-intruder", "ze_test_reserved"},
		{"no prefix", "test-holder", "test_bare"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("expected a panic, got none")
				}
			}()
			MustReserveTableNames(tc.owner, testTableControl, tc.table)
		})
	}
}

// testTableControl is the control every test reservation declares.
var testTableControl = TableControl{
	Plugin:  "test plugin",
	Decides: "the test's own state",
	Command: "show firewall ruleset test_reserved",
}

// VALIDATES: a reservation whose control leaves a field empty is a defect in the
// calling package and panics at init, so no reserved table is ever reported with
// nothing that decides its rules; TableControlOf answers the control a
// reservation declared, and false for a name no producer reserved.
// PREVENTS: a message about a rule in a plugin's table that sends the operator
// to nothing, or to an editor that cannot reach the table.
func TestMustReserveTableNamesNeedsAControl(t *testing.T) {
	for _, tc := range []struct {
		name    string
		control TableControl
	}{
		{"no plugin", TableControl{Decides: "x", Command: "show x"}},
		{"nothing decides", TableControl{Plugin: "x", Command: "show x"}},
		{"no command", TableControl{Plugin: "x", Decides: "x"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("expected a panic, got none")
				}
			}()
			MustReserveTableNames("test-holder", tc.control, "ze_test_no_control")
		})
	}
	if _, ok := TableControlOf("ze_test_no_control"); ok {
		t.Error("a reservation refused for its control was recorded")
	}

	reserveForTest(t, "ze_test_reserved")
	got, ok := TableControlOf("ze_test_reserved")
	if !ok || got != testTableControl {
		t.Errorf("TableControlOf(ze_test_reserved) = %+v, %v; want %+v, true", got, ok, testTableControl)
	}
	if got, ok := TableControlOf("ze_operator_table"); ok {
		t.Errorf("TableControlOf answered %+v for a name no producer reserved", got)
	}
}
