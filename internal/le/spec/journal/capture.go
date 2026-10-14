// Design: docs/architecture/core-design.md -- le tool answers stay structured until rendering
// Related: journal.go -- the class reader that collects the Captured: lines
// Related: validate.go -- the working-tree reader that judges them before commit
//
// capture.go parses a class file's Captured: line and resolves the check,
// proof test or rule point it names. plan/journal/README.md documents the
// grammar. The report resolves against git HEAD and the validator against the
// working tree, through the same captureSource contract, never a mix.
package specjournal

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	capturePrefix = "Captured:"
	pointsDir     = "ai/rules/points/"
	// markerFixed and markerOpen are the only two Fix-cell leads. The match is
	// exact and case-sensitive: "Fixed: " and "fixed:" are unmarked.
	markerFixed = "fixed: "
	markerOpen  = "open: "
)

// fixMarker is the lead of a row's Fix cell. Its zero is fixUnmarked, the
// refused state, so a row nobody classified fails closed.
type fixMarker uint8

const (
	fixUnmarked fixMarker = iota
	fixFixed
	fixOpen
)

// fixMarkerOf reads the marker a Fix cell starts with.
func fixMarkerOf(fix string) fixMarker {
	if strings.HasPrefix(fix, markerFixed) {
		return fixFixed
	}
	if strings.HasPrefix(fix, markerOpen) {
		return fixOpen
	}
	return fixUnmarked
}

// withoutFixMarker strips an exact leading marker, and nothing else, so a row
// whose only change is the added marker pairs with its unmarked HEAD row.
func withoutFixMarker(fix string) string {
	if rest, found := strings.CutPrefix(fix, markerFixed); found {
		return rest
	}
	if rest, found := strings.CutPrefix(fix, markerOpen); found {
		return rest
	}
	return fix
}

// isCaptureLine reports whether line is a Captured: line. Such a line is never
// a row, even when it carries a pipe: it is judged as a capture instead.
func isCaptureLine(line string) bool {
	return strings.HasPrefix(strings.TrimLeftFunc(line, unicode.IsSpace), capturePrefix)
}

// captureLine is one Captured: line and its 1-based line number.
type captureLine struct {
	number int
	text   string
}

// captureLinesOf returns every Captured: line of a class file.
func captureLinesOf(content string) []captureLine {
	var lines []captureLine
	for index, line := range strings.Split(content, "\n") {
		if isCaptureLine(line) {
			lines = append(lines, captureLine{number: index + 1, text: strings.TrimSpace(line)})
		}
	}
	return lines
}

// funcRef is a Go function named by its repo-relative directory.
type funcRef struct {
	dir  string
	name string
}

func (ref funcRef) String() string {
	var tb textbuf.Buffer
	return tb.Str(ref.dir).Byte('.').Str(ref.name).String()
}

// parsedCapture is a Captured: line whose grammar holds. Its references are
// not yet resolved: resolveCapture decides that against one tree.
type parsedCapture struct {
	capture Capture
	line    int
	check   funcRef
	proof   funcRef
}

// captureProblem is one refusal of a class's capture, in either reader.
type captureProblem struct {
	kind   string
	line   int
	detail string
}

// captureVerdict is a class's capture judgment: both fields nil means the
// class is uncaptured; a problem means the capture is refused.
type captureVerdict struct {
	parsed  *parsedCapture
	problem *captureProblem
}

// judgeCaptureLines parses a class's Captured: lines. latest is the newest date
// a capture may carry and lateReason says what it is.
func judgeCaptureLines(lines []captureLine, latest, lateReason string) captureVerdict {
	if len(lines) == 0 {
		return captureVerdict{}
	}
	if len(lines) > 1 {
		var detail textbuf.Buffer
		detail.Str("lines ").Int(int64(lines[0].number)).Str(" and ").Int(int64(lines[1].number)).
			Str(" both start Captured:, and a class holds one capture")
		return captureVerdict{problem: &captureProblem{kind: ProblemCaptureDuplicated,
			line: lines[1].number, detail: detail.String()}}
	}
	parsed, err := parseCapture(lines[0].text, latest, lateReason)
	if err != nil {
		return captureVerdict{problem: &captureProblem{kind: ProblemCaptureMalformed,
			line: lines[0].number, detail: err.Error()}}
	}
	parsed.line = lines[0].number
	return captureVerdict{parsed: &parsed}
}

// parseCapture reads one Captured: line against its grammar, before any file
// is read: a pipe, a NUL, a "..", or an absolute path is refused here.
func parseCapture(text, latest, lateReason string) (parsedCapture, error) {
	if strings.ContainsRune(text, '|') {
		return parsedCapture{}, errors.New("the line carries a pipe, which every reader counts as a malformed row")
	}
	if strings.ContainsRune(text, 0) {
		return parsedCapture{}, errors.New("the line carries a NUL")
	}
	fields := strings.Fields(strings.TrimPrefix(strings.TrimSpace(text), capturePrefix))
	if len(fields) < 2 {
		return parsedCapture{}, errors.New("the line names no date and kind")
	}
	date := fields[0]
	if _, err := time.Parse(time.DateOnly, date); err != nil {
		return parsedCapture{}, captureError("date ", date, " is not YYYY-MM-DD")
	}
	if date > latest {
		return parsedCapture{}, captureError("date ", date, " is "+lateReason)
	}
	switch CaptureKind(fields[1]) {
	case CaptureCheck:
		return parseCheckCapture(date, fields[2:])
	case CaptureRule:
		return parseRuleCapture(date, fields[2:])
	case CaptureNone:
		return parsedCapture{}, errors.New("the kind is check or rule; an uncaptured class carries no Captured: line")
	default:
		// The kind is open text from a class file: refuse what the table lacks.
		return parsedCapture{}, captureError("kind ", fields[1], " is neither check nor rule")
	}
}

func parseCheckCapture(date string, rest []string) (parsedCapture, error) {
	if len(rest) != 3 {
		return parsedCapture{}, errors.New("a check capture is: DATE check DIR.Function proof DIR.TestFunction")
	}
	if rest[1] != "proof" {
		return parsedCapture{}, errors.New("a check capture names its proof test after the word proof")
	}
	check, err := parseFuncRef(rest[0])
	if err != nil {
		return parsedCapture{}, err
	}
	proof, err := parseFuncRef(rest[2])
	if err != nil {
		return parsedCapture{}, err
	}
	if !strings.HasPrefix(proof.name, "Test") {
		return parsedCapture{}, captureError("proof ", rest[2], " is not a Go test function")
	}
	return parsedCapture{
		capture: Capture{Date: date, Kind: CaptureCheck, Check: check.String(), Proof: proof.String()},
		check:   check,
		proof:   proof,
	}, nil
}

func parseRuleCapture(date string, rest []string) (parsedCapture, error) {
	if len(rest) != 1 {
		return parsedCapture{}, errors.New("a rule capture is: DATE rule POINT-ID")
	}
	point := rest[0]
	if !safeRepoPath(point) {
		return parsedCapture{}, captureError("point ", point, " is not a repo-relative point id")
	}
	if strings.HasSuffix(point, markdownExt) {
		return parsedCapture{}, captureError("point ", point, " is named without .md")
	}
	return parsedCapture{capture: Capture{Date: date, Kind: CaptureRule, Point: point}}, nil
}

// parseFuncRef reads DIR.Function, split at the last dot.
func parseFuncRef(text string) (funcRef, error) {
	dot := strings.LastIndexByte(text, '.')
	if dot <= 0 {
		return funcRef{}, captureError("reference ", text, " is not DIR.Function")
	}
	ref := funcRef{dir: text[:dot], name: text[dot+1:]}
	if !safeRepoPath(ref.dir) {
		return funcRef{}, captureError("reference ", text, " does not name a repo-relative directory")
	}
	if !goIdentifier(ref.name) {
		return funcRef{}, captureError("reference ", text, " does not end in a Go function name")
	}
	return ref, nil
}

// safeRepoPath accepts a slash-separated relative path of plain characters,
// with no "..", so a reference cannot leave the checkout.
func safeRepoPath(text string) bool {
	if text == "" {
		return false
	}
	if strings.HasPrefix(text, "/") {
		return false
	}
	if strings.Contains(text, "..") {
		return false
	}
	for _, one := range []byte(text) {
		switch {
		case one >= 'a' && one <= 'z', one >= 'A' && one <= 'Z', one >= '0' && one <= '9':
		case one == '/', one == '-', one == '_', one == '.':
		default:
			return false
		}
	}
	return true
}

func goIdentifier(name string) bool {
	for index, one := range []byte(name) {
		switch {
		case one >= 'a' && one <= 'z', one >= 'A' && one <= 'Z', one == '_':
		case index > 0 && one >= '0' && one <= '9':
		default:
			return false
		}
	}
	return name != ""
}

func captureError(lead, value, tail string) error {
	var tb textbuf.Buffer
	return errors.New(tb.Str(lead).Str(value).Str(tail).String())
}

// captureSource answers whether a tree declares what a capture names. The
// report's source reads git HEAD and the validator's reads the working tree.
type captureSource interface {
	// declares reports whether a .go file directly in ref.dir declares
	// ref.name as a function or a method.
	declares(ref funcRef) (bool, error)
	// hasPoint reports whether ai/rules/points/<id>.md exists.
	hasPoint(id string) (bool, error)
	// where names the tree in a refusal: "at HEAD", "in the working tree".
	where() string
}

// resolveCapture turns a parsed capture into a verdict against source.
func resolveCapture(verdict captureVerdict, source captureSource) (captureVerdict, error) {
	if verdict.parsed == nil {
		return verdict, nil
	}
	parsed := verdict.parsed
	if parsed.capture.Kind == CaptureRule {
		present, err := source.hasPoint(parsed.capture.Point)
		if err != nil {
			return captureVerdict{}, err
		}
		if !present {
			return unresolved(parsed, "point "+pointsDir+parsed.capture.Point+markdownExt+
				" does not exist "+source.where()), nil
		}
		return verdict, nil
	}
	for _, ref := range [...]funcRef{parsed.check, parsed.proof} {
		present, err := source.declares(ref)
		if err != nil {
			return captureVerdict{}, err
		}
		if !present {
			return unresolved(parsed, ref.String()+" is declared in no .go file of "+ref.dir+" "+source.where()), nil
		}
	}
	return verdict, nil
}

func unresolved(parsed *parsedCapture, detail string) captureVerdict {
	return captureVerdict{problem: &captureProblem{kind: ProblemCaptureUnresolved, line: parsed.line, detail: detail}}
}

// declaresFunc reports whether Go source declares name as a top-level function
// or a method, by its "func" line.
func declaresFunc(source, name string) bool {
	for line := range strings.SplitSeq(source, "\n") {
		rest, found := strings.CutPrefix(line, "func ")
		if !found {
			continue
		}
		if strings.HasPrefix(rest, "(") {
			end := strings.IndexByte(rest, ')')
			if end < 0 {
				continue
			}
			rest = strings.TrimLeft(rest[end+1:], " ")
		}
		after, named := strings.CutPrefix(rest, name)
		if !named {
			continue
		}
		if strings.HasPrefix(after, "(") {
			return true
		}
		if strings.HasPrefix(after, "[") {
			return true
		}
	}
	return false
}

// headCaptureSource answers from git HEAD. It is built once over every
// directory and point the parsed captures name: one ls-tree and one cat-file
// batch, whatever the number of captured classes.
type headCaptureSource struct {
	goFiles map[string][]string // directory -> content of each .go file directly in it
	points  map[string]bool
}

func newHeadCaptureSource(tree string, verdicts []captureVerdict) (headCaptureSource, error) {
	source := headCaptureSource{goFiles: map[string][]string{}, points: map[string]bool{}}
	var wanted []string
	for _, verdict := range verdicts {
		if verdict.parsed == nil {
			continue
		}
		parsed := verdict.parsed
		if parsed.capture.Kind == CaptureRule {
			wanted = append(wanted, pointsDir+parsed.capture.Point+markdownExt)
			continue
		}
		wanted = append(wanted, parsed.check.dir, parsed.proof.dir)
	}
	if len(wanted) == 0 {
		return source, nil
	}
	slices.Sort(wanted)
	wanted = slices.Compact(wanted)
	dirs := make(map[string]bool, len(wanted))
	for _, one := range wanted {
		dirs[one] = true
	}
	answer := runGit(tree, append([]string{"ls-tree", "--name-only", "-r", "HEAD", "--"}, wanted...)...)
	if answer.startErr != nil {
		return source, answer.startErr
	}
	if answer.code != 0 {
		return source, gitReadError("git ls-tree HEAD over the captured references failed: ", answer)
	}
	var goPaths []string
	for line := range strings.SplitSeq(string(answer.stdout), "\n") {
		listed := strings.TrimSpace(line)
		if strings.HasPrefix(listed, pointsDir) && strings.HasSuffix(listed, markdownExt) {
			source.points[strings.TrimSuffix(strings.TrimPrefix(listed, pointsDir), markdownExt)] = true
		}
		if strings.HasSuffix(listed, ".go") && dirs[path.Dir(listed)] {
			goPaths = append(goPaths, listed)
		}
	}
	contents, err := headFiles(tree, goPaths)
	if err != nil {
		return source, err
	}
	for _, goPath := range goPaths {
		source.goFiles[path.Dir(goPath)] = append(source.goFiles[path.Dir(goPath)], contents[goPath])
	}
	return source, nil
}

func (source headCaptureSource) declares(ref funcRef) (bool, error) {
	for _, content := range source.goFiles[ref.dir] {
		if declaresFunc(content, ref.name) {
			return true, nil
		}
	}
	return false, nil
}

func (source headCaptureSource) hasPoint(id string) (bool, error) { return source.points[id], nil }

func (headCaptureSource) where() string { return "at HEAD" }

// treeCaptureSource answers from the working tree under root. References are
// already safeRepoPath, so a join cannot leave root.
type treeCaptureSource struct {
	root string
}

func (source treeCaptureSource) declares(ref funcRef) (bool, error) {
	dir := filepath.Join(source.root, filepath.FromSlash(ref.dir))
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, entry.Name())) //nolint:gosec // a safeRepoPath directory under the checkout
		if err != nil {
			return false, err
		}
		if declaresFunc(string(content), ref.name) {
			return true, nil
		}
	}
	return false, nil
}

func (source treeCaptureSource) hasPoint(id string) (bool, error) {
	info, err := os.Lstat(filepath.Join(source.root, filepath.FromSlash(pointsDir+id+markdownExt)))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return info.Mode().IsRegular(), nil
}

func (treeCaptureSource) where() string { return "in the working tree" }

// headCommitDate is the HEAD commit's committer date, YYYY-MM-DD. The report
// refuses a capture dated after it, so its output depends on HEAD and never on
// the day it runs.
func headCommitDate(tree string) (string, error) {
	answer := runGit(tree, "log", "-1", "--format=%cs", "HEAD")
	if answer.startErr != nil {
		return "", answer.startErr
	}
	if answer.code != 0 {
		return "", gitReadError("git log -1 HEAD failed: ", answer)
	}
	date := strings.TrimSpace(string(answer.stdout))
	if _, err := time.Parse(time.DateOnly, date); err != nil {
		return "", captureError("git log -1 HEAD answered committer date ", date, ", not YYYY-MM-DD")
	}
	return date, nil
}
