package firewall

import (
	"testing"

	"github.com/stretchr/testify/assert"

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

	findings := AuditTables()
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

	findings := AuditTables()
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

	findings := AuditTables()
	assert.Equal(t, 0, findings)

	warnings := report.Warnings()
	for _, w := range warnings {
		if w.Code == reportCodeFirewallStaleTable || w.Code == reportCodeFirewallDrift {
			t.Fatalf("unexpected warning %s raised on clean audit", w.Code)
		}
	}
}

// resetBackendsForTest clears the backend registry for test isolation.
func resetBackendsForTest() {
	backendsMu.Lock()
	defer backendsMu.Unlock()
	backends = make(map[string]func() (Backend, error))
	activeBackend = nil
}
