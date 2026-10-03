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
	"errors"
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
type RenameReport struct {
	Moves     []renamePair    `json:"moves"`
	Evidence  []RenameRewrite `json:"evidence"`
	Citations []string        `json:"citations"`
	Mentions  []string        `json:"mentions"`
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
	for _, pair := range pairs {
		refusals = append(refusals, refusePair(tree, pair, stemsByFile[pair.Source], stems, platforms)...)
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

// refusePair answers every reason one pair is refused, each naming its path.
func refusePair(tree string, pair renamePair, tagStems, stems map[string]bool, platforms goPlatformSet) []string {
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
	if platforms.suffix(path.Base(pair.Source)) != platforms.suffix(path.Base(pair.Target)) {
		refuse(pair.Target, "changes the GOOS/GOARCH file-name suffix, so other platforms would compile it")
	}
	if _, err := os.Lstat(treePath(tree, pair.Target)); err == nil {
		refuse(pair.Target, "already exists")
	}
	if why := sourceDiffersFromHead(tree, pair.Source); why != "" {
		refuse(pair.Source, why)
	}
	if len(out) > 0 {
		return out
	}
	src, err := os.ReadFile(treePath(tree, pair.Source)) // #nosec G304 -- a cleaned repo-relative path
	if err != nil {
		refuse(pair.Source, "cannot be read")
		return out
	}
	file := namedTestFile{Rel: pair.Target, TagStems: tagStems, Marker: readNamingMarker(string(src))}
	if verdict, refused := judgeTestFileName(file, stems); refused {
		var tb textbuf.Buffer
		why := tb.Str("fails the test file naming rule: ").Str(verdict.Problem)
		if verdict.Target != "" {
			why.Str("; expected ").Str(verdict.Target)
		}
		refuse(pair.Target, why.String())
	}
	return out
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
// The rewrite is textual and exact: a JSON string equal to the source, or
// opening with the source then `::`. Formatting and every other byte, another
// session's hunks included, stay where they are.
func planEvidence(tree string, pairs []renamePair) ([]renameFile, error) {
	var out []renameFile
	for _, dir := range []string{discriminationRel, auditRel} {
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
			updated, keys := rewriteEvidencePaths(original, pairs)
			if keys > 0 {
				out = append(out, renameFile{Rel: rel, Original: original, Updated: updated, Keys: keys})
			}
		}
	}
	return out, nil
}

// rewriteEvidencePaths answers text with every quoted path key moved, and how
// many it moved.
func rewriteEvidencePaths(text []byte, pairs []renamePair) ([]byte, int) {
	keys := 0
	for _, pair := range pairs {
		for _, suffix := range []string{`"`, `::`} {
			from := []byte(`"` + pair.Source + suffix)
			keys += bytes.Count(text, from)
			text = bytes.ReplaceAll(text, from, []byte(`"`+pair.Target+suffix))
		}
	}
	return text, keys
}

// planCitations rewrites every citation of a source in a tracked file, through
// the link sweep's own grammar (citation.Paths), and lists every other
// line that mentions a source's base name.
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
// working-tree text holds a source's base name, vendor/ excluded. git grep
// answers 1 for no match and anything else above it for a failure, and the
// two are told apart, because "no file mentions it" is an answer and a failed
// search is not.
func trackedFilesMentioning(tree string, pairs []renamePair) ([]string, error) {
	args := make([]string, 0, 5+2*len(pairs)+3)
	args = append(args, "grep", "-l", "-z", "-I", "-F")
	for _, pair := range pairs {
		args = append(args, "-e", path.Base(pair.Source))
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
	report := RenameReport{Moves: plan.Pairs, Citations: plan.Citations, Mentions: plan.Mentions}
	for _, pair := range plan.Pairs {
		if err := os.Remove(treePath(tree, pair.Source)); err != nil {
			return report, err
		}
	}
	for _, file := range slices.Concat(plan.Evidence, plan.Cited) {
		if err := replaceFileAtomically(treePath(tree, file.Rel), file.Updated); err != nil {
			return report, err
		}
	}
	for _, file := range plan.Evidence {
		report.Evidence = append(report.Evidence, RenameRewrite{File: file.Rel, Keys: file.Keys})
	}
	return report, nil
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

// goPlatformSet is the GOOS and GOARCH names the toolchain knows, which decide
// what file-name suffix constrains a build.
type goPlatformSet struct {
	OS   map[string]bool
	Arch map[string]bool
}

// goPlatforms asks the toolchain for its platform list rather than holding a
// copy of it, so a new port is known the day the toolchain knows it.
func goPlatforms() (goPlatformSet, error) {
	out, err := exec.Command("go", "tool", "dist", "list").Output() //nolint:gosec,noctx // fixed argv
	if err != nil {
		return goPlatformSet{}, errors.New("rfc rename: cannot list the Go platforms: " + err.Error())
	}
	set := goPlatformSet{OS: map[string]bool{}, Arch: map[string]bool{}}
	for line := range strings.SplitSeq(string(out), "\n") {
		goos, goarch, found := strings.Cut(strings.TrimSpace(line), "/")
		if found {
			set.OS[goos], set.Arch[goarch] = true, true
		}
	}
	if len(set.OS) == 0 {
		return goPlatformSet{}, errors.New("rfc rename: the Go platform list is empty")
	}
	return set, nil
}

// suffix answers the build-constraint suffix of a Go file name, `_os`, `_arch`
// or `_os_arch`, or empty. It follows go/build's rule: the name less `.go` and
// `_test`, split on underscores, where the first element never counts.
func (s goPlatformSet) suffix(base string) string {
	name := strings.TrimSuffix(strings.TrimSuffix(base, ".go"), "_test")
	parts := strings.Split(name, "_")
	count := len(parts)
	if count >= 3 && s.OS[parts[count-2]] && s.Arch[parts[count-1]] {
		return "_" + parts[count-2] + "_" + parts[count-1]
	}
	if count >= 2 && (s.OS[parts[count-1]] || s.Arch[parts[count-1]]) {
		return "_" + parts[count-1]
	}
	return ""
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
		tb.Str("left out, target taken: ").Str(line).Byte('\n')
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
// Left out and reported: a target that exists or that two findings share, which
// takes a hand-chosen topic, and a file already named for ANOTHER stem, whose
// name and tags disagree and must be read before anything moves. The output is
// created, never overwritten.
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
	var body textbuf.Buffer
	for _, verdict := range candidates {
		_, statErr := os.Lstat(treePath(tree, verdict.Target))
		if statErr == nil || shared[verdict.Target] > 1 {
			report.Collisions = append(report.Collisions, verdict.Rel+" -> "+verdict.Target)
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
		return nil, 2
	}
	return report, 0
}
