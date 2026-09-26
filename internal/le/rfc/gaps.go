// Design: docs/architecture/core-design.md -- the rfc area, as one command
// Related: tags.go -- the gap marker the scanner reads
// Related: check_core.go -- evaluate, which judges the proof tags beside these
//
// gaps.go ties a demonstrated-gap tag to the test that demonstrates it.
//
// A `{gap}` row is prose: it says Ze does not meet the requirement, and nothing
// notices the day Ze starts to. A gap tag, `RFC requirement: <ID> gap`, names a
// Go test that asserts the RFC-correct behavior through rfcgap.Demonstrate. The
// helper inverts the result, so the test passes while the gap stands and fails
// the day the behavior lands. This gate is static and never runs that test, so
// what it can check is the tie: the row is still `{gap}`, and the unit around
// the tag calls the helper with the tag's own id.
package rfc

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The helper a gap test calls. The import is matched on its path suffix rather
// than on the full path, because the module path is go.mod's to declare and a
// fixture module carries the helper under its own.
const (
	rfcgapImportSuffix = "/internal/test/rfcgap"
	rfcgapPackage      = "rfcgap"
	rfcgapFunc         = "Demonstrate"
)

// splitGapTags separates the gap tags from the proof tags, keeping scan order.
//
// A gap tag proves no polarity. Every coverage reader, every ratchet and the
// discrimination obligation walk the proof corpus, so a gap tag left in it
// would reach each of them as a tag with an empty polarity: one more tag on a
// row that owes a positive and a negative, and one more unit billed a
// discrimination record whose green half cannot be observed until the gap
// closes. Splitting once, where the tree is scanned, keeps every one of those
// readers exactly as it was.
func splitGapTags(tags []Tag) (proof, gap []Tag) {
	for _, tag := range tags {
		if tag.Gap {
			gap = append(gap, tag)
			continue
		}
		proof = append(proof, tag)
	}
	return proof, gap
}

// gapDemonstration answers the unit key of the Go function around line when
// its body calls rfcgap.Demonstrate with rid as the id argument, written as a
// string literal. It answers "" in every other case: the tag sits outside
// exactly one function, the file does not import the helper, the unit does not
// parse, or no call names this id. The gate refuses a gap tag with "" here, so
// each of those cases is a refusal and none is a silent pass.
//
// The unit is UnitAt's, the one definition of the text a tag governs, so the
// helper call must sit in the same function the discrimination fingerprints
// read. go/ast reads the call inside it, because a text match would also accept
// a commented-out call or an id in another argument.
func gapDemonstration(path, src string, line int, rid string) string {
	unit := UnitAt(path, src, line)
	if unit.Scope != ScopeFunc {
		return ""
	}
	qualifier := rfcgapQualifier(path, src)
	if qualifier == "" {
		return ""
	}
	file, err := parser.ParseFile(token.NewFileSet(), path, "package unit\n"+unit.Text,
		parser.SkipObjectResolution)
	if err != nil {
		return ""
	}
	if !callsDemonstrate(file, qualifier, rid) {
		return ""
	}
	return unitKeyAt(newScopeIndex(), path, src, line)
}

// rfcgapQualifier answers the name this file calls the helper package by, and
// "" when the file does not import it or imports it under `_` or `.`.
func rfcgapQualifier(path, src string) string {
	file, err := parser.ParseFile(token.NewFileSet(), path, src, parser.ImportsOnly)
	if err != nil {
		return ""
	}
	for _, spec := range file.Imports {
		imported, unquoteErr := strconv.Unquote(spec.Path.Value)
		if unquoteErr != nil {
			continue
		}
		if !strings.HasSuffix(imported, rfcgapImportSuffix) {
			continue
		}
		if spec.Name == nil {
			return rfcgapPackage
		}
		if spec.Name.Name == "_" {
			return ""
		}
		if spec.Name.Name == "." {
			return ""
		}
		return spec.Name.Name
	}
	return ""
}

// callsDemonstrate reports whether any call under node is
// `<qualifier>.Demonstrate(<tb>, "<rid>", ...)`.
func callsDemonstrate(node ast.Node, qualifier, rid string) bool {
	found := false
	ast.Inspect(node, func(current ast.Node) bool {
		if found {
			return false
		}
		call, isCall := current.(*ast.CallExpr)
		if !isCall {
			return true
		}
		found = demonstratesID(call, qualifier, rid)
		return !found
	})
	return found
}

// demonstratesID reports whether one call is the helper naming rid.
func demonstratesID(call *ast.CallExpr, qualifier, rid string) bool {
	selector, isSelector := call.Fun.(*ast.SelectorExpr)
	if !isSelector {
		return false
	}
	pkg, isIdent := selector.X.(*ast.Ident)
	if !isIdent {
		return false
	}
	if pkg.Name != qualifier {
		return false
	}
	if selector.Sel.Name != rfcgapFunc {
		return false
	}
	if len(call.Args) < 2 {
		return false
	}
	literal, isLiteral := call.Args[1].(*ast.BasicLit)
	if !isLiteral {
		return false
	}
	if literal.Kind != token.STRING {
		return false
	}
	id, err := strconv.Unquote(literal.Value)
	if err != nil {
		return false
	}
	return id == rid
}

// annotatedGap reports whether a row carries the `{gap}` annotation.
func annotatedGap(req *Requirement) bool {
	if req.Annotation == nil {
		return false
	}
	return req.Annotation.Kind == AnnotationGap
}

// gapRefusal is why the gate refuses one gap tag: the short issue a finding
// table shows, and the advice its message ends with.
type gapRefusal struct {
	issue  string
	advice string
}

// gapTagRefusal answers why the gate refuses a gap tag on req, and false when
// it accepts it. This is the ONE definition of an accepted gap tag: the gate's
// findings and every published demonstrated-gap count read it.
func gapTagRefusal(req *Requirement, tag *Tag) (gapRefusal, bool) {
	if !goScoped(tag.File) {
		return gapRefusal{issue: "carries a gap tag outside a Go test",
			advice: "a gap is demonstrated in a Go test that calls rfcgap.Demonstrate, which inverts its result. " +
				"A .ci or .et test cannot invert its own, so move the assertion into a Go test"}, true
	}
	if !annotatedGap(req) {
		return gapRefusal{issue: "carries a gap tag but is not annotated {gap}",
			advice: "if the behavior landed, the gap is closed: retag the test positive or negative " +
				"and replace rfcgap.Demonstrate with a plain assertion. If the row lost its {gap} " +
				"annotation by mistake, restore it"}, true
	}
	if tag.Demonstration == "" {
		var advice textbuf.Buffer
		return gapRefusal{issue: "carries a gap tag whose test does not call rfcgap.Demonstrate with this id",
			advice: advice.Str("put the tag inside the Go test function whose body calls rfcgap.Demonstrate(t, \"").
				Str(req.RID).Str("\", ...), with the id written as a string literal").String()}, true
	}
	return gapRefusal{}, false
}

// evaluateGapTags answers one finding for each gap tag the gate refuses.
//
// It judges every summary, enrolled or not: a gap tag claims a demonstration,
// and a claim the gate cannot tie to a unit is refused wherever it sits. The
// proof tags on the same rows stay evaluate's, which keeps refusing a positive
// or negative tag on a `{gap}` row.
func evaluateGapTags(requirements []Requirement, gapTags []Tag) []Finding {
	byRID := make(map[string]*Requirement, len(requirements))
	for index := range requirements {
		byRID[requirements[index].RID] = &requirements[index]
	}
	var errs []Finding
	for index := range gapTags {
		tag := &gapTags[index]
		req, known := byRID[tag.RID]
		if !known {
			var tb textbuf.Buffer
			errs = append(errs, note(tb.Str(tagWhere(tag.File, tag.Line)).
				Str(": unknown RFC requirement: ").Str(tag.RID).String()))
			continue
		}
		refusal, refused := gapTagRefusal(req, tag)
		if !refused {
			continue
		}
		var tb textbuf.Buffer
		errs = append(errs, requirementFinding(*req, refusal.issue,
			tb.Str(tagWhere(tag.File, tag.Line)).Str(": ").Str(req.RID).Str(" (").
				Str(requirementWhere(*req)).Str(") ").Str(refusal.issue).Str(": ").
				Str(refusal.advice).String()))
	}
	return errs
}

// demonstratedGaps answers the accepted gap tag of every `{gap}` row a test
// demonstrates, keyed by requirement id. The first accepted tag in scan order
// names the row's demonstration.
func demonstratedGaps(requirements []Requirement, gapTags []Tag) map[string]Tag {
	byRID := make(map[string]*Requirement, len(requirements))
	for index := range requirements {
		byRID[requirements[index].RID] = &requirements[index]
	}
	out := map[string]Tag{}
	for index := range gapTags {
		tag := &gapTags[index]
		req, known := byRID[tag.RID]
		if !known {
			continue
		}
		if _, refused := gapTagRefusal(req, tag); refused {
			continue
		}
		if _, held := out[tag.RID]; held {
			continue
		}
		out[tag.RID] = *tag
	}
	return out
}

// GapCount splits one summary's `{gap}` rows by what stands behind them: a test
// that demonstrates the gap, or the annotation's prose alone.
type GapCount struct {
	Demonstrated int `json:"demonstrated"`
	Described    int `json:"described"`
}

// gapCounts answers the split for every summary holding a `{gap}` row, at any
// level, keyed by stem. `{gap}` stays the single summary fact: the count reads
// the annotation, and a gap tag only moves a row from Described to
// Demonstrated.
func gapCounts(requirements []Requirement, demonstrated map[string]Tag) map[string]GapCount {
	out := map[string]GapCount{}
	for index := range requirements {
		req := &requirements[index]
		if !annotatedGap(req) {
			continue
		}
		count := out[req.RFC]
		if _, held := demonstrated[req.RID]; held {
			count.Demonstrated++
		} else {
			count.Described++
		}
		out[req.RFC] = count
	}
	return out
}
