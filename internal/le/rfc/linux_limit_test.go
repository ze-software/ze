// Design: docs/contributing/rfc-conformance-gates.md -- the linux-limit annotation

package rfc

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const linuxLimitFixtureID = "LL-XFRM-EXAMPLE"
const linuxLimitFixtureReason = "no Ze code substitutes for the missing primitive"
const linuxLimitFixtureMarker = "{linux-limit: " + linuxLimitFixtureID + "; " + linuxLimitFixtureReason + "}"
const linuxLimitFixtureLine = "- [ ] [RFC9999-2-1] [MUST] A speaker MUST send the widget (§2) " + linuxLimitFixtureMarker
const linuxLimitFixtureRegister = "## Entries\n\n### " + linuxLimitFixtureID + "\n\n" +
	"- **Kind:** kernel\n- **Effect:** blocks\n- **Status:** verified\n" +
	"- **Checked:** Linux 7.2.9, 2026-10-20\n" +
	"- **Blocks:** RFC9999-2-1\n- **Evidence:** net/example.c, send_widget rejects the primitive\n" +
	"- **Re-check:** inspect send_widget at the shipped kernel version\n"

// TestALinuxLimitAnnotationCarriesItsRegisterID checks the canonical summary parser and vocabulary.
func TestALinuxLimitAnnotationCarriesItsRegisterID(t *testing.T) {
	req, err := oneLine(t, linuxLimitFixtureLine)
	if err != nil {
		t.Fatal(err)
	}
	if req.Annotation == nil || req.Annotation.Kind != AnnotationLinuxLimit || req.Annotation.LimitID != linuxLimitFixtureID || req.Annotation.Reason != linuxLimitFixtureReason {
		t.Fatalf("annotation = %+v", req.Annotation)
	}
	if !slices.Contains(AnnotationKinds(), AnnotationLinuxLimit) {
		t.Fatal("linux-limit absent from declared vocabulary")
	}
}

// TestALinuxLimitReasonIsRefusedWithoutBothFacts pins the row and shared format on every refusal.
func TestALinuxLimitReasonIsRefusedWithoutBothFacts(t *testing.T) {
	for _, body := range []string{"linux-limit", "linux-limit:", "linux-limit: reason", "linux-limit: ; reason", "linux-limit: LL-; reason", "linux-limit: LL-lowercase; reason", "linux-limit: LL-XFRM-EXAMPLE;", "linux-limit: LL-XFRM-EXAMPLE"} {
		t.Run(body, func(t *testing.T) {
			_, err := oneLine(t, strings.Replace(linuxLimitFixtureLine, linuxLimitFixtureMarker, "{"+body+"}", 1))
			if err == nil {
				t.Fatal("invalid annotation accepted")
			}
			for _, want := range []string{"rfc/short/rfc9999.md:7", linuxLimitFormat} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("refusal %q lacks %q", err, want)
				}
			}
		})
	}
	if _, err := oneLine(t, strings.Replace(linuxLimitFixtureLine, "linux-limit:", "unknown-limit:", 1)); err == nil {
		t.Fatal("unknown annotation kind accepted")
	}
}

// TestALinuxLimitAnnotationCannotDisplaceAGap checks both disposition orders.
func TestALinuxLimitAnnotationCannotDisplaceAGap(t *testing.T) {
	for _, line := range []string{linuxLimitFixtureLine + " {gap: work remains}", strings.Replace(linuxLimitFixtureLine, linuxLimitFixtureMarker, "{gap: work remains} "+linuxLimitFixtureMarker, 1)} {
		if _, err := oneLine(t, line); err == nil || !strings.Contains(err.Error(), "annotation") {
			t.Fatalf("mixed dispositions answered %v", err)
		}
	}
}

// TestCheckLinuxLimitReadsTheRegisterAndTheRow drives Check, not a count-only helper.
func TestCheckLinuxLimitReadsTheRegisterAndTheRow(t *testing.T) {
	for _, one := range []struct{ name, old, replacement, want string }{
		{name: "verified"},
		{"missing entry", "### LL-XFRM-EXAMPLE", "### LL-OTHER", "has no entry"},
		{"design constraint", "**Effect:** blocks", "**Effect:** constrains design", "another shape"},
		{"unverified", "**Status:** verified", "**Status:** unverified", "inspect send_widget"},
		{"library", "**Kind:** kernel", "**Kind:** library", "carried diff"},
		{"wrong requirement", "**Blocks:** RFC9999-2-1", "**Blocks:** RFC9999-2-2", "Blocks"},
		{"prefix is not identity", "**Blocks:** RFC9999-2-1", "**Blocks:** RFC9999-2-10", "Blocks"},
		{"suffix is not identity", "**Blocks:** RFC9999-2-1", "**Blocks:** XRFC9999-2-1", "Blocks"},
		{"embedded is not identity", "**Blocks:** RFC9999-2-1", "**Blocks:** RFC9999-2-1-extra", "Blocks"},
		{"empty evidence", "net/example.c, send_widget rejects the primitive", "", "Evidence"},
		{"stale kernel", "Linux 7.2.9", "Linux 7.2.8", "kernel.version"},
		{"malformed register", "**Status:** verified", "**Status:** invented", "Status"},
		{"duplicate entry", "## Entries", "## Entries\n\n### LL-XFRM-EXAMPLE\n", "more than one heading"},
		{"continuation", "**Blocks:** RFC9999-2-1", "**Blocks:** missing primitive\n  RFC9999-2-1", ""},
	} {
		t.Run(one.name, func(t *testing.T) {
			page := linuxLimitFixtureRegister
			if one.old != "" {
				page = strings.Replace(page, one.old, one.replacement, 1)
			}
			if one.name == "library" {
				page = strings.Replace(page, "Linux 7.2.9", "module example.org/library v1.0.0", 1)
			}
			root := linuxLimitCheckFixture(t, page, linuxLimitFixtureLine)
			report, code := Check(root, nil)
			if report.CannotRun != "" {
				t.Fatal(report.CannotRun)
			}
			var found []Finding
			for _, finding := range report.Findings {
				if finding.Check == CheckAnnotations {
					found = append(found, finding)
				}
			}
			if one.want == "" {
				if len(found) != 0 || code != 0 {
					t.Fatalf("valid entry refused, exit %d: %s", code, report.Text())
				}
				if len(report.LinuxLimitCandidates) != 0 {
					t.Fatal("an existing linux-limit annotation was proposed again")
				}
				return
			}
			if len(found) != 1 || code == 0 {
				t.Fatalf("want one annotation refusal, exit %d: %s", code, report.Text())
			}
			for _, want := range []string{one.want, linuxLimitFixtureID, linuxLimitRegisterRel, selftestRIDSend, selftestSummaryRel} {
				if !strings.Contains(found[0].Message, want) {
					t.Errorf("refusal %q lacks %q", found[0].Message, want)
				}
			}
			if found[0].RID != selftestRIDSend || found[0].Where == "" {
				t.Fatalf("refusal lost source parts: %+v", found[0])
			}
		})
	}
}

func linuxLimitCheckFixture(t *testing.T, page, line string) string {
	t.Helper()
	return checkFixtureTree(t, map[string]string{
		selftestSummaryRel:    "# RFC 9999\n\n" + selftestMeta + "\n## Compliance Checklist\n\n" + line + "\n",
		linuxLimitRegisterRel: page,
		linuxLimitKernelRel:   "7.2.9\n",
	})
}

// TestLinuxLimitMissingInputsFailClosed keeps legacy corpora independent of the register.
func TestLinuxLimitMissingInputsFailClosed(t *testing.T) {
	for _, missing := range []string{linuxLimitRegisterRel, linuxLimitKernelRel} {
		t.Run(missing, func(t *testing.T) {
			root := linuxLimitCheckFixture(t, linuxLimitFixtureRegister, linuxLimitFixtureLine)
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(missing))); err != nil {
				t.Fatal(err)
			}
			report, code := Check(root, nil)
			if report.CannotRun != "" || code == 0 || !strings.Contains(strings.Join(report.Violations, "\n"), missing) {
				t.Fatalf("missing input passed: %s", report.Text())
			}
			writeFixtureFiles(t, root, map[string]string{selftestSummaryRel: "# RFC 9999\n\n" + selftestMeta + "\n## Compliance Checklist\n\n" + strings.Replace(linuxLimitFixtureLine, linuxLimitFixtureMarker, "{gap: work remains}", 1) + "\n"})
			report, _ = Check(root, nil)
			if report.CannotRun != "" || strings.Contains(strings.Join(report.Violations, "\n"), missing) {
				t.Fatalf("old corpus acquired register dependency: %s", report.Text())
			}
		})
	}
}

// TestALinuxLimitAnnotationIsStaleBesideATaggedTest verifies every proof polarity refuses.
func TestALinuxLimitAnnotationIsStaleBesideATaggedTest(t *testing.T) {
	req, err := oneLine(t, linuxLimitFixtureLine)
	if err != nil {
		t.Fatal(err)
	}
	for _, polarity := range Polarities() {
		findings := evaluate([]Requirement{*req}, []Tag{{RID: req.RID, Polarity: polarity, File: "a_test.go", Line: 3}}, map[string]bool{req.RFC: true})
		if len(findings) != 1 || !strings.Contains(findings[0].Message, "annotation is stale") {
			t.Fatalf("%s: %+v", polarity, findings)
		}
	}
	if got := rollupTargetState(*req, map[string]bool{PolarityPositive: true, PolarityNegative: true}); got != RollupGap {
		t.Fatalf("rollup target = %v, want gap", got)
	}
}

// TestALinuxLimitRowCountsAsUnmetInTheStatusRow checks disclosure and exact Remaining counts.
func TestALinuxLimitRowCountsAsUnmetInTheStatusRow(t *testing.T) {
	req, err := oneLine(t, linuxLimitFixtureLine)
	if err != nil {
		t.Fatal(err)
	}
	if got := checkStatusAgreement([]Requirement{*req}, map[string]LedgerRow{req.RFC: {}}, map[string]bool{req.RFC: true}); len(got) != 0 {
		t.Fatalf("a public row derives its Unmet cell, yet refused: %v", got)
	}
	if got := checkStatusAgreement([]Requirement{*req}, map[string]LedgerRow{}, map[string]bool{req.RFC: true}); len(got) != 1 {
		t.Fatalf("an enrolled RFC hiding its {linux-limit} row behind no public row was accepted: %v", got)
	}
}

// TestSupportedDisclosurePreservesOtherUnmetAnnotations ensures the Linux-limit
// support prohibition does not change the existing gap or partial disclosure rule.
func TestSupportedDisclosurePreservesOtherUnmetAnnotations(t *testing.T) {
	for _, kind := range []string{AnnotationGap, AnnotationPartial} {
		req := Requirement{RFC: selftestStem, RID: selftestRIDSend,
			Annotation: &Annotation{Kind: kind, Reason: "work remains"}}
		rows := map[string]LedgerRow{selftestStem: {}}
		if errs := checkStatusAgreement([]Requirement{req}, rows, map[string]bool{selftestStem: true}); len(errs) != 0 {
			t.Fatalf("%s lost its existing disclosure behavior: %v", kind, errs)
		}
	}
}

// TestCheckLinuxLimitAndGapRowsRaiseNoFinding proves a disclosed unmet row,
// {linux-limit} or {gap}, raises no finding of its own once the fixture's other
// row is proven and its extraction signed off. The Supported-row refusal this
// test once pinned left with the authored Support status cell: a support claim
// now lives in a feature's Level, and `./le feature check` S3 judges it from
// the derived proof (internal/le/feature TestCheckAcceptsSupportedWithLinuxLimitOnly).
func TestCheckLinuxLimitAndGapRowsRaiseNoFinding(t *testing.T) {
	const secondRID = "RFC9999-2-2"
	const secondQuote = "A receiver MUST preserve the widget."
	const secondLine = "- [ ] [" + secondRID + "] [MUST] " + secondQuote + " (§2)"
	for _, one := range []struct {
		name, marker string
	}{
		{"linux-limit", linuxLimitFixtureMarker},
		{"ordinary gap", "{gap: work remains}"},
	} {
		t.Run(one.name, func(t *testing.T) {
			meta := selftestMeta
			line := strings.Replace(linuxLimitFixtureLine, linuxLimitFixtureMarker, one.marker, 1)
			root := checkFixtureTree(t, map[string]string{
				selftestSummaryRel:     "# RFC 9999\n\n" + meta + "\n## Compliance Checklist\n\n" + line + "\n" + secondLine + "\n",
				"rfc/full/rfc9999.txt": checkFixtureSource + "\n    " + secondQuote + "\n",
				"test/plugin/widget.ci": "# RFC requirement: " + secondRID + " positive\n" +
					"# RFC requirement: " + secondRID + " negative\n",
				linuxLimitRegisterRel: linuxLimitFixtureRegister,
				linuxLimitKernelRel:   "7.2.9\n",
			})
			report, code := Check(root, nil)
			if report.CannotRun != "" {
				t.Fatal(report.CannotRun)
			}
			if report.Tags != 2 || report.Gated != 2 || report.Signed != 1 {
				t.Fatalf("fixture did not isolate the unmet row: tags=%d gated=%d signed=%d\n%s",
					report.Tags, report.Gated, report.Signed, report.Text())
			}
			if code != 0 || len(report.Findings) != 0 {
				t.Fatalf("a disclosed unmet row was refused, exit %d: %s", code, report.Text())
			}
		})
	}
}

// TestUnimplementedVerdictStandsOverLinuxLimit keeps an unmet binding obligation out of not-applicable.
func TestUnimplementedVerdictStandsOverLinuxLimit(t *testing.T) {
	req, err := oneLine(t, linuxLimitFixtureLine)
	if err != nil {
		t.Fatal(err)
	}
	verdict := map[string]any{"verdict": VerdictUnimplemented, "note": "the code cannot comply", "code": map[string]any{"a.go::send": "fingerprint"}}
	if errs := verdictClaims(req.RFC, req.RID, verdict, *req, nil); len(errs) != 0 {
		t.Fatal(errs)
	}
	verdict["verdict"] = VerdictNotApplicable
	verdict["no_code_path"] = "no primitive exists"
	if errs := verdictClaims(req.RFC, req.RID, verdict, *req, nil); len(errs) != 1 || !strings.Contains(errs[0], "carries {linux-limit}") {
		t.Fatalf("not-applicable = %v", errs)
	}
}

// TestLinuxLimitCandidatesAreReportedNotApplied checks structured and prose evidence without editing rows.
func TestLinuxLimitCandidatesAreReportedNotApplied(t *testing.T) {
	line := strings.Replace(linuxLimitFixtureLine, linuxLimitFixtureMarker, "{gap: work remains}", 1)
	root := linuxLimitCheckFixture(t, linuxLimitFixtureRegister, line)
	before, err := os.ReadFile(filepath.Join(root, selftestSummaryRel))
	if err != nil {
		t.Fatal(err)
	}
	report, code := Check(root, nil)
	if code != 0 || len(report.LinuxLimitCandidates) != 1 {
		t.Fatalf("candidates = %+v, exit %d: %s", report.LinuxLimitCandidates, code, report.Text())
	}
	candidate := report.LinuxLimitCandidates[0]
	if candidate.RID != selftestRIDSend || candidate.LimitID != linuxLimitFixtureID || candidate.Where == "" {
		t.Fatalf("candidate lost its sources: %+v", candidate)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), "linux-limit-candidates") || !strings.Contains(report.Text(), "owner approval") {
		t.Fatalf("candidate missing from report: %s", report.Text())
	}
	if !strings.HasPrefix(report.Text(), "rfc-requirements OK:") {
		t.Fatalf("candidate displaced the report verdict: %s", report.Text())
	}
	after, err := os.ReadFile(filepath.Join(root, selftestSummaryRel))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatal("candidate report annotated the summary")
	}
	writeFixtureFiles(t, root, map[string]string{linuxLimitRegisterRel: strings.Replace(linuxLimitFixtureRegister, "**Status:** verified", "**Status:** unverified", 1)})
	report, _ = Check(root, nil)
	if len(report.LinuxLimitCandidates) != 0 {
		t.Fatal("unverified entry offered as candidate")
	}
	writeFixtureFiles(t, root, map[string]string{linuxLimitRegisterRel: strings.Replace(linuxLimitFixtureRegister, "**Status:** verified", "**Status:** invented", 1)})
	report, code = Check(root, nil)
	if code != 0 || report.CannotRun != "" || len(report.LinuxLimitCandidates) != 0 {
		t.Fatalf("malformed optional register changed the old corpus verdict: %s", report.Text())
	}
}

// TestALinuxLimitRequirementIsAnnotatedAndNotProven holds the share fixed when a gap changes classification.
func TestALinuxLimitRequirementIsAnnotatedAndNotProven(t *testing.T) {
	req, err := oneLine(t, linuxLimitFixtureLine)
	if err != nil {
		t.Fatal(err)
	}
	metas := map[string]Meta{req.RFC: {Enrolment: enrolmentEnrolled, Implementation: implementationZe, Aggregate: "bgp-core", Row: true}}
	before := *req
	before.Annotation = &Annotation{Kind: AnnotationGap, Reason: "work remains"}
	gap, err := shareOver(t, metas, []Requirement{before}, map[string]bool{req.RFC: true}, map[string]bool{req.RID: true})
	if err != nil {
		t.Fatal(err)
	}
	limit, err := shareOver(t, metas, []Requirement{*req}, map[string]bool{req.RFC: true}, map[string]bool{req.RID: true})
	if err != nil {
		t.Fatal(err)
	}
	if gap != limit || limit.Proven != 0 || limit.Gated != 1 {
		t.Fatalf("classification moved share: gap=%+v, limit=%+v", gap, limit)
	}
	rows := CoverageRows([]Requirement{*req}, nil, nil)
	if len(rows) != 1 || rows[0].Annotated != 1 || rows[0].Gated != 1 || rows[0].Both != 0 {
		t.Fatalf("linux-limit escaped counted, annotated population: %+v", rows)
	}
}
