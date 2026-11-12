// Design: docs/architecture/firewall/table-ownership-and-shutdown-flush.md -- Rule 2b, a producer's table name is reserved
// Related: gtsm.go -- filterOwner, filterTableName

package gtsm

import "github.com/ze-software/ze/internal/component/firewall"

// init reserves this producer's firewall table names, so no other owner and no
// operator table can share them: ApplyAll would merge the two into one kernel
// table. The owner MUST be the one this package passes to RegisterTables, or
// its own registration is refused.
//
//nolint:gochecknoinits // registration: a reservation is declared from init, before any RegisterTables
func init() {
	firewall.MustReserveTableNames(filterOwner, firewall.TableControl{
		Plugin:  "gtsm component",
		Decides: "the BGP peers whose configuration sets the TTL security check (RFC 5082)",
		Command: "show firewall ruleset gtsm",
	}, filterTableName)
}
