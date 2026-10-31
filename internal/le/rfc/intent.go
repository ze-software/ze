// Design: docs/contributing/rfc-conformance-gates.md -- the intent marker
// Related: summary.go -- stripMarkers, the slot this marker parses into
// Related: profilecoverage.go -- countIntent, the counts it feeds
//
// intent.go reads the `{intent: ...}` marker: what Ze means to do about an
// optional (SHOULD / MAY) row it has not proven. Two states are declared,
// not yet and not ever. A third, not decided, is the absence of the marker,
// and proven is never declared: it is computed from the discrimination
// records, so no marker can claim it.
package rfc

import (
	"errors"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// IntentKind is the row marker declaring Ze's intent for an optional row. Its
// own slot beside the coverage annotation, like {superseded}: a row can be a
// {gap} and not yet supported at once, and each says a different thing.
const IntentKind = "intent"

// IntentState is a declared intent. The zero value is IntentUnspecified, which
// no marker parses to, so an Intent nobody parsed cannot read as a decision.
type IntentState uint8

const (
	IntentUnspecified IntentState = iota
	// IntentNotYet says Ze means to support the row and has not yet.
	IntentNotYet
	// IntentNotEver says Ze will not support the row. Only the owner sets it;
	// the parser cannot see who wrote a marker, so review enforces that.
	IntentNotEver
)

// The marker words, one per declared state.
const (
	intentNotYetWord  = "not-yet"
	intentNotEverWord = "not-ever"
)

// intentFormat is the one format sentence every intent refusal ends with.
const intentFormat = `Format: {intent: not-yet}, or {intent: not-ever; "<the RFC sentence making the row optional>"; <why Ze will never support it>}`

// String answers the marker word, and nothing for IntentUnspecified.
func (s IntentState) String() string {
	switch s {
	case IntentNotYet:
		return intentNotYetWord
	case IntentNotEver:
		return intentNotEverWord
	case IntentUnspecified:
		return ""
	default:
		panic("BUG: unexpected intent state")
	}
}

// MarshalText writes the marker word, so the JSON row says which state.
func (s IntentState) MarshalText() ([]byte, error) {
	if s == IntentUnspecified {
		return nil, errors.New("rfc: an unspecified intent state has no marker word")
	}
	return []byte(s.String()), nil
}

// UnmarshalText reads the marker word MarshalText writes.
func (s *IntentState) UnmarshalText(text []byte) error {
	switch string(text) {
	case intentNotYetWord:
		*s = IntentNotYet
	case intentNotEverWord:
		*s = IntentNotEver
	default:
		return errors.New("rfc: unknown intent state " + pyRepr(string(text)))
	}
	return nil
}

// Intent is one row's declared intent. Quote and Reason are set on not ever
// only: Quote is the RFC sentence that makes the row optional, Reason why Ze
// will never support it.
type Intent struct {
	State  IntentState `json:"state"`
	Quote  string      `json:"quote,omitempty"`
	Reason string      `json:"reason,omitempty"`
}

// parseIntent reads `{intent: not-yet}` and `{intent: not-ever; "<sentence>";
// <why>}`.
//
// A not-ever marker must cite the RFC's optionality: its quote carries an
// advisory BCP 14 level, or no mandatory one and an optionality word
// (optionalityMarked), the shape a {feature-declined} quote's optionality
// takes. A not-yet marker takes no reason, since it records only that the
// work is owed.
func parseIntent(body, where string) (*Intent, error) {
	var tb textbuf.Buffer
	_, value, _ := strings.Cut(body, ":")
	word, reason, reasoned := strings.Cut(value, ";")
	word, reason = strings.TrimSpace(word), strings.TrimSpace(reason)
	switch word {
	case "":
		return nil, parseErr(tb.Str(where).Str(": {").Str(IntentKind).
			Str("} names no state. ").Str(intentFormat))
	case intentNotYetWord:
		if reasoned {
			return nil, parseErr(tb.Str(where).Str(": {").Str(IntentKind).Str(": ").Str(intentNotYetWord).
				Str("} takes no reason: it records only that the work is owed. ").Str(intentFormat))
		}
		return &Intent{State: IntentNotYet}, nil
	case intentNotEverWord:
		return parseNotEver(reason, where)
	default:
		return nil, parseErr(tb.Str(where).Str(": unknown intent state ").Str(pyRepr(word)).
			Str(". Proven is computed from the discrimination records and never declared, ").
			Str("and an unmarked row is not decided. ").Str(intentFormat))
	}
}

// parseNotEver reads the reason of a not-ever marker: the quoted RFC sentence
// making the row optional, then why.
func parseNotEver(reason, where string) (*Intent, error) {
	var tb textbuf.Buffer
	if reason == "" {
		return nil, parseErr(tb.Str(where).Str(": {").Str(IntentKind).Str(": ").Str(intentNotEverWord).
			Str("} has no reason. Not ever must cite the RFC's optionality and say why Ze will never support the row. ").
			Str(intentFormat))
	}
	if !strings.HasPrefix(reason, `"`) {
		return nil, parseErr(tb.Str(where).Str(": {").Str(IntentKind).Str(": ").Str(intentNotEverWord).
			Str("} quotes no RFC sentence. Quote the sentence that makes the row optional. ").Str(intentFormat))
	}
	parts := featureDeclinedRE.FindStringSubmatch(reason)
	if parts == nil {
		return nil, parseErr(tb.Str(where).Str(": {").Str(IntentKind).Str(": ").Str(intentNotEverWord).
			Str("} gives no reason after the quote. Close the quote, then write `; <why>`. ").Str(intentFormat))
	}
	quote, why := strings.TrimSpace(parts[1]), strings.TrimSpace(parts[2])
	advisory, mandatory := bcp14LevelsIn(quote)
	optional := len(advisory) > 0 || (len(mandatory) == 0 && optionalityMarked(quote))
	if !optional {
		return nil, parseErr(tb.Str(where).Str(": {").Str(IntentKind).Str(": ").Str(intentNotEverWord).
			Str("} quotes ").Str(pyRepr(quote)).
			Str(", which states no optionality: it carries no SHOULD, RECOMMENDED, MAY or OPTIONAL, ").
			Str("and no optional, optionally or if ... supports. Quote the RFC sentence that makes the row optional. ").
			Str(intentFormat))
	}
	return &Intent{State: IntentNotEver, Quote: quote, Reason: why}, nil
}

// intentOnGatedRow refuses an intent marker on a gated row: a MUST-level
// obligation is owed whatever anyone intends, so declaring it not yet or not
// ever would hide an owed row behind an intent.
func intentOnGatedRow(req *Requirement, where string) error {
	if req.Intent == nil {
		return nil
	}
	if !req.Gated() {
		return nil
	}
	var tb textbuf.Buffer
	return parseErr(tb.Str(where).Str(": {").Str(IntentKind).Str("} on a ").Str(req.Level).
		Str(" row. A ").Str(req.Level).Str(" obligation is owed whatever Ze intends, so only an optional row (SHOULD, RECOMMENDED, MAY, OPTIONAL and their negations) carries an intent. ").
		Str("Remove the marker, or record the unmet obligation with {gap: <why>}"))
}

// evaluateIntent refuses an {intent} marker that says nothing true: one on a
// row a layer under Ze performs (osMet), which the kernel already performs and
// which counts OS verified or OS gap instead; one on a row an annotation takes
// out of the rows Ze owes, judged by the rule the figures use
// (figureEvidence.excluded over judge), so a row whose not-owed evidence fails
// stays owed and may carry an intent, as it does in the counts; or one on a
// row a proving record covers, whose state is computed and never declared.
// proven is provenRequirements over the tree's verdicts. judge is the
// exclusion judge the published figures read ((*evidenceReader).notOwedGap).
func evaluateIntent(requirements []Requirement, proven map[string]bool, judge exclusionJudge) []Finding {
	evidence := figureEvidence{exclusions: judge}
	var findings []Finding
	for index := range requirements {
		req := &requirements[index]
		if req.Intent == nil {
			continue
		}
		var why string
		fix := "remove the intent marker from that line"
		switch {
		case osMet(req):
			// Owner, 2026-10-31: the kernel performs a lower-layer row, so
			// it is OS verified or an OS gap, and nothing is left to intend.
			why = "is annotated {" + req.Annotation.Kind + "}: a layer under Ze performs it, so the kernel already performs it and no intent applies"
			fix += ", or reclassify the row if Ze's own code performs it"
		case evidence.excluded(req):
			why = "is annotated {" + req.Annotation.Kind + "}, which says Ze does not owe it, so no intent applies"
		case proven[req.RID]:
			why = "is proven by its discrimination record, and proven is computed, never declared"
		default:
			continue
		}
		var tb textbuf.Buffer
		findings = append(findings, requirementFinding(*req, "carries a stale {intent} marker",
			tb.Str(requirementWhere(*req)).Str(": ").Str(req.RID).Str(" carries {").Str(IntentKind).Str(": ").
				Str(req.Intent.State.String()).Str("} and ").Str(why).
				Str(": ").Str(fix).String()))
	}
	return findings
}

// intentQuoteGap refuses a not-ever marker whose quote is not the RFC's own
// words: it must be a whole sentence of the row's resolved section or its
// subsections (quoteSource.wholeSentence), as a {not-applicable} role quote
// must (roleQuoteGap), so a marker cannot cite optionality the RFC never
// states. "" and false for every other row.
func (reader *evidenceReader) intentQuoteGap(req *Requirement) (string, bool) {
	if req.Intent == nil {
		return "", false
	}
	if req.Intent.State != IntentNotEver {
		return "", false
	}
	var tb textbuf.Buffer
	tb.Str(requirementWhere(*req)).Str(": ").Str(req.RID).Str(" is marked {").Str(IntentKind).Str(": ").
		Str(intentNotEverWord).Str("} and ")
	place, section := placeQuote(loadQuoteSource(reader.sources, reader.tree, req.RFC), req.Section, req.Intent.Quote)
	switch place {
	case quoteNoText:
		return tb.Str("the RFC's own text is not in this repository, so its quote can be checked against nothing; fetch it to ").
			Str(fullRel).Byte('/').Str(req.RFC).Str(".txt. ").Str(intentFormat).String(), true
	case quoteSectionUnresolved:
		return tb.Str("the section it cites, ").Str(req.Section).
			Str(", names no heading of the RFC, so no section holds its quote; cite the section the row is in. ").
			Str(intentFormat).String(), true
	case quoteInSection:
		if loadQuoteSource(reader.sources, reader.tree, req.RFC).wholeSentence(section, req.Intent.Quote) {
			return "", false
		}
	case quoteOutsideSection, quoteNotInRFC:
		// The quote is not in the cited section at all, which no whole
		// sentence of it can be: the refusal below names the cited section.
		section = req.Section
	default:
		panic("BUG: unexpected quote place")
	}
	return tb.Str("its quote ").Str(pyRepr(req.Intent.Quote)).Str(" is not a whole sentence of section ").Str(section).
		Str(" or its subsections: quote the RFC sentence that makes the row optional verbatim, from the word that opens it to the full stop that ends it. ").
		Str(intentFormat).String(), true
}
