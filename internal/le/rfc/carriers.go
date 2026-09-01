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
	"github.com/ze-software/ze/internal/le/leroot"
)

// The three execution tiers.
const (
	tierVerify  = "verify"  // runs in the native full verifier on every push
	tierNightly = "nightly" // runs in a scheduled advisory workflow
	tierUnrun   = "unrun"   // nothing runs it automatically: a tag here is refused
)

// The evidence KINDS a carrier declares: which layer a tagged test exercises.
// A unit table test proves the algorithm, a `.ci` proves the daemon exposes it,
// an interop scenario proves a foreign peer accepts it.
const (
	kindUnit       = "unit"
	kindFunctional = "functional"
	kindEditor     = "editor"
	kindInterop    = "interop"
	// kindUnknown is the interop tree nothing runs. It declares a kind of its
	// own rather than borrowing kindInterop, because a tag there is refused and
	// calling it interop would credit it with a pipeline it does not have.
	kindUnknown = "unknown"
)

// carrierKindOrder and carrierTierOrder are the order evidence READS in, from
// the cheapest and most certain to the most distant.
//
// Unit first: it proves the algorithm and it runs on every push. Then
// functional, which proves the daemon exposes the behavior, then editor, then
// interop, which proves a foreign peer accepts it and is the slowest and most
// often nightly. Unknown last, because nothing runs it.
//
// Tier orders WITHIN a kind, so unit/verify precedes unit/nightly and both
// precede every functional row.
//
// The order lives HERE, beside the vocabulary it orders. A consumer that
// published its own sequence would be a second declaration of this set, and it
// would sort a kind added here to the end in silence (ai/rules/principles.md).
var (
	carrierKindOrder = []string{kindUnit, kindFunctional, kindEditor, kindInterop, kindUnknown}
	carrierTierOrder = []string{tierVerify, tierNightly, tierUnrun}
)

// CarrierRank answers one sortable rank for a `kind/tier` pair, and false for a
// pair this vocabulary does not declare.
//
// A caller that publishes evidence in order reads this rather than writing the
// sequence again. The false answer is not a default: a caller MUST place an
// unranked pair deliberately rather than let it land at rank zero, which is
// where `unit/verify` lives (ai/rules/principles.md).
func CarrierRank(kind, tier string) (int, bool) {
	kindAt := indexOf(carrierKindOrder, kind)
	tierAt := indexOf(carrierTierOrder, tier)
	if kindAt < 0 || tierAt < 0 {
		return 0, false
	}
	return kindAt*len(carrierTierOrder) + tierAt, true
}

// CarrierLabelRank answers the rank of a `kind/tier` label as the ledger prints
// it, which is the form every consumer of a tagged unit holds.
func CarrierLabelRank(label string) (int, bool) {
	kind, tier, held := strings.Cut(label, "/")
	if !held {
		return 0, false
	}
	return CarrierRank(kind, tier)
}

// indexOf answers where one value sits in an ordered vocabulary, and -1 for a
// value outside it.
func indexOf(order []string, value string) int {
	for at, one := range order {
		if one == value {
			return at
		}
	}
	return -1
}

// The two functional carrier suffixes, and the pipeline an unrun carrier names.
const (
	ciSuffix          = ".ci"
	etSuffix          = ".et"
	noAutomatedCaller = "no automated caller"
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
	Runner   string `json:"runner"`   // the native action that executes it
	Pipeline string `json:"pipeline"` // where that action runs
	// Derived is true for a row generated from the run list rather than
	// declared literally. The HEAD table swaps exactly these out.
	Derived bool `json:"derived"`
}

// interopTree is one native interop test package and the action that executes it.
// The tier is derived from whether CI schedules that action.
type interopTree struct{ name, prefix, action string }

var interopTrees = [...]interopTree{
	{"interop-bgp", "internal/le/interoplab/bgp/", "integration/interop"},
	{"interop-ipsec", "internal/le/interoplab/ipsec/", "integration/interop-ipsec"},
	{"interop-l2tp", "internal/le/interoplab/l2tp/", "deployment/docker-l2tp-ppp-test"},
	{"interop-pppoe", "internal/le/interoplab/pppoe/", "deployment/docker-pppoe-accel-test"},
}

var legacyInteropTrees = [...]interopTree{
	{"interop-bgp", "test/interop/scenarios/", "integration/interop"},
	{"interop-ipsec", "test/interop-ipsec/", "integration/interop-ipsec"},
	{"interop-l2tp", "test/interop-l2tp/", "deployment/docker-l2tp-ppp-test"},
	{"interop-pppoe", "test/interop-pppoe/", "deployment/docker-pppoe-accel-test"},
}

// FunctionalSuites answers the suites `./le functional` runs.
//
// The answer comes from the functional area's read-only catalog. RFC evidence
// classification therefore consumes the same run list as the native runner
// without importing or parsing internal/le/functional/actions.go.
func FunctionalSuites() []string { return functional.GatingNames() }

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
func interopCarriers(scheduled map[string]string) []Carrier {
	out := make([]Carrier, 0, len(interopTrees))
	for _, tree := range interopTrees {
		workflow := scheduled[tree.action]
		tier, pipeline := tierUnrun, noAutomatedCaller
		if workflow != "" {
			var tb textbuf.Buffer
			tier = tierNightly
			pipeline = tb.Str(".github/workflows/").Str(workflow).Str(" (advisory)").String()
		}
		out = append(out, Carrier{
			Name: tree.name, Kind: kindInterop, Tier: tier, Prefix: tree.prefix,
			Suffix: ".go", Reader: "go",
			Runner:   "./le " + strings.Replace(tree.action, "/", " ", 1),
			Pipeline: pipeline,
		})
	}
	return out
}

func legacyInteropCarriers(scheduled map[string]string) []Carrier {
	out := make([]Carrier, 0, len(legacyInteropTrees))
	for _, tree := range legacyInteropTrees {
		workflow := scheduled[tree.action]
		tier, pipeline := tierUnrun, noAutomatedCaller
		if workflow != "" {
			tier = tierNightly
			pipeline = ".github/workflows/" + workflow + " (advisory)"
		}
		out = append(out, Carrier{
			Name: tree.name, Kind: kindInterop, Tier: tier, Prefix: tree.prefix,
			Suffix: "/check.py", Reader: "legacy-python",
			Runner:   "./le " + strings.Replace(tree.action, "/", " ", 1),
			Pipeline: pipeline,
		})
	}
	return out
}

// carriers answers the whole table for one checkout.
//
// It is ORDERED: the first entry whose prefix AND suffix match wins, so the
// specific scenario trees are declared before the unclassified catch-all.
//
// It is computed per call rather than once, because two of its rows are read
// off the tree: a checkout whose workflows changed under a long-lived process
// must not be judged by the table the process started with.
func carriers(tree string) ([]Carrier, error) {
	scheduled, err := scheduledWorkflowActions(tree)
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
	out = append(out, interopCarriers(scheduled)...)
	out = append(out, Carrier{
		Name: "interop-unrun", Kind: kindUnknown, Tier: tierUnrun, Prefix: "internal/le/interoplab/",
		Suffix: ".go", Reader: "go", Runner: "no declared native interop action",
		Pipeline: noAutomatedCaller,
	}, Carrier{
		Name: kindUnit, Kind: kindUnit, Tier: tierVerify, Prefix: "", Suffix: "_test.go",
		Reader: "go", Runner: "./le verify deps unit-cached",
		Pipeline: "./le verify current mode full (unit stage)",
	})
	// ONE verify-tier row per suite the run list actually names. A single
	// empty-prefix row credited ANY .ci anywhere under internal/, pkg/ or test/
	// as verify evidence, which made two silent evasions possible: move a
	// tagged .ci out of a run suite or into the gitignored incubator.
	out = append(out, suiteCarriers(kindFunctional, ciSuffix, "ci",
		"./le functional", "./le verify current mode full (functional stage", suites)...)
	// `.et` is the cheapest verify-tier non-unit carrier available, and it
	// costs one row: it is .ci semantics exactly, and only test/editor/ is
	// walked for it.
	out = append(out, suiteCarriers(kindEditor, etSuffix, "ci",
		"./le functional editor", "./le verify current mode full (functional stage",
		editorSuites)...)
	// test/exabgp-compat is not one of the run list's suites. It has its own
	// native action and stage, so it is a declared row rather than a second
	// suite list.
	out = append(out, Carrier{
		Name: "functional-exabgp", Kind: kindFunctional, Tier: tierVerify,
		Prefix: "test/exabgp-compat/", Suffix: ciSuffix, Reader: "ci",
		Runner:   "./le functional exabgp-test",
		Pipeline: "./le verify current mode full (exabgp stage)",
	}, Carrier{
		Name: "functional-unrun", Kind: kindFunctional, Tier: tierUnrun, Prefix: "",
		Suffix: ciSuffix, Reader: "ci",
		Runner: "no native full-verifier stage walks this directory",
		Pipeline: unrunCI.Str("no automated caller; ./le functional runs ").
			Join(suites, ", ").String(),
	}, Carrier{
		Name: "editor-unrun", Kind: kindEditor, Tier: tierUnrun, Prefix: "",
		Suffix: etSuffix, Reader: "ci",
		Runner: "no native full-verifier stage walks this directory",
		Pipeline: unrunET.Str("no automated caller; only test/").Str(editorSuite).
			Str("/ is walked for .et").String(),
	})
	return out
}

// CarrierFor answers the carrier a repo-relative path belongs to, and false
// when the shape carries no tags.
//
// test/draft/ and development-tool tests outside interoplab answer false.
// Interoplab tests exercise foreign implementations and can carry protocol
// evidence when their native action is scheduled.
func CarrierFor(rel string, carriers []Carrier) (Carrier, bool) {
	if strings.HasPrefix(rel, draftPrefix) ||
		(strings.HasPrefix(rel, developmentToolsPrefix) && !strings.HasPrefix(rel, "internal/le/interoplab/")) {
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
		Str("to a .ci instead, which runs inside the native full verifier on every push"))
}

// skipDirs are never walked: two hold code this repository does not author, and
// testdata holds fixtures rather than tests.
var skipDirs = map[string]bool{".git": true, "vendor": true, "testdata": true}

// ScanTree answers every tag under the three test roots.
//
// The walk visits a directory's files before its subdirectories and takes both
// them in sorted order. It is load-bearing because an unrun carrier refuses on
// the first tag it finds, so order decides which offending file is named.
func ScanTree(tree string) ([]Tag, error) {
	carriers, err := carriers(tree)
	if err != nil {
		return nil, err
	}
	return scanTreeWith(tree, carriers)
}

// scanTreeWith is that same walk over a carrier table the caller resolved.
//
// One walk, so a caller that already has a table cannot end up with a second
// scanner beside this one.
func scanTreeWith(tree string, carriers []Carrier) ([]Tag, error) {
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
		// the per-line scanner never has to run. Skipping is safe because each
		// reader only reports a tag whose line contains the marker.
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

// UnscannedTag is one `RFC requirement:` comment in production Go, where no
// carrier reads it.
//
// It looks like evidence to a person opening the file and is counted by
// nothing: no gate resolves its id, no gate demands its polarity, and no gate
// asks whether anything runs it. Ten of them sit in this checkout.
type UnscannedTag struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Rest     string `json:"rest"`
	Refusal  string `json:"refusal,omitempty"`
	Polarity string `json:"polarity,omitempty"`
}

// unscannedTags answers every `RFC requirement:` comment sitting in a non-test
// Go file that no carrier claims.
//
// Not a violation, and deliberately so: the ten in this tree predate the check,
// and a ratchet that reds the tree over standing debt gets removed rather than
// obeyed. It is REPORTED, in the shape the extraction backlog is reported, so a
// reader can see the population and act on it.
//
// The scan is the tag comment rather than the marker string. `internal/le/rfc`
// spells the marker in a constant and in three regexes, and a marker match
// would report this package's own parser as evidence.
//
// A test file is out of scope by construction: every one of them either sits on
// a carrier, or sits under a path CarrierFor refuses on purpose, and both of
// those are answers rather than oversights.
func unscannedTags(tree string, carriers []Carrier) ([]UnscannedTag, error) {
	var out []UnscannedTag
	for _, sub := range testRoots {
		base := treePath(tree, sub)
		info, statErr := os.Stat(base)
		if statErr != nil || !info.IsDir() {
			continue
		}
		found, walkErr := scanUnscannedDir(tree, base, carriers)
		if walkErr != nil {
			return nil, walkErr
		}
		out = append(out, found...)
	}
	return out, nil
}

// scanUnscannedDir walks one directory for production Go carrying a tag.
func scanUnscannedDir(tree, dir string, carriers []Carrier) ([]UnscannedTag, error) {
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
		if strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	sort.Strings(dirs)

	var out []UnscannedTag
	for _, name := range files {
		path := filepath.Join(dir, name)
		rel := relTo(tree, path)
		if _, carried := CarrierFor(rel, carriers); carried {
			continue
		}
		src, readErr := readFile(path, rel)
		if readErr != nil {
			return nil, readErr
		}
		if !strings.Contains(src, tagMarker) {
			continue
		}
		out = append(out, unscannedTagsIn(src, rel)...)
	}
	for _, name := range dirs {
		found, walkErr := scanUnscannedDir(tree, filepath.Join(dir, name), carriers)
		if walkErr != nil {
			return nil, walkErr
		}
		out = append(out, found...)
	}
	return out, nil
}

// unscannedTagsIn reads one production file's tags, and records for each
// whether parseTagRest would have accepted it.
//
// The refusal is the sharp half of the report. A tag with no polarity would be
// REFUSED outright by every scanner, so it is not merely uncounted evidence: it
// is evidence that could not be counted even if a carrier claimed the file.
func unscannedTagsIn(src, rel string) []UnscannedTag {
	var out []UnscannedTag
	for index, line := range strings.Split(src, "\n") {
		found := goTagRE.FindStringSubmatch(line)
		if found == nil {
			continue
		}
		entry := UnscannedTag{File: rel, Line: index + 1, Rest: strings.TrimSpace(found[1])}
		tag, err := parseTagRest(found[1], tagWhere(rel, index+1))
		if err != nil {
			entry.Refusal = err.Error()
		} else {
			entry.Polarity = tag.Polarity
		}
		out = append(out, entry)
	}
	return out
}

// readerFor answers the scanner one carrier's shape needs.
func readerFor(name string) func(src, path string) ([]Tag, error) {
	switch name {
	case "go":
		return scanGoTags
	case "ci":
		return scanCITags
	case "legacy-python":
		return scanLegacyPythonTags
	default:
		return func(_ string, path string) ([]Tag, error) {
			var tb textbuf.Buffer
			return nil, parseErr(tb.Str(path).Str(": unknown RFC carrier reader ").Str(name))
		}
	}
}

// ─── The workflow half: whether CI runs a native action ─────────────────────

// cmdWrappers sit before `./le` and are not part of its action identity.
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

// registeredCommand reports whether a whole name is a registered le command.
//
// It is a seam rather than a direct call so a test can state its own command
// population: this package's test binary links only this package, so a live
// registry read there answers one command and would make every namespaced
// invocation parse as its one-word fallback.
func registeredCommand(name string) bool { return leroot.LookupCommand(name) != nil }

// nativeActionsIn answers every `./le <area> <verb>` action a workflow invokes.
//
// It models command lines, not a shell. Chains are split, but substitutions,
// backticks, and subshells are not parsed.
func nativeActionsIn(src string, registered func(string) bool) []string {
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
			out = append(out, actionsInCommand(strings.Fields(frag), registered)...)
		}
	}
	return out
}

func actionsInCommand(fields []string, registered func(string) bool) []string {
	if len(fields) > 0 && fields[0] == "-" {
		fields = fields[1:]
	}
	// The YAML command key is not part of the command.
	if len(fields) > 0 && fields[0] == "run:" {
		fields = fields[1:]
	}
	for len(fields) > 0 && (cmdWrappers[fields[0]] ||
		strings.HasPrefix(fields[0], "-") || strings.Contains(fields[0], "=")) {
		fields = fields[1:]
	}
	if len(fields) < 3 || (fields[0] != "./le" && fields[0] != "le") {
		return nil
	}
	// Which words are the command is a question only the registry answers. A
	// namespaced command takes two of them, so `./le doc check links` is the
	// links verb of `doc check`, while `./le verify current mode full` is the
	// current verb of `verify` and `mode` is its argument. Counting words would
	// read the first as the `check` verb of a `doc` command that does not
	// exist, and the second correctly, which is the worst kind of wrong.
	var tb textbuf.Buffer
	twoWord := tb.Str(fields[1]).Byte(' ').Str(fields[2]).String()
	if len(fields) > 3 && registered(twoWord) {
		tb.Reset()
		return []string{tb.Str(twoWord).Byte('/').Str(fields[3]).String()}
	}

	tb.Reset()
	return []string{tb.Str(fields[1]).Byte('/').Str(fields[2]).String()}
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

// scheduledActionsFrom answers `area/verb -> the scheduled workflow that runs
// it`. The pure core, so the tree and the HEAD table share one notion of "CI
// runs this".
func scheduledActionsFrom(sources map[string]string) map[string]string {
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
		for _, action := range nativeActionsIn(src, registeredCommand) {
			if _, seen := out[action]; !seen {
				out[action] = name
			}
		}
	}
	return out
}

// readWorkflowSources answers every workflow file's text, keyed by base name.
// It raises rather than answering an empty map: not knowing what CI runs is a
// different fact from CI running nothing.
func readWorkflowSources(tree string) (map[string]string, error) {
	dir := treePath(tree, workflowsRel)
	entries, err := os.ReadDir(dir)
	if err != nil {
		var tb textbuf.Buffer
		return nil, parseErr(tb.Str(dir).
			Str(": cannot read the workflow directory, so which native actions CI runs on ").
			Str("a schedule is unknown. The interop evidence tier is derived from that ").
			Str("set, and a check that answers 'everything runs' in this state would ").
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

// scheduledWorkflowActions answers which native actions a scheduled workflow
// invokes. It fails closed: an unreadable or empty workflow directory means we
// do not know what CI runs.
func scheduledWorkflowActions(tree string) (map[string]string, error) {
	sources, err := readWorkflowSources(tree)
	if err != nil {
		return nil, err
	}
	return scheduledActionsFrom(sources), nil
}
