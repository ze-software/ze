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

func closureStem(root string, paths, removed []string) (string, error) {
	stems := make(map[string]bool)
	for _, path := range removed {
		base := filepath.Base(path)
		if filepath.ToSlash(filepath.Dir(path)) == "plan" &&
			strings.HasPrefix(base, "spec-") && strings.HasSuffix(base, ".md") {
			stems[strings.TrimSuffix(strings.TrimPrefix(base, "spec-"), ".md")] = true
		}
	}

	journalPaths := make([]string, 0)
	for _, path := range paths {
		if strings.HasPrefix(path, "plan/journal/") &&
			strings.HasSuffix(path, ".md") && filepath.Base(path) != "README.md" {
			journalPaths = append(journalPaths, path)
		}
	}
	if len(journalPaths) != 0 {
		found, malformed, err := journal.AddedSpecEvidence(root, journalPaths)
		if err != nil {
			return "", fmt.Errorf("read added journal evidence: %w", err)
		}
		if len(malformed) != 0 {
			return "", fmt.Errorf("journal has malformed row(s): %s", strings.Join(malformed, ", "))
		}
		for _, stem := range found {
			stems[stem] = true
		}
	}
	if len(stems) == 0 {
		return "", nil
	}
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
func CheckReview(root, session, stem string, paths []string) ReviewResult {
	artifact := filepath.ToSlash(filepath.Join("tmp", "review", stem+"-"+session+".md"))
	result := ReviewResult{Spec: stem, Artifact: artifact}
	for _, path := range unique(paths) {
		if isReviewCode(path) {
			result.CodeFiles = append(result.CodeFiles, path)
		}
	}
	sort.Strings(result.CodeFiles)
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(artifact)))
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
	content, err := os.ReadFile(path)
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
	words := []string{"le", "commit", "review-check", "spec", stem}
	for _, path := range paths {
		if isReviewCode(path) {
			words = append(words, "file", path)
		}
	}
	return quotePaths(words)
}
