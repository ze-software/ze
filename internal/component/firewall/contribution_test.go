// Design: docs/architecture/firewall/table-ownership-and-shutdown-flush.md -- contributed tables tests

package firewall

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
)

// TestContributionRefusesUnreservedTable checks that a plugin contributes only
// tables it reserved: a name another owner reserved and a ze_ name no owner
// reserved are refused with an operator error naming the table, an owner that
// reserved nothing is refused outright, and the caller's own reserved name
// passes. Method: reserve names no producer uses under two test owners, judge
// each case, and delete the test reservations in cleanup.
func TestContributionRefusesUnreservedTable(t *testing.T) {
	const owner = "test-contribution-owner"
	const other = "test-contribution-other"
	const ownTable = "ze_test_contribution_own"
	const otherTable = "ze_test_contribution_other"
	const freeTable = "ze_test_contribution_free"
	control := TableControl{Plugin: "contribution test", Decides: "the test", Command: "ze show firewall ruleset"}
	t.Cleanup(func() {
		tableReservations.mu.Lock()
		delete(tableReservations.owners, ownTable)
		delete(tableReservations.owners, otherTable)
		tableReservations.mu.Unlock()
	})
	MustReserveTableNames(owner, control, ownTable)
	MustReserveTableNames(other, control, otherTable)

	if err := refuseOwnerWithoutReservation(owner); err != nil {
		t.Fatalf("owner %q reserved a table and was refused: %v", owner, err)
	}
	if err := refuseOwnerWithoutReservation("test-contribution-nobody"); err == nil {
		t.Fatal("an owner that reserved no table was not refused")
	}

	for _, refused := range []string{otherTable, freeTable} {
		err := refuseUnreservedContribution(owner, []Table{{Name: ownTable}, {Name: refused}})
		if err == nil {
			t.Fatalf("table %q is not %q's and was accepted", refused, owner)
		}
		var failure *opererr.Error
		if !errors.As(err, &failure) {
			t.Fatalf("refusal of %q is not an operator error: %T %v", refused, err, err)
		}
		if !strings.Contains(failure.Problem(), refused) {
			t.Fatalf("refusal does not name table %q: %q", refused, failure.Problem())
		}
	}

	if err := refuseUnreservedContribution(owner, []Table{{Name: ownTable}}); err != nil {
		t.Fatalf("owner %q's own reserved table was refused: %v", owner, err)
	}
}
