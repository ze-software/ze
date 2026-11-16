// Design: docs/architecture/firewall/table-ownership-and-shutdown-flush.md -- Rule 2b, a producer's table name is reserved
// Related: register_contribution.go -- the engine operation that judges a contribution here
// Related: reservation.go -- tableReservedBy, the reservations these checks read

package firewall

import "github.com/ze-software/ze/internal/core/opererr"

// contributionFix names what makes a contribution acceptable. The caller is a
// plugin compiled into ze, so the fix is in its own package.
const contributionFix = "Reserve each contributed table from the plugin's init() with firewall.MustReserveTableNames, or leave the table out of the contribution"

// refuseOwnerWithoutReservation answers an operator error when owner reserved
// no table name at all. A plugin outside the ze binary reserves nothing, so it
// is refused before its section is read, an empty one included.
func refuseOwnerWithoutReservation(owner string) error {
	tableReservations.mu.RLock()
	defer tableReservations.mu.RUnlock()
	for _, held := range tableReservations.owners {
		if held.owner == owner {
			return nil
		}
	}
	return opererr.New(
		"Plugin "+owner+" may not contribute firewall tables",
		"Only a plugin compiled into this ze binary that reserved its table names may contribute tables, so no outside process can write packet-filter state",
		contributionFix,
	)
}

// refuseUnreservedContribution answers an operator error for the first table
// in tables whose kernel name owner did not reserve: a name no owner reserved,
// which the operator's firewall config may use, or one another owner reserved.
// A contribution is never merged into a table someone else writes.
func refuseUnreservedContribution(owner string, tables []Table) error {
	for i := range tables {
		holder, held := tableReservedBy(tables[i].Name)
		if !held {
			return opererr.New(
				"Plugin "+owner+" contributed firewall table "+tables[i].Name+", which no owner reserved",
				"A table no producer reserved belongs to the operator's firewall config, and a contribution never writes into it",
				contributionFix,
			)
		}
		if holder != owner {
			return opererr.New(
				"Plugin "+owner+" contributed firewall table "+tables[i].Name+", which owner "+holder+" reserved",
				"Two owners in one kernel table would overwrite each other's chains, so "+holder+" keeps its table",
				contributionFix,
			)
		}
	}
	return nil
}
