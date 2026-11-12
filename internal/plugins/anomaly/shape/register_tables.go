// Design: docs/architecture/firewall/table-ownership-and-shutdown-flush.md -- Rule 2b, a producer's table name is reserved
// Related: responder.go -- owner, match.go -- tableNameV4, tableNameV6

package shape

import "github.com/ze-software/ze/internal/component/firewall"

// init reserves this producer's firewall table names, so no other owner and no
// operator table can share them: ApplyAll would merge the two into one kernel
// table. The owner MUST be the one this package passes to RegisterTables, or
// its own registration is refused.
//
//nolint:gochecknoinits // registration: a reservation is declared from init, before any RegisterTables
func init() {
	firewall.MustReserveTableNames(owner, firewall.TableControl{
		Plugin:  "anomaly-shape plugin",
		Decides: "the anomaly responder's mode and the sources it armed against",
		Command: "show anomaly shape",
	}, tableNameV4, tableNameV6)
}
