// The census is the migration's own report card. This file stops the census
// from flattering itself.
//
// letools/parity counted a gate as ported when a REGISTERED command claimed
// the gate. It never asked whether that command did the work. Ten gates were
// answered by a Python script: five deployment proofs, two docker-exec scans,
// two interop labs, and the stress runner. Every gate was inside the ported
// number. A gate is now in one of three states. The third state is claimed but
// NOT converted.
//
// The list below is what is STILL forked, so it shrinks as drivers are ported
// and it is the migration's own remaining-work number. Two of the five
// deployment proofs left it on 2026-08-26, when their drivers became Go
// (letools/deployment, l2tpppp.go and gokrazyl2tp.go).
//
// These cases live here rather than beside letools/parity because they need the
// registry this binary composes. The parity package links no area, so its own
// tests see a process where nobody claims anything.

package main

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/letools/deployment"
	"github.com/ze-software/ze/letools/parity"
)

// forkedToday is every gate this checkout reaches by starting a script. Each
// one is a port somebody still owes, so the list is the migration's remaining
// driver work written down.
//
// This list is asserted rather than derived. Deriving it from the table that
// the census reads would make the values agree by construction. Such agreement
// tests nothing (ai/rules/interop-and-goal-validation.md).
var forkedToday = []string{
	"ze-deployment-docker-l2tp-ppp-test",
	"ze-deployment-docker-pppoe-accel-test",
	"ze-deployment-vpp-test",
	// The changed-file router answers eight of its twelve delegated targets
	// by calling the Go package that holds them (letools/docwiring,
	// delegate.go). What is left is scripts/dev/check_doc_links.py, which
	// has no Go implementation, so the gate is claimed and not converted.
	"ze-doc-wiring-check",
	"ze-functional-docker-exec-check",
	"ze-functional-docker-exec-selftest",
	"ze-interop-ipsec-test",
	"ze-interop-test",
	"ze-stress-bird-test",
}

// census answers what `le parity` answers in this process.
func census(t *testing.T) parity.Census {
	t.Helper()
	payload, _ := parity.Answer(nil)
	counted, ok := payload.(parity.Census)
	if !ok {
		t.Fatalf("`le parity` answered %T rather than a census", payload)
	}
	return counted
}

// TestCensusCountsAForkedDriverApartFromConvertedWork tests the real
// composition root. A gate whose command runs a script must be counted and
// NAMED as claimed-but-not-converted. The converted count must exclude that
// gate.
func TestCensusCountsAForkedDriverApartFromConvertedWork(t *testing.T) {
	counted := census(t)

	for _, gate := range forkedToday {
		if !slices.Contains(counted.ForkedGates, gate) {
			t.Errorf("%q starts a script and the census does not name it forked: %v", gate, counted.ForkedGates)
		}
		if slices.Contains(counted.UnportedGates, gate) {
			t.Errorf("%q is claimed by a registered command and the census names it unported", gate)
		}
	}
	if counted.Forked != len(counted.ForkedGates) {
		t.Errorf("census says %d forked and names %d", counted.Forked, len(counted.ForkedGates))
	}
	if counted.Forked < len(forkedToday) {
		t.Errorf("census counts %d forked drivers, and %d are known to exist", counted.Forked, len(forkedToday))
	}
	if counted.Converted+counted.Forked+counted.Unported != counted.Gates {
		t.Errorf("converted %d + forked %d + unported %d does not account for %d gates",
			counted.Converted, counted.Forked, counted.Unported, counted.Gates)
	}
	if counted.Complete() {
		t.Errorf("%d drivers are still scripts and the census calls itself complete", counted.Forked)
	}
}

// TestCensusCountsNoScriptDriverAsConverted tests the forbidden state. The
// converted count MUST NOT include a gate whose action table starts a script.
//
// The two sides come from different places. The census supplies the forked
// set. The Python le's declaration says that a gate exists. If the census
// stopped reading the action tables, the forked list would be empty. This test
// would then find ten unaccounted gates.
func TestCensusCountsNoScriptDriverAsConverted(t *testing.T) {
	counted := census(t)

	claimed := 0
	for _, gate := range forkedToday {
		if !slices.Contains(counted.UnportedGates, gate) {
			claimed++
		}
	}
	if claimed != len(forkedToday) {
		t.Fatalf("%d of the %d known script drivers are claimed; the rest are unported and this case cannot see them",
			claimed, len(forkedToday))
	}
	converted := claimed - len(counted.ForkedGates)
	if converted > 0 {
		t.Errorf("%d claimed gates whose driver is a script are inside the converted count", converted)
	}
}

// TestAreaListingPublishesTheScriptItStarts tests the operator's view. A reader
// of `le deployment` learns which proofs use Go and which drivers the migration
// has not reached. The reader does not need to open the table.
func TestAreaListingPublishesTheScriptItStarts(t *testing.T) {
	listing := deployment.Actions()

	forks := map[string][]string{}
	for _, row := range listing.Actions {
		forks[row.Gate] = row.Forks
	}
	if got := forks["ze-deployment-vpp-test"]; !slices.Contains(got, "scripts/evidence/effective-vpp.py") {
		t.Errorf("the vpp proof publishes %v, and it starts scripts/evidence/effective-vpp.py", got)
	}
	if got := forks["ze-deployment-l2tp-test"]; len(got) != 0 {
		t.Errorf("the L2TP proof does its work in Go and publishes a fork: %v", got)
	}
}

// processStarters are the four spellings a package reaches another program by.
//
// gaterun is the spelling that the areas share. Everything else uses exec.
// letools/docwiring answered eight gates by starting `make` for two months, but
// it claimed that the gates were converted. A guard that watched only gaterun
// did not detect those starts. A guard that misses the commonest spelling does
// not guard the census.
var processStarters = [...]string{
	"gaterun.Run(",
	"gaterun.Stream(",
	"exec.Command(",
	"exec.CommandContext(",
}

// sessionScratchHelper is the ONE repository script that a gate CAN start. The
// gate does not assign its work to another program.
//
// scripts/dev/session-scratch.sh answers a PATH. Its header states why the rule
// stays in shell, with make and Go implementations for their callers. Starting
// this script says nothing about where a gate's work occurs. Thus, the start is
// not evidence of an unported driver.
const sessionScratchHelper = "session-scratch.sh"

// namesAScript reports whether a package's source names a program that this
// repository carries as SOURCE. The name is a script path or a make invocation
// that reaches a script through a recipe.
//
// A bare ".py" or ".sh" is a suffix test rather than a program (letools/rules
// scans for both), so a literal that is only the extension is not evidence.
func namesAScript(body string) bool {
	if strings.Contains(body, `"make"`) {
		return true
	}
	for _, literal := range scriptLiterals(body) {
		if literal != ".py" && literal != ".sh" && !strings.Contains(literal, sessionScratchHelper) {
			return true
		}
	}
	return false
}

// scriptLiterals answers every double-quoted literal in body that ends in a
// script extension.
func scriptLiterals(body string) []string {
	var found []string
	for _, chunk := range strings.Split(body, `"`)[1:] {
		if strings.HasSuffix(chunk, ".py") || strings.HasSuffix(chunk, ".sh") {
			found = append(found, chunk)
		}
	}
	return found
}

// TestEveryAreaThatStartsAProcessDeclaresWhatItStarts guards the census from
// erosion. Go does not stop a new area from running a Python driver or a Make
// recipe. The area can then claim that its gate is ported. The third state
// prevents that reading.
//
// The check reads SOURCE rather than the registry because a closure cannot
// report what it will exec. A package that CLAIMS a gate must publish the
// program it starts when both conditions that follow apply. The package reaches
// one of the four process starters, and it names a program that this repository
// carries as source. It publishes an action table's Forks argv or its own Forks
// function (letools/docwiring). letools/leaction then decides from the argv
// whether that program is a script.
//
// All three conditions are necessary. Each condition removes a population that
// this guard does not judge. A package that claims no gate cannot flatter the
// census. A package that starts nothing has nothing to declare. A package whose
// starts are `git`, `go`, or `docker` does its own work with a compiled tool.
// That is what a converted gate looks like.
func TestEveryAreaThatStartsAProcessDeclaresWhatItStarts(t *testing.T) {
	starts, declares := map[string]bool{}, map[string]bool{}
	claims := map[string]bool{}
	walkGo(t, func(rel, body string) {
		if strings.HasSuffix(rel, "_test.go") {
			return
		}
		pkg := filepath.Dir(rel)
		if pkg == filepath.Join("letools", "gaterun") {
			return
		}
		if strings.Contains(body, "parity.Claim(") || strings.Contains(body, "parity.ClaimForked(") {
			claims[pkg] = true
		}
		for _, starter := range processStarters {
			if strings.Contains(body, starter) && namesAScript(body) {
				starts[pkg] = true
			}
		}
		if strings.Contains(body, "Forks:") || strings.Contains(body, "func Forks()") {
			declares[pkg] = true
		}
	})

	watched := 0
	for pkg := range starts {
		if !claims[pkg] {
			continue
		}
		watched++
		if !declares[pkg] {
			t.Errorf("%s claims a gate and starts a program this repository carries as source,"+
				" and it publishes no Forks argv, so letools/parity cannot tell a converted gate"+
				" from a forked one", pkg)
		}
	}
	if watched == 0 {
		t.Fatal("no gate-claiming package starts a script, so this guard is watching nothing")
	}
}
