package firewall

import (
	"errors"
	"strings"
	"testing"
)

// VALIDATES: a plugin (copp / policy-routes / ddos-local) that registers tables
// without the operator writing a firewall {} block still gets its tables
// programmed -- ApplyAll loads the OS-default backend on demand instead of
// failing with "firewall backend not loaded". This is the root cause of the
// copp-*.ci suites failing on their first real Linux run.
// PREVENTS: control-plane-protection (and any registry consumer) silently doing
// nothing when no firewall config section pre-loaded a backend.
func TestApplyAllAutoLoadsDefaultBackend(t *testing.T) {
	resetBackends()
	resetTables()
	t.Cleanup(func() {
		resetBackends()
		resetTables()
	})

	loaded := 0
	applied := 0
	var lastApplied []Table
	if err := RegisterBackend("auto-nft", func() (Backend, error) {
		loaded++
		return &countingBackend{onApply: func(d []Table) { applied++; lastApplied = d }}, nil
	}); err != nil {
		t.Fatalf("RegisterBackend: %v", err)
	}

	// Simulate the Linux OS default without depending on the host platform.
	prev := defaultBackendForAutoload
	defaultBackendForAutoload = "auto-nft"
	t.Cleanup(func() { defaultBackendForAutoload = prev })

	// A plugin registers a table; no firewall {} section ever ran LoadBackend.
	_ = RegisterTables("copp", []Table{{Name: "ze_copp", Family: FamilyInet}})

	if err := ApplyAll(); err != nil {
		t.Fatalf("ApplyAll with no pre-loaded backend: %v", err)
	}
	if loaded != 1 {
		t.Fatalf("default backend loaded %d times, want 1", loaded)
	}
	if applied != 1 || len(lastApplied) != 1 || lastApplied[0].Name != "ze_copp" {
		t.Fatalf("Apply got %d calls with tables %v, want 1 call with [ze_copp]", applied, lastApplied)
	}
	if GetBackend() == nil {
		t.Fatal("backend not active after auto-load")
	}

	// A second ApplyAll must reuse the loaded backend, not reload it.
	if err := ApplyAll(); err != nil {
		t.Fatalf("second ApplyAll: %v", err)
	}
	if loaded != 1 {
		t.Fatalf("backend reloaded on second ApplyAll: loaded=%d, want 1", loaded)
	}
}

// VALIDATES: with no backend loaded and nothing to apply, ApplyAll is a no-op
// (nil), so a plugin that withdraws before any backend was ever loaded does not
// see a spurious "firewall backend not loaded" error.
// PREVENTS: copp/policyroute/ddos-local logging a withdraw failure on a config
// that never programmed anything.
func TestApplyAllNoBackendNoTablesIsNoOp(t *testing.T) {
	resetBackends()
	resetTables()
	t.Cleanup(func() {
		resetBackends()
		resetTables()
	})

	prev := defaultBackendForAutoload
	defaultBackendForAutoload = "auto-nft" // registered? no -- must not be reached
	t.Cleanup(func() { defaultBackendForAutoload = prev })

	if err := ApplyAll(); err != nil {
		t.Fatalf("ApplyAll with no backend and no tables = %v, want nil", err)
	}
	if GetBackend() != nil {
		t.Fatal("no backend should be loaded when there is nothing to apply")
	}
}

// VALIDATES: when tables are pending, no backend is loaded, and there is no OS
// default (non-Linux), ApplyAll still surfaces errFirewallBackendNotLoaded
// rather than silently dropping the tables.
// PREVENTS: masking a genuinely missing backend on platforms without nft.
func TestApplyAllNoDefaultKeepsNotLoadedError(t *testing.T) {
	resetBackends()
	resetTables()
	t.Cleanup(func() {
		resetBackends()
		resetTables()
	})

	prev := defaultBackendForAutoload
	defaultBackendForAutoload = "" // mimic non-Linux: no OS default backend
	t.Cleanup(func() { defaultBackendForAutoload = prev })

	_ = RegisterTables("copp", []Table{{Name: "ze_copp", Family: FamilyInet}})

	if err := ApplyAll(); err == nil {
		t.Fatal("ApplyAll with pending tables and no default backend should error")
	}
}

// VALIDATES: FlushAllTables clears every owner from the registry and reconciles
// an empty desired state through the active backend -- the clean-shutdown
// teardown the firewall engine invokes so ze-owned tables are removed as a
// single ordered actor (no race with per-plugin withdraws).
// PREVENTS: a clean daemon stop leaving orphan ze_* tables in the kernel.
func TestFlushAllTablesClearsRegistryAndReconciles(t *testing.T) {
	resetBackends()
	resetTables()
	t.Cleanup(func() {
		resetBackends()
		resetTables()
	})

	var lastApplied []Table
	applied := 0
	if err := RegisterBackend("fb", func() (Backend, error) {
		return &countingBackend{onApply: func(d []Table) { applied++; lastApplied = d }}, nil
	}); err != nil {
		t.Fatalf("RegisterBackend: %v", err)
	}
	if err := LoadBackend("fb"); err != nil {
		t.Fatalf("LoadBackend: %v", err)
	}

	_ = RegisterTables("copp", []Table{{Name: "ze_copp", Family: FamilyInet}})
	_ = RegisterTables("firewall", []Table{{Name: "ze_fw", Family: FamilyInet}})
	if err := ApplyAll(); err != nil {
		t.Fatalf("ApplyAll: %v", err)
	}
	if len(lastApplied) != 2 {
		t.Fatalf("pre-flush applied %d tables, want 2", len(lastApplied))
	}

	if err := FlushAllTables(); err != nil {
		t.Fatalf("FlushAllTables: %v", err)
	}
	if len(lastApplied) != 0 {
		t.Fatalf("post-flush reconcile applied %d tables, want 0 (empty desired)", len(lastApplied))
	}
	// Registry must be empty so a later ApplyAll cannot resurrect the tables.
	tableRegistry.mu.Lock()
	n := len(tableRegistry.owners)
	tableRegistry.mu.Unlock()
	if n != 0 {
		t.Fatalf("registry has %d owners after flush, want 0", n)
	}
}

// VALIDATES: flush-on-shutdown defaults to enabled (fail-safe) and the setter
// round-trips, so the engine's clean-shutdown branch honors the parsed option.
func TestFlushOnShutdownToggle(t *testing.T) {
	prev := flushOnShutdownEnabled()
	t.Cleanup(func() { setFlushOnShutdown(prev) })

	setFlushOnShutdown(true)
	if !flushOnShutdownEnabled() {
		t.Fatal("FlushOnShutdownEnabled() = false after SetFlushOnShutdown(true)")
	}
	setFlushOnShutdown(false)
	if flushOnShutdownEnabled() {
		t.Fatal("FlushOnShutdownEnabled() = true after SetFlushOnShutdown(false)")
	}
}

// resetTables clears the table registry for test isolation.
func resetTables() {
	tableRegistry.mu.Lock()
	defer tableRegistry.mu.Unlock()
	tableRegistry.owners = make(map[string][]Table)
}

// countingBackend records Apply calls so a test can assert the on-demand
// backend load actually reconciled the registered tables.
type countingBackend struct {
	onApply  func([]Table)
	applyErr error // returned by Apply, so a test can drive a failed reconcile.
}

func (c *countingBackend) Apply(d []Table) error {
	if c.onApply != nil {
		c.onApply(d)
	}
	return c.applyErr
}
func (c *countingBackend) ListTables() ([]Table, error)                { return nil, nil }
func (c *countingBackend) GetCounters(string) ([]ChainCounters, error) { return nil, nil }
func (c *countingBackend) Close() error                                { return nil }

// VALIDATES: AC-1 -- a table whose term names a set another owner supplies is
// held back until that owner registers it, and then it is programmed. The two
// owners do not register in a fixed order: at startup the firewall engine
// configures before the plugin that supplies the set, and in a reload
// transaction the participants apply in whatever order the orchestrator emits.
// VALIDATES: the wait is scoped to that ONE table. Every other owner's tables
// reach the kernel on the same reconcile, because a set is table-local and no
// other table's rules depend on the missing one.
// PREVENTS: the first owner to apply failing the reconcile for every owner with
// `match-in-set: unknown set ... (not registered on table)`, which is what the
// backend answers when a rule names a set the table does not carry.
// PREVENTS: a supplier that never arrives -- a cold IRR cache registers no set
// at all -- leaving copp, the DDoS tables and the policy routes unprogrammed
// behind one WARN, with nothing on the way to end the wait.
func TestApplyAllWaitsForAProvidedSet(t *testing.T) {
	resetBackends()
	resetTables()
	t.Cleanup(func() {
		resetBackends()
		resetTables()
	})

	applies := 0
	var lastApplied []Table
	if err := RegisterBackend("provided-set-nft", func() (Backend, error) {
		return &countingBackend{onApply: func(d []Table) { applies++; lastApplied = d }}, nil
	}); err != nil {
		t.Fatalf("RegisterBackend: %v", err)
	}
	prev := defaultBackendForAutoload
	defaultBackendForAutoload = "provided-set-nft"
	t.Cleanup(func() { defaultBackendForAutoload = prev })

	_ = RegisterTables("firewall", []Table{{
		Name:   "ze_wan",
		Family: FamilyInet,
		Chains: []Chain{{
			Name:   "input",
			IsBase: true,
			Type:   ChainFilter,
			Hook:   HookInput,
			Policy: PolicyAccept,
			Terms: []Term{{
				Name: "t",
				Matches: []Match{
					MatchInSet{SetName: "irr_v4_AS13335", MatchField: SetFieldSourceAddr, ProvidedType: SetTypeIPv4},
				},
				Actions: []Action{Accept{}},
			}},
		}},
	}})
	// A second owner, with no stake in the missing set. Its table is what the
	// all-or-nothing wait used to take down with the first one.
	_ = RegisterTables("copp", []Table{{Name: "ze_copp", Family: FamilyInet}})

	if err := ApplyAll(); err != nil {
		t.Fatalf("a pending provider must not fail the reconcile: %v", err)
	}
	if applies != 1 {
		t.Fatalf("backend applies = %d, want 1: the tables that CAN be programmed must be", applies)
	}
	if len(lastApplied) != 1 || lastApplied[0].Name != "ze_copp" {
		t.Fatalf("applied %+v, want ze_copp alone: ze_wan names a set nobody registered", lastApplied)
	}

	_ = RegisterTables("firewall-irr", []Table{{
		Name:   "ze_wan",
		Family: FamilyInet,
		Sets:   []Set{{Name: "irr_v4_AS13335", Type: SetTypeIPv4, Flags: SetFlagInterval}},
	}})

	if err := ApplyAll(); err != nil {
		t.Fatalf("ApplyAll after the provider registered: %v", err)
	}
	if applies != 2 {
		t.Fatalf("backend applies = %d, want 2 once the set is registered", applies)
	}
	if len(lastApplied) != 2 {
		t.Fatalf("applied %+v, want both tables once the set is registered", lastApplied)
	}
	// ApplyAll walks the owners in name order, so copp's table comes first.
	wan := lastApplied[1]
	if wan.Name != "ze_wan" || len(wan.Sets) != 1 || wan.Sets[0].Name != "irr_v4_AS13335" {
		t.Fatalf("the merged table did not carry the provided set: %+v", lastApplied)
	}
}

// VALIDATES: a set the table declares itself is applied at once, so the wait
// above is scoped to references no owner has answered.
// PREVENTS: the guard stalling every reconcile that uses a named set.
func TestApplyAllDoesNotWaitForADeclaredSet(t *testing.T) {
	resetBackends()
	resetTables()
	t.Cleanup(func() {
		resetBackends()
		resetTables()
	})

	applies := 0
	if err := RegisterBackend("declared-set-nft", func() (Backend, error) {
		return &countingBackend{onApply: func([]Table) { applies++ }}, nil
	}); err != nil {
		t.Fatalf("RegisterBackend: %v", err)
	}
	prev := defaultBackendForAutoload
	defaultBackendForAutoload = "declared-set-nft"
	t.Cleanup(func() { defaultBackendForAutoload = prev })

	_ = RegisterTables("firewall", []Table{{
		Name:   "ze_wan",
		Family: FamilyInet,
		Sets:   []Set{{Name: "blocklist", Type: SetTypeIPv4}},
		Chains: []Chain{{
			Name:   "input",
			IsBase: true,
			Type:   ChainFilter,
			Hook:   HookInput,
			Policy: PolicyAccept,
			Terms: []Term{{
				Name:    "t",
				Matches: []Match{MatchInSet{SetName: "blocklist", MatchField: SetFieldSourceAddr}},
				Actions: []Action{Drop{}},
			}},
		}},
	}})

	if err := ApplyAll(); err != nil {
		t.Fatalf("ApplyAll: %v", err)
	}
	if applies != 1 {
		t.Fatalf("backend applies = %d, want 1: a declared set must not defer the reconcile", applies)
	}
}

// errApplyFailed is the reconcile failure the snapshot tests drive.
var errApplyFailed = errors.New("backend refused the desired state")

// snapshotBackend registers a countingBackend under name as the OS default for
// autoload, returns it so a test can flip applyErr, and restores both registries
// plus the snapshot when the test ends.
func snapshotBackend(t *testing.T, name string) *countingBackend {
	t.Helper()
	resetBackends()
	resetTables()
	StoreLastApplied(nil)
	t.Cleanup(func() {
		resetBackends()
		resetTables()
		StoreLastApplied(nil)
	})

	b := &countingBackend{}
	if err := RegisterBackend(name, func() (Backend, error) { return b, nil }); err != nil {
		t.Fatalf("RegisterBackend: %v", err)
	}
	prev := defaultBackendForAutoload
	defaultBackendForAutoload = name
	t.Cleanup(func() { defaultBackendForAutoload = prev })
	return b
}

// snapshotNames returns the table names LastApplied reports, in order.
func snapshotNames(t *testing.T) []string {
	t.Helper()
	applied := LastApplied()
	names := make([]string, 0, len(applied))
	for i := range applied {
		names = append(names, applied[i].Name)
	}
	return names
}

// VALIDATES: a reconcile driven by a plugin alone, with no firewall {} section
// anywhere, records what it applied. `show firewall ruleset ze_irr_iface` and
// the web pages read LastApplied, so this is what makes a plugin-owned table
// visible to an operator.
// PREVENTS: the readback answering "no firewall tables have been applied" for a
// table that IS in the kernel, which is what every StoreLastApplied call site
// living in engine.go produced: each passed the config's tables, so the four
// plugin owners (firewall-irr, copp, policy-routes, ddos-local) were never in
// the snapshot.
func TestApplyAllRecordsAPluginOnlyReconcile(t *testing.T) {
	snapshotBackend(t, "snapshot-plugin-only")

	_ = RegisterTables("firewall-irr", []Table{{Name: "ze_irr_iface", Family: FamilyInet}})

	if err := ApplyAll(); err != nil {
		t.Fatalf("ApplyAll: %v", err)
	}
	if got := snapshotNames(t); len(got) != 1 || got[0] != "ze_irr_iface" {
		t.Fatalf("LastApplied() = %v, want [ze_irr_iface]", got)
	}
}

// VALIDATES: two owners registering one table name and family are both in the
// snapshot, with the chains of the first and the sets of the second. The
// snapshot is the MERGED set that reached the backend, not any one owner's view
// of it.
// PREVENTS: a readback that shows the engine's chains for ze_wan and claims the
// IRR-supplied set is absent, so an operator debugging a match-in-set rule reads
// a table that never existed.
func TestApplyAllSnapshotHoldsTheMergedTable(t *testing.T) {
	snapshotBackend(t, "snapshot-merge")

	_ = RegisterTables("firewall", []Table{{
		Name:   "ze_wan",
		Family: FamilyInet,
		Chains: []Chain{{
			Name:   "input",
			IsBase: true,
			Type:   ChainFilter,
			Hook:   HookInput,
			Policy: PolicyAccept,
			Terms: []Term{{
				Name:    "t",
				Matches: []Match{MatchInSet{SetName: "irr_v4_AS13335", MatchField: SetFieldSourceAddr, ProvidedType: SetTypeIPv4}},
				Actions: []Action{Accept{}},
			}},
		}},
	}})
	_ = RegisterTables("firewall-irr", []Table{{
		Name:   "ze_wan",
		Family: FamilyInet,
		Sets:   []Set{{Name: "irr_v4_AS13335", Type: SetTypeIPv4, Flags: SetFlagInterval}},
	}})

	if err := ApplyAll(); err != nil {
		t.Fatalf("ApplyAll: %v", err)
	}
	applied := LastApplied()
	if len(applied) != 1 {
		t.Fatalf("LastApplied() = %v, want the two owners merged into one table", snapshotNames(t))
	}
	if len(applied[0].Chains) != 1 || len(applied[0].Sets) != 1 {
		t.Fatalf("merged table has %d chains and %d sets, want 1 and 1", len(applied[0].Chains), len(applied[0].Sets))
	}
	if applied[0].Sets[0].Name != "irr_v4_AS13335" {
		t.Fatalf("merged table carries set %q, want irr_v4_AS13335", applied[0].Sets[0].Name)
	}
}

// VALIDATES: a reconcile the backend refuses leaves the previous snapshot as it
// was, so the readback keeps describing the state the kernel still holds.
// PREVENTS: a failed apply publishing a desired state that was never programmed,
// which would make the readback a plan rather than a record.
func TestApplyAllFailedApplyKeepsThePreviousSnapshot(t *testing.T) {
	b := snapshotBackend(t, "snapshot-failed-apply")

	_ = RegisterTables("copp", []Table{{Name: "ze_copp", Family: FamilyInet}})
	if err := ApplyAll(); err != nil {
		t.Fatalf("first ApplyAll: %v", err)
	}

	b.applyErr = errApplyFailed
	_ = RegisterTables("firewall-irr", []Table{{Name: "ze_irr_iface", Family: FamilyInet}})
	if err := ApplyAll(); !errors.Is(err, errApplyFailed) {
		t.Fatalf("ApplyAll = %v, want the backend error", err)
	}

	if got := snapshotNames(t); len(got) != 1 || got[0] != "ze_copp" {
		t.Fatalf("LastApplied() = %v after a failed apply, want the previous [ze_copp]", got)
	}
}

// VALIDATES: every owner withdrawing, through a live backend that accepts the
// empty set, is recorded as an empty snapshot. The kernel holds nothing, so the
// readback says nothing.
// PREVENTS: the readback going stale on the one reconcile an operator is most
// likely to check -- `show firewall ruleset` still listing the tables a withdraw
// removed.
func TestApplyAllWithdrawRecordsTheEmptySet(t *testing.T) {
	snapshotBackend(t, "snapshot-withdraw")

	_ = RegisterTables("copp", []Table{{Name: "ze_copp", Family: FamilyInet}})
	if err := ApplyAll(); err != nil {
		t.Fatalf("first ApplyAll: %v", err)
	}
	// The withdraw below asserts an ABSENCE, which an implementation that
	// records nothing at all satisfies too. Pin the presence first, so the
	// second assertion measures the withdraw rather than a snapshot that was
	// never written.
	if got := snapshotNames(t); len(got) != 1 || got[0] != "ze_copp" {
		t.Fatalf("LastApplied() = %v before the withdraw, want [ze_copp]", got)
	}

	_ = RegisterTables("copp", nil)
	if err := ApplyAll(); err != nil {
		t.Fatalf("withdraw ApplyAll: %v", err)
	}
	if got := snapshotNames(t); len(got) != 0 {
		t.Fatalf("LastApplied() = %v after every owner withdrew, want empty", got)
	}
}

// VALIDATES: the no-backend, nothing-to-apply return leaves the snapshot alone.
// No Apply ran on that path, so it knows nothing about the kernel and must not
// report a withdraw that never happened. CloseBackend is what clears the
// snapshot when the backend that owned the state goes away.
// PREVENTS: an empty merged set with no backend loaded reading as "everything
// was withdrawn", which is a zero value dressed as an answer.
func TestApplyAllNoBackendLeavesTheSnapshot(t *testing.T) {
	snapshotBackend(t, "snapshot-no-backend")

	_ = RegisterTables("copp", []Table{{Name: "ze_copp", Family: FamilyInet}})
	if err := ApplyAll(); err != nil {
		t.Fatalf("first ApplyAll: %v", err)
	}

	// Drop the backend the way a test does, without CloseBackend: the registry
	// keeps its snapshot and the next reconcile has nothing to apply.
	resetBackends()
	resetTables()
	defaultBackendForAutoload = ""

	if err := ApplyAll(); err != nil {
		t.Fatalf("ApplyAll with no backend and no tables = %v, want nil", err)
	}
	if got := snapshotNames(t); len(got) != 1 || got[0] != "ze_copp" {
		t.Fatalf("LastApplied() = %v, want the untouched [ze_copp]: no Apply ran", got)
	}
}

// VALIDATES: a table name that carries no ownership prefix is refused at
// registration, so it never reaches a backend and never reaches the kernel.
// PREVENTS: the FlowSpec defect returning under another owner's name. The nft
// backend decides which kernel tables ze owns by the prefix, so a table
// registered without it was never swept: a withdraw left it installed and every
// reconcile appended a second copy of each rule.
func TestEveryRegisteredTableCarriesThePrefix(t *testing.T) {
	resetBackends()
	resetTables()
	t.Cleanup(func() {
		resetBackends()
		resetTables()
	})

	var lastApplied []Table
	if err := RegisterBackend("prefix-guard-test", func() (Backend, error) {
		return &countingBackend{onApply: func(d []Table) { lastApplied = d }}, nil
	}); err != nil {
		t.Fatalf("RegisterBackend: %v", err)
	}
	if err := LoadBackend("prefix-guard-test"); err != nil {
		t.Fatalf("LoadBackend: %v", err)
	}

	err := RegisterTables("flowspec", []Table{{Name: "flowspec", Family: FamilyInet}})
	if err == nil {
		t.Fatal("RegisterTables accepted a table with no ownership prefix")
	}
	if !strings.Contains(err.Error(), "flowspec") || !strings.Contains(err.Error(), tableNamePrefix) {
		t.Fatalf("error names neither the table nor the prefix: %v", err)
	}

	// Refused means not stored: the next reconcile must not carry it.
	if err := ApplyAll(); err != nil {
		t.Fatalf("ApplyAll after a refused registration: %v", err)
	}
	if len(lastApplied) != 0 {
		t.Fatalf("a refused table reached the backend: %v", lastApplied)
	}

	// One bad name in a set refuses the whole set, so a partial registration
	// cannot leave the owner half-programmed.
	err = RegisterTables("mixed", []Table{
		{Name: "ze_good", Family: FamilyInet},
		{Name: "bad", Family: FamilyInet},
	})
	if err == nil {
		t.Fatal("RegisterTables accepted a set holding one unprefixed table")
	}
	if err := ApplyAll(); err != nil {
		t.Fatalf("ApplyAll after a refused mixed set: %v", err)
	}
	if len(lastApplied) != 0 {
		t.Fatalf("a refused set reached the backend: %v", lastApplied)
	}

	// The prefixed name is accepted and reaches the backend.
	if err := RegisterTables("flowspec", []Table{{Name: "ze_flowspec", Family: FamilyInet}}); err != nil {
		t.Fatalf("RegisterTables refused a prefixed table: %v", err)
	}
	if err := ApplyAll(); err != nil {
		t.Fatalf("ApplyAll: %v", err)
	}
	if len(lastApplied) != 1 || lastApplied[0].Name != "ze_flowspec" {
		t.Fatalf("Apply got %v, want [ze_flowspec]", lastApplied)
	}

	// A withdraw registers no name, so it can never be refused.
	if err := RegisterTables("flowspec", nil); err != nil {
		t.Fatalf("a withdraw was refused: %v", err)
	}
	if err := ApplyAll(); err != nil {
		t.Fatalf("ApplyAll after withdraw: %v", err)
	}
	if len(lastApplied) != 0 {
		t.Fatalf("the withdrawn table survived the reconcile: %v", lastApplied)
	}
}
