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
	// Structural says a red here means the tree is BROKEN rather than merely
	// unverified, so a commit is refused instead of taking a debt row.
	//
	// It lives on the stage because the population lives here. The commit gate
	// used to keep its own list of the same eight names, and a stage renamed
	// or re-argumented in this file silently left that set: its red then filed
	// as unattributed and the commit went through. Two lists of one population
	// drift, and this one drifted toward letting commits past a broken tree.
	Structural bool `json:"structural,omitempty"`
	// Scoped says the stage's action reads the change set the run publishes
	// (publishChangeScope) and judges only that part of its subject when the
	// answer is narrow. The full population publishes the whole tree, so there
	// it judges everything; a partial population's verdict on it covers its
	// scope alone (JudgesInPart, carryForward). An action that starts reading
	// the published scope MUST be marked here, or a partial green would replace
	// a full red it never re-judged.
	Scoped bool `json:"scoped,omitempty"`
}

// JudgesInPart reports whether mode's population judges the named stage only
// over its published scope while the full population judges all of it: the
// stage is in both populations and Scoped. Only such a stage's full verdict
// survives a partial run; a stage the partial run judges at its full identity
// is a full judgement. The full population judges every stage in full.
func JudgesInPart(mode, name string) bool {
	if mode == Mode {
		return false
	}
	partial := false
	for _, one := range StagesForMode(mode) {
		if one.Identity.Name == name {
			partial = one.Scoped
			break
		}
	}
	if !partial {
		return false
	}
	for _, one := range fullStages() {
		if one.Identity.Name == name {
			return true
		}
	}
	return false
}

// Structural answers the stages whose red refuses a commit outright.
func Structural(mode string) map[string]bool {
	named := map[string]bool{}
	for _, one := range StagesForMode(mode) {
		if one.Structural {
			named[one.Identity.Name] = true
		}
	}
	return named
}

// staticcheckStages returns one stage per piece of the Staticcheck feature
// matrix. The rows each piece judges are dealt by the matrix itself, so a
// feature gate added to `feature-gates.txt` needs nothing here.
func staticcheckStages() []Stage {
	stages := make([]Stage, 0, staticcheckParts)
	for part := 1; part <= staticcheckParts; part++ {
		stages = append(stages, scoped(structural("go staticcheck", "check",
			"part", strconv.Itoa(part), "of", strconv.Itoa(staticcheckParts))))
	}
	return stages
}

// raceStages returns one stage per piece of the race pass. Each piece lists
// the changed population itself and runs its share, so a package added to the
// tree needs nothing here.
func raceStages() []Stage {
	stages := make([]Stage, 0, raceParts)
	for part := 1; part <= raceParts; part++ {
		stages = append(stages, stage("verify deps", "unit-race-changed",
			"part", strconv.Itoa(part), "of", strconv.Itoa(raceParts)))
	}
	return stages
}

// fullStages returns the native verification actions in execution order.
func fullStages() []Stage {
	stages := []Stage{
		structural("go lint", "run"),
		structural("arch tier", "check"),
		stage("rfc", "check"),
		structural("arch iface-resolution"),
		structural("plugin boundary", "check"),
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
	}
	stages = slices.Concat(stages, staticcheckStages(), []Stage{
		structural("repo compiles", "check"),
		stage("go vet-platforms", "darwin", "freebsd", "openbsd"),
		structural("doc wiring"),
		stage("doc check", "verify"),
		stage("doc check", "links"),
		// Every caller of a name the subject-first rename retired is rewritten,
		// so a line naming one again is a regression (AC-16 of
		// spec-le-subject-first-command-tree).
		stage("doc check", "retired-commands"),
		stage("repo", "tree-check"),
		stage("plugin imports", "check"),
		// The gated-plugin table is generated from the same manifest and the
		// registrations, so a gate added without regenerating it reddens here.
		stage("plugin gated-table", "check"),
		// A plugin that consumes what only another gate's plugin produces, with
		// no declared edge, runs silently empty in a build without that gate.
		stage("plugin coupling", "check"),
		stage("plugin declarations", "check"),
		// arch enumeration judges the CHANGE SET rather than the tree: the copies
		// already in the tree are `le arch enumeration report`, and this stage's red
		// means the change under test added one (owner decision, 2026-09-14).
		scoped(stage("arch enumeration", "check")),
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
		stage("ai rules", "render-check"),
		stage("ai rules", "index-check"),
		stage("ai rules", "condensed-check"),
		stage("ai rules", "lint"),
		stage("repo arch-map", "check"),
		stage("test health", "check"),
		stage("site facts", "check"),
		stage("web htmx", "check"),
		structural("verify deps", "evidence-vet"),
		stage("ai hooks", "unit"),
		stage("verify deps", "vulnerability"),
		stage("verify deps", "unit-cached"),
	}, raceStages(), []Stage{
		stage(FunctionalCommand, "exabgp-test"),
		scoped(stage(FunctionalCommand, "gating")),
		stage("verify deps", "alloc"),
	})
	return stages
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
// itself narrows fast mode too, with no edit here.
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
// selector widened, which it says on stderr. Each stage marked scoped reads the
// scope the run published; the race pieces select their groups from the same
// anchor through the refs, as they do in the full population:
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
		scoped(structural("go lint", "run", "changed")),
		scoped(stage("verify deps", "unit-changed")),
	}
	return slices.Concat(stages, staticcheckStages(), raceStages(), []Stage{
		scoped(stage(FunctionalCommand, "gating")),
	})
}

// scoped marks a stage whose action reads the run's published change set.
func scoped(one Stage) Stage {
	one.Scoped = true
	return one
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

// structural is stage for an action whose red says the tree is broken.
func structural(command string, args ...string) Stage {
	return Stage{Identity: identity(command, args), Structural: true}
}

func identity(command string, args []string) Identity {
	name := command
	if len(args) > 0 {
		name += "/" + strings.Join(args, "/")
	}
	return Identity{Name: name, Command: command, Args: args}
}
