// The census is what lets this migration run two implementations at once, so
// these tests are about one thing: can it go RED. A parity gate that cannot
// fail proves nothing about how far along the port is, and it is the specific
// trap of a duplicate-then-swap route.
//
// Take is a pure function, so each red is driven by handing it the inputs that
// produce it rather than by breaking the checkout.

package parity

import (
	"context"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/letools/lepath"
	"github.com/ze-software/ze/letools/leroot"
)

// realGates reads the Python le's gate list, which is the denominator for as
// long as that side owns the gates.
func realGates(t *testing.T) []Gate {
	t.Helper()
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("find checkout: %v", err)
	}
	gates, err := ReadGates(context.Background(), root)
	if err != nil {
		t.Fatalf("read gates: %v", err)
	}
	return gates
}

// noScripts stands in for the end state, where scripts/ holds no code. The
// gate half of the census is what most of these cases are about, so they hold
// the file half at its finished value rather than at today's.
var noScripts = ScriptCount{ByLanguage: map[string]int{}, ByDir: map[string]int{}}

// realScripts counts what scripts/ holds right now.
func realScripts(t *testing.T) ScriptCount {
	t.Helper()
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("find checkout: %v", err)
	}
	scripts, err := CountScripts(root)
	if err != nil {
		t.Fatalf("count scripts: %v", err)
	}
	return scripts
}

// registeredNone stands in for a process where no le tool registered a root
// handler.
func registeredNone(string) bool { return false }

// registeredAll stands in for a process where every claiming tool is wired.
func registeredAll(string) bool { return true }

// TestParityNamesEveryUnportedGate holds the census to its contract over the
// real checkout: every gate the Python le declares is counted, every gate no Go
// command serves is NAMED, and the census refuses to call itself complete
// while any remain.
func TestParityNamesEveryUnportedGate(t *testing.T) {
	gates := realGates(t)
	scripts := realScripts(t)
	census := takeHere(gates, scripts)

	if census.Gates != len(gates) {
		t.Errorf("census counted %d gates, `le gates --json` declared %d", census.Gates, len(gates))
	}
	if census.Ported+census.Unported != census.Gates {
		t.Errorf("ported %d + unported %d does not account for %d gates", census.Ported, census.Unported, census.Gates)
	}
	if len(census.UnportedGates) != census.Unported {
		t.Errorf("census says %d unported but names %d", census.Unported, len(census.UnportedGates))
	}
	served := map[string]bool{}
	for _, gates := range claimSnapshot() {
		for _, name := range gates {
			served[name] = true
		}
	}
	for _, gate := range gates {
		if !served[gate.Name] && !slices.Contains(census.UnportedGates, gate.Name) {
			t.Errorf("gate %q is claimed by nobody and is not named as unported", gate.Name)
		}
	}
	if census.Unported > 0 && census.Complete() {
		t.Errorf("census reports %d unported gates and still calls itself complete", census.Unported)
	}
	if census.ScriptFiles > 0 && census.Complete() {
		t.Errorf("census reports %d code files under scripts/ and still calls itself complete", census.ScriptFiles)
	}
}

// TestParityCountsEveryCodeFileLeftUnderScripts is the second population. The
// owner's end state is that scripts/ holds no code, and that number is derived
// from the filesystem, so nothing has to be kept in step with it by hand.
func TestParityCountsEveryCodeFileLeftUnderScripts(t *testing.T) {
	scripts := realScripts(t)

	if scripts.Total == 0 {
		t.Fatal("scripts/ was counted as empty while the Python le still lives there")
	}
	byLanguage := 0
	for _, n := range scripts.ByLanguage {
		byLanguage += n
	}
	if byLanguage != scripts.Total {
		t.Errorf("per-language counts sum to %d, total says %d", byLanguage, scripts.Total)
	}
	byDir := 0
	for _, n := range scripts.ByDir {
		byDir += n
	}
	if byDir != scripts.Total {
		t.Errorf("per-directory counts sum to %d, total says %d", byDir, scripts.Total)
	}
	for _, ext := range []string{".py", ".go", ".sh"} {
		if scripts.ByLanguage[ext] == 0 {
			t.Errorf("no %s file was counted under scripts/, which holds several", ext)
		}
	}
	if scripts.ByDir["scripts/le"] == 0 {
		t.Error("scripts/le holds the Python le and was counted as empty")
	}
}

// TestParityIsRedWhileScriptsHoldsCode pins the second population's verdict:
// every gate can be served and the census still refuses, because the files are
// still there.
func TestParityIsRedWhileScriptsHoldsCode(t *testing.T) {
	gates := []Gate{{Area: "repository", Name: "ze-tier-check"}}
	claimed := map[string][]string{"repository": {"ze-tier-check"}}
	left := ScriptCount{Total: 1, ByLanguage: map[string]int{".py": 1}, ByDir: map[string]int{"scripts/le": 1}}

	census := Take(gates, left, claimed, registeredAll, []string{"repository"})
	if census.Unported != 0 {
		t.Fatalf("the gate half is not green: %d unported", census.Unported)
	}
	if census.Complete() {
		t.Error("every gate served, one code file left under scripts/, and the census calls itself complete")
	}
}

// TestParityCountsAClaimedGateAsPorted is the census's only green path: a gate
// leaves the unported list when a REGISTERED command claims it, and the count
// falls by exactly one.
func TestParityCountsAClaimedGateAsPorted(t *testing.T) {
	gates := []Gate{
		{Area: "repository", Name: "ze-tier-check", Why: "where code may live"},
		{Area: "repository", Name: "ze-repository-check", Why: "structure"},
	}

	before := Take(gates, noScripts, map[string][]string{}, registeredNone, []string{"parity"})
	if before.Unported != 2 || before.Ported != 0 {
		t.Fatalf("with no claim: ported %d unported %d, want 0 and 2", before.Ported, before.Unported)
	}

	after := Take(gates, noScripts, map[string][]string{"repository": {"ze-tier-check"}}, registeredAll, []string{"parity", "repository"})
	if after.Ported != 1 || after.Unported != 1 {
		t.Fatalf("with one claim: ported %d unported %d, want 1 and 1", after.Ported, after.Unported)
	}
	if slices.Contains(after.UnportedGates, "ze-tier-check") {
		t.Error("a claimed, wired gate is still named as unported")
	}
	if !slices.Contains(after.UnportedGates, "ze-repository-check") {
		t.Error("an unclaimed gate stopped being named")
	}
	if after.Complete() {
		t.Error("one gate of two ported and the census calls itself complete")
	}
}

// TestParityRefusesAClaimOnAnUndeclaredGate is the drift red. A gate renamed
// or deleted on the Python side while the Go side still claims it is exactly
// what this migration must not do in silence.
func TestParityRefusesAClaimOnAnUndeclaredGate(t *testing.T) {
	gates := []Gate{{Area: "repository", Name: "ze-tier-check"}}

	census := Take(gates, noScripts, map[string][]string{"repository": {"ze-tier-check-renamed"}}, registeredAll, []string{"repository"})
	if len(census.UnknownClaims) != 1 {
		t.Fatalf("claims naming an undeclared gate: %v, want exactly one", census.UnknownClaims)
	}
	if census.Ported != 0 {
		t.Errorf("an undeclared gate counted as ported: %d", census.Ported)
	}
	if census.Complete() {
		t.Error("a claim on a gate nobody declares and the census calls itself complete")
	}
}

// TestParityRefusesAClaimWhoseCommandRegisteredNothing is the not-wired red.
// A tool that was written and never reached from the composition root must not
// lower the count, which is the spec's "a step that ports a tool and leaves
// the count unchanged has not wired it" read from the other side.
func TestParityRefusesAClaimWhoseCommandRegisteredNothing(t *testing.T) {
	gates := []Gate{{Area: "repository", Name: "ze-tier-check"}}

	census := Take(gates, noScripts, map[string][]string{"repository": {"ze-tier-check"}}, registeredNone, []string{})
	if len(census.UnwiredClaims) != 1 {
		t.Fatalf("claims whose command is unregistered: %v, want exactly one", census.UnwiredClaims)
	}
	if census.Ported != 0 {
		t.Errorf("an unreachable command's claim counted as ported: %d", census.Ported)
	}
	if !slices.Contains(census.UnportedGates, "ze-tier-check") {
		t.Error("an unreachable command's gate stopped being named as unported")
	}
	if census.Complete() {
		t.Error("a claim nothing can reach and the census calls itself complete")
	}
}

// TestParityIsCompleteOnlyWhenEveryGateIsServed pins the swap's precondition:
// the census turns green on zero unported, and on nothing else.
func TestParityIsCompleteOnlyWhenEveryGateIsServed(t *testing.T) {
	gates := []Gate{
		{Area: "repository", Name: "ze-tier-check"},
		{Area: "repository", Name: "ze-repository-check"},
	}
	claimed := map[string][]string{"repository": {"ze-tier-check", "ze-repository-check"}}

	census := Take(gates, noScripts, claimed, registeredAll, []string{"repository"})
	if !census.Complete() {
		t.Fatalf("every gate served and the census is still red: %+v", census)
	}
	if census.Unported != 0 || census.Ported != 2 {
		t.Errorf("ported %d unported %d, want 2 and 0", census.Ported, census.Unported)
	}
}

// TestParityRegistersItsCommand is the wiring: importing this package is what
// puts `le parity` in the registry, and nothing else does.
func TestParityRegistersItsCommand(t *testing.T) {
	if !registry.HasRootHandler("parity") {
		t.Fatal("importing letools/parity registered no `parity` root handler")
	}
}

// TestParityCountsOnlyLeCommands is what le linking the product costs the
// census. A tool that introspects ze must load ze's registry to read it, so
// this process's registry carries ze's root commands beside le's. Counting one
// of those as a Go le command would report a migration further along than it
// is, and a claim on it would pass because ZE registered the name.
//
// The probe registers straight on the shared registry, which is exactly what a
// product package's init() does.
func TestParityCountsOnlyLeCommands(t *testing.T) {
	const name = "parity-foreign-root-probe"
	registry.MustRegisterRootHandler(name, func(*registry.RuntimeContext, []string) int { return 0 },
		registry.Meta{Description: "a test probe", Mode: "offline", Section: registry.SectionTest})

	if slices.Contains(rootNames(), name) {
		t.Errorf("the census counts %q, which le did not register", name)
	}
	if !slices.Contains(rootNames(), "parity") {
		t.Error("the census does not count `parity`, which le did register")
	}

	// The two predicates differ on a product root, which is what makes the
	// choice between them observable at all.
	if !registry.HasRootHandler(name) {
		t.Fatal("the probe did not register, so the two predicates cannot be told apart here")
	}
	if leroot.Owns(name) {
		t.Fatal("le claims a name it never registered")
	}

	// Now drive the census THIS PROCESS takes, which is the wiring `le parity`
	// runs. A gate claimed by a command only ze registered must land in
	// UnwiredClaims and must not lower the unported count.
	gates := realGates(t)
	unclaimed := firstUnclaimedGate(t, gates)
	Claim(name, unclaimed)

	census := takeHere(gates, noScripts)
	if !slices.Contains(census.UnwiredClaims, claimLine(name, unclaimed)) {
		t.Errorf("a claim from a command le never registered was not reported unwired: %v", census.UnwiredClaims)
	}
	if !slices.Contains(census.UnportedGates, unclaimed) {
		t.Errorf("%q was counted as ported on a claim from a command le never registered", unclaimed)
	}
}

// firstUnclaimedGate answers a declared gate that no tool claims, so a test can
// claim it without changing what any other case sees.
func firstUnclaimedGate(t *testing.T, gates []Gate) string {
	t.Helper()
	claimed := map[string]bool{}
	for _, names := range claimSnapshot() {
		for _, name := range names {
			claimed[name] = true
		}
	}
	for _, gate := range gates {
		if !claimed[gate.Name] {
			return gate.Name
		}
	}
	t.Fatal("every declared gate is claimed, so this test has no gate left to claim")
	return ""
}

// TestParityRefusesArguments holds the CLI contract: the rendering is chosen
// with a pipe operator, so a --json flag is a caller error rather than a
// second way to ask.
func TestParityRefusesArguments(t *testing.T) {
	payload, code := Answer([]string{"--json"})
	if code == 0 {
		t.Error("`le parity --json` was accepted; the rendering is `| json`")
	}
	if payload != nil {
		t.Errorf("a refused invocation answered a payload: %v", payload)
	}
}
