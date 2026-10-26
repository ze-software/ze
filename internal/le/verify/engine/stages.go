// Design: docs/architecture/testing/verify-freshness-scope.md -- fixed pre-commit stage population
// Package verifyengine orchestrates the native actions that make up full
// verification. The ordered population is declared here rather than discovered
// at run time.
package verifyengine

import (
	"errors"
	"slices"
	"strconv"
	"strings"
)

// staticcheckParts is the number of pieces the Staticcheck feature matrix is
// cut into, one stage per piece.
//
// One Staticcheck run type-checks the whole module once per matrix row, and the
// matrix has held 38 rows since 2026-07-24, so the undivided stage measured
// 23m36s in CI (run 33450825487) against its own 25-minute bound and hit that
// bound on nearly every other run. Six pieces put 6 or 7 rows in a stage.
//
// Six is the shard count `.github/workflows/verify.yml` runs. That workflow
// deals this list round robin, so six CONSECUTIVE stages always land on six
// different shards. Coverage does not depend on the two numbers agreeing: every
// piece runs exactly once whatever the shard count is, and only the balance
// does.
const staticcheckParts = 6

// raceParts is the number of pieces the race pass over the changed packages
// is cut into, one stage per piece.
//
// With no green baseline, which is every CI clone, the changed population is
// the whole tree, and the undivided `verify deps unit-race-changed` reached the
// 120-minute job bound of `.github/workflows/verify.yml` on shard 2 of run
// 2026-10-17 before it finished. The packages are dealt by a hash of their
// import path (verifydeps.dealRacePackages), so three pieces each race about a
// third of the tree. The three stages are consecutive and follow unit-cached,
// so a six-shard round robin puts each on a shard of its own, none beside the
// cached pass. The tail after them is ordered exabgp-test, gating, alloc so the
// short alloc stage, not a functional suite, is the one that wraps onto
// unit-cached's shard. Coverage does not depend on any of this, only balance.
const raceParts = 3

// Identity names one native root/action invocation.
type Identity struct {
	Name    string   `json:"name"`
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
}

// Stage is one ordered pre-commit action.
type Stage struct {
	Identity Identity `json:"identity"`
	// Whole declares that the stage's action judges its whole subject whatever
	// change scope the run published (publishChangeScope), so a partial
	// population's verdict on it is a full judgement and replaces a full run's
	// red (JudgesInPart, carryForward).
	//
	// The zero value is the safe answer, and it is the default on purpose: a
	// stage a partial population shares with the full population and that does
	// not declare Whole is judged in part there, so its partial green never
	// replaces a full red it did not re-judge. A stage that learns to read the
	// scope therefore needs no edit here to stay correct; only the claim of a
	// full judgement does, and TestAWholeStageReachesNoChangeScopeReader
	// (internal/le) refuses that claim for a stage whose action's own package
	// reads the scope, unless the readers there were reviewed as ones the
	// stage never reaches. That check is package-granular and does not follow
	// imports: a reader in a package the action calls into is not seen, so a
	// Whole claim over such an action rests on the reviewer of the claim.
	Whole bool `json:"whole,omitempty"`
}

// JudgesInPart reports whether mode's population judges the named stage only
// over its published scope while the full population judges all of it: the
// stage is in both populations and does not declare Whole. Only such a
// stage's full verdict survives a partial run. A cut run's mode
// (`changed-part-1-of-2`, Part.Name) is judged as the population it cut
// (BaseMode), and a mode that names no population is refused, never answered:
// a guessed "full" would let a partial green replace a full red.
func JudgesInPart(mode, name string) (bool, error) {
	base, err := BaseMode(mode)
	if err != nil {
		return false, err
	}
	if base == Mode {
		return false, nil
	}
	return judgesInPart(name, StagesForMode(base), fullStages())
}

// judgesInPart is JudgesInPart over the two populations it compares, so a test
// can add a stage nobody declared. A stage the partial population does not
// hold is not judged by it at all, which is an error rather than an answer.
func judgesInPart(name string, partial, full []Stage) (bool, error) {
	index := slices.IndexFunc(partial, func(one Stage) bool { return one.Identity.Name == name })
	if index < 0 {
		return false, errors.New("the population holds no stage " + name)
	}
	if !slices.ContainsFunc(full, func(one Stage) bool { return one.Identity.Name == name }) {
		return false, nil
	}
	return !partial[index].Whole, nil
}

// BaseMode answers the population a mode names: the mode itself for a
// population StagesForMode knows, or the population a cut run's mode
// (`<base>-part-<i>-of-<n>`, Part.Name) was dealt from. Any other mode is an
// error, because no population answers for it.
func BaseMode(mode string) (string, error) {
	if len(StagesForMode(mode)) > 0 {
		return mode, nil
	}
	refused := errors.New("verify mode " + strconv.Quote(mode) +
		" names no stage population: it is none of full, fast or changed, nor a cut of one (<mode>-part-<i>-of-<n>)")
	base, numbers, isCut := strings.CutLast(mode, "-part-")
	if !isCut {
		return "", refused
	}
	index, count, found := strings.Cut(numbers, "-of-")
	if !found {
		return "", refused
	}
	part, err := parsePart(index, count)
	if err != nil {
		return "", refused
	}
	if len(StagesForMode(base)) == 0 {
		return "", refused
	}
	if part.Name(base) != mode {
		return "", refused
	}
	return base, nil
}

// parsePart reads a cut's index and count as a valid Part.
func parsePart(index, count string) (Part, error) {
	i, err := strconv.Atoi(index)
	if err != nil {
		return Part{}, err
	}
	n, err := strconv.Atoi(count)
	if err != nil {
		return Part{}, err
	}
	part := Part{Index: i, Of: n}
	return part, part.Valid()
}

// staticcheckStages returns one stage per piece of the Staticcheck feature
// matrix. The rows each piece judges are dealt by the matrix itself, so a
// feature gate added to `feature-gates.txt` needs nothing here.
func staticcheckStages() []Stage {
	stages := make([]Stage, 0, staticcheckParts)
	for part := 1; part <= staticcheckParts; part++ {
		stages = append(stages, stage("go staticcheck", "check",
			"part", strconv.Itoa(part), "of", strconv.Itoa(staticcheckParts)))
	}
	return stages
}

// raceStages returns one stage per piece of the race pass. Each piece lists
// the changed population itself and runs its share, so a package added to the
// tree needs nothing here. A piece selects its groups from the anchor through
// the refs, not from the published scope, so it judges the same packages in
// every population and is declared Whole.
func raceStages() []Stage {
	stages := make([]Stage, 0, raceParts)
	for part := 1; part <= raceParts; part++ {
		stages = append(stages, stage("verify deps", "unit-race-changed",
			"part", strconv.Itoa(part), "of", strconv.Itoa(raceParts)))
	}
	return whole(stages...)
}

// fullStages returns the native verification actions in execution order.
//
// Every stage whose action judges its whole subject whatever scope the run
// published is declared so through whole (Stage.Whole). The Staticcheck parts,
// `arch enumeration check` and the gating suite read the published scope and
// declare nothing, which is what makes a partial run's verdict on them a
// partial one. A stage added here without whole is judged in part by any
// partial population that shares it, which is the safe answer.
func fullStages() []Stage {
	stages := whole(
		stage("go lint", "run"),
		stage("arch tier", "check"),
		stage("rfc", "check"),
		stage("arch iface-resolution"),
		stage("plugin boundary", "check"),
		stage("config coercion", "check"),
		stage("arch fs-persistence", "check"),
		stage("cli stdio", "check"),
		stage("config ports", "check"),
		stage("go version-pin", "check"),
		// Every patch of both families against what it was reviewed for: file
		// reads and one go env, so the fast mode keeps it.
		stage("go patch-stamps", "check"),
		stage("config claims"),
		stage("test sensitivity", "check"),
	)
	return slices.Concat(stages, staticcheckStages(), whole(
		stage("repo compiles", "check"),
		stage("go vet-platforms", "darwin", "freebsd", "openbsd"),
		stage("doc wiring"),
		stage("doc check", "all"),
		stage("doc check", "links"),
		// Every caller of a name the subject-first rename retired is rewritten,
		// so a line naming one again is a regression (AC-16 of
		// spec-le-subject-first-command-tree).
		stage("doc check", "retired-commands"),
		stage("repo", "check", "tree"),
		stage("plugin imports", "check"),
		// The gated-plugin table is generated from the same manifest and the
		// registrations, so a gate added without regenerating it reddens here.
		stage("plugin gated-table", "check"),
		// A plugin that consumes what only another gate's plugin produces, with
		// no declared edge, runs silently empty in a build without that gate.
		stage("plugin coupling", "check"),
		stage("plugin declarations", "check"),
	), []Stage{
		// arch enumeration judges the CHANGE SET rather than the tree: the copies
		// already in the tree are `le arch enumeration report`, and this stage's red
		// means the change under test added one (owner decision, 2026-09-14).
		stage("arch enumeration", "check"),
	}, whole(
		// arch compound-guard judges the LINES in the unpushed range, committed or
		// not: the base is the merge base of HEAD and the upstream, or origin/main,
		// read from refs, so this detached worktree answers the same range as its
		// source. A clean pushed checkout judges nothing, correctly, because pushed
		// code was judged before its push. No base exits 2. The tree held about
		// 3,000 splittable || guards in shipped code when it was written, and a
		// package scope would make a neighbor's old guards due on every touch.
		stage("arch compound-guard", "check"),
		// arch refuse-by-default judges the same unpushed LINES against the same
		// base as compound-guard: a changed line that answers a plausible value
		// where the code has no answer (an unmatched switch value or a miss
		// answered as a hit, an ADD-PATH identifier compared with zero, a state
		// answered from a literal). The tree's existing instances are owned by
		// spec-refuse-instead-of-defaulting and listed by `le arch
		// refuse-by-default report`, so a tree scope would red every commit.
		stage("arch refuse-by-default", "check"),
		stage("yang glue", "check"),
		stage("repo feature-tags", "check"),
		stage("doc check", "templ-output"),
		stage("web vendor", "check"),
		stage("web assets", "check"),
		stage("doc index", "check"),
		stage("ai rules", "check", "render"),
		stage("ai rules", "check", "index"),
		stage("ai rules", "check", "condensed"),
		stage("ai rules", "check", "metadata"),
		stage("repo arch-map", "check"),
		stage("test health", "check"),
		stage("site facts", "check"),
		stage("web htmx", "check"),
		stage("verify deps", "evidence-vet"),
		stage("ai hooks", "unit"),
		stage("verify deps", "vulnerability"),
		stage("verify deps", "unit-cached"),
	), raceStages(), whole(
		stage(FunctionalCommand, "exabgp-test"),
	), []Stage{
		stage(FunctionalCommand, "gating"),
	}, whole(
		stage("verify deps", "alloc"),
	))
}

// fastStages returns the per-edit population: every full stage except the
// three whose subject is the whole tree rather than the edit. Evidence vetting
// and the allocation benchmarks read populations no edit narrows, and the
// cached unit pass covers every package, so what an edit still owes is the
// race pass over its own groups, which the `verify deps/unit-race-changed`
// pieces run in both modes.
//
// Every other stage keeps the identity it holds in full mode, and that is not
// a naming detail. A stage narrows itself or it does not: the Staticcheck
// matrix reads the run's feature-tag answer and judges fewer rows for it
// (gostaticcheck.DeriveScoped), while `go lint/run` reads no
// scope at all and loads the whole module whichever mode called it. Lint is a
// large part of a run's wall clock, so this list makes the population smaller
// without making the run proportionally faster. A stage that learns to narrow
// itself narrows fast mode too, with no edit here, and its Whole claim is then
// refused until it drops it (TestAWholeStageReachesNoChangeScopeReader).
func fastStages() []Stage {
	full := fullStages()
	stages := make([]Stage, 0, len(full)-3)
	for _, current := range full {
		switch current.Identity.Name {
		case "verify deps/evidence-vet", "verify deps/unit-cached", "verify deps/alloc":
			continue
		default:
			stages = append(stages, current)
		}
	}
	return stages
}

// changedStages returns the population of `./le verify changed`: the tests and
// the checks of what the change set reaches, and nothing that judges the whole
// tree. Every stage here narrows to the change set and widens only when the
// selector widened, which it says on stderr. Every stage but the race pieces
// reads the scope the run published; the race pieces select their groups from
// the same anchor through the refs, as they do in the full population, and are
// the only stages here declared Whole:
//
//   - `go lint/run/changed` lints the scope's packages
//   - `verify deps/unit-changed` tests the scope's packages under the scope's
//     tags, vets them, and compiles the bare ze_core binary
//   - the Staticcheck matrix judges the rows of the scope's tags
//   - the race pieces race the change set's groups
//   - the gating functional suite runs the suites the suite map selects
//
// It is the evidence a push owes, so an agent may start it: it is admitted
// through the job slots like any job and is not a release check.
func changedStages() []Stage {
	stages := []Stage{
		stage("go lint", "run", "changed"),
		stage("verify deps", "unit-changed"),
	}
	return slices.Concat(stages, staticcheckStages(), raceStages(), []Stage{
		stage(FunctionalCommand, "gating"),
	})
}

// whole declares each stage a full judgement in every population that holds
// it (Stage.Whole). It is the only way a stage claims one, so the claim is
// written where the stage is, and TestAWholeStageReachesNoChangeScopeReader
// (internal/le) refuses it for a stage that can reach a reader of the scope.
func whole(stages ...Stage) []Stage {
	for index := range stages {
		stages[index].Whole = true
	}
	return stages
}

// FunctionalCommand is the command every functional suite stage runs. Those
// stages bind the loopback addresses `./le setup` adds, so a caller that starts
// a population holding one checks the host first (verify.HostPreflight).
const FunctionalCommand = "test functional"

// Dealt answers the stages the given part of mode's population runs, or the
// error that says why the mode or the part names no population.
func Dealt(mode string, part Part) ([]Stage, error) {
	population := StagesForMode(mode)
	if len(population) == 0 {
		return nil, errors.New("no verify stages configured for mode " + mode)
	}
	return part.deal(population)
}

// StagesForMode returns a fresh stage population for a supported certificate
// mode. Unknown modes return nil and therefore cannot certify a tree.
func StagesForMode(mode string) []Stage {
	switch mode {
	case Mode:
		return fullStages()
	case FastMode:
		return fastStages()
	case ChangedMode:
		return changedStages()
	default:
		return nil
	}
}

func stage(command string, args ...string) Stage {
	return Stage{Identity: identity(command, args)}
}

func identity(command string, args []string) Identity {
	name := command
	if len(args) > 0 {
		name += "/" + strings.Join(args, "/")
	}
	return Identity{Name: name, Command: command, Args: args}
}
