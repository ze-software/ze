// Design: docs/architecture/core-design.md -- what a tagged unit could be broken by
// Overview: discriminate.go -- the record those candidates become
// Detail: discriminate_action.go -- the record mode that observes one of them
// Related: goscope.go -- UnitAt, the one definition of the tagged unit
//
// discriminate_propose.go turns a gomu report into candidate breaks for one
// tagged unit. Two filters and one ranking: a candidate must sit in code the
// tagged unit EXECUTES, because a mutant the unit never reaches can never
// redden it, and the candidates that touch a symbol the tag's own prose names
// come first, because those are the ones that engage the claim.
//
// Nothing here writes. Proposing is reading, and the human decision it leaves
// is "pick one and read it" rather than "invent a break".
package rfc

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/gotoolchain"
)

// DiscriminationCandidate is one break that could be recorded for one tag.
//
// Mutant is the selector `./le rfc discriminate-record` takes: the mutant's
// position in the producer, which is stable under a report regenerated over the
// same tree and unique unless two operators apply at one column.
type DiscriminationCandidate struct {
	RID      string   `json:"rid"`
	Polarity string   `json:"polarity"`
	Unit     string   `json:"unit"`
	Producer string   `json:"producer"`
	Mutant   string   `json:"mutant"`
	Break    string   `json:"break"`
	Symbols  []string `json:"symbols"`
}

// gomuKilled is the status gomu writes for a mutant its package's tests noticed.
const gomuKilled = "KILLED"

// reportMutant is one mutant gomu generated, in the fields a candidate needs.
//
// A decoder of its own rather than gomu's Go type: `check` must never link
// against a mutation engine, and the dependency this area accepts is on the
// report FORMAT, which is vendored, not on a run (R-5).
type reportMutant struct {
	FilePath string `json:"filePath"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Original string `json:"original"`
	Mutated  string `json:"mutated"`
	// Killed says the package's own tests noticed this mutant. A mutant nothing
	// noticed cannot be reddened by ONE of those tests either, and a mutant that
	// did not compile is a build failure rather than a break, so neither is ever
	// proposed.
	Killed bool `json:"-"`
	// Ordinal separates the operators that apply at ONE position, 1-based in
	// report order. gomu's own id does not: one report carries `config.go_2`
	// twice, and two operators at one column are ordinary rather than rare --
	// `hasRole` is replaced by both `true` and `false` at 190:4.
	Ordinal int `json:"-"`
}

// at answers the position this mutant sits at.
func (m reportMutant) at() string {
	var tb textbuf.Buffer
	return tb.Str(m.FilePath).Byte(':').Int(int64(m.Line)).Byte(':').Int(int64(m.Column)).String()
}

// selector answers the value `./le rfc discriminate-record` takes under
// `mutant`, which names exactly one operator.
func (m reportMutant) selector() string {
	var tb textbuf.Buffer
	return tb.Str(m.at()).Byte('#').Int(int64(m.Ordinal)).String()
}

// text answers what the break does, for a reviewer to read.
func (m reportMutant) text() string {
	var tb textbuf.Buffer
	return tb.Str("line ").Int(int64(m.Line)).Str(": ").Quoted(m.Original).
		Str(" -> ").Quoted(m.Mutated).String()
}

// loadGomuReport reads the mutants one `gomu run --output json` generated.
//
// The report is an author-supplied path, so a file that cannot be read is an
// error rather than an empty candidate list: "no candidate" and "I read
// nothing" must never be the same answer (ai/rules/principles.md).
func loadGomuReport(tree, path string) ([]reportMutant, error) {
	raw, err := readFile(path, path)
	if err != nil {
		return nil, err
	}
	var file struct {
		Results []struct {
			Mutant reportMutant `json:"mutant"`
			Status string       `json:"status"`
		} `json:"results"`
	}
	var tb textbuf.Buffer
	if decodeErr := json.Unmarshal([]byte(raw), &file); decodeErr != nil {
		return nil, parseErr(tb.Str(path).Str(": cannot read as a gomu report: ").Err(decodeErr))
	}
	if len(file.Results) == 0 {
		return nil, parseErr(tb.Str(path).Str(": holds no result. A report with no mutant ").
			Str("proposes nothing, and an empty proposal must not read as a producer with ").
			Str("no break in it"))
	}
	out := make([]reportMutant, 0, len(file.Results))
	seen := map[string]int{}
	for index := range file.Results {
		mutant := file.Results[index].Mutant
		rel, relErr := repoRelative(tree, mutant.FilePath, path)
		if relErr != nil {
			return nil, relErr
		}
		mutant.FilePath = rel
		mutant.Killed = file.Results[index].Status == gomuKilled
		seen[mutant.at()]++
		mutant.Ordinal = seen[mutant.at()]
		out = append(out, mutant)
	}
	return out, nil
}

// repoRelative answers a report path as this checkout spells it, and refuses one
// that does not name a file in it.
//
// gomu writes the path it walked, which is absolute when it was run with an
// absolute root. Every other key in this package is repo-relative, and two
// spellings of one file would compare unequal in the coverage lookup.
//
// The report is AUTHORED INPUT and its path is not a trusted one: it becomes the
// key of a Go overlay, and on the interop carrier requireRedInTree writes the
// broken source to it with os.WriteFile. So it passes the refusal a record's
// own keys pass, rather than a trim that would carry `../` straight through.
func repoRelative(tree, path, where string) (string, error) {
	// The tree prefix and its separator come off together. Trimming a leading
	// slash on its own turns /etc/passwd into etc/passwd, which reads as an
	// ordinary repo-relative path and is the silently-wrong value this refusal
	// exists to stop (ai/rules/principles.md). Measured by
	// TestGomuReportRefusesAPathOutsideTheCheckout, which caught it.
	rel := path
	if tree != "" {
		rel = strings.TrimPrefix(path, tree+"/")
	}
	if insideRepo(rel) {
		return rel, nil
	}
	var tb textbuf.Buffer
	return "", parseErr(tb.Str(where).Str(": the report names the file ").Quoted(path).
		Str(", which is not a path in this checkout. A break is applied to that path and, on ").
		Str("the interop carrier, written to it, so a report is never a trusted path source"))
}

// coverageSet is the lines a run executed at least once, per repo-relative file.
//
// Lines rather than byte offsets, because that is what a Go coverage profile
// and a gomu mutant each name, and converting either one would put a second
// coordinate system between them.
type coverageSet map[string][]lineRange

// lineRange is one executed block, first and last line inclusive.
type lineRange struct{ first, last int }

// coverProfileRE matches one block of a Go coverage profile:
// `<import path>/<file>:<line>.<col>,<line>.<col> <statements> <count>`.
var coverProfileRE = regexp.MustCompile(
	`^(.+):(\d+)\.\d+,(\d+)\.\d+ \d+ (\d+)$`)

// parseCoverProfile answers the lines a `go test -coverprofile` run executed.
//
// A block with count 0 is dropped rather than stored: the question this set
// answers is "did the tagged unit reach this line", and a compiled-but-unrun
// block is exactly the unreached producer R-10 is about.
func parseCoverProfile(module, text string) (coverageSet, error) {
	out := coverageSet{}
	var tb textbuf.Buffer
	prefix := tb.Str(module).Byte('/').String()
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		match := coverProfileRE.FindStringSubmatch(line)
		if match == nil {
			var errBuf textbuf.Buffer
			return nil, parseErr(errBuf.Str("cannot read a coverage profile line: ").Quoted(line))
		}
		count, _ := strconv.Atoi(match[4])
		if count == 0 {
			continue
		}
		first, _ := strconv.Atoi(match[2])
		last, _ := strconv.Atoi(match[3])
		rel := strings.TrimPrefix(match[1], prefix)
		out[rel] = append(out[rel], lineRange{first: first, last: last})
	}
	return out, nil
}

// covers answers whether the run executed the named line.
func (c coverageSet) covers(file string, line int) bool {
	for _, block := range c[file] {
		if line >= block.first && line <= block.last {
			return true
		}
	}
	return false
}

// claimSymbolRE matches an identifier a tag's prose can name.
//
// Three characters at least, because `is`, `an` and `to` name nothing, and a
// two-letter Go identifier is rare enough that missing one costs a ranking
// position rather than a candidate.
var claimSymbolRE = regexp.MustCompile(`\b[A-Za-z_][A-Za-z0-9_]{2,}\b`)

// claimSymbols answers the identifiers a tag's prose names.
//
// The prose is the half no gate can read, and this is the one thing a machine
// CAN take from it: which names it mentions. It decides the ORDER candidates
// are offered in and nothing else, so a prose that names nothing costs a
// ranking rather than a refusal (R-7).
func claimSymbols(claim string) []string {
	seen := map[string]bool{}
	var out []string
	for _, word := range claimSymbolRE.FindAllString(claim, -1) {
		lowered := strings.ToLower(word)
		if seen[lowered] {
			continue
		}
		seen[lowered] = true
		out = append(out, word)
	}
	sort.Strings(out)
	return out
}

// touchedSymbols answers which of the claim's symbols this break's text names.
//
// The haystack is the producer's SYMBOL and the break's own text, never the
// producer's path: a directory name carries no signal and plenty of noise --
// `component` contains `one`, so a claim naming "exactly one" would score every
// mutant in the tree. Matching is by substring rather than by word, because a Go
// name is compound: `extractPeerRoleConfigs` is what a claim naming `role` means.
func touchedSymbols(mutant reportMutant, producer string, symbols []string) []string {
	_, symbol, _ := strings.Cut(producer, "::")
	var tb textbuf.Buffer
	haystack := strings.ToLower(tb.Str(symbol).Byte(' ').Str(mutant.Original).
		Byte(' ').Str(mutant.Mutated).String())
	var out []string
	for _, symbol := range symbols {
		if strings.Contains(haystack, strings.ToLower(symbol)) {
			out = append(out, symbol)
		}
	}
	return out
}

// candidatesFor answers the breaks that could prove one tag, best first.
//
// COVERED is the filter and it is not a preference: a mutant the tagged unit
// never executes cannot redden it, so proposing one would send an author to
// spend a `go test` run on a break that can only stay green.
//
// The rank is the count of claim symbols the break's text touches. It is a
// ranking rather than a filter, because a claim's prose can be true and name no
// identifier at all, and the gate never judges whether a break is a GOOD break
// (R-7). A reviewer reads the stored break; this only decides what is offered
// first.
func candidatesFor(tag Tag, unitKey, claim string, mutants []reportMutant,
	covered coverageSet, producers map[string]string) []DiscriminationCandidate {
	symbols := claimSymbols(claim)
	out := make([]DiscriminationCandidate, 0, len(mutants))
	for index := range mutants {
		mutant := mutants[index]
		if !covered.covers(mutant.FilePath, mutant.Line) {
			continue
		}
		producer, held := producers[mutant.at()]
		if !held {
			continue
		}
		out = append(out, DiscriminationCandidate{
			RID: tag.RID, Polarity: tag.Polarity, Unit: unitKey,
			Producer: producer, Mutant: mutant.selector(), Break: mutant.text(),
			Symbols: touchedSymbols(mutant, producer, symbols),
		})
	}
	sortCandidates(out)
	return out
}

// sortCandidates puts the breaks that engage the claim first, and orders the
// rest so that two runs over one report answer in one order.
func sortCandidates(candidates []DiscriminationCandidate) {
	sort.SliceStable(candidates, func(left, right int) bool {
		if len(candidates[left].Symbols) != len(candidates[right].Symbols) {
			return len(candidates[left].Symbols) > len(candidates[right].Symbols)
		}
		if candidates[left].Producer != candidates[right].Producer {
			return candidates[left].Producer < candidates[right].Producer
		}
		return candidates[left].Mutant < candidates[right].Mutant
	})
}

// producerIndex answers, for each mutant, the fingerprint key of the function
// it sits inside.
//
// A mutant carries a file and a line and gomu fills no enclosing function (A-2,
// measured BROKEN over three packages), so the function is resolved here, from
// the same spans UnitAt resolves a tagged unit with. A mutant outside every
// function -- in a var block, say -- gets no entry and is not proposed: a
// record names the FUNCTION a break was applied to, so a break with no function
// around it has no key to be recorded under.
func producerIndex(reader *sourceReader, index *scopeIndex,
	mutants []reportMutant) map[string]string {
	out := make(map[string]string, len(mutants))
	for position := range mutants {
		mutant := mutants[position]
		if !mutant.Killed {
			continue
		}
		content := reader.read(mutant.FilePath)
		if content == nil {
			continue
		}
		name := functionAtLine(index, *content, mutant.Line)
		if name == "" {
			continue
		}
		var tb textbuf.Buffer
		out[mutant.at()] = tb.Str(mutant.FilePath).Str("::").Str(name).String()
	}
	return out
}

// functionAtLine answers the name of the function one line sits in.
func functionAtLine(index *scopeIndex, content string, line int) string {
	offset := lineOffset(content, line)
	if offset < 0 {
		return ""
	}
	for _, found := range index.of(content) {
		if offset >= found.begin && offset < found.end {
			return funcNameIn(content[found.begin:found.end])
		}
	}
	return ""
}

// proposeBreaks answers the candidate breaks for every unproven unit tag.
//
// Only the unit kind, and that is a fact about gomu rather than a choice:
// it runs unit tests only, so no mutant it generates can ever redden a `.ci` or
// an interop scenario (docs/contributing/testing.md). Those two take the revert
// route, which needs no report and no proposal.
//
// One `go test -coverprofile` run per tagged unit, because "the code this unit
// covers" has no cheaper answer: a package-wide profile would credit a unit
// with every line its siblings reach, which is the attribution error A-2's
// broken field would have caused.
func proposeBreaks(tree, report string, unproven []Tag) ([]DiscriminationCandidate, error) {
	mutants, err := loadGomuReport(tree, report)
	if err != nil {
		return nil, err
	}
	toolchain, err := gotoolchain.New(tree)
	if err != nil {
		return nil, err
	}
	module, err := modulePath(tree)
	if err != nil {
		return nil, err
	}
	table, err := carriers(tree)
	if err != nil {
		return nil, err
	}
	reader := newSourceReader(tree)
	index := newScopeIndex()
	mutants = textuallyApplicable(reader, mutants)
	producers := producerIndex(reader, index, mutants)
	packages := mutantPackages(mutants)

	out := []DiscriminationCandidate{}
	for _, tag := range unproven {
		carrier, carried := CarrierFor(tag.File, table)
		if !carried || carrier.Kind != kindUnit {
			continue
		}
		unitKey, err := tagUnitKey(reader, index, tag)
		if err != nil {
			return nil, err
		}
		_, symbol, err := fingerprintKey(unitKey, report)
		if err != nil {
			return nil, err
		}
		if symbol == "" {
			continue
		}
		covered, err := unitCoverage(tree, toolchain, module, tag, symbol, packages)
		if err != nil {
			return nil, err
		}
		out = append(out, candidatesFor(tag, unitKey, tag.Claim,
			mutants, covered, producers)...)
	}
	return out, nil
}

// mutantPackages answers the -coverpkg patterns one report needs instrumented.
func mutantPackages(mutants []reportMutant) string {
	dirs := map[string]bool{}
	for position := range mutants {
		dirs[filepath.ToSlash(filepath.Dir(mutants[position].FilePath))] = true
	}
	patterns := make([]string, 0, len(dirs))
	for _, dir := range sortedSet(dirs) {
		var tb textbuf.Buffer
		patterns = append(patterns, tb.Str("./").Str(dir).String())
	}
	return strings.Join(patterns, ",")
}

// unitCoverage answers the lines one tagged unit executes.
//
// A failing run is an error rather than an empty set: "this unit reaches
// nothing" and "this unit could not be run" would then be the same answer, and
// the first one would silently propose no candidate at all
// (ai/rules/principles.md).
func unitCoverage(tree string, toolchain gotoolchain.Toolchain, module string, tag Tag,
	symbol, packages string) (coverageSet, error) {
	scratch, err := observationScratch(tree)
	if err != nil {
		return nil, err
	}
	profile := filepath.Join(scratch, "propose-cover.out")
	var tb textbuf.Buffer
	argv := toolchain.GoTest(gotoolchain.TestOptions{},
		"-run", tb.Byte('^').Str(symbol).Byte('$').String(), "-count=1",
		"-coverprofile", profile, "-coverpkg", packages,
		tb.Reset().Str("./").Str(filepath.ToSlash(filepath.Dir(tag.File))).String())

	ctx, cancel := context.WithTimeout(context.Background(), unitRunDeadline)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...) //nolint:gosec // the unit and its package come from ScanTree and UnitAt
	cmd.Dir = tree
	cmd.Env = toolchain.Environment(gotoolchain.EnvOptions{Procs: true})
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if runErr := cmd.Run(); runErr != nil {
		return nil, parseErr(tb.Reset().Str("cannot measure what ").Str(tag.File).Str("::").
			Str(symbol).Str(" covers, so which mutants it could redden is unknown:\n").
			Str(excerpt(out.String())))
	}
	raw, err := os.ReadFile(profile) // #nosec G304 -- a path this run just wrote under the session scratch
	if err != nil {
		return nil, parseErr(tb.Reset().Str("cannot read the coverage profile: ").Err(err))
	}
	return parseCoverProfile(module, string(raw))
}

// textuallyApplicable keeps the mutants that compile and that a text
// substitution can apply exactly.
//
// gomu mutates an AST and reports a line, a column and the two texts. The column
// is the enclosing STATEMENT's, not the expression's -- a branch_condition at
// `if roleMap, hasRole := m["role"].(map[string]any); hasRole {` is reported at
// the `if` -- so the only thing that locates the expression is its own text. A
// mutant whose original appears twice on its line is dropped rather than guessed
// at: substituting the wrong occurrence produces a build failure, and a build
// failure is a red for the wrong reason.
//
// A mutant gomu did not KILL is dropped for the same reason. NOT_VIABLE does not
// compile -- 100 of the 1,042 in one package -- and SURVIVED means the whole
// package's tests did not notice it, so one of those tests will not notice it
// either. Proposing either spends a `go test` run to reach a refusal.
func textuallyApplicable(reader *sourceReader, mutants []reportMutant) []reportMutant {
	out := make([]reportMutant, 0, len(mutants))
	for position := range mutants {
		mutant := mutants[position]
		content := reader.read(mutant.FilePath)
		if content == nil {
			continue
		}
		lines := strings.Split(*content, "\n")
		if mutant.Line < 1 || mutant.Line > len(lines) {
			continue
		}
		if strings.Count(lines[mutant.Line-1], mutant.Original) != 1 {
			continue
		}
		out = append(out, mutant)
	}
	return out
}
