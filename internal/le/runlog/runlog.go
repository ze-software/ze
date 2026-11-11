// Design: docs/contributing/running-commands.md -- the lines of a run log a reader needs first
// Related: ../verify/engine/summary.go -- the verification failure index built from these lines
// Related: ../job/quiet.go -- the same lines, answered by one job
// Related: ../test/unit/pkg.go -- Failures and PackageResults, the answer of the scoped package test
//
// A run log is thousands of lines, and a reader who wants to know what broke
// needs five of them. This package owns WHICH lines those are, so the
// verification failure index and a quiet job name the same lines instead of
// each carrying its own pattern of what a failure looks like.
//
// Every function here reads one line at a time. A stage log of a full verification
// run is hundreds of megabytes, so memory here is bounded by the longest line
// plus the lines that are kept.
package runlog

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// failureLine matches the line openings that a reader of a failed run needs
// first: the assertion that failed, the package that failed, and the two ways
// a Go process dies.
var failureLine = regexp.MustCompile(`^(--- FAIL:|FAIL[[:space:]]|panic:|fatal error:|Error:|\[FAIL\])`)

// Key returns up to limit failure lines from a run log, each prefixed with its
// 1-based line number, so a reader can find the line in the full log.
//
// An empty result says the log holds no line of that shape. It does not say
// the run passed: a caller that wants the verdict reads the exit code.
func Key(reader io.Reader, limit int) ([]string, error) {
	lines := make([]string, 0, limit)
	err := forEachLine(reader, func(number int, text string) bool {
		if !failureLine.MatchString(text) {
			return true
		}
		lines = append(lines, strconv.Itoa(number)+":"+text)
		return len(lines) < limit
	})
	return lines, err
}

// failedTestPrefix opens the line `go test` prints for a top-level test that
// failed. A subtest's line is indented, so it does not open with this prefix:
// re-running the top-level test runs its subtests again.
const failedTestPrefix = "--- FAIL: "

// FailedTests returns the name of every top-level test a `go test` log
// reports failed, once each, in the order the log first names it.
//
// The name is the text after the prefix up to the first space, which is where
// `go test` puts the elapsed time. A log run with -count above one names a
// test once per run, and the answer still names it once.
//
// An empty result says no `--- FAIL:` line is in the log. It does not say the
// run passed: a package that did not compile fails with no such line.
func FailedTests(reader io.Reader) ([]string, error) {
	var names []string
	seen := make(map[string]bool)
	err := forEachLine(reader, func(_ int, text string) bool {
		rest, found := strings.CutPrefix(text, failedTestPrefix)
		if !found {
			return true
		}
		name, _, _ := strings.Cut(strings.TrimSpace(rest), " ")
		if name == "" {
			return true
		}
		if seen[name] {
			return true
		}
		seen[name] = true
		names = append(names, name)
		return true
	})
	return names, err
}

// The line openings Failures reads, beside failureLine. A failed test's block
// is its `--- FAIL:` line at the start of a line and every indented line under
// it, which is where `go test` prints the assertion text, a subtest's own
// `--- FAIL:` line included. A compile or vet failure opens with the `# <pkg>`
// header the go command prints and continues to the next result line.
var (
	diedLine   = regexp.MustCompile(`^(panic:|fatal error:)`)
	resultLine = regexp.MustCompile(`^(ok[[:space:]]|\?[[:space:]]|FAIL$|FAIL[[:space:]]|PASS$|--- |=== )`)
)

// buildHeaderPrefix opens the line the go command prints above a package's
// compile or vet errors.
const buildHeaderPrefix = "# "

// failureBlock is where Failures stands in a log: outside any failure, inside a
// failed test's block, or inside a package's compile or vet errors.
type failureBlock uint8

const (
	outsideFailure failureBlock = iota
	inFailedTest
	inBuildErrors
)

// Failures returns up to limit lines that say WHY a `go test` or `go vet` run
// failed, each prefixed with its 1-based line number as Key prefixes them: the
// block of every failed test (its `--- FAIL:` line and the indented lines under
// it), every compile or vet error block, and every panic or fatal error line,
// in log order.
//
// Key names a failed test and stops there, so its reader's next step is to
// open the log for the assertion text. Failures keeps that text. When the log
// holds none of those blocks, the answer is Key's lines, so a failure of a
// shape no block describes is still named.
//
// An empty result says the log holds no failure line of any shape. It does not
// say the run passed: a caller that wants the verdict reads the exit code.
func Failures(reader io.Reader, limit int) ([]string, error) {
	blocks := make([]string, 0, limit)
	keys := make([]string, 0, limit)
	state := outsideFailure
	err := forEachLine(reader, func(number int, text string) bool {
		if len(keys) < limit && failureLine.MatchString(text) {
			keys = append(keys, strconv.Itoa(number)+":"+text)
		}
		state = nextFailureBlock(state, text)
		if state == outsideFailure && !diedLine.MatchString(text) {
			return true
		}
		blocks = append(blocks, strconv.Itoa(number)+":"+text)
		return len(blocks) < limit
	})
	if len(blocks) == 0 {
		return keys, err
	}
	return blocks, err
}

// nextFailureBlock answers the block a line belongs to, given the block the
// line before it was in.
func nextFailureBlock(state failureBlock, text string) failureBlock {
	if strings.HasPrefix(text, failedTestPrefix) {
		return inFailedTest
	}
	if strings.HasPrefix(text, buildHeaderPrefix) {
		return inBuildErrors
	}
	switch state {
	case inFailedTest:
		if strings.HasPrefix(text, " ") || strings.HasPrefix(text, "\t") {
			return inFailedTest
		}
	case inBuildErrors:
		if text != "" && !resultLine.MatchString(text) {
			return inBuildErrors
		}
	case outsideFailure:
	}
	return outsideFailure
}

// PackageCounts is how many packages a `go test` log reports in each verdict.
// NoTest counts both lines on which a package ran no test: `ok` marked
// `[no tests to run]` (a -run regex that matched nothing in it) and `?` marked
// `[no test files]`. A package counted there was not tested, so it is never
// counted as passed.
type PackageCounts struct {
	Passed int `json:"passed"`
	NoTest int `json:"no-test"`
	Failed int `json:"failed"`
}

// The package result lines `go test` prints without -v, one per package: the
// verdict word, the padding, the package, a tab, then the time or a marker.
// A test that prints "ok" at the start of a line of its own output is not a
// result line, because the package word and the tab after it are required.
var (
	passedPackage = regexp.MustCompile(`^ok\s+\S+\t`)
	noTestFiles   = regexp.MustCompile(`^\?\s+\S+\t\[no test files\]`)
	failedPackage = regexp.MustCompile(`^FAIL\t\S+`)
)

// noTestsToRun is the marker on an `ok` line whose package ran no test.
const noTestsToRun = "[no tests to run]"

// PackageResults counts the package result lines of a `go test` log.
//
// A zero count is an answer about the log, not about the run: a run that died
// before any package reported, or a `go vet` log, holds no result line.
func PackageResults(reader io.Reader) (PackageCounts, error) {
	var counts PackageCounts
	err := forEachLine(reader, func(_ int, text string) bool {
		if passedPackage.MatchString(text) {
			if strings.Contains(text, noTestsToRun) {
				counts.NoTest++
				return true
			}
			counts.Passed++
			return true
		}
		if noTestFiles.MatchString(text) {
			counts.NoTest++
			return true
		}
		if failedPackage.MatchString(text) {
			counts.Failed++
		}
		return true
	})
	return counts, err
}

// Head returns the first limit lines of a run log, unnumbered. A linter writes
// its findings from the first line, so the head IS the answer there.
func Head(reader io.Reader, limit int) ([]string, error) {
	lines := make([]string, 0, limit)
	err := forEachLine(reader, func(_ int, text string) bool {
		lines = append(lines, text)
		return len(lines) < limit
	})
	return lines, err
}

// forEachLine calls visit for each line of reader, numbering from 1, and stops
// when visit answers false or the log ends. A final line with no newline is a
// line, and a trailing newline does not produce an empty one.
func forEachLine(reader io.Reader, visit func(number int, text string) bool) error {
	buffered := bufio.NewReader(reader)
	for number := 1; ; number++ {
		text, err := buffered.ReadString('\n')
		ended := err != nil
		text = strings.TrimSuffix(text, "\n")

		if !ended || text != "" {
			if !visit(number, text) {
				return nil
			}
		}
		if !ended {
			continue
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		return fmt.Errorf("read run log: %w", err)
	}
}
