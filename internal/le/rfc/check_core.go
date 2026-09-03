// Design: docs/architecture/core-design.md -- the rfc area, as one command
// Related: check.go -- the centralized check driver that orders these checks
//
// check_core.go evaluates the live requirement list and forward lineage. These
// checks need no committed baseline.
package rfc

import (
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// annotationBarsATest answers the kinds whose claim is contradicted by a tagged
// test on the same requirement.
//
// Each of the three says NO test carries this id, for a different reason: the
// obligation does not bind Ze, Ze does not meet it, or a layer under Ze meets
// it and Ze's own boundary holds nothing to assert. A tag falsifies all three
// the same way, so the annotation is stale rather than the tag being wrong. It
// is also what keeps {lower-layer} out of the proven numerator by more than
// bookkeeping: a requirement Ze can prove is one this annotation may not cover.
func annotationBarsATest(kind string) bool {
	return kind == AnnotationNotApplicable || kind == AnnotationGap || kind == AnnotationLowerLayer
}

// evaluate answers one finding per coverage violation, in the PARTS it had
// before it formatted them.
//
// It is the producer of 148 of this tree's 155 findings, which is why it
// carries parts and the other checks carry their message alone: a published
// table of columns is worth having for the population that fills it, and
// re-parsing the sentence back into fields would be a second reader of a format
// nobody declared (owner review, 2026-09-01).
func evaluate(requirements []Requirement, tags []Tag, enrolled map[string]bool) []Finding {
	known := map[string]bool{}
	for _, req := range requirements {
		known[req.RID] = true
	}
	byRID := map[string][]Tag{}
	var errs []Finding
	for _, tag := range tags {
		if !known[tag.RID] {
			var tb textbuf.Buffer
			errs = append(errs, note(tb.Str(tag.File).Byte(':').Int(int64(tag.Line)).
				Str(": unknown RFC requirement: ").Str(tag.RID).String()))
			continue
		}
		byRID[tag.RID] = append(byRID[tag.RID], tag)
	}
	for _, req := range requirements {
		if !enrolled[req.RFC] {
			continue
		}
		found := byRID[req.RID]
		polarity := map[string]bool{}
		for _, tag := range found {
			polarity[tag.Polarity] = true
		}
		where := requirementWhere(req)
		if req.Ticked {
			var tb textbuf.Buffer
			const issue = "has a ticked checkbox, which is a claim rather than coverage"
			errs = append(errs, requirementFinding(req, issue, tb.Str(where).Str(": ").Str(req.RID).
				Str(" has a ticked checkbox. The box is a template marker, not coverage state -- a tick is a claim, and this gate exists because claims are what rot. Untick it; coverage comes from the test tags").String()))
		}
		annotation := req.Annotation
		if annotation != nil && annotationBarsATest(annotation.Kind) {
			if len(found) > 0 {
				locations := make([]string, 0, len(found))
				for _, tag := range found {
					var tb textbuf.Buffer
					locations = append(locations, tb.Str(tag.File).Byte(':').Int(int64(tag.Line)).String())
				}
				var tb textbuf.Buffer
				var issue textbuf.Buffer
				errs = append(errs, requirementFinding(req,
					issue.Str("is annotated {").Str(annotation.Kind).Str("} and IS tested, so the annotation is stale").String(),
					tb.Str(where).Str(": ").Str(req.RID).Str(" is annotated {").
						Str(annotation.Kind).Str("} but IS tested (").Str(strings.Join(locations, ", ")).
						Str("); the annotation is stale -- remove it").String()))
			}
			continue
		}
		if !req.Gated() {
			continue
		}
		if annotation != nil && annotation.Kind == AnnotationSinglePolarity {
			other := PolarityNegative
			if annotation.Polarity == PolarityNegative {
				other = PolarityPositive
			}
			if polarity[other] {
				var locations []string
				for _, tag := range found {
					if tag.Polarity == other {
						var tb textbuf.Buffer
						locations = append(locations, tb.Str(tag.File).Byte(':').Int(int64(tag.Line)).String())
					}
				}
				var tb textbuf.Buffer
				var issue textbuf.Buffer
				errs = append(errs, requirementFinding(req,
					issue.Str("is annotated {single-polarity: ").Str(annotation.Polarity).
						Str("} and a ").Str(other).Str(" test exists, so the annotation is stale").String(),
					tb.Str(where).Str(": ").Str(req.RID).Str(" is annotated {single-polarity: ").
						Str(annotation.Polarity).Str("} but a ").Str(other).Str(" test exists (").
						Str(strings.Join(locations, ", ")).Str("); the annotation is stale -- remove it and cover both polarities").String()))
			}
			if !polarity[annotation.Polarity] {
				var tb textbuf.Buffer
				var issue textbuf.Buffer
				errs = append(errs, requirementFinding(req,
					issue.Str("has no ").Str(annotation.Polarity).Str(" test, which its annotation requires").String(),
					tb.Str(where).Str(": ").Str(req.RID).Str(" [").Str(req.Level).
						Str("] has no ").Str(annotation.Polarity).Str(" test: ").Str(truncateRunes(req.Text, 70)).String()))
			}
			continue
		}
		if len(found) == 0 {
			var tb textbuf.Buffer
			errs = append(errs, requirementFinding(req, "has no test and no annotation",
				tb.Str(where).Str(": ").Str(req.RID).Str(" [").Str(req.Level).
					Str("] has no test and no annotation: ").Str(truncateRunes(req.Text, 70)).String()))
			continue
		}
		var missing []string
		for _, value := range []string{PolarityNegative, PolarityPositive} {
			if !polarity[value] {
				missing = append(missing, value)
			}
		}
		sort.Strings(missing)
		for _, value := range missing {
			held := make([]string, 0, len(polarity))
			for current := range polarity {
				held = append(held, current)
			}
			sort.Strings(held)
			var tb textbuf.Buffer
			var issue textbuf.Buffer
			errs = append(errs, requirementFinding(req,
				issue.Str("has no ").Str(value).Str(" test, only ").Str(strings.Join(held, "/")).String(),
				tb.Str(where).Str(": ").Str(req.RID).Str(" [").Str(req.Level).
					Str("] has no ").Str(value).Str(" test (only ").Str(strings.Join(held, "/")).
					Str("). A ").Str(value).Str("-less test cannot distinguish correct behavior from blanket accept/reject. Add one, or annotate {single-polarity: ...; why}").String()))
		}
	}
	return errs
}

func truncateRunes(text string, maximum int) string {
	runes := []rune(text)
	if len(runes) <= maximum {
		return text
	}
	return string(runes[:maximum])
}

func checkSuperseded(tree string, requirements []Requirement, successors map[string]string,
	stems map[string]bool) []string {
	idsByStem := map[string]map[string]bool{}
	for _, req := range requirements {
		if idsByStem[req.RFC] == nil {
			idsByStem[req.RFC] = map[string]bool{}
		}
		idsByStem[req.RFC][req.RID] = true
	}
	var errs []string
	for _, req := range requirements {
		where := requirementWhere(req)
		successor := successors[req.RFC]
		mark := req.Superseded
		if successor == "" {
			if mark != nil {
				var tb textbuf.Buffer
				errs = append(errs, tb.Str(where).Str(": ").Str(req.RID).Str(" carries {").Str(SupersededKind).
					Str("} but rfc/short/").Str(req.RFC).Str(".md names no successor in its forward Meta row. Name the obsoleting RFC in the Meta table, or remove the marker").String())
			}
			continue
		}
		if mark == nil {
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(where).Str(": ").Str(req.RID).Str(" [").Str(req.Level).
				Str("] states an obligation of a document ").Str(Prefix(successor)).
				Str(" obsoletes, and does not say where that obligation now lives. Add {superseded: restated ").
				Str(Prefix(successor)).Str("-<section>-<n>; why}, {superseded: dropped; why}, {superseded: unextracted <§section>; why} or {superseded: unresolved; why}. The marker says the obligation MOVED; it never says Ze stops owing it").String())
			continue
		}
		path, textHeld := SourcePath(tree, successor)
		if mark.Disposition == successorUnresolved {
			if textHeld {
				var tb textbuf.Buffer
				errs = append(errs, tb.Str(where).Str(": ").Str(req.RID).Str(" is marked {superseded: unresolved}, which says ").Str(Prefix(successor)).Str(" is not in this repository, but ").Str(path).Str(" is. Read it and say what it does with the obligation").String())
			}
			continue
		}
		if !textHeld {
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(where).Str(": ").Str(req.RID).Str(" is marked {superseded: ").Str(mark.Disposition).
				Str("}, which claims somebody read ").Str(Prefix(successor)).Str(", but its text is not in this repository. Fetch it, or record the debt with {superseded: unresolved; why}").String())
			continue
		}
		if mark.Disposition != successorRestated {
			continue
		}
		if !stems[successor] {
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(where).Str(": ").Str(req.RID).Str(" points at ").Str(mark.Target).
				Str(", but rfc/short/").Str(successor).Str(".md is not in this repository, so no id in it can be checked. Record the debt with {superseded: unextracted <§section>; why}").String())
			continue
		}
		if !hasRIDStem(mark.Target, successor) {
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(where).Str(": ").Str(req.RID).Str(" points at ").Str(mark.Target).
				Str(", which is not a ").Str(Prefix(successor)).Str(" requirement. rfc/short/").Str(req.RFC).
				Str(".md names ").Str(Prefix(successor)).Str(" as its successor, and the lineage that matters runs forward (ai/rules/rfc-compliance.md)").String())
			continue
		}
		if !idsByStem[successor][mark.Target] {
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(where).Str(": ").Str(req.RID).Str(" points at ").Str(mark.Target).
				Str(", which rfc/short/").Str(successor).Str(".md does not declare. Three answers are different and only one of them is this: the successor renumbered the obligation (restated <the real id>), the successor dropped it (dropped), or the successor states it and nobody extracted it (unextracted <§section>)").String())
		}
	}
	return errs
}

// checkLowerLayerProducer holds every {lower-layer} annotation against the
// tree it names.
//
// The kind's whole defense against becoming a second {not-applicable} is that
// its reason claims a FACT rather than a judgement: a layer performs the
// behavior, and this function in this repository installs into that layer. A
// producer nobody can find says neither, so the annotation is refused here and
// not merely at the parser, which can only see that the words are there.
//
// The refusal is by NAME, not by line: a producer that moved keeps its name and
// passes, a producer that was deleted or renamed fails, and that is the event
// this check exists to catch. Reading the file rather than the symbol index is
// what makes it cheap enough to run over every summary on every gate.
func checkLowerLayerProducer(reader *sourceReader, requirements []Requirement) []string {
	var errs []string
	for _, req := range requirements {
		if req.Annotation == nil || req.Annotation.Kind != AnnotationLowerLayer {
			continue
		}
		where := requirementWhere(req)
		parts := producerRE.FindStringSubmatch(req.Annotation.Producer)
		if parts == nil {
			// The parser fills Producer or refuses the line, so this arm is
			// reached only by a requirement built in code. It REFUSES rather
			// than skipping: a missing producer is the one state this kind may
			// not hold, and passing it silently is how a guard stops guarding
			// (ai/rules/principles.md).
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(where).Str(": ").Str(req.RID).
				Str(" is annotated {lower-layer} and names no producer. ").
				Str(lowerLayerFormat).String())
			continue
		}
		path, symbol := parts[1], parts[3]
		content := reader.read(path)
		if content == nil {
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(where).Str(": ").Str(req.RID).
				Str(" is annotated {lower-layer: ").Str(req.Annotation.Layer).
				Str("} and names the producer ").Str(req.Annotation.Producer).
				Str(", whose file this checkout does not carry. The kind rests on a producer a reader can open: name the file that installs into ").
				Str(req.Annotation.Layer).Str(", or the annotation claims what nothing here can show").String())
			continue
		}
		if !declaresFunction(*content, symbol) {
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(where).Str(": ").Str(req.RID).
				Str(" is annotated {lower-layer: ").Str(req.Annotation.Layer).
				Str("} and names the producer ").Str(req.Annotation.Producer).
				Str(", but ").Str(path).Str(" declares no ").Str(symbol).
				Str(". The producer was renamed or deleted under the annotation: name the function that installs into ").
				Str(req.Annotation.Layer).Str(" today").String())
		}
	}
	return errs
}

// declaresFunction answers whether a Go file declares a top-level function of
// this name. A method matches on its own name, because that is the name
// funcNameIn reads and the receiver is not part of it.
func declaresFunction(content, name string) bool {
	for _, unit := range FunctionUnits(content) {
		if unit.Name == name {
			return true
		}
	}
	return false
}
