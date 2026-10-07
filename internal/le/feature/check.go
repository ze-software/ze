// Design: docs/contributing/feature-maturity.md -- the generic bar and what the check proves
// Related: declaration.go -- the parsed values judged here
// Related: runrecord.go -- the recorded green runs S1 reads
//
// The check resolves every evidence pointer through the producer that owns the
// fact, computes each feature's ceiling, and refuses a declared level above it.
// RFC facts come from internal/le/rfc's own parse; nothing here reads
// rfc/short/*.md.

package feature

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/derived"
	"github.com/ze-software/ze/internal/le/rfc"
	journal "github.com/ze-software/ze/internal/le/spec/journal"
)

// immediateSpecDir holds the specs for defects that block a release.
const immediateSpecDir = "plan/immediate"

// Verdict is the check's answer for one declaration.
type Verdict struct {
	Declaration Declaration
	// Ceiling is the highest level the evidence supports; LevelUnspecified for
	// a scope with no level.
	Ceiling Level
	// Unmet lists, per level above the ceiling, the criteria that level lacks.
	Unmet map[Level][]string
	// Refusals are the reasons the declaration cannot stand as written.
	Refusals []string
	// Bounds name what the ceiling does not cover (D-3: an unenrolled stem).
	Bounds []string
}

// evidence is everything outside the declarations the check reads, loaded once
// per run.
type evidence struct {
	tree      string
	rfc       rfc.Collected
	immediate map[string]string // spec path -> its Files to Modify section
	journal   []journal.Row
	dates     *changeDates
	// scenarios caches each interop suite's scenarios, name -> directory
	// relative to tree; catalogErrors holds the suites whose catalog could
	// not list them.
	scenarios     map[string]map[string]string
	catalogErrors map[string]string
}

// Check judges every declaration of the tree. The error is for a tree that
// cannot be read; a refused declaration is a Verdict with Refusals.
func Check(tree string) ([]Verdict, []error, error) {
	declarations, problems, err := Load(tree)
	if err != nil {
		return nil, nil, err
	}
	collected, err := rfc.Collect(tree)
	if err != nil {
		return nil, nil, err
	}
	immediate, err := immediateSpecs(tree)
	if err != nil {
		return nil, nil, err
	}
	rows, err := journal.HeadRows(tree)
	if err != nil {
		return nil, nil, err
	}
	dates, err := newChangeDates(tree)
	if err != nil {
		return nil, nil, err
	}
	in := &evidence{tree: tree, rfc: collected, immediate: immediate, journal: rows, dates: dates,
		scenarios: map[string]map[string]string{}, catalogErrors: map[string]string{}}
	byID := make(map[string]*Declaration, len(declarations))
	for i := range declarations {
		byID[declarations[i].ID] = &declarations[i]
	}
	verdicts := make([]Verdict, 0, len(declarations))
	for i := range declarations {
		if declarations[i].Kind == KindUmbrella {
			continue
		}
		verdicts = append(verdicts, in.judge(&declarations[i]))
	}
	// Umbrellas last: their ceiling is the worst of their parts' ceilings.
	for i := range declarations {
		if declarations[i].Kind != KindUmbrella {
			continue
		}
		verdicts = append(verdicts, judgeUmbrella(&declarations[i], byID, verdicts))
	}
	slices.SortFunc(verdicts, func(a, b Verdict) int { //nolint:gocritic // hugeParam: SortFunc fixes the comparator's value signature
		return strings.Compare(a.Declaration.ID, b.Declaration.ID)
	})
	return verdicts, problems, nil
}

// judge computes one non-umbrella declaration's verdict.
func (in *evidence) judge(d *Declaration) Verdict {
	verdict := Verdict{Declaration: *d, Unmet: map[Level][]string{}}
	in.checkPaths(d, &verdict)
	in.staleDocReview(d, &verdict)
	if !d.Scope.Implemented() {
		return verdict
	}
	in.criterionRealPath(d, &verdict)
	in.criterionInterop(d, &verdict)
	in.criterionRFC(d, &verdict)
	in.criterionDocs(d, &verdict)
	in.criterionDefects(d, &verdict)
	in.staleDefectReview(d, &verdict)
	criterionStub(d, &verdict)
	in.criterionExtra(d, &verdict)
	verdict.Ceiling = ceilingOf(verdict.Unmet)
	refuseAboveCeiling(&verdict)
	return verdict
}

// ceilingOf answers the highest level none of whose criteria are unmet.
func ceilingOf(unmet map[Level][]string) Level {
	for _, entry := range slices.Backward(levelNames) {
		if len(unmet[entry.value]) == 0 {
			return entry.value
		}
	}
	return LevelUnspecified
}

func refuseAboveCeiling(verdict *Verdict) {
	declared := verdict.Declaration.Level
	if declared <= verdict.Ceiling {
		return
	}
	var tb textbuf.Buffer
	tb.Str("Level ").Str(declared.String()).Str(" is above the evidence ceiling ").
		Str(verdict.Ceiling.String()).Str(": ").Str(strings.Join(verdict.Unmet[declared], "; "))
	verdict.Refusals = append(verdict.Refusals, tb.String())
}

// checkPaths refuses a declared path that escapes the tree or does not exist
// (AC-3), whatever the level: a pointer to nothing is a false statement.
func (in *evidence) checkPaths(d *Declaration, verdict *Verdict) {
	fields := []struct {
		name  string
		paths []string
	}{
		{fieldComponents, d.Components},
		{fieldDocs, d.Docs},
	}
	if d.Page != "" {
		fields = append(fields, struct {
			name  string
			paths []string
		}{fieldPage, []string{strings.SplitN(d.Page, "#", 2)[0]}})
	}
	for _, field := range fields {
		for _, rel := range field.paths {
			if problem := in.repoPath(rel); problem != "" {
				verdict.Refusals = append(verdict.Refusals, field.name+" "+problem)
			}
		}
	}
	for _, item := range d.RealPathTests {
		if problem := in.testItem(item); problem != "" {
			verdict.Refusals = append(verdict.Refusals, fieldRealPathTests+" "+problem)
		}
	}
	for _, item := range d.Interop {
		if problem := in.interopItem(item); problem != "" {
			verdict.Refusals = append(verdict.Refusals, fieldInterop+" "+problem)
		}
	}
	for _, stem := range d.RFCs {
		if _, known := in.rfc.Metas[stem]; !known {
			verdict.Refusals = append(verdict.Refusals,
				fieldRFCs+" '"+stem+"' names no rfc/short/"+stem+".md (or its Meta does not parse)")
		}
	}
	evidenceItems := slices.Concat(d.RealPathTests, d.Interop)
	for _, item := range d.StubEvidence {
		if !slices.Contains(evidenceItems, item) {
			verdict.Refusals = append(verdict.Refusals,
				fieldStubEvidence+" '"+item+"' is not one of the listed Real-path tests or Interop items")
		}
	}
}

// repoPath answers why rel is not an existing path inside the tree, or "".
func (in *evidence) repoPath(rel string) string {
	if rel == "" {
		return "is empty"
	}
	if filepath.IsAbs(rel) {
		return "'" + rel + "' is absolute; declarations name repository-relative paths"
	}
	clean := path.Clean(rel)
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "'" + rel + "' escapes the repository"
	}
	if _, err := os.Stat(filepath.Join(in.tree, filepath.FromSlash(clean))); err != nil {
		if derivedArtifact(clean) {
			return ""
		}
		return "'" + rel + "' does not exist"
	}
	return ""
}

// derivedArtifact reports whether rel is a registered derived artifact. Such a
// page is gitignored and rebuilt by `./le` when a command names it, so a fresh
// checkout that has not rendered it yet is not missing it.
func derivedArtifact(rel string) bool {
	for _, artifact := range derived.All() {
		if artifact.Path == rel {
			return true
		}
	}
	return false
}

// testItem answers why a Real-path tests item does not resolve, or "". A Go
// test is `file.go::TestName` and the named function must be declared there.
func (in *evidence) testItem(item string) string {
	file, function, named := strings.Cut(item, goTestSeparator)
	if problem := in.repoPath(file); problem != "" {
		return problem
	}
	if !named {
		if strings.HasSuffix(file, "_test.go") {
			return "'" + item + "' names a Go test file without '::<TestName>'"
		}
		// A-7: a file no runner discovers runs nowhere, so the runners' own
		// declarations answer, not the file's existence (runner.go).
		_, problem := runnerOf(file)
		return problem
	}
	if !declaresFunction(filepath.Join(in.tree, filepath.FromSlash(file)), function) {
		return "'" + item + "': " + file + " declares no function " + function
	}
	return ""
}

func declaresFunction(file, function string) bool {
	parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.SkipObjectResolution)
	if err != nil {
		return false
	}
	for _, declaration := range parsed.Decls {
		fn, ok := declaration.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fn.Recv == nil && fn.Name.Name == function {
			return true
		}
	}
	return false
}

// criterionRealPath is S1 with D-6: Supported needs at least one real-path
// test, and every listed one needs a recorded green run of its present content.
func (in *evidence) criterionRealPath(d *Declaration, verdict *Verdict) {
	if len(d.RealPathTests) == 0 {
		verdict.Unmet[LevelSupported] = append(verdict.Unmet[LevelSupported], "S1: no real-path test is listed")
		return
	}
	record, err := loadRunRecord(in.tree, d.ID)
	if err != nil {
		verdict.Refusals = append(verdict.Refusals, err.Error())
		return
	}
	for _, item := range d.RealPathTests {
		file, _, _ := strings.Cut(item, goTestSeparator)
		blob, err := blobID(in.tree, file)
		if err != nil {
			continue // checkPaths already refused a missing file.
		}
		switch record.stateOf(item, blob) {
		case runStateCurrent:
		case runStateNotRun:
			verdict.Unmet[LevelSupported] = append(verdict.Unmet[LevelSupported],
				"S1: "+item+" exists, not run (no green run recorded in "+runRecordRel(d.ID)+")")
		case runStateStale:
			verdict.Unmet[LevelSupported] = append(verdict.Unmet[LevelSupported],
				"S1: "+item+" changed since its recorded green run")
		case runStateUnspecified:
			panic("BUG: stateOf answered no run state")
		}
	}
}

// criterionRFC is S3 under D-3: only enrolled stems count, and a counting stem
// must be published as supported by the ledger itself with no open gap on a
// gated (MUST-level) requirement. The ledger's own Support status is the
// verdict `./le rfc check` already gates, so it is read, never re-derived.
func (in *evidence) criterionRFC(d *Declaration, verdict *Verdict) {
	for _, stem := range d.RFCs {
		meta, known := in.rfc.Metas[stem]
		if !known {
			continue // checkPaths refused it.
		}
		if !meta.Enrolled() {
			verdict.Bounds = append(verdict.Bounds, "S3 bounded: "+stem+" not enrolled")
			continue
		}
		if !strings.HasPrefix(meta.Status, "Supported") {
			verdict.Unmet[LevelSupported] = append(verdict.Unmet[LevelSupported],
				"S3: "+stem+" ledger Support status is '"+meta.Status+"'")
		}
		if gaps := openGaps(in.rfc.Requirements, stem); len(gaps) > 0 {
			verdict.Unmet[LevelSupported] = append(verdict.Unmet[LevelSupported],
				"S3: "+stem+" open gap on "+strings.Join(gaps, ", "))
		}
	}
}

// openGaps answers the gated requirement ids of stem carrying a {gap}.
func openGaps(requirements []rfc.Requirement, stem string) []string {
	prefix := rfc.Prefix(stem) + "-"
	var gaps []string
	for _, requirement := range requirements {
		if !strings.HasPrefix(requirement.RID, prefix) {
			continue
		}
		if !rfc.IsGatedLevel(requirement.Level) {
			continue
		}
		if requirement.Annotation == nil {
			continue
		}
		if requirement.Annotation.Kind == rfc.AnnotationGap {
			gaps = append(gaps, requirement.RID)
		}
	}
	return gaps
}

// criterionDocs is S4's presence half: a missing page or review lowers the
// ceiling below Supported.
func (in *evidence) criterionDocs(d *Declaration, verdict *Verdict) {
	if len(d.Docs) == 0 {
		verdict.Unmet[LevelSupported] = append(verdict.Unmet[LevelSupported], "S4: no Docs page is listed")
	}
	if !d.DocReview.Present() {
		verdict.Unmet[LevelSupported] = append(verdict.Unmet[LevelSupported], "S4: no Doc review is recorded")
	}
}

// criterionDefects is S5's mechanical half (AC-7): no immediate spec may name a
// Components path in its Files to Modify.
func (in *evidence) criterionDefects(d *Declaration, verdict *Verdict) {
	for _, spec := range sortedKeys(in.immediate) {
		section := in.immediate[spec]
		for _, component := range d.Components {
			if strings.Contains(section, component) {
				verdict.Unmet[LevelSupported] = append(verdict.Unmet[LevelSupported],
					"S5: "+spec+" names "+component+" in its Files to Modify")
				break
			}
		}
	}
	if !d.DefectReview.Present() {
		verdict.Unmet[LevelSupported] = append(verdict.Unmet[LevelSupported], "S5: no Defect review is recorded")
	}
}

// criterionStub is S6 (AC-9): Supported needs an S1 or S2 item not marked
// stub, and stub-backed is only for a feature whose evidence is all stub.
func criterionStub(d *Declaration, verdict *Verdict) {
	items := slices.Concat(d.RealPathTests, d.Interop)
	nonStub := 0
	for _, item := range items {
		if !slices.Contains(d.StubEvidence, item) {
			nonStub++
		}
	}
	if len(items) > 0 {
		if nonStub == 0 {
			verdict.Unmet[LevelSupported] = append(verdict.Unmet[LevelSupported], "S6: every S1 and S2 item is a stub")
			verdict.Unmet[LevelExperimental] = append(verdict.Unmet[LevelExperimental], "S6: every S1 and S2 item is a stub")
		}
	}
	if d.Level == LevelStubBacked {
		if len(d.StubEvidence) == 0 {
			verdict.Refusals = append(verdict.Refusals, "Level stub-backed names no Stub evidence item")
		}
	}
}

// judgeUmbrella bounds an umbrella by the worst of its parts (AC-10).
func judgeUmbrella(d *Declaration, byID map[string]*Declaration, verdicts []Verdict) Verdict {
	verdict := Verdict{Declaration: *d, Unmet: map[Level][]string{}, Ceiling: LevelSupported}
	worstScope := ScopeComplete
	for _, id := range d.Parts {
		part, known := byID[id]
		if !known {
			verdict.Refusals = append(verdict.Refusals, fieldParts+" '"+id+"' names no declaration")
			continue
		}
		if part.Kind == KindUmbrella {
			verdict.Refusals = append(verdict.Refusals, fieldParts+" '"+id+"' is itself an umbrella; parts are leaves")
			continue
		}
		if part.Scope != ScopeComplete {
			worstScope = ScopePartial
		}
		ceiling := ceilingOfPart(id, verdicts)
		if ceiling < verdict.Ceiling {
			verdict.Ceiling = ceiling
			verdict.Unmet[LevelSupported] = append(verdict.Unmet[LevelSupported], "part "+id+" bounds it at "+ceiling.String())
		}
	}
	if d.Scope == ScopeComplete {
		if worstScope == ScopePartial {
			verdict.Refusals = append(verdict.Refusals, "Scope complete while a part is not complete")
		}
	}
	refuseAboveCeiling(&verdict)
	return verdict
}

func ceilingOfPart(id string, verdicts []Verdict) Level {
	for i := range verdicts {
		if verdicts[i].Declaration.ID == id {
			return verdicts[i].Ceiling
		}
	}
	return LevelUnspecified
}

// immediateSpecs answers each plan/immediate spec's Files to Modify section.
func immediateSpecs(tree string) (map[string]string, error) {
	entries, err := os.ReadDir(filepath.Join(tree, immediateSpecDir))
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		rel := immediateSpecDir + "/" + entry.Name()
		text, err := os.ReadFile(filepath.Join(tree, rel)) //nolint:gosec // rel is an entry of plan/immediate, listed above
		if err != nil {
			return nil, err
		}
		if section, found := sectionBody(string(text), "## Files to Modify"); found {
			out[rel] = section
		}
	}
	return out, nil
}

func sortedKeys(set map[string]string) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
