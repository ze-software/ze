package rfc

import (
	"strings"
	"testing"
)

// intentOptionality is an RFC sentence carrying an advisory keyword: the
// optionality a {intent: not-ever} reason cites.
const intentOptionality = "A speaker SHOULD log every widget it discards."

// intentRows parses one SHOULD row per marker, so a test reads what the parser
// made of the marker and nothing else.
func intentRows(t *testing.T, markers ...string) []Requirement {
	t.Helper()
	var text strings.Builder
	for index, marker := range markers {
		text.WriteString("- [ ] [RFC9999-2-")
		text.WriteString(string(rune('1' + index)))
		text.WriteString("] [SHOULD] A speaker SHOULD log every widget it discards (§2) ")
		text.WriteString(marker)
		text.WriteString("\n")
	}
	requirements, err := ParseSummaryText(text.String(), "rfc9999", "rfc/short/rfc9999.md")
	if err != nil {
		t.Fatalf("the intent markers did not parse: %v", err)
	}
	return requirements
}

// TestIntentMarkerStates checks AC-5 on the parser. Goal: `{intent: not-yet}`
// and `{intent: not-ever; "<sentence>"; <why>}` each parse into the row's
// Intent, and a row with no marker carries none, which the figures read as not
// decided. Method: three SHOULD rows through ParseSummaryText.
func TestIntentMarkerStates(t *testing.T) {
	rows := intentRows(t,
		"{intent: not-yet}",
		`{intent: not-ever; "`+intentOptionality+`"; Ze keeps no discard log}`,
		"",
	)
	if rows[0].Intent == nil || rows[0].Intent.State != IntentNotYet {
		t.Errorf("not-yet parsed to %+v, want state not-yet", rows[0].Intent)
	}
	never := rows[1].Intent
	if never == nil || never.State != IntentNotEver {
		t.Fatalf("not-ever parsed to %+v, want state not-ever", never)
	}
	if never.Quote != intentOptionality || never.Reason != "Ze keeps no discard log" {
		t.Errorf("not-ever parsed to quote %q, reason %q", never.Quote, never.Reason)
	}
	if rows[2].Intent != nil {
		t.Errorf("a row with no marker parsed to intent %+v, want none: absent is not decided", rows[2].Intent)
	}
	if strings.Contains(rows[0].Text, "intent") || strings.Contains(rows[1].Text, "intent") {
		t.Errorf("the marker stayed in the requirement text: %q, %q", rows[0].Text, rows[1].Text)
	}
}

// TestIntentMarkerComposes checks the marker has its own slot. Goal: an intent
// marker sits beside a coverage annotation and an {updated-by} marker on one
// line, each parsed into its own field. Method: one SHOULD row carrying all
// three.
func TestIntentMarkerComposes(t *testing.T) {
	rows := intentRows(t, "{gap: no discard log yet} {intent: not-yet} {updated-by: RFC10000-3-1; RFC 10000 narrows it}")
	row := rows[0]
	if row.Annotation == nil || row.Annotation.Kind != AnnotationGap {
		t.Errorf("annotation = %+v, want {gap}", row.Annotation)
	}
	if row.Intent == nil || row.Intent.State != IntentNotYet {
		t.Errorf("intent = %+v, want not-yet", row.Intent)
	}
	if row.UpdatedBy == nil {
		t.Error("the {updated-by} marker beside the intent marker did not parse")
	}
}

// TestIntentMarkerRefusals checks the marker's closed form. Goal: an unknown
// state, a not-ever with no reason, no quote, a quote without optionality or
// no why, a not-yet with a payload and a second marker are each refused, the
// refusal naming the fault. Method: one SHOULD row per bad marker.
func TestIntentMarkerRefusals(t *testing.T) {
	cases := []struct {
		name, marker, refused string
	}{
		{"an empty body", "{intent: }", "names no state"},
		{"an unknown state", "{intent: proven}", "unknown intent state 'proven'"},
		{"not-ever with no reason", "{intent: not-ever}", "has no reason"},
		{"not-ever with an empty reason", "{intent: not-ever; }", "has no reason"},
		{"not-ever with no quote", "{intent: not-ever; Ze will never log discards}", "quotes no RFC sentence"},
		{"not-ever whose quote states no optionality", `{intent: not-ever; "A speaker MUST log every widget it discards."; never}`,
			"states no optionality"},
		{"not-ever with no why", `{intent: not-ever; "` + intentOptionality + `"}`, "gives no reason after the quote"},
		{"not-yet with a payload", "{intent: not-yet; soon}", "takes no reason"},
		{"two markers", "{intent: not-yet} {intent: not-yet}", "two {intent} markers"},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			line := "- [ ] [RFC9999-2-1] [SHOULD] A speaker SHOULD log every widget it discards (§2) " + one.marker
			_, err := ParseSummaryText(line, "rfc9999", "rfc/short/rfc9999.md")
			if err == nil {
				t.Fatalf("%s was accepted", one.marker)
			}
			if !strings.Contains(err.Error(), one.refused) {
				t.Errorf("the refusal %q does not say %q", err, one.refused)
			}
		})
	}
}

// TestIntentCounts checks AC-5 on the figures. Goal: the optional rows that
// are owed, not proven and not excluded each land in exactly one of NotYet,
// NotEver and NotDecided, by their marker; a proven row, an excluded row and
// a gated row land in none, and no existing count moves. Method: rows of each
// shape through figuresByStem, with a proof set naming two of them.
func TestIntentCounts(t *testing.T) {
	notYet := &Intent{State: IntentNotYet}
	notEver := &Intent{State: IntentNotEver, Quote: intentOptionality, Reason: "never"}
	requirements := []Requirement{
		{RFC: "rfc1", RID: "RFC1-1-1", Level: levelShould, Intent: notYet},
		{RFC: "rfc1", RID: "RFC1-1-2", Level: levelMay, Intent: notEver},
		{RFC: "rfc1", RID: "RFC1-1-3", Level: levelShould},
		{RFC: "rfc1", RID: "RFC1-1-4", Level: levelShould, Annotation: &Annotation{Kind: AnnotationGap, Reason: "unmet"}},
		// Proven: computed, never declared, so its stale marker counts nowhere.
		{RFC: "rfc1", RID: "RFC1-1-5", Level: levelShould, Intent: notYet},
		{RFC: "rfc1", RID: "RFC1-1-6", Level: levelShould},
		// Excluded: not owed by Ze, so no intent applies.
		{RFC: "rfc1", RID: "RFC1-1-7", Level: levelShould, Annotation: &Annotation{Kind: AnnotationNotApplicable, Reason: "binds a relay"}},
		// Gated: the mandatory figure's row, never an intent.
		{RFC: "rfc1", RID: "RFC1-1-8", Level: levelMust},
		// A second not-yet row, so the three counts differ and a swapped arm shows.
		{RFC: "rfc1", RID: "RFC1-1-9", Level: levelRecommended, Intent: notYet},
	}
	stems := map[string]bool{"rfc1": true}
	proven := map[string]bool{"RFC1-1-5": true, "RFC1-1-6": true}
	figures := figuresByStem(requirements, stems, stems, proven, figureEvidence{})["rfc1"]
	if figures.NotYet != 2 || figures.NotEver != 1 || figures.NotDecided != 2 {
		t.Errorf("NotYet=%d NotEver=%d NotDecided=%d, want 2, 1 and 2", figures.NotYet, figures.NotEver, figures.NotDecided)
	}
	optional := figures.Optional
	if optional.Total != 8 || optional.Proven != 2 || optional.Attainable != 7 || optional.Excluded[AnnotationNotApplicable] != 1 {
		t.Errorf("optional = %+v, want 2 proven of 8, 7 attainable, 1 not-applicable", optional)
	}
	if sum := figures.NotYet + figures.NotEver + figures.NotDecided + optional.Proven; sum != optional.Attainable {
		t.Errorf("intent counts plus proven = %d, want the %d attainable optional rows", sum, optional.Attainable)
	}
	if figures.Mandatory.Total != 1 {
		t.Errorf("mandatory total = %d, want 1", figures.Mandatory.Total)
	}
}
