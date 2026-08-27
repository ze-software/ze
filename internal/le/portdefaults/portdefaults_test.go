// Related: portdefaults.go -- the gate these tests drive from its entry point
//
// Every test here calls the tool as a function. The gate used to be reachable
// only as a subprocess, so its selftest reported one line of output and a
// failure named no case.

package portdefaults

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/leroot"
)

// VALIDATES: each selftest case behaves as it declares, named one by one.
// PREVENTS: the comparison breaking silently. This is what the script's
// --selftest carried as a single bool, and a failure now names which of the
// eight properties stopped holding.
func TestEachSelftestCaseHolds(t *testing.T) {
	for _, testCase := range selftestCases {
		t.Run(testCase.name, func(t *testing.T) {
			if failure := testCase.run(); failure != "" {
				t.Error(failure)
			}
		})
	}
}

// VALIDATES: the selftest answers one result per case, and passes here.
// PREVENTS: a selftest that reports OK having run nothing.
func TestSelftestAnswersOneResultPerCase(t *testing.T) {
	report := Selftest()
	if len(report.Results) != len(selftestCases) {
		t.Fatalf("the selftest answered %d results, want one per case (%d)", len(report.Results), len(selftestCases))
	}
	if failures := report.Failures(); len(failures) != 0 {
		t.Errorf("the selftest failed: %v", failures)
	}
	if code := report.Code(2); code != 0 {
		t.Errorf("a passing selftest answers %d, want 0", code)
	}
	for i, result := range report.Results {
		if result.Case != selftestCases[i].name {
			t.Errorf("result %d names %q, want %q", i, result.Case, selftestCases[i].name)
		}
	}
}

// VALIDATES: a failing selftest answers 2, which is the code the script
// answered, and not the 1 a drifted table answers.
// PREVENTS: a caller that reads the two codes apart losing the distinction. A
// broken gate and a broken table need different responses.
func TestABrokenSelftestAnswersTwoAndADriftedTableOne(t *testing.T) {
	broken := leroot.NewSelftestReport("ok", "failed", leroot.Fail("match", "the comparison stopped comparing"))
	if code := broken.Code(2); code != 2 {
		t.Errorf("a failing selftest answers %d, want 2", code)
	}

	result, err := Check(fixture(t, "100", "200"))
	if err != nil {
		t.Fatalf("check the fixture: %v", err)
	}
	if result.Valid {
		t.Fatalf("a table that drifted was reported valid: %+v", result)
	}
}

// fixture writes a tree carrying the whole central table, every service agreeing
// with its module, except that web's YANG module refines the port to webYANG.
//
// Every mapped service is written because Check reads the package's own
// serviceYANG map: a fixture naming one service would report the other seven as
// stale mappings, which is the gate working rather than the fixture.
func fixture(t *testing.T, webGo, webYANG string) string {
	t.Helper()
	dir := t.TempDir()

	var table strings.Builder
	table.WriteString("package config\n\nfunc RegisterBuiltinListenerDefaults() {\n") //nolint:errcheck // strings.Builder never fails
	port := 5000
	for _, service := range sortedServices() {
		goPort, yangPort := strconv.Itoa(port), strconv.Itoa(port)
		if service == "web" {
			goPort, yangPort = webGo, webYANG
		}
		table.WriteString("\tRegisterListenerDefault(\"" + service + "\", \"0.0.0.0\", \"" + goPort + "\")\n") //nolint:errcheck // strings.Builder never fails
		write(t, dir, serviceYANG[service], "module m { uses zt:listener { refine port { default "+yangPort+"; } } }\n")
		port++
	}
	table.WriteString("}\n") //nolint:errcheck // strings.Builder never fails
	write(t, dir, goTablePath, table.String())
	return dir
}

// sortedServices answers every mapped service name, in a fixed order so a
// fixture is the same tree on every run.
func sortedServices() []string {
	services := make([]string, 0, len(serviceYANG))
	for service := range serviceYANG {
		services = append(services, service)
	}
	sort.Strings(services)
	return services
}

func write(t *testing.T, dir, rel, body string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("create %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// VALIDATES: an agreeing pair is valid, a disagreeing pair is one mismatch
// naming both ports, and the count of services compared reaches the answer.
// PREVENTS: a gate that reports a clean table having compared nothing, which
// its Checked field is what a reader has to see.
func TestAgreementAndDriftOverATree(t *testing.T) {
	result, err := Check(fixture(t, "8080", "8080"))
	if err != nil {
		t.Fatalf("check the agreeing tree: %v", err)
	}
	if !result.Valid || len(result.Drifts) != 0 {
		t.Fatalf("an agreeing pair drew %+v", result)
	}
	if result.Checked != len(serviceYANG) {
		t.Errorf("the gate compared %d services, want every mapped one (%d)", result.Checked, len(serviceYANG))
	}

	result, err = Check(fixture(t, "8080", "9090"))
	if err != nil {
		t.Fatalf("check the drifted tree: %v", err)
	}
	if len(result.Drifts) != 1 || result.Drifts[0].Reason != ReasonMismatch {
		t.Fatalf("a drifted pair drew %+v", result.Drifts)
	}
	if result.Drifts[0].GoPort != 8080 || result.Drifts[0].YANGPort != 9090 {
		t.Errorf("the drift names go-port=%d yang-port=%d, want both values", result.Drifts[0].GoPort, result.Drifts[0].YANGPort)
	}
}

// VALIDATES: a module carrying NO refine port default and a module carrying TWO
// are both ambiguous, and only exactly one is read.
// PREVENTS: the gate picking the first of several blocks. A module with two
// listeners has no single port to pin, and guessing one pins the wrong half
// while reporting agreement. The selftest covers the zero case; this covers the
// two case, which no fixture in this repository holds.
func TestOnlyExactlyOneRefineBlockIsRead(t *testing.T) {
	cases := map[string]struct {
		module string
		port   int
		ok     bool
	}{
		"one block":  {"refine port { default 8080; }", 8080, true},
		"no block":   {"container x { leaf name { type string; } }", 0, false},
		"two blocks": {"refine port { default 8080; }\nrefine port { default 9090; }", 0, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			port, ok := yangPortDefault(tc.module)
			if ok != tc.ok || port != tc.port {
				t.Errorf("the module answers (%d, %v), want (%d, %v)", port, ok, tc.port, tc.ok)
			}
		})
	}
}

// VALIDATES: a tree whose central table cannot be read is an ERROR, not a clean
// answer.
// PREVENTS: the gate reporting an agreeing table over a tree it never opened.
func TestATreeWithNoCentralTableIsAnError(t *testing.T) {
	if _, err := Check(t.TempDir()); err == nil {
		t.Error("a tree holding no listener table passed, so the gate compared nothing and said so to nobody")
	}
}

// VALIDATES: the result is structured data under the keys the script published,
// including the kebab-case ones.
// PREVENTS: a key renamed by the port, which would change what every caller of
// the JSON reads.
func TestResultIsStructuredData(t *testing.T) {
	raw, err := json.Marshal(Result{Drifts: []Drift{{Service: "web", GoPort: 1, YANGPort: 2, File: "f", Reason: ReasonMismatch}}, Checked: 3})
	if err != nil {
		t.Fatalf("marshal the result: %v", err)
	}
	for _, key := range []string{`"drifts"`, `"services-checked"`, `"valid"`, `"go-port"`, `"yang-port"`, `"service"`, `"reason"`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("the payload has no %s key: %s", key, raw)
		}
	}
}

// VALIDATES: the page carries the heading, the count and either the drift list
// or the verdict.
// PREVENTS: a page that says OK over a result holding drift.
func TestTextCarriesTheCountAndTheVerdict(t *testing.T) {
	clean := Result{Checked: 8, Valid: true}.Text()
	if !strings.Contains(clean, "Services checked: 8") || !strings.Contains(clean, "port-defaults: OK\n") {
		t.Errorf("the clean page is:\n%s", clean)
	}

	drifted := Result{Drifts: []Drift{{Service: "web", GoPort: 1, YANGPort: 2, File: "f.yang", Reason: ReasonMismatch}}, Checked: 8}.Text()
	if strings.Contains(drifted, "port-defaults: OK") {
		t.Errorf("a page holding drift still says OK:\n%s", drifted)
	}
	if !strings.Contains(drifted, "  [port-mismatch] service=web go-port=1 yang-port=2 f.yang\n") {
		t.Errorf("the drifted page does not carry its drift:\n%s", drifted)
	}
	if !strings.Contains(drifted, "## Drift (1)") || !strings.Contains(drifted, "port-defaults: FAILED") {
		t.Errorf("the drifted page does not carry the verdict:\n%s", drifted)
	}
}

// VALIDATES: the two actions are reachable by their verbs, an unknown verb
// answers 2, and a value after a verb answers 1.
// PREVENTS: an area whose gates cannot be selected.
func TestTheAreaDispatchesItsTwoGates(t *testing.T) {
	rows := Actions()
	if len(rows.Actions) != 2 || rows.Actions[0].Verb != "check" || rows.Actions[1].Verb != "selftest" {
		t.Fatalf("the area lists %v, want check and selftest", rows.Actions)
	}
	if _, code := Answer([]string{"selftest"}); code != 0 {
		t.Errorf("the selftest action answers %d over its own cases, want 0", code)
	}
	if _, code := Answer([]string{"nonesuch"}); code != 2 {
		t.Errorf("an unknown action answers %d, want 2", code)
	}
	if _, code := Answer([]string{"check", "web"}); code != 1 {
		t.Errorf("a value after an action answers %d, want 1", code)
	}
}

// VALIDATES: this checkout passes the gate, from the entry point a developer
// runs.
// PREVENTS: the daemon binding a port the schema documents differently. This is
// where TestPortDefaultsGate and TestPortDefaultsSelftest
// (scripts/checks/port_defaults_test.go) now live: the first forked the script
// and asserted the page reads OK, the second forked --selftest for the same one
// line. TestEachSelftestCaseHolds carries the second one, per case.
func TestThisCheckoutPassesTheGate(t *testing.T) {
	payload, code := Answer([]string{"check"})
	result, ok := payload.(Result)
	if !ok {
		t.Fatalf("the check answered %T, want a Result", payload)
	}
	if code != 0 || !result.Valid {
		t.Fatalf("the gate answers %d over this checkout:\n%s", code, result.Text())
	}
	if result.Checked < len(serviceYANG) {
		t.Errorf("the gate compared %d services against %d mappings: a service left the table without leaving the map", result.Checked, len(serviceYANG))
	}
}
