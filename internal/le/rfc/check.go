// Design: docs/architecture/core-design.md -- the rfc area, as one command
// Detail: check_baseline.go -- committed snapshots and HEAD carrier construction
// Detail: check_ratchets.go -- monotonic requirement and evidence checks
// Detail: check_status.go -- public ledger agreement checks
// Detail: check_audit.go -- audit schema, freshness, disclosure, and ratchets
// Detail: check_extraction.go -- extraction, drain, and generated-page checks
// Detail: check_compile.go -- tagged-package type checking
//
// check.go owns the control flow for `le rfc check`. Every leaf computes one
// concern and this function fixes the diagnostic order to the Python producer's.
package rfc

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// CheckReport is the structured result of one RFC check.
type CheckReport struct {
	CannotRun        string         `json:"cannot-run,omitempty"`
	Violations       []string       `json:"violations,omitempty"`
	Gated            int            `json:"gated,omitempty"`
	Enrolled         int            `json:"enrolled,omitempty"`
	Tags             int            `json:"tags,omitempty"`
	Evidence         map[string]int `json:"evidence,omitempty"`
	Signed           int            `json:"signed,omitempty"`
	SignedByRegister map[string]int `json:"signed-by-register,omitempty"`
	Unsigned         int            `json:"unsigned,omitempty"`
	AuditProven      int            `json:"audit-proven,omitempty"`
	AuditFindings    int            `json:"audit-findings,omitempty"`
	AuditVerdicts    int            `json:"audit-verdicts,omitempty"`
	AuditDone        int            `json:"audit-done,omitempty"`
	AuditTotal       int            `json:"audit-total,omitempty"`
}

// Text renders the diagnostics and success summary the Python gate prints.
func (r CheckReport) Text() string {
	var tb textbuf.Buffer
	if r.CannotRun != "" {
		return tb.Str("rfc-requirements: cannot run: ").Str(r.CannotRun).Byte('\n').String()
	}
	if len(r.Violations) > 0 {
		tb.Str("rfc-requirements: ").Int(int64(len(r.Violations))).Str(" violation(s)\n\n")
		for _, violation := range r.Violations {
			tb.Str("  * ").Str(violation).Byte('\n')
		}
		tb.Str("\nRules: every MUST-level requirement of an enrolled RFC needs\n").
			Str("a positive AND a negative test tagged `RFC requirement: <ID> <polarity>`,\n").
			Str("or an annotation saying why not. See ai/skills/ze-rfc.md.\n")
		return tb.String()
	}
	percentage := 0.0
	if r.AuditTotal > 0 {
		percentage = 100 * float64(r.AuditDone) / float64(r.AuditTotal)
	}
	tb.Str("rfc-requirements OK: ").Int(int64(r.Gated)).Str(" gated MUST-level requirement(s) across ").
		Int(int64(r.Enrolled)).Str(" enrolled RFC(s); ").Int(int64(r.Tags)).Str(" test tag(s) resolved.\n")
	tb.Str("evidence: ").Str(evidencePhrase(r.Evidence)).Str(" (unit evidence proves the algorithm; only a running non-unit test proves the daemon or a peer).\n")
	tb.Str("extraction: ").Str(registerPhrase(r.SignedByRegister)).Str(" signed off of ").Int(int64(r.Enrolled)).
		Str(" enrolled; ").Int(int64(r.Unsigned)).Str(" unsigned (grandfathered backlog).\n")
	tb.Str("audit: ").Int(int64(r.AuditProven)).Str(" proven, ").Int(int64(r.AuditFindings)).
		Str(" audited-but-not-proven, of ").Int(int64(r.AuditVerdicts)).Str(" verdict(s); ").
		Int(int64(r.AuditDone)).Str(" of ").Int(int64(r.AuditTotal)).Str(" auditable requirement(s) audited (").
		Str(strconv.FormatFloat(percentage, 'f', 2, 64)).Str("%); a missing verdict is legal (the audit is sampled, the gate is total).\n")
	return tb.String()
}

func evidenceCounts(tags []Tag, carriers []Carrier) map[string]int {
	out := map[string]int{}
	for _, carrier := range carriers {
		if carrier.Tier != tierUnrun {
			out[carrier.Label()] = 0
		}
	}
	for _, tag := range tags {
		out[evidenceLabel(tag.File, carriers)]++
	}
	return out
}

func evidencePhrase(counts map[string]int) string {
	ordered := []string{"unit/verify", "functional/verify", "editor/verify", "interop/nightly"}
	knownSet := map[string]bool{}
	var parts []string
	for _, label := range ordered {
		if _, held := counts[label]; !held {
			continue
		}
		knownSet[label] = true
		var tb textbuf.Buffer
		parts = append(parts, tb.Str(label).Byte(' ').Int(int64(counts[label])).String())
	}
	var extra []string
	for label := range counts {
		if !knownSet[label] {
			extra = append(extra, label)
		}
	}
	sort.Strings(extra)
	for _, label := range extra {
		var tb textbuf.Buffer
		parts = append(parts, tb.Str(label).Byte(' ').Int(int64(counts[label])).String())
	}
	return strings.Join(parts, ", ")
}

// Check runs the complete read-only check over tree and returns its exit code.
func Check(tree string) (CheckReport, int) {
	report, err := check(tree, time.Now())
	if err != nil {
		return CheckReport{CannotRun: err.Error()}, 2
	}
	if len(report.Violations) > 0 {
		return report, 2
	}
	return report, 0
}

func check(tree string, today time.Time) (CheckReport, error) {
	collected, err := Collect(tree)
	if err != nil {
		return CheckReport{}, err
	}
	stems, err := summaryStems(tree)
	if err != nil {
		return CheckReport{}, err
	}
	baselineEnrolled, enrolledKnown := baselineEnrolled(tree)
	baseEnrolled := baselineEnrolled
	if !enrolledKnown {
		baseEnrolled = map[string]bool{}
	}
	levels := baselineLevels(tree)
	ids := baselineIDs(levels)
	baselineStems, stemsKnown := baselineSummaryStems(tree)
	if !stemsKnown {
		baselineStems = map[string]bool{}
	}
	rows, err := loadStatusLedger(tree)
	if err != nil {
		return CheckReport{}, err
	}
	baselineRows, rowsKnown := baselineStatusRows(tree)
	dispositions, err := loadDispositions(tree)
	if err != nil {
		return CheckReport{}, err
	}
	baselineDispositionSet := baselineDispositions(tree)
	deriver := NewDeriver(tree)
	signed, extractionErrors, err := evaluateExtractions(deriver, collected.Requirements)
	if err != nil {
		return CheckReport{}, err
	}
	signedSet := map[string]bool{}
	for stem := range signed {
		signedSet[stem] = true
	}
	newly := map[string]bool{}
	if enrolledKnown {
		for stem := range collected.Enrolled {
			if !baselineEnrolled[stem] {
				newly[stem] = true
			}
		}
	}

	var violations []string
	violations = append(violations, checkEnrolment(tree, collected.Enrolled, baseEnrolled, stems, newly, signedSet)...)
	violations = append(violations, checkNewSummaries(deriver, stems, baselineStems, collected.Enrolled,
		collected.Requirements, collected.ParseByStem, stemsKnown)...)
	if intersects(collected.Enrolled, baseEnrolled) {
		baselineTagSet := baselineTags(tree)
		carriers, err := carriers(tree)
		if err != nil {
			return CheckReport{}, err
		}
		violations = append(violations, checkRetiredRequirements(collected.Requirements, collected.Enrolled,
			ids, baseEnrolled, stems, baselineStems, collected.ParseByStem)...)
		violations = append(violations, checkLevelRatchet(tree, collected.Requirements, collected.Enrolled,
			levels, baseEnrolled)...)
		violations = append(violations, checkCoverageRatchet(collected.Requirements, collected.Tags,
			collected.Enrolled, baselinePolarities(baselineTagSet), baseEnrolled)...)
		violations = append(violations, checkEvidenceRatchet(collected.Requirements, collected.Tags,
			collected.Enrolled, carriers, baselineEvidence(tree, baselineTagSet), baseEnrolled)...)
	}
	violations = append(violations, collected.ParseErrors...)
	violations = append(violations, checkIDAllocation(collected.Requirements, ids)...)
	violations = append(violations, evaluate(collected.Requirements, collected.Tags, collected.Enrolled)...)
	successors, err := summarySuccessors(tree, stems)
	if err != nil {
		return CheckReport{}, err
	}
	violations = append(violations, checkSuperseded(tree, collected.Requirements, successors, stems)...)
	carriers, err := carriers(tree)
	if err != nil {
		return CheckReport{}, err
	}
	compileErrors, err := checkTagPackagesCompile(tree, collected.Tags, carriers)
	if err != nil {
		return CheckReport{}, err
	}
	violations = append(violations, compileErrors...)
	violations = append(violations, checkStatusAgreement(collected.Requirements, rows, collected.Enrolled)...)
	violations = append(violations, checkSummaryDisposition(stems, collected.Enrolled, dispositions, baselineDispositionSet)...)
	violations = append(violations, checkStatusCompleteness(collected.Enrolled, rows, baselineRows, rowsKnown,
		newly, baseEnrolled)...)
	derived, err := derivedRegisters(deriver, signed, collected.Requirements)
	if err != nil {
		return CheckReport{}, err
	}
	violations = append(violations, checkUnprovenSupport(collected.Requirements, rows, stems, dispositions,
		signed, derived)...)
	violations = append(violations, checkGapCountAgreement(collected.Requirements, rows)...)

	audits, err := loadAudits(tree, collected.Enrolled)
	if err != nil {
		return CheckReport{}, err
	}
	baselineAuditSet, auditsKnown := baselineAudits(tree)
	auditFileErrors, err := checkAuditFiles(tree, collected.Enrolled, stems)
	if err != nil {
		return CheckReport{}, err
	}
	violations = append(violations, auditFileErrors...)
	violations = append(violations, checkAuditSchema(collected.Requirements, collected.Tags, audits)...)
	states := AuditFreshness(AuditFreshnessInput{Tree: tree, Requirements: collected.Requirements,
		Tags: collected.Tags, Enrolled: collected.Enrolled, Audits: audits})
	violations = append(violations, checkAuditFreshness(collected.Requirements, states)...)
	violations = append(violations, checkAuditDisclosure(collected.Requirements, rows, collected.Enrolled, audits)...)
	violations = append(violations, checkAuditNote(tree, collected.Requirements, collected.Tags, collected.Enrolled, audits)...)
	violations = append(violations, checkAuditFindings(collected.Requirements, collected.Enrolled, audits,
		baselineAuditSet, auditsKnown)...)
	violations = append(violations, checkAuditVerdictRatchet(collected.Requirements, collected.Enrolled, audits,
		baselineAuditSet, auditsKnown, baseEnrolled)...)

	violations = append(violations, extractionErrors...)
	extractions, err := LoadExtractions(tree)
	if err != nil {
		return CheckReport{}, err
	}
	violations = append(violations, checkExtractionRatchet(tree, extractions)...)
	violations = append(violations, checkDrainFloor(tree, collected.Enrolled, signed, today)...)
	ledgerErrors, err := checkLedgerFresh(tree, collected, rows, dispositions)
	if err != nil {
		return CheckReport{}, err
	}
	violations = append(violations, ledgerErrors...)

	report := CheckReport{Violations: violations}
	if len(violations) > 0 {
		return report, nil
	}
	for _, req := range collected.Requirements {
		if req.Gated() && collected.Enrolled[req.RFC] {
			report.Gated++
		}
	}
	credited := credited(signed, collected.Enrolled)
	report.Enrolled = len(collected.Enrolled)
	report.Tags = len(collected.Tags)
	report.Evidence = evidenceCounts(collected.Tags, carriers)
	report.Signed = len(credited)
	report.SignedByRegister = registerCounts(credited)
	report.Unsigned = len(collected.Enrolled) - len(credited)
	auditRows, worklist := auditCoverageRows(auditCoverageInput{Requirements: collected.Requirements,
		Tags: collected.Tags, Enrolled: collected.Enrolled, Carriers: carriers, Audits: audits, States: states})
	for _, row := range auditRows {
		report.AuditProven += row.Proven
		report.AuditVerdicts += row.Verdicts
		report.AuditDone += row.Audited
		report.AuditTotal += row.Auditable
	}
	report.AuditFindings = len(worklist)
	return report, nil
}

func intersects(left, right map[string]bool) bool {
	for key := range left {
		if right[key] {
			return true
		}
	}
	return false
}
