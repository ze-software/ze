// Design: docs/architecture/core-design.md -- the rfc area, as one command
// Related: names.go -- the naming rule a rename target must pass
// Related: check_baseline.go -- the baselines that follow a byte-pure rename
//
// rename.go moves a tagged unit test file and carries its evidence with it.
// Every fingerprint in rfc/discrimination/ and rfc/audit/ is path-independent,
// so a byte-pure move keeps the evidence verified once the path KEYS are
// rewritten: nothing is re-stamped and nothing is re-judged.
package rfc

import (
	"bytes"
	"encoding/json"
	"errors"
	"go/build"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/doc/citation"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// renamePair is one move: both paths repo-relative and slash-separated.
type renamePair struct {
	Source string `json:"from"`
	Target string `json:"to"`
}

// RenameRewrite is one evidence file the rename rewrote and how many path
// keys and fields in it now name the new path.
type RenameRewrite struct {
	File string `json:"file"`
	Keys int    `json:"keys"`
}

// RenameReport is what `./le rfc rename` answers after it wrote.
//
// Moves, Evidence and Citations name only what was written. When a write failed
// part-way, Stopped carries the failure and Linked names each target created
// whose source is still in place, so the operator sees the tree as it is.
type RenameReport struct {
	Moves     []renamePair    `json:"moves"`
	Evidence  []RenameRewrite `json:"evidence"`
	Citations []string        `json:"citations"`
	Mentions  []string        `json:"mentions"`
	Stale     []string        `json:"stale"`
	Linked    []renamePair    `json:"linked,omitempty"`
	Stopped   string          `json:"stopped,omitempty"`
}

// renameNextStep is the command the derived ledgers need after a rename.
const renameNextStep = "./le rfc index-update"

// Text renders one line per move, rewrite, citation and mention, then the next step.
func (r RenameReport) Text() string {
	var tb textbuf.Buffer
	for _, move := range r.Moves {
		tb.Str("moved ").Str(move.Source).Str(" -> ").Str(move.Target).Byte('\n')
	}
	for _, rewrite := range r.Evidence {
		tb.Str("rewrote ").Str(rewrite.File).Str(": ").Int(int64(rewrite.Keys)).Str(" key(s)\n")
	}
	for _, where := range r.Citations {
		tb.Str("citation rewritten: ").Str(where).Byte('\n')
	}
	for _, where := range r.Mentions {
		tb.Str("plain mention left unchanged, read it: ").Str(where).Byte('\n')
	}
	for _, where := range r.Stale {
		tb.Str("citation still names the old path, edit it by hand: ").Str(where).Byte('\n')
	}
	for _, link := range r.Linked {
		tb.Str("created ").Str(link.Target).Str(", source still in place: ").Str(link.Source).Byte('\n')
	}
	if r.Stopped != "" {
		tb.Str("stopped part-way, nothing above this line was undone: ").Str(r.Stopped).Byte('\n')
	}
	return tb.Str("next: ").Str(renameNextStep).Byte('\n').String()
}

// renameFile is one tracked file the rename rewrites, held with the bytes it
// was read with so a concurrent edit is refused rather than clobbered.
type renameFile struct {
	Rel      string
	Original []byte
	Updated  []byte
	Keys     int
}

// renamePlan is every write one rename makes, decided before any is made.
type renamePlan struct {
	Pairs     []renamePair
	Evidence  []renameFile
	Cited     []renameFile
	Citations []string
	Mentions  []string
	Stale     []string
}

// renameFiles plans and applies a batch of moves: every refusal is judged for
// every pair first, and one refusal anywhere writes nothing.
func renameFiles(tree string, pairs []renamePair) (RenameReport, error) {
	plan, err := planRename(tree, pairs)
	if err != nil {
		return RenameReport{}, err
	}
	return applyRename(tree, plan)
}

// planRename judges every pair and computes every rewrite, and writes nothing.
func planRename(tree string, pairs []renamePair) (renamePlan, error) {
	if len(pairs) == 0 {
		return renamePlan{}, errors.New("rfc rename: no pair to move")
	}
	var refusals []string
	refusals = append(refusals, refuseBatchShape(pairs)...)
	collected, err := Collect(tree)
	if err != nil {
		return renamePlan{}, err
	}
	stems, err := summaryStems(tree)
	if err != nil {
		return renamePlan{}, err
	}
	stemsByFile := tagStemsByFile(slices.Concat(collected.Tags, collected.GapTags), collected.Requirements, stems)
	platforms, err := goPlatforms()
	if err != nil {
		return renamePlan{}, err
	}
	table, err := carriers(tree)
	if err != nil {
		return renamePlan{}, err
	}
	judged := renameJudgement{TagStems: stemsByFile, Stems: stems, Carriers: table, Platforms: platforms}
	for _, pair := range pairs {
		refusals = append(refusals, refusePair(tree, pair, judged)...)
	}
	if len(refusals) > 0 {
		return renamePlan{}, errors.New(strings.Join(refusals, "\n"))
	}
	plan := renamePlan{Pairs: pairs}
	if plan.Evidence, err = planEvidence(tree, pairs); err != nil {
		return renamePlan{}, err
	}
	if err := planCitations(tree, &plan); err != nil {
		return renamePlan{}, err
	}
	return plan, nil
}

// refuseBatchShape refuses a pair that names one path twice, two pairs moving
// one source, and two pairs naming one target.
func refuseBatchShape(pairs []renamePair) []string {
	var out []string
	sources, targets := map[string]bool{}, map[string]bool{}
	for _, pair := range pairs {
		var tb textbuf.Buffer
		if sources[pair.Source] {
			out = append(out, tb.Str(pair.Source).Str(": named as a source by two pairs").String())
		}
		tb.Reset()
		if targets[pair.Target] {
			out = append(out, tb.Str(pair.Target).Str(": named as the target of two pairs").String())
		}
		sources[pair.Source], targets[pair.Target] = true, true
	}
	return out
}

// renameJudgement is what every pair of one batch is judged against.
type renameJudgement struct {
	// TagStems is read by refusePair to judge a pair's source; the naming
	// check leaves it unset, because each of its verdicts carries its source.
	TagStems  map[string]map[string]bool
	Stems     map[string]bool
	Carriers  []Carrier
	Platforms []goPlatform
}

// refusePair answers every reason one pair is refused, each naming its path.
func refusePair(tree string, pair renamePair, judged renameJudgement) []string {
	var out []string
	refuse := func(rel, why string) {
		var tb textbuf.Buffer
		out = append(out, tb.Str(rel).Str(": ").Str(why).String())
	}
	for _, rel := range []string{pair.Source, pair.Target} {
		if !cleanRepoPath(rel) {
			refuse(rel, "not a clean path inside the checkout")
			return out
		}
		if !strings.HasSuffix(rel, testFileSuffix) {
			refuse(rel, "not a _test.go file")
		}
	}
	if path.Dir(pair.Source) != path.Dir(pair.Target) {
		refuse(pair.Target, "is in another directory than its source; a rename stays in its package")
	}
	if why := sourceDiffersFromHead(tree, pair.Source); why != "" {
		refuse(pair.Source, why)
	}
	src, err := os.ReadFile(treePath(tree, pair.Source)) // #nosec G304 -- a cleaned repo-relative path
	if err != nil {
		refuse(pair.Source, "cannot be read")
		return out
	}
	source := namedTestFile{Rel: pair.Source, TagStems: judged.TagStems[pair.Source],
		Marker: readNamingMarker(string(src))}
	refusal, err := judgeRenameTarget(tree, pair.Target, source, judged)
	if err != nil {
		refuse(pair.Target, err.Error())
		return out
	}
	if refusal.MovesPlatforms {
		refuse(pair.Target, "changes the GOOS/GOARCH file-name suffix, so another set of platforms would compile it")
	}
	if refusal.Taken {
		refuse(pair.Target, "already exists")
	}
	if refusal.Misnamed {
		var tb textbuf.Buffer
		why := tb.Str("fails the test file naming rule: ").Str(refusal.Naming.Problem)
		if refusal.Naming.Target != "" {
			why.Str("; expected ").Str(refusal.Naming.Target)
		}
		refuse(pair.Target, why.String())
	}
	return out
}

// targetRefusal is every reason `./le rfc rename` refuses one target that
// judgeRenameTarget decides. Each is a named outcome, so an accepted target is
// one with every field false rather than an empty reason.
type targetRefusal struct {
	// Taken is set when a file already holds the target's name.
	Taken bool
	// MovesPlatforms is set when the target's GOOS/GOARCH suffix compiles the
	// file on other platforms than the source's name does.
	MovesPlatforms bool
	// Misnamed is set when the naming rule refuses the target, and Naming is
	// then its verdict.
	Misnamed bool
	Naming   nameVerdict
}

// judgeRenameTarget is the one predicate for whether a target may take a
// source's place: refusePair applies it to every pair, and repairBlocked to
// every rename a finding would name, so a finding never suggests a target the
// rename refuses. The target is judged with the source's tags and marker,
// because the move carries both.
//
// The naming rule judges a target only where the check judges it: a file
// CarrierFor holds as a unit carrier (testFileNameVerdicts). A file the check
// never names cannot be refused a name the check would accept.
func judgeRenameTarget(tree, target string, source namedTestFile, judged renameJudgement) (targetRefusal, error) {
	var refusal targetRefusal
	moves, err := buildSuffixMoves(path.Base(source.Rel), path.Base(target), judged.Platforms)
	if err != nil {
		return targetRefusal{}, err
	}
	refusal.MovesPlatforms = moves
	if _, statErr := os.Lstat(treePath(tree, target)); statErr == nil {
		refusal.Taken = true
	}
	if carrier, held := CarrierFor(target, judged.Carriers); !held || carrier.Kind != kindUnit {
		return refusal, nil
	}
	moved := namedTestFile{Rel: target, TagStems: source.TagStems, Marker: source.Marker}
	refusal.Naming, refusal.Misnamed = judgeTestFileName(moved, judged.Stems)
	return refusal, nil
}

// cleanRepoPath reports whether rel is a clean, relative, slash-separated path
// that stays inside the checkout.
func cleanRepoPath(rel string) bool {
	if rel == "" || path.IsAbs(rel) || filepath.IsAbs(rel) {
		return false
	}
	if path.Clean(rel) != rel || strings.Contains(rel, "\\") {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, "../")
}

// sourceDiffersFromHead answers why a source may not move, or empty when it is
// tracked and its bytes equal HEAD's: a move of an edited file is not byte-pure,
// and the edit belongs to whoever made it.
func sourceDiffersFromHead(tree, rel string) string {
	var spec textbuf.Buffer
	head, ok := gitOutput(tree, "rev-parse", "--verify", "--quiet", spec.Str(headRevision).Byte(':').Str(rel).String())
	if !ok {
		return "is not tracked at HEAD"
	}
	work, ok := gitOutput(tree, "hash-object", "--", rel)
	if !ok {
		return "cannot be hashed"
	}
	if !bytes.Equal(bytes.TrimSpace(head), bytes.TrimSpace(work)) {
		return "differs from HEAD; commit or leave the edit before moving it"
	}
	return ""
}

// planEvidence rewrites every path key and field naming a source in the
// discrimination records and the audit verdicts.
//
// The rewrite is by field and in place: in a discrimination file the `unit` and
// `producer` of each record, in an audit file each key of a requirement's
// `tests`, `units` and `code` maps, when it equals the source or opens with the
// source then `::`. Every other string keeps its bytes, however it reads, and so
// does the formatting and every other byte, another session's hunks included.
func planEvidence(tree string, pairs []renamePair) ([]renameFile, error) {
	var out []renameFile
	for dir, fields := range map[string]func(evidenceString) bool{
		discriminationRel: discriminationPathField, auditRel: auditPathKey,
	} {
		entries, err := os.ReadDir(treePath(tree, dir))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			var tb textbuf.Buffer
			return nil, parseErr(tb.Str(dir).Str(": cannot read: ").Err(err))
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), jsonSuffix) {
				continue
			}
			rel := path.Join(dir, entry.Name())
			original, err := os.ReadFile(treePath(tree, rel)) // #nosec G304 -- a listed evidence file
			if err != nil {
				var tb textbuf.Buffer
				return nil, parseErr(tb.Str(rel).Str(": cannot read: ").Err(err))
			}
			updated, keys, err := rewriteEvidencePaths(original, pairs, fields)
			if err != nil {
				var tb textbuf.Buffer
				return nil, parseErr(tb.Str(rel).Str(": ").Err(err))
			}
			if keys > 0 {
				out = append(out, renameFile{Rel: rel, Original: original, Updated: updated, Keys: keys})
			}
		}
	}
	slices.SortFunc(out, func(a, b renameFile) int { return strings.Compare(a.Rel, b.Rel) })
	return out, nil
}

// discriminationPathField holds a record's `unit` and `producer` values.
func discriminationPathField(found evidenceString) bool {
	if found.Key || len(found.Path) != 3 {
		return false
	}
	if found.Path[0] != "records" || found.Path[1] != evidenceArrayStep {
		return false
	}
	return found.Path[2] == "unit" || found.Path[2] == "producer"
}

// auditPathKey holds the keys of a requirement's `tests`, `units` and `code`
// fingerprint maps.
func auditPathKey(found evidenceString) bool {
	if !found.Key || len(found.Path) != 3 || found.Path[0] != "requirements" {
		return false
	}
	switch found.Path[2] {
	case "tests", "units", fingerprintCode:
		return true
	}
	return false
}

// evidenceArrayStep stands in an evidenceString path for one array element.
const evidenceArrayStep = "[]"

// evidenceString is one JSON string of an evidence file: the chain of object
// keys (and evidenceArrayStep for an array element) that leads to it, whether it
// is itself an object key, its decoded value, and the offset just past it.
type evidenceString struct {
	Path  []string
	Key   bool
	Value string
	End   int
}

// evidenceFrame is one open object or array while evidenceStrings walks.
type evidenceFrame struct {
	Object  bool
	WantKey bool
	Key     string
}

// evidenceStrings answers every string of text in order, keys included, and an
// error when text is not one JSON value.
func evidenceStrings(text []byte) ([]evidenceString, error) {
	decoder := json.NewDecoder(bytes.NewReader(text))
	decoder.UseNumber()
	var stack []evidenceFrame
	var out []evidenceString
	values := 0
	for {
		token, err := decoder.Token()
		// The decoder answers io.EOF between two tokens at any depth, and reads a
		// stream of values, so a truncated file and a second value are caught here.
		if errors.Is(err, io.EOF) {
			if len(stack) != 0 {
				return nil, errors.New("the input ends inside an object or array")
			}
			if values != 1 {
				var tb textbuf.Buffer
				return nil, errors.New(tb.Str("the input holds ").Int(int64(values)).Str(" values, want one").String())
			}
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		if delim, ok := token.(json.Delim); ok {
			if delim == '{' || delim == '[' {
				stack = append(stack, evidenceFrame{Object: delim == '{', WantKey: delim == '{'})
				continue
			}
			stack = stack[:len(stack)-1]
		}
		if value, ok := token.(string); ok {
			key := len(stack) > 0 && stack[len(stack)-1].WantKey
			out = append(out, evidenceString{Path: evidencePath(stack, key), Key: key, Value: value,
				End: int(decoder.InputOffset())})
			if key {
				stack[len(stack)-1].Key, stack[len(stack)-1].WantKey = value, false
				continue
			}
		}
		// A value just ended: at the top it is the whole document, and inside an
		// object the object expects its next key.
		if len(stack) == 0 {
			values++
			continue
		}
		if stack[len(stack)-1].Object {
			stack[len(stack)-1].WantKey = true
		}
	}
}

// evidencePath answers the chain leading to the next string; a key's chain
// stops at the object holding it.
func evidencePath(stack []evidenceFrame, key bool) []string {
	frames := stack
	if key {
		frames = stack[:len(stack)-1]
	}
	out := make([]string, 0, len(stack))
	for _, frame := range frames {
		if frame.Object {
			out = append(out, frame.Key)
			continue
		}
		out = append(out, evidenceArrayStep)
	}
	return out
}

// rewriteEvidencePaths answers text with every string fields holds that names a
// source moved to its target, and how many it moved. A matching string whose
// bytes are not its plain quoted value (an escape inside it) is refused rather
// than rewritten in a second spelling.
func rewriteEvidencePaths(text []byte, pairs []renamePair, fields func(evidenceString) bool) ([]byte, int, error) {
	found, err := evidenceStrings(text)
	if err != nil {
		return nil, 0, errors.New("not JSON the rename can rewrite: " + err.Error())
	}
	var out []byte
	written, keys := 0, 0
	for _, candidate := range found {
		if !fields(candidate) {
			continue
		}
		moved, ok := movedEvidencePath(candidate.Value, pairs)
		if !ok {
			continue
		}
		start := candidate.End - len(candidate.Value) - 2
		if start < written || string(text[start:candidate.End]) != `"`+candidate.Value+`"` {
			return nil, 0, errors.New("the path " + candidate.Value + " is escaped, so it is not rewritten in place")
		}
		out = append(out, text[written:start]...)
		out = append(out, '"')
		out = append(out, moved...)
		out = append(out, '"')
		written = candidate.End
		keys++
	}
	if keys == 0 {
		return text, 0, nil
	}
	return append(out, text[written:]...), keys, nil
}

// movedEvidencePath answers value with its source path replaced by the target,
// when value is a source or a source then `::`.
func movedEvidencePath(value string, pairs []renamePair) (string, bool) {
	for _, pair := range pairs {
		if value == pair.Source {
			return pair.Target, true
		}
		if rest, scoped := strings.CutPrefix(value, pair.Source+"::"); scoped {
			return pair.Target + "::" + rest, true
		}
	}
	return "", false
}

// planCitations rewrites every citation of a source in a tracked file the link
// sweep polices (citation.Policed), through the sweep's own grammar
// (citation.Paths), and lists every other line of such a file that mentions a
// source's base name. A record tree (a journal row, a weakened shard, a spec)
// is left byte-identical and unlisted, because its path is a fact about the
// day it was written.
//
// A citation the grammar expands from braces (`dir/widget{_test,}.go`) holds no
// literal source path to replace, so after the rewrite every line that still
// cites a source is listed as stale, for a reader to edit.
//
// A moved file is never rewritten, and neither is an evidence file, which
// planEvidence owns: a citation inside a moved file is listed instead, because
// the move is byte-pure or it is not followed.
func planCitations(tree string, plan *renamePlan) error {
	files, err := trackedFilesMentioning(tree, plan.Pairs)
	if err != nil {
		return err
	}
	moved := map[string]bool{}
	for _, pair := range plan.Pairs {
		moved[pair.Source] = true
	}
	for _, rel := range files {
		if isEvidenceFile(rel) {
			continue
		}
		// A record tree keeps the path it was written with, so a file the link
		// sweep does not police is neither rewritten nor listed.
		if !citation.Policed(rel) {
			continue
		}
		original, err := os.ReadFile(treePath(tree, rel)) // #nosec G304 -- a tracked file git named
		if err != nil {
			var tb textbuf.Buffer
			return parseErr(tb.Str(rel).Str(": cannot read: ").Err(err))
		}
		lines := strings.Split(string(original), "\n")
		changed := false
		for index, line := range lines {
			rewritten := line
			if !moved[rel] {
				rewritten = rewriteLineCitations(tree, line, plan.Pairs)
			}
			var where textbuf.Buffer
			where.Str(rel).Byte(':').Int(int64(index + 1))
			if !moved[rel] && citesAnySource(tree, rewritten, plan.Pairs) {
				plan.Stale = append(plan.Stale, where.String())
			}
			if rewritten != line {
				lines[index], changed = rewritten, true
				plan.Citations = append(plan.Citations, where.String())
				continue
			}
			if mentionsAny(line, plan.Pairs) {
				plan.Mentions = append(plan.Mentions, where.String())
			}
		}
		if changed {
			plan.Cited = append(plan.Cited, renameFile{Rel: rel, Original: original,
				Updated: []byte(strings.Join(lines, "\n"))})
		}
	}
	return nil
}

// rewriteLineCitations answers line with each occurrence of a source that the
// citation grammar reads as a citation replaced by its target. An occurrence is
// a citation when replacing it removes one citation of the source; a plain
// mention on the same line stays as it was.
func rewriteLineCitations(tree, line string, pairs []renamePair) string {
	for _, pair := range pairs {
		start := 0
		for {
			offset := strings.Index(line[start:], pair.Source)
			if offset < 0 {
				break
			}
			at := start + offset
			candidate := line[:at] + pair.Target + line[at+len(pair.Source):]
			if citationCount(tree, candidate, pair.Source) < citationCount(tree, line, pair.Source) {
				line = candidate
				start = at + len(pair.Target)
				continue
			}
			start = at + len(pair.Source)
		}
	}
	return line
}

func citationCount(tree, line, rel string) int {
	count := 0
	for _, cited := range citation.Paths(tree, line) {
		if cited == rel {
			count++
		}
	}
	return count
}

// citesAnySource reports whether the citation grammar reads line as citing a
// source. Only a line holding a brace can, once rewriteLineCitations replaced
// every literal citation, so no other line pays for the grammar.
func citesAnySource(tree, line string, pairs []renamePair) bool {
	if !strings.Contains(line, "{") {
		return false
	}
	for _, pair := range pairs {
		if citationCount(tree, line, pair.Source) > 0 {
			return true
		}
	}
	return false
}

func mentionsAny(line string, pairs []renamePair) bool {
	for _, pair := range pairs {
		if strings.Contains(line, path.Base(pair.Source)) {
			return true
		}
	}
	return false
}

func isEvidenceFile(rel string) bool {
	return strings.HasPrefix(rel, discriminationRel+"/") || strings.HasPrefix(rel, auditRel+"/")
}

// trackedFilesMentioning answers every tracked file under the checkout whose
// working-tree text holds a source's base name or its directory, vendor/
// excluded: the directory is what a brace citation of the source still spells. git grep
// answers 1 for no match and anything else above it for a failure, and the
// two are told apart, because "no file mentions it" is an answer and a failed
// search is not.
func trackedFilesMentioning(tree string, pairs []renamePair) ([]string, error) {
	args := make([]string, 0, 5+4*len(pairs)+3)
	args = append(args, "grep", "-l", "-z", "-I", "-F")
	for _, pair := range pairs {
		args = append(args, "-e", path.Base(pair.Source), "-e", path.Dir(pair.Source)+"/")
	}
	args = append(args, "--", ".", ":(exclude)vendor")
	cmd := exec.Command("git", args...) //nolint:gosec,noctx // this developer tool searches the checkout it was given
	cmd.Dir = tree
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			return nil, nil
		}
		return nil, errors.New("rfc rename: cannot search the tracked files: " + err.Error())
	}
	var files []string
	for entry := range strings.SplitSeq(string(out), "\x00") {
		if entry != "" {
			files = append(files, entry)
		}
	}
	return files, nil
}

// applyRename re-reads every file the plan rewrites and refuses the whole
// batch, writing nothing, when another session changed one since it was read.
// It then links every target, removes every source, and replaces each
// rewritten file atomically.
//
// A target is created by link, never by rename, so an existing file is never
// overwritten: a link onto an existing path fails.
func applyRename(tree string, plan renamePlan) (RenameReport, error) {
	for _, file := range slices.Concat(plan.Evidence, plan.Cited) {
		current, err := os.ReadFile(treePath(tree, file.Rel)) // #nosec G304 -- a file this plan read
		if err != nil || !bytes.Equal(current, file.Original) {
			var tb textbuf.Buffer
			return RenameReport{}, errors.New(tb.Str(file.Rel).
				Str(": changed since the rename read it; nothing was written, run the rename again").String())
		}
	}
	var linked []string
	for _, pair := range plan.Pairs {
		if err := os.Link(treePath(tree, pair.Source), treePath(tree, pair.Target)); err != nil {
			for _, rel := range linked {
				_ = os.Remove(treePath(tree, rel)) //nolint:errcheck // undoing this run's own link
			}
			var tb textbuf.Buffer
			return RenameReport{}, errors.New(tb.Str(pair.Target).Str(": cannot create: ").Err(err).
				Str("; nothing was written").String())
		}
		linked = append(linked, pair.Target)
	}
	report := RenameReport{Mentions: plan.Mentions, Stale: plan.Stale}
	for index, pair := range plan.Pairs {
		if err := os.Remove(treePath(tree, pair.Source)); err != nil {
			// Reachable only through a race: the link above needed write access to
			// this directory, which a remove needs too, so no test drives this arm.
			report.Linked = plan.Pairs[index:]
			return stopRename(report, err)
		}
		report.Moves = append(report.Moves, pair)
	}
	for _, file := range plan.Evidence {
		if err := replaceFileAtomically(treePath(tree, file.Rel), file.Updated); err != nil {
			return stopRename(report, err)
		}
		report.Evidence = append(report.Evidence, RenameRewrite{File: file.Rel, Keys: file.Keys})
	}
	for _, file := range plan.Cited {
		if err := replaceFileAtomically(treePath(tree, file.Rel), file.Updated); err != nil {
			return stopRename(report, err)
		}
		prefix := file.Rel + ":"
		for _, where := range plan.Citations {
			if strings.HasPrefix(where, prefix) {
				report.Citations = append(report.Citations, where)
			}
		}
	}
	return report, nil
}

// stopRename answers the report of the writes made before err, marked as
// stopped, with err: a rename that failed part-way has changed the tree, and
// dropping the report would hide which part.
func stopRename(report RenameReport, err error) (RenameReport, error) {
	var tb textbuf.Buffer
	report.Stopped = tb.Err(err).String()
	return report, errors.New("rfc rename: stopped part-way: " + report.Stopped)
}

// replaceFileAtomically writes body beside target and renames it over target,
// so a reader sees the old file or the new one, never half of either.
func replaceFileAtomically(target string, body []byte) error {
	info, err := os.Stat(target)
	if err != nil {
		return err
	}
	staged, err := os.CreateTemp(filepath.Dir(target), ".rename-*.tmp")
	if err != nil {
		return err
	}
	name := staged.Name()
	defer func() { _ = os.Remove(name) }() //nolint:errcheck // gone after the rename succeeds
	if _, err := staged.Write(body); err != nil {
		_ = staged.Close() //nolint:errcheck // the write error is the one reported
		return err
	}
	if err := staged.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, info.Mode().Perm()); err != nil {
		return err
	}
	return os.Rename(name, target)
}

// goPlatform is one GOOS/GOARCH pair the toolchain builds for.
type goPlatform struct {
	OS   string
	Arch string
}

// goPlatforms asks the toolchain for its platform list rather than holding a
// copy of it, so a new port is known the day the toolchain knows it.
func goPlatforms() ([]goPlatform, error) {
	out, err := exec.Command("go", "tool", "dist", "list").Output() //nolint:gosec,noctx // fixed argv
	if err != nil {
		return nil, errors.New("rfc rename: cannot list the Go platforms: " + err.Error())
	}
	var platforms []goPlatform
	for line := range strings.SplitSeq(string(out), "\n") {
		goos, goarch, found := strings.Cut(strings.TrimSpace(line), "/")
		if found {
			platforms = append(platforms, goPlatform{OS: goos, Arch: goarch})
		}
	}
	if len(platforms) == 0 {
		return nil, errors.New("rfc rename: the Go platform list is empty")
	}
	return platforms, nil
}

// platformsBuilding answers, for each platform in order, whether go/build
// compiles a Go file named base there, judging the name alone.
//
// go/build itself decides, through build.Context.MatchFile, so its own list of
// known GOOS and GOARCH names applies: that list is wider than the toolchain's
// ports (`_sparc` and `_zos` constrain a file no port builds), and it carries
// the implied names (`_linux` builds for android, `_solaris` for illumos).
// OpenFile answers a bare package clause, so no build tag is read and the file
// need not exist.
func platformsBuilding(base string, platforms []goPlatform) ([]bool, error) {
	out := make([]bool, len(platforms))
	for index, platform := range platforms {
		judge := build.Default
		judge.GOOS, judge.GOARCH = platform.OS, platform.Arch
		judge.BuildTags, judge.ToolTags, judge.ReleaseTags = nil, nil, nil
		judge.UseAllFiles = false
		judge.OpenFile = func(string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("package p\n")), nil
		}
		match, err := judge.MatchFile(".", base)
		if err != nil {
			return nil, errors.New("rfc rename: go/build cannot judge " + base + ": " + err.Error())
		}
		out[index] = match
	}
	return out, nil
}

// buildSuffixMoves reports whether renaming source to target changes the set
// of platforms that compile the file.
func buildSuffixMoves(source, target string, platforms []goPlatform) (bool, error) {
	before, err := platformsBuilding(source, platforms)
	if err != nil {
		return false, err
	}
	after, err := platformsBuilding(target, platforms)
	if err != nil {
		return false, err
	}
	return !slices.Equal(before, after), nil
}

// ProposeReport is what `./le rfc rename propose` answers.
type ProposeReport struct {
	Plan       string   `json:"plan"`
	Pairs      int      `json:"pairs"`
	Collisions []string `json:"collisions"`
	Mismatches []string `json:"mismatches"`
}

// Text renders the plan path and count, then every file left out and why.
func (r ProposeReport) Text() string {
	var tb textbuf.Buffer
	tb.Str("wrote ").Int(int64(r.Pairs)).Str(" pair(s) to ").Str(r.Plan).Byte('\n')
	for _, line := range r.Collisions {
		tb.Str("left out, the rename would refuse it: ").Str(line).Byte('\n')
	}
	for _, line := range r.Mismatches {
		tb.Str("left out, named for another RFC (read it before renaming): ").Str(line).Byte('\n')
	}
	return tb.Str("next: ./le rfc rename plan ").Str(r.Plan).Byte('\n').String()
}

// proposeRenames writes a plan of one pair per part (b) finding of the naming
// rule under the directory `under` (the whole tree when empty), each target the
// exact rename the finding names.
//
// Left out and reported: a target the rename would refuse or that two findings
// share, judged by repairBlocked through judgeRenameTarget, the predicate
// refusePair applies, so the plan never holds a pair that refuses its batch;
// and a file already named for ANOTHER stem, whose name and tags disagree and
// must be read before anything moves. The output is created, never overwritten.
func proposeRenames(tree, under, output string) (ProposeReport, error) {
	if under != "" && !cleanRepoPath(under) {
		return ProposeReport{}, errors.New("rfc rename propose: under must be a clean directory inside the checkout")
	}
	collected, err := Collect(tree)
	if err != nil {
		return ProposeReport{}, err
	}
	stems, err := summaryStems(tree)
	if err != nil {
		return ProposeReport{}, err
	}
	table, err := carriers(tree)
	if err != nil {
		return ProposeReport{}, err
	}
	verdicts, err := testFileNameVerdicts(tree, table, slices.Concat(collected.Tags, collected.GapTags),
		collected.Requirements, stems)
	if err != nil {
		return ProposeReport{}, err
	}
	report := ProposeReport{Plan: output}
	var candidates []nameVerdict
	shared := map[string]int{}
	for _, verdict := range verdicts {
		if verdict.Target == "" || !underDirectory(verdict.Rel, under) {
			continue
		}
		if stemOfFileName(path.Base(verdict.Rel), stems) != "" {
			report.Mismatches = append(report.Mismatches, verdict.Rel+" -> "+verdict.Target)
			continue
		}
		candidates = append(candidates, verdict)
		shared[verdict.Target]++
	}
	// TagStems stays unset: each verdict carries its own source's tags. The
	// platforms are listed only when a candidate needs judging, because listing
	// them runs the toolchain.
	judged := renameJudgement{Stems: stems, Carriers: table}
	if len(candidates) > 0 {
		if judged.Platforms, err = goPlatforms(); err != nil {
			return ProposeReport{}, err
		}
	}
	var body textbuf.Buffer
	for _, verdict := range candidates {
		blocked, err := repairBlocked(tree, verdict, verdict.Target, shared[verdict.Target], judged)
		if err != nil {
			return ProposeReport{}, err
		}
		if blocked != "" {
			var line textbuf.Buffer
			line.Str(verdict.Rel).Str(" -> ").Str(verdict.Target).Str(": ").Str(blocked)
			report.Collisions = append(report.Collisions, line.String())
			continue
		}
		body.Str(verdict.Rel).Byte(' ').Str(verdict.Target).Byte('\n')
		report.Pairs++
	}
	if err := createPlanFile(tree, output, body.String()); err != nil {
		return ProposeReport{}, err
	}
	return report, nil
}

func underDirectory(rel, under string) bool {
	return under == "" || strings.HasPrefix(rel, strings.TrimSuffix(under, "/")+"/")
}

// createPlanFile writes a plan file that must not exist yet.
func createPlanFile(tree, output, body string) error {
	file, err := os.OpenFile(resolvePlanPath(tree, output), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return errors.New("rfc rename propose: cannot create the plan file: " + err.Error())
	}
	if _, err := file.WriteString(body); err != nil {
		_ = file.Close() //nolint:errcheck // the write error is the one reported
		return err
	}
	return file.Close()
}

// resolvePlanPath answers a plan file's path: absolute as given, otherwise
// under the checkout.
func resolvePlanPath(tree, rel string) string {
	if filepath.IsAbs(rel) {
		return rel
	}
	return filepath.Join(tree, filepath.FromSlash(rel))
}

// readRenamePlan reads one `<from> <to>` pair per line; a blank line and a
// line opening with `#` are skipped, and any other shape is refused.
func readRenamePlan(tree, rel string) ([]renamePair, error) {
	raw, err := os.ReadFile(resolvePlanPath(tree, rel)) // #nosec G304 -- the plan the caller named
	if err != nil {
		return nil, errors.New("rfc rename: cannot read the plan file: " + err.Error())
	}
	var pairs []renamePair
	for index, line := range strings.Split(string(raw), "\n") {
		text := strings.TrimSpace(line)
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		fields := strings.Fields(text)
		if len(fields) != 2 {
			var tb textbuf.Buffer
			return nil, errors.New(tb.Str(rel).Byte(':').Int(int64(index + 1)).
				Str(": want `<from> <to>`, got: ").Str(text).String())
		}
		pairs = append(pairs, renamePair{Source: fields[0], Target: fields[1]})
	}
	return pairs, nil
}

// The keywords `./le rfc rename` adds to the grammar; from is audit-stamp's.
const (
	keyTo      = "to"
	keyPlan    = "plan"
	keyPropose = "propose"
	keyUnder   = "under"
)

// renameAnswer dispatches the three exclusive forms of `./le rfc rename`:
// from with to, plan, or propose. Naming two forms, or none, is refused.
func renameAnswer(args leaction.Arguments) (any, int) {
	forms := 0
	for _, keyword := range []string{keyFrom, keyPlan, keyPropose} {
		if args.Has(keyword) {
			forms++
		}
	}
	if forms != 1 || args.Has(keyFrom) != args.Has(keyTo) {
		leaction.ReportError(errors.New("rfc rename takes exactly one of: from <old> to <new>, plan <file>, propose <file>"))
		return nil, 2
	}
	if args.Has(keyUnder) && !args.Has(keyPropose) {
		leaction.ReportError(errors.New("rfc rename: under narrows propose only"))
		return nil, 2
	}
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	if args.Has(keyPropose) {
		report, err := proposeRenames(tree, args.One(keyUnder), args.One(keyPropose))
		if err != nil {
			leaction.ReportError(err)
			return nil, 2
		}
		return report, 0
	}
	pairs := []renamePair{{Source: args.One(keyFrom), Target: args.One(keyTo)}}
	if args.Has(keyPlan) {
		if pairs, err = readRenamePlan(tree, args.One(keyPlan)); err != nil {
			leaction.ReportError(err)
			return nil, 2
		}
	}
	report, err := renameFiles(tree, pairs)
	if err != nil {
		leaction.ReportError(err)
		if report.Stopped != "" {
			return report, 2
		}
		return nil, 2
	}
	return report, 0
}
