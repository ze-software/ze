// Design: docs/architecture/firewall/table-ownership-and-shutdown-flush.md -- Rule 2b, a producer's table name is reserved
// Related: registry.go -- RegisterTables refuses a name another owner reserved
// Related: engine.go -- refuseReservedTableNames, the operator's half of the check

package firewall

import (
	"fmt"
	"strings"
	"sync"
)

// tableReservations maps a kernel table name to the registry owner that
// reserved it. Each producer fills it through MustReserveTableNames from the
// init() of its own register_tables.go (the firewall engine's conntrack-helper
// tables from its register.go), so the firewall component names no producer:
// a compiled-out producer reserves nothing, and it then installs no producer
// table for another owner's table of that name to merge with. Safe for
// concurrent use.
var tableReservations = struct {
	mu     sync.RWMutex
	owners map[string]string
}{
	owners: make(map[string]string),
}

// MustReserveTableNames records that owner alone may register each named
// table. ApplyAll merges same-name tables of different owners into one kernel
// table, so a name two owners use gives one table two owners' chains; a
// reserved name is refused to every other owner by RegisterTables, and to the
// operator's firewall config at verify.
//
// A name another owner already reserved, or one without the ze_ ownership
// prefix, is a defect in the calling package, so it panics. MUST be called
// from init(), before any RegisterTables.
func MustReserveTableNames(owner string, names ...string) {
	tableReservations.mu.Lock()
	defer tableReservations.mu.Unlock()
	for _, name := range names {
		if !strings.HasPrefix(name, tableNamePrefix) {
			panic("BUG: firewall: owner " + owner + " reserved table " + name + " without the ze_ ownership prefix")
		}
		if holder, held := tableReservations.owners[name]; held && holder != owner {
			panic("BUG: firewall: owner " + owner + " reserved table " + name + ", which owner " + holder + " already reserved")
		}
		tableReservations.owners[name] = owner
	}
}

// tableReservedBy answers the owner that reserved name, and whether one did.
func tableReservedBy(name string) (string, bool) {
	tableReservations.mu.RLock()
	defer tableReservations.mu.RUnlock()
	owner, held := tableReservations.owners[name]
	return owner, held
}

// refuseForeignReservations returns an error for the first table in tables
// whose name another owner reserved. RegisterTables calls it before it changes
// the registry, so a refused registration leaves every owner's tables as they
// were.
func refuseForeignReservations(owner string, tables []Table) error {
	for i := range tables {
		holder, held := tableReservedBy(tables[i].Name)
		if !held {
			continue
		}
		if holder == owner {
			continue
		}
		return fmt.Errorf("firewall: owner %q registered table %q, which owner %q reserved; two owners in one kernel table would merge their chains, so the registration is refused and %q keeps its tables",
			owner, tables[i].Name, holder, holder)
	}
	return nil
}

// refuseReservedTableNames refuses an operator firewall table whose kernel
// name a producer reserved. It runs on the operator's tables only, at verify
// and at configure: the conntrack-helper tables are the firewall engine's own
// reserved names and are judged by RegisterTables instead.
func refuseReservedTableNames(tables []Table) error {
	for i := range tables {
		holder, held := tableReservedBy(tables[i].Name)
		if !held {
			continue
		}
		return fmt.Errorf("firewall table %q: its kernel name %q is reserved by owner %q, and two owners in one kernel table would merge their chains; rename the table",
			StripZeTablePrefix(tables[i].Name), tables[i].Name, holder)
	}
	return nil
}
