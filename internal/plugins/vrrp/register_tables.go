// Design: docs/architecture/firewall/table-ownership-and-shutdown-flush.md -- Rule 2b, a producer's table name is reserved
// Related: acceptfilter.go, ownerfilter.go, backupfilter.go -- the three table owners

package vrrp

import "github.com/ze-software/ze/internal/component/firewall"

// init reserves this producer's firewall table names, so no other owner and no
// operator table can share them: ApplyAll would merge the two into one kernel
// table. The owner MUST be the one this package passes to RegisterTables, or
// its own registration is refused.
//
//nolint:gochecknoinits // registration: a reservation is declared from init, before any RegisterTables
func init() {
	firewall.MustReserveTableNames(acceptFilterOwner, vrrpTableControl, acceptFilterTableName)
	firewall.MustReserveTableNames(ownerFilterOwner, vrrpTableControl, ownerFilterARPTableName, ownerFilterNDTableName)
	firewall.MustReserveTableNames(backupFilterOwner, vrrpTableControl, backupFilterTableName)
}

// vrrpTableControl is what decides the rules of every VRRP table: each virtual
// router's configuration and its current state.
var vrrpTableControl = firewall.TableControl{
	Plugin:  "vrrp plugin",
	Decides: "the VRRP virtual routers of the configuration and whether each is master or backup",
	Command: "show vrrp",
}
