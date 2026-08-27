// Design: docs/architecture/core-design.md -- what makes a tag evidence
// Overview: rfc.go -- the types, the paths and the closed sets every reader here shares
//
// carriers.go decides, for one path, whether a tag there is evidence at all.
//
// Evidence has TWO independent axes, and conflating them is how "we have
// interop coverage" becomes true and worthless at once. KIND is which layer the
// test exercises: a unit table test proves the algorithm, a `.ci` proves the
// daemon exposes it, an interop scenario proves a foreign peer accepts it. TIER
// is whether anything EXECUTES it, and a tag in a suite no pipeline runs is not
// weaker evidence -- it is the absence of evidence wearing evidence's clothes.
//
// Neither axis is a literal. A suite's verify tier comes from the run list that
// executes it, and an interop tree's nightly tier from whether a SCHEDULED
// workflow names its runner. Deleting that job takes the tier away, which is
// the property four hard-coded tiers could not have.
package rfc

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/functional"
)

// The three execution tiers.
const (
	tierVerify  = "verify"  // runs in a ze-precommit-verify stage, on every push
	tierNightly = "nightly" // runs in a scheduled advisory workflow
	tierUnrun   = "unrun"   // nothing runs it automatically: a tag here is refused
)

// draftPrefix is the gitignored incubator. Every repo-wide scanner SKIPS it
// rather than refusing it: a draft must be able neither to claim evidence nor
// to redden someone else's run.
const draftPrefix = "test/draft/"

// developmentToolsPrefix holds tests for the repository tool itself. They run
// in the unit suite, but they test the evidence scanner rather than a protocol
// implementation and therefore cannot prove an RFC requirement.
const developmentToolsPrefix = "internal/le/"

// editorSuite is the one suite walked for `.et`.
const editorSuite = "editor"

// workflowsRel is where CI declares what it runs.
const workflowsRel = ".github/workflows"

// Carrier is one recognized evidence carrier: its shape, its reader, and what
// executes it.
type Carrier struct {
	Name     string `json:"name"`     // stable identity, used in errors
	Kind     string `json:"kind"`     // evidence kind published in the ledger
	Tier     string `json:"tier"`     // execution tier, one of the three above
	Prefix   string `json:"prefix"`   // repo-relative path prefix ("" matches anywhere)
	Suffix   string `json:"suffix"`   // path suffix selecting this carrier
	Reader   string `json:"reader"`   // which scanner parses this shape
	Runner   string `json:"runner"`   // the make target that executes it
	Pipeline string `json:"pipeline"` // where that target runs
	// Derived is true for a row generated from the run list rather than
	// declared literally. The HEAD table swaps exactly these out.
	Derived bool `json:"derived"`
}

// interopTree is one interop tree: its path prefix and the make target that
// executes it. The TIER is not here, because the tier is not a property of the
// tree -- it is a property of whether CI runs that target.
type interopTree struct{ name, prefix, target string }

var interopTrees = [...]interopTree{
	{"interop-bgp", "test/interop/scenarios/", "ze-interop-test"},
	{"interop-ipsec", "test/interop-ipsec/", "ze-interop-ipsec-test"},
	{"interop-l2tp", "test/interop-l2tp/", "ze-deployment-docker-l2tp-ppp-test"},
	{"interop-pppoe", "test/interop-pppoe/", "ze-deployment-docker-pppoe-accel-test"},
}

// FunctionalSuites answers the suites `make ze-functional-test` runs.
//
// The Python reads them out of scripts/le/application/functional.py's source,
// because a `//go:build ignore` module cannot be imported by the gate that
// needs it. A compiled package has no such problem, so the list is READ from
// the package that runs it. The three refusals the source reader owed -- an
// unreadable module, two GATING assignments, a name with no Suite record --
// are two impossibilities and one check that already exists beside the list
// (internal/le/functional, TestEveryGatingNameIsASuite).
func FunctionalSuites() []string { return append([]string(nil), functional.Gating...) }

// suiteCarriers answers one verify-tier row per suite, so the prefix carries
// the execution claim.
//
// A single empty-prefix row is what let a `.ci` claim merge-gate tier by
// extension alone; per-suite prefixes make the claim checkable against the run
// list that produces them.
func suiteCarriers(kind, suffix, reader, runner, stage string, suites []string) []Carrier {
	out := make([]Carrier, 0, len(suites))
	for _, suite := range suites {
		var name, prefix, pipeline textbuf.Buffer
		out = append(out, Carrier{
			Name:     name.Str(kind).Byte('-').Str(suite).String(),
			Kind:     kind,
			Tier:     tierVerify,
			Prefix:   prefix.Str("test/").Str(suite).Byte('/').String(),
			Suffix:   suffix,
			Reader:   reader,
			Runner:   runner,
			Pipeline: pipeline.Str(stage).Str(", ").Str(suite).Str(" suite)").String(),
			Derived:  true,
		})
	}
	return out
}

// interopCarriers answers one row per interop tree, tier DERIVED from whether a
// scheduled workflow names its runner. A tree nobody schedules resolves unrun
// and its tags stay refused, which is the same answer four literals used to
// assert -- but now it is measured.
func interopCarriers(suffix string, scheduled map[string]string) []Carrier {
	out := make([]Carrier, 0, len(interopTrees))
	for _, tree := range interopTrees {
		workflow := scheduled[tree.target]
		tier, pipeline := tierUnrun, "no automated caller"
		if workflow != "" {
			var tb textbuf.Buffer
			tier = tierNightly
			pipeline = tb.Str(".github/workflows/").Str(workflow).Str(" (advisory)").String()
		}
		var runner textbuf.Buffer
		out = append(out, Carrier{
			Name: tree.name, Kind: "interop", Tier: tier, Prefix: tree.prefix,
			Suffix: suffix, Reader: "python",
			Runner:   runner.Str("make ").Str(tree.target).String(),
			Pipeline: pipeline,
		})
	}
	return out
}

// Carriers answers the whole table for one checkout.
//
// It is ORDERED: the first entry whose prefix AND suffix match wins, so the
// specific scenario trees are declared before the unclassified catch-all.
//
// It is computed per call rather than once, because two of its rows are read
// off the tree: a checkout whose workflows changed under a long-lived process
// must not be judged by the table the process started with.
func Carriers(tree string) ([]Carrier, error) {
	scheduled, err := ScheduledWorkflowTargets(tree)
	if err != nil {
		return nil, err
	}
	return carriersFor(FunctionalSuites(), scheduled), nil
}

// carriersFor answers the carrier table for an explicit suite and workflow
// snapshot. The HEAD baseline uses it with HEAD's own source data so a tier
// downgrade is visible instead of relabeling both sides with today's table.
func carriersFor(suites []string, scheduled map[string]string) []Carrier {
	var editorSuites []string
	for _, suite := range suites {
		if suite == editorSuite {
			editorSuites = append(editorSuites, suite)
		}
	}

	var unrunCI, unrunET textbuf.Buffer
	out := make([]Carrier, 0, len(suites)+len(editorSuites)+len(interopTrees)+5)
	out = append(out, Carrier{
		Name: "unit", Kind: "unit", Tier: tierVerify, Prefix: "", Suffix: "_test.go",
		Reader: "go", Runner: "make ze-unit-test",
		Pipeline: "ze-precommit-verify (unit stage)",
	})
	// ONE verify-tier row per suite the run list actually names. A single
	// empty-prefix row credited ANY .ci anywhere under internal/, pkg/ or test/
	// as merge-gate evidence, which made three silent evasions possible: move a
	// tagged .ci out of a run suite, into the gitignored incubator, or into a
	// tree whose sibling check.py the SAME table refuses as unrun.
	out = append(out, suiteCarriers("functional", ".ci", "ci",
		"make ze-functional-test", "ze-precommit-verify (functional stage", suites)...)
	// `.et` is the cheapest verify-tier non-unit carrier available, and it
	// costs one row: it is .ci semantics exactly, and only test/editor/ is
	// walked for it.
	out = append(out, suiteCarriers("editor", ".et", "ci",
		"make ze-functional-editor-test", "ze-precommit-verify (functional stage",
		editorSuites)...)
	// test/exabgp-compat is NOT one of the run list's suites: it has its own
	// stage. A separate target is a separate fact, so it is a declared row
	// rather than a second suite list. The two catch-alls follow it, reached by
	// a file under a suite no verify stage runs -- and they follow it in this
	// order because the FIRST matching row wins.
	out = append(out, Carrier{
		Name: "functional-exabgp", Kind: "functional", Tier: tierVerify,
		Prefix: "test/exabgp-compat/", Suffix: ".ci", Reader: "ci",
		Runner:   "make ze-functional-exabgp-test",
		Pipeline: "ze-precommit-verify (exabgp stage)",
	}, Carrier{
		Name: "functional-unrun", Kind: "functional", Tier: tierUnrun, Prefix: "",
		Suffix: ".ci", Reader: "ci",
		Runner: "no ze-precommit-verify stage walks this directory",
		Pipeline: unrunCI.Str("no automated caller; ze-functional-test runs ").
			Join(suites, ", ").String(),
	}, Carrier{
		Name: "editor-unrun", Kind: "editor", Tier: tierUnrun, Prefix: "",
		Suffix: ".et", Reader: "ci",
		Runner: "no ze-precommit-verify stage walks this directory",
		Pipeline: unrunET.Str("no automated caller; only test/").Str(editorSuite).
			Str("/ is walked for .et").String(),
	})
	out = append(out, interopCarriers("/check.py", scheduled)...)
	// Catch-all. Other trees hold check.py files, and any future tree will too.
	// Refusing a tag there by DEFAULT is the fail-closed shape: a carrier whose
	// pipeline nobody has declared is exactly the case where silence would be
	// indistinguishable from proof.
	out = append(out, Carrier{
		Name: "scenario-check", Kind: "unknown", Tier: tierUnrun, Prefix: "",
		Suffix: "/check.py", Reader: "python", Runner: "no declared runner",
		Pipeline: "no automated caller",
	})
	return out
}

// CarrierFor answers the carrier a repo-relative path belongs to, and false
// when the shape carries no tags.
//
// test/draft/ and internal/le/ answer false. The incubator executes in no
// repository gate. Development-tool tests exercise the scanner and cannot
// become protocol proof merely because the tool moved under internal/.
func CarrierFor(rel string, carriers []Carrier) (Carrier, bool) {
	if strings.HasPrefix(rel, draftPrefix) || strings.HasPrefix(rel, developmentToolsPrefix) {
		return Carrier{}, false
	}
	for _, one := range carriers {
		if strings.HasPrefix(rel, one.Prefix) && strings.HasSuffix(rel, one.Suffix) {
			return one, true
		}
	}
	return Carrier{}, false
}

// refuseUnrun is the message an unrun carrier's tag gets. A refusal, not a
// marker: a ledger note would decorate evidence that never executes, and a
// decorated absence still reads as presence.
func refuseUnrun(carrier Carrier, tag Tag) error {
	var tb textbuf.Buffer
	return parseErr(tb.Str(tag.File).Byte(':').Int(int64(tag.Line)).
		Str(": RFC requirement tag for ").Str(tag.RID).Str(" sits in carrier '").
		Str(carrier.Name).Str("', which nothing executes automatically (runner: ").
		Str(carrier.Runner).Str("; pipeline: ").Str(carrier.Pipeline).
		Str("). A tag is only evidence if something runs the test, so this one is ").
		Str("refused rather than counted. Fix it by adding that runner to a SCHEDULED ").
		Str("workflow (the interop jobs in .github/workflows/evidence-nightly.yml are ").
		Str("the pattern); an interop carrier's tier is derived from that set, so the ").
		Str("job is the whole fix and CARRIERS needs no edit -- or bind the requirement ").
		Str("to a .ci instead, which runs inside ze-precommit-verify on every push"))
}

// skipDirs are never walked: two hold code this repository does not author, and
// testdata holds fixtures rather than tests.
var skipDirs = map[string]bool{".git": true, "vendor": true, "testdata": true}

// ScanTree answers every tag under the three test roots.
//
// The walk visits a directory's files before its subdirectories and takes both
// in sorted order, which is the order the Python walk produces. It is
// load-bearing for one reason: an unrun carrier refuses on the FIRST tag it
// finds, so the order decides which of several offending files is named.
func ScanTree(tree string) ([]Tag, error) {
	carriers, err := Carriers(tree)
	if err != nil {
		return nil, err
	}
	var tags []Tag
	for _, sub := range testRoots {
		base := treePath(tree, sub)
		info, statErr := os.Stat(base)
		if statErr != nil || !info.IsDir() {
			continue
		}
		found, walkErr := scanDir(tree, base, carriers)
		if walkErr != nil {
			return nil, walkErr
		}
		tags = append(tags, found...)
	}
	return tags, nil
}

// scanDir walks one directory, files first, then subdirectories.
func scanDir(tree, dir string, carriers []Carrier) ([]Tag, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		var tb textbuf.Buffer
		return nil, parseErr(tb.Str(relTo(tree, dir)).Str(": cannot read: ").Err(err))
	}
	var files, dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			if !skipDirs[entry.Name()] {
				dirs = append(dirs, entry.Name())
			}
			continue
		}
		files = append(files, entry.Name())
	}
	sort.Strings(files)
	sort.Strings(dirs)

	var tags []Tag
	for _, name := range files {
		path := filepath.Join(dir, name)
		rel := relTo(tree, path)
		carrier, ok := CarrierFor(rel, carriers)
		if !ok {
			continue
		}
		src, err := readFile(path, rel)
		if err != nil {
			return nil, err
		}
		// THE pre-filter tagMarker exists for. Every tag in every carrier
		// contains this literal, so a file without it certainly holds none and
		// the expensive answer -- a per-line regex, or a whole Python comment
		// walk for a check.py -- never has to be computed. Skipping is safe
		// precisely BECAUSE the readers only ever report a tag whose line
		// contains it, so no reachable verdict can change.
		if !strings.Contains(src, tagMarker) {
			continue
		}
		found, err := readerFor(carrier.Reader)(src, rel)
		if err != nil {
			return nil, err
		}
		if len(found) > 0 && carrier.Tier == tierUnrun {
			return nil, refuseUnrun(carrier, found[0])
		}
		tags = append(tags, found...)
	}
	for _, name := range dirs {
		found, err := scanDir(tree, filepath.Join(dir, name), carriers)
		if err != nil {
			return nil, err
		}
		tags = append(tags, found...)
	}
	return tags, nil
}

// readerFor answers the scanner one carrier's shape needs.
func readerFor(name string) func(src, path string) ([]Tag, error) {
	switch name {
	case "go":
		return ScanGoTags
	case "ci":
		return ScanCITags
	}
	return ScanPythonTags
}

// ─── The workflow half: whether CI runs a target ────────────────────────────

var makeFlagsWithArg = map[string]bool{
	"-C": true, "-f": true, "-j": true, "-l": true, "-o": true, "-W": true,
}

// cmdWrappers sit BEFORE `make` and are not part of it.
var cmdWrappers = map[string]bool{"sudo": true, "env": true, "then": true, "do": true}

var workflowSuffixes = [...]string{".yml", ".yaml"}

// stripYAMLComments removes `#` line comments, so a commented-out command can
// never grant a tier.
func stripYAMLComments(src string) string {
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		lines[i], _, _ = strings.Cut(line, "#")
	}
	return strings.Join(lines, "\n")
}

// MakeTargetsIn answers every make target a workflow body invokes, one bare
// word after `make` per entry.
//
// It models shell command LINES, not a shell: `$(MAKE)`, `bash -c "..."`,
// backticks and subshells are not parsed.
func MakeTargetsIn(src string) []string {
	var out []string
	for line := range strings.SplitSeq(src, "\n") {
		cmd := strings.TrimSpace(line)
		cmd = strings.TrimPrefix(cmd, "- ")
		// A quoted YAML scalar is not a different command.
		cmd = strings.Trim(cmd, "\"'")
		for _, sep := range []string{"&&", ";", "||", "|"} {
			cmd = strings.ReplaceAll(cmd, sep, "\x00")
		}
		for frag := range strings.SplitSeq(cmd, "\x00") {
			out = append(out, targetsInCommand(strings.Fields(frag))...)
		}
	}
	return out
}

func targetsInCommand(fields []string) []string {
	if len(fields) > 0 && fields[0] == "-" {
		fields = fields[1:]
	}
	// `run: make ...` -- the YAML command key is not part of the command.
	if len(fields) > 0 && fields[0] == "run:" {
		fields = fields[1:]
	}
	for len(fields) > 0 && (cmdWrappers[fields[0]] ||
		strings.HasPrefix(fields[0], "-") || strings.Contains(fields[0], "=")) {
		fields = fields[1:]
	}
	if len(fields) < 2 || fields[0] != "make" {
		return nil
	}
	var out []string
	for i := 1; i < len(fields); i++ {
		arg := fields[i]
		if makeFlagsWithArg[arg] {
			i++ // the flag AND its separate argument
			continue
		}
		if !strings.HasPrefix(arg, "-") && !strings.Contains(arg, "=") {
			// EVERY bare word is a target: `make a b` invokes both.
			out = append(out, arg)
		}
	}
	return out
}

// topLevelBlock answers the indented body of a top-level `key:` line, and false
// when there is no such block. Scoping the trigger test to the `on:` block is
// what keeps it honest: a step command containing the word `schedule` would
// satisfy a whole-file substring test.
func topLevelBlock(src, key string) (string, bool) {
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		if strings.TrimRight(line, " \t") != key {
			continue
		}
		var body []string
		for _, next := range lines[i+1:] {
			if strings.TrimSpace(next) == "" {
				body = append(body, next)
				continue
			}
			if !strings.HasPrefix(next, " ") && !strings.HasPrefix(next, "\t") {
				break // back to column 0: the block ended
			}
			body = append(body, next)
		}
		return strings.Join(body, "\n"), true
	}
	return "", false
}

// isScheduled reports whether a workflow's own `on:` trigger includes
// `schedule`.
//
// It answers NO for anything it cannot classify. That direction is the safe
// one: an unclassified workflow grants no tier, which refuses tags rather than
// crediting evidence to a pipeline nobody confirmed.
func isScheduled(src string) bool {
	if block, ok := topLevelBlock(src, "on:"); ok {
		return strings.Contains(block, "schedule")
	}
	for line := range strings.SplitSeq(src, "\n") {
		if trigger, found := strings.CutPrefix(line, "on:"); found {
			return strings.Contains(trigger, "schedule")
		}
	}
	return false
}

// ScheduledTargetsFrom answers `make target -> the scheduled workflow that runs
// it`. The pure core, so the tree and the HEAD table share one notion of "CI
// runs this".
func ScheduledTargetsFrom(sources map[string]string) map[string]string {
	out := map[string]string{}
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		src := stripYAMLComments(sources[name])
		if !isScheduled(src) {
			continue
		}
		for _, target := range MakeTargetsIn(src) {
			if _, seen := out[target]; !seen {
				out[target] = name
			}
		}
	}
	return out
}

// ReadWorkflowSources answers every workflow file's text, keyed by base name.
// It raises rather than answering an empty map: not knowing what CI runs is a
// different fact from CI running nothing.
func ReadWorkflowSources(tree string) (map[string]string, error) {
	dir := treePath(tree, workflowsRel)
	entries, err := os.ReadDir(dir)
	if err != nil {
		var tb textbuf.Buffer
		return nil, parseErr(tb.Str(dir).
			Str(": cannot read the workflow directory, so which make targets CI runs on ").
			Str("a schedule is unknown. The interop evidence tier is derived from that ").
			Str("set, and a gate that answers 'everything runs' in this state would ").
			Str("credit evidence to a pipeline nobody confirmed: ").Err(err))
	}
	sources := map[string]string{}
	for _, entry := range entries {
		name := entry.Name()
		if !hasWorkflowSuffix(name) {
			continue
		}
		var tb textbuf.Buffer
		raw, readErr := os.ReadFile(filepath.Join(dir, name)) // #nosec G304 -- a workflow of the checkout
		if readErr != nil {
			return nil, parseErr(tb.Str(dir).Byte('/').Str(name).
				Str(": cannot read the workflow: ").Err(readErr))
		}
		sources[name] = string(raw)
	}
	if len(sources) == 0 {
		var tb textbuf.Buffer
		return nil, parseErr(tb.Str(dir).
			Str(": no *.yml/*.yaml workflow files found, so no scheduled pipeline can ").
			Str("be confirmed and no interop carrier can justify a tier"))
	}
	return sources, nil
}

func hasWorkflowSuffix(name string) bool {
	for _, suffix := range workflowSuffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

// ScheduledWorkflowTargets answers which make targets a scheduled workflow
// invokes. It fails closed: an unreadable or empty workflow directory means we
// do not know what CI runs.
func ScheduledWorkflowTargets(tree string) (map[string]string, error) {
	sources, err := ReadWorkflowSources(tree)
	if err != nil {
		return nil, err
	}
	return ScheduledTargetsFrom(sources), nil
}
