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

// noForks stands in for an area whose every action does the work in Go.
var noForks = map[string][]string{}

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
	if census.Converted+census.Forked+census.Unported != census.Gates {
		t.Errorf("converted %d + forked %d + unported %d does not account for %d gates",
			census.Converted, census.Forked, census.Unported, census.Gates)
	}
	if len(census.UnportedGates) != census.Unported {
		t.Errorf("census says %d unported but names %d", census.Unported, len(census.UnportedGates))
	}
	served := map[string]bool{}
	claimed, forkedClaims := claimSnapshot()
	for _, table := range []map[string][]string{claimed, forkedClaims} {
		for _, gates := range table {
			for _, name := range gates {
				served[name] = true
			}
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

	census := Take(gates, left, claimed, noForks, registeredAll, []string{"repository"})
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

	before := Take(gates, noScripts, map[string][]string{}, noForks, registeredNone, []string{"parity"})
	if before.Unported != 2 || before.Converted != 0 {
		t.Fatalf("with no claim: converted %d unported %d, want 0 and 2", before.Converted, before.Unported)
	}

	after := Take(gates, noScripts, map[string][]string{"repository": {"ze-tier-check"}}, noForks, registeredAll, []string{"parity", "repository"})
	if after.Converted != 1 || after.Unported != 1 {
		t.Fatalf("with one claim: converted %d unported %d, want 1 and 1", after.Converted, after.Unported)
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

	census := Take(gates, noScripts, map[string][]string{"repository": {"ze-tier-check-renamed"}}, noForks, registeredAll, []string{"repository"})
	if len(census.UnknownClaims) != 1 {
		t.Fatalf("claims naming an undeclared gate: %v, want exactly one", census.UnknownClaims)
	}
	if census.Converted != 0 {
		t.Errorf("an undeclared gate counted as converted: %d", census.Converted)
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

	census := Take(gates, noScripts, map[string][]string{"repository": {"ze-tier-check"}}, noForks, registeredNone, []string{})
	if len(census.UnwiredClaims) != 1 {
		t.Fatalf("claims whose command is unregistered: %v, want exactly one", census.UnwiredClaims)
	}
	if census.Converted != 0 {
		t.Errorf("an unreachable command's claim counted as converted: %d", census.Converted)
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

	census := Take(gates, noScripts, claimed, noForks, registeredAll, []string{"repository"})
	if !census.Complete() {
		t.Fatalf("every gate served and the census is still red: %+v", census)
	}
	if census.Unported != 0 || census.Converted != 2 {
		t.Errorf("converted %d unported %d, want 2 and 0", census.Converted, census.Unported)
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
	tables, forkedTables := claimSnapshot()
	for _, table := range []map[string][]string{tables, forkedTables} {
		for _, names := range table {
			for _, name := range names {
				claimed[name] = true
			}
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

// TestParityCountsAForkedDriverApartFromConvertedWork verifies the census distinction.
//
// The first Take implementation counted a gate as ported when any registered command claimed it.
// It did not inspect whether that command performed the work.
// Five deployment proofs, two docker-exec scans, and three lab runners still started Python scripts.
// The ported count nevertheless included all ten.
// This case requires separate counts for Go work and a claimed gate whose driver remains a script.
func TestParityCountsAForkedDriverApartFromConvertedWork(t *testing.T) {
	gates := []Gate{
		{Area: "repository", Name: "ze-tier-check", Why: "where code may live"},
		{Area: "deployment", Name: "ze-deployment-vpp-test", Why: "a real VPP daemon"},
	}
	claimed := map[string][]string{
		"repository": {"ze-tier-check"},
		"deployment": {"ze-deployment-vpp-test"},
	}
	forkedClaims := map[string][]string{"deployment": {"ze-deployment-vpp-test"}}

	census := Take(gates, noScripts, claimed, forkedClaims, registeredAll, []string{"repository", "deployment"})

	if census.Converted != 1 {
		t.Errorf("converted %d, want 1: only ze-tier-check does its work in Go", census.Converted)
	}
	if census.Forked != 1 {
		t.Errorf("forked %d, want 1: ze-deployment-vpp-test starts a script", census.Forked)
	}
	if census.Unported != 0 {
		t.Errorf("unported %d, want 0: both gates are claimed", census.Unported)
	}
	if !slices.Contains(census.ForkedGates, "ze-deployment-vpp-test") {
		t.Errorf("the forked gate is counted and not named: %v", census.ForkedGates)
	}
	if slices.Contains(census.UnportedGates, "ze-deployment-vpp-test") {
		t.Error("a claimed gate whose driver is a script was reported as unported, which loses the claim")
	}
	if census.Complete() {
		t.Error("one driver is still a script and the census calls itself complete")
	}
	if len(census.UnknownClaims) != 0 || len(census.UnwiredClaims) != 0 {
		t.Errorf("a gate claimed twice was reported as a fault: unknown %v unwired %v",
			census.UnknownClaims, census.UnwiredClaims)
	}
}

// TestParityTurnsAForkedGateIntoAConvertedOneWhenItsDriverMoves is the same
// fact read forwards: porting a driver is what moves a gate between the two
// claimed states, and nothing else is.
func TestParityTurnsAForkedGateIntoAConvertedOneWhenItsDriverMoves(t *testing.T) {
	gates := []Gate{{Area: "deployment", Name: "ze-deployment-vpp-test", Why: "a real VPP daemon"}}
	claimed := map[string][]string{"deployment": {"ze-deployment-vpp-test"}}

	before := Take(gates, noScripts, claimed, map[string][]string{"deployment": {"ze-deployment-vpp-test"}},
		registeredAll, []string{"deployment"})
	after := Take(gates, noScripts, claimed, noForks, registeredAll, []string{"deployment"})

	if before.Converted != 0 || before.Forked != 1 {
		t.Errorf("with the driver forked: converted %d forked %d, want 0 and 1", before.Converted, before.Forked)
	}
	if after.Converted != 1 || after.Forked != 0 {
		t.Errorf("with the driver in Go: converted %d forked %d, want 1 and 0", after.Converted, after.Forked)
	}
	if before.Complete() {
		t.Error("a forked driver and the census calls itself complete")
	}
	if !after.Complete() {
		t.Errorf("every gate converted, scripts/ empty, and the census is still red: %+v", after)
	}
}

// TestParityReportsOneMistakeOnceWhenBothClaimsNameIt verifies that merged claims are unique.
// An area passes the same gate to Claim and ClaimForked.
// If Python no longer declares that gate, unknown-claims must name the mistake once.
func TestParityReportsOneMistakeOnceWhenBothClaimsNameIt(t *testing.T) {
	gates := []Gate{{Area: "deployment", Name: "ze-deployment-vpp-test"}}
	claimed := map[string][]string{"deployment": {"ze-deployment-renamed"}}
	forkedClaims := map[string][]string{"deployment": {"ze-deployment-renamed"}}

	census := Take(gates, noScripts, claimed, forkedClaims, registeredAll, []string{"deployment"})
	if len(census.UnknownClaims) != 1 {
		t.Errorf("one renamed gate claimed by both tables was reported as %v, want one row", census.UnknownClaims)
	}
}

// TestParityCountsAForkedClaimNobodyRegistered preserves the unwired error in the third state.
// A driver that the migration has not reached remains a claim.
// Marking it forked must not hide an unwired tool.
func TestParityCountsAForkedClaimNobodyRegistered(t *testing.T) {
	gates := []Gate{{Area: "deployment", Name: "ze-deployment-vpp-test"}}
	forkedClaims := map[string][]string{"deployment": {"ze-deployment-vpp-test"}}

	census := Take(gates, noScripts, map[string][]string{}, forkedClaims, registeredNone, []string{})
	if len(census.UnwiredClaims) != 1 {
		t.Errorf("a forked claim from an unregistered command: %v, want one unwired row", census.UnwiredClaims)
	}
	if census.Forked != 0 {
		t.Errorf("forked %d, want 0: nothing can reach the command that claimed it", census.Forked)
	}
	if !slices.Contains(census.UnportedGates, "ze-deployment-vpp-test") {
		t.Error("an unreachable command's forked claim stopped being named as unported")
	}
}
