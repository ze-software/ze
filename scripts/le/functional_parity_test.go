// The migration's proof for the functional and integration areas: the script
// and the commands agree about what they would run.
//
// internal/le/functional and internal/le/integration replace
// scripts/le/application/functional.py and integration.py. Both versions remain
// until the swap (plan/spec-le-is-a-ze-binary.md, step 14). This file makes that
// overlap safe. It is deliberately HERE because it is a migration artifact. The
// commit that deletes the scripts also deletes this file.
//
// VALIDATES: spec-le-is-a-ze-binary AC-11. Over the real checkout and fixture
// environments, both halves build the same argv for each suite and gate. They
// publish the same suite table and derive the same budgets and concurrency.
// PREVENTS: a silent behavior change in a port with unexamined OUTPUT. A suite
// can still pass with the wrong -p, cap, or binary set. In that case, the gate
// proves a different claim.
//
// A PYTHON TOOL'S SEAMS ARE ITS PROCESS BOUNDARY, so nothing here calls both
// halves in one process. What is compared is the ARGV each side would run and
// the table each side publishes, which is the pair of facts the two share.
//
// One difference is DELIBERATE and is asserted rather than compared. The script
// DROPS an unknown name from its run list, but the command refuses it.
// TestGatingRunStillDropsAnUnknownSuiteInTheScript pins the fixed direction. It
// fails when somebody repairs the script.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"

	"github.com/ze-software/ze/internal/le/deployment"
	"github.com/ze-software/ze/internal/le/evidence"
	"github.com/ze-software/ze/internal/le/functional"
	"github.com/ze-software/ze/internal/le/gotoolchain"
	"github.com/ze-software/ze/internal/le/integration"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/qemu"
)

// runTimeout bounds one Python run. Each of these imports two modules and
// prints a table, so a run past this is a hung interpreter.
const runTimeout = 120 * time.Second

// runFunctionalPython runs a fragment against the scripts/ package path and answers what it
// printed on stdout.
func runFunctionalPython(t *testing.T, root, fragment string, environ ...string) []byte {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()

	//nolint:gosec // the interpreter and the fragment are this test's own
	cmd := exec.CommandContext(ctx, "python3", "-c", fragment)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PYTHONPATH="+filepath.Join(root, "scripts"))
	cmd.Env = append(cmd.Env, environ...)

	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("python fragment failed: %v\n%s", err, complaintOf(err))
	}
	return out
}

// complaintOf answers what a failed command wrote, so a broken fragment reports
// the interpreter's own complaint rather than "exit status 1".
func complaintOf(err error) []byte {
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		return exit.Stderr
	}
	return nil
}

// areaCheckout answers the tree both halves are compared over.
func areaCheckout(t *testing.T) string {
	t.Helper()
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("find the checkout: %v", err)
	}
	return root
}

// TestFunctionalCatalogMatchesTheScript compares the table read by two external
// guards. scripts/status/verify_run.go and the scripts/docvalid drift check fail
// closed for an empty or malformed answer. Thus, a shape change disables the
// guards instead of making them fail.
func TestFunctionalCatalogMatchesTheScript(t *testing.T) {
	root := areaCheckout(t)

	for _, probe := range []struct {
		name string
		env  []string
	}{
		{"as the checkout is", nil},
		{"a small host", []string{"ZE_SUITE_CORES=4"}},
		{"a large host", []string{"ZE_SUITE_CORES=64"}},
		{"a container that cannot count its cores", []string{"ZE_SUITE_CORES="}},
		{"one suite moved off the shared cap", []string{"ZE_SUITE_TIMEOUT_ENCODE=900s"}},
		{"the shared cap raised", []string{"ZE_SUITE_TIMEOUT=1200s"}},
		{"one suite's own concurrency", []string{"ZE_PLUGIN_PARALLEL=3", "ZE_SUITE_CORES=64"}},
	} {
		t.Run(probe.name, func(t *testing.T) {
			script := runFunctionalPython(t, root,
				"import json,le.application.functional as f;print(json.dumps(f.catalogue()))", probe.env...)

			var want []map[string]any
			if err := json.Unmarshal(script, &want); err != nil {
				t.Fatalf("decode the script's table: %v", err)
			}

			got := commandCatalog(t, probe.env)
			if len(want) != len(got) {
				t.Fatalf("the script published %d suites and the command %d", len(want), len(got))
			}
			for i := range want {
				if !reflect.DeepEqual(want[i], got[i]) {
					t.Errorf("suite %v differs:\n  script:  %v\n  command: %v",
						want[i]["name"], want[i], got[i])
				}
			}
		})
	}
}

// commandCatalog answers the Go table under one environment, decoded the same
// way the script's is so the comparison is over values rather than over
// formatting.
func commandCatalog(t *testing.T, environ []string) []map[string]any {
	t.Helper()
	var rows []map[string]any
	withAreaEnv(t, environ, func() {
		raw, err := json.Marshal(functional.Catalog())
		if err != nil {
			t.Fatalf("encode the command's table: %v", err)
		}
		if err := json.Unmarshal(raw, &rows); err != nil {
			t.Fatalf("decode the command's table: %v", err)
		}
	})
	return rows
}

// TestFunctionalSuiteArgvMatchesTheScript compares the full command line each
// half runs, cap included. A budget the report reads and `timeout` does not is
// worse than no budget, and only this comparison can see the two disagree.
func TestFunctionalSuiteArgvMatchesTheScript(t *testing.T) {
	root := areaCheckout(t)
	const binaries = "/nonexistent/testbin/bin"

	for _, environ := range [][]string{
		nil,
		{"ZE_SUITE_CORES=16"},
		{"ZE_SUITE_TIMEOUT=45s", "ZE_SUITE_KILL_AFTER=2s"},
		{"ZE_SUITE_TIMEOUT_PLUGIN=99s"},
	} {
		script := runFunctionalPython(t, root, `
import json, pathlib
import le.application.functional as f
b = f.BinarySet(directory=pathlib.Path("`+binaries+`"), remove=False)
print(json.dumps({s.name: f.command_line(s, b) for s in f.SUITES}))
`, environ...)

		var want map[string][]string
		if err := json.Unmarshal(script, &want); err != nil {
			t.Fatalf("decode the script's command lines: %v", err)
		}

		withAreaEnv(t, environ, func() {
			set := functional.BinarySet{Dir: binaries}
			for _, suite := range functional.Suites {
				got := functional.CommandLine(suite, set)
				if !reflect.DeepEqual(want[suite.Name], got) {
					t.Errorf("env %v, suite %s:\n  script:  %v\n  command: %v",
						environ, suite.Name, want[suite.Name], got)
				}
			}
		})
	}
}

// TestFunctionalBuildCommandsMatchTheScript compares what each half compiles.
// Wrong tags can make an isolated set prove a smaller product than the shipped
// product. Every assertion in that set can still pass.
func TestFunctionalBuildCommandsMatchTheScript(t *testing.T) {
	root := areaCheckout(t)
	const binaries = "/nonexistent/testbin/bin"

	for _, chaos := range []bool{false, true} {
		flag := "False"
		if chaos {
			flag = "True"
		}
		script := runFunctionalPython(t, root, `
import json, pathlib
import le.application.functional as f
print(json.dumps(f._build_commands(pathlib.Path("`+binaries+`"), chaos=`+flag+`)))
`)

		var want [][]string
		if err := json.Unmarshal(script, &want); err != nil {
			t.Fatalf("decode the script's build commands: %v", err)
		}

		tc, err := gotoolchain.New(root)
		if err != nil {
			t.Fatalf("derive the toolchain: %v", err)
		}
		got := functional.BuildCommands(tc, binaries, chaos)
		if !reflect.DeepEqual(want, got) {
			t.Errorf("chaos=%v:\n  script:  %v\n  command: %v", chaos, want, got)
		}
	}
}

// TestIntegrationGateArgvMatchesTheScript compares every gate served by this
// area. Two of the 21 script gates are deliberately absent.
// internal/le/deployment and internal/le/evidence own and serve their gate-name
// families. Listing a row in two areas creates the drift that this parity gate
// prevents.
func TestIntegrationGateArgvMatchesTheScript(t *testing.T) {
	root := areaCheckout(t)

	for _, environ := range [][]string{
		nil,
		{"INTEROP_SCENARIO=bgp-ebgp-ipv4-frr"},
		{"IPSEC_INTEROP_SCENARIO=ipsec-psk-tunnel"},
		{"VERBOSE=1", "SESSION_TIMEOUT=90"},
	} {
		script := runFunctionalPython(t, root,
			"import json,le.application.integration as i;"+
				"print(json.dumps({g.name: list(g.argv) for g in i.GATES.gates}))", environ...)

		var want map[string][]string
		if err := json.Unmarshal(script, &want); err != nil {
			t.Fatalf("decode the script's gate table: %v", err)
		}

		withAreaEnv(t, environ, func() {
			for _, gate := range integration.Table() {
				got := gate.Argv()
				if !reflect.DeepEqual(want[gate.Name], got) {
					t.Errorf("env %v, gate %s:\n  script:  %v\n  command: %v",
						environ, gate.Name, want[gate.Name], got)
				}
				delete(want, gate.Name)
			}
		})
	}
}

// TestEveryIntegrationGateOfTheScriptIsServedSomewhere tests the other half of
// that split. The owning areas must declare the two gates absent from
// internal/le/integration. Otherwise, the port dropped them.
func TestEveryIntegrationGateOfTheScriptIsServedSomewhere(t *testing.T) {
	root := areaCheckout(t)
	script := runFunctionalPython(t, root,
		"import json,le.application.integration as i;print(json.dumps([g.name for g in i.GATES.gates]))")

	var declared []string
	if err := json.Unmarshal(script, &declared); err != nil {
		t.Fatalf("decode the script's gate names: %v", err)
	}

	served := map[string]bool{}
	for _, name := range integration.Gates() {
		served[name] = true
	}
	// The remaining gates belong to the areas that own their gate-name families.
	// The test READS them from those areas instead of keeping another list. A
	// local list would be a third record of gate ownership and invite the drift
	// that the census detects.
	//
	// internal/le/qemu joined these areas on 2026-08-26 when it received
	// ze-qemu-vpp-hugepages-test. This case read only three areas, although the
	// fourth already owned a gate declared by the script.
	for _, area := range []leaction.List{
		deployment.Actions(), evidence.Actions(), qemu.Actions(),
	} {
		for _, row := range area.Actions {
			served[row.Gate] = true
		}
	}

	for _, name := range declared {
		if !served[name] {
			t.Errorf("the script declares %s and no le command serves it", name)
		}
	}
}

// TestGatingRunStillDropsAnUnknownSuiteInTheScript is the fail-open this port
// closed, asserted on the SCRIPT so it reddens the day somebody repairs it.
//
// run_gating removes an unknown name from its run list. A typo therefore removes
// the suite from both the run and the progress denominator. The run reports that
// every suite passed even though one never started. This applies the `ipsec`
// failure from the functional.py docstring to the script itself.
func TestGatingRunStillDropsAnUnknownSuiteInTheScript(t *testing.T) {
	root := areaCheckout(t)

	out := runFunctionalPython(t, root, `
import json, pathlib
import le.application.functional as f

ran = []
f.prepare = lambda label, *, chaos: f.BinarySet(directory=pathlib.Path("/nonexistent/bin"), remove=False)
def fake(suite, binaries, *, cover=None):
    ran.append(suite.name)
    return 0, 0
f.execute = fake
f.GATING = ("encode", "no-such-suite")
code = f.run_gating()
print(json.dumps({"code": code, "ran": ran}))
`)

	// The script prints its whole budget report to stdout before the document,
	// so the answer is its LAST line.
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var answer struct {
		Code int      `json:"code"`
		Ran  []string `json:"ran"`
	}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &answer); err != nil {
		t.Fatalf("decode the script's run: %v\n%s", err, out)
	}

	if answer.Code != 0 || len(answer.Ran) != 1 {
		t.Fatalf("the script no longer fails open: it exited %d after running %v."+
			" If it was repaired, delete this test and the journal row it pins"+
			" (plan/journal/gate-excludes-part-of-its-population.md)",
			answer.Code, answer.Ran)
	}

	// The command refuses the same run list rather than shrinking it.
	if _, err := functional.GatingSuites([]string{"encode", "no-such-suite"}, functional.Suites); err == nil {
		t.Error("the command dropped the unknown name too, so the port fixed nothing")
	}
}

// withAreaEnv runs body with each NAME=VALUE applied, and restores what was there.
//
// t.Setenv cannot be used: internal/core/env caches os.Environ() on first read,
// so the reset below is what makes a second probe see the second environment.
func withAreaEnv(t *testing.T, environ []string, body func()) {
	t.Helper()
	for _, entry := range environ {
		name, value, ok := strings.Cut(entry, "=")
		if !ok {
			t.Fatalf("environment entry %q is not NAME=VALUE", entry)
		}
		t.Setenv(name, value)
	}
	env.ResetCache()
	defer env.ResetCache()
	body()
}
