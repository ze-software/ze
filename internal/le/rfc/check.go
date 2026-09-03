// Design: docs/architecture/core-design.md -- the rfc area, as one command
// Detail: check_baseline.go -- committed snapshots and HEAD carrier construction
// Detail: check_ratchets.go -- monotonic requirement and evidence checks
// Detail: check_status.go -- public ledger agreement checks
// Detail: check_audit.go -- audit schema, freshness, disclosure, and ratchets
// Detail: check_extraction.go -- extraction, drain, and generated-page checks
// Detail: check_compile.go -- tagged-package type checking
// Detail: discriminate.go -- the recorded proofs this check reads and counts
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

// Finding is one thing the gate refuses, in the PARTS the check had before it
// formatted them.
//
// Message is what the check authored and what `./le rfc check` prints, so it is
// carried rather than rebuilt: a formatter here would be a second author of a
// sentence twenty checks already write, and it would drift. The parts beside it
// are the check's own inputs, never a parse of Message. A consumer that wants
// columns -- the published gate page does -- reads the parts; one that wants
// the line reads Message (ai/rules/principles.md).
//
// A check with no requirement in hand fills Message alone. That is a finding
// about a file, a ratchet or a ledger row, and it states itself.
type Finding struct {
	Message string `json:"message"`
	// Where is the summary file and line this was raised at.
	Where string `json:"where,omitempty"`
	// RID, Level, Section and Text are the requirement's own.
	RID     string `json:"rid,omitempty"`
	Level   string `json:"level,omitempty"`
	Section string `json:"section,omitempty"`
	Text    string `json:"text,omitempty"`
	// Issue is what is wrong, without the requirement's own text after it.
	Issue string `json:"issue,omitempty"`
}

// note answers a finding that carries a message and no parts.
func note(message string) Finding { return Finding{Message: message} }

// notes wraps a check that answers messages rather than findings, so one list
// carries every violation and Violations is rendered from that one list.
func notes(messages []string) []Finding {
	out := make([]Finding, 0, len(messages))
	for _, message := range messages {
		out = append(out, note(message))
	}
	return out
}

// findingMessages renders the lines the gate prints.
func findingMessages(findings []Finding) []string {
	if len(findings) == 0 {
		return nil
	}
	out := make([]string, 0, len(findings))
	for _, finding := range findings {
		out = append(out, finding.Message)
	}
	return out
}

// requirementFinding answers a finding that carries one requirement's parts.
func requirementFinding(req Requirement, issue, message string) Finding {
	return Finding{Message: message, Where: requirementWhere(req), RID: req.RID,
		Level: req.Level, Section: req.Section, Text: req.Text, Issue: issue}
}

// CheckReport is the structured result of one RFC check.
type CheckReport struct {
	CannotRun string `json:"cannot-run,omitempty"`
	// Findings is every violation, in parts. Violations is rendered from it, so
	// the two cannot hold different populations.
	Findings         []Finding      `json:"findings,omitempty"`
	Violations       []string       `json:"violations,omitempty"`
	Gated            int            `json:"gated,omitempty"`
	Enrolled         int            `json:"enrolled,omitempty"`
	Tags             int            `json:"tags,omitempty"`
	Evidence         map[string]int `json:"evidence,omitempty"`
	Signed           int            `json:"signed,omitempty"`
	SignedByRegister map[string]int `json:"signed-by-register,omitempty"`
	Unsigned         int            `json:"unsigned,omitempty"`
	SignedUnenrolled []string       `json:"signed-unenrolled,omitempty"`
	AuditProven      int            `json:"audit-proven,omitempty"`
	AuditFindings    int            `json:"audit-findings,omitempty"`
	AuditVerdicts    int            `json:"audit-verdicts,omitempty"`
	AuditDone        int            `json:"audit-done,omitempty"`
	AuditTotal       int            `json:"audit-total,omitempty"`
	// The three discrimination figures render even at zero, unlike every count
	// above them. They are published DEBT, and an absent key would let a
	// consumer read "nothing is proven yet" as "this gate has no such stage".
	DiscriminationProven  int `json:"discrimination-proven"`
	DiscriminationOwed    int `json:"discrimination-owed"`
	DiscriminationEscaped int `json:"discrimination-escaped"`
	// DiscriminationRemovable names the records whose TAG is gone. They are
	// reported rather than refused: a record dies with the tag it proves, so an
	// orphan has nothing left to be wrong about and the next action is to delete
	// it, not to re-record it.
	DiscriminationRemovable []string `json:"discrimination-removable,omitempty"`
	// DiscriminationDrifted names the records the working tree contradicts under
	// an edit nobody has committed. Reported rather than refused: the drift
	// belongs to whoever is editing that file, and it becomes their violation at
	// their commit (owner decision, 2026-08-31). None of them is counted as
	// proven, because an unverified verdict is counted by nothing.
	DiscriminationDrifted []string `json:"discrimination-drifted,omitempty"`
	// DiscriminationChanged counts the grandfathered tagged units whose behavior
	// changed since HEAD with no proof recorded. A MEASUREMENT of the unproven
	// backlog, never a violation: it renders at zero for the reason the three
	// figures above do, so an absent key cannot read as "this gate has no such
	// stage" (owner decision, 2026-08-31).
	DiscriminationChanged int `json:"discrimination-changed"`
	// DiscriminationBacklog counts the tagged units the unpushed commits added
	// without proving. A MEASUREMENT of how much sits behind the obligation,
	// never a violation: the unpushed set is hundreds of commits deep here,
	// nobody can clear it inside the change in hand, and billing it is what gets
	// a ratchet removed rather than obeyed.
	//
	// nil says the branch it is measured against did not resolve, and renders as
	// JSON null. A plain int cannot carry that: a branch nobody could read and a
	// branch nothing is ahead of are opposite answers that both count 0, and a
	// reader of the second kind of 0 would take a measurement that never ran for
	// a clean backlog (ai/rules/principles.md). The branch is named by
	// backlogRevision, which is where that name is declared.
	DiscriminationBacklog *int `json:"discrimination-backlog"`
	// DiscriminationUnresolved counts the tagged units that measurement could
	// not read at BOTH revisions. It renders beside the count above because a
	// zero the walk never looked for is not a zero: almost every tag keys on a
	// function, and a resolver answering nothing would publish an empty backlog
	// over a corpus it never opened (ai/rules/principles.md).
	DiscriminationUnresolved int `json:"discrimination-unresolved"`
	// UnscannedTags are the `RFC requirement:` comments in production Go that
	// no carrier claims. Published debt, not a violation: they predate the
	// check, and a ratchet that reds the tree over standing debt gets removed
	// rather than obeyed.
	UnscannedTags []UnscannedTag `json:"unscanned-tags,omitempty"`
}

// Text renders the diagnostics and success summary the Python gate prints.
func (r *CheckReport) Text() string {
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
	// The line above counts the ENROLLED set, so a completed walk for a stem
	// nobody has enrolled yet lands in no figure on it. Reading the file count
	// beside the total is then the only way to notice, and nothing said so.
	if len(r.SignedUnenrolled) > 0 {
		tb.Str("extraction: ").Int(int64(len(r.SignedUnenrolled))).
			Str(" further sign-off(s) are valid but uncounted above, because the stem is not enrolled (").
			Str(strings.Join(r.SignedUnenrolled, ", ")).
			Str("). The walk is done and starts counting the day its summary declares `| Enrolment | enrolled |`.\n")
	}
	tb.Str("audit: ").Int(int64(r.AuditProven)).Str(" proven, ").Int(int64(r.AuditFindings)).
		Str(" audited-but-not-proven, of ").Int(int64(r.AuditVerdicts)).Str(" verdict(s); ").
		Int(int64(r.AuditDone)).Str(" of ").Int(int64(r.AuditTotal)).Str(" auditable requirement(s) audited (").
		Str(strconv.FormatFloat(percentage, 'f', 2, 64)).Str("%); a missing verdict is legal (the audit is sampled, the gate is total).\n")
	tb.Str("discrimination: ").Int(int64(r.DiscriminationProven)).Str(" proven, ").
		Int(int64(r.DiscriminationOwed)).Str(" owed, ").Int(int64(r.DiscriminationEscaped)).
		Str(" escaped; a proof is a recorded break under which the tagged unit itself goes red, ").
		Str("and a tag the commit under test did not add is grandfathered.\n")
	if r.DiscriminationBacklog != nil {
		tb.Str("discrimination: ").Int(int64(*r.DiscriminationBacklog)).
			Str(" tagged unit(s) carry a tag added since ").Str(backlogRevision).
			Str(" with no proof recorded. A MEASUREMENT of the unpushed backlog, not an ").
			Str("obligation being enforced: nothing on this line is a violation and the exit ").
			Str("code is unchanged. The owed count above bills only what the commit under test ").
			Str("added, which is the one change its author can still record a proof in.\n")
	}
	if len(r.DiscriminationRemovable) > 0 {
		tb.Str("discrimination: ").Int(int64(len(r.DiscriminationRemovable))).
			Str(" record(s) can be removed, because the tag each one proved is gone (").
			Str(strings.Join(r.DiscriminationRemovable, ", ")).
			Str("). A record dies with its tag, so an orphan is deleted rather than re-recorded.\n")
	}
	if len(r.DiscriminationDrifted) > 0 {
		tb.Str("discrimination: ").Int(int64(len(r.DiscriminationDrifted))).
			Str(" record(s) do not match the working tree under an edit nobody has committed (").
			Str(strings.Join(r.DiscriminationDrifted, ", ")).
			Str("). None counts as proven while that edit stands, and none is a violation here: ").
			Str("the drift belongs to whoever edited that file, and it becomes their violation ").
			Str("at their commit.\n")
	}
	tb.Str("discrimination: ").Int(int64(r.DiscriminationChanged)).
		Str(" grandfathered tagged unit(s) changed behavior since HEAD with no proof recorded. ").
		Str("A MEASUREMENT of the unproven backlog, not an obligation: nothing on this line is a ").
		Str("violation and the exit code is unchanged. Whether it becomes one is a later decision.\n")
	if r.DiscriminationUnresolved > 0 {
		tb.Str("discrimination: ").Int(int64(r.DiscriminationUnresolved)).
			Str(" further tagged unit(s) could not be read at both HEAD and here, so the count ").
			Str("above them counted neither. A unit key names a function, and one that resolves ").
			Str("at neither revision is a measurement that did not run rather than a clean one.\n")
	}
	if len(r.UnscannedTags) > 0 {
		tb.Str("unscanned: ").Int(int64(len(r.UnscannedTags))).
			Str(" 'RFC requirement:' comment(s) sit in production Go on no carrier, so nothing resolves them and nothing runs them; ").
			Int(int64(unscannedRefused(r.UnscannedTags))).
			Str(" of those would be refused outright, having no polarity. They read as evidence to a person and are counted by no gate.\n")
	}
	return tb.String()
}

// unscannedRefused counts the production tags no scanner could read even if one
// claimed their file.
func unscannedRefused(tags []UnscannedTag) int {
	refused := 0
	for index := range tags {
		if tags[index].Refusal != "" {
			refused++
		}
	}
	return refused
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
	baseMetas, enrolledKnown := baselineMetas(tree)
	baselineEnrolled := enrolledFrom(baseMetas)
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
	rows := rowsFrom(collected.Metas)
	dispositions := dispositionsFrom(collected.Metas)
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

	records, err := loadDiscrimination(tree)
	if err != nil {
		return CheckReport{}, err
	}
	// One reader and one scope index for every discrimination consumer below.
	// Each resolves a unit key inside a file's text, and a second cache would
	// re-read the same files for the same answers.
	discriminationSources := newSourceReader(tree)
	discriminationIndex := newScopeIndex()
	// The committed tag corpus is read once, at the three revisions this check
	// compares, and serves four consumers: the polarity and evidence ratchets
	// below, which only run where the enrolled sets meet; the obligation, which
	// bills what the tip commit added against the commit before it; the backlog,
	// which measures what the unpushed commits added against the pushed branch;
	// and the changed-unit measurement, which reads the tip's own blobs.
	committed := readCommittedTags(tree, discriminationIndex)
	// Every tag is resolved to the unit it sits in ONCE. Three consumers read
	// that answer: the claim fingerprint each record is verified against, the
	// obligation a unit new since HEAD owes, and the orphan exemption that tells
	// a record whose tag is gone from one the tree contradicts.
	discriminationCovers, err := tagCovers(discriminationSources, discriminationIndex, collected.Tags)
	if err != nil {
		return CheckReport{}, err
	}
	// The stored proofs are re-verified ONCE, and the verdicts serve both the
	// ratchet and the published figures. A record that no longer verifies is
	// therefore refused and uncounted by one decision rather than two.
	discrimination, err := verifyDiscrimination(tree, records, discriminationCovers)
	if err != nil {
		return CheckReport{}, err
	}

	var findings []Finding
	findings = append(findings, notes(checkEnrolment(tree, collected.Enrolled, baseEnrolled, stems, newly, signedSet))...)
	findings = append(findings, notes(checkNewSummaries(deriver, stems, baselineStems, collected.Enrolled,
		collected.Requirements, collected.ParseByStem, stemsKnown, collected.Metas))...)
	if intersects(collected.Enrolled, baseEnrolled) {
		carriers, err := carriers(tree)
		if err != nil {
			return CheckReport{}, err
		}
		findings = append(findings, notes(checkRetiredRequirements(collected.Requirements, collected.Enrolled,
			ids, baseEnrolled, stems, baselineStems, collected.ParseByStem))...)
		findings = append(findings, notes(checkLevelRatchet(tree, collected.Requirements, collected.Enrolled,
			levels, baseEnrolled))...)
		findings = append(findings, notes(checkCoverageRatchet(collected.Requirements, collected.Tags,
			collected.Enrolled, baselinePolarities(committed.Tags), baseEnrolled))...)
		findings = append(findings, notes(checkEvidenceRatchet(collected.Requirements, collected.Tags,
			collected.Enrolled, carriers, baselineEvidence(tree, committed.Tags), baseEnrolled))...)
	}
	findings = append(findings, notes(collected.ParseErrors)...)
	findings = append(findings, notes(checkIDAllocation(collected.Requirements, ids))...)
	findings = append(findings, evaluate(collected.Requirements, collected.Tags, collected.Enrolled)...)
	successors := successorsFrom(collected.Metas)
	findings = append(findings, notes(checkSuperseded(tree, collected.Requirements, successors, stems))...)
	carriers, err := carriers(tree)
	if err != nil {
		return CheckReport{}, err
	}
	compileErrors, err := checkTagPackagesCompile(tree, collected.Tags, carriers)
	if err != nil {
		return CheckReport{}, err
	}
	findings = append(findings, notes(compileErrors)...)
	findings = append(findings, notes(checkLowerLayerProducer(discriminationSources, collected.Requirements))...)
	findings = append(findings, notes(checkStatusAgreement(collected.Requirements, rows, collected.Enrolled))...)
	findings = append(findings, notes(checkSummaryDisposition(tree, collected.Metas, collected.Requirements))...)
	findings = append(findings, notes(checkPublicRowMonotonic(collected.Metas, baseMetas,
		enrolledKnown, newly))...)
	derived, err := derivedRegisters(deriver, signed, collected.Requirements)
	if err != nil {
		return CheckReport{}, err
	}
	findings = append(findings, notes(checkUnprovenSupport(collected.Requirements, rows, stems, dispositions,
		signed, derived, CoverageRows(collected.Requirements, collected.Tags, carriers)))...)
	// ARMED 2026-09-01, once every support-promising stem carried a sign-off.
	// It was written unwired on purpose: arming it while 16 stems still owed one
	// would have redded this gate for every session sharing the checkout, and a
	// gate everybody learns to ignore enforces nothing.
	//
	// It reads the ROWS rather than the summaries, and since 2026-09-01 those
	// are the same population: a row is a summary's own `Support` declaration,
	// so a public claim whose stem has no summary cannot be written. What the
	// two checks still divide is the QUESTION -- checkUnprovenSupport asks
	// whether the checklist is empty, this one whether a walk of the source
	// bounds what the checklist left out.
	findings = append(findings, notes(checkSupportedSignoff(rows, signed))...)
	findings = append(findings, notes(checkGapCountAgreement(collected.Requirements, rows))...)

	audits, err := loadAudits(tree, collected.Enrolled)
	if err != nil {
		return CheckReport{}, err
	}
	baselineAuditSet, auditsKnown := baselineAudits(tree)
	auditFileErrors, err := checkAuditFiles(tree, collected.Enrolled, stems)
	if err != nil {
		return CheckReport{}, err
	}
	findings = append(findings, notes(auditFileErrors)...)
	findings = append(findings, notes(checkAuditSchema(collected.Requirements, collected.Tags, audits))...)
	states := auditFreshness(auditFreshnessInput{Tree: tree, Requirements: collected.Requirements,
		Tags: collected.Tags, Enrolled: collected.Enrolled, Audits: audits})
	findings = append(findings, notes(checkAuditFreshness(collected.Requirements, states))...)
	findings = append(findings, notes(checkAuditDisclosure(collected.Requirements, rows, collected.Enrolled, audits))...)
	findings = append(findings, notes(checkAuditNote(tree, collected.Requirements, collected.Tags, collected.Enrolled, audits))...)
	findings = append(findings, notes(checkAuditFindings(collected.Requirements, collected.Enrolled, audits,
		baselineAuditSet, auditsKnown))...)
	findings = append(findings, notes(checkAuditVerdictRatchet(collected.Requirements, collected.Enrolled, audits,
		baselineAuditSet, auditsKnown, baseEnrolled))...)

	headRecords, headRecordsKnown := baselineDiscrimination(tree)
	headRecordBlobs, headRecordBlobsKnown := baselineRecordBlobs(tree, records)
	gated := map[string]bool{}
	for _, req := range collected.Requirements {
		if req.Gated() && collected.Enrolled[req.RFC] {
			gated[req.RID] = true
		}
	}
	obligations := discriminationInput{Verdicts: discrimination,
		Requirements: collected.Requirements, Gated: gated, Covers: discriminationCovers,
		HeadCovers: committed.Head, PriorCovers: committed.Prior, PriorKnown: committed.PriorKnown,
		BacklogCovers: committed.Backlog, BacklogRef: committed.BacklogRef,
		HeadRecords: headRecords, HeadKnown: headRecordsKnown, Carriers: carriers,
		Sources: discriminationSources, Index: discriminationIndex,
		HeadSources: newTextReader(headRecordBlobs), HeadBlobsKnown: headRecordBlobsKnown,
		HeadTagBlobs: committed.Blobs}
	findings = append(findings, notes(checkDiscriminationRatchet(obligations))...)

	findings = append(findings, notes(extractionErrors)...)
	extractions, err := LoadExtractions(tree)
	if err != nil {
		return CheckReport{}, err
	}
	findings = append(findings, notes(checkExtractionRatchet(tree, extractions))...)
	findings = append(findings, notes(checkDrainFloor(tree, collected.Enrolled, signed, today))...)
	ledgerErrors, err := checkLedgerFresh(tree, collected, rows, dispositions)
	if err != nil {
		return CheckReport{}, err
	}
	findings = append(findings, notes(ledgerErrors)...)

	report := CheckReport{Findings: findings, Violations: findingMessages(findings)}
	if len(findings) > 0 {
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
	report.SignedUnenrolled = uncredited(signed, collected.Enrolled)
	auditRows, worklist := auditCoverageRows(auditCoverageInput{Requirements: collected.Requirements,
		Tags: collected.Tags, Enrolled: collected.Enrolled, Carriers: carriers, Audits: audits, States: states})
	for _, row := range auditRows {
		report.AuditProven += row.Proven
		report.AuditVerdicts += row.Verdicts
		report.AuditDone += row.Audited
		report.AuditTotal += row.Auditable
	}
	report.AuditFindings = len(worklist)
	report.DiscriminationProven, report.DiscriminationEscaped = discriminationRouteCounts(discrimination)
	report.DiscriminationOwed = len(discriminationOwed(obligations))
	report.DiscriminationBacklog = discriminationBacklog(obligations)
	report.DiscriminationRemovable = discriminationRemovable(discrimination)
	report.DiscriminationDrifted = discriminationDrifted(obligations)
	report.DiscriminationChanged, report.DiscriminationUnresolved = discriminationChangedUnits(obligations)
	report.UnscannedTags, err = unscannedTags(tree, carriers)
	if err != nil {
		return CheckReport{}, err
	}
	return report, nil
}

// discriminationRemovable names the records whose tag is gone, in record order.
func discriminationRemovable(verdicts []DiscriminationVerdict) []string {
	var out []string
	for index := range verdicts {
		if !verdicts[index].removable() {
			continue
		}
		var tb textbuf.Buffer
		out = append(out, tb.Str(verdicts[index].Record.RID).Byte(' ').
			Str(verdicts[index].Record.Polarity).Str(" at ").
			Str(verdicts[index].Record.Unit).String())
	}
	return out
}

func intersects(left, right map[string]bool) bool {
	for key := range left {
		if right[key] {
			return true
		}
	}
	return false
}
