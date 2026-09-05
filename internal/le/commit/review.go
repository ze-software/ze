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
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/le/journal"
	specsession "github.com/ze-software/ze/internal/le/spec/session"
	"github.com/ze-software/ze/internal/le/spec/specpath"
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
// and carried in another in the same commit. A DIFFERENT name is a closure.
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
		if bucket, ok := specpath.Bucket(path); ok {
			removedFrom[filepath.Base(path)] = bucket
		}
	}

	moved := make(map[string]bool)
	for _, path := range paths {
		bucket, ok := specpath.Bucket(path)
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
// refuse a second one: one commit closes one spec.
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
// Journal paths are still READ, for their shape alone. A malformed row refuses
// the commit here, where the file is in hand.
func closureStem(root string, paths, removed []string) (string, error) {
	journalPaths := make([]string, 0)
	for _, path := range paths {
		if strings.HasPrefix(path, "plan/journal/") &&
			strings.HasSuffix(path, ".md") && filepath.Base(path) != "README.md" {
			journalPaths = append(journalPaths, path)
		}
	}
	if len(journalPaths) != 0 {
		_, malformed, err := journal.AddedSpecEvidence(root, journalPaths)
		if err != nil {
			return "", fmt.Errorf("read added journal evidence: %w", err)
		}
		if len(malformed) != 0 {
			return "", fmt.Errorf("journal has malformed row(s): %s", strings.Join(malformed, ", "))
		}
	}

	moved := relocatedSpecs(paths, removed)
	stems := make(map[string]bool)
	for _, path := range removed {
		base := filepath.Base(path)
		if specpath.IsSpec(path) && !moved[base] {
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

// oneStem answers the single closure, and refuses a commit that closes two.
// One commit closes one spec, so a second removal is the batch this gate exists
// to stop.
func oneStem(stems map[string]bool) (string, error) {
	if len(stems) > 1 {
		names := make([]string, 0, len(stems))
		for stem := range stems {
			names = append(names, stem)
		}
		sort.Strings(names)
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
// was the wrong one: `le spec session review record` writes under the harness
// session, this read asked for the eight-hex commit namespace, and no closure
// could satisfy the gate (specsession.ReviewArtifactPath).
func CheckReview(root, stem string, paths []string) ReviewResult {
	artifact, err := specsession.ReviewArtifactPath(root, stem)
	if err != nil {
		return ReviewResult{Spec: stem, Problems: []string{"resolve review artifact path: " + err.Error()}}
	}
	result := ReviewResult{Spec: stem, Artifact: artifact}
	for _, path := range unique(paths) {
		if isReviewCode(path) {
			result.CodeFiles = append(result.CodeFiles, path)
		}
	}
	sort.Strings(result.CodeFiles)
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(artifact))) //nolint:gosec // the path is this session's commit artifact or a tracked file under the checkout root
	if err != nil {
		result.Problems = []string{"no independent-review artifact at " + artifact}
		return result
	}
	header := reviewHeaderPattern.FindStringSubmatch(string(content))
	if len(header) != 2 {
		result.Problems = []string{"review artifact has no valid ze-review header: " + artifact}
		return result
	}
	result.Verdict = strings.ToLower(header[1])
	hashes := make(map[string]string)
	for line := range strings.SplitSeq(string(content), "\n") {
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
		if recorded != reviewHash(filepath.Join(root, filepath.FromSlash(path))) {
			result.Stale = append(result.Stale, path)
		}
	}
	if result.Verdict != "clean" {
		result.Problems = append(result.Problems, "review artifact verdict is "+result.Verdict+", not clean")
	}
	if len(result.Unreviewed) != 0 {
		result.Problems = append(result.Problems, fmt.Sprintf("%d code file(s) were not covered by the review: %s", len(result.Unreviewed), strings.Join(result.Unreviewed, ", ")))
	}
	if len(result.Stale) != 0 {
		result.Problems = append(result.Problems, fmt.Sprintf("%d reviewed file(s) changed after the review: %s", len(result.Stale), strings.Join(result.Stale, ", ")))
	}
	result.Clean = len(result.Problems) == 0
	return result
}

func reviewHash(path string) string {
	content, err := os.ReadFile(path) //nolint:gosec // the path is this session's commit artifact or a tracked file under the checkout root
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
	words := []string{"./le", "commit", actionReviewCheck, "spec", stem}
	for _, path := range paths {
		if isReviewCode(path) {
			words = append(words, "file", path)
		}
	}
	return quotePaths(words)
}
