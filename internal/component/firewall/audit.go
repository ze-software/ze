// Design: docs/architecture/doctor-and-health-checks.md -- firewall drift detection
// Related: accessor.go -- LastApplied snapshot
// Related: backend.go -- Backend.ListTables for kernel state

package firewall

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const reportSourceFirewall = "firewall"
const reportCodeFirewallStaleTable = "firewall-stale-table"
const reportCodeFirewallDrift = "firewall-drift"

// errAuditNoBackend says the audit had tables to check and no way to read the
// kernel. It is not "the firewall is idle": that case returns before it.
var errAuditNoBackend = errors.New("firewall audit: tables are applied and no backend is loaded")

// AuditTables compares kernel state (from the active backend) against the
// last-applied config snapshot. Reports:
//   - firewall-stale-table (AC-15): ze_* tables in kernel not in config
//   - firewall-drift (AC-16): ze_* table content differs from config, which
//     includes a desired table the kernel no longer has
//
// This is a read-only check; it never modifies nftables state.
//
// It returns the number of findings (0 = clean) and an error when the audit
// COULD NOT RUN. A caller MUST tell the two apart: zero findings beside a
// non-nil error is the absence of an answer, not a clean kernel, and reporting
// it as health is how a flushed ruleset stayed green.
func AuditTables() (int, error) {
	desired := LastApplied()
	if len(desired) == 0 {
		// Nothing has been applied, so no table is desired and none can have
		// drifted. This is a clean audit rather than one that did not run, and
		// it is the state every process is in before the first apply.
		return 0, nil
	}
	backend := GetBackend()
	if backend == nil {
		return 0, errAuditNoBackend
	}

	actual, err := backend.ListTables()
	if err != nil {
		return 0, fmt.Errorf("firewall audit: ListTables: %w", err)
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
			// A table ze applied and the kernel no longer has is drift, not an
			// apply failure: the apply that installed it returned long ago, so
			// an external `nft flush ruleset` or another tool removed it since.
			// Nothing else sees it -- the stale-table branch above looks only
			// for tables ze does NOT want. ze_vrrp carries the RFC 9568
			// Section 6.4.3 accept filter, so a vanished table is a non-owner
			// router accepting packets for the virtual address.
			driftNames = append(driftNames, dt.Name)
			continue
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

	return findings, nil
}
