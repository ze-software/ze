// Design: docs/architecture/core-design.md -- recording a proof that was observed
// Overview: discriminate.go -- the record this mode writes and check re-verifies
// Detail: discriminate_propose.go -- the candidate breaks this mode records one of
// Related: carriers.go -- the carrier table that says what runs a tagged unit
//
// discriminate_action.go is the only writer of a discrimination record, and it
// writes one only after it has SEEN the red. Apply the break, run the tagged
// unit, require the failure to NAME that unit, and only then seal and store.
// A record nobody observed publishes a red nobody observed, which is the single
// failure the artifact exists to prevent (AC-11).
//
// The break is applied through a Go overlay wherever the compiler that reads it
// is one this process starts, so no source file on disk is modified: several
// sessions share this checkout, and a producer disabled in the tree for the
// minute a suite takes would red every one of them. The interop carrier is the
// exception and requireRedInTree (discriminate_observe.go) says why.
package rfc

import (
	"bytes"
	"encoding/json"
	"errors"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// The keywords `discriminate` and `discriminate-record` take their values
// under. Keyword before value, and every value is compared against something
// the tree declares before it reaches an argv (ai/rules/cli.md).
const (
	keyReport   = "report"
	keyMutant   = "mutant"
	keyPolarity = "polarity"
	keyUnit     = "unit"
	keyRoute    = "route"
	keyProducer = "producer"
	keyCitation = "citation"
	// keyPath is the value word for a filesystem path, shared by every action
	// that takes one.
	keyPath = "path"
)

// How long one observation may take.
//
// Everything here is bounded, in the shape of the git-list deadline in
// testsensitivity: a `go test` that hangs must end the run rather than the day.
// The carrier deadline is the larger of the two because a functional suite
// builds its own isolated binaries first, and an interop scenario costs 21 to
// 150 seconds warm and 353 cold before a single assertion runs.
const (
	unitRunDeadline    = 15 * time.Minute
	carrierRunDeadline = 60 * time.Minute
)

// revertBody is what a revert break puts in the producer's place.
//
// A halt rather than a zero return, for two reasons. It compiles for every
// signature, including named results and generics, so one mechanism serves
// every producer. And a zero return is the silently-wrong value
// ai/rules/principles.md forbids: a test that passed because the producer
// answered zero would record a red for a reason nobody stated.
const revertBody = `panic("BUG: ./le rfc discriminate-record disabled this producer to observe the red")`

// interopScenarioRE reads the scenario a checker drives from its own body.
//
// The interop checkers declare it as `const name = "<scenario>"`, and the
// scenario is then confirmed against test/interop/scenarios/, so the value that
// reaches the runner's environment comes from the tree rather than from a
// record's free text.
var interopScenarioRE = regexp.MustCompile(`(?m)^\s*name\s*=\s*"([a-z0-9][a-z0-9-]*)"`)

// interopScenarioRel is where a scenario's directory lives.
const interopScenarioRel = "test/interop/scenarios"

// discriminationRecordReport is what `./le rfc discriminate-record` answers.
//
// The observed run is part of the answer rather than a log line, because the
// red is the whole product: a reader who cannot see it has to take the record
// on trust, which is the position this artifact exists to end.
type discriminationRecordReport struct {
	Artifact string               `json:"artifact"`
	Record   DiscriminationRecord `json:"record"`
	Observed ObservedRed          `json:"observed"`
}

// Text renders the record and the red it rests on.
func (r discriminationRecordReport) Text() string {
	var tb textbuf.Buffer
	tb.Str(r.Artifact).Str(": recorded ").Str(r.Record.RID).Byte(' ').Str(r.Record.Polarity).
		Str(" at ").Str(r.Record.Unit).Str(" by ").Str(r.Record.Route).Byte('\n')
	if r.Record.Proves() {
		tb.Str("break: ").Str(r.Record.Break).Str(" in ").Str(r.Record.Producer).Byte('\n').
			Str("command: ").Str(r.Observed.Command).Byte('\n').
			Str("observed red in ").Int(int64(r.Observed.Seconds)).Str("s:\n").
			Str(r.Observed.Red).Byte('\n')
	}
	return tb.String()
}

// ObservedRed is the run that made the tagged unit fail.
type ObservedRed struct {
	Command string `json:"command"`
	Seconds int    `json:"seconds"`
	Red     string `json:"red"`
}

// recordRequest is one authored record before anything is checked.
type recordRequest struct {
	rid, polarity, unit, route string
	producer, citation, reason string
	report, mutant             string
}

// discriminateRecordAnswer records one proof or one escape in this checkout.
//
// It answers 2 for anything that stopped it, and a refusal to write is one of
// those things: a break that left the tagged unit green is the state this mode
// exists to catch, and reporting it as success would publish a proof nobody
// observed.
func discriminateRecordAnswer(args leaction.Arguments) (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := recordDiscrimination(tree, recordRequest{
		rid: args[keyID], polarity: args[keyPolarity], unit: args[keyUnit],
		route: args[keyRoute], producer: args[keyProducer],
		citation: args[keyCitation], reason: args[keyReason],
		report: args[keyReport], mutant: args[keyMutant],
	})
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return report, 0
}

// recordDiscrimination is the whole record mode: check, observe, seal, write.
func recordDiscrimination(tree string, request recordRequest) (discriminationRecordReport, error) {
	collected, err := Collect(tree)
	if err != nil {
		return discriminationRecordReport{}, err
	}
	stem, err := stemOfRID(collected.Requirements, request.rid)
	if err != nil {
		return discriminationRecordReport{}, err
	}
	table, err := carriers(tree)
	if err != nil {
		return discriminationRecordReport{}, err
	}
	reader := newSourceReader(tree)
	index := newScopeIndex()
	covers, err := tagCovers(reader, index, collected.Tags)
	if err != nil {
		return discriminationRecordReport{}, err
	}

	tag, unitKey, err := taggedUnitOf(reader, index, collected.Tags, request)
	if err != nil {
		return discriminationRecordReport{}, err
	}
	carrier, carried := CarrierFor(tag.File, table)
	if !carried {
		var tb textbuf.Buffer
		return discriminationRecordReport{}, parseErr(tb.Str(tag.File).
			Str(" sits on no carrier, so nothing runs it and no red can be observed there"))
	}

	record := DiscriminationRecord{
		RID: request.rid, Polarity: request.polarity, Unit: unitKey, Route: request.route,
		Producer: request.producer, Citation: request.citation, Reason: request.reason,
		Source: discriminationSource(stem),
	}
	observed, err := observeFor(tree, reader, index, carrier, tag, &record, request)
	if err != nil {
		return discriminationRecordReport{}, err
	}
	return sealAndStore(tree, stem, reader, index, table, covers, record, observed)
}

// sealAndStore mints the fingerprints, re-checks the record the way the gate
// will, and writes it.
//
// The record is put through validateDiscrimination and verifyOneDiscrimination
// before it lands, so a row `./le rfc check` would refuse never reaches the
// tree. The author who ran the observation is the one who should see that
// refusal, not the next session's gate.
func sealAndStore(tree, stem string, reader *sourceReader, index *scopeIndex, table []Carrier,
	covers map[Cover][]Tag, record DiscriminationRecord,
	observed ObservedRed) (discriminationRecordReport, error) {
	sealed, err := sealDiscrimination(tree, covers, record)
	if err != nil {
		return discriminationRecordReport{}, err
	}
	if err := validateDiscrimination(sealed, sealed.Source, 1); err != nil {
		return discriminationRecordReport{}, err
	}
	verdict, err := verifyOneDiscrimination(reader, index, table, covers, sealed)
	if err != nil {
		return discriminationRecordReport{}, err
	}
	if !verdict.Verified() {
		var tb textbuf.Buffer
		return discriminationRecordReport{}, parseErr(tb.Str("the record just made does not verify (").
			Str(verdict.State).Str("): ").Str(verdict.Detail).
			Str(". Nothing was written: a record `./le rfc check` would refuse must not reach the tree"))
	}
	if err := mergeDiscrimination(tree, stem, sealed); err != nil {
		return discriminationRecordReport{}, err
	}
	return discriminationRecordReport{Artifact: sealed.Source, Record: sealed, Observed: observed}, nil
}

// discriminationSource answers one stem's artifact path.
func discriminationSource(stem string) string {
	var tb textbuf.Buffer
	return tb.Str(discriminationRel).Byte('/').Str(stem).Str(jsonSuffix).String()
}

// stemOfRID answers which RFC declares a requirement.
//
// A requirement no summary declares is refused HERE rather than at the ratchet,
// because the record's file name is derived from the answer: an undeclared id
// has no file to be written into.
func stemOfRID(requirements []Requirement, rid string) (string, error) {
	if rid == "" {
		return "", errors.New("rfc discriminate-record requires id <ID>")
	}
	for _, req := range requirements {
		if req.RID == rid {
			return req.RFC, nil
		}
	}
	var tb textbuf.Buffer
	return "", parseErr(tb.Str(rid).Str(" is declared by no summary in rfc/short/. A proof of an ").
		Str("obligation nobody wrote down proves nothing"))
}

// taggedUnitOf answers the tag this record proves and the unit key it is
// recorded under.
//
// The key is DERIVED from the scanned tag through UnitAt, never taken from the
// request: it reaches a `go test -run` argv and a coverage lookup, so it comes
// from the tree's own scan. The authored value is compared against the derived
// one and named in the refusal, which is what makes a typo readable.
func taggedUnitOf(reader *sourceReader, index *scopeIndex, tags []Tag,
	request recordRequest) (Tag, string, error) {
	if !polarities[request.polarity] {
		var tb textbuf.Buffer
		return Tag{}, "", parseErr(tb.Str("rfc discriminate-record has polarity ").
			Quoted(request.polarity).Str(", want one of: ").Join(Polarities(), ", "))
	}
	var keys []string
	for _, tag := range tags {
		if tag.RID != request.rid || tag.Polarity != request.polarity {
			continue
		}
		key, err := tagUnitKey(reader, index, tag)
		if err != nil {
			return Tag{}, "", err
		}
		if key == request.unit {
			return tag, key, nil
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var tb textbuf.Buffer
	tb.Str("no tag for ").Str(request.rid).Byte(' ').Str(request.polarity).
		Str(" resolves to the unit ").Quoted(request.unit).Str(". ")
	if len(keys) == 0 {
		return Tag{}, "", parseErr(tb.Str("No tag in the tree carries that requirement and ").
			Str("polarity at all, so there is nothing to prove"))
	}
	return Tag{}, "", parseErr(tb.Str("The tagged unit(s) that do are: ").Join(compactSorted(keys), ", "))
}

// compactSorted drops the repeats one unit's several tags produce.
func compactSorted(sorted []string) []string {
	out := sorted[:0]
	for index, one := range sorted {
		if index == 0 || sorted[index-1] != one {
			out = append(out, one)
		}
	}
	return out
}

// tagUnitKey answers the fingerprint key of the unit one tag sits in.
//
// A reader for the file and unitKeyAt for the key, so this adds no second
// answer to "which unit does this tag sit in": that question has one producer,
// and both the obligation and the record's own key read it.
func tagUnitKey(reader *sourceReader, index *scopeIndex, tag Tag) (string, error) {
	content := reader.read(tag.File)
	if content == nil {
		var tb textbuf.Buffer
		return "", parseErr(tb.Str(tag.File).Str(": cannot read the file a tag was scanned from"))
	}
	return unitKeyAt(index, tag.File, *content, tag.Line), nil
}

// observeFor produces the evidence one route owes and fills the record's
// derived halves.
//
// The escape observes nothing, because it claims there is nothing to observe.
// Its cost is the preconditions, and every one of them is paid on the GATE's own
// path: sealAndStore runs verifyOneDiscrimination before the record lands, which
// reaches escapeCheck. A guard that ran only here would be invisible to a record
// authored by hand, and the gate is what has to hold.
func observeFor(tree string, reader *sourceReader, index *scopeIndex, carrier Carrier,
	tag Tag, record *DiscriminationRecord, request recordRequest) (ObservedRed, error) {
	if record.Route == RouteNoBreak {
		return ObservedRed{}, nil
	}
	broken, err := brokenSource(tree, reader, index, record, request)
	if err != nil {
		return ObservedRed{}, err
	}
	runner, err := newObservationRunner(tree, reader, index, carrier, tag, *record)
	if err != nil {
		return ObservedRed{}, err
	}
	if err := runner.requireCleanGreen(); err != nil {
		return ObservedRed{}, err
	}
	return runner.requireRed(broken)
}

// brokenSource answers the file content one break produces, and fills the
// record's producer and break text from what was actually applied.
func brokenSource(tree string, reader *sourceReader, index *scopeIndex,
	record *DiscriminationRecord, request recordRequest) (overlayFile, error) {
	if record.Route == RouteMutant {
		return mutantBreak(tree, reader, index, record, request)
	}
	return revertBreak(reader, index, record)
}

// overlayFile is one file replaced for the length of one observation.
type overlayFile struct {
	rel     string
	content string
}

// mutantBreak answers the source one gomu mutant produces.
//
// The mutant is selected by position rather than by gomu's id, because that id
// is not unique: one report carries `config.go_2` twice, at two columns. File,
// line and column name exactly one operator, and an ambiguous selector is
// refused with the count named rather than resolved by order.
func mutantBreak(tree string, reader *sourceReader, index *scopeIndex,
	record *DiscriminationRecord, request recordRequest) (overlayFile, error) {
	var tb textbuf.Buffer
	if request.report == "" || request.mutant == "" {
		return overlayFile{}, parseErr(tb.Str("the ").Str(RouteMutant).
			Str(" route needs report <path> and mutant <file:line:column#n>, which name the ").
			Str("generated break to apply. `./le rfc discriminate ... report <path>` proposes them"))
	}
	mutants, err := loadGomuReport(tree, request.report)
	if err != nil {
		return overlayFile{}, err
	}
	var found []reportMutant
	for position := range mutants {
		if mutants[position].selector() == request.mutant {
			found = append(found, mutants[position])
		}
	}
	if len(found) != 1 {
		return overlayFile{}, parseErr(tb.Str(request.report).Str(" holds ").Int(int64(len(found))).
			Str(" mutant(s) at ").Str(request.mutant).
			Str(", and a break is applied to exactly one. `./le rfc discriminate ... report <path>` ").
			Str("prints the selector of every candidate, ordinal included"))
	}
	mutant := found[0]
	content := reader.read(mutant.FilePath)
	if content == nil {
		return overlayFile{}, parseErr(tb.Str(mutant.FilePath).
			Str(": the report names a file this tree does not hold"))
	}
	broken, err := replaceOnLine(*content, mutant)
	if err != nil {
		return overlayFile{}, err
	}
	producer := functionAtLine(index, *content, mutant.Line)
	if producer == "" {
		return overlayFile{}, parseErr(tb.Str(request.mutant).
			Str(" sits in no function, and a record names the FUNCTION a break was applied to"))
	}
	record.Producer = tb.Reset().Str(mutant.FilePath).Str("::").Str(producer).String()
	record.Break = mutant.text()
	return overlayFile{rel: mutant.FilePath, content: broken}, nil
}

// replaceOnLine answers the file with one mutant's text substituted.
//
// The substitution is bounded to the mutant's own line, so the same original
// text elsewhere in the file is untouched, and a line that no longer carries
// that text is a refusal rather than a silent no-op: a break that changed
// nothing would run green and be read as a test that does not discriminate.
func replaceOnLine(content string, mutant reportMutant) (string, error) {
	lines := strings.Split(content, "\n")
	var tb textbuf.Buffer
	if mutant.Line < 1 || mutant.Line > len(lines) {
		return "", parseErr(tb.Str(mutant.FilePath).Str(" has ").Int(int64(len(lines))).
			Str(" line(s), and the report names line ").Int(int64(mutant.Line)).
			Str(". The report was taken over a different tree; regenerate it"))
	}
	line := lines[mutant.Line-1]
	// Exactly once, never "at least once". gomu reports the enclosing
	// STATEMENT's column rather than the expression's, so the original text is
	// the only thing that locates what to replace. Two occurrences and a
	// substitution would pick the wrong one, which compiles into a build failure
	// -- a red for the wrong reason, and the shape this gate must never record.
	switch strings.Count(line, mutant.Original) {
	case 1:
	case 0:
		return "", parseErr(tb.Str(mutant.FilePath).Byte(':').Int(int64(mutant.Line)).
			Str(" does not carry ").Quoted(mutant.Original).
			Str(" any more, so this break would change nothing and the run would stay green ").
			Str("for the wrong reason. Regenerate the report"))
	default:
		return "", parseErr(tb.Str(mutant.FilePath).Byte(':').Int(int64(mutant.Line)).
			Str(" carries ").Quoted(mutant.Original).
			Str(" more than once, and gomu reports the enclosing statement's column rather ").
			Str("than the expression's, so which occurrence to break cannot be told. Take ").
			Str("another candidate: `./le rfc discriminate ... report <path>` offers only ").
			Str("the mutants a substitution applies exactly"))
	}
	lines[mutant.Line-1] = strings.Replace(line, mutant.Original, mutant.Mutated, 1)
	return strings.Join(lines, "\n"), nil
}

// revertBreak answers the source one disabled producer produces.
//
// This is the house method `docs/contributing/testing.md` already prescribes --
// disable the producing function and confirm the test flips to red -- with the
// disabling done mechanically and the red recorded instead of forgotten.
func revertBreak(reader *sourceReader, index *scopeIndex,
	record *DiscriminationRecord) (overlayFile, error) {
	var tb textbuf.Buffer
	rel, symbol, err := fingerprintKey(record.Producer, record.Source)
	if err != nil {
		return overlayFile{}, err
	}
	if symbol == "" {
		return overlayFile{}, parseErr(tb.Str(record.Producer).
			Str(" names a whole file, and a revert disables one FUNCTION"))
	}
	content := reader.read(rel)
	if content == nil {
		return overlayFile{}, parseErr(tb.Str(rel).
			Str(": the producer names a file this tree does not hold"))
	}
	texts := index.funcTexts(*content, symbol)
	if len(texts) != 1 {
		return overlayFile{}, parseErr(tb.Str(rel).Str(" declares ").Int(int64(len(texts))).
			Str(" function(s) named ").Str(symbol).
			Str(", and a revert disables exactly one. An unresolved producer is the ").
			Str("defect, never a proof of one"))
	}
	disabled, ok := disableBody(texts[0])
	if !ok {
		return overlayFile{}, parseErr(tb.Str(record.Producer).
			Str(": cannot find the body to disable. The declaration's opening brace is ").
			Str("expected to end its line, which is what gofmt writes"))
	}
	record.Break = tb.Reset().Str("body of ").Str(symbol).Str(" replaced by ").Str(revertBody).String()
	broken := strings.Replace(*content, texts[0], disabled, 1)
	return overlayFile{rel: rel, content: dropOrphanedImports(rel, broken)}, nil
}

// dropOrphanedImports answers the source with the imports the disabled body
// left behind removed.
//
// Replacing a body with a halt orphans every import that only that body used.
// An unused import does not compile, so the overlay fails to BUILD and the run
// reports a build failure where a red was owed: the proof is refused for a
// reason that says nothing about whether the test discriminates. Eleven RFC
// 2865 records were unobtainable for that reason alone, every one of them
// naming a producer whose file imported "context" or "fmt" nowhere else.
//
// An import is dropped only when the name it is reached by appears NOWHERE in
// the file outside the import block, which is the same test the compiler
// applies. A blank and a dot import are always kept, because neither is reached
// by a name and both carry an effect the compiler cannot see.
//
// The one assumption is that an import whose package name differs from the last
// element of its path carries that name explicitly, which is what goimports
// writes. Where it does not, the prune drops an import still in use and the
// build fails exactly as it failed before, so the assumption costs nothing it
// did not already cost.
func dropOrphanedImports(rel, broken string) string {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, rel, broken, parser.ParseComments)
	if err != nil {
		return broken
	}
	used := fileNames(file)

	pruned := false
	for _, decl := range file.Decls {
		group, isDecl := decl.(*ast.GenDecl)
		if !isDecl || group.Tok != token.IMPORT {
			continue
		}
		kept := group.Specs[:0]
		for _, spec := range group.Specs {
			imported, isImport := spec.(*ast.ImportSpec)
			if !isImport {
				kept = append(kept, spec)
				continue
			}
			name := importName(imported)
			if name == "_" || name == "." || used[name] {
				kept = append(kept, spec)
				continue
			}
			pruned = true
		}
		group.Specs = kept
	}
	if !pruned {
		return broken
	}

	var out bytes.Buffer
	if err := format.Node(&out, fset, file); err != nil {
		return broken
	}
	return out.String()
}

// fileNames answers every identifier the file uses outside its import block.
//
// The import block is skipped so an import does not keep itself alive: the name
// in `import "fmt"` is the path, but an explicitly named one declares an
// identifier that would otherwise count as its own use.
func fileNames(file *ast.File) map[string]bool {
	used := map[string]bool{}
	for _, decl := range file.Decls {
		if group, isDecl := decl.(*ast.GenDecl); isDecl && group.Tok == token.IMPORT {
			continue
		}
		ast.Inspect(decl, func(node ast.Node) bool {
			if ident, isIdent := node.(*ast.Ident); isIdent {
				used[ident.Name] = true
			}
			return true
		})
	}
	return used
}

// importName answers the name an import is reached by: the one it declares, or
// the last element of its path when it declares none.
func importName(spec *ast.ImportSpec) string {
	if spec.Name != nil {
		return spec.Name.Name
	}
	path := strings.Trim(spec.Path.Value, `"`)
	if cut := strings.LastIndex(path, "/"); cut >= 0 {
		return path[cut+1:]
	}
	return path
}

// disableBody answers the function with its body replaced by a halt.
//
// The opening brace is the first one that ENDS its line, which is what gofmt
// writes for a declaration and what keeps a generic constraint's own brace --
// `func F[T interface{ ~int }](x T) {` -- from being mistaken for it.
func disableBody(text string) (string, bool) {
	open := -1
	for index := 0; index+1 < len(text); index++ {
		if text[index] == '{' && text[index+1] == '\n' {
			open = index
			break
		}
	}
	closing := strings.LastIndex(text, "}")
	if open < 0 || closing <= open {
		return "", false
	}
	var tb textbuf.Buffer
	return tb.Str(text[:open+1]).Str("\n\t").Str(revertBody).Byte('\n').Str(text[closing:]).String(), true
}

// mergeDiscrimination writes one record into its stem's artifact.
//
// The whole file is re-read, the record replaces any row with the same cover,
// and the rows are sorted by requirement id then unit key, so two sessions
// recording two proofs of one RFC produce a diff each can read (R-11).
func mergeDiscrimination(tree, stem string, record DiscriminationRecord) error {
	rel := discriminationSource(stem)
	kept := []DiscriminationRecord{}
	if _, err := os.Stat(treePath(tree, rel)); err == nil {
		existing, loadErr := loadDiscriminationFile(tree, stem+jsonSuffix)
		if loadErr != nil {
			return loadErr
		}
		for position := range existing {
			if existing[position].Cover() != record.Cover() {
				kept = append(kept, existing[position])
			}
		}
	}
	kept = append(kept, record)
	sort.SliceStable(kept, func(left, right int) bool {
		if kept[left].RID != kept[right].RID {
			return kept[left].RID < kept[right].RID
		}
		return kept[left].Unit < kept[right].Unit
	})
	raw, err := json.MarshalIndent(discriminationFile{RFC: stem, Records: kept}, "", "  ")
	if err != nil {
		var tb textbuf.Buffer
		return parseErr(tb.Str(rel).Str(": cannot render: ").Err(err))
	}
	return writePage(tree, rel, string(raw))
}
