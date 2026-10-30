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
