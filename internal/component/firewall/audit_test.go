package firewall

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/report"
)

// VALIDATES: AC-15 -- ze_* tables in kernel but not in config produce
// firewall-stale-table warning.
// PREVENTS: Orphan nftables tables going unnoticed.
func TestFirewallStaleTableWarning(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	fb := &fakeBackend{
		tables: []Table{
			{Name: "ze_filter", Family: FamilyInet},
			{Name: "ze_orphan", Family: FamilyInet},
		},
	}
	resetBackendsForTest()
	_ = RegisterBackend("audit-test-stale", func() (Backend, error) { return fb, nil })
	_ = LoadBackend("audit-test-stale")
	defer func() { _ = CloseBackend() }()

	StoreLastApplied([]Table{
		{Name: "ze_filter", Family: FamilyInet},
	})
	defer StoreLastApplied(nil)

	findings, err := AuditTables()
	require.NoError(t, err)
	assert.Equal(t, 1, findings)

	warnings := report.Warnings()
	found := false
	for _, w := range warnings {
		if w.Code == reportCodeFirewallStaleTable {
			found = true
			assert.Contains(t, w.Message, "ze_orphan")
		}
	}
	if !found {
		t.Fatal("firewall-stale-table warning not raised for orphan table")
	}
}

// VALIDATES: AC-16 -- External modification of ze_* rules detected as drift.
// PREVENTS: Config/kernel divergence going unnoticed between Apply cycles.
func TestFirewallDriftWarning(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	fb := &fakeBackend{
		tables: []Table{
			{Name: "ze_filter", Family: FamilyInet, Chains: []Chain{
				{Name: "input"}, {Name: "forward"}, {Name: "extra-injected"},
			}},
		},
	}
	resetBackendsForTest()
	_ = RegisterBackend("audit-test-drift", func() (Backend, error) { return fb, nil })
	_ = LoadBackend("audit-test-drift")
	defer func() { _ = CloseBackend() }()

	StoreLastApplied([]Table{
		{Name: "ze_filter", Family: FamilyInet, Chains: []Chain{
			{Name: "input"}, {Name: "forward"},
		}},
	})
	defer StoreLastApplied(nil)

	findings, err := AuditTables()
	require.NoError(t, err)
	assert.Equal(t, 1, findings)

	warnings := report.Warnings()
	found := false
	for _, w := range warnings {
		if w.Code == reportCodeFirewallDrift {
			found = true
			assert.Contains(t, w.Message, "ze_filter")
		}
	}
	if !found {
		t.Fatal("firewall-drift warning not raised for modified table")
	}
}

// VALIDATES: AC-15/16 -- No warnings when kernel matches config.
// PREVENTS: False positive audit warnings.
func TestFirewallAuditClean(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	fb := &fakeBackend{
		tables: []Table{
			{Name: "ze_filter", Family: FamilyInet, Chains: []Chain{{Name: "input"}}},
		},
	}
	resetBackendsForTest()
	_ = RegisterBackend("audit-test-clean", func() (Backend, error) { return fb, nil })
	_ = LoadBackend("audit-test-clean")
	defer func() { _ = CloseBackend() }()

	StoreLastApplied([]Table{
		{Name: "ze_filter", Family: FamilyInet, Chains: []Chain{{Name: "input"}}},
	})
	defer StoreLastApplied(nil)

	findings, err := AuditTables()
	require.NoError(t, err)
	assert.Equal(t, 0, findings)

	warnings := report.Warnings()
	for _, w := range warnings {
		if w.Code == reportCodeFirewallStaleTable || w.Code == reportCodeFirewallDrift {
			t.Fatalf("unexpected warning %s raised on clean audit", w.Code)
		}
	}
}

// TestFirewallDriftDetectsATableTheKernelLost covers the drift the audit could
// not see: a table ze applied and the kernel no longer has.
//
// VALIDATES: a desired ze_* table absent from Backend.ListTables raises
// firewall-drift and counts as a finding.
// PREVENTS: the `continue` that skipped an absent table as "a different issue
// (apply failure)". The apply that installed it returned long ago, so an
// external `nft flush ruleset` removed it since, and nothing else looks: the
// stale-table branch only reports tables ze does NOT want. ze_vrrp carries the
// RFC 9568 Section 6.4.3 accept filter, so the table vanishing means a
// non-owner router accepts packets for the virtual address with no warning.
func TestFirewallDriftDetectsATableTheKernelLost(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	fb := &fakeBackend{
		tables: []Table{
			{Name: "ze_filter", Family: FamilyInet, Chains: []Chain{{Name: "input"}}},
		},
	}
	resetBackendsForTest()
	_ = RegisterBackend("audit-test-missing", func() (Backend, error) { return fb, nil })
	_ = LoadBackend("audit-test-missing")
	defer func() { _ = CloseBackend() }()

	StoreLastApplied([]Table{
		{Name: "ze_filter", Family: FamilyInet, Chains: []Chain{{Name: "input"}}},
		{Name: "ze_vrrp", Family: FamilyInet, Chains: []Chain{{Name: "input"}}},
	})
	defer StoreLastApplied(nil)

	findings, err := AuditTables()
	require.NoError(t, err)
	assert.Equal(t, 1, findings, "the vanished table is one finding")

	found := false
	for _, w := range report.Warnings() {
		if w.Code == reportCodeFirewallDrift {
			found = true
			assert.Contains(t, w.Message, "ze_vrrp")
		}
	}
	if !found {
		t.Fatal("firewall-drift warning not raised for a desired table the kernel lost")
	}
}

// TestFirewallAuditSaysWhenItCouldNotCheck covers the other half: an audit that
// never read the kernel must not answer like a clean one.
//
// VALIDATES: with tables applied and no backend loaded, AuditTables returns an
// error rather than zero findings.
// PREVENTS: the caller reading 0 as "checked and sound". checkFirewallHealth
// discarded the return entirely, so a process whose backend failed to load
// reported StatusHealthy for a kernel it had never looked at.
func TestFirewallAuditSaysWhenItCouldNotCheck(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	resetBackendsForTest()
	StoreLastApplied([]Table{{Name: "ze_vrrp", Family: FamilyInet}})
	defer StoreLastApplied(nil)

	findings, err := AuditTables()
	require.ErrorIs(t, err, errAuditNoBackend)
	assert.Equal(t, 0, findings, "an audit that did not run reports no finding")
}

// TestFirewallAuditIdleBeforeTheFirstApply pins the guard the error above must
// not swallow: before any apply nothing is desired, so nothing can have
// drifted and no backend is needed to say so.
func TestFirewallAuditIdleBeforeTheFirstApply(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	resetBackendsForTest()
	StoreLastApplied(nil)

	findings, err := AuditTables()
	require.NoError(t, err)
	assert.Equal(t, 0, findings)
}

// resetBackendsForTest clears the backend registry for test isolation.
func resetBackendsForTest() {
	backendsMu.Lock()
	defer backendsMu.Unlock()
	backends = make(map[string]func() (Backend, error))
	activeBackend = nil
}
