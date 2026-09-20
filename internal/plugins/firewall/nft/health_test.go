package firewallnft

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/health"
	"github.com/ze-software/ze/internal/core/report"
)

func TestFirewallHealthCheckHealthy(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	status, _ := checkFirewallHealth()
	assert.Equal(t, health.StatusHealthy, status, "healthy when no warnings")
}

func TestFirewallHealthCheckWarningCodes(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	report.RaiseWarning("firewall", "firewall-stale-table", "ze_filter", "stale", nil)
	status, reason := checkFirewallHealth()
	assert.Equal(t, health.StatusDegraded, status)
	assert.Contains(t, reason, "stale")
}

// auditStubBackend answers ListTables with a fixed set and nothing else. The
// health check reads only that method, so the other three refuse rather than
// returning a plausible empty answer.
type auditStubBackend struct {
	tables []firewall.Table
}

func (b *auditStubBackend) Apply([]firewall.Table) error { return errors.New("stub: Apply") }
func (b *auditStubBackend) ListTables() ([]firewall.Table, error) {
	return b.tables, nil
}

func (b *auditStubBackend) GetCounters(string) ([]firewall.ChainCounters, error) {
	return nil, errors.New("stub: GetCounters")
}
func (b *auditStubBackend) Close() error { return nil }

// TestFirewallHealthReportsATableTheKernelLost drives the health poll, not the
// audit helper, because the poll is where the answer reached the operator.
//
// VALIDATES: a desired ze_* table the kernel no longer has makes the firewall
// health check report StatusDegraded and name the table.
// PREVENTS: the audit's `continue` over an absent table. ze_vrrp carries the
// RFC 9568 Section 6.4.3 accept filter, so this poll was the only thing that
// could tell an operator a backup router had started accepting packets for the
// virtual address, and it answered healthy.
func TestFirewallHealthReportsATableTheKernelLost(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	stub := &auditStubBackend{tables: []firewall.Table{
		{Name: "ze_filter", Family: firewall.FamilyInet},
	}}
	require.NoError(t, firewall.RegisterBackend("nft-health-missing", func() (firewall.Backend, error) { return stub, nil }))
	require.NoError(t, firewall.LoadBackend("nft-health-missing"))
	defer func() { _ = firewall.CloseBackend() }()

	firewall.StoreLastApplied([]firewall.Table{
		{Name: "ze_filter", Family: firewall.FamilyInet},
		{Name: "ze_vrrp", Family: firewall.FamilyInet},
	})
	defer firewall.StoreLastApplied(nil)

	status, reason := checkFirewallHealth()
	assert.Equal(t, health.StatusDegraded, status)
	assert.Contains(t, reason, "ze_vrrp")
}

// TestFirewallHealthSaysWhenItCouldNotCheck pins the other outcome the check
// used to fold into healthy.
//
// VALIDATES: with tables applied and no backend loaded, the health check
// reports StatusDegraded and says the audit could not run.
// PREVENTS: checkFirewallHealth discarding the audit's return. Zero findings
// from an audit that never read the kernel is the absence of an answer, and it
// was published as "healthy".
func TestFirewallHealthSaysWhenItCouldNotCheck(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	require.NoError(t, firewall.CloseBackend())
	firewall.StoreLastApplied([]firewall.Table{{Name: "ze_vrrp", Family: firewall.FamilyInet}})
	defer firewall.StoreLastApplied(nil)

	status, reason := checkFirewallHealth()
	assert.Equal(t, health.StatusDegraded, status)
	assert.Contains(t, reason, "could not run")
}

func TestFirewallHealthCheckDriftWarning(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	report.RaiseWarning("firewall", "firewall-drift", "ze_filter", "drift detected", nil)
	status, reason := checkFirewallHealth()
	assert.Equal(t, health.StatusDegraded, status)
	assert.Contains(t, reason, "drift")
}
