// Design: docs/architecture/firewall/table-ownership-and-shutdown-flush.md -- Rule 2b, a producer's table name is reserved
// Related: rpf_linux.go -- ReplaceIPv6RPF, which writes the reserved table

package ifacenetlink

import "github.com/ze-software/ze/internal/component/firewall"

// rpfTableName is the nftables table that carries the IPv6 reverse-path check
// (rpf_linux.go). It is declared in this untagged file so the reservation below
// holds in every build, including one that validates a config off Linux.
const rpfTableName = "ze_iface_rpf"

// rpfTableOwner names the IPv6 reverse-path check in a reservation refusal.
// ReplaceIPv6RPF writes the table over netlink and never calls
// firewall.RegisterTables, so this owner appears only in that refusal.
const rpfTableOwner = "iface-rpf"

// init reserves the reverse-path table's name, so no operator firewall table
// and no other owner can share it. The firewall nft backend would otherwise
// delete and recreate the shared table on its apply, wiping the reverse-path
// chain and sets, and ReplaceIPv6RPF would wipe the operator's rules in turn.
//
//nolint:gochecknoinits // registration: a reservation is declared from init, before any RegisterTables
func init() {
	firewall.MustReserveTableNames(rpfTableOwner, firewall.TableControl{
		Plugin:  "iface netlink backend",
		Decides: "the rpf-check setting of each interface in the configuration",
		Command: "show interface",
	}, rpfTableName)
}
