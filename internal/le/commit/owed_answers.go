// Design: docs/architecture/testing/verify-freshness-scope.md -- the answers that close a person-act obligation, derived from git on every read
// Related: owed.go -- the derivation that asks these producers whether an owed item is answered
// Related: review.go -- closedSpecStem and judgeReviewCoverage, the readings the answers re-run
package commit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/le/rfc"
	specpath "github.com/ze-software/ze/internal/le/spec/path"
)

// answerNotApplicable re-runs the producer that CREATED the obligation, over
// the commit that owes it, and answers nil when that producer finds nothing:
// a review is not owed by a commit that closes no spec, and an owner approval
// is not owed by a commit that changes no RFC-tagged unit. The owed kind
// decides which producer runs.
func answerNotApplicable(root string, kind owedItemKind, facts commitFacts) error {
	switch kind {
	case owedItemReview:
		stem, err := closedSpecStem(facts.Paths, facts.Removed, commitSides(root, facts))
		if err != nil {
			return err
		}
		if stem != "" {
			return errors.New("commit " + facts.SHA + " closes spec " + stem +
				", so a review was owed for it")
		}
		return nil
	case owedItemRFCApproval:
		changed, err := changedTaggedUnits(root, facts)
		if err != nil {
			return err
		}
		if len(changed) != 0 {
			return errors.New("commit " + facts.SHA + " changes RFC-tagged test(s) " +
				strings.Join(changed, ", "))
		}
		return nil
	case owedItemLinuxLimit:
		return errors.New("a Linux-limit obligation requires the owner's attestation")
	case owedItemUnspecified:
		return errors.New("BUG: an owed item with no kind has no not-applicable reading")
	}
	return errors.New("BUG: owed item kind " + strconv.Itoa(int(kind)) + " has no not-applicable reading")
}

// changedTaggedUnits answers the RFC-tagged units the commit changed, as
// `<package>.<Test>` (RFCChange.unit), by the same reading the owner-approval
// gate itself performs: a unit that carried a tag at the PARENT and whose text
// moved at the commit.
func changedTaggedUnits(root string, facts commitFacts) ([]string, error) {
	names := make([]string, 0)
	if facts.Parent == "" {
		// A root commit changed no unit that existed before it.
		return names, nil
	}
	for _, path := range append(append([]string{}, facts.Paths...), facts.Removed...) {
		if !rfc.IsTagCarrier(path) {
			continue
		}
		oldText, _, problem := committedText(root, facts.Parent, path)
		if problem != "" {
			return nil, errors.New(problem)
		}
		if oldText == "" || !rfc.HasRequirementTag(oldText) {
			continue
		}
		newText, _, problem := committedText(root, facts.SHA, path)
		if problem != "" {
			return nil, errors.New(problem)
		}
		for _, unit := range rfc.ChangedUnits(path, oldText, newText) {
			change := RFCChange(unit)
			names = append(names, change.unit())
		}
	}
	slices.Sort(names)
	return names, nil
}

// answerClosed answers whether the spec the commit CLOSES recorded a
// review that ran, read out of the spec's own bytes at the closure commit's
// parent. Closure removes the spec, so the parent is where its text lives.
func answerClosed(root string, facts commitFacts) error {
	spec, err := removedSpecPath(root, facts)
	if err != nil {
		return err
	}
	if spec == "" {
		return errors.New("commit " + facts.SHA + " removes no spec, so it closes nothing")
	}
	text, present, problem := committedText(root, facts.Parent, spec)
	if problem != "" {
		return errors.New(problem)
	}
	if !present {
		return errors.New(spec + " is not in " + facts.SHA + "^, so its Review Gate cannot be read")
	}
	// A spec that carried no implementation owed no review, and the Status at
	// the parent is what says so. Reading the ABSENCE of a gate would say the
	// same thing about a spec whose review is simply missing (AC-20, AC-7b).
	status := specStatus(text)
	if status == "skeleton" || status == "design" {
		return nil
	}
	if status == "" {
		return errors.New(spec + " has no Status row at " + facts.SHA + "^")
	}
	return reviewGateRecorded(spec, text)
}

// answerReviewed judges a review artifact against the bytes of the commit it
// reviewed, and falls back to the Review Gate the closure committed when that
// artifact is gone. artifact is a checkout-relative path, or "" when no
// artifact is known.
//
// The fallback is the durable half. Artifacts live under tmp/review/, which is
// untracked and is emptied, so a review recorded months ago has no artifact to
// read. The Review Gate is committed prose the closure preserves for ever.
func answerReviewed(root, artifact string, facts commitFacts) error {
	if artifact == "" {
		return answerClosed(root, facts)
	}
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(artifact))) //nolint:gosec // the path is a review artifact under the checkout root
	if err != nil {
		return answerClosed(root, facts)
	}
	// The discharge judges the population the closure gate judged: the
	// closure commit's paths and those of every commit attributed to its spec
	// (specWorkPopulation), read at the closure commit's tree (R-8).
	stem, err := closedSpecStem(facts.Paths, facts.Removed, commitSides(root, facts))
	if err != nil {
		return err
	}
	paths := facts.Paths
	if stem != "" {
		population, err := specWorkPopulation(root, stem, facts.SHA)
		if err != nil {
			return err
		}
		paths = unique(slices.Concat(facts.Paths, population.Paths))
	}
	result := ReviewResult{Spec: stem, Artifact: artifact}
	judgeReviewCoverage(&result, string(content), paths, func(path string) string {
		text, present, problem := committedText(root, facts.SHA, path)
		if problem != "" {
			return "UNREADABLE"
		}
		if !present {
			return "DELETED"
		}
		sum := sha256.Sum256([]byte(text))
		return hex.EncodeToString(sum[:])
	})
	if !result.Clean {
		return errors.New(strings.Join(result.Problems, "; "))
	}
	return nil
}

// commitSides judges a commit already made: its parent holds the removed
// spec's text and the commit holds the added spec's, so the rename verdict is
// read from the commit's own blobs and never from today's working tree.
func commitSides(root string, facts commitFacts) specSides {
	return specSides{
		before: func(path string) (string, bool, string) { return committedText(root, facts.Parent, path) },
		after:  func(path string) (string, bool, string) { return committedText(root, facts.SHA, path) },
	}
}

// removedSpecPath answers the path of the spec the commit closes, or "".
func removedSpecPath(root string, facts commitFacts) (string, error) {
	stem, err := closedSpecStem(facts.Paths, facts.Removed, commitSides(root, facts))
	if err != nil || stem == "" {
		return "", err
	}
	for _, path := range facts.Removed {
		if _, isSpec := specpath.HistoricalBucket(path); isSpec && specpath.Stem(filepath.Base(path)) == stem {
			return path, nil
		}
	}
	return "", errors.New("closure stem " + stem + " names no removed spec path")
}

// specStatus answers a spec's Status cell, lowercased, from its metadata table.
func specStatus(spec string) string {
	for line := range strings.SplitSeq(spec, "\n") {
		cells := strings.Split(line, "|")
		if len(cells) != 4 {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(cells[1]), "Status") {
			return strings.ToLower(strings.TrimSpace(cells[2]))
		}
	}
	return ""
}

// reviewGateRecorded answers nil when a spec's `## Review Gate` section records
// a review that RAN: an artifact reference that NAMES A FILE, and a rounds
// count.
//
// The predicate reads the section's ROWS and never one field label. The verdict
// row is spelled `review_gate.py check` in some closed specs and `review check`
// in others, the two do not sort by date, and a verifier matching one spelling
// reads the other as absent. What every recorded gate carries whatever it calls
// its verdict row is a filled artifact and a count of the rounds it ran.
//
// "Filled" is a file, not a non-empty cell. Every artifact cell measured across
// the closed specs names a path under `tmp/review/`, while `n/a` and `-` say
// the review produced nothing and are what an author writes where no review
// ran: a cell test that only refuses the empty string reads those as evidence.
func reviewGateRecorded(spec, text string) error {
	rows, found := reviewGateRows(text)
	if !found {
		return errors.New(spec + " carries no ## Review Gate section")
	}
	artifact := ""
	rounds := 0
	for _, row := range rows {
		field, value := row[0], row[1]
		lower := strings.ToLower(value)
		if strings.Contains(lower, "not recorded") || strings.Contains(lower, "not run") {
			return errors.New(spec + " records its Review Gate as " + strconv.Quote(value))
		}
		if isTemplatePlaceholder(value) {
			continue
		}
		if strings.Contains(strings.ToLower(field), "artifact") && namesAFile(value) {
			artifact = value
		}
		if strings.Contains(strings.ToLower(field), "round") {
			rounds = leadingCount(value)
		}
	}
	if artifact == "" {
		return errors.New(spec + " has a Review Gate whose artifact row names no file")
	}
	if rounds < 1 {
		return errors.New(spec + " has a Review Gate with no rounds count")
	}
	return nil
}

// reviewGateRows answers the field and value of every table row under the
// `## Review Gate` heading, and whether the heading was found at all.
func reviewGateRows(text string) ([][2]string, bool) {
	rows := make([][2]string, 0)
	inside := false
	for line := range strings.SplitSeq(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			if strings.EqualFold(trimmed, "## Review Gate") {
				inside = true
				continue
			}
			if inside && strings.HasPrefix(trimmed, "## ") {
				break
			}
			continue
		}
		if !inside {
			continue
		}
		cells := strings.Split(line, "|")
		if len(cells) != 4 {
			continue
		}
		field := strings.Trim(strings.TrimSpace(cells[1]), "`")
		value := strings.TrimSpace(cells[2])
		if field == "" || strings.HasPrefix(field, "---") {
			continue
		}
		rows = append(rows, [2]string{field, value})
	}
	return rows, inside
}

// namesAFile answers whether a cell names a path with a file name on the end.
//
// A file name is what separates a reference from a note: `tmp/review/x.md`
// names one, and `n/a`, `-` and `none` do not. The test is a token holding a
// separator, with a dot AFTER the last separator, so `n/a` fails on the dot
// that a file name carries and a bare word fails on the separator.
func namesAFile(value string) bool {
	for field := range strings.FieldsSeq(value) {
		name := strings.Trim(field, "`'\"(),;")
		_, base, found := strings.CutLast(name, "/")
		if !found {
			continue
		}
		if strings.Contains(base, ".") {
			return true
		}
	}
	return false
}

// isTemplatePlaceholder answers whether a cell still holds the template's own
// square-bracketed instruction rather than a filled value.
func isTemplatePlaceholder(value string) bool {
	return value == "" || (strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]"))
}

// leadingCount answers the first whole number in a cell, or zero.
func leadingCount(value string) int {
	digits := 0
	for digits < len(value) && value[digits] >= '0' && value[digits] <= '9' {
		digits++
	}
	if digits == 0 {
		return 0
	}
	count, err := strconv.Atoi(value[:digits])
	if err != nil {
		return 0
	}
	return count
}

// commitFacts is what an answer re-derives about one commit.
type commitFacts struct {
	SHA     string
	Parent  string
	Subject string
	Paths   []string
	Removed []string
}

// commitCache holds the facts of each commit the answers ask about. Its bound
// is the number of DISTINCT commits named, which for the derivation is the
// commits a push would publish.
//
// load reads a whole set of commits in two git calls, whatever the size of the
// set. Reading each commit with its own forks cost the ledger reader 2.7 to
// 3.0 s on a loaded machine for 60 commits (2026-09-26), most of the
// session-start hook's budget, and the derivation answers on that hook's path.
//
// Not safe for concurrent use.
type commitCache struct {
	facts   map[string]commitFacts
	refused map[string]string
}

// commitLoadMax bounds the commits one git call names, so the argument list
// stays far below the platform limit however many commits are asked about.
const commitLoadMax = 256

func newCommitCache() *commitCache {
	return &commitCache{
		facts:   make(map[string]commitFacts),
		refused: make(map[string]string),
	}
}

// read answers the facts of one revision, loading it when no earlier load did.
func (c *commitCache) read(root, revision string) (commitFacts, error) {
	if !c.known(revision) {
		c.load(root, []string{revision})
	}
	if problem, refused := c.refused[revision]; refused {
		return commitFacts{}, errors.New(problem)
	}
	return c.facts[revision], nil
}

// known answers whether a load already decided this revision either way.
func (c *commitCache) known(revision string) bool {
	if _, loaded := c.facts[revision]; loaded {
		return true
	}
	_, refused := c.refused[revision]
	return refused
}

// load decides every revision it is given: each one ends in facts or in
// refused, so read never answers a zero commitFacts for a revision it did not
// resolve. A failed git call refuses every revision it was reading, which
// leaves the items that name them owed.
func (c *commitCache) load(root string, revisions []string) {
	pending := make([]string, 0, len(revisions))
	for _, revision := range revisions {
		if c.known(revision) {
			continue
		}
		if slices.Contains(pending, revision) {
			continue
		}
		if revision == "" {
			c.refused[revision] = "name the commit: commit <sha>"
			continue
		}
		pending = append(pending, revision)
	}
	for first := 0; first < len(pending); first += commitLoadMax {
		c.loadChunk(root, pending[first:min(first+commitLoadMax, len(pending))])
	}
}

// loadChunk reads at most commitLoadMax revisions: one cat-file resolves them
// and one show reads each commit's subject, parent and file population.
// Rename detection is OFF: a spec moved between release buckets or renamed is
// a removal and an addition, which is the pair relocatedSpecs and
// renamedSpecs read.
func (c *commitCache) loadChunk(root string, revisions []string) {
	shas, err := resolveCommits(root, revisions)
	if err != nil {
		c.refuseAll(revisions, err.Error())
		return
	}
	resolved := make([]string, 0, len(shas))
	for index, revision := range revisions {
		if shas[index] == "" {
			c.refused[revision] = "commit " + strconv.Quote(revision) +
				" does not resolve in this checkout"
			continue
		}
		if !slices.Contains(resolved, shas[index]) {
			resolved = append(resolved, shas[index])
		}
	}
	if len(resolved) == 0 {
		return
	}
	shown, err := gitOutput(root, append(append(commitShow(), "--name-status",
		"--format=%x1e%H%x00%P%x00%s%x00"), resolved...)...)
	if err != nil {
		c.refuseAll(revisions, err.Error())
		return
	}
	facts := parseCommitFacts(shown)
	for index, revision := range revisions {
		sha := shas[index]
		if sha == "" {
			continue
		}
		one, shownHere := facts[sha]
		if !shownHere {
			c.refused[revision] = "git show " + sha + " printed no subject"
			continue
		}
		c.facts[revision] = one
	}
}

// gitNoColor turns color off on every git call whose output a parser reads.
const gitNoColor = "--no-color"

// gitNoExtDiff keeps a configured external diff driver out of every diff the
// derivation parses, so the output is git's own.
const gitNoExtDiff = "--no-ext-diff"

// commitShow is the start of the batched `git show` call. It pins every output
// setting a user's git config can flip, because parseCommitFacts reads the
// output's shape: core.quotePath quotes a non-ASCII path, color adds escapes,
// and log.showSignature adds lines.
func commitShow() []string {
	return []string{"-c", "core.quotePath=false", "show", "--no-renames", gitNoColor,
		gitNoExtDiff, "--no-show-signature"}
}

// refuseAll refuses every revision of a chunk a git call could not read.
func (c *commitCache) refuseAll(revisions []string, problem string) {
	for _, revision := range revisions {
		c.refused[revision] = problem
	}
}

// resolveCommits answers the full SHA of each revision as a commit, in order,
// and an empty string for a revision that names no commit. `cat-file
// --batch-check` answers one line for each line it reads, which is what lets
// a single call carry every revision.
func resolveCommits(root string, revisions []string) ([]string, error) {
	var input bytes.Buffer
	for _, revision := range revisions {
		input.WriteString(revision)
		input.WriteString("^{commit}\n")
	}
	command := exec.CommandContext(context.Background(), "git", "cat-file", "--batch-check") // #nosec G204 -- fixed Git query; the revisions go through stdin.
	command.Dir = root
	command.Stdin = &input
	var stdout, complaint bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &complaint
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("git cat-file --batch-check: %w: %s", err,
			strings.TrimSpace(complaint.String()))
	}
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != len(revisions) {
		return nil, fmt.Errorf("git cat-file --batch-check answered %d lines for %d revisions",
			len(lines), len(revisions))
	}
	shas := make([]string, len(revisions))
	for index, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 3 && fields[1] == "commit" {
			shas[index] = fields[0]
		}
	}
	return shas, nil
}

// parseCommitFacts reads the output of loadChunk's name-status show: each
// commit opens with a record separator, then its SHA, its parents and its
// subject, each closed by NUL, then one name-status line per file. The parent
// is the FIRST parent, and a root commit has none, which is not a failure:
// nothing existed before it, so every "did this change X" question answers no.
func parseCommitFacts(shown string) map[string]commitFacts {
	facts := make(map[string]commitFacts)
	for record := range strings.SplitSeq(shown, "\x1e") {
		fields := strings.SplitN(record, "\x00", 4)
		if len(fields) != 4 {
			continue
		}
		one := commitFacts{SHA: strings.TrimSpace(fields[0]), Subject: strings.TrimSpace(fields[2])}
		if parents := strings.Fields(fields[1]); len(parents) != 0 {
			one.Parent = parents[0]
		}
		for line := range strings.SplitSeq(fields[3], "\n") {
			status, path, tabbed := strings.Cut(strings.TrimSpace(line), "\t")
			if !tabbed {
				continue
			}
			if path == "" {
				continue
			}
			if strings.HasPrefix(status, "D") {
				one.Removed = append(one.Removed, path)
				continue
			}
			one.Paths = append(one.Paths, path)
		}
		facts[one.SHA] = one
	}
	return facts
}

// gitOutput runs one read-only git query and answers its stdout.
func gitOutput(root string, args ...string) (string, error) {
	command := exec.CommandContext(context.Background(), "git", args...) // #nosec G204 -- fixed Git queries; every value is an argv operand.
	command.Dir = root
	var stdout, complaint bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &complaint
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err,
			strings.TrimSpace(complaint.String()))
	}
	return stdout.String(), nil
}
