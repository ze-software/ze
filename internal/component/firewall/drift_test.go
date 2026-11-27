package firewall

import (
	"errors"
	"maps"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/drift"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/textbuf"
)

type driftTestBackend struct {
	tables  []Table
	listErr error
}

func (b *driftTestBackend) Apply([]Table) error                         { return nil }
func (b *driftTestBackend) ListTables() ([]Table, error)                { return b.tables, b.listErr }
func (b *driftTestBackend) GetCounters(string) ([]ChainCounters, error) { return nil, nil }
func (b *driftTestBackend) Close() error                                { return nil }

func setupFirewallDriftTest(t *testing.T, b Backend) {
	t.Helper()

	backendsMu.Lock()
	prevBackends := maps.Clone(backends)
	prevVerifiers := maps.Clone(verifiers)
	prevActive := activeBackend
	prevName := ActiveBackendName()
	activeBackend = b
	backendsMu.Unlock()

	prevApplied := LastApplied()

	t.Cleanup(func() {
		StoreLastApplied(prevApplied)
		backendsMu.Lock()
		backends = prevBackends
		verifiers = prevVerifiers
		activeBackend = prevActive
		setActiveBackendName(prevName)
		backendsMu.Unlock()
	})
}

// VALIDATES: checkFirewallDrift returns StatusClean with zero checked when LastApplied is empty.
// PREVENTS: idle firewall plugin reporting drift or unavailability before first apply.
func TestFirewallDriftEmptyDesiredClean(t *testing.T) {
	setupFirewallDriftTest(t, nil)
	StoreLastApplied(nil)

	out := checkFirewallDrift()
	assert.Equal(t, driftSubsystemFirewall, out.Subsystem)
	assert.Equal(t, string(drift.StatusClean), out.Status)
	assert.Equal(t, 0, out.Checked)
	assert.Equal(t, 0, out.Drifted)
	assert.False(t, out.Truncated)
	require.NotNil(t, out.Findings)
	assert.Empty(t, out.Findings)
}

// VALIDATES: checkFirewallDrift returns StatusUnavailable when desired tables exist and backend is nil, without loading a backend.
// PREVENTS: read-only drift check from calling LoadBackend or falsely reporting clean without reading kernel state.
func TestFirewallDriftNilBackendUnavailable(t *testing.T) {
	setupFirewallDriftTest(t, nil)
	StoreLastApplied([]Table{{Name: "ze_filter", Family: FamilyInet}})

	out := checkFirewallDrift()
	assert.Equal(t, driftSubsystemFirewall, out.Subsystem)
	assert.Equal(t, string(drift.StatusUnavailable), out.Status)
	assert.Equal(t, 0, out.Checked)
	assert.Equal(t, 0, out.Drifted)
	require.NotNil(t, out.Findings)
	assert.Empty(t, out.Findings)
	assert.Nil(t, GetBackend(), "drift check must never call LoadBackend")
}

// VALIDATES: checkFirewallDrift returns StatusClean when live tables, chains, and term counts match LastApplied, without mutating report warnings.
// PREVENTS: false-positive drift findings or side-effecting report warnings on clean state.
func TestFirewallDriftCleanMatch(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	tables := []Table{
		{
			Name:   "ze_filter",
			Family: FamilyInet,
			Chains: []Chain{
				{Name: "input", Terms: []Term{{Name: "allow-ssh"}, {Name: "drop-rest"}}},
				{Name: "forward", Terms: []Term{{Name: "allow-established"}}},
			},
		},
	}
	setupFirewallDriftTest(t, &driftTestBackend{tables: tables})
	StoreLastApplied(tables)

	out := checkFirewallDrift()
	assert.Equal(t, driftSubsystemFirewall, out.Subsystem)
	assert.Equal(t, string(drift.StatusClean), out.Status)
	assert.Equal(t, 1, out.Checked)
	assert.Equal(t, 0, out.Drifted)
	assert.False(t, out.Truncated)
	require.NotNil(t, out.Findings)
	assert.Empty(t, out.Findings)
	assert.Empty(t, report.Warnings(), "drift check must not mutate report warnings")
}

// VALIDATES: checkFirewallDrift reports KindMissing with stripped table name when a desired table is absent from the kernel.
// PREVENTS: vanished ze_* tables going undetected.
func TestFirewallDriftMissingTable(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	setupFirewallDriftTest(t, &driftTestBackend{
		tables: []Table{{Name: "ze_filter", Family: FamilyInet}},
	})
	StoreLastApplied([]Table{
		{Name: "ze_filter", Family: FamilyInet},
		{Name: "ze_vrrp", Family: FamilyInet},
	})

	out := checkFirewallDrift()
	assert.Equal(t, string(drift.StatusDrifted), out.Status)
	assert.Equal(t, 2, out.Checked)
	assert.Equal(t, 1, out.Drifted)
	require.Len(t, out.Findings, 1)
	assert.Equal(t, "vrrp", out.Findings[0].Subject)
	assert.Equal(t, string(drift.KindMissing), out.Findings[0].Kind)
	assert.Equal(t, driftStatePresent, out.Findings[0].Expected)
	assert.Equal(t, driftStateAbsent, out.Findings[0].Actual)
	assert.Empty(t, report.Warnings(), "drift check must not raise report warnings")
}

// VALIDATES: checkFirewallDrift reports KindExtra for orphan ze_* tables while ignoring non-ze_* foreign tables.
// PREVENTS: stale ze_* tables going unnoticed or non-owned tables being flagged as drift.
func TestFirewallDriftExtraStaleTable(t *testing.T) {
	setupFirewallDriftTest(t, &driftTestBackend{
		tables: []Table{
			{Name: "ze_filter", Family: FamilyInet},
			{Name: "ze_orphan", Family: FamilyInet},
			{Name: "foreign_table", Family: FamilyInet},
		},
	})
	StoreLastApplied([]Table{{Name: "ze_filter", Family: FamilyInet}})

	out := checkFirewallDrift()
	assert.Equal(t, string(drift.StatusDrifted), out.Status)
	assert.Equal(t, 1, out.Checked)
	assert.Equal(t, 1, out.Drifted)
	require.Len(t, out.Findings, 1)
	assert.Equal(t, "orphan", out.Findings[0].Subject)
	assert.Equal(t, string(drift.KindExtra), out.Findings[0].Kind)
	assert.Equal(t, driftStateAbsent, out.Findings[0].Expected)
	assert.Equal(t, driftStatePresent, out.Findings[0].Actual)
}

// VALIDATES: checkFirewallDrift reports KindMismatch when actual chain count differs from desired.
// PREVENTS: externally added or removed chains within a ze_* table going unnoticed.
func TestFirewallDriftChainCountMismatch(t *testing.T) {
	setupFirewallDriftTest(t, &driftTestBackend{
		tables: []Table{
			{
				Name:   "ze_filter",
				Family: FamilyInet,
				Chains: []Chain{{Name: "input"}, {Name: "forward"}, {Name: "injected"}},
			},
		},
	})
	StoreLastApplied([]Table{
		{
			Name:   "ze_filter",
			Family: FamilyInet,
			Chains: []Chain{{Name: "input"}, {Name: "forward"}},
		},
	})

	out := checkFirewallDrift()
	assert.Equal(t, string(drift.StatusDrifted), out.Status)
	assert.Equal(t, 1, out.Checked)
	assert.Equal(t, 1, out.Drifted)
	require.Len(t, out.Findings, 1)
	assert.Equal(t, "filter", out.Findings[0].Subject)
	assert.Equal(t, string(drift.KindMismatch), out.Findings[0].Kind)
	assert.Equal(t, "2 chains", out.Findings[0].Expected)
	assert.Equal(t, "3 chains", out.Findings[0].Actual)
}

// VALIDATES: checkFirewallDrift reports KindMismatch on "<table>/<chain>" when matching chain term counts differ.
// PREVENTS: rule insertion or deletion inside an existing chain going unnoticed.
func TestFirewallDriftTermCountMismatch(t *testing.T) {
	setupFirewallDriftTest(t, &driftTestBackend{
		tables: []Table{
			{
				Name:   "ze_filter",
				Family: FamilyInet,
				Chains: []Chain{
					{Name: "input", Terms: []Term{{Name: "allow-ssh"}}},
				},
			},
		},
	})
	StoreLastApplied([]Table{
		{
			Name:   "ze_filter",
			Family: FamilyInet,
			Chains: []Chain{
				{Name: "input", Terms: []Term{{Name: "allow-ssh"}, {Name: "allow-bgp"}, {Name: "drop-rest"}}},
			},
		},
	})

	out := checkFirewallDrift()
	assert.Equal(t, string(drift.StatusDrifted), out.Status)
	assert.Equal(t, 1, out.Checked)
	assert.Equal(t, 1, out.Drifted)
	require.Len(t, out.Findings, 1)
	assert.Equal(t, "filter/input", out.Findings[0].Subject)
	assert.Equal(t, string(drift.KindMismatch), out.Findings[0].Kind)
	assert.Equal(t, "3 terms", out.Findings[0].Expected)
	assert.Equal(t, "1 terms", out.Findings[0].Actual)
}

// VALIDATES: checkFirewallDrift returns StatusError with Error text when Backend.ListTables fails.
// PREVENTS: backend read errors being misreported as clean or silently ignored.
func TestFirewallDriftBackendListError(t *testing.T) {
	setupFirewallDriftTest(t, &driftTestBackend{
		listErr: errors.New("netlink permission denied"),
	})
	StoreLastApplied([]Table{{Name: "ze_filter", Family: FamilyInet}})

	out := checkFirewallDrift()
	assert.Equal(t, string(drift.StatusError), out.Status)
	assert.Equal(t, "netlink permission denied", out.Error)
	require.NotNil(t, out.Findings)
	assert.Empty(t, out.Findings)
}

// VALIDATES: checkFirewallDrift caps findings at drift.MaxFindingsPerSubsystem (64) and sets Truncated.
// PREVENTS: unbounded finding slices over RPC when many firewall tables drift.
func TestFirewallDriftTruncation(t *testing.T) {
	const total = drift.MaxFindingsPerSubsystem + 6
	desired := make([]Table, 0, total)
	for i := range total {
		var tb textbuf.Buffer
		desired = append(desired, Table{
			Name:   tb.Str("ze_tbl_").Int(int64(i)).String(),
			Family: FamilyInet,
		})
	}
	setupFirewallDriftTest(t, &driftTestBackend{tables: nil})
	StoreLastApplied(desired)

	out := checkFirewallDrift()
	assert.Equal(t, string(drift.StatusDrifted), out.Status)
	assert.Equal(t, total, out.Checked)
	assert.Equal(t, total, out.Drifted)
	assert.True(t, out.Truncated)
	assert.Len(t, out.Findings, drift.MaxFindingsPerSubsystem)

	handlerOut, err := handleDriftCheck(driftSubsystemFirewall)
	require.NoError(t, err)
	assert.Equal(t, out, handlerOut)
}
