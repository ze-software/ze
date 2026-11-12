// Design: docs/architecture/firewall/table-ownership-and-shutdown-flush.md -- Rule 2b, a producer's table name is reserved
// Related: registry.go -- RegisterTables refuses a name another owner reserved
// Related: engine.go -- refuseReservedTableNames, the operator's half of the check

package firewall

import (
	"fmt"
	"strings"
	"sync"
)

// TableControl names, for an operator, what decides the rules of a reserved
// table. A producer rewrites its table whenever that state changes, so an nft
// rule an operator adds there, or a term of the firewall config, does not last:
// a message about one of the table's rules names this instead. Every field is
// required.
type TableControl struct {
	// Plugin names the plugin or component that writes the table, as an operator
	// knows it, such as "flowspec-firewall plugin".
	Plugin string
	// Decides says what decides the table's rules, in the operator's words, so a
	// message can quote it after "its rules follow ".
	Decides string
	// Command is the exact command that shows that state.
	Command string
}

// tableReservation is one reserved kernel table name: the registry owner that
// reserved it, and what decides its rules.
type tableReservation struct {
	owner   string
	control TableControl
}

// tableReservations maps a kernel table name to the reservation of it. Each
// producer fills it through MustReserveTableNames from the init() of its own
// register_tables.go (the firewall engine's conntrack-helper tables from its
// register.go), so the firewall component names no producer: a compiled-out
// producer reserves nothing, and it then installs no producer table for another
// owner's table of that name to merge with. Safe for concurrent use.
var tableReservations = struct {
	mu     sync.RWMutex
	owners map[string]tableReservation
}{
	owners: make(map[string]tableReservation),
}

// MustReserveTableNames records that owner alone may register each named
// table. ApplyAll merges same-name tables of different owners into one kernel
// table, so a name two registry owners use gives one table two owners' chains,
// and a producer that writes its table outside the registry (the iface
// backend's ze_iface_rpf) and the nft backend each delete and recreate the
// shared table, wiping the other's chains. A reserved name is refused to every
// other owner by RegisterTables, and to the operator's firewall config at
// verify.
//
// control says what decides the rules of each named table, for a message about
// one of those rules to name (TableControlOf).
//
// A name another owner already reserved, one without the ze_ ownership prefix,
// or a control with an empty field is a defect in the calling package, so it
// panics. MUST be called from init(), before any RegisterTables.
func MustReserveTableNames(owner string, control TableControl, names ...string) {
	if control.Plugin == "" {
		panic("BUG: firewall: owner " + owner + " reserved tables naming no plugin that writes them")
	}
	if control.Decides == "" {
		panic("BUG: firewall: owner " + owner + " reserved tables naming nothing that decides their rules")
	}
	if control.Command == "" {
		panic("BUG: firewall: owner " + owner + " reserved tables naming no command that shows what decides their rules")
	}
	tableReservations.mu.Lock()
	defer tableReservations.mu.Unlock()
	for _, name := range names {
		if !strings.HasPrefix(name, tableNamePrefix) {
			panic("BUG: firewall: owner " + owner + " reserved table " + name + " without the ze_ ownership prefix")
		}
		if held, ok := tableReservations.owners[name]; ok && held.owner != owner {
			panic("BUG: firewall: owner " + owner + " reserved table " + name + ", which owner " + held.owner + " already reserved")
		}
		tableReservations.owners[name] = tableReservation{owner: owner, control: control}
	}
}

// tableReservedBy answers the owner that reserved name, and whether one did.
func tableReservedBy(name string) (string, bool) {
	tableReservations.mu.RLock()
	defer tableReservations.mu.RUnlock()
	held, ok := tableReservations.owners[name]
	return held.owner, ok
}

// TableControlOf answers what decides the rules of the kernel table name, and
// whether a producer reserved it. A ze_ table no producer reserved belongs to
// the operator's firewall config: its kernel name is ze_ and the config's table
// name, and the firewall-irr and firewall-domain producers that also register
// into such a table add sets to it, never a chain. The reservations are filled
// from init(), so every process of the ze binary holds the same set, a forked
// plugin's included. Safe for concurrent use.
func TableControlOf(name string) (TableControl, bool) {
	tableReservations.mu.RLock()
	defer tableReservations.mu.RUnlock()
	held, ok := tableReservations.owners[name]
	return held.control, ok
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
		return fmt.Errorf("firewall: owner %q registered table %q, which owner %q reserved; two owners in one kernel table would overwrite each other's chains, so the registration is refused and %q keeps its tables",
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
		return fmt.Errorf("firewall table %q: its kernel name %q is reserved by owner %q, and two owners in one kernel table would overwrite each other's chains; rename the table",
			StripZeTablePrefix(tables[i].Name), tables[i].Name, holder)
	}
	return nil
}
