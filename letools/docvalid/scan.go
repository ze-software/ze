// Design: docs/architecture/core-design.md -- what the drift gate reads
// Overview: drift.go -- the checks these counts feed
//
// scan.go holds the reading half of the drift gate: the counts it takes from
// the tree, and the small parsers that pull a claim out of a documentation
// line. Every check in drift.go compares one of these numbers against one of
// those claims.
//
// The collector is a VALUE, not a package variable. The script kept the
// unreadable-file list in a global, which makes two runs in one process share
// it and makes a test of the second run report the first one's findings
// (letools/consistency did the same conversion in step 2).

package docvalid

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// checker is one run of the drift gate over one tree.
//
// unreadable collects the files and directories this run could not read. Every
// count here is drawn from a scan, and a scan that stops early yields a LOW
// count with no finding. That is the shape that reports a document accurate
// because the check never reached the drift, so it is reported as drift of its
// own.
type checker struct {
	root       string
	unreadable []Issue
}

// noteUnreadable records a file whose scan stopped before the end.
func (c *checker) noteUnreadable(path string, err error) {
	var tb textbuf.Buffer
	c.unreadable = append(c.unreadable, Issue{
		File:    path,
		Message: "read stopped early, so the checks over this file are incomplete",
		Detail:  tb.Err(err).String(),
	})
}

// noteUnopenable records a file or a directory the scan could not open at all.
//
// The script covered the scan that STOPS and left the one that never starts
// silent: os.Open failing answered a zero count, and a zero count agrees with
// any document that understates the tree. A dangling symbolic link, a
// directory closed to this user, and a file whose mode changed under the walk
// all reach this, and all of them used to pass.
func (c *checker) noteUnopenable(path string, err error) {
	var tb textbuf.Buffer
	c.unreadable = append(c.unreadable, Issue{
		File:    path,
		Message: "could not be read, so the checks over it never ran",
		Detail:  tb.Err(err).String(),
	})
}

// readLines answers every line of path.
//
// Two failures are told apart. A file that is ABSENT is not drift: a tree that
// carries no README.md makes no claim about the tree, and the caller skips it.
// A file that is PRESENT and cannot be read is recorded, because the check over
// it did not run and its silence would read as agreement.
func (c *checker) readLines(path string) ([]string, error) {
	f, err := os.Open(path) //nolint:gosec // repository-relative documentation path
	if err != nil {
		if !vanished(path) {
			c.noteUnopenable(path, err)
		}
		return nil, err
	}
	defer f.Close() //nolint:errcheck // read-only

	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		c.noteUnreadable(path, err)
		return nil, err
	}
	return lines, nil
}

// countMatchingLines returns how many lines of path match re.
//
// A scan that stops early yields a LOW count, and a low count is the direction
// that agrees with a document claiming fewer tests than the tree holds. Both
// halves of that failure are recorded: the scan that stops, and the file that
// never opened.
func (c *checker) countMatchingLines(path string, re *regexp.Regexp) int {
	f, err := os.Open(path) //nolint:gosec // path comes from a walk of the tree under check
	if err != nil {
		if !vanished(path) {
			c.noteUnopenable(path, err)
		}
		return 0
	}
	defer f.Close() //nolint:errcheck // read-only

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if re.MatchString(scanner.Text()) {
			count++
		}
	}
	if err := scanner.Err(); err != nil {
		c.noteUnreadable(path, err)
	}
	return count
}

// walkFailure decides what a walk error means for the tree under check.
//
// A walk ROOT that does not exist ends the walk and says nothing: a tree with
// no test/ holds no functional tests. Anything else is a part of the tree this
// scan cannot read, so it is recorded and the walk CONTINUES: one closed
// directory must not cost the rest of the tree its counts.
func (c *checker) walkFailure(walkRoot, path string, err error) error {
	if path == walkRoot || vanished(path) {
		return filepath.SkipAll
	}
	c.noteUnopenable(path, err)
	return nil
}

// countCITests answers how many .ci files testDir holds, and how many each of
// its top-level directories holds.
func (c *checker) countCITests(testDir string) (int, map[string]int) {
	total := 0
	byDir := make(map[string]int)
	//nolint:errcheck // walkFailure records every failure; the walk's own answer adds nothing
	_ = filepath.WalkDir(testDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return c.walkFailure(testDir, path, err)
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".ci") {
			return nil
		}
		total++
		rel, relErr := filepath.Rel(testDir, path)
		if relErr == nil {
			dir, _, _ := strings.Cut(rel, string(filepath.Separator))
			byDir[dir]++
		}
		return nil
	})
	return total, byDir
}

// countInteropScenarios answers how many scenario directories scenariosDir
// holds.
//
// A dotfile-prefixed directory (a Python linter's cache left behind by a
// scenario's check.py) is never a scenario, and counting one inflated the
// doc-claimed count by one per cache directory.
func (c *checker) countInteropScenarios(scenariosDir string) int {
	entries, err := os.ReadDir(scenariosDir)
	if err != nil {
		if !vanished(scenariosDir) {
			c.noteUnopenable(scenariosDir, err)
		}
		return 0
	}
	count := 0
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			count++
		}
	}
	return count
}

// countFuzzTargets answers how many fuzz targets the tree holds.
func (c *checker) countFuzzTargets(root string) int {
	count := 0
	re := regexp.MustCompile(`^func Fuzz`)
	//nolint:errcheck // walkFailure records every failure; the walk's own answer adds nothing
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return c.walkFailure(root, path, err)
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		if strings.Contains(path, "vendor") {
			return nil
		}
		count += c.countMatchingLines(path, re)
		return nil
	})
	return count
}

// countGoTestFunctions answers how many Go test functions the product tree
// holds.
func (c *checker) countGoTestFunctions(root string) int {
	count := 0
	re := regexp.MustCompile(`^func Test`)
	for _, area := range []string{"internal", "pkg", "cmd"} {
		areaRoot := filepath.Join(root, area)
		//nolint:errcheck // walkFailure records every failure; the walk's own answer adds nothing
		_ = filepath.WalkDir(areaRoot, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return c.walkFailure(areaRoot, path, err)
			}
			if d.IsDir() || !strings.HasSuffix(d.Name(), "_test.go") {
				return nil
			}
			count += c.countMatchingLines(path, re)
			return nil
		})
	}
	return count
}

// extractCount reads the first number a pattern's first group captures on a
// line, and answers 0 when the line makes no such claim.
func extractCount(line, pattern string) int {
	re := regexp.MustCompile(pattern)
	m := re.FindStringSubmatch(line)
	if len(m) < 2 {
		return 0
	}
	n, err := strconv.Atoi(strings.ReplaceAll(m[1], ",", ""))
	if err != nil {
		return 0
	}
	return n
}

// extractApprox reads a "~N" style claim. It is extractCount under the name the
// checks read it by.
func extractApprox(line, pattern string) int {
	return extractCount(line, pattern)
}

// extractFloorCount reads a count that may be written as a floor ("100+").
//
// The pattern must expose two groups: the digits, and an optional literal `+`.
// isFloor reports whether the `+` was present, so the caller can require
// actual >= claimed instead of equality. It answers 0 when the line does not
// match, which the callers already treat as "no claim on this line".
func extractFloorCount(line, pattern string) (n int, isFloor bool) {
	m := regexp.MustCompile(pattern).FindStringSubmatch(line)
	if len(m) < 3 {
		return 0, false
	}
	n, err := strconv.Atoi(strings.ReplaceAll(m[1], ",", ""))
	if err != nil {
		return 0, false
	}
	return n, m[2] == "+"
}

// withinThreshold reports whether a claimed count is close enough to the real
// one, which is what an approximate claim ("~2,000 functional tests") asks for.
func withinThreshold(claimed, actual int, threshold float64) bool {
	if actual == 0 {
		return claimed == 0
	}
	diff := float64(claimed-actual) / float64(actual)
	if diff < 0 {
		diff = -diff
	}
	return diff <= threshold
}

// extractTableColumn answers one column of the first markdown table whose
// header row names both headers.
func extractTableColumn(lines []string, header1, header2 string, colIdx int) []string {
	var values []string
	inTable := false
	pastSeparator := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inTable {
			if strings.Contains(trimmed, header1) && strings.Contains(trimmed, header2) && strings.HasPrefix(trimmed, "|") {
				inTable = true
			}
			continue
		}
		if !pastSeparator {
			if strings.Contains(trimmed, "---") {
				pastSeparator = true
			}
			continue
		}
		if !strings.HasPrefix(trimmed, "|") {
			break
		}
		cells := splitTableRow(trimmed)
		if colIdx < len(cells) {
			values = append(values, strings.TrimSpace(cells[colIdx]))
		}
	}
	return values
}

// splitTableRow answers the cells of one markdown table row.
func splitTableRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// lineNumberContaining answers the 1-based line holding fragment, or 0.
func lineNumberContaining(lines []string, fragment string) int {
	for i, line := range lines {
		if strings.Contains(line, fragment) {
			return i + 1
		}
	}
	return 0
}
