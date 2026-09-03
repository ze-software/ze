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
// Each of the four says NO test carries this id, for a different reason: the
// obligation does not bind Ze, Ze does not meet it, a layer under Ze meets it
// and Ze's own boundary holds nothing to assert, or its condition is a feature
// Ze declined. A tag falsifies all four the same way, so the annotation is
// stale rather than the tag being wrong. It is also what keeps {lower-layer}
// and {feature-declined} out of the proven numerator by more than bookkeeping:
// a requirement Ze can prove is one this annotation may not cover.
func annotationBarsATest(kind string) bool {
	return kind == AnnotationNotApplicable || kind == AnnotationGap ||
		kind == AnnotationLowerLayer || kind == AnnotationFeatureDeclined
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
		state, path, symbol := resolveProducer(reader, req.Annotation.Producer)
		switch state {
		case producerFound:
		case producerUnnamed:
			// The parser fills Producer or refuses the line, so this arm is
			// reached only by a requirement built in code. It REFUSES rather
			// than skipping: a missing producer is the one state this kind may
			// not hold, and passing it silently is how a guard stops guarding
			// (ai/rules/principles.md).
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(where).Str(": ").Str(req.RID).
				Str(" is annotated {lower-layer} and names no producer. ").
				Str(lowerLayerFormat).String())
		case producerFileAbsent:
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(where).Str(": ").Str(req.RID).
				Str(" is annotated {lower-layer: ").Str(req.Annotation.Layer).
				Str("} and names the producer ").Str(req.Annotation.Producer).
				Str(", whose file this checkout does not carry. The kind rests on a producer a reader can open: name the file that installs into ").
				Str(req.Annotation.Layer).Str(", or the annotation claims what nothing here can show").String())
		case producerSymbolAbsent:
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

// producerState is what this checkout can show about the `<path>.go::<Symbol>`
// an annotation names.
type producerState int

const (
	producerFound producerState = iota
	producerUnnamed
	producerFileAbsent
	producerSymbolAbsent
)

// resolveProducer answers what the tree shows for one producer key, and the
// path and the symbol it read.
//
// Two kinds name a producer and each owes its own sentence, so the RESOLUTION
// is shared and the refusals are not. A second copy of "read the file, find the
// function" would be one rule two checks could disagree about
// (ai/rules/principles.md).
func resolveProducer(reader *sourceReader, producer string) (producerState, string, string) {
	parts := producerRE.FindStringSubmatch(producer)
	if parts == nil {
		return producerUnnamed, "", ""
	}
	path, symbol := parts[1], parts[3]
	content := reader.read(path)
	if content == nil {
		return producerFileAbsent, path, symbol
	}
	if !declaresFunction(*content, symbol) {
		return producerSymbolAbsent, path, symbol
	}
	return producerFound, path, symbol
}

// checkFeatureDeclined holds every {feature-declined} annotation against the
// two things it claims: the RFC's own text, and this tree.
//
// The kind says an obligation is conditional on a feature the RFC makes
// optional and Ze does not offer, so the condition is false and nothing is
// owed. Both halves of that sentence are checkable, and the check is what keeps
// the kind apart from {not-applicable}, whose judgement nothing in the tree can
// contradict. The QUOTE has to be in the RFC, whitespace aside, so the reader
// can see the document making the feature optional rather than take the
// author's word for it. The PRODUCER has to be findable, so a rename or a
// deletion under the annotation turns the gate red.
//
// An RFC whose text this repository does not hold is REFUSED rather than
// skipped. A quote nobody can check is the assertion this kind exists not to
// be, and enrolment already requires the text (checkEnrolment).
func checkFeatureDeclined(tree string, reader *sourceReader, requirements []Requirement) []string {
	sources := map[string]string{}
	var errs []string
	for _, req := range requirements {
		if req.Annotation == nil || req.Annotation.Kind != AnnotationFeatureDeclined {
			continue
		}
		where := requirementWhere(req)
		errs = append(errs, featureDeclinedQuote(where, req, sources, tree)...)
		state, path, symbol := resolveProducer(reader, req.Annotation.Producer)
		switch state {
		case producerFound:
		case producerUnnamed:
			// Unreachable through the parser, which refuses the line first. The
			// check still refuses it, because a guard that skips the one state
			// it exists to catch is not a guard (ai/rules/principles.md).
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(where).Str(": ").Str(req.RID).
				Str(" is annotated {feature-declined} and names no producer. ").
				Str(featureDeclinedFormat).String())
		case producerFileAbsent:
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(where).Str(": ").Str(req.RID).
				Str(" is annotated {feature-declined} and names the producer ").
				Str(req.Annotation.Producer).
				Str(", whose file this checkout does not carry. The kind rests on code a reader can open: name the function that does the narrower thing ze chose, or the annotation claims what nothing here can show").String())
		case producerSymbolAbsent:
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(where).Str(": ").Str(req.RID).
				Str(" is annotated {feature-declined} and names the producer ").
				Str(req.Annotation.Producer).Str(", but ").Str(path).
				Str(" declares no ").Str(symbol).
				Str(". The producer was renamed or deleted under the annotation: name the function that does the narrower thing ze chose today").String())
		}
	}
	return errs
}

// featureDeclinedQuote holds one annotation's quote against the RFC's own text,
// reading each document at most once.
func featureDeclinedQuote(where string, req Requirement, sources map[string]string,
	tree string,
) []string {
	source, loaded := sources[req.RFC]
	if !loaded {
		source, _ = SourceText(tree, req.RFC)
		sources[req.RFC] = source
	}
	if source == "" {
		var tb textbuf.Buffer
		return []string{tb.Str(where).Str(": ").Str(req.RID).
			Str(" is annotated {feature-declined} and the RFC's own text is not in this repository, so its quote can be checked against nothing. Fetch it to ").
			Str(fullRel).Byte('/').Str(req.RFC).Str(".txt or ").Str(draftsRel).Byte('/').
			Str(req.RFC).Str(".txt").String()}
	}
	if strings.Contains(squashWhitespace(source), squashWhitespace(req.Annotation.Quote)) {
		return nil
	}
	var tb textbuf.Buffer
	return []string{tb.Str(where).Str(": ").Str(req.RID).
		Str(" is annotated {feature-declined} and quotes ").
		Str(pyRepr(truncateRunes(req.Annotation.Quote, 60))).
		Str(", which is not in ").Str(fullRel).Byte('/').Str(req.RFC).
		Str(".txt. The kind rests on the DOCUMENT making the feature optional, so the sentence has to be the RFC's own: quote it verbatim (line breaks are ignored), or the obligation is unconditional and this is a {gap}").String()}
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
