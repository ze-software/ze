// Design: docs/architecture/firewall/table-ownership-and-shutdown-flush.md -- contributed tables tests
// Related: contribution.go -- replaceContributedTables, withdrawContributedTables under test

package firewall

import (
	"errors"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// The owner, tables and backend these tests use, names no producer uses.
const (
	replaceTestOwner    = "test-contribution-replace"
	replaceTestTable    = "ze_test_contribution_replace"
	replaceTestRefused  = "ze_test_contribution_refused"
	replaceTestOperator = "ze_test_contribution_operator"
	replaceTestBackend  = "contribution-test"
)

// contributionLab is one fake backend with the tables it was last handed.
type contributionLab struct {
	backend *countingBackend
	applied []Table
}

// openContributionLab resets the registry and the backends, loads a fake
// backend whose verifier refuses replaceTestRefused, reserves the two test
// tables under replaceTestOwner, and undoes all of it in cleanup.
func openContributionLab(t *testing.T) *contributionLab {
	t.Helper()
	resetBackends()
	resetTables()
	lab := &contributionLab{}
	lab.backend = &countingBackend{onApply: func(d []Table) { lab.applied = d }}
	t.Cleanup(func() {
		resetBackends()
		resetTables()
		tableReservations.mu.Lock()
		delete(tableReservations.owners, replaceTestTable)
		delete(tableReservations.owners, replaceTestRefused)
		tableReservations.mu.Unlock()
		contributions.mu.Lock()
		delete(contributions.callers, replaceTestOwner)
		contributions.mu.Unlock()
		for _, issue := range report.Warnings() {
			if issue.Source == contributionReportSource {
				report.ClearWarning(issue.Source, issue.Code, issue.Subject)
			}
		}
	})
	control := TableControl{Plugin: "contribution test", Decides: "the test", Command: "ze show firewall ruleset"}
	MustReserveTableNames(replaceTestOwner, control, replaceTestTable, replaceTestRefused)
	if err := RegisterBackend(replaceTestBackend, func() (Backend, error) { return lab.backend, nil }); err != nil {
		t.Fatalf("register the test backend: %v", err)
	}
	refuse := func(desired []Table) error {
		for i := range desired {
			if desired[i].Name == replaceTestRefused {
				return errors.New("the test verifier refuses " + replaceTestRefused)
			}
		}
		return nil
	}
	if err := RegisterVerifier(replaceTestBackend, refuse); err != nil {
		t.Fatalf("register the test verifier: %v", err)
	}
	if err := LoadBackend(replaceTestBackend); err != nil {
		t.Fatalf("load the test backend: %v", err)
	}
	return lab
}

// appliedNames answers the names of the tables the backend was last handed.
func (l *contributionLab) appliedNames() []string {
	names := make([]string, 0, len(l.applied))
	for i := range l.applied {
		names = append(names, l.applied[i].Name)
	}
	slices.Sort(names)
	return names
}

func contributionProcess() *process.Process {
	return process.NewProcess(plugin.PluginConfig{Name: replaceTestOwner, Run: "test"})
}

func inetTable(name string) Table { return Table{Name: name, Family: FamilyInet} }

// TestContributionReplacesOwnersCompleteSet proves a contribution takes the
// configured tables' path and keeps to its own set: the contributed table
// reaches the backend beside an operator table, a later config apply keeps it,
// a refused set leaves the registry and the backend with the previous set, and
// an empty set withdraws it while the operator table stays. Method: a fake
// backend records what each reconcile hands it, and its verifier refuses one
// reserved name.
func TestContributionReplacesOwnersCompleteSet(t *testing.T) {
	lab := openContributionLab(t)
	proc := contributionProcess()

	if err := registerConfigTables(&firewallConfig{Tables: []Table{inetTable(replaceTestOperator)}}); err != nil {
		t.Fatalf("apply the operator's table: %v", err)
	}
	output, err := replaceContributedTables(proc, []Table{inetTable(replaceTestTable)})
	if err != nil {
		t.Fatalf("a reserved, valid table was refused: %v", err)
	}
	if !slices.Equal(output.Tables, []string{replaceTestTable}) {
		t.Fatalf("answer names %v, want [%s]", output.Tables, replaceTestTable)
	}
	if want := []string{replaceTestOperator, replaceTestTable}; !slices.Equal(lab.appliedNames(), want) {
		t.Fatalf("backend holds %v after the contribution, want %v", lab.appliedNames(), want)
	}

	// A config apply registers only the configured owners, so the contribution stays.
	if err := registerConfigTables(&firewallConfig{Tables: []Table{inetTable(replaceTestOperator)}}); err != nil {
		t.Fatalf("re-apply the operator's table: %v", err)
	}
	if !slices.Contains(lab.appliedNames(), replaceTestTable) {
		t.Fatalf("a config apply dropped the contributed table: backend holds %v", lab.appliedNames())
	}

	if _, err := replaceContributedTables(proc, []Table{inetTable(replaceTestTable), inetTable(replaceTestRefused)}); err == nil {
		t.Fatal("a set the backend verifier refuses was accepted")
	}
	held := registeredTables(replaceTestOwner)
	if len(held) != 1 || held[0].Name != replaceTestTable {
		t.Fatalf("after a refused set the registry holds %v, want the previous [%s]", held, replaceTestTable)
	}
	if want := []string{replaceTestOperator, replaceTestTable}; !slices.Equal(lab.appliedNames(), want) {
		t.Fatalf("after a refused set the backend holds %v, want the previous %v", lab.appliedNames(), want)
	}

	output, err = replaceContributedTables(proc, nil)
	if err != nil {
		t.Fatalf("the empty set was refused: %v", err)
	}
	if len(output.Tables) != 0 {
		t.Fatalf("the withdrawal answers tables %v, want none", output.Tables)
	}
	if registeredTables(replaceTestOwner) != nil {
		t.Fatal("the empty set left the owner registered")
	}
	if want := []string{replaceTestOperator}; !slices.Equal(lab.appliedNames(), want) {
		t.Fatalf("after the withdrawal the backend holds %v, want %v", lab.appliedNames(), want)
	}
}

// TestGenerationCleanupWithdrawsOwner proves the generation cleanup withdraws
// the tables of the process that contributed them and of no other: an older
// process of the same name that ends after its replacement contributed
// withdraws nothing, the current one's end withdraws its tables from the
// registry and the backend, and a withdrawal the backend refuses raises the
// doctor code until the next successful reconcile clears it. Method: two
// processes under one name and a fake backend whose Apply can be made to fail.
func TestGenerationCleanupWithdrawsOwner(t *testing.T) {
	lab := openContributionLab(t)
	older := contributionProcess()
	current := contributionProcess()

	if _, err := replaceContributedTables(older, []Table{inetTable(replaceTestTable)}); err != nil {
		t.Fatalf("the older process's contribution was refused: %v", err)
	}
	if _, err := replaceContributedTables(current, []Table{inetTable(replaceTestTable)}); err != nil {
		t.Fatalf("the current process's contribution was refused: %v", err)
	}
	withdrawContributedTables(older)
	if registeredTables(replaceTestOwner) == nil {
		t.Fatal("the end of an older process withdrew its replacement's tables")
	}

	lab.backend.applyErr = errors.New("the test backend refuses this reconcile")
	withdrawContributedTables(current)
	if registeredTables(replaceTestOwner) != nil {
		t.Fatal("the end of the contributing process left its tables registered")
	}
	if !contributionWarningRaised() {
		t.Fatalf("a refused withdrawal raised no %s warning", diagnostic.CodeDoctorFirewallContributedTables)
	}

	lab.backend.applyErr = nil
	if err := ApplyAll(); err != nil {
		t.Fatalf("reconcile after the backend recovered: %v", err)
	}
	if slices.Contains(lab.appliedNames(), replaceTestTable) {
		t.Fatalf("the ended process's table is still applied: %v", lab.appliedNames())
	}
	if contributionWarningRaised() {
		t.Fatal("a successful reconcile left the contributed-tables warning in place")
	}
}

func contributionWarningRaised() bool {
	for _, issue := range report.Warnings() {
		if issue.Source != contributionReportSource {
			continue
		}
		if issue.Code == diagnostic.CodeDoctorFirewallContributedTables {
			return true
		}
	}
	return false
}
