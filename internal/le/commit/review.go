// Design: docs/features/ai-first.md -- hash-pinned independent review enforcement
package commit

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/spec"
	specjournal "github.com/ze-software/ze/internal/le/spec/journal"
	specpath "github.com/ze-software/ze/internal/le/spec/path"

	"github.com/ze-software/ze/internal/le/commit/hunks"
)

var (
	reviewHeaderPattern = regexp.MustCompile(`<!-- ze-review\s+[^>]*verdict=([^\s>]+)[^>]*-->`)
	reviewFilePattern   = regexp.MustCompile(`^\s{2}([0-9a-f]{64}|DELETED)\s{2}(.+)$`)
)

var reviewCodeSuffixes = []string{
	".go", ".ci", ".et", ".py", ".sh", ".yang", ".wb", ".mk", ".tmpl",
	".html", ".c", ".rs", ".s", ".rego", ".tac",
}

// ReviewResult is the hash-pinned verdict for one closure population.
type ReviewResult struct {
	Spec       string   `json:"spec"`
	Artifact   string   `json:"artifact"`
	Verdict    string   `json:"verdict,omitempty"`
	CodeFiles  []string `json:"code-files,omitempty"`
	Unreviewed []string `json:"unreviewed,omitempty"`
	Stale      []string `json:"stale,omitempty"`
	Problems   []string `json:"problems,omitempty"`
	Clean      bool     `json:"clean"`
}

// relocatedSpecs answers the spec file names this commit MOVES between release
// buckets rather than closes: the same name removed from one bucket directory
// and carried in another in the same commit. A DIFFERENT name is a closure
// unless renamedSpecs pairs it with the spec it was renamed to.
//
// The distinction is the whole of it. A closure retires a spec because its work
// is done, and one commit may close one, so the review gate below refuses a
// second. A relocation retires nothing: the spec is still open, still
// unimplemented, and has only been re-filed under the release that owes it.
// Counting a relocation as a closure made the gate refuse every batch of them,
// which is the shape the 2026-08-24 row in
// plan/journal/gate-fires-outside-its-population.md asks for: the question is
// not "does this commit remove a spec" but "does this commit CLOSE it".
//
// The rule read "removed from plan/ and added under plan/future/" until the
// release buckets replaced plan/future/. It now reads over every pair of
// buckets, in both directions, because a triage sweep re-files work upward as
// often as downward.
func relocatedSpecs(paths, removed []string) map[string]bool {
	removedFrom := make(map[string]string, len(removed))
	for _, path := range removed {
		if bucket, ok := specpath.HistoricalBucket(path); ok {
			removedFrom[filepath.Base(path)] = bucket
		}
	}

	moved := make(map[string]bool)
	for _, path := range paths {
		bucket, ok := specpath.HistoricalBucket(path)
		if !ok {
			continue
		}
		base := filepath.Base(path)
		if from, wasRemoved := removedFrom[base]; wasRemoved && from != bucket {
			moved[base] = true
		}
	}

	return moved
}

// closureStem answers the one spec this commit closes, or the empty string.
//
// A REMOVED spec file is the closure, and it is the only signal. Removals still
// refuse a second one: one commit closes one spec. A removal that relocatedSpecs
// or renamedSpecs explains is no closure.
//
// A journal row is evidence ABOUT a spec and never says the spec is closing.
// Reading a row as a closure fired outside its own population for a year, in
// two shapes with the same cost. A class file under plan/journal/ is shared, so
// it carries rows nobody in this commit wrote and the commit stages it whole:
// that charged a session with other people's specs, and the rows could land
// only by committing the file the charge refused. Six rows in
// plan/journal/gate-fires-outside-its-population.md record it and three
// overrides were spent on it. Commit 80c0133c1 filtered a FOREIGN session's row
// and named the second shape as still open, because attribution cannot reach
// it: a session's own row naming its own claimed spec reads as a closure in the
// middle of the spec, when CLAUDE.md requires a row for every defect walked
// into. That is the ordinary commit, not the rare one.
//
// Status cannot separate the two. There is no closed status: the vocabulary in
// docs/contributing/spec-workflow.md ends at in-progress and verification, and
// closure IS the removal. So commit A of a two-commit closure (code plus spec,
// ai/rules/planning.md "Spec Closure") is byte-for-byte an ordinary in-progress
// commit, and no content signal tells them apart.
//
// What this gives up is real and bounded: commit A now lands its code before
// the review artifact is read. Commit B cannot, so no spec closes without a
// clean independent review. Trading a gate that misfires on the common case for
// one that fires on the exact case is the whole of the change (Thomas,
// 2026-08-31).
//
// Journal paths are still READ, for their shape alone. Each one goes through
// specjournal.ValidateFile, the check the post-write hook runs, so a row the
// hook warns about refuses the commit here, where the file is in hand, however
// it was written: a malformed row, a bad Date or Spec cell, or a row added or
// rewritten against HEAD over specjournal.RowCharsMax. An open row the commit
// adds or changes against HEAD that no spec's owned journal rows table lists is
// refused too (specjournal.UnownedOpenRows), so every recorded defect has the
// spec that will fix it. That check judges against base: HEAD for a commit of
// its own, and HEAD under the earlier blocks for a block appended to a script
// (scriptBase), because the appended block lands after them.
func closureStem(root string, paths, removed []string, base specjournal.Base) (string, error) {
	return closureStemSelected(root, paths, removed, base, nil)
}

// closureStemSelected is closureStem over a commit whose hunks paths carry
// the selected blob: the journal shape check, the owned-row check and the spec
// sides all read a selected path through selection, so a row or an owner entry
// that only the working tree holds is judged absent.
func closureStemSelected(root string, paths, removed []string, base specjournal.Base, selection hunks.Selection) (string, error) {
	carried := func(path string) ([]byte, error) { return selection.Read(root, path) }
	for _, path := range paths {
		if !strings.HasPrefix(path, "plan/journal/") {
			continue
		}
		if !strings.HasSuffix(path, ".md") {
			continue
		}
		if filepath.Base(path) == "README.md" {
			continue
		}
		report, err := specjournal.ValidateCarried(root, path, carried)
		if err != nil {
			return "", fmt.Errorf("validate journal %s: %w", path, err)
		}
		if report.ExitCode() != 0 {
			return "", fmt.Errorf("journal is not commit-readable:\n%s", strings.TrimSuffix(report.Text(), "\n"))
		}
	}
	unowned, err := specjournal.UnownedOpenRows(root, paths, removed, base, carried)
	if err != nil {
		return "", fmt.Errorf("read journal row owners: %w", err)
	}
	if len(unowned) != 0 {
		return "", errors.New(strings.TrimSuffix(specjournal.UnownedText(unowned), "\n"))
	}

	return closedSpecStem(paths, removed, workingTreeSides(root, selection))
}

// appendBase answers the tree a prepared commit lands on, for the journal
// ownership check. A commit of its own lands on HEAD, the zero Base. An
// appended block lands after the script's earlier blocks, which have not run:
// commit B of a two-commit spec closure removes the spec while commit A, still
// unrun, carries the row's `fixed: ` marker or moves its entry to another spec.
func appendBase(root, session string, options *Options) (specjournal.Base, error) {
	if !options.Append {
		return specjournal.Base{}, nil
	}
	scriptPath, existing, err := targetScript(root, session, "", options)
	if err != nil {
		return specjournal.Base{}, err
	}
	return scriptBase(root, scriptPath, existing)
}

// privateIndexUpdate starts every line a block writes to its private index with
// update-index. renderPrivateIndex writes exactly two such lines, indexInfoOpen
// and privateRemovalPrefix, so any other line with this start is a script
// scriptBase cannot read.
const privateIndexUpdate = `GIT_INDEX_FILE="$_ze_index" git update-index`

// scriptBase reads back what the blocks of a prepared script commit under
// plan/: each snapshot entry's content, from the blob it names, and each path a
// block removes. A later block's entry for a path replaces an earlier one's, as
// the script's commits do. A script it cannot read is an error naming the
// script and the line, never a base that silently falls back to HEAD: a
// snapshot heredoc with no closing delimiter, a private-index update-index line
// in neither rendered form, or a snapshot line that is not an index entry.
func scriptBase(root, scriptPath, script string) (specjournal.Base, error) {
	base := specjournal.Base{Files: map[string]string{}, Removed: map[string]bool{}}
	opened := 0
	for index, line := range strings.Split(script, "\n") {
		number := index + 1
		if opened != 0 {
			if line == indexInfoDelimiter {
				opened = 0
				continue
			}
			if err := laySnapshotEntry(root, line, &base); err != nil {
				return specjournal.Base{}, scriptLineError(scriptPath, number, err)
			}
			continue
		}
		if line == indexInfoOpen {
			opened = number
			continue
		}
		raw, found := strings.CutPrefix(line, privateRemovalPrefix)
		if found {
			for _, path := range parseShellWords(raw) {
				delete(base.Files, path)
				base.Removed[path] = true
			}
			continue
		}
		if strings.HasPrefix(line, privateIndexUpdate) {
			return specjournal.Base{}, scriptLineError(scriptPath, number,
				errors.New("a private-index update-index line that is neither the snapshot heredoc nor the removal"))
		}
	}
	if opened != 0 {
		return specjournal.Base{}, scriptLineError(scriptPath, opened,
			errors.New("a snapshot heredoc with no closing "+indexInfoDelimiter+" line"))
	}
	return base, nil
}

// scriptLineError names the script and line scriptBase could not read, and the
// command that writes a script it can.
func scriptLineError(scriptPath string, number int, err error) error {
	return fmt.Errorf("cannot read what the earlier blocks of %s commit, line %d: %w; "+
		"prepare the whole commit again with ./le commit create replace", scriptPath, number, err)
}

// laySnapshotEntry lays one `git ls-files -s` line of an earlier block over the
// base, `<mode> <object> <stage>\t<path>`, when it names a plan/ markdown file:
// the only files that hold journal rows or owned tables. Every line is checked
// for that shape first, so a malformed line is refused whatever path it names.
func laySnapshotEntry(root, entry string, base *specjournal.Base) error {
	fields, path, found := strings.Cut(entry, "\t")
	if !found {
		return fmt.Errorf("a snapshot line with no path, %q", entry)
	}
	parts := strings.Fields(fields)
	if len(parts) != 3 {
		return fmt.Errorf("a snapshot line for %s that is not mode, object and stage, %q", path, entry)
	}
	if !strings.HasPrefix(path, "plan/") {
		return nil
	}
	if !strings.HasSuffix(path, ".md") {
		return nil
	}
	content, err := gitOutput(root, "cat-file", "blob", parts[1])
	if err != nil {
		return fmt.Errorf("read %s from the object the snapshot names: %w", path, err)
	}
	base.Files[path] = content
	delete(base.Removed, path)
	return nil
}

// closedSpecStem answers the one spec a file population closes, from the
// population and the two sides of the commit that sides reads.
//
// It is split out of closureStem because a discharge re-derives the closure of
// a commit made months ago, and closureStem's journal read judges the WORKING
// TREE: a shard deleted since would refuse the discharge for a reason that has
// nothing to do with whether the commit closed a spec. Both callers read this
// one statement of the rule, so neither holds a copy of it. Each supplies the
// sides it can see: a prepared commit reads HEAD and the working tree
// (workingTreeSides), a discharge reads the commit's parent and the commit
// itself (commitSides), and the rename judgement is the same over either pair.
func closedSpecStem(paths, removed []string, sides specSides) (string, error) {
	moved := relocatedSpecs(paths, removed)
	renamed, err := renamedSpecs(paths, removed, moved, sides)
	if err != nil {
		return "", err
	}
	stems := make(map[string]bool)
	for _, path := range removed {
		base := filepath.Base(path)
		if _, isSpec := specpath.HistoricalBucket(path); isSpec && !moved[base] && !renamed[path] {
			stems[specpath.Stem(base)] = true
		}
	}
	if len(stems) != 0 {
		return oneStem(stems)
	}
	// No spec removed: this commit closes nothing, so it owes no review
	// artifact, and any journal rows it carries ride along.
	return "", nil
}

// specSide answers a path's text on one side of a commit, whether that side
// holds the path, and the reason the read failed. present is what tells an
// absent path from an empty file.
type specSide func(path string) (text string, present bool, problem string)

// specSides is the pair of texts a rename is judged over: before is the commit
// the change starts from, and after is the content the change carries.
type specSides struct {
	before specSide
	after  specSide
}

// workingTreeSides judges a commit being PREPARED: HEAD is its parent, and the
// working tree holds the content the snapshot is about to take.
func workingTreeSides(root string, selection hunks.Selection) specSides {
	return specSides{
		before: func(path string) (string, bool, string) { return committedText(root, "HEAD", path) },
		after:  func(path string) (string, bool, string) { return workingText(root, path, selection) },
	}
}

// workingText answers a path's bytes in the working tree, in the shape of
// committedText.
func workingText(root, path string, selection hunks.Selection) (text string, present bool, problem string) {
	content, err := selection.Read(root, path)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, ""
	}
	if err != nil {
		return "", false, "read " + path + ": " + err.Error()
	}
	return string(content), true, ""
}

// renamedSpecs answers the removed spec paths this commit RENAMES rather than
// closes (Thomas, 2026-10-18: a rename is allowed and is not a closure).
//
// A removed spec is renamed only when the commit adds exactly one NEW spec, one
// its parent does not hold, whose text equals the removed spec's text at the
// parent with every occurrence of the old stem replaced by the new stem, and
// that new spec matches no other removal. The replacement covers the title and
// every self-reference, which is all a rename rewrites. Any other edit, a
// missing parent blob, an added spec that already existed, or a match that is
// not one to one leaves the removal a closure, because the failure this guards
// is a real closure passing as a rename: a closure deletes a spec and carries
// its work elsewhere, never its text verbatim under a new name.
//
// A relocation (moved) is judged before this and is never a rename candidate
// on either side.
func renamedSpecs(paths, removed []string, moved map[string]bool, sides specSides) (map[string]bool, error) {
	gone := make([]string, 0, len(removed))
	removedSet := make(map[string]bool, len(removed))
	for _, path := range removed {
		removedSet[path] = true
		if _, isSpec := specpath.HistoricalBucket(path); isSpec && !moved[filepath.Base(path)] {
			gone = append(gone, path)
		}
	}
	added, err := addedSpecTexts(paths, removedSet, moved, sides)
	if err != nil {
		return nil, err
	}
	renamed := make(map[string]bool)
	if len(gone) == 0 {
		return renamed, nil
	}
	if len(added) == 0 {
		return renamed, nil
	}

	matches := make(map[string][]string, len(gone))
	claims := make(map[string]int, len(added))
	for _, path := range gone {
		oldText, present, problem := sides.before(path)
		if problem != "" {
			return nil, errors.New(problem)
		}
		if !present {
			continue
		}
		oldStem := specpath.Stem(path)
		if oldStem == "" {
			continue
		}
		for newPath, newText := range added {
			if strings.ReplaceAll(oldText, oldStem, specpath.Stem(newPath)) == newText {
				matches[path] = append(matches[path], newPath)
				claims[newPath]++
			}
		}
	}

	for path, targets := range matches {
		if len(targets) == 1 && claims[targets[0]] == 1 {
			renamed[path] = true
		}
	}
	return renamed, nil
}

// addedSpecTexts answers the text of every spec the commit ADDS: named in the
// population, not removed, not a relocation's target, and absent from the
// parent.
func addedSpecTexts(paths []string, removed, moved map[string]bool, sides specSides) (map[string]string, error) {
	added := make(map[string]string)
	for _, path := range paths {
		if _, isSpec := specpath.HistoricalBucket(path); !isSpec {
			continue
		}
		if removed[path] {
			continue
		}
		if moved[filepath.Base(path)] {
			continue
		}
		if _, ok := added[path]; ok {
			continue
		}
		_, existed, problem := sides.before(path)
		if problem != "" {
			return nil, errors.New(problem)
		}
		if existed {
			continue
		}
		text, present, problem := sides.after(path)
		if problem != "" {
			return nil, errors.New(problem)
		}
		if present {
			added[path] = text
		}
	}
	return added, nil
}

// oneStem answers the single closure, and refuses a commit that closes two.
// One commit closes one spec, so a second removal is the batch this gate exists
// to stop.
func oneStem(stems map[string]bool) (string, error) {
	if len(stems) > 1 {
		names := make([]string, 0, len(stems))
		for stem := range stems {
			names = append(names, stem)
		}
		slices.Sort(names)
		return "", fmt.Errorf("commit names more than one spec closure: %s", strings.Join(names, ", "))
	}
	for stem := range stems {
		return stem, nil
	}
	return "", nil
}

// CheckReview verifies that the current session's clean review artifact covers
// every code-bearing path and still hashes to the reviewed bytes.
//
// The artifact's name comes from the package that WRITES it. Building it here
// gave this gate a second opinion about which session id names the file, and it
// was the wrong one: `le spec review record` writes under the harness
// session, this read asked for the eight-hex commit namespace, and no closure
// could satisfy the gate (spec.ReviewArtifactPath).
func CheckReview(root, stem string, paths []string) ReviewResult {
	return checkReviewSelected(root, stem, paths, nil)
}

// checkReviewSelected is CheckReview over a commit whose hunks paths carry the
// selected blob: the review must have pinned that content, which
// `le spec review record ... hunks <path> <patch>` does, not the working-tree
// file, which holds hunks the commit leaves behind. Like is compared with like:
// the pin and the gate both hash HEAD's blob with the patch applied.
func checkReviewSelected(root, stem string, paths []string, selection hunks.Selection) ReviewResult {
	artifact, err := spec.ReviewArtifactPath(root, stem)
	if err != nil {
		return ReviewResult{Spec: stem, Problems: []string{"resolve review artifact path: " + err.Error()}}
	}
	result := ReviewResult{Spec: stem, Artifact: artifact}
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(artifact))) //nolint:gosec // the path is this session's commit artifact or a tracked file under the checkout root
	if err != nil {
		result.Problems = []string{"no independent-review artifact at " + artifact}
		return result
	}
	var unreadable []string
	judgeReviewCoverage(&result, string(content), paths, func(path string) string {
		if !selection.Selected(path) {
			return reviewHash(filepath.Join(root, filepath.FromSlash(path)))
		}
		carried, err := selection.Read(root, path)
		if err != nil {
			unreadable = append(unreadable, err.Error())
		}
		return reviewDigest(carried, err)
	})
	result.Problems = append(result.Problems, unreadable...)
	var repin []string
	for _, path := range append(slices.Clone(result.Stale), result.Unreviewed...) {
		if selection.Selected(path) {
			repin = append(repin, path)
		}
	}
	if len(repin) != 0 {
		result.Problems = append(result.Problems, countedPaths(len(repin), " path(s) committed by hunks: ", repin)+
			"; the review pins the content this commit carries, not the working-tree file, so record it again with:\n  "+
			hunksReviewRecord(stem, string(content), result.CodeFiles, selection))
		result.Clean = false
	}
	return result
}

// reviewHeaderValues read the record keywords a review artifact holds, so the
// command that re-records it carries the same reviewers, rounds and reasons.
var (
	reviewRoundsPattern    = regexp.MustCompile(`<!-- ze-review\s+[^>]*rounds=(\d+)`)
	reviewReviewersPattern = regexp.MustCompile(`<!-- ze-review\s+[^>]*reviewers=([^\s>]+)`)
	reviewReasonPattern    = regexp.MustCompile(`(?m)^rounds-reason: (.+)$`)
	reviewOwnerPattern     = regexp.MustCompile(`(?m)^owner-authorised: (.+)$`) //nolint:misspell // owner-authorised is the artifact line spec.reviewArtifact.Document writes
)

// hunksReviewRecord answers the exact `./le spec review record` command that
// re-pins the review over every code file of this commit: each hunks path with
// the patch the commit names, every other code file whole, and the verdict,
// rounds, reviewers and reasons the artifact already holds. A record replaces
// the artifact, so the command names the whole population, never only the
// stale path. Every value is shell-quoted, so a path with a space pastes as one
// word.
func hunksReviewRecord(stem, artifactText string, codeFiles []string, selection hunks.Selection) string {
	var command textbuf.Buffer
	command.Str("./le spec review record spec ").Str(shellQuote(stem)).Str(" verdict clean rounds ")
	// An artifact `record` wrote always holds its rounds. One that does not
	// was written by hand, and no value here would be its truth, so the
	// command names the hole rather than inventing a count.
	if match := reviewRoundsPattern.FindStringSubmatch(artifactText); len(match) == 2 {
		command.Str(match[1])
	} else {
		command.Str("<the rounds the review ran: the artifact names none>")
	}
	if match := reviewReviewersPattern.FindStringSubmatch(artifactText); len(match) == 2 {
		command.Str(" reviewers ").Str(shellQuote(match[1]))
	}
	if match := reviewReasonPattern.FindStringSubmatch(artifactText); len(match) == 2 {
		command.Str(" rounds-reason ").Str(shellQuote(match[1]))
	}
	if match := reviewOwnerPattern.FindStringSubmatch(artifactText); len(match) == 2 {
		command.Str(" owner-authorised ").Str(shellQuote(match[1])) //nolint:misspell // owner-authorised is the review record keyword spec.parseReviewRecord reads
	}
	for _, path := range codeFiles {
		if selection.Selected(path) {
			command.Str(" hunks ").Str(shellQuote(path)).Byte(' ').Str(shellQuote(selection[path].Patch))
			continue
		}
		command.Str(" file ").Str(shellQuote(path))
	}
	return command.String()
}

// judgeReviewCoverage fills the verdict, the coverage and the problems of one
// review artifact over one file population. hashOf answers the digest each
// covered path MUST carry, which is what makes the judgement usable twice: the
// working tree answers for a prospective commit, and a commit's own blobs
// answer for a discharge months later.
func judgeReviewCoverage(result *ReviewResult, artifactText string, paths []string, hashOf func(string) string) {
	for _, path := range unique(paths) {
		if isReviewCode(path) {
			result.CodeFiles = append(result.CodeFiles, path)
		}
	}
	slices.Sort(result.CodeFiles)
	header := reviewHeaderPattern.FindStringSubmatch(artifactText)
	if len(header) != 2 {
		result.Problems = []string{"review artifact has no valid ze-review header: " + result.Artifact}
		return
	}
	result.Verdict = strings.ToLower(header[1])
	hashes := make(map[string]string)
	for line := range strings.SplitSeq(artifactText, "\n") {
		match := reviewFilePattern.FindStringSubmatch(line)
		if len(match) == 3 {
			hashes[match[2]] = match[1]
		}
	}
	for _, path := range result.CodeFiles {
		recorded, exists := hashes[path]
		if !exists {
			result.Unreviewed = append(result.Unreviewed, path)
			continue
		}
		if recorded != hashOf(path) {
			result.Stale = append(result.Stale, path)
		}
	}
	if result.Verdict != "clean" {
		result.Problems = append(result.Problems, "review artifact verdict is "+result.Verdict+", not clean")
	}
	if len(result.Unreviewed) != 0 {
		result.Problems = append(result.Problems, countedPaths(len(result.Unreviewed),
			" code file(s) were not covered by the review: ", result.Unreviewed))
	}
	if len(result.Stale) != 0 {
		result.Problems = append(result.Problems, countedPaths(len(result.Stale),
			" reviewed file(s) changed after the review: ", result.Stale))
	}
	result.Clean = len(result.Problems) == 0
}

// countedPaths renders "<n><what><path>, <path>" for a problem line.
func countedPaths(count int, what string, paths []string) string {
	var text textbuf.Buffer
	text.Int(int64(count)).Str(what)
	for index, path := range paths {
		if index != 0 {
			text.Str(", ")
		}
		text.Str(path)
	}
	return text.String()
}

func reviewHash(path string) string {
	content, err := os.ReadFile(path) //nolint:gosec // the path is this session's commit artifact or a tracked file under the checkout root
	return reviewDigest(content, err)
}

// reviewDigest answers the review pin of content read with err: DELETED for
// an absent file, UNREADABLE for any other failure, else its SHA-256.
func reviewDigest(content []byte, err error) string {
	if errors.Is(err, os.ErrNotExist) {
		return "DELETED"
	}
	if err != nil {
		return "UNREADABLE"
	}
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

func isReviewCode(path string) bool {
	if filepath.Base(path) == "Makefile" {
		return true
	}
	for _, suffix := range reviewCodeSuffixes {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}

func reviewCheckCommand(stem string, paths []string) string {
	// `./le`, not `le`. The generated script RUNS this line under `set -e`, and
	// the launcher is a file at the checkout root rather than a command on PATH,
	// so a bare name aborts the commit with "le: command not found".
	words := []string{"./le", area, actionReviewCheck, "spec", stem}
	for _, path := range paths {
		if isReviewCode(path) {
			words = append(words, "file", path)
		}
	}
	return quotePaths(words)
}
