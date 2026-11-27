// Design: docs/architecture/diagnostics/production-diagnostics.md -- firewall read-only drift checker
// Related: audit.go -- periodic warning-raising firewall audit
// Related: engine.go -- plugin engine registration of OnDriftCheck

package firewall

import (
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/drift"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	driftSubsystemFirewall = "firewall"
	driftStatePresent      = "present"
	driftStateAbsent       = "absent"
	driftNounChains        = "chains"
	driftNounTerms         = "terms"
	zeTablePrefix          = "ze_"
)

func handleDriftCheck(_ string) (rpc.DriftCheckOutput, error) {
	return checkFirewallDrift(), nil
}

// checkFirewallDrift compares the last-applied firewall snapshot against live
// backend tables without mutating nftables state, backend lifecycle, or report
// warnings. Safe for concurrent use.
func checkFirewallDrift() rpc.DriftCheckOutput {
	desired := LastApplied()
	if len(desired) == 0 {
		return rpc.DriftCheckOutput{
			Subsystem: driftSubsystemFirewall,
			Status:    string(drift.StatusClean),
			Checked:   0,
			Findings:  []rpc.DriftFinding{},
		}
	}

	b := GetBackend()
	if b == nil {
		return rpc.DriftCheckOutput{
			Subsystem: driftSubsystemFirewall,
			Status:    string(drift.StatusUnavailable),
			Checked:   0,
			Findings:  []rpc.DriftFinding{},
		}
	}

	actual, err := b.ListTables()
	if err != nil {
		return rpc.DriftCheckOutput{
			Subsystem: driftSubsystemFirewall,
			Status:    string(drift.StatusError),
			Checked:   0,
			Findings:  []rpc.DriftFinding{},
			Error:     err.Error(),
		}
	}

	desiredByName, desiredNames := indexFirewallTables(desired)
	actualByName, actualNames := indexFirewallTables(actual)

	findings := make([]rpc.DriftFinding, 0)
	drifted := 0
	record := func(f rpc.DriftFinding) {
		drifted++
		if len(findings) < drift.MaxFindingsPerSubsystem {
			findings = append(findings, f)
		}
	}

	for _, name := range desiredNames {
		dt := desiredByName[name]
		bare := StripZeTablePrefix(dt.Name)
		at, exists := actualByName[dt.Name]
		if !exists {
			record(rpc.DriftFinding{
				Subject:  bare,
				Kind:     string(drift.KindMissing),
				Expected: driftStatePresent,
				Actual:   driftStateAbsent,
			})
			continue
		}
		if len(at.Chains) != len(dt.Chains) {
			record(rpc.DriftFinding{
				Subject:  bare,
				Kind:     string(drift.KindMismatch),
				Expected: formatDriftCount(len(dt.Chains), driftNounChains),
				Actual:   formatDriftCount(len(at.Chains), driftNounChains),
			})
		}
		compareChainTerms(bare, dt.Chains, at.Chains, record)
	}

	for _, name := range actualNames {
		if !strings.HasPrefix(name, zeTablePrefix) {
			continue
		}
		if _, wanted := desiredByName[name]; wanted {
			continue
		}
		record(rpc.DriftFinding{
			Subject:  StripZeTablePrefix(name),
			Kind:     string(drift.KindExtra),
			Expected: driftStateAbsent,
			Actual:   driftStatePresent,
		})
	}

	status := string(drift.StatusClean)
	if drifted > 0 {
		status = string(drift.StatusDrifted)
	}
	return rpc.DriftCheckOutput{
		Subsystem: driftSubsystemFirewall,
		Status:    status,
		Checked:   len(desiredNames),
		Drifted:   drifted,
		Truncated: drifted > drift.MaxFindingsPerSubsystem,
		Findings:  findings,
	}
}

func indexFirewallTables(tables []Table) (map[string]Table, []string) {
	byName := make(map[string]Table, len(tables))
	names := make([]string, 0, len(tables))
	for i := range tables {
		name := tables[i].Name
		if _, seen := byName[name]; !seen {
			names = append(names, name)
		}
		byName[name] = tables[i]
	}
	slices.Sort(names)
	return byName, names
}

func compareChainTerms(tableName string, desiredChains, actualChains []Chain, record func(rpc.DriftFinding)) {
	actualByName := make(map[string]Chain, len(actualChains))
	for i := range actualChains {
		actualByName[actualChains[i].Name] = actualChains[i]
	}

	sortedDesired := slices.Clone(desiredChains)
	slices.SortFunc(sortedDesired, func(a, b Chain) int {
		return strings.Compare(a.Name, b.Name)
	})

	for i := range sortedDesired {
		dc := sortedDesired[i]
		ac, exists := actualByName[dc.Name]
		if !exists {
			continue
		}
		if len(ac.Terms) != len(dc.Terms) {
			record(rpc.DriftFinding{
				Subject:  tableName + "/" + dc.Name,
				Kind:     string(drift.KindMismatch),
				Expected: formatDriftCount(len(dc.Terms), driftNounTerms),
				Actual:   formatDriftCount(len(ac.Terms), driftNounTerms),
			})
		}
	}
}

func formatDriftCount(n int, noun string) string {
	var tb textbuf.Buffer
	return tb.Int(int64(n)).Byte(' ').Str(noun).String()
}
