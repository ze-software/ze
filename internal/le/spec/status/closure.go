// Design: docs/architecture/core-design.md -- native spec lifecycle checks
// Related: closure_report.go -- structured closure findings

package specstatus

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ze-software/ze/internal/le/journal"
	"github.com/ze-software/ze/internal/le/spec/specpath"
)

const learnedDir = "plan/learned"

var (
	learnedFilePattern = regexp.MustCompile(`^(\d{3,})-([a-z0-9][a-z0-9-]*)\.md$`)
	learnedRefPattern  = regexp.MustCompile(`plan/learned/(\d{3,}-[a-z0-9][a-z0-9-]*)\.md`)
	uncheckedPattern   = regexp.MustCompile(`(?im)^\s*-\s*\[ \].*(before closing|still to run|re-run shows 0 BLOCKER)`)
)

type learnedFile struct {
	path   string
	slug   string
	tokens []string
}

// closureInventory reads every active spec and the closure evidence in the Git
// index and journal HEAD. The Git index is the producer's tracked-file contract
// for learned summaries. Journal evidence comes from journal's canonical HEAD
// population and row parser.
func closureInventory(root string) (ClosureReport, []string, error) {
	learned, err := learnedFiles(root)
	if err != nil {
		return nil, nil, err
	}
	tracked, err := trackedPaths(root)
	if err != nil {
		return nil, nil, err
	}
	journalEvidence, malformed, err := journal.HeadSpecEvidence(root)
	if err != nil {
		return nil, nil, err
	}

	relatives, err := specpath.All(root)
	if err != nil {
		return nil, nil, fmt.Errorf("match active specs: %w", err)
	}
	paths := make([]string, 0, len(relatives))
	for _, relative := range relatives {
		if filepath.Base(relative) != specTemplateFile {
			paths = append(paths, filepath.Join(root, filepath.FromSlash(relative)))
		}
	}
	stems := make(map[string]bool, len(paths))
	for _, path := range paths {
		stems[specpath.Stem(filepath.Base(path))] = true
	}

	report := make(ClosureReport, 0, len(stems))
	for _, path := range paths {
		one, err := inspectClosureSpec(root, path, learned, tracked, journalEvidence, stems)
		if err != nil {
			return nil, malformed, err
		}
		report = append(report, one)
	}
	return report, malformed, nil
}

// CheckClosure checks one spec spelling. An absent spec is a clean answer.
func CheckClosure(root, spec string) (ClosureReport, []string, error) {
	path, err := resolveSpecPath(root, spec)
	if errors.Is(err, specpath.ErrNoSpec) {
		return ClosureReport{}, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ClosureReport{}, nil, nil
		}
		return nil, nil, err
	}
	all, malformed, err := closureInventory(root)
	if err != nil {
		return nil, malformed, err
	}
	stem := specpath.Stem(filepath.Base(path))
	for _, one := range all {
		if one.Stem != stem {
			continue
		}
		if !one.CompletedNotClosed {
			return ClosureReport{one}, malformed, nil
		}
		ack := filepath.Join(root, "tmp", "session", ".closure-ack-"+stem)
		if _, err := os.Stat(ack); err == nil {
			one.Acknowledged = true
		}
		return ClosureReport{one}, malformed, nil
	}
	return ClosureReport{}, malformed, nil
}

func inspectClosureSpec(root, path string, learned []learnedFile, tracked map[string]bool, journalEvidence map[string]string, allStems map[string]bool) (closureSpec, error) {
	body, err := os.ReadFile(path) //nolint:gosec // path is a spec file this walk found under the checkout
	if err != nil {
		return closureSpec{}, fmt.Errorf("read %s: %w", path, err)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return closureSpec{}, err
	}
	rel = filepath.ToSlash(rel)
	stem := specpath.Stem(filepath.Base(path))
	text := string(body)
	rows, _ := metaRows(text)
	one := closureSpec{
		Spec:              rel,
		Stem:              stem,
		Status:            strings.ToLower(metaField(rows, "Status")),
		ReviewGatePresent: strings.Contains(text, "## Review Gate"),
		UncheckedCloseBox: uncheckedPattern.MatchString(text),
	}
	stemTokens := strings.Split(stem, "-")
	one.IsUmbrella = strings.Contains("-"+stem+"-", "-umbrella-")
	if !one.IsUmbrella {
		prefix := stem + "-"
		for other := range allStems {
			if other == stem {
				continue
			}
			if !strings.HasPrefix(other, prefix) {
				continue
			}
			rest := strings.TrimPrefix(other, prefix)
			if startsWithDigit(rest) {
				one.IsUmbrella = true
				break
			}
		}
	}

	for _, file := range learned {
		if !tracked[file.path] {
			continue
		}
		if one.LearnedExact == "" {
			if file.slug == stem {
				one.LearnedExact = file.path
			}
		}
		if one.LearnedStem == "" {
			if contiguousTokens(stemTokens, file.tokens) {
				one.LearnedStem = file.path
			}
		}
	}
	for _, match := range learnedRefPattern.FindAllStringSubmatch(text, -1) {
		candidate := learnedDir + "/" + match[1] + ".md"
		if tracked[candidate] {
			one.LearnedRef = candidate
			break
		}
	}
	one.JournalMatch = journalEvidence[stem]
	one.GateFinished = closureGateFinished(one)
	one.CompletedNotClosed = closureCompletedNotClosed(one)
	one.NeedsVerification = closureNeedsVerification(one)
	one.Evidence = firstNonempty(one.LearnedExact, one.JournalMatch, one.LearnedStem, one.LearnedRef)
	return one, nil
}

func learnedFiles(root string) ([]learnedFile, error) {
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(learnedDir)))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	files := make([]learnedFile, 0, len(entries))
	for _, entry := range entries {
		match := learnedFilePattern.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}
		files = append(files, learnedFile{path: learnedDir + "/" + entry.Name(), slug: match[2], tokens: strings.Split(match[2], "-")})
	}
	return files, nil
}

func contiguousTokens(needle, haystack []string) bool {
	if len(needle) == 0 {
		return false
	}
	if len(needle) > len(haystack) {
		return false
	}
	for start := 0; start <= len(haystack)-len(needle); start++ {
		match := true
		for index := range needle {
			if needle[index] != haystack[start+index] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func startsWithDigit(value string) bool {
	if value == "" {
		return false
	}
	if value[0] < '0' {
		return false
	}
	return value[0] <= '9'
}

func closureGateFinished(one closureSpec) bool {
	if !one.ReviewGatePresent {
		return false
	}
	return !one.UncheckedCloseBox
}

func closureCompletedNotClosed(one closureSpec) bool {
	if one.Status != statusInProgress {
		return false
	}
	if one.IsUmbrella {
		return false
	}
	if one.LearnedExact != "" {
		return true
	}
	if one.JournalMatch == "" {
		return false
	}
	return one.GateFinished
}

func closureNeedsVerification(one closureSpec) bool {
	if one.CompletedNotClosed {
		return false
	}
	if one.Status != statusInProgress {
		return false
	}
	if one.LearnedStem != "" {
		return true
	}
	return one.LearnedRef != ""
}

// resolveSpecPath answers the absolute path of one spec spelling. An absolute
// path and a path with a separator name the file themselves; a bare name or a
// bare stem names no bucket, so specpath resolves which one holds it.
func resolveSpecPath(root, spec string) (string, error) {
	if filepath.IsAbs(spec) {
		return spec, nil
	}
	if strings.ContainsRune(spec, filepath.Separator) || strings.ContainsRune(spec, '/') {
		return filepath.Join(root, filepath.FromSlash(filepath.Clean(spec))), nil
	}
	relative, err := specpath.Find(root, spec)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, filepath.FromSlash(relative)), nil
}

func firstNonempty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// trackedPaths reads Git's index without starting Git. The closure producer used
// `git ls-files`, whose learned-summary contract is index membership.
func trackedPaths(root string) (map[string]bool, error) {
	gitDir, err := repositoryGitDir(root)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(filepath.Join(gitDir, "index")) //nolint:gosec // repository metadata under root
	if err != nil {
		return nil, fmt.Errorf("read Git index: %w", err)
	}
	defer file.Close() //nolint:errcheck // read-only
	return readIndexPaths(file)
}

func repositoryGitDir(root string) (string, error) {
	path := filepath.Join(root, ".git")
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return path, nil
	}
	body, err := os.ReadFile(path) //nolint:gosec // path is the .git pointer inside the checkout
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(string(body))
	if !strings.HasPrefix(line, "gitdir: ") {
		return "", fmt.Errorf("read .git: invalid gitdir file")
	}
	dir := strings.TrimPrefix(line, "gitdir: ")
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(root, dir)
	}
	return filepath.Clean(dir), nil
}

func readIndexPaths(reader io.Reader) (map[string]bool, error) {
	header := make([]byte, 12)
	if _, err := io.ReadFull(reader, header); err != nil {
		return nil, fmt.Errorf("read Git index header: %w", err)
	}
	if string(header[:4]) != "DIRC" {
		return nil, fmt.Errorf("read Git index: invalid signature")
	}
	version := binary.BigEndian.Uint32(header[4:8])
	if version < 2 {
		return nil, fmt.Errorf("read Git index: unsupported version %d", version)
	}
	if version > 4 {
		return nil, fmt.Errorf("read Git index: unsupported version %d", version)
	}
	count := binary.BigEndian.Uint32(header[8:12])
	paths := make(map[string]bool)
	previous := ""
	for range count {
		fixed := make([]byte, 62)
		if _, err := io.ReadFull(reader, fixed); err != nil {
			return nil, fmt.Errorf("read Git index entry: %w", err)
		}
		flags := binary.BigEndian.Uint16(fixed[60:62])
		entrySize := 62
		if flags&0x4000 != 0 {
			extra := make([]byte, 2)
			if _, err := io.ReadFull(reader, extra); err != nil {
				return nil, err
			}
			entrySize += 2
		}
		var path string
		if version == 4 {
			remove, _, err := readIndexVarint(reader)
			if err != nil {
				return nil, err
			}
			suffix, err := readNULTerminated(reader)
			if err != nil {
				return nil, err
			}
			if remove > len(previous) {
				return nil, fmt.Errorf("read Git index: invalid path compression")
			}
			path = previous[:len(previous)-remove] + suffix
			// A version 4 entry carries no alignment padding, so entrySize is
			// not needed on this branch.
		} else {
			name, err := readNULTerminated(reader)
			if err != nil {
				return nil, err
			}
			path = name
			entrySize += len(name) + 1
			padding := (8 - entrySize%8) % 8
			if padding > 0 {
				if _, err := io.CopyN(io.Discard, reader, int64(padding)); err != nil {
					return nil, err
				}
			}
		}
		paths[filepath.ToSlash(path)] = true
		previous = path
	}
	return paths, nil
}

func readNULTerminated(reader io.Reader) (string, error) {
	var body []byte
	one := []byte{0}
	for len(body) <= 1<<20 {
		if _, err := io.ReadFull(reader, one); err != nil {
			return "", err
		}
		if one[0] == 0 {
			return string(body), nil
		}
		body = append(body, one[0])
	}
	return "", fmt.Errorf("read Git index: path exceeds 1 MiB")
}

func readIndexVarint(reader io.Reader) (int, int, error) {
	value := 0
	used := 0
	one := []byte{0}
	for used < 10 {
		if _, err := io.ReadFull(reader, one); err != nil {
			return 0, used, err
		}
		used++
		value = (value << 7) | int(one[0]&0x7f)
		if one[0]&0x80 == 0 {
			return value, used, nil
		}
		value++
	}
	return 0, used, fmt.Errorf("read Git index: invalid path-compression integer")
}
