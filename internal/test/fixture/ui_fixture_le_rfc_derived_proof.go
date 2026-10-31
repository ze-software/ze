// Design: docs/contributing/rfc-conformance-gates.md -- the public ledger publishes derived proof only.
// Related: ui_fixture_le_rfc_answers.go -- the scoped export driver these fixtures run in.

package fixture

import (
	"encoding/json"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// leRFCRetiredStem is the summary the retired-cell fixture writes. It is not
// the partial fixture's stem, so the two fixtures never read each other's row.
const (
	leRFCRetiredStem    = "rfc99998"
	leRFCRetiredSummary = "rfc/short/" + leRFCRetiredStem + ".md"
	leRFCRetiredLabel   = "Support status"
	// leRFCRetiredMeta is a Meta table in the shape the derived ledger
	// accepts: classification (Support, Support area, Use cases) and no
	// authored status, coverage or remaining cell.
	leRFCRetiredMeta = "# RFC 99998\n\n## Meta\n\n| Field | Value |\n|---|---|\n| Use cases | - |\n| Optional use cases | - |\n" +
		"| Title | Retired Cell Fixture |\n| Enrolment | backlog |\n" +
		"| Enrolment reason | isolated tooling fixture |\n| Implementation | ze |\n" +
		"| Implementation reason | the fixture's own Go boundary |\n| Support | bgp-core |\n" +
		"| Support area | Retired Cell Fixture |\n"
	// leRFCStatusPage is statusRel in internal/le/rfc/ledger.go: the page
	// `le rfc update` writes.
	leRFCStatusPage = "docs/features/rfc-status.md"
)

// leRFCRetiredSupportCell proves `le rfc check` refuses a summary that still
// carries the retired `Support status` cell, in one message that names the
// summary and the label, says the cell is retired and the proof is derived,
// and gives the fix: delete the row.
//
// Method: a summary whose Meta table is otherwise in the derived shape gains
// one `Support status` row; the JSON answer must exit 1 and hold one string
// carrying every one of those facts.
func leRFCRetiredSupportCell(t leRFCPartialTree) string {
	t.write(leRFCRetiredSummary, leRFCRetiredMeta+"| "+leRFCRetiredLabel+" | Partial |\n\n## Compliance Checklist\n\n")
	code, report := t.checkJSON("retired cell check")
	leRFCAnswersRequire(code == 1, "a summary carrying %q was not refused: exit %d", leRFCRetiredLabel, code)
	facts := []string{leRFCRetiredSummary, leRFCRetiredLabel, "retired", "derived", "delete the row"}
	for _, text := range leRFCJSONStrings(report) {
		if leRFCHoldsAll(text, facts) {
			return "OK: a retired Support status cell is refused with its fix"
		}
	}
	encoded, err := json.Marshal(report["violations"])
	leRFCAnswersRequireNoError(err, "encode violations")
	leRFCAnswersRequire(false, "no refusal names all of %q: violations=%s", facts, encoded)
	return ""
}

// leRFCStatusTable is one table of the status page, by its header cells.
type leRFCStatusTable struct {
	header []string
}

// leRFCTreeRow is the part of one `le rfc tree | json` row this fixture reads.
type leRFCTreeRow struct {
	Stem   string `json:"stem"`
	Walked bool   `json:"walked"`
	Proven int    `json:"mandatory-proven"`
	Total  int    `json:"mandatory-total"`
}

// leRFCWideHeader and leRFCBriefHeader are the two table shapes of the derived
// status page. Neither carries an authored Status, Coverage or Remaining cell.
var (
	leRFCWideHeader  = []string{"RFC", "Area", "Proof", "Evidence", "Unmet"}
	leRFCBriefHeader = []string{"Standard", "Area", "Proof", "Unmet"}
	// leRFCEvidenceKinds is the evidence vocabulary, strongest first
	// (carriers.go::CarrierFor).
	leRFCEvidenceKinds = []string{"interop", "functional", "editor", "unit"}
	// leRFCGatedGapRow matches a gated requirement row carrying a {gap}.
	leRFCGatedGapRow  = regexp.MustCompile(`^- \[[ x]\] \[([^\]]+)\] \[(MUST|MUST NOT|SHALL|SHALL NOT|REQUIRED)\] .*\{gap[:}]`)
	leRFCNumberedStem = regexp.MustCompile(`^rfc(\d+)$`)
)

// leRFCDerivedProofRender proves `le rfc update` renders the status page from
// derived proof: no Status column, and per RFC a Proof cell (`N of M MUST
// proven`, or `not walked`), an Evidence cell (proven rows by kind) and an
// Unmet cell (open requirement ids, or `-`). A brief row (a draft) carries
// Proof and Unmet, so a draft's `{gap}` is still named by its id.
//
// Method: the corpus is the fixture's private HEAD export. `le rfc tree`
// answers each RFC's verified-record figure independently of the page, and
// the fixture picks three RFCs from it by property: one walked with every
// gated MUST proven, one unwalked, and one walked whose summary holds a gated
// `{gap}` row, plus one draft whose summary holds a gated `{gap}` row. Each
// one's page row must state the tree's figure and name its gaps.
func leRFCDerivedProofRender(t leRFCPartialTree) string {
	tree := t.runLE("rfc", "tree", "|", renderJSON)
	leRFCAnswersRequire(tree.code == 0, "rfc tree failed: %s", tree.stderr)
	var answer struct {
		Rows []leRFCTreeRow `json:"rows"`
	}
	leRFCAnswersRequireNoError(json.Unmarshal([]byte(tree.stdout), &answer), "decode rfc tree")
	update := t.runLE("rfc", actionUpdate)
	leRFCAnswersRequire(update.code == 0, "rfc update refused the export: %s%s", update.stdout, update.stderr)
	wide, brief := leRFCStatusTables(string(leRFCAnswersRead(t.tree, leRFCStatusPage)))

	full, unwalked, gapped, draftGapped := false, false, false, false
	for index := range answer.Rows {
		row := &answer.Rows[index]
		if cells, held := brief[row.Stem]; held && !draftGapped {
			draftGapped = leRFCRequireDraftGaps(t, row, cells)
		}
		cells, held := wide[leRFCRowName(row.Stem)]
		if !held {
			continue
		}
		switch {
		case !full && row.Walked && row.Total > 0 && row.Proven == row.Total:
			leRFCRequireFullRow(row, cells)
			full = true
		case !unwalked && !row.Walked:
			leRFCAnswersRequire(cells[2] == "not walked", "%s is unwalked but its Proof reads %q", row.Stem, cells[2])
			leRFCAnswersRequire(!strings.ContainsAny(cells[3], "0123456789"), "%s is unwalked but its Evidence carries a number: %q", row.Stem, cells[3])
			unwalked = true
		case !gapped && row.Walked && row.Proven < row.Total:
			gaps := leRFCGatedGaps(t, row.Stem)
			if len(gaps) == 0 {
				continue
			}
			leRFCAnswersRequire(cells[2] == leRFCProofCell(row.Proven, row.Total), "%s Proof reads %q, the tree says %d of %d", row.Stem, cells[2], row.Proven, row.Total)
			for _, rid := range gaps {
				leRFCAnswersRequire(strings.Contains(cells[4], rid), "%s Unmet %q omits the gap %s", row.Stem, cells[4], rid)
			}
			gapped = true
		}
	}
	leRFCAnswersRequire(full, "the export holds no walked, fully proven RFC in a wide table")
	leRFCAnswersRequire(unwalked, "the export holds no unwalked RFC in a wide table")
	leRFCAnswersRequire(gapped, "the export holds no walked RFC with a gated {gap} in a wide table")
	leRFCAnswersRequire(draftGapped, "the export holds no draft with a gated {gap} in a brief table")
	return "OK: the status page states derived proof, evidence and unmet rows, and no Status"
}

// leRFCRequireDraftGaps checks one brief row: when the draft's summary holds a
// gated `{gap}`, its Proof states the tree's figure and its Unmet names every
// such id. It answers whether the row was checked; a draft with no gated gap
// answers false so the caller tries the next one.
func leRFCRequireDraftGaps(t leRFCPartialTree, row *leRFCTreeRow, cells []string) bool {
	gaps := leRFCGatedGaps(t, row.Stem)
	if len(gaps) == 0 {
		return false
	}
	proof := "not walked"
	if row.Walked {
		proof = leRFCProofCell(row.Proven, row.Total)
	}
	leRFCAnswersRequire(cells[2] == proof, "draft %s Proof reads %q, the tree says %q", row.Stem, cells[2], proof)
	for _, rid := range gaps {
		leRFCAnswersRequire(strings.Contains(cells[3], rid), "draft %s Unmet %q omits the gap %s", row.Stem, cells[3], rid)
	}
	return true
}

// leRFCRequireFullRow checks the row of a walked RFC whose every gated MUST a
// verified record proves: its figure, evidence kinds summing to it, nothing unmet.
func leRFCRequireFullRow(row *leRFCTreeRow, cells []string) {
	leRFCAnswersRequire(cells[2] == leRFCProofCell(row.Proven, row.Total), "%s Proof reads %q, the tree says %d of %d", row.Stem, cells[2], row.Proven, row.Total)
	sum := 0
	for part := range strings.SplitSeq(cells[3], ", ") {
		kind, count, found := strings.Cut(part, " ")
		leRFCAnswersRequire(found, "%s Evidence %q is not `<kind> <count>` pairs", row.Stem, cells[3])
		leRFCAnswersRequire(slices.Contains(leRFCEvidenceKinds, kind), "%s Evidence names kind %q outside %q", row.Stem, kind, leRFCEvidenceKinds)
		number, err := strconv.Atoi(count)
		leRFCAnswersRequireNoError(err, row.Stem+" Evidence count")
		sum += number
	}
	leRFCAnswersRequire(sum == row.Proven, "%s Evidence %q sums to %d, the tree proves %d", row.Stem, cells[3], sum, row.Proven)
	leRFCAnswersRequire(cells[4] == "-", "%s is fully proven but its Unmet reads %q", row.Stem, cells[4])
}

// leRFCStatusTables reads every table of the status page, refuses a header in
// neither derived shape, and answers the wide and the brief tables' rows by
// their first cell.
func leRFCStatusTables(page string) (wide, brief map[string][]string) {
	wide, brief = map[string][]string{}, map[string][]string{}
	var current *leRFCStatusTable
	for line := range strings.SplitSeq(page, "\n") {
		// The separator row sits inside its table, and `|-----|` does not open
		// with "| ", so it is skipped before the test that ends a table.
		if strings.HasPrefix(line, "|-") || strings.HasPrefix(line, "| -") {
			continue
		}
		if !strings.HasPrefix(line, "| ") {
			current = nil
			continue
		}
		cells := leRFCCells(line)
		if current == nil {
			leRFCAnswersRequire(!slices.Contains(cells, "Status"), "%s still prints a Status column: %q", leRFCStatusPage, line)
			brief := slices.Equal(cells, leRFCBriefHeader)
			leRFCAnswersRequire(brief || slices.Equal(cells, leRFCWideHeader), "%s header %q is neither %q nor %q", leRFCStatusPage, line, leRFCWideHeader, leRFCBriefHeader)
			current = &leRFCStatusTable{header: cells}
			continue
		}
		leRFCAnswersRequire(len(cells) == len(current.header), "%s row %q has %d cells under a %d-cell header", leRFCStatusPage, line, len(cells), len(current.header))
		if slices.Equal(current.header, leRFCWideHeader) {
			wide[cells[0]] = cells
		} else {
			brief[cells[0]] = cells
		}
	}
	return wide, brief
}

// leRFCCells splits one markdown table line into its trimmed cells.
func leRFCCells(line string) []string {
	inner := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(line), "|"), "|")
	parts := strings.Split(inner, " | ")
	for index := range parts {
		parts[index] = strings.TrimSpace(parts[index])
	}
	return parts
}

// leRFCRowName is the name the status page gives an RFC row: `RFC 1334` for
// rfc1334. A stem in another form answers "" and is never selected.
func leRFCRowName(stem string) string {
	match := leRFCNumberedStem.FindStringSubmatch(stem)
	if match == nil {
		return ""
	}
	return "RFC " + match[1]
}

// leRFCProofCell is the derived Proof cell of a walked RFC.
func leRFCProofCell(proven, total int) string {
	return strconv.Itoa(proven) + " of " + strconv.Itoa(total) + " MUST proven"
}

// leRFCGatedGaps answers the ids of a summary's gated rows carrying a {gap}.
func leRFCGatedGaps(t leRFCPartialTree, stem string) []string {
	var out []string
	for line := range strings.SplitSeq(string(leRFCAnswersRead(t.tree, "rfc/short/"+stem+".md")), "\n") {
		if match := leRFCGatedGapRow.FindStringSubmatch(line); match != nil {
			out = append(out, match[1])
		}
	}
	return out
}

// leRFCJSONStrings answers every string a decoded JSON value holds, at any depth.
func leRFCJSONStrings(value any) []string {
	var out []string
	var walk func(any)
	walk = func(node any) {
		switch typed := node.(type) {
		case string:
			out = append(out, typed)
		case []any:
			for _, item := range typed {
				walk(item)
			}
		case map[string]any:
			for _, item := range typed {
				walk(item)
			}
		}
	}
	walk(value)
	return out
}

// leRFCHoldsAll reports whether text contains every fact, ignoring case.
func leRFCHoldsAll(text string, facts []string) bool {
	lower := strings.ToLower(text)
	for _, fact := range facts {
		if !strings.Contains(lower, strings.ToLower(fact)) {
			return false
		}
	}
	return true
}
