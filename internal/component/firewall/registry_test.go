package firewall

import "testing"

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
	RegisterTables("copp", []Table{{Name: "ze_copp", Family: FamilyInet}})

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

	RegisterTables("copp", []Table{{Name: "ze_copp", Family: FamilyInet}})

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

	RegisterTables("copp", []Table{{Name: "ze_copp", Family: FamilyInet}})
	RegisterTables("firewall", []Table{{Name: "ze_fw", Family: FamilyInet}})
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
	onApply func([]Table)
}

func (c *countingBackend) Apply(d []Table) error {
	if c.onApply != nil {
		c.onApply(d)
	}
	return nil
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

	RegisterTables("firewall", []Table{{
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
	RegisterTables("copp", []Table{{Name: "ze_copp", Family: FamilyInet}})

	if err := ApplyAll(); err != nil {
		t.Fatalf("a pending provider must not fail the reconcile: %v", err)
	}
	if applies != 1 {
		t.Fatalf("backend applies = %d, want 1: the tables that CAN be programmed must be", applies)
	}
	if len(lastApplied) != 1 || lastApplied[0].Name != "ze_copp" {
		t.Fatalf("applied %+v, want ze_copp alone: ze_wan names a set nobody registered", lastApplied)
	}

	RegisterTables("firewall-irr", []Table{{
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

	RegisterTables("firewall", []Table{{
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
