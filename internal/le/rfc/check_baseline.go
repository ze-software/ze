// Design: docs/architecture/core-design.md -- the rfc area, as one command
// Related: check.go -- the centralized check driver that consumes these HEAD snapshots
//
// check_baseline.go reads the committed side of every monotonic comparison.
// A baseline that cannot be read accuses nobody. Optional maps preserve the
// cases where an empty committed set and an unreadable commit have opposite
// meanings to a current-minus-baseline comparison.
package rfc

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

type baselineExtraction struct {
	excluded     int
	signedOff    string
	resignReason string
}

func baselineParseError(message string) error { return &ParseError{msg: message} }

// The three committed revisions this file reads.
//
// headRevision is the commit under test. priorRevision is the one before it,
// and it is what the discrimination obligation is judged against: a tag the tip
// commit ADDED owes its proof in that commit, and `./le verify worktree` checks
// the commit out detached, where the tip IS what is being judged. Judging
// against the tip instead would bill every session for the uncommitted tags of
// every other session sharing this checkout, which is the failure R-8 names.
// backlogRevision is the pushed branch, which the backlog is MEASURED against
// and never billed.
const (
	headRevision    = "HEAD"
	priorRevision   = "HEAD^"
	backlogRevision = "origin/main"
)

// revisionExists reports whether git resolves revision to a commit in tree.
//
// Named rather than derived from a failed read, because three ordinary states
// answer "no" here and none of them is an error: a tree with no git at all, a
// root commit that has no parent, and a clone whose remote is not called origin.
// A baseline nobody could read accuses nobody, so each caller judges nothing.
func revisionExists(tree, revision string) bool {
	var tb textbuf.Buffer
	_, ok := gitOutput(tree, "rev-parse", "--verify", "--quiet", tb.Str(revision).Str("^{commit}").String())
	return ok
}

func gitOutput(tree string, args ...string) ([]byte, bool) {
	cmd := exec.Command("git", args...) //nolint:gosec,noctx // this developer tool queries the fixture checkout it was given
	cmd.Dir = tree
	out, err := cmd.Output()
	return out, err == nil
}

// gitCatBlobs answers the text each path has at revision, and false when git
// could not be read.
//
// The second return is the whole point: an empty map is what that revision
// holding none of these paths looks like, and it is also what a failed
// `git cat-file` looks like. A caller that cannot tell them apart reads a broken reader as a clean
// baseline, and every ratchet built on that baseline then judges a corpus it
// never saw (ai/rules/principles.md).
func gitCatBlobs(tree, revision string, paths []string) (map[string]string, bool) {
	if len(paths) == 0 {
		return map[string]string{}, true
	}
	var input bytes.Buffer
	for _, rel := range paths {
		input.WriteString(revision)
		input.WriteByte(':')
		input.WriteString(rel)
		input.WriteByte('\n')
	}
	cmd := exec.Command("git", "cat-file", "--batch") //nolint:gosec,noctx // this developer tool queries the fixture checkout it was given
	cmd.Dir = tree
	cmd.Stdin = &input
	data, err := cmd.Output()
	if err != nil {
		return nil, false
	}
	out := map[string]string{}
	position := 0
	for _, rel := range paths {
		newline := bytes.IndexByte(data[position:], '\n')
		if newline < 0 {
			return nil, false
		}
		newline += position
		header := strings.Fields(string(data[position:newline]))
		position = newline + 1
		if len(header) == 2 && (header[1] == "missing" || header[1] == "ambiguous") {
			continue
		}
		if len(header) != 3 {
			return nil, false
		}
		size, err := strconv.Atoi(header[2])
		if err != nil || position+size >= len(data) {
			return nil, false
		}
		body := data[position : position+size]
		position += size + 1
		if header[1] == "blob" {
			out[rel] = strings.ToValidUTF8(string(body), "�")
		}
	}
	return out, true
}

func gitTreePaths(tree, revision, dir, suffix string) ([]string, bool) {
	raw, ok := gitOutput(tree, "ls-tree", "-r", "-z", "--name-only", revision, dir)
	if !ok {
		return nil, false
	}
	var paths []string
	for rel := range strings.SplitSeq(string(raw), "\x00") {
		if strings.HasSuffix(rel, suffix) && !strings.Contains(rel, "\n") {
			paths = append(paths, rel)
		}
	}
	return paths, true
}

// baselineMetas answers what every summary at HEAD^ declares about itself.
//
// HEAD^ and not HEAD, for the reason baselineLevels gives: `./le verify
// worktree` checks the commit under test out detached, where the tree equals
// HEAD, so an un-enrolment, a new enrolment or a dropped public row that commit
// made was already in a HEAD baseline and nothing fired at the gate. A tree with
// no HEAD^ answers false.
//
// This is what the four gated ratchets stand on. `check` runs
// checkRetiredRequirements, checkLevelRatchet, checkCoverageRatchet and
// checkEvidenceRatchet only where the current enrolled set INTERSECTS the
// baseline one, so a baseline that cannot be read disarms all four -- which is
// right for a checkout git cannot answer about, and would have been a hole at
// the one commit that moved enrolment into the summaries.
//
// A summary at HEAD^ that does not parse is SKIPPED rather than failing the
// whole baseline. HEAD^ is not this change's to fix, and one unreadable summary
// must not take the other 190 out of every ratchet's population.
func baselineMetas(tree string) (map[string]Meta, bool) {
	if !revisionExists(tree, priorRevision) {
		return nil, false
	}
	paths, ok := gitTreePaths(tree, priorRevision, summaryRel, ".md")
	if !ok {
		return nil, false
	}
	blobs, known := gitCatBlobs(tree, priorRevision, paths)
	if !known {
		return nil, false
	}
	out := map[string]Meta{}
	for _, rel := range paths {
		text, held := blobs[rel]
		if !held {
			continue
		}
		stem := strings.TrimSuffix(filepath.Base(rel), ".md")
		meta, err := ParseMeta(text, stem, rel)
		if err != nil {
			meta, err = ParseMeta(retiredBaselineShapes(text), stem, rel)
		}
		if err != nil {
			continue
		}
		out[stem] = meta
	}
	if len(out) > 0 {
		return out, true
	}
	return baselineMetasBeforeMigration(tree)
}

// retiredBaselineShapes rewrites a HEAD^ summary written before any Meta
// migration into the shape ParseMeta reads: the aggregate migration of
// 2026-10-25 (retiredSectionRank), the declared use-case rows of 2026-10-26
// (retiredUseCaseRows), then the authored support cells the derived proof
// replaced (retiredSupportCells). It reads GIT HISTORY only, for the reason
// retiredSectionRank gives.
func retiredBaselineShapes(text string) string {
	return retiredSupportCells(retiredUseCaseRows(retiredSectionRank(text)))
}

// retiredSupportCells drops every `Support status`, `Support coverage` and
// `Support remaining` row from a HEAD^ summary written before the public
// ledger derived its proof. The ratchets this baseline feeds read enrolment
// and the public row, never those cells, so dropping them changes no baseline
// fact; without it every summary at the commit that deleted the cells fails to
// parse and is skipped, and the ratchets stop running over exactly that
// commit. A summary in the tree that still writes one is refused by
// ParseMeta (refuseNearMiss).
func retiredSupportCells(text string) string {
	lines := strings.SplitAfter(text, "\n")
	kept := lines[:0]
	for _, line := range lines {
		if !isRetiredSupportRow(line) {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "")
}

// isRetiredSupportRow reports whether line is a Meta table row whose label is
// one of retiredSupportLabels.
func isRetiredSupportRow(line string) bool {
	cell, isRow := strings.CutPrefix(line, "| ")
	if !isRow {
		return false
	}
	label, _, closed := strings.Cut(cell, " |")
	return closed && retiredSupportLabels[label]
}

// retiredUseCaseRows adds `-` use-case rows to a HEAD^ summary written before
// every summary declared them, below its `Support` row. The ratchets this
// baseline feeds read enrolment and the public row, never use-case
// membership, so `-` changes no baseline fact; without it the commit that
// added the rows is the one commit whose baseline is unreadable. A summary in
// the tree with no such rows is refused by ParseMeta.
func retiredUseCaseRows(text string) string {
	if strings.Contains(text, "\n| "+metaUseCases+" |") {
		return text
	}
	location := retiredSupportRowRE.FindStringIndex(text)
	if location == nil {
		return text
	}
	return text[:location[1]] + "\n| " + metaUseCases + " | - |\n| " + metaOptionalUseCases + " | - |" +
		text[location[1]:]
}

// retiredSupportRowRE matches the first `Support` row, which every summary
// carried in both retired shapes.
var retiredSupportRowRE = regexp.MustCompile(`(?m)^\| Support \| [^\n]* \|$`)

// retiredSupportRE matches the `| Support | <section> <rank> |` cell every
// summary carried until the aggregate migration of 2026-10-25, and
// retiredNoRowRE the `| Support | - |` cell, which then named no aggregate.
var (
	retiredSupportRE = regexp.MustCompile(`(?m)^\| Support \| ([a-z-]+) \d+ \|$`)
	retiredNoRowRE   = regexp.MustCompile(`(?m)^\| Support \| - \|$`)
)

// retiredShapeAggregate is the aggregate a baseline read in the retired shape
// states for a summary that had no row: that shape named none, and the
// ratchets this baseline feeds read enrolment and the row, never the aggregate.
const retiredShapeAggregate = "retired-section-rank"

// retiredSectionRank rewrites a HEAD^ summary written before the aggregate
// migration into the shape ParseMeta reads.
//
// It reads GIT HISTORY only, for the reason baselineMetasBeforeMigration
// gives: without it the migration commit is the one commit whose baseline is
// unreadable, and the four gated ratchets and the public-row ratchet stop
// running over exactly the change that moved every Support cell. The working
// tree has one shape and one reader (ai/rules/no-layering.md); a summary in the tree that still
// writes a rank is refused by ParseMeta.
func retiredSectionRank(text string) string {
	if strings.Contains(text, "\n| "+metaAggregate+" |") {
		return text
	}
	text = retiredSupportRE.ReplaceAllString(text, "| Support | $1 |")
	return retiredNoRowRE.ReplaceAllString(text, "| Support | - |\n| "+metaAggregate+" | "+retiredShapeAggregate+" |")
}

// baselineMetasBeforeMigration reads an enrolment HEAD^ states in the shape it
// used before 2026-09-01: rfc/enrolled.txt and rfc/not-enrolled.txt.
//
// It reads GIT HISTORY, never the working tree, and it is reached only when no
// summary at HEAD^ declares an enrolment at all. That is not a fallback inside
// the live path -- the tree has exactly one shape and one reader
// (ai/rules/no-layering.md) -- it is the ability to compare against a commit
// written before the shape changed. Without it the migration commit is the one
// commit whose baseline is unreadable, and four ratchets stop running over
// exactly the change they exist to judge.
func baselineMetasBeforeMigration(tree string) (map[string]Meta, bool) {
	blobs, known := gitCatBlobs(tree, priorRevision, []string{enrolledRel, notEnrolledRel})
	if !known {
		return nil, false
	}
	out := map[string]Meta{}
	for line := range strings.SplitSeq(blobs[enrolledRel], "\n") {
		if stem, reason, ok := legacyLedgerRow(line); ok {
			// The retired shape predates the implementation fact entirely, and
			// this baseline answers one question: was the stem GATED at HEAD.
			// Reading an absent fact as "not Ze's Go" would answer that
			// question no for every stem the old files held, which retires
			// every ratchet in silence.
			out[stem] = Meta{Enrolment: enrolmentEnrolled, EnrolmentReason: reason,
				Implementation: implementationZe, ImplementationReason: legacyImplementationReason}
		}
	}
	for line := range strings.SplitSeq(blobs[notEnrolledRel], "\n") {
		stem, rest, ok := legacyLedgerRow(line)
		if !ok {
			continue
		}
		kind, reason := cutFirstWord(rest)
		out[stem] = Meta{Enrolment: kind, EnrolmentReason: reason}
	}
	return out, len(out) > 0
}

// legacyImplementationReason is what a baseline read from the retired ledger
// files says about a fact those files never carried.
const legacyImplementationReason = "read from the retired ledger files, which carried no implementation row"

// legacyLedgerRow reads one row of the two retired ledger files: the first
// whitespace run separates the stem from the rest.
func legacyLedgerRow(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}
	stem, rest := cutFirstWord(line)
	return stem, rest, true
}

// cutFirstWord splits off the first whitespace-delimited token.
func cutFirstWord(line string) (string, string) {
	cut := strings.IndexAny(line, " \t")
	if cut < 0 {
		return line, ""
	}
	return line[:cut], strings.TrimSpace(line[cut:])
}

func baselineSummaryStems(tree string) (map[string]bool, bool) {
	if !revisionExists(tree, priorRevision) {
		return nil, false
	}
	paths, ok := gitTreePaths(tree, priorRevision, summaryRel, ".md")
	if !ok {
		return nil, false
	}
	out := map[string]bool{}
	for _, rel := range paths {
		out[strings.TrimSuffix(filepath.Base(rel), ".md")] = true
	}
	return out, true
}

// baselineLevels answers the level of every requirement id at HEAD^, the
// commit before the one under test, and its evidence fingerprint
// (evidenceFingerprint), which the exclusion evidence ratchet reads to tell a
// row the commit added or changed from one it left alone. One parse of HEAD^
// serves both. It keeps unreadable summaries separate from
// known-empty history: a listed path without a readable checklist cannot
// establish that an ID is new.
//
// HEAD^ and not HEAD, because `./le verify worktree` checks the commit under
// test out detached, where the tree equals HEAD: an id or a demotion that
// commit made is already in a HEAD baseline, so the id allocation and level
// ratchets built on it saw nothing at the one gate that runs. Against HEAD^ they
// judge the tip commit and whatever the tree adds over it. A tree with no HEAD^
// answers false, and both ratchets then judge nothing.
func baselineLevels(tree string) (map[string]string, map[string]string, map[string]bool, bool) {
	if !revisionExists(tree, priorRevision) {
		return nil, nil, nil, false
	}
	paths, ok := gitTreePaths(tree, priorRevision, summaryRel, ".md")
	if !ok {
		return nil, nil, nil, false
	}
	blobs, ok := gitCatBlobs(tree, priorRevision, paths)
	if !ok {
		return nil, nil, nil, false
	}
	out := map[string]string{}
	evidence := map[string]string{}
	unreadable := map[string]bool{}
	for _, rel := range paths {
		stem := strings.TrimSuffix(filepath.Base(rel), ".md")
		text, held := blobs[rel]
		if !held {
			unreadable[stem] = true
			continue
		}
		requirements, err := ParseSummaryText(text, stem, rel)
		if err != nil {
			unreadable[stem] = true
			continue
		}
		for i := range requirements {
			req := &requirements[i]
			out[req.RID] = req.Level
			evidence[req.RID] = evidenceFingerprint(req)
		}
	}
	return out, evidence, unreadable, true
}

func baselineIDs(levels map[string]string) map[string]bool {
	out := make(map[string]bool, len(levels))
	for rid := range levels {
		out[rid] = true
	}
	return out
}

func functionalSuitesFromGo(src, where string) ([]string, error) {
	var tb textbuf.Buffer
	file, err := parser.ParseFile(token.NewFileSet(), where, src, 0)
	if err != nil {
		return nil, baselineParseError(strings.NewReplacer("\n", " ").Replace(err.Error()))
	}
	constants := map[string]string{}
	for _, decl := range file.Decls {
		gen, held := decl.(*ast.GenDecl)
		if !held || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			value, held := spec.(*ast.ValueSpec)
			if !held {
				continue
			}
			for i, name := range value.Names {
				if i >= len(value.Values) {
					continue
				}
				literal, held := value.Values[i].(*ast.BasicLit)
				if !held || literal.Kind != token.STRING {
					continue
				}
				decoded, decodeErr := strconv.Unquote(literal.Value)
				if decodeErr == nil {
					constants[name.Name] = decoded
				}
			}
		}
	}
	for _, decl := range file.Decls {
		gen, held := decl.(*ast.GenDecl)
		if !held || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			value, held := spec.(*ast.ValueSpec)
			if !held {
				continue
			}
			for i, name := range value.Names {
				if name.Name != "Gating" || i >= len(value.Values) {
					continue
				}
				list, held := value.Values[i].(*ast.CompositeLit)
				if !held {
					return nil, baselineParseError(tb.Reset().Str(where).Str(": Gating is not a composite literal").String())
				}
				out := make([]string, 0, len(list.Elts))
				for _, element := range list.Elts {
					switch one := element.(type) {
					case *ast.Ident:
						name, known := constants[one.Name]
						if !known {
							return nil, baselineParseError(tb.Reset().Str(where).Str(": Gating references an unknown suite constant ").Str(one.Name).String())
						}
						out = append(out, name)
					case *ast.BasicLit:
						name, decodeErr := strconv.Unquote(one.Value)
						if decodeErr != nil {
							return nil, baselineParseError(tb.Reset().Str(where).Str(": Gating contains an invalid string").String())
						}
						out = append(out, name)
					default:
						return nil, baselineParseError(tb.Reset().Str(where).Str(": Gating contains a non-name expression").String())
					}
				}
				if len(out) == 0 {
					return nil, baselineParseError(tb.Reset().Str(where).Str(": Gating is empty").String())
				}
				return out, nil
			}
		}
	}
	return nil, baselineParseError(tb.Reset().Str(where).Str(": no Gating declaration found").String())
}

func headCarriers(tree string) ([]Carrier, error) {
	suites := FunctionalSuites()
	if raw, ok := gitOutput(tree, "show", "HEAD:internal/le/test/functional/suites.go"); ok {
		if found, err := functionalSuitesFromGo(string(raw), "HEAD:internal/le/test/functional/suites.go"); err == nil {
			suites = found
		}
	}
	scheduled, err := scheduledWorkflowActions(tree)
	if err != nil {
		return nil, err
	}
	paths, ok := gitTreePaths(tree, headRevision, workflowsRel, ".yml")
	if ok {
		yamlPaths, yamlOK := gitTreePaths(tree, headRevision, workflowsRel, ".yaml")
		if yamlOK {
			paths = append(paths, yamlPaths...)
		}
		sources := map[string]string{}
		// Best effort by construction: the schedule falls back to the working
		// tree's own answer two lines down when nothing parses.
		blobs, _ := gitCatBlobs(tree, headRevision, paths)
		for rel, text := range blobs {
			sources[filepath.Base(rel)] = text
		}
		if parsed := scheduledActionsFrom(sources); len(parsed) > 0 {
			scheduled = parsed
		}
	}
	carriers := carriersFor(suites, scheduled)
	return append(carriers, legacyInteropCarriers(scheduled)...), nil
}

func scanTagsTolerant(blob, rel string, carriers []Carrier) []Tag {
	carrier, held := CarrierFor(rel, carriers)
	if !held {
		return nil
	}
	found, err := readerFor(carrier.Reader)(blob, rel)
	if err == nil {
		return found
	}
	if carrier.Reader != "go" {
		return nil
	}
	var out []Tag
	for lineIndex, line := range strings.Split(blob, "\n") {
		match := goTagRE.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		tag, parseErr := parseTagRest(match[1], tagWhere(rel, lineIndex+1))
		if parseErr != nil {
			continue
		}
		tag.File = rel
		tag.Line = lineIndex + 1
		out = append(out, tag)
	}
	return out
}

// committedTags is the tag corpus at the revisions this check compares.
//
// One value rather than six returns, because the three revisions are read
// together and every one of them is optional in its own way. A caller holding
// them separately can pair the tip's tags with the wrong baseline's covers, and
// there is nothing in the types to catch it.
type committedTags struct {
	// Tags and Blobs are the tip commit's, read by the polarity and the
	// evidence ratchets and by the changed-unit measurement.
	Tags  []Tag
	Blobs map[string]string
	// Head is the tip commit's cover set. It carries the tags themselves,
	// because a violation names the file and the line the author must open.
	Head map[Cover][]Tag
	// Prior is the cover set of the commit BEFORE the tip, and PriorKnown is
	// false where git could not answer. The discrimination obligation is billed
	// against it.
	Prior      map[Cover]bool
	PriorKnown bool
	// Backlog is the cover set of the pushed branch and BacklogRef names it,
	// empty when that ref does not resolve. The unproven backlog is MEASURED
	// against it and never billed.
	Backlog    map[Cover]bool
	BacklogRef string
}

// readCommittedTags reads the tag corpus at the tip commit, at the commit
// before it, and at the pushed branch.
//
// ONE carrier table serves all three. A table read per revision would answer
// that every tag in a file is new when only that file's carrier declaration
// moved, which is a diff of the scanner rather than of the corpus.
//
// A revision git cannot resolve leaves its own field empty and says so, and
// each consumer judges nothing there. That is the rule every baseline in this
// file follows, and here it is what keeps the ratchet silent on a fresh clone,
// on a root commit, and on a checkout whose remote is not named origin.
func readCommittedTags(tree string, index *scopeIndex) committedTags {
	carriers, err := headCarriers(tree)
	if err != nil {
		return committedTags{}
	}
	tags, blobs, known := baselineTaggedAt(tree, headRevision, carriers)
	if !known {
		return committedTags{}
	}
	out := committedTags{Tags: tags, Blobs: blobs, Head: coversOfTags(index, tags, blobs)}
	if revisionExists(tree, priorRevision) {
		out.Prior, out.PriorKnown = coversAt(tree, priorRevision, carriers, index, exactRenamesSince)
	}
	if revisionExists(tree, backlogRevision) {
		if found, ok := coversAt(tree, backlogRevision, carriers, index, exactRenamesSince); ok {
			out.Backlog, out.BacklogRef = found, backlogRevision
		}
	}
	return out
}

// coversOfTags answers what every tag of one revision PROVES, keyed
// unit-precisely.
//
// The unit is resolved from that revision's own BLOB rather than from the
// working tree, because a tag that moved between functions is a new cover even
// though its file and its text are unchanged.
func coversOfTags(index *scopeIndex, tags []Tag, blobs map[string]string) map[Cover][]Tag {
	out := make(map[Cover][]Tag, len(tags))
	for _, tag := range tags {
		key := Cover{RID: tag.RID, Polarity: tag.Polarity,
			Unit: unitKeyAt(index, tag.File, blobs[tag.File], tag.Line)}
		out[key] = append(out[key], tag)
	}
	return out
}

// coversAt answers the cover keys one revision holds, and false when git could
// not read it.
//
// A baseline is compared against, never reported from, so it keeps the keys and
// drops the tags. It goes through coversOfTags rather than keying tags itself:
// two ways to mint a cover key is two answers to the question the whole
// comparison rests on.
//
// A cover in a file HEAD holds under another name with the same bytes is keyed
// by its HEAD path (exactRenamesSince), so a byte-pure rename moves no cover.
// This is the one place a baseline cover is minted, so the HEAD^ obligation and
// the origin/main backlog both follow it. A rename map git cannot read leaves
// the whole baseline unknown, because "no renames" would bill every moved cover.
// renamesOf is exactRenamesSince in every caller but the test that drives an
// unreadable map over a readable baseline.
func coversAt(tree, revision string, carriers []Carrier, index *scopeIndex, renamesOf renameReader) (map[Cover]bool, bool) {
	tags, blobs, known := baselineTaggedAt(tree, revision, carriers)
	if !known {
		return nil, false
	}
	renames, known := renamesOf(tree, revision, carriers)
	if !known {
		return nil, false
	}
	covers := coversOfTags(index, tags, blobs)
	out := make(map[Cover]bool, len(covers))
	for key := range covers {
		key.Unit = followRename(key.Unit, renames)
		out[key] = true
	}
	return out, true
}

// renameReader answers the byte-pure renames between revision and HEAD, and
// false when they cannot be read.
type renameReader func(tree, revision string, carriers []Carrier) (map[string]string, bool)

// exactRenamesSince answers every carrier file git reports renamed between
// revision and HEAD with an unchanged blob id, old path to new path, and false
// when git could not answer.
//
// Only an identical blob is followed, compared by id rather than trusted from a
// similarity score: byte purity is what lets every other gate read the rename as
// no change, and a rename that carries an edit is reviewed as an edit. The false
// return is the guard that keeps an unreadable map from reading as "no renames".
func exactRenamesSince(tree, revision string, carriers []Carrier) (map[string]string, bool) {
	args := append([]string{"diff", "--raw", "-z", "--no-abbrev", "-M100%", "--diff-filter=R",
		revision, headRevision, "--"}, testRoots[:]...)
	raw, ok := gitOutput(tree, args...)
	if !ok {
		return nil, false
	}
	return parseExactRenames(raw, carriers)
}

// parseExactRenames reads `git diff --raw -z` rename records, and false when one
// is truncated or malformed.
//
// A record is `:<mode> <mode> <old-sha> <new-sha> R<score>`, then the old and the
// new path, each NUL terminated. A pair is kept only when both shas are equal and
// the carrier table holds both paths.
func parseExactRenames(raw []byte, carriers []Carrier) (map[string]string, bool) {
	out := map[string]string{}
	fields := strings.Split(string(raw), "\x00")
	// Every record ends in NUL, so the field after the last one is empty and the
	// records fill exactly the fields before it, three each.
	records := fields[:len(fields)-1]
	if len(records)%3 != 0 {
		return nil, false
	}
	for position := 0; position < len(records); position += 3 {
		header := strings.Fields(strings.TrimPrefix(records[position], ":"))
		if len(header) != 5 {
			return nil, false
		}
		if !strings.HasPrefix(header[4], "R") {
			return nil, false
		}
		source, target := records[position+1], records[position+2]
		if source == "" {
			return nil, false
		}
		if target == "" {
			return nil, false
		}
		if header[2] != header[3] {
			continue
		}
		if _, held := CarrierFor(source, carriers); !held {
			continue
		}
		if _, held := CarrierFor(target, carriers); !held {
			continue
		}
		out[source] = target
	}
	return out, true
}

// followRename answers a baseline cover's unit with its file part moved through
// renames, or unchanged when its file was not renamed.
//
// The unit is `<path>::<Func>` or the bare path of a file-scoped key; only the
// text before `::` is a path.
func followRename(unit string, renames map[string]string) string {
	path, function, scoped := strings.Cut(unit, "::")
	target, moved := renames[path]
	if !moved {
		return unit
	}
	if !scoped {
		return target
	}
	var tb textbuf.Buffer
	return tb.Str(target).Str("::").Str(function).String()
}

// baselineDiscrimination answers the covers the recorded proofs carried at
// HEAD, and false when git could not answer.
//
// A blob that does not decode is skipped rather than refused, which is what
// every baseline reader here does: the baseline says what WAS committed, and a
// malformed committed record is a violation the working-tree loader raises
// against the file itself.
func baselineDiscrimination(tree string) (map[Cover]bool, bool) {
	paths, ok := gitTreePaths(tree, headRevision, discriminationRel, jsonSuffix)
	if !ok {
		return nil, false
	}
	blobs, known := gitCatBlobs(tree, headRevision, paths)
	if !known {
		return nil, false
	}
	out := map[Cover]bool{}
	for rel, blob := range blobs {
		if _, held := evidenceAt(discriminationRel, rel); !held {
			continue
		}
		var records []DiscriminationRecord
		if json.Unmarshal([]byte(blob), &records) != nil {
			continue
		}
		for position := range records {
			out[records[position].Cover()] = true
		}
	}
	return out, true
}

// baselineTaggedAt answers every tag one revision holds and that revision's
// text for each file carrying one, and false when git could not read it.
//
// The blobs are kept rather than dropped because two callers need them: the tag
// list, which every polarity and evidence ratchet reads, and the unit each tag
// sat in, which only that revision's blob can answer.
//
// The carrier table is the caller's rather than this function's, so every
// revision in one comparison is scanned with one table.
func baselineTaggedAt(tree, revision string, carriers []Carrier) ([]Tag, map[string]string, bool) {
	var tb textbuf.Buffer
	prefix := tb.Str(revision).Byte(':').String()
	args := append([]string{"grep", "-l", "-z", "-F", tagMarker, revision, "--"}, testRoots[:]...)
	raw, ok := gitOutput(tree, args...)
	if !ok && len(raw) == 0 {
		return nil, nil, false
	}
	var paths []string
	for entry := range strings.SplitSeq(string(raw), "\x00") {
		if !strings.HasPrefix(entry, prefix) {
			continue
		}
		rel := strings.TrimPrefix(entry, prefix)
		carrier, held := CarrierFor(rel, carriers)
		if !held || carrier.Tier == tierUnrun || strings.Contains(rel, "\n") {
			continue
		}
		parts := strings.Split(rel, "/")
		skipped := false
		for _, part := range parts {
			if part == ".git" || part == "vendor" || part == "testdata" {
				skipped = true
			}
		}
		if !skipped {
			paths = append(paths, rel)
		}
	}
	var out []Tag
	blobs, known := gitCatBlobs(tree, revision, paths)
	if !known {
		return nil, nil, false
	}
	for rel, blob := range blobs {
		// The baseline is a proof corpus like Collect's, so a gap tag stays
		// out of it for the same reason (splitGapTags).
		proof, _ := splitGapTags(scanTagsTolerant(blob, rel, carriers))
		out = append(out, proof...)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out, blobs, true
}

func baselinePolarities(tags []Tag) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for _, tag := range tags {
		if out[tag.RID] == nil {
			out[tag.RID] = map[string]bool{}
		}
		out[tag.RID][tag.Polarity] = true
	}
	return out
}

func nonunitEvidence(tags []Tag, carriers []Carrier) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for _, tag := range tags {
		carrier, held := CarrierFor(tag.File, carriers)
		if !held || carrier.Kind == kindUnit {
			continue
		}
		if out[tag.RID] == nil {
			out[tag.RID] = map[string]bool{}
		}
		out[tag.RID][carrier.Label()] = true
	}
	return out
}

func baselineEvidence(tree string, tags []Tag) map[string]map[string]bool {
	carriers, err := headCarriers(tree)
	if err != nil {
		return map[string]map[string]bool{}
	}
	return nonunitEvidence(tags, carriers)
}

// baselineAudits answers every audit verdict at HEAD^, the commit before the one
// under test, and false when that history cannot be read.
//
// HEAD^ and not HEAD, because `./le verify worktree` checks the commit under
// test out detached, where the tree equals HEAD: a verdict that commit deleted
// or upgraded is already gone from a HEAD baseline, so the findings and verdict
// ratchets built on it saw nothing at the one gate that runs. A tree with no
// HEAD^ answers false, and both ratchets then judge nothing.
func baselineAudits(tree string) (map[string]map[string]map[string]any, bool) {
	if !revisionExists(tree, priorRevision) {
		return nil, false
	}
	paths, ok := gitTreePaths(tree, priorRevision, auditRel, ".json")
	if !ok {
		return nil, false
	}
	blobs, known := gitCatBlobs(tree, priorRevision, paths)
	if !known {
		return nil, false
	}
	out := map[string]map[string]map[string]any{}
	for rel, blob := range blobs {
		file, held := evidenceAt(auditRel, rel)
		if !held {
			continue
		}
		var verdict map[string]any
		if json.Unmarshal([]byte(blob), &verdict) != nil {
			continue
		}
		name, _ := verdict["verdict"].(string)
		if _, known := auditVerdicts[name]; !known {
			continue
		}
		if out[file.Stem] == nil {
			out[file.Stem] = map[string]map[string]any{}
		}
		out[file.Stem][file.RID] = verdict
	}
	return out, true
}

// baselineExtractions answers every extraction sign-off at HEAD^, the commit
// before the one under test, and false when that history cannot be read.
//
// HEAD^ and not HEAD, for the reason baselineAudits gives: in the detached
// verify worktree the tree equals HEAD, so a sign-off the commit under test
// deleted, or an exclusion count it raised, was already in a HEAD baseline and
// checkExtractionRatchet saw nothing. A tree with no HEAD^ answers false, and
// the ratchet then judges nothing.
func baselineExtractions(tree string) (map[string]baselineExtraction, bool) {
	if !revisionExists(tree, priorRevision) {
		return nil, false
	}
	paths, ok := gitTreePaths(tree, priorRevision, extractionRel, ".json")
	if !ok {
		return nil, false
	}
	blobs, known := gitCatBlobs(tree, priorRevision, paths)
	if !known {
		return nil, false
	}
	out := map[string]baselineExtraction{}
	for rel, blob := range blobs {
		var document map[string]any
		if json.Unmarshal([]byte(blob), &document) != nil {
			continue
		}
		excluded := 0
		if sites, held := document[keySites].([]any); held {
			for _, value := range sites {
				site, held := value.(map[string]any)
				if held && site[keyDisposition] == DispositionExcluded {
					excluded++
				}
			}
		}
		signedOff, _ := document[keySignedOff].(string)
		resignReason, _ := document["resign-reason"].(string)
		stem := strings.TrimSuffix(filepath.Base(rel), ".json")
		out[stem] = baselineExtraction{excluded: excluded, signedOff: signedOff, resignReason: resignReason}
	}
	return out, true
}

// baselineRecordBlobs answers the HEAD text of every file the recorded proofs
// fingerprint, and false when git could not answer.
//
// A stale record is the author's to fix when the change that staled it was
// COMMITTED. Several sessions share this checkout, so judging the drift against
// the working tree reds `./le rfc check` for every one of them over an edit that
// is nobody else's, and a rule that reds the tree on unrelated work gets removed
// rather than obeyed (docs/contributing/rfc-conformance-gates.md). HEAD is what
// tells the two apart (owner decision, 2026-08-31).
//
// A path missing from the answer is a path HEAD does not hold, which the caller
// tells from an unreadable git by the second return.
func baselineRecordBlobs(tree string, records []DiscriminationRecord) (map[string]string, bool) {
	if _, ok := gitOutput(tree, "rev-parse", "--verify", "HEAD"); !ok {
		return nil, false
	}
	seen := map[string]bool{}
	var paths []string
	for position := range records {
		keys := []string{records[position].Unit, records[position].Producer}
		for _, paired := range records[position].Paired {
			keys = append(keys, paired.Producer)
		}
		for _, key := range keys {
			rel := keyFile(key)
			if rel == "" || seen[rel] {
				continue
			}
			seen[rel] = true
			paths = append(paths, rel)
		}
	}
	slices.Sort(paths)
	return gitCatBlobs(tree, headRevision, paths)
}
