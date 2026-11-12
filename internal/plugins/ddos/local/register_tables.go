// Design: docs/architecture/firewall/table-ownership-and-shutdown-flush.md -- Rule 2b, a producer's table name is reserved
// Related: responder.go -- tableName is both the owner and the table

package local

import "github.com/ze-software/ze/internal/component/firewall"

// init reserves this producer's firewall table names, so no other owner and no
// operator table can share them: ApplyAll would merge the two into one kernel
// table. The owner MUST be the one this package passes to RegisterTables, or
// its own registration is refused.
//
//nolint:gochecknoinits // registration: a reservation is declared from init, before any RegisterTables
func init() {
	firewall.MustReserveTableNames(tableName, firewall.TableControl{
		Plugin: "ddos-local plugin",
		Decides: "the attacks the DDoS detector finds: a drop rule for an attacked destination stays " +
			"while the attack lasts, up to max-mitigation-duration, or until a commit turns mitigation off",
		Command: "show ddos local",
	}, tableName)
}
