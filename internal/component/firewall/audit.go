// Design: docs/architecture/doctor-and-health-checks.md -- firewall drift detection
// Related: accessor.go -- LastApplied snapshot
// Related: backend.go -- Backend.ListTables for kernel state

package firewall

import (
	"strconv"

	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const reportSourceFirewall = "firewall"
const reportCodeFirewallStaleTable = "firewall-stale-table"
const reportCodeFirewallDrift = "firewall-drift"

// AuditTables compares kernel state (from the active backend) against the
// last-applied config snapshot. Reports:
//   - firewall-stale-table (AC-15): ze_* tables in kernel not in config
//   - firewall-drift (AC-16): ze_* table content differs from config
//
// This is a read-only check; it never modifies nftables state.
// Returns the number of findings (0 = clean).
func AuditTables() int {
	desired := LastApplied()
	if desired == nil {
		return 0
	}
	backend := GetBackend()
	if backend == nil {
		return 0
	}

	actual, err := backend.ListTables()
	if err != nil {
		loggerPtr.Load().Warn("firewall audit: ListTables failed", "error", err)
		return 0
	}

	desiredNames := make(map[string]bool, len(desired))
	for _, t := range desired {
		desiredNames[t.Name] = true
	}

	findings := 0

	// AC-15: tables in kernel but not in config.
	var staleNames []string
	for _, t := range actual {
		if !desiredNames[t.Name] {
			staleNames = append(staleNames, t.Name)
		}
	}
	if len(staleNames) > 0 {
		report.RaiseWarning(reportSourceFirewall, reportCodeFirewallStaleTable, "audit",
			strconv.Itoa(len(staleNames))+" stale ze_* tables: "+textbuf.Join(staleNames, ", "),
			map[string]any{"tables": staleNames, "count": len(staleNames)})
		findings += len(staleNames)
	} else {
		report.ClearWarning(reportSourceFirewall, reportCodeFirewallStaleTable, "audit")
	}

	// AC-16: tables in both but with different chain counts (structural drift).
	actualMap := make(map[string]Table, len(actual))
	for _, t := range actual {
		actualMap[t.Name] = t
	}

	var driftNames []string
	for _, dt := range desired {
		at, exists := actualMap[dt.Name]
		if !exists {
			continue // missing table is a different issue (apply failure)
		}
		if len(at.Chains) != len(dt.Chains) {
			driftNames = append(driftNames, dt.Name)
		}
	}
	if len(driftNames) > 0 {
		report.RaiseWarning(reportSourceFirewall, reportCodeFirewallDrift, "audit",
			strconv.Itoa(len(driftNames))+" ze_* tables differ from config: "+textbuf.Join(driftNames, ", "),
			map[string]any{"tables": driftNames, "count": len(driftNames)})
		findings += len(driftNames)
	} else {
		report.ClearWarning(reportSourceFirewall, reportCodeFirewallDrift, "audit")
	}

	return findings
}
