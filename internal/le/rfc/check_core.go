// Design: docs/architecture/core-design.md -- the rfc area, as one command
// Related: check.go -- the centralized check driver that orders these checks
//
// check_core.go evaluates the live requirement list and forward lineage. These
// checks need no committed baseline.
package rfc

import (
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// annotationBarsATest answers the kinds whose claim is contradicted by a tagged
// test on the same requirement.
//
// Each of the five says NO test carries this id, for a different reason: the
// obligation does not bind Ze, Ze does not meet it, a layer under Ze meets it
// and Ze's own boundary holds nothing to assert, its condition is a feature
// Ze declined, or its status is derived from other rows and a test on it would
// claim more than its body checks. A tag falsifies all five the same way, so
// the annotation is stale rather than the tag being wrong. It is also what
// keeps {lower-layer}, {feature-declined} and {rollup} out of the proven
// numerator by more than bookkeeping: a requirement Ze can prove is one this
// annotation MUST NOT cover.
func annotationBarsATest(kind string) bool {
	return kind == AnnotationNotApplicable || kind == AnnotationGap ||
		kind == AnnotationLowerLayer || kind == AnnotationFeatureDeclined ||
		kind == AnnotationRollup
}

// evaluate answers one finding per coverage violation, in the PARTS it had
// before it formatted them.
//
// It is the producer of 148 of this tree's 155 findings, which is why it
// carries parts and the other checks carry their message alone: a published
// table of columns is worth having for the population that fills it, and
// re-parsing the sentence back into fields would be a second reader of a format
// nobody declared (owner review, 2026-09-01).
//
// It also FILLS Requirement.Derived and DerivedCause on every
// {rollup} row, in place, through the slice it was given (rollupDeriver.fill):
// the derivation reads the state this loop computes for the other rows, so no
// second reader of the tags exists. A rollup raises no finding: what it
// derives from is reported once, under the target's own id. A renderer fills
// the same fields through deriveRollups.
func evaluate(requirements []Requirement, tags []Tag, enrolled map[string]bool) []Finding {
	known := map[string]bool{}
	for _, req := range requirements {
		known[req.RID] = true
	}
	var errs []Finding
	for _, tag := range tags {
		if known[tag.RID] {
			continue
		}
		var tb textbuf.Buffer
		errs = append(errs, note(tb.Str(tag.File).Byte(':').Int(int64(tag.Line)).
			Str(": unknown RFC requirement: ").Str(tag.RID).String()))
	}
	// A tag naming no requirement is keyed under an id no row reads.
	byRID := tagsByRID(tags)
	newRollupDeriver(requirements, byRID, enrolled).fill(requirements)
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
			// A rollup raises no finding of its own: fill wrote its derived
			// state and cause onto the row, and every state it can derive is
			// already reported once under the target's own id, a {gap} by
			// the Remaining cell and an unproven row by the arm below
			// (owner decision, 2026-09-15).
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
		slices.Sort(missing)
		for _, value := range missing {
			held := make([]string, 0, len(polarity))
			for current := range polarity {
				held = append(held, current)
			}
			slices.Sort(held)
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

// checkRollupTargets holds every {rollup} annotation against the corpus: each
// target has to be a row of an enrolled summary or an enrolled summary itself,
// no target is the row itself, and no walk over rollup-to-rollup targets leads
// back to it.
//
// The parser holds a target to its FORM alone, because it reads one line and
// the corpus is loaded after. This is where the target is shown to exist, and
// the refusal is what keeps the kind apart from {not-applicable}: a rollup
// over a row nobody can open is a judgement, and the kind's value is that a
// reader can check it (ai/rules/principles.md). An unenrolled target is
// refused too, because the gate holds no state for it to derive from.
//
// The parser stores no id-versus-stem classification, and a draft stem
// matches idRE as well as stemRE, so a target is resolved against BOTH the id
// set and the stem set rather than by its shape. A name in both sets is read
// as an id: the id set is consulted first, and that order is the whole rule.
//
// The cycle walk (R-1 in the spec) runs per row, from each target that is a
// rollup, following id targets that are rollups and stopping at every other
// target: a stem expands to the rows of a summary and never to a rollup. Every
// step adds one rollup the walk has not seen, so its depth is bounded by the
// number of rollups in the corpus, and it refuses when it meets the row it
// started from. A row in the cycle is refused on its own walk, so two rollups
// naming each other produce one refusal each, and a rollup that merely leads
// INTO a cycle is silent here: the rows in the cycle already carry the
// finding, and its own targets are shown to exist above.
func checkRollupTargets(requirements []Requirement, enrolled map[string]bool) []string {
	stems := map[string]bool{}
	ids := map[string]string{}
	rollups := map[string][]string{}
	for _, req := range requirements {
		stems[req.RFC] = true
		ids[req.RID] = req.RFC
		if req.Rollup() {
			rollups[req.RID] = req.Annotation.Targets
		}
	}
	var errs []string
	for _, req := range requirements {
		if !req.Rollup() {
			continue
		}
		where := requirementWhere(req)
		for _, target := range req.Annotation.Targets {
			var tb textbuf.Buffer
			tb.Str(where).Str(": ").Str(req.RID).Str(" is annotated {rollup} and names ")
			if target == req.RID {
				errs = append(errs, tb.Str("itself. A rollup derives from OTHER rows; drop the target. ").
					Str(rollupFormat).String())
				continue
			}
			tb.Str(pyRepr(target)).Str(", ")
			if stem, isID := ids[target]; isID {
				if !enrolled[stem] {
					errs = append(errs, tb.Str("a row of ").Str(stem).
						Str(", which is not enrolled. A rollup derives only from gated rows: enroll ").
						Str(stem).Str(" first, or drop the target. ").Str(rollupFormat).String())
					continue
				}
				if leadsBackTo(rollups, target, req.RID) {
					errs = append(errs, tb.Str("a rollup whose targets lead back to ").Str(req.RID).
						Str(". A rollup derives from rows that derive from nothing that includes it; break the cycle. ").
						Str(rollupFormat).String())
				}
				continue
			}
			if enrolled[target] {
				continue
			}
			if stems[target] {
				errs = append(errs, tb.Str("a summary that is not enrolled. A rollup derives only from gated rows: enroll ").
					Str(target).Str(" first, or drop the target. ").Str(rollupFormat).String())
				continue
			}
			errs = append(errs, tb.Str("which no summary holds as a requirement id and no summary answers to as a stem. ").
				Str("Name a row or a summary this corpus can show. ").Str(rollupFormat).String())
		}
	}
	return errs
}

// leadsBackTo answers whether following rollup targets from start reaches
// origin.
//
// The walk is breadth-first over rollups keyed by id, and seen holds every
// rollup it has queued, so each rollup is queued at most once and the walk
// ends after at most len(rollups) steps whatever the graph's shape. A target
// that is not a rollup is a leaf: a plain row, a stem, or a target the caller
// refused separately, none of which leads anywhere.
func leadsBackTo(rollups map[string][]string, start, origin string) bool {
	seen := map[string]bool{start: true}
	queue := []string{start}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, target := range rollups[current] {
			if target == origin {
				return true
			}
			if _, isRollup := rollups[target]; !isRollup {
				continue
			}
			if seen[target] {
				continue
			}
			seen[target] = true
			queue = append(queue, target)
		}
	}
	return false
}

// rollupVerdict is one derived state and, where the state is not met, the
// sentence naming the first row that decided it.
type rollupVerdict struct {
	State RollupState
	Cause string
}

// rollupDeriver answers the derived state of every {rollup} row from the state
// evaluate computed for the rows they name.
//
// states is what each gated, enrolled row that is not a rollup contributes;
// targets is every rollup's target list, enrolled or not; stemRows is the gated,
// non-rollup ids of each enrolled summary, in summary order, which is what a
// stem target expands to. memo holds every rollup already answered and
// visiting every rollup on the path being answered, so a derivation enters
// each rollup at most once.
type rollupDeriver struct {
	states   map[string]RollupState
	targets  map[string][]string
	stemRows map[string][]string
	enrolled map[string]bool
	memo     map[string]rollupVerdict
	visiting map[string]bool
}

// deriveRollups fills Requirement.Derived on every {rollup} row of
// requirements, in place, for a renderer.
//
// It is the fill evaluate performs, without the findings: those are the gate's
// to report (Check), and a page that repeated them would be a second gate.
func deriveRollups(requirements []Requirement, tags []Tag, enrolled map[string]bool) {
	newRollupDeriver(requirements, tagsByRID(tags), enrolled).fill(requirements)
}

// newRollupDeriver makes one pass over the rows and records what
// each contributes, so derive reads a map rather than the tags.
func newRollupDeriver(requirements []Requirement, byRID map[string][]Tag,
	enrolled map[string]bool) *rollupDeriver {
	deriver := &rollupDeriver{
		states:   map[string]RollupState{},
		targets:  map[string][]string{},
		stemRows: map[string][]string{},
		enrolled: enrolled,
		memo:     map[string]rollupVerdict{},
		visiting: map[string]bool{},
	}
	for _, req := range requirements {
		// Every rollup is recorded, enrolled or not: a renderer prints every
		// summary, and a rollup row it reads MUST carry a derived state
		// (Requirement.DerivedMark). What a rollup derives FROM is the
		// enrolled, gated rows alone, so a rollup in a summary nobody
		// enrolled derives unproven from its own rows, and
		// checkRollupTargets names the row for the same reason.
		if req.Rollup() {
			deriver.targets[req.RID] = req.Annotation.Targets
			continue
		}
		if !enrolled[req.RFC] {
			continue
		}
		if !req.Gated() {
			continue
		}
		polarity := map[string]bool{}
		for _, tag := range byRID[req.RID] {
			polarity[tag.Polarity] = true
		}
		deriver.states[req.RID] = rollupTargetState(req, polarity)
		deriver.stemRows[req.RFC] = append(deriver.stemRows[req.RFC], req.RID)
	}
	return deriver
}

// rollupTargetState answers what one gated row contributes to a rollup that
// names it: met when the row is proven or excused, a gap when it is one, and
// unproven otherwise.
//
// A {single-polarity} row is met by the one test its annotation requires. An
// excusing annotation is met even beside a stale tag: the stale finding is the
// row's own, and either the annotation or the test meets the obligation. A
// {not-applicable} row is met because an obligation excluded from Ze is not
// one a rollup can owe; the exclusion is that row's claim, reviewed there.
func rollupTargetState(req Requirement, polarity map[string]bool) RollupState {
	annotation := req.Annotation
	if annotation == nil {
		if polarity[PolarityPositive] && polarity[PolarityNegative] {
			return RollupMet
		}
		return RollupUnproven
	}
	switch annotation.Kind {
	case AnnotationGap:
		return RollupGap
	case AnnotationSinglePolarity:
		if polarity[annotation.Polarity] {
			return RollupMet
		}
		return RollupUnproven
	case AnnotationNotApplicable, AnnotationLowerLayer, AnnotationFeatureDeclined:
		return RollupMet
	default:
		// A kind this switch does not name contributes nothing a rollup can
		// rest on. The parser refuses an unknown kind and the builder above
		// never passes a rollup here, so this arm is the closed answer
		// rather than a case.
		return RollupUnproven
	}
}

// fill writes the derived state and its cause onto every rollup row, whatever
// its level and whether or not its summary is enrolled, so a renderer can
// show them. This is the ONE place Derived and DerivedCause are written.
func (d *rollupDeriver) fill(requirements []Requirement) {
	for index := range requirements {
		req := &requirements[index]
		if !req.Rollup() {
			continue
		}
		verdict := d.derive(req.RID)
		req.Derived = verdict.State
		req.DerivedCause = verdict.Cause
	}
}

// derive answers one rollup's state.
//
// The recursion is over rollup-to-rollup targets only, and it is bounded by
// the number of rollups in the corpus: memo answers a rollup already derived
// and does not enter it again, and visiting turns a rollup met on its own path
// into an unproven verdict rather than a second entry. checkRollupTargets
// refuses a rollup ON a cycle, but a rollup that leads INTO one is silent
// there, so the bound is this function's own.
func (d *rollupDeriver) derive(rid string) rollupVerdict {
	if verdict, done := d.memo[rid]; done {
		return verdict
	}
	if d.visiting[rid] {
		return rollupVerdict{State: RollupUnproven,
			Cause: rid + " is a rollup whose derivation leads back into a cycle"}
	}
	d.visiting[rid] = true
	verdict := d.answer(rid)
	delete(d.visiting, rid)
	d.memo[rid] = verdict
	return verdict
}

// answer reads every target of one rollup, in author order, and settles the
// state: the first gap decides, else the first unproven row, else met.
func (d *rollupDeriver) answer(rid string) rollupVerdict {
	var unproven rollupVerdict
	for _, target := range d.targets[rid] {
		for _, one := range d.verdictsOf(target) {
			if one.State == RollupGap {
				return one
			}
			if one.State == RollupUnproven && unproven.State == RollupNone {
				unproven = one
			}
		}
	}
	if unproven.State == RollupUnproven {
		return unproven
	}
	return rollupVerdict{State: RollupMet}
}

// verdictsOf answers what one target contributes: a rollup's own derived
// verdict, a row's state, or one verdict per gated row of a stem.
//
// A target the deriver holds no state for is UNPROVEN, never skipped: a row of
// a summary that is not enrolled, an id that is not gated, and a name that is
// neither a row nor an enrolled summary are each refused by checkRollupTargets
// in the gate, and a unit test that builds rows in code and skips that check
// still cannot derive met from a target nobody can show. A stem that holds no
// gated row is unproven for the same reason: a rollup over nothing is refused
// at parse time, and a stem that expands to nothing is the same rollup.
func (d *rollupDeriver) verdictsOf(target string) []rollupVerdict {
	if _, isRollup := d.targets[target]; isRollup {
		return []rollupVerdict{d.derive(target)}
	}
	if state, held := d.states[target]; held {
		return []rollupVerdict{rollupRowVerdict(target, state)}
	}
	if !d.enrolled[target] {
		return []rollupVerdict{{State: RollupUnproven, Cause: pyRepr(target) +
			" is neither a gated row of an enrolled summary nor an enrolled summary, so the gate holds no state for it"}}
	}
	rows := d.stemRows[target]
	if len(rows) == 0 {
		return []rollupVerdict{{State: RollupUnproven,
			Cause: target + " holds no gated row to derive from"}}
	}
	out := make([]rollupVerdict, 0, len(rows))
	for _, row := range rows {
		out = append(out, rollupRowVerdict(row, d.states[row]))
	}
	return out
}

// rollupRowVerdict words one row's contribution, so the cause names the row
// that owes the work rather than the rollup.
func rollupRowVerdict(rid string, state RollupState) rollupVerdict {
	switch state {
	case RollupGap:
		return rollupVerdict{State: state, Cause: rid + " is annotated {gap}"}
	case RollupUnproven:
		return rollupVerdict{State: state, Cause: rid + " is not proven"}
	default:
		return rollupVerdict{State: state}
	}
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
