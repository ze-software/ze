// The migration's proof for `le fuzz` and for the toolchain behind it: the
// script and the command run the same Go commands.
//
// internal/le/fuzz and internal/le/gotoolchain replace scripts/le/application/fuzz.py
// and scripts/le/devtools/toolchain.py. Both versions remain until the swap. This
// file is deliberately HERE because it is a migration artifact. The commit that
// deletes the scripts also deletes this proof.
//
// VALIDATES: spec-le-is-a-ze-binary AC-11. Over the real checkout, both halves
// discover the same fuzz targets. They build byte-identical `go test` argv for
// every target and for a named run.
// PREVENTS: a fuzz sweep that runs a different product from the script. Each
// argv uses the tag set. One missing tag compiles out modules, so the sweep
// tests a smaller daemon but reports the same target count.
//
// A Python tool's seams are the process boundary, so nothing here calls both
// halves. What is compared is the argv each would exec, which is the whole of
// what either tool does to the machine.

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/fuzz"
	"github.com/ze-software/ze/internal/le/gotoolchain"
	"github.com/ze-software/ze/internal/le/lepath"
)

// repoRoot answers the checkout both halves are pointed at.
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := lepath.Root()
	if err != nil {
		t.Skipf("no checkout: %v", err)
	}
	return root
}

// python runs one program against the Python le and answers its stdout. A
// non-zero exit is a failed test rather than a value, because every program
// here is a query with no failure mode of its own.
func python(t *testing.T, root, program string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "python3", "-c", program)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PYTHONPATH="+filepath.Join(root, "scripts"))

	out, err := cmd.Output()
	if err != nil {
		var stderr string
		var exit *exec.ExitError
		if ok := asExitError(err, &exit); ok {
			stderr = string(exit.Stderr)
		}
		t.Fatalf("python3 -c: %v\n%s", err, stderr)
	}
	return string(out)
}

// asExitError is errors.As without the import, kept local because one call site
// wants only the captured stderr.
func asExitError(err error, into **exec.ExitError) bool {
	exit, ok := err.(*exec.ExitError) //nolint:errorlint // one call site, one type
	if ok {
		*into = exit
	}
	return ok
}

// chainOf answers the Go toolchain for the checkout, which is what every argv
// below is built from.
func chainOf(t *testing.T, root string) gotoolchain.Toolchain {
	t.Helper()
	chain, err := gotoolchain.New(root)
	if err != nil {
		t.Fatalf("gotoolchain.New: %v", err)
	}
	return chain
}

// --- Discovery --------------------------------------------------------------

// TestDiscoveryAgreesWithTheScript compares the real-tree results instead of
// only their counts. A walk that loses one package can retain the same count
// when another package gains a target in that commit.
func TestDiscoveryAgreesWithTheScript(t *testing.T) {
	root := repoRoot(t)

	raw := python(t, root, `
import json
from le.application.fuzz import discover
print(json.dumps([[t.name, t.package] for t in discover()]))
`)
	var scripted [][]string
	if err := json.Unmarshal([]byte(raw), &scripted); err != nil {
		t.Fatalf("decode the script's targets: %v", err)
	}

	targets, err := fuzz.Discover(root)
	if err != nil {
		t.Fatalf("fuzz.Discover: %v", err)
	}

	ported := make([][]string, 0, len(targets))
	for _, target := range targets {
		ported = append(ported, []string{target.Name, target.Package})
	}

	if len(scripted) == 0 {
		t.Fatal("the script discovered nothing: the comparison below would be vacuous")
	}
	if !slices.EqualFunc(scripted, ported, slices.Equal) {
		t.Errorf("the two halves discovered different targets\nscript: %v\ncommand: %v", scripted, ported)
	}
}

// --- The argv ---------------------------------------------------------------

// TestEveryTargetsArgvIsByteIdentical is the comparison that matters: what each
// half would hand to the Go toolchain, for every target in the tree.
func TestEveryTargetsArgvIsByteIdentical(t *testing.T) {
	root := repoRoot(t)

	raw := python(t, root, `
import json
from le.application.fuzz import discover
print(json.dumps([t.command() for t in discover()]))
`)
	var scripted [][]string
	if err := json.Unmarshal([]byte(raw), &scripted); err != nil {
		t.Fatalf("decode the script's argv: %v", err)
	}

	targets, err := fuzz.Discover(root)
	if err != nil {
		t.Fatalf("fuzz.Discover: %v", err)
	}
	chain := chainOf(t, root)

	if len(scripted) != len(targets) {
		t.Fatalf("the script planned %d runs and the command planned %d", len(scripted), len(targets))
	}
	if len(scripted) == 0 {
		t.Fatal("neither half planned a run: the comparison is vacuous")
	}

	for i, target := range targets {
		got := target.Command(chain, fuzz.DefaultFuzzTime, fuzz.DefaultTimeout)
		if !slices.Equal(scripted[i], got) {
			t.Fatalf("run %d differs\nscript:  %s\ncommand: %s",
				i, strings.Join(scripted[i], " "), strings.Join(got, " "))
		}
	}
}

// TestANamedRunsArgvIsByteIdentical covers the developer-entered form:
// `make ze-fuzz-test-one FUZZ=... PKG=... TIME=...`. It passes a Go regular
// expression and wildcard package directly to the tool.
func TestANamedRunsArgvIsByteIdentical(t *testing.T) {
	root := repoRoot(t)
	const name = "FuzzParse.*"
	const pkg = "./internal/component/bgp/wireu/..."
	const budget = "30s"

	raw := python(t, root, `
import json
from le.application.fuzz import TIMEOUT
from le.devtools.toolchain import toolchain
argv = toolchain().go_test(
    '-fuzz=`+name+`',
    '-fuzztime=`+budget+`',
    f'-timeout={TIMEOUT}',
    '`+pkg+`',
)
print(json.dumps(argv))
`)
	var scripted []string
	if err := json.Unmarshal([]byte(raw), &scripted); err != nil {
		t.Fatalf("decode the script's argv: %v", err)
	}

	sweeper := &fuzz.Sweeper{Chain: chainOf(t, root), Root: root, Name: name, Package: pkg, FuzzTime: budget}
	plan, err := sweeper.Plan()
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(plan.Runs) != 1 {
		t.Fatalf("a named run planned %d runs", len(plan.Runs))
	}
	if !slices.Equal(scripted, plan.Runs[0].Argv) {
		t.Errorf("the named run differs\nscript:  %s\ncommand: %s",
			strings.Join(scripted, " "), strings.Join(plan.Runs[0].Argv, " "))
	}
}

// --- The toolchain environment ----------------------------------------------

// TestTheToolchainOverridesAgree compares the variables each half SETS. The
// inherited environment is the machine's and says nothing about the port.
func TestTheToolchainOverridesAgree(t *testing.T) {
	root := repoRoot(t)

	raw := python(t, root, `
import json, os
from le.devtools.toolchain import toolchain
before = dict(os.environ)
after = toolchain().environment(procs=True)
print(json.dumps({k: v for k, v in after.items() if before.get(k) != v}))
`)
	var scripted map[string]string
	if err := json.Unmarshal([]byte(raw), &scripted); err != nil {
		t.Fatalf("decode the script's environment: %v", err)
	}

	ported := map[string]string{}
	for _, entry := range chainOf(t, root).Overrides(gotoolchain.EnvOptions{Procs: true}) {
		key, value, _ := strings.Cut(entry, "=")
		ported[key] = value
	}

	if len(scripted) == 0 {
		t.Fatal("the script overrode nothing: the comparison is vacuous")
	}
	for key, want := range scripted {
		if got, ok := ported[key]; !ok || got != want {
			t.Errorf("%s: script sets %q, command sets %q (present=%v)", key, want, got, ok)
		}
	}
}

// --- The fail-open the port closed and the script keeps -----------------------

// fixtureCheckout writes a tree holding whatever manifest is asked for, beside a
// go.mod, and answers its root.
func fixtureCheckout(t *testing.T, manifest string, withManifest bool) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if withManifest {
		if err := os.WriteFile(filepath.Join(root, "feature-gates.txt"), []byte(manifest), 0o600); err != nil {
			t.Fatalf("write manifest: %v", err)
		}
	}
	return root
}

// scriptTestTags asks the SCRIPT what tag set it would build with, for a tree
// the test controls.
func scriptTestTags(t *testing.T, root, target string) string {
	t.Helper()
	return strings.TrimSpace(python(t, root, `
from pathlib import Path
from le.devtools.toolchain import toolchain
print(toolchain(Path('`+target+`')).test_tags)
`))
}

// TestTheScriptStillFailsOpenOnAnAbsentManifest is the requested parity case.
// The PORT fixes the defect, but the script retains it. Thus, the case pins the
// script answer and FAILS when somebody repairs the script. At that point, the
// two halves agree, and the case must be deleted instead of weakened.
//
// The script answers `ze_core` alone, which is byte-identical to its own
// core_tags: the reduced set its docstring calls "a defect everywhere else".
func TestTheScriptStillFailsOpenOnAnAbsentManifest(t *testing.T) {
	root := repoRoot(t)
	fixture := fixtureCheckout(t, "", false)

	if got := scriptTestTags(t, root, fixture); got != "ze_core" {
		t.Errorf("the script now answers %q for a checkout with no feature-gates.txt; "+
			"if it was repaired, delete this case and its twin below", got)
	}

	if _, err := gotoolchain.New(fixture); err == nil {
		t.Error("the port accepted a checkout with no feature-gates.txt")
	}
}

// TestTheScriptStillFailsOpenOnAManifestWithNoGate is the second route, and it
// is the one no filesystem error reports: the file is there, it parses, and it
// declares nothing.
func TestTheScriptStillFailsOpenOnAManifestWithNoGate(t *testing.T) {
	root := repoRoot(t)
	fixture := fixtureCheckout(t, "# a comment\nnotagate something\n", true)

	if got := scriptTestTags(t, root, fixture); got != "ze_core" {
		t.Errorf("the script now answers %q for a manifest declaring no ze_ tag; "+
			"if it was repaired, delete this case and its twin above", got)
	}

	if _, err := gotoolchain.New(fixture); err == nil {
		t.Error("the port accepted a manifest declaring no ze_ tag")
	}
}

// TestTheTwoHalvesAgreeOnARealManifest is the non-empty counterpart to the two
// disagreement cases above. It proves that the halves agree when the manifest
// is real. Without this case, a port that refuses every checkout would pass both
// other cases.
func TestTheTwoHalvesAgreeOnARealManifest(t *testing.T) {
	root := repoRoot(t)
	fixture := fixtureCheckout(t, "ze_bgp\tBGP\nze_l2tp\tL2TP\n", true)

	chain, err := gotoolchain.New(fixture)
	if err != nil {
		t.Fatalf("the port refused a real manifest: %v", err)
	}
	if got, want := scriptTestTags(t, root, fixture), chain.TestTags(); got != want {
		t.Errorf("script tags %q, command tags %q", got, want)
	}
}
