// Design: docs/architecture/testing/test-health.md -- the weakening detector
// Related: scope.go -- the unit boundaries that give each verdict a test name.
//
// The detector is lexical by design. It preserves the producer's ordered
// verdicts, including advisory count drops, so the edit hook and commit gate do
// not acquire different definitions of a weakened test during the port.
package weakened

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	assertPattern = regexp.MustCompile(
		`(?:\bt\.(?:Error|Errorf|Fatal|Fatalf|Fail|FailNow)|assert\.|require\.)`,
	)
	fatalPattern              = regexp.MustCompile(`(?:\bt\.(?:Fatal|Fatalf|FailNow)|require\.)`)
	skipPattern               = regexp.MustCompile(`\b[A-Za-z_]\w*\.Skip(?:Now|f)?[ \t]*\(`)
	ignoreTagPattern          = regexp.MustCompile(`//(?:go:build ignore\b|[ \t]*\+build ignore\b)`)
	tableCasePattern          = regexp.MustCompile(`\{\s*(?:name|Name)\s*:`)
	goTestFunctionPattern     = regexp.MustCompile(`(?m)^func (Test|Fuzz|Benchmark)`)
	commentedAssertionPattern = regexp.MustCompile(
		`//[^\n]*(?:\bt\.(?:Error|Errorf|Fatal|Fatalf|Fail|FailNow)|assert\.|require\.)`,
	)
	ciCoveragePattern = regexp.MustCompile(
		`(?m)^[ \t]*(?:expect|reject|cmd)=` +
			`|^[ \t]*assert\b` +
			`|^[ \t]*(?:\w+\.)?(?:runtime_fail|fail)[ \t]*\(` +
			`|\b(?:wait_until|dispatch_until)[ \t]*\(`,
	)
	ciRejectPattern      = regexp.MustCompile(`(?m)^[ \t]*reject=`)
	ciEmptyNeedlePattern = regexp.MustCompile(
		`(?m)^[ \t]*(?:expect|reject)=[ \t]*$` +
			`|^[ \t]*(?:expect|reject)=.*:[A-Za-z_][\w-]*=[ \t]*$`,
	)
	// heredocOpenPattern finds a carrier directive line that opens an embedded
	// heredoc body (stdin=, tmpfs=, or any future directive shaped the same
	// way), capturing the terminator name that closes it. Every occurrence in
	// the corpus carries `terminator=<NAME>` as the line's final field.
	heredocOpenPattern        = regexp.MustCompile(`terminator=([A-Za-z0-9_]+)[ \t]*$`)
	pythonTestFunctionPattern = regexp.MustCompile(
		`(?m)^[ \t]*(?:async[ \t]+)?def[ \t]+test`,
	)
	pythonSkipPattern = regexp.MustCompile(
		`\bself\.skipTest[ \t]*\(` +
			`|\braise[ \t]+(?:unittest\.)?SkipTest\b` +
			`|@(?:unittest\.)?skip(?:If|Unless)?[ \t]*\(` +
			`|@(?:unittest\.)?expectedFailure\b` +
			`|@pytest\.mark\.skip` +
			`|@pytest\.mark\.xfail` +
			`|\bpytest\.skip[ \t]*\(`,
	)
	pythonAssertionPattern = regexp.MustCompile(
		`(?m)\bself\.assert[A-Za-z]*[ \t]*\(` +
			`|\bpytest\.raises[ \t]*\(` +
			`|^[ \t]*assert[ \t]`,
	)
	tautologyConstantPattern = regexp.MustCompile(
		`(?m)\bassert[ \t]+(?:True|1)[ \t]*(?:$|[,#])` +
			`|\bassert[ \t]+not[ \t]+(?:False|0)[ \t]*(?:$|[,#])` +
			`|\b(?:require|assert)\.(?:True|Truef)\([^,]+,[ \t]*true[ \t]*[,)]` +
			`|\b(?:require|assert)\.(?:False|Falsef)\([^,]+,[ \t]*false[ \t]*[,)]`,
	)
	tautologyEqualityPattern = regexp.MustCompile(
		`(?m)\bassert[ \t]+(\S+)[ \t]*==[ \t]*(\S+)[ \t]*(?:$|[,#])`,
	)
)

type detectorVerdict struct {
	blocking []string
	advisory []string
}

// detect answers every ordered blocking and advisory verdict for one unit.
func detect(oldText, newText, path string) detectorVerdict {
	var verdict detectorVerdict
	hashComments := strings.HasSuffix(path, ".ci") ||
		strings.HasSuffix(path, ".et") || strings.HasSuffix(path, ".py")
	oldSource, oldComments := splitComments(oldText, hashComments)
	newSource, newComments := splitComments(newText, hashComments)

	if strings.TrimSpace(newText) == "" && strings.TrimSpace(oldText) != "" {
		verdict.blocking = append(verdict.blocking, "replacing test content with empty string")
	}
	if goTestFunctionPattern.MatchString(oldText) && !goTestFunctionPattern.MatchString(newText) {
		verdict.blocking = append(verdict.blocking, "deleting Test/Fuzz/Benchmark function")
	}
	oldRuns := strings.Count(oldSource, "t.Run(")
	newRuns := strings.Count(newSource, "t.Run(")
	if oldRuns > newRuns {
		verdict.advisory = append(verdict.advisory,
			countVerdict("removing t.Run cases", oldRuns, newRuns, ""))
	}
	oldTable := matchCount(tableCasePattern, oldSource)
	newTable := matchCount(tableCasePattern, newSource)
	if oldTable > newTable {
		verdict.advisory = append(verdict.advisory,
			countVerdict("removing table-driven cases", oldTable, newTable, ""))
	}
	oldAssertions := matchCount(assertPattern, oldSource)
	newAssertions := matchCount(assertPattern, newSource)
	if oldAssertions > newAssertions && strings.TrimSpace(newText) != "" {
		verdict.advisory = append(verdict.advisory,
			countVerdict("removing assertions", oldAssertions, newAssertions, ""))
	}
	oldFatal := matchCount(fatalPattern, oldSource)
	newFatal := matchCount(fatalPattern, newSource)
	if oldFatal > newFatal && newAssertions >= oldAssertions {
		verdict.advisory = append(verdict.advisory, countVerdict(
			"downgrading fatal assertions to non-fatal", oldFatal, newFatal, " require/Fatal"))
	}
	oldSkip := matchCount(skipPattern, oldSource)
	newSkip := matchCount(skipPattern, newSource)
	if newSkip > oldSkip {
		verdict.blocking = append(verdict.blocking,
			countVerdict("adding t.Skip", oldSkip, newSkip, "; the test stops running"))
	}
	if ignoreTagPattern.MatchString(newText) && !ignoreTagPattern.MatchString(oldText) {
		verdict.blocking = append(verdict.blocking,
			"adding 'ignore' build tag; file dropped from the build")
	}
	oldCommented := matchCount(commentedAssertionPattern, oldComments)
	newCommented := matchCount(commentedAssertionPattern, newComments)
	if newCommented > oldCommented {
		verdict.blocking = append(verdict.blocking,
			countVerdict("commenting out assertions", oldCommented, newCommented, ""))
	}

	if strings.HasSuffix(path, ".ci") || strings.HasSuffix(path, ".et") {
		verdict = detectCarrierVerdicts(verdict, oldText, newText, oldSource, newSource)
	}
	if isPythonTest(path) {
		verdict = detectPythonVerdicts(verdict, newText, oldSource, newSource)
	}

	oldTautologies := tautologyCount(oldSource)
	newTautologies := tautologyCount(newSource)
	if newTautologies > oldTautologies {
		verdict.blocking = append(verdict.blocking, countVerdict(
			"introducing an assertion that cannot fail", oldTautologies, newTautologies, ""))
	}
	return verdict
}

func detectCarrierVerdicts(
	verdict detectorVerdict,
	oldText, newText, oldSource, newSource string,
) detectorVerdict {
	// The coverage and reject counts below are judged on the carrier's own
	// directives, never on a heredoc body's program (a Python fixture, a
	// pushed config file, ...). Stripping still leaves every real
	// expect=/reject=/cmd= line, on both sides, so a directive genuinely
	// dropped from the carrier is still caught: only what an embedded body
	// happened to contain stops being counted as the carrier's own coverage.
	oldCarrier := stripHeredocBodies(oldSource)
	newCarrier := stripHeredocBodies(newSource)
	oldCoverage := matchCount(ciCoveragePattern, oldCarrier)
	newCoverage := matchCount(ciCoveragePattern, newCarrier)
	if oldCoverage > newCoverage {
		verdict.advisory = append(verdict.advisory, countVerdict(
			"removing expectations", oldCoverage, newCoverage,
			" expect=/reject=/cmd=/assert/fail"))
	}
	oldRejects := matchCount(ciRejectPattern, oldCarrier)
	newRejects := matchCount(ciRejectPattern, newCarrier)
	if oldRejects > newRejects {
		verdict.advisory = append(verdict.advisory,
			countVerdict("removing negative expectations", oldRejects, newRejects, " reject="))
	}
	// The empty-needle check stays on the raw, unstripped text: it is
	// blocking, and a needle emptied inside a heredoc body still matches
	// anything the moment that body is the thing dispatched.
	oldEmpty := matchCount(ciEmptyNeedlePattern, oldText)
	newEmpty := matchCount(ciEmptyNeedlePattern, newText)
	if newEmpty > oldEmpty {
		verdict.blocking = append(verdict.blocking, countVerdict(
			"emptying an expectation's needle", oldEmpty, newEmpty,
			"; it now matches anything"))
	}
	return verdict
}

// stripHeredocBodies removes the embedded body of every carrier heredoc from
// text, so a .ci/.et carrier is judged on its own directives rather than on
// whatever program a heredoc happens to carry. A heredoc opens on a line
// ending `terminator=NAME` and its body ends at the first following line
// that is exactly NAME; both the opening directive and the closing line stay,
// only the lines strictly between them are dropped. An unterminated heredoc
// consumes the rest of the file as body -- this never panics, and it never
// drops a directive that precedes the open it cannot close.
// fixtureBlockOpener reports whether a block opener writes a FILE rather than
// feeding the carrier's own input.
//
// The distinction decides what may be dropped, and getting it wrong blinds the
// gate to the strongest assertions this repository has. A `tmpfs=` block is a
// file the carrier writes to disk, so its bytes are a fixture. A `stdin=` block
// is carrier input, and `stdin=peer:terminator=EOF_PEER` carries the
// `expect=bgp:...:hex=...` wire expectations themselves: 548 of 1955 carriers
// hold an expect= or reject= line inside a block, and every one of them is an
// assertion a reader must still be shown losing.
func fixtureBlockOpener(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "tmpfs=")
}

func stripHeredocBodies(text string) string {
	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))
	index := 0
	for index < len(lines) {
		line := lines[index]
		kept = append(kept, line)
		index++
		match := heredocOpenPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		if !fixtureBlockOpener(line) {
			continue
		}
		terminator := match[1]
		for index < len(lines) && lines[index] != terminator {
			index++
		}
		if index < len(lines) {
			kept = append(kept, lines[index])
			index++
		}
	}
	return strings.Join(kept, "\n")
}

func detectPythonVerdicts(
	verdict detectorVerdict,
	newText, oldSource, newSource string,
) detectorVerdict {
	if pythonTestFunctionPattern.MatchString(oldSource) &&
		!pythonTestFunctionPattern.MatchString(newSource) {
		verdict.blocking = append(verdict.blocking, "deleting every def test_ function")
	}
	oldSkips := matchCount(pythonSkipPattern, oldSource)
	newSkips := matchCount(pythonSkipPattern, newSource)
	if newSkips > oldSkips {
		verdict.blocking = append(verdict.blocking, countVerdict(
			"adding a Python skip", oldSkips, newSkips, "; the test stops running"))
	}
	oldAssertions := matchCount(pythonAssertionPattern, oldSource)
	newAssertions := matchCount(pythonAssertionPattern, newSource)
	if oldAssertions > newAssertions && strings.TrimSpace(newText) != "" {
		verdict.advisory = append(verdict.advisory,
			countVerdict("removing assertions", oldAssertions, newAssertions, ""))
	}
	return verdict
}

func countVerdict(kind string, oldCount, newCount int, suffix string) string {
	var text textbuf.Buffer
	text.Str(kind).Str(" (").Int(int64(oldCount)).Str(" -> ").Int(int64(newCount))
	if strings.HasPrefix(suffix, ";") {
		return text.Byte(')').Str(suffix).String()
	}
	return text.Str(suffix).Byte(')').String()
}

func decimal(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	at := len(digits)
	for value > 0 {
		at--
		digits[at] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[at:])
}

func matchCount(pattern *regexp.Regexp, text string) int {
	return len(pattern.FindAllStringIndex(text, -1))
}

func tautologyCount(text string) int {
	count := matchCount(tautologyConstantPattern, text)
	for _, match := range tautologyEqualityPattern.FindAllStringSubmatch(text, -1) {
		if len(match) == 3 && match[1] == match[2] {
			count++
		}
	}
	return count
}

func splitComments(text string, hashed bool) (string, string) {
	marker := "//"
	if hashed {
		marker = "#"
	}
	lines := strings.Split(text, "\n")
	comments := make([]string, len(lines))
	for index, line := range lines {
		source := stripLine(line, marker)
		lines[index] = source
		comments[index] = line[len(source):]
	}
	return strings.Join(lines, "\n"), strings.Join(comments, "\n")
}

func stripLine(line, marker string) string {
	var quote byte
	cut := -1
	for index := 0; index < len(line); index++ {
		character := line[index]
		if quote != 0 {
			if character == '\\' {
				index++
				continue
			}
			if character == quote {
				quote = 0
			}
			continue
		}
		if character == '"' || character == '\'' || character == '`' {
			quote = character
			continue
		}
		if cut < 0 && strings.HasPrefix(line[index:], marker) {
			cut = index
		}
	}
	if quote != 0 {
		return cutAtFirst(line, marker)
	}
	if cut < 0 {
		return line
	}
	return line[:cut]
}

func cutAtFirst(line, marker string) string {
	before, _, found := strings.Cut(line, marker)
	if !found {
		return line
	}
	return before
}

func isPythonTest(path string) bool {
	name := filepath.Base(path)
	return strings.HasSuffix(name, "_test.py") ||
		(strings.HasPrefix(name, "test_") && strings.HasSuffix(name, ".py"))
}

// executableTestText masks Python string tokens while retaining byte positions
// and newlines. If the source has an unterminated string, it returns the source
// unchanged so malformed input cannot hide a weakening.
func executableTestText(path, text string) string {
	if !strings.HasSuffix(path, ".py") {
		return text
	}
	masked, ok := maskPythonStrings(text)
	if !ok {
		return text
	}
	return masked
}

func maskPythonStrings(text string) (string, bool) {
	masked := []byte(text)
	for index := 0; index < len(text); {
		if text[index] == '#' {
			if newline := strings.IndexByte(text[index:], '\n'); newline >= 0 {
				index += newline + 1
				continue
			}
			break
		}
		start, quoteAt, ok := pythonStringStart(text, index)
		if !ok {
			index++
			continue
		}
		end, complete := pythonStringEnd(text, quoteAt)
		if !complete {
			return text, false
		}
		for at := start; at < end; at++ {
			if masked[at] != '\n' && masked[at] != '\r' {
				masked[at] = ' '
			}
		}
		index = end
	}
	return string(masked), true
}

func pythonStringStart(text string, index int) (int, int, bool) {
	if text[index] == '\'' || text[index] == '"' {
		return index, index, true
	}
	if !isPythonPrefixByte(text[index]) {
		return 0, 0, false
	}
	if index > 0 && (unicode.IsLetter(rune(text[index-1])) ||
		unicode.IsDigit(rune(text[index-1])) || text[index-1] == '_') {
		return 0, 0, false
	}
	end := index
	for end < len(text) && end-index < 2 && isPythonPrefixByte(text[end]) {
		end++
	}
	if end < len(text) && (text[end] == '\'' || text[end] == '"') &&
		validPythonPrefix(strings.ToLower(text[index:end])) {
		return index, end, true
	}
	return 0, 0, false
}

func isPythonPrefixByte(character byte) bool {
	return strings.ContainsRune("rRuUbBfF", rune(character))
}

func validPythonPrefix(prefix string) bool {
	switch prefix {
	case "r", "u", "b", "f", "br", "rb", "fr", "rf":
		return true
	default:
		return false
	}
}

func pythonStringEnd(text string, quoteAt int) (int, bool) {
	quote := text[quoteAt]
	triple := quoteAt+2 < len(text) && text[quoteAt+1] == quote && text[quoteAt+2] == quote
	index := quoteAt + 1
	if triple {
		index = quoteAt + 3
	}
	for index < len(text) {
		if text[index] == '\\' {
			index += 2
			continue
		}
		if triple {
			if index+2 < len(text) && text[index] == quote &&
				text[index+1] == quote && text[index+2] == quote {
				return index + 3, true
			}
			index++
			continue
		}
		if text[index] == quote {
			return index + 1, true
		}
		if text[index] == '\n' || text[index] == '\r' {
			return 0, false
		}
		index++
	}
	return 0, false
}
