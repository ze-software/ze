// Design: docs/architecture/core-design.md -- what a no-break record claims, and what the tree must hold for it
// Overview: discriminate.go -- the record, its fingerprints, and the verdict this file answers
// Detail: rfc.go -- the closed escape vocabulary and the fact each reason names
// Related: discriminate_propose.go -- claimSymbols, the identifiers a tag's prose names
//
// A no-break record is the ESCAPE: it discharges a tag's obligation with
// nothing observed. That makes it the cheapest route on the board, and R-9 is
// the risk that it becomes the answer.
//
// Two things stop that, and both live here. Each reason names a FACT about the
// tree, and the fact is checked. And every escape is tied to the CLAIM it
// discharges: a reason that holds about some file an author names, with nothing
// linking that file to the tag under judgement, discharges every tag equally.
// That is the blanket opt-out wearing a closed vocabulary, and it is what this
// file exists to refuse.
package rfc

import (
	"go/parser"
	"go/token"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// escapeNamesShown bounds the declared names a refusal lists. A generated file
// declares hundreds, and a reader needs enough to write one of them down.
const escapeNamesShown = 8

// escapeCheck is one no-break record and everything the tree says about it.
//
// A struct rather than seven parameters: every check below reads the same
// record against the same tree, and the parent holds the state while each
// method answers one question about it. The receiver is a pointer because the
// struct carries a whole record, and the linter counts the copy at every call.
type escapeCheck struct {
	reader  *sourceReader
	index   *scopeIndex
	carrier Carrier
	carried bool
	// unit is the tagged unit's text, resolved once by the verify walk.
	unit string
	// tagged are the tags that sit in that unit and carry this record's
	// requirement and polarity. Their prose is what ties the escape to the claim.
	tagged []Tag
	record DiscriminationRecord
}

// verdict answers what the tree says about this escape.
//
// The blanket refusal comes first, because it does not depend on which reason
// was given: gomu can generate a break for a unit-tier tag whose producer it can
// mutate, so "no break exists" is false about it whatever reason is offered
// (AC-7). It is checked HERE, on the gate's own path, rather than only where a
// record is written: a record authored by hand, or one whose producer became
// mutatable after it was sealed, reaches `./le rfc check` and never reaches
// `./le rfc discriminate-record`.
func (e *escapeCheck) verdict() DiscriminationVerdict {
	if detail, refused := e.gomuCanBreak(); refused {
		return e.unfounded(detail)
	}
	if e.record.Reason == escapeForeign {
		return e.foreign()
	}
	producerFile := keyFile(e.record.Producer)
	content := e.reader.read(producerFile)
	if content == nil || *content == "" {
		var tb textbuf.Buffer
		return e.unfounded(tb.Str("the escape names the producer ").Quoted(e.record.Producer).
			Str(", which this tree does not hold, so its reason cannot be checked").String())
	}
	if detail, refused := e.producerSymbolMissing(); refused {
		return e.unfounded(detail)
	}
	if detail, refused := e.producerOutOfReach(producerFile); refused {
		return e.unfounded(detail)
	}
	if detail, refused := e.reasonFact(producerFile, *content); refused {
		return e.unfounded(detail)
	}
	if detail, refused := e.claimNamesProducer(producerFile, *content); refused {
		return e.unfounded(detail)
	}
	return DiscriminationVerdict{Record: e.record, State: ProofVerified}
}

// unfounded answers the verdict for an escape the tree does not support.
func (e *escapeCheck) unfounded(detail string) DiscriminationVerdict {
	return DiscriminationVerdict{Record: e.record, State: ProofEscapeUnfounded, Detail: detail}
}

// gomuCanBreak is the refusal that does not depend on the reason.
//
// A unit-tier tag whose producer resolves and sits in a file gomu mutates has a
// break available for the asking. Where a break can be generated, the escape's
// whole premise is false.
func (e *escapeCheck) gomuCanBreak() (string, bool) {
	if e.carrier.Kind != kindUnit || e.record.Producer == "" {
		return "", false
	}
	rel := keyFile(e.record.Producer)
	content := e.reader.read(rel)
	if content == nil || len(e.index.of(*content)) == 0 {
		return "", false
	}
	if gomuIgnores(e.reader, rel) {
		return "", false
	}
	var tb textbuf.Buffer
	return tb.Str("the escape is refused whatever its reason: ").Str(keyFile(e.record.Unit)).
		Str(" is a ").Str(kindUnit).Str(" carrier and its producer ").Str(rel).
		Str(" is a file gomu mutates, so a break can be generated for it. Take the ").
		Str(RouteMutant).Str(" route: `./le rfc discriminate id ").Str(e.record.RID).
		Str(" report <gomu.json>` proposes the candidates").String(), true
}

// foreign checks the escape that names no producer at all.
//
// Two facts, and the second is what ties it to the claim. The carrier must be
// interop, because that is the only kind whose subject is an implementation this
// repository does not build. And the record must CITE the assertion its own
// checker makes about that implementation: the carrier kind alone is a property
// of 37 tags at once, so it discharges every one of them equally, which is the
// blanket opt-out this vocabulary replaced. The citation is checked by
// interopCitationState, the same reader AC-8 uses for an interop proof.
func (e *escapeCheck) foreign() DiscriminationVerdict {
	if !e.carried || e.carrier.Kind != kindInterop {
		var tb textbuf.Buffer
		return e.unfounded(tb.Str("the escape is ").Str(escapeForeign).Str(", which claims the ").
			Str("behavior is produced outside this repository, but ").Str(keyFile(e.record.Unit)).
			Str(" is a ").Str(carrierKindOf(e.carrier, e.carried)).
			Str(" carrier, which runs only code this repository builds").String())
	}
	state, detail := interopCitationState(e.record.Citation, e.unit)
	if state == ProofVerified {
		return DiscriminationVerdict{Record: e.record, State: ProofVerified}
	}
	var tb textbuf.Buffer
	return e.unfounded(tb.Str("the escape is ").Str(escapeForeign).Str(" and ").Str(detail).
		Str(". No code here produces the behavior, so the assertion this checker makes about ").
		Str("it is the only thing that ties the escape to THIS claim rather than to every ").
		Str("interop tag at once").String())
}

// producerSymbolMissing refuses an escape naming a function its producer file
// does not declare.
//
// Both producer-naming reasons are facts about a FILE: a declaration-only file
// holds no function body, and a generated file carries its marker at the top.
// Each check reads the whole file, so the SYMBOL half of the producer key is
// read by nothing, and `widget.go::NoSuchFunc` passes on what widget.go says
// while naming code that is not there. A record half of which nobody checks is
// the value that is silently wrong (ai/rules/principles.md).
//
// resolveKeyText is the resolver every other half of this gate uses, so "does
// this key name code the tree holds" keeps one answer. A file-scoped producer
// resolves to the file's own text, which verdict has already read.
func (e *escapeCheck) producerSymbolMissing() (string, bool) {
	_, held, err := resolveKeyText(e.reader.read, e.index, e.record.Producer, e.record.Source)
	if err == nil && held {
		return "", false
	}
	var tb textbuf.Buffer
	tb.Str("the escape names the producer ").Quoted(e.record.Producer).
		Str(", and this tree holds no such function in ").Str(keyFile(e.record.Producer))
	if e.record.Reason == escapeDeclaration {
		return tb.Str(". A ").Str(escapeDeclaration).
			Str(" file declares no function at all, so name the file alone").String(), true
	}
	return tb.Str(". Name a function that file declares, or name the file alone").String(), true
}

// producerOutOfReach refuses an escape whose producer is not code the tagged
// unit reaches.
//
// The reason's fact and the claim tie both judge the producer FILE, and neither
// asks what that file has to do with THIS test. 605 of the 4,020 claims in the
// tree carry a whole word that some function-free file somewhere declares
// (measured 2026-08-31), so an author who cannot prove a claim can go and find
// one: a claim naming "path" is discharged by
// internal/le/yang/migration/testdata/path/move-success/internal/path.go, which
// declares `const path` and no function. The escape says the code THIS claim
// rests on cannot be broken, so the file has to be code this unit runs.
//
// Reach is what the CARRIER makes it, and both branches are the same question.
// A unit test reaches its own package and the packages its file imports, which
// is what keeps the honest doc.go, embed.go and generated-composition-root
// cases. A .ci or an interop scenario runs the whole daemon, and the code that
// produces what it observes is nothing its own file imports, so what it reaches
// is every file the Go tool compiles. testdata is the one thing no carrier
// reaches: the tool compiles nothing under it, so no test can observe behavior
// it produces. A carrier no table claims is judged by the strict branch, since
// a record whose carrier is unknown is the one to be least sure about.
func (e *escapeCheck) producerOutOfReach(producerFile string) (string, bool) {
	var tb textbuf.Buffer
	head := tb.Str("the escape names ").Str(producerFile).
		Str(" as the code this claim rests on, but ")
	if slices.Contains(strings.Split(producerFile, "/"), "testdata") {
		return head.Str("the Go tool compiles nothing under testdata/, so no test observes ").
			Str("any behavior it produces. Name the code the tagged unit runs, or take a ").
			Str("proof route").String(), true
	}
	unitFile := keyFile(e.record.Unit)
	if path.Dir(producerFile) == path.Dir(unitFile) {
		return "", false
	}
	if e.carried && e.carrier.Kind != kindUnit {
		return "", false
	}
	if e.unitImports(unitFile, producerFile) {
		return "", false
	}
	return head.Str(unitFile).Str(" neither sits in that package nor imports it, so nothing ").
		Str("that file declares runs when this unit runs. A reason that holds about any file ").
		Str("an author names discharges every tag equally, which is the blanket opt-out the ").
		Str("closed vocabulary replaced (R-9): name the code this unit reaches, or take a ").
		Str("proof route").String(), true
}

// unitImports answers whether the tagged unit's own file imports the producer's
// package.
//
// The import path is matched by SUFFIX against the producer's directory, so no
// go.mod read is needed and a fixture tree that carries none answers what the
// checkout answers. A third-party module path ending in the same directory
// would match too, and it buys an author nothing: the producer must also be a
// file this tree holds, whose declaration this claim names.
func (e *escapeCheck) unitImports(unitFile, producerFile string) bool {
	content := e.reader.read(unitFile)
	if content == nil {
		return false
	}
	dir := path.Dir(producerFile)
	for _, imported := range goImportPaths(*content, unitFile) {
		if imported == dir || strings.HasSuffix(imported, "/"+dir) {
			return true
		}
	}
	return false
}

// goImportPaths answers the import paths one Go file names.
//
// go/parser in ImportsOnly mode rather than a regex: an import block is exactly
// what that mode reads, and check_baseline.go already parses Go this way. A
// file that does not parse answers nothing, which REFUSES the escape rather
// than granting it.
func goImportPaths(content, where string) []string {
	file, err := parser.ParseFile(token.NewFileSet(), where, content, parser.ImportsOnly)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(file.Imports))
	for _, spec := range file.Imports {
		value, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		out = append(out, value)
	}
	return out
}

// reasonFact checks what one producer-naming reason says about the tree.
func (e *escapeCheck) reasonFact(producerFile, content string) (string, bool) {
	if e.record.Reason == escapeGenerated {
		if generatedMarkerRE.MatchString(content) {
			return "", false
		}
		var tb textbuf.Buffer
		return tb.Str("the escape is ").Str(escapeGenerated).Str(", but ").Str(producerFile).
			Str(" carries no `// Code generated ... DO NOT EDIT.` line").String(), true
	}
	declared := e.index.of(content)
	if len(declared) == 0 {
		return "", false
	}
	var tb textbuf.Buffer
	return tb.Str("the escape is ").Str(escapeDeclaration).Str(", but ").Str(producerFile).
		Str(" declares ").Int(int64(len(declared))).
		Str(" function(s). A function has a body, and a body can be broken").String(), true
}

// claimNamesProducer ties an escape to the claim it discharges.
//
// The reason's own fact is about a FILE, and nothing in it is about this tag: a
// declaration-only file exists in every package, so naming any doc.go would
// escape any tag on any tier. The tie is the claim's own prose. An escape says
// the code the claim rests on cannot be broken, so the claim names that code
// back: one identifier the producer file declares must appear in the tag's
// claim, which claimSHA has already pinned against rewording (AC-13).
//
// Coverage cannot answer this question, and that is measured rather than
// assumed: a declaration-only file carries no statement, so `go test
// -coverprofile` emits no block for it and no profile can ever show a tagged
// unit reaching it (2026-08-31).
//
// The match is case-insensitive and whole-word. Prose capitalizes a sentence,
// and a substring match would let a claim saying "the" satisfy a file declaring
// `theWidgetTable`.
func (e *escapeCheck) claimNamesProducer(producerFile, content string) (string, bool) {
	declared := declaredNames(content)
	named := make(map[string]bool, len(declared))
	for _, name := range declared {
		named[strings.ToLower(name)] = true
	}
	for index := range e.tagged {
		for _, word := range claimSymbols(e.tagged[index].Claim) {
			if named[strings.ToLower(word)] {
				return "", false
			}
		}
	}
	var tb textbuf.Buffer
	tb.Str("the escape names ").Str(producerFile).Str(" as the code this claim rests on, and ").
		Str("the claim names nothing that file declares")
	if len(declared) == 0 {
		return tb.Str(". It declares nothing at all, so no claim can rest on it").String(), true
	}
	return tb.Str(" (").Join(shownNames(declared), ", ").
		Str("). A reason that holds about any file an author names discharges every tag ").
		Str("equally, which is the blanket opt-out the closed vocabulary replaced (R-9): ").
		Str("name the declaration in the tag's own claim, or take a proof route").String(), true
}

// shownNames bounds a declared-name list to what a refusal can carry.
func shownNames(declared []string) []string {
	if len(declared) <= escapeNamesShown {
		return declared
	}
	var tb textbuf.Buffer
	return append(declared[:escapeNamesShown:escapeNamesShown],
		tb.Str("and ").Int(int64(len(declared)-escapeNamesShown)).Str(" more").String())
}

// The three shapes a top-level Go declaration takes, read line by line.
//
// Regexes rather than a parse, which is how goscope.go already reads Go source
// in this package: adding a parser would give "what does this file declare" a
// second answer to drift from the first.
var (
	// goDeclGroupRE opens a parenthesized group: `var (`, `const (`, `type (`.
	goDeclGroupRE = regexp.MustCompile(`^(?:var|const|type)\s*\(\s*$`)
	// goDeclNameRE is one top-level var, const or type on its own line.
	goDeclNameRE = regexp.MustCompile(`^(?:var|const|type)\s+([A-Za-z_][A-Za-z0-9_]*)`)
	// goGroupNameRE is one name inside a group, which gofmt indents by one tab.
	goGroupNameRE = regexp.MustCompile(`^\t([A-Za-z_][A-Za-z0-9_]*)`)
)

// declaredNames answers the top-level identifiers one Go file declares.
//
// A name it MISSES costs an author a refusal they answer by naming the
// declaration exactly, and a name it INVENTS cannot exist: every answer is read
// off a `func`, `var`, `const` or `type` line of the file itself, and a raw
// string literal is skipped so quoted Go source inside one reads as the text it
// is. The skip is by backtick parity, so a stray backtick costs a MISS and
// never an invention.
func declaredNames(content string) []string {
	var out []string
	grouped := false
	quoted := false
	for line := range strings.SplitSeq(content, "\n") {
		if strings.Count(line, "`")%2 == 1 {
			quoted = !quoted
			continue
		}
		if quoted {
			continue
		}
		if grouped {
			if strings.HasPrefix(line, ")") {
				grouped = false
				continue
			}
			if match := goGroupNameRE.FindStringSubmatch(line); match != nil {
				out = append(out, match[1])
			}
			continue
		}
		if goDeclGroupRE.MatchString(line) {
			grouped = true
			continue
		}
		if match := goDeclNameRE.FindStringSubmatch(line); match != nil {
			out = append(out, match[1])
			continue
		}
		if match := goFuncDeclRE.FindStringSubmatch(line); match != nil {
			out = append(out, match[1])
		}
	}
	return out
}

// gomuIgnores answers whether .gomuignore excludes a file from mutation.
//
// The patterns are READ from the file rather than restated here, so gomu's
// exclusion set is declared once (ai/rules/principles.md). A pattern ending in
// `/` is a directory prefix and everything else is matched against the base
// name, which is the shape every line in that file takes.
func gomuIgnores(reader *sourceReader, rel string) bool {
	content := reader.read(".gomuignore")
	if content == nil {
		return false
	}
	base := filepath.Base(rel)
	for line := range strings.SplitSeq(*content, "\n") {
		pattern := strings.TrimSpace(line)
		if pattern == "" || strings.HasPrefix(pattern, "#") {
			continue
		}
		if strings.HasSuffix(pattern, "/") {
			if strings.HasPrefix(rel, pattern) {
				return true
			}
			continue
		}
		if matched, err := filepath.Match(pattern, base); err == nil && matched {
			return true
		}
	}
	return false
}

// carrierKindOf names a carrier's kind for a refusal, and says so when a tag
// sits on no carrier at all.
func carrierKindOf(carrier Carrier, carried bool) string {
	if !carried {
		return "unrecognized"
	}
	return carrier.Kind
}

// generatedMarkerRE is the line Go defines for generated source. One line, at
// the start of a line, exactly as `go generate` specifies it.
var generatedMarkerRE = regexp.MustCompile(`(?m)^// Code generated .* DO NOT EDIT\.$`)
