// Design: docs/architecture/firewall/table-ownership-and-shutdown-flush.md -- Rule 2b, a producer's table name is reserved
// Related: register_contribution.go -- the engine operation and the generation cleanup that call these
// Related: reservation.go -- tableReservedBy, the reservations these checks read
// Related: doctor_contribution.go -- the check that reports a contributed set the kernel may not hold
// Related: registry.go -- RegisterTables and ApplyAll, the path configured tables take too

package firewall

import (
	"sync"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// contributionFix names what makes a contribution acceptable. The caller is a
// plugin compiled into ze, so the fix is in its own package.
const contributionFix = "Reserve each contributed table from the plugin's init() with firewall.MustReserveTableNames, or leave the table out of the contribution"

// contributions serializes every change to a contributed set, so one owner's
// replacement and its undo never interleave with another's, and records the
// process that last made a contribution under each owner name. The generation
// cleanup withdraws an owner's tables only for that process, never for an
// older generation that ended after its replacement contributed. Safe for
// concurrent use.
var contributions = struct {
	mu      sync.Mutex
	callers map[string]*process.Process
}{callers: map[string]*process.Process{}}

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

// replaceContributedTables makes tables the complete set proc contributes and
// reconciles the merged set of every owner, so the answer is the kernel's
// outcome. The tables take the path the operator's configured tables take:
// ValidateTables, then the table registry, then ApplyAll, which runs the
// backend's verifier over the merged set before it applies. An empty set
// withdraws every table the owner contributed.
//
// A refused set changes nothing: the owner's previous set is registered again
// and reconciled. When that restore fails too, the kernel may hold neither
// set, so the failure is raised as a warning that doctor reports.
//
// The caller MUST have passed refuseOwnerWithoutReservation and
// refuseUnreservedContribution for these tables.
func replaceContributedTables(proc *process.Process, tables []Table) (*rpc.FirewallTablesReplaceOutput, error) {
	owner := proc.Name()
	if err := ValidateTables(tables); err != nil {
		return nil, opererr.Wrap(err,
			"Plugin "+owner+" contributed firewall tables that do not validate",
			"A contributed table passes the checks the operator's firewall config passes, and no table of a refused contribution is applied",
			"Correct the table the error names in the plugin's contribution",
		)
	}
	contributions.mu.Lock()
	defer contributions.mu.Unlock()
	contributions.callers[owner] = proc
	previous := registeredTables(owner)
	if err := RegisterTables(owner, contributedSet(tables)); err != nil {
		return nil, opererr.Wrap(err,
			"Plugin "+owner+" contributed firewall tables the table registry refused",
			"The registry keeps the owner's previous set, so nothing was changed",
			contributionFix,
		)
	}
	if err := ApplyAll(); err != nil {
		restoreContributedTables(owner, previous)
		return nil, opererr.Wrap(err,
			"Plugin "+owner+" contributed firewall tables the firewall backend refused",
			"The owner's previous set was registered and reconciled again, so the kernel keeps the tables it held",
			"Correct the contribution, or select a firewall backend that implements it; ze show firewall ruleset lists what the kernel holds",
		)
	}
	names := make([]string, 0, len(tables))
	for i := range tables {
		names = append(names, tables[i].Name)
	}
	return &rpc.FirewallTablesReplaceOutput{Tables: names}, nil
}

// withdrawContributedTables withdraws every table proc contributed, when proc
// is the process that made its owner's last contribution, and reconciles. The
// generation cleanup calls it after the process ended, so a failure has no
// caller to answer and is raised as a warning that doctor reports.
func withdrawContributedTables(proc *process.Process) {
	owner := proc.Name()
	contributions.mu.Lock()
	defer contributions.mu.Unlock()
	declarer, known := contributions.callers[owner]
	if !known {
		return
	}
	if declarer != proc {
		return
	}
	delete(contributions.callers, owner)
	if registeredTables(owner) == nil {
		return
	}
	if err := RegisterTables(owner, nil); err != nil {
		raiseContributionFailure(owner, "were not withdrawn after the plugin ended", err)
		return
	}
	if err := ApplyAll(); err != nil {
		raiseContributionFailure(owner, "were not removed after the plugin ended", err)
	}
}

// restoreContributedTables registers previous as owner's set again after a
// refused replacement and reconciles it. A failure is raised, because the
// kernel may then hold neither the refused set nor the previous one.
func restoreContributedTables(owner string, previous []Table) {
	if err := RegisterTables(owner, previous); err != nil {
		raiseContributionFailure(owner, "could not be registered again after a refused replacement", err)
		return
	}
	if err := ApplyAll(); err != nil {
		raiseContributionFailure(owner, "could not be applied again after a refused replacement", err)
	}
}

// registeredTables answers the set owner holds in the table registry, nil when
// it holds none.
func registeredTables(owner string) []Table {
	tableRegistry.mu.Lock()
	defer tableRegistry.mu.Unlock()
	return tableRegistry.owners[owner]
}

// contributedSet answers the value RegisterTables takes for a contribution: nil
// for an empty set, which withdraws the owner, so an owner with no table holds
// no registry entry.
func contributedSet(tables []Table) []Table {
	if len(tables) == 0 {
		return nil
	}
	return tables
}

// raiseContributionFailure logs and raises a warning that owner's contributed
// tables may not match the kernel. The next successful reconcile of any owner
// clears it (clearContributionWarnings), because the kernel then holds the
// registry's merged set.
func raiseContributionFailure(owner, what string, err error) {
	const fix = "ze show firewall ruleset lists what the kernel holds; ze request plugin restart "
	if log := loggerPtr.Load(); log != nil {
		log.Warn("contributed firewall tables "+what,
			"plugin", owner, "error", err,
			"code", codeDoctorFirewallContributedTables,
			"fix", fix+owner)
	}
	report.RaiseWarningFix(contributionReportSource, codeDoctorFirewallContributedTables,
		contributionSubjectPlugin+owner,
		report.FitMessage("firewall tables contributed by plugin "+owner+" "+what+": "+err.Error()),
		fix+owner)
}
