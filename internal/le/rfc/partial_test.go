// Design: docs/contributing/rfc-conformance-gates.md -- clause-scoped evidence.
package rfc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const partialSentence = "A widget MUST be zero when sent and ignored on receipt."
const partialMarker = `{partial: tested "ignored on receipt."; gap "MUST be zero when sent"; native sending is absent at ` + stampProducerKey + `}`

func partialRow(t *testing.T) Requirement {
	t.Helper()
	req, err := parseChecklistLine("- [ ] ["+selftestRIDSend+"] [MUST] "+partialSentence+" (§2) "+partialMarker, selftestStem, selftestSummaryRel, 1)
	if err != nil {
		t.Fatal(err)
	}
	return *req
}

func partialFiles() map[string]string {
	return map[string]string{
		selftestSummaryRel:     "# RFC 9999\n\n" + strings.Replace(selftestMeta, "Zero MUST gaps.", "One MUST row remains unmet: native sending is absent.", 1) + "\n## Compliance Checklist\n\n- [ ] [" + selftestRIDSend + "] [MUST] " + partialSentence + " (§2) " + partialMarker + "\n",
		"rfc/full/rfc9999.txt": "Test RFC 9999\n\n2.  Widgets\n\n" + partialSentence + "\n",
		selftestProducerPath:   selftestProducerSource,
		selftestCIPath:         "# RFC requirement: " + selftestRIDSend + " positive -- receipt accepts zero\n# RFC requirement: " + selftestRIDSend + " negative -- receipt ignores nonzero\nexpect=stdout:contains=OK\n",
	}
}

// The public checker and shard retain full source text while reporting scoped coverage.
func TestPartialRequirementCheckReportsScope(t *testing.T) {
	root := checkFixtureTree(t, partialFiles())
	report, code := Check(root, nil)
	if code != 0 || report.Partial != 1 || report.Gated != 1 {
		t.Fatalf("partial check: code=%d report=%+v\n%s", code, report, report.Text())
	}
	files := partialFiles()
	files[selftestCIPath] = "# RFC requirement: " + selftestRIDSend + " positive -- receipt accepts zero\nexpect=stdout:contains=OK\n"
	incomplete, incompleteCode := Check(checkFixtureTree(t, files), nil)
	if incompleteCode != 2 || incomplete.Partial != 1 || incomplete.Gated != 1 ||
		len(incomplete.PartialScopes) != 1 || incomplete.PartialScopes[0].RID != selftestRIDSend ||
		len(incomplete.Violations) == 0 {
		t.Fatalf("missing negative proof concealed scope or accepted the row: code=%d report=%+v", incompleteCode, incomplete)
	}
	collected, err := Collect(root)
	if err != nil {
		t.Fatal(err)
	}
	in, err := NewRenderInput(root, collected, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	row := RequirementRows(in)[selftestStem][0]
	for _, want := range []string{"ignored on receipt.", "MUST be zero when sent", "Scoped evidence", "zero whole-requirement credit"} {
		if !strings.Contains(row.Note, want) {
			t.Errorf("note omits %q: %s", want, row.Note)
		}
	}
	if row.Positive == "--" || row.Negative == "--" {
		t.Fatal("scoped links lost")
	}
	req := collected.Requirements[0]
	if req.Text != partialSentence+" (§2)" || req.RID != selftestRIDSend || req.Section != "2" || req.Level != levelMust {
		t.Fatalf("identity/source changed: %+v", req)
	}
	for n := range 3 {
		findings := evaluate([]Requirement{req}, collected.Tags[:n], collected.Enrolled)
		if (len(findings) == 0) != (n == 2) {
			t.Errorf("%d polarities: %+v", n, findings)
		}
	}
	if got := rollupTargetState(req, map[string]bool{PolarityPositive: true, PolarityNegative: true}); got != RollupGap {
		t.Fatalf("partial derives %v", got)
	}
	req.Annotation = &Annotation{Kind: AnnotationGap, Reason: "absent"}
	if len(evaluate([]Requirement{req}, collected.Tags, collected.Enrolled)) == 0 {
		t.Fatal("ordinary gap accepted tags")
	}
}

// Malformed selectors fail closed, including duplicate markers and quoted delimiters.
func TestPartialSelectorsRejectInvalidScope(t *testing.T) {
	for name, marker := range map[string]string{
		"empty":          strings.Replace(partialMarker, `"ignored on receipt."`, `""`, 1),
		"absent":         strings.Replace(partialMarker, "ignored on receipt.", "dropped on receipt.", 1),
		"overlap":        strings.Replace(partialMarker, `"MUST be zero when sent"`, `"on receipt."`, 1),
		"whole parent":   strings.Replace(partialMarker, `"ignored on receipt."`, `"`+partialSentence+`"`, 1),
		"word boundary":  strings.Replace(partialMarker, "ignored on receipt.", "gnored on receipt.", 1),
		"bad escape":     strings.Replace(partialMarker, "ignored", `\qignored`, 1),
		"no reason":      `{partial: tested "ignored on receipt."; gap "MUST be zero when sent"; }`,
		"no producer":    `{partial: tested "ignored on receipt."; gap "MUST be zero when sent"; missing sending}`,
		"producer alone": `{partial: tested "ignored on receipt."; gap "MUST be zero when sent"; internal/widget/send.go::SendWidget}`,
		"test producer":  strings.Replace(partialMarker, ".go::", "_test.go::", 1),
		"traversal":      strings.Replace(partialMarker, selftestProducerPath, "../"+selftestProducerPath, 1),
		"duplicate":      partialMarker + " {gap: absent}",
		"brace":          strings.Replace(partialMarker, "native", "{native}", 1),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parseChecklistLine("- [ ] ["+selftestRIDSend+"] [MUST] "+partialSentence+" (§2) "+marker, selftestStem, selftestSummaryRel, 1)
			if err == nil {
				t.Fatal("invalid scope accepted")
			}
			if !isParseError(err) {
				t.Fatalf("not a named parse refusal: %v", err)
			}
		})
	}
	ambiguous := partialRow(t)
	ambiguous.Text = partialSentence + " ignored on receipt. (§2)"
	if !strings.Contains(partialScopeRefusal(&ambiguous), "exactly once") {
		t.Fatal("ambiguous selector accepted")
	}
	touching := partialRow(t)
	touching.Text = "A widget MUST be zero when sent;ignored on receipt. (§2)"
	touching.Annotation.Gap = "MUST be zero when sent;"
	if why := partialScopeRefusal(&touching); why != "" {
		t.Fatalf("touching, non-overlapping spans refused: %s", why)
	}
	repeated := partialRow(t)
	repeated.Text = "MUST inspect aaaa and ignore receipt. (§2)"
	repeated.Annotation.Tested = "aaa"
	repeated.Annotation.Gap = "ignore receipt."
	if !strings.Contains(partialScopeRefusal(&repeated), "exactly once") {
		t.Fatal("overlapping repeated selector accepted")
	}
	quoted := `A widget MUST send "x;y" and ignore "a;b" on receipt.`
	marker := `{partial: tested "ignore \"a;b\" on receipt."; gap "MUST send \"x;y\""; sending absent at internal/widget/send.go::SendWidget}`
	if _, err := parseChecklistLine("- [ ] ["+selftestRIDSend+"] [MUST] "+quoted+" (§2) "+marker, selftestStem, "fixture", 1); err != nil {
		t.Fatal(err)
	}
	for _, length := range []int{23, 24, 25} {
		quote := "MUST " + strings.Repeat("x", length-5)
		req := Requirement{RFC: selftestStem, RID: selftestRIDSend, Text: quote + " (§2)", Section: "2"}
		_, refused := rowQuoteRefusal(&req, newQuoteSource("2.  Widgets\n\n"+quote))
		if refused != (length < 24) {
			t.Errorf("quote length %d refused=%t", length, refused)
		}
	}
	root := t.TempDir()
	req := partialRow(t)
	if len(checkPartialScopes(newSourceReader(root), []Requirement{req})) == 0 {
		t.Fatal("absent producer accepted")
	}
	writeFixtureFiles(t, root, map[string]string{selftestProducerPath: selftestProducerSource})
	if errs := checkPartialScopes(newSourceReader(root), []Requirement{req}); len(errs) != 0 {
		t.Fatal(errs)
	}
}

// One partial row remains counted once and earns no whole proof even with invalid enforced data.
func TestPartialNeverMovesWholeProofNumerator(t *testing.T) {
	req := partialRow(t)
	tags := pairTags(req.RID)
	row := CoverageRows([]Requirement{req}, tags, nil)[0]
	if row.Gated != 1 || row.Annotated != 1 || row.Partial != 1 || row.Both+row.One+row.Missing != 0 {
		t.Fatalf("partition: %+v", row)
	}
	if cell := proofCell(row); !strings.Contains(cell, "0 proven") || !strings.Contains(cell, "1 partial subset") {
		t.Fatalf("public proof cell loses scope: %s", cell)
	}
	files := partialFiles()
	records := partialRecords(t, files)
	proven, escaped := discriminationRouteCounts(verifyFixture(t, files, nil, records...))
	if proven != 2 || escaped != 0 {
		t.Fatalf("scoped claim records: %d proven, %d escaped", proven, escaped)
	}
	root := checkFixtureTree(t, partialFiles())
	collected, err := Collect(root)
	if err != nil {
		t.Fatal(err)
	}
	share, err := ProvenShareOf(collected.Metas, collected.Requirements, collected.Tags, nil)
	if err != nil {
		t.Fatal(err)
	}
	if share.Proven != 0 || share.Gated != 1 {
		t.Fatalf("whole credit: %+v", share)
	}
	audits := map[string]Audit{selftestStem: {Verdicts: map[string]any{req.RID: map[string]any{"verdict": VerdictEnforced}}}}
	rows, work := auditCoverageRows(auditCoverageInput{Requirements: []Requirement{req}, Tags: tags, Enrolled: map[string]bool{selftestStem: true}, Audits: audits})
	if rows[0].Proven != 0 || rows[0].Findings != 1 || len(work) != 1 {
		t.Fatalf("invalid enforced credited: %+v %+v", rows, work)
	}
	for _, level := range []string{levelMust, "SHOULD"} {
		req.Level = level
		audits[selftestStem].Verdicts[req.RID] = map[string]any{"verdict": VerdictPartial}
		rows, work = auditCoverageRows(auditCoverageInput{Requirements: []Requirement{req}, Enrolled: map[string]bool{selftestStem: true}, Audits: audits})
		if rows[0].Findings != 1 || len(work) != 1 {
			t.Fatalf("partial finding hidden at %s", level)
		}
	}
}

// Disclosure counts unmet rows, without changing the ordinary gap demonstration population.
func TestPartialDisclosureCountsRows(t *testing.T) {
	req := partialRow(t)
	enrolled := map[string]bool{selftestStem: true}
	if len(checkStatusAgreement([]Requirement{req}, nil, enrolled)) == 0 {
		t.Fatal("missing public row accepted")
	}
	rows := map[string]LedgerRow{selftestStem: {Status: "Supported", Remaining: "None"}}
	if len(checkStatusAgreement([]Requirement{req}, rows, enrolled)) == 0 {
		t.Fatal("clean support accepted")
	}
	rows[selftestStem] = LedgerRow{Status: "Partial", Remaining: "One MUST row remains unmet: sending absent"}
	if errs := checkStatusAgreement([]Requirement{req}, rows, enrolled); len(errs) != 0 {
		t.Fatal(errs)
	}
	if errs := checkGapCountAgreement([]Requirement{req}, rows); len(errs) != 0 {
		t.Fatal(errs)
	}
	rows[selftestStem] = LedgerRow{Status: "Partial", Remaining: "Zero MUST rows remain unmet"}
	if len(checkGapCountAgreement([]Requirement{req}, rows)) == 0 {
		t.Fatal("omitted partial count accepted")
	}
}

// Scope changes do not alter extraction inventory or native claim identities.
func TestPartialKeepsExtractionAndClaimIdentity(t *testing.T) {
	files := partialFiles()
	record := sealFixture(t, files, DiscriminationRecord{RID: selftestRIDSend, Polarity: PolarityPositive, Unit: selftestCIPath, Producer: stampProducerKey, Route: RouteRevert, Break: selftestBreak, Citation: "expect=stdout:contains=OK"})
	root := checkFixtureTree(t, files)
	before, err := NewDeriver(root).Inventory(selftestStem, 1)
	if err != nil {
		t.Fatal(err)
	}
	extraction, err := os.ReadFile(filepath.Join(root, checkFixtureExtractionRel))
	if err != nil {
		t.Fatal(err)
	}
	plain := strings.Replace(files[selftestSummaryRel], " "+partialMarker, "", 1)
	writeFixtureFiles(t, root, map[string]string{selftestSummaryRel: plain})
	after, err := NewDeriver(root).Inventory(selftestStem, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("scope changed source inventory")
	}
	again, err := os.ReadFile(filepath.Join(root, checkFixtureExtractionRel))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(extraction, again) {
		t.Fatal("scope rewrote extraction")
	}
	if verdicts := verifyFixture(t, files, map[string]string{selftestSummaryRel: plain}, record); !verdicts[0].Verified() {
		t.Fatal("scope changed observation identity")
	}
	changed := strings.Replace(files[selftestCIPath], "receipt accepts zero", "receipt accepts everything", 1)
	if verdicts := verifyFixture(t, files, map[string]string{selftestCIPath: changed}, record); verdicts[0].Verified() {
		t.Fatal("changed tag claim retained proof")
	}
}

// A partial finding uses the same deletion and unchanged-unit upgrade ratchets.
func TestPartialFindingCannotVanish(t *testing.T) {
	req := partialRow(t)
	was := map[string]any{"verdict": VerdictPartial, fingerprintUnits: map[string]any{"test/widget.ci": RequirementSHA("unit")}}
	baseline := map[string]map[string]map[string]any{selftestStem: {req.RID: was}}
	enrolled := map[string]bool{selftestStem: true}
	if len(checkAuditFindings([]Requirement{req}, enrolled, nil, baseline, true)) == 0 {
		t.Fatal("deleted partial finding accepted")
	}
	now := map[string]any{"verdict": VerdictEnforced, fingerprintUnits: was[fingerprintUnits]}
	if rejudgeRefusal(req.RID, was, now) == "" {
		t.Fatal("unchanged-unit upgrade accepted without reason")
	}
	now[verdictFieldUpgradeReason] = "The full sentence is now independently proven."
	if why := rejudgeRefusal(req.RID, was, now); why != "" {
		t.Fatal(why)
	}
	if errs := verdictClaims(selftestStem, req.RID, now, req, pairTags(req.RID)); len(errs) == 0 {
		t.Fatal("reason bypassed standing partial scope")
	}
}

// The machine-readable count remains explicit even at zero.
func TestPartialReportJSONAlwaysCarriesSubset(t *testing.T) {
	body, err := json.Marshal(CheckReport{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"partial":0`) {
		t.Fatalf("partial subset omitted: %s", body)
	}
}

// A malformed marker must not disappear between collection and publication,
// including whitespace accepted before the annotation kind.
func TestMalformedPartialCannotPublishWholeCredit(t *testing.T) {
	for _, space := range []string{"", " ", "\t", "\u00a0"} {
		t.Run(fmt.Sprintf("%q", space), func(t *testing.T) {
			root := checkFixtureTree(t, partialFiles())
			body := partialFiles()[selftestSummaryRel]
			marker := strings.Replace(partialMarker, "{partial", "{"+space+"partial", 1)
			valid := strings.Replace(body, partialMarker, marker, 1)
			writeFixtureFiles(t, root, map[string]string{selftestSummaryRel: valid})
			collected, err := Collect(root)
			if err != nil {
				t.Fatal(err)
			}
			in, err := NewRenderInput(root, collected, nil, nil)
			if err != nil {
				t.Fatalf("accepted kind whitespace refused: %v", err)
			}
			cover := CoverageRows(in.Requirements, in.Tags, in.Carriers)[0]
			if cover.Partial != 1 || cover.Both != 0 {
				t.Fatalf("kind whitespace lost scope: %+v", cover)
			}
			malformed := strings.Replace(marker, "native sending", "missing {native} sending", 1)
			writeFixtureFiles(t, root, map[string]string{selftestSummaryRel: strings.Replace(body, partialMarker, malformed, 1)})
			collected, err = Collect(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(collected.ParseErrors) == 0 {
				t.Fatal("malformed marker was silently parsed as an ordinary requirement")
			}
			if _, err := NewRenderInput(root, collected, nil, nil); err == nil {
				t.Fatal("publication accepted an incomplete parsed population")
			}
			report, code := Check(root, nil)
			if code != 2 || !strings.Contains(report.Text(), "malformed {partial}") {
				t.Fatalf("gate did not refuse malformed scope: %s", report.Text())
			}
			from := stampPending(t, root, selftestStem, map[string]any{selftestRIDSend: map[string]any{"verdict": VerdictEnforced, "note": "not a whole proof"}})
			if _, err := auditStamp(root, selftestStem, from, stampModeNew, stampNow); err == nil {
				t.Fatal("malformed scope acquired a whole audit")
			}
			if len(readStampAudit(t, root)) != 0 {
				t.Fatal("refused stamp wrote audit data")
			}
		})
	}
}
