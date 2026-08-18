// Design: docs/architecture/core-design.md — firewall table registry
// Related: metrics.go -- observeApply, the reconcile latency and timeout report

package firewall

import (
	"errors"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

var errFirewallBackendNotLoaded = errors.New("firewall backend not loaded")

// flushOnShutdown gates whether an orderly ze process stop (SIGTERM) removes
// ze-owned firewall tables from the kernel. Default true: stopping the daemon
// leaves no rules behind. Set false via `firewall { flush-on-shutdown false; }`
// to use ze as a one-shot provisioner -- program the rules, exit, and leave them
// running (like nft -f). This keys off how the process exits and is unrelated to
// BGP graceful restart. A crash never runs the shutdown path (no cleanup
// executes), so tables always persist across a crash regardless of this setting.
var flushOnShutdown atomic.Bool

//nolint:gochecknoinits // package default: flush-on-shutdown is on unless config disables it
func init() { flushOnShutdown.Store(true) }

// setFlushOnShutdown records the parsed firewall `flush-on-shutdown` option.
// Called from the firewall engine when it processes a firewall config section;
// a copp-only config (no firewall block) leaves the default (true) in place.
func setFlushOnShutdown(v bool) { flushOnShutdown.Store(v) }

// flushOnShutdownEnabled reports whether a clean shutdown should flush tables.
func flushOnShutdownEnabled() bool { return flushOnShutdown.Load() }

// FlushAllTables removes every ze-owned table the active backend applied, by
// clearing the registry and reconciling an empty desired state. The firewall
// engine calls this on a CLEAN shutdown (when flush-on-shutdown is enabled)
// before CloseBackend, so removal is a single ordered actor holding a live
// backend -- no race with the per-plugin withdraw paths. A no-op when no
// backend is loaded.
func FlushAllTables() error {
	tableRegistry.mu.Lock()
	tableRegistry.owners = make(map[string][]Table)
	tableRegistry.mu.Unlock()
	return ApplyAll()
}

// defaultBackendForAutoload is the backend ApplyAll loads on demand when a
// plugin (copp, policy-routes, ddos-local) registers tables but the operator
// wrote no firewall {} block, so no firewall config section ever loaded a
// backend. It mirrors defaultBackendName ("nft" on Linux, "" elsewhere) but is
// a var so host tests can inject a fake backend name where the OS default is
// empty (darwin).
var defaultBackendForAutoload = defaultBackendName

var tableRegistry = struct {
	mu     sync.Mutex
	owners map[string][]Table
}{
	owners: make(map[string][]Table),
}

// reconcileMu serializes the ENTIRE ApplyAll body -- snapshot AND apply -- so
// that at most one owner is ever inside Backend.Apply at a time. It is the
// OUTERMOST firewall lock; the lock order is:
//
//	reconcileMu -> tableRegistry.mu -> backendsMu -> backend-internal
//
// No ApplyAll call site holds tableRegistry.mu or backendsMu on entry (every
// caller releases both first; FlushAllTables unlocks tableRegistry.mu before
// calling ApplyAll), so acquiring reconcileMu first never inverts an existing
// order and cannot self-deadlock.
//
// It must span the whole body, not just b.Apply: the desired-state snapshot is
// taken under tableRegistry.mu and released before b.Apply runs. Locking only
// around b.Apply would let two callers apply STALE snapshots out of order and
// converge the kernel to a superseded state (owner A's older snapshot landing
// after owner B's newer one). Serializing snapshot+apply together makes each
// reconcile observe every registration that completed before it and forbids
// concurrent Backend.Apply, which no backend is required to tolerate (see the
// single-writer contract on Backend.Apply in backend.go).
var reconcileMu sync.Mutex

// RegisterTables stores a component's desired nftables tables under an
// owner key. Call ApplyAll to reconcile the merged set against the kernel.
func RegisterTables(owner string, tables []Table) {
	tableRegistry.mu.Lock()
	defer tableRegistry.mu.Unlock()
	if tables == nil {
		delete(tableRegistry.owners, owner)
		return
	}
	tableRegistry.owners[owner] = tables
}

// ApplyAll merges tables from all registered owners and calls
// backend.Apply with the full set. Tables with the same Name and Family
// from different owners are merged: their Chains, Sets, and Flowtables
// are concatenated so that e.g. a plugin can register sets for a table
// whose chains are owned by the firewall engine.
func ApplyAll() error {
	// Report the reconcile AFTER reconcileMu is released. Registered before the
	// unlock defer, so LIFO runs it last: observeApply writes a log line on a
	// timeout, and a syscall under the process-wide reconcile lock would extend
	// exactly the hold this spec exists to keep short.
	//
	// applyResult stays empty until Apply is entered, which is what keeps the
	// early returns below (no backend, autoload failure) out of the histogram:
	// no reconcile was attempted, so there is nothing to record.
	var (
		applyStart  time.Time
		applyResult string
		err         error
	)
	defer func() {
		if applyResult == "" {
			return
		}
		observeApply(time.Since(applyStart), applyResult, err)
	}()

	// Serialize the whole snapshot-plus-apply so at most one owner is inside
	// Backend.Apply at a time and no stale snapshot lands out of order. See the
	// reconcileMu doc for the lock order and rationale.
	reconcileMu.Lock()
	defer reconcileMu.Unlock()

	tableRegistry.mu.Lock()
	totalCap := 0
	for _, t := range tableRegistry.owners {
		totalCap += len(t)
	}
	all := make([]Table, 0, totalCap)
	owners := make([]string, 0, len(tableRegistry.owners))
	for owner := range tableRegistry.owners {
		owners = append(owners, owner)
	}
	sort.Strings(owners)
	for _, owner := range owners {
		all = append(all, tableRegistry.owners[owner]...)
	}
	tableRegistry.mu.Unlock()
	all = mergeSameNameTables(all)

	// A term can name a set that a DIFFERENT owner registers (the IRR leaves
	// are the case that exists today). The two owners meet here and nowhere
	// earlier, and they do not register in a fixed order: at startup the
	// firewall engine configures before the plugin that supplies the set, and
	// in a reload transaction the participants apply in whatever order the
	// orchestrator emits. Handing the backend a rule whose set is not there
	// yet fails the whole reconcile for every owner, so wait instead: the
	// supplying owner calls ApplyAll when it registers, and that run programs
	// the complete ruleset. Reported at WARN because a supplier that never
	// arrives leaves the ruleset unapplied, and the operator must be able to
	// see why.
	if tbl, set, waiting := unresolvedProvidedSet(all); waiting {
		if log := loggerPtr.Load(); log != nil {
			log.Warn("firewall: reconcile deferred, a rule names a set no owner has registered yet",
				"table", tbl, "set", set,
				"effect", "no firewall table is programmed until the owner of that set registers it")
		}
		return nil
	}

	backendsMu.Lock()
	// A plugin (copp, policy-routes, ddos-local) can register tables without
	// the operator writing a firewall {} block, so no firewall config section
	// ever loaded a backend. Load the OS default on demand so plugin-owned
	// tables still reach the kernel. Guarded on len(all) > 0 so a withdraw
	// (register nil + reconcile) with nothing loaded stays a no-op instead of
	// spinning up a backend just to apply an empty set.
	if activeBackend == nil && len(all) > 0 && defaultBackendForAutoload != "" {
		if err := loadBackendLocked(defaultBackendForAutoload); err != nil {
			backendsMu.Unlock()
			return err
		}
	}
	b := activeBackend
	backendsMu.Unlock()

	if b == nil {
		// No backend and nothing to apply: nothing to reconcile, so a
		// plugin withdraw before any backend loaded succeeds as a no-op.
		// No backend but tables pending (non-Linux, no OS default): surface
		// the not-loaded error as before.
		if len(all) == 0 {
			return nil
		}
		return errFirewallBackendNotLoaded
	}
	// Time every reconcile, and report a wedged dataplane at the one place that
	// calls Backend.Apply. An owner sees only its own failed apply; this layer is
	// where "the dataplane is behind the registry, for every owner" becomes
	// observable (metrics.go). The observation itself runs after the unlock, in
	// the defer registered at the top of this function.
	//
	// applyResult is set to panic BEFORE the call and overwritten only once Apply
	// has returned, so a backend that unwinds is recorded as a panic rather than
	// lost or filed as healthy.
	applyStart = time.Now()
	applyResult = applyResultPanic

	err = b.Apply(all)
	applyResult = applyResultOf(err)
	return err
}

type tableKey struct {
	name   string
	family TableFamily
}

func mergeSameNameTables(tables []Table) []Table {
	if len(tables) <= 1 {
		return tables
	}
	groups := make(map[tableKey]int, len(tables))
	merged := make([]Table, 0, len(tables))
	for _, t := range tables {
		k := tableKey{t.Name, t.Family}
		if idx, ok := groups[k]; ok {
			merged[idx].Chains = append(merged[idx].Chains, t.Chains...)
			merged[idx].Sets = append(merged[idx].Sets, t.Sets...)
			merged[idx].Flowtables = append(merged[idx].Flowtables, t.Flowtables...)
		} else {
			groups[k] = len(merged)
			merged = append(merged, t)
		}
	}
	return merged
}

// unresolvedProvidedSet reports the first match in tables that names a set
// another owner supplies (MatchInSet.ProvidedType is set) and that the merged
// tables do not declare. It returns the table and set names for the log line.
// A match with no ProvidedType is not examined here: ValidateTables already
// refused it at verify, where the operator sees the error.
func unresolvedProvidedSet(tables []Table) (tableName, setName string, found bool) {
	for i := range tables {
		tbl := &tables[i]
		declared := collectSetNames(tbl)
		for j := range tbl.Chains {
			for k := range tbl.Chains[j].Terms {
				for _, m := range tbl.Chains[j].Terms[k].Matches {
					in, ok := m.(MatchInSet)
					if !ok || in.ProvidedType == 0 {
						continue
					}
					if _, ok := declared[in.SetName]; !ok {
						return tbl.Name, in.SetName, true
					}
				}
			}
		}
	}
	return "", "", false
}
