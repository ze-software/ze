// Design: docs/architecture/firewall/table-ownership-and-shutdown-flush.md -- Rule 2b, a producer's table name is reserved
// Related: source_guard.go -- sourceGuardOwner, sourceGuardTableName

package engine

import "github.com/ze-software/ze/internal/component/firewall"

// init reserves this producer's firewall table names, so no other owner and no
// operator table can share them: ApplyAll would merge the two into one kernel
// table. The owner MUST be the one this package passes to RegisterTables, or
// its own registration is refused.
//
//nolint:gochecknoinits // registration: a reservation is declared from init, before any RegisterTables
func init() {
	firewall.MustReserveTableNames(sourceGuardOwner, firewall.TableControl{
		Plugin:  "IKE engine's tunnel-source guard",
		Decides: "the IPsec site-to-site peers of the configuration and their tunnel addresses",
		Command: "show firewall ruleset ipsec_source_guard",
	}, sourceGuardTableName)
}
