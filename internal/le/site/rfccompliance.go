// Design: website/AI.md -- the RFC compliance report is internal/le/rfc's own answer
// Detail: health.go holds the other quality page, over internal/le/testhealth.
package site

import (
	"encoding/json"
	"fmt"
	"html"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/le/rfc"
	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

// The RFC compliance report registers from here.
func init() {
	registerProducer(Producer{Name: rfcComplianceProducerName, Render: renderRFCCompliance})
}

// Where the RFC compliance report is published.
const (
	rfcComplianceProducerName = "rfc-compliance"
	rfcComplianceRoute        = "/quality/rfc-compliance/"
	rfcComplianceDest         = "quality/rfc-compliance/" + pageIndexFile
	rfcComplianceRoot         = "../../"
	// rfcComplianceSnapshot is the machine-readable form of the same answer,
	// linked from the page's own Check results section.
	rfcComplianceSnapshot = "data/rfc-compliance.json"
	// rfcGapBucket is the bucket a {gap} annotation puts a requirement in. It
	// is named because the gap disclosure counts it by key.
	rfcGapBucket = "gap"
	// The border color a card takes, one for each thing a number can mean.
	rfcToneOK   = "ok"
	rfcToneInfo = "info"
	rfcToneWarn = "warn"
	rfcToneBad  = "bad"
	// rfcIssuesShown bounds the open issues the page inlines. A gate that goes
	// red on a bad merge answers thousands of diagnostics, and a page is not a
	// log file.
	rfcIssuesShown = 25
)

// rfcCompliance is one whole reading of the RFC gate.
//
// It is a value rather than a set of calls so the page renders from a snapshot:
// the build derives it once, publishes it beside the page as
// data/rfc-compliance.json, and every figure the page shows comes from it.
type rfcCompliance struct {
	Gate         rfcGate     `json:"gate"`
	Satisfaction []rfcBucket `json:"satisfaction"`
	Gaps         rfcGaps     `json:"gaps"`
	Audit        rfcAudit    `json:"audit"`
	Verify       rfcVerify   `json:"verify"`
}

// rfcGate is the gate's verdict and the scale it judged.
type rfcGate struct {
	OK         bool     `json:"ok"`
	ErrorCount int      `json:"error-count"`
	GatedMust  int      `json:"gated-must"`
	Enrolled   int      `json:"enrolled-rfcs"`
	TestTags   int      `json:"test-tags"`
	Message    string   `json:"message"`
	Violations []string `json:"violations,omitempty"`
}

// rfcBucket counts the gated requirements satisfied one way.
type rfcBucket struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

// rfcGaps is what the declared gaps amount to, and what the public page says
// about the RFCs carrying them.
type rfcGaps struct {
	Requirements           int              `json:"requirements"`
	RFCs                   int              `json:"rfcs"`
	StatusCounts           []rfcStatusCount `json:"status-counts"`
	SupportedWithRemaining []rfcRemaining   `json:"supported-with-remaining"`
	TopRFCs                []rfcGapCluster  `json:"top-rfcs"`
}

// rfcStatusCount counts the RFCs with a gap that carry one public status.
type rfcStatusCount struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

// rfcRemaining is a Supported row that still discloses a gap.
type rfcRemaining struct {
	RFC       string `json:"rfc"`
	Remaining string `json:"remaining"`
}

// rfcGapCluster is one RFC and how many gaps it declares.
type rfcGapCluster struct {
	RFC    string `json:"rfc"`
	Count  int    `json:"count"`
	Status string `json:"status"`
}

// rfcAudit counts the recorded reader verdicts by freshness.
//
// A mechanical re-stamp is kept apart from a real judgement change: folding
// `shifted` into `stale` would report a line shift as a void verdict.
type rfcAudit struct {
	Verdicts int `json:"verdicts"`
	Fresh    int `json:"fresh"`
	Shifted  int `json:"shifted"`
	Stale    int `json:"stale"`
	Missing  int `json:"missing"`
}

// rfcVerify says how the gate is wired into what stops a commit.
//
// It REPLACES the retired renderer's agent-guard block, which grepped a marker
// string out of .claude/hooks/pretool-writeedit.py, counted a Makefile target,
// and counted one call in scripts/status/verify_run.go. All three files are
// gone, and the approval token that block advertised was retired by owner
// ruling on 2026-08-19 (ai/rules/testing.md), so a port would publish a
// mechanism this repository bans. This is derived from the live stage
// population instead, which is a registry rather than a text search.
type rfcVerify struct {
	Command    string `json:"command"`
	Stages     int    `json:"stages"`
	GateStages int    `json:"gate-stages"`
}

// rfcSatisfaction is how a gated requirement can be satisfied, in the order the
// page lists them, with the label, the tape's short form, and the condition
// that puts a requirement in it.
//
// The KEYS are the annotation kinds internal/le/rfc parses, and
// TestEveryAnnotationKindHasABucket holds this table against the ones the
// corpus actually carries, so a fourth kind cannot land in no bucket.
var rfcSatisfaction = []struct{ Key, Label, Short, Condition string }{
	{"both_polarities", "Positive and negative tests", "Test pair", "positive tag + negative tag"},
	{"single_polarity", "One polarity plus reason", "Single polarity",
		"{single-polarity} annotation + required tag"},
	{"not_applicable", "Not applicable", "Not applicable", "{not-applicable} annotation"},
	{rfcGapBucket, "Declared gap", "Gap", "{gap} annotation + public ledger disclosure"},
	{"one_polarity_unexcused", "One polarity, unexcused", "Unexcused one side", "tag without annotation"},
	{"missing_unexcused", "Missing, unexcused", "Missing", "no tag, no annotation"},
}

// rfcAnnotationBuckets maps an annotation kind to the bucket it satisfies.
var rfcAnnotationBuckets = map[string]string{
	"not-applicable": "not_applicable", rfcGapBucket: rfcGapBucket, "single-polarity": "single_polarity",
}

// rfcStatusOrder is the order the gap-disclosure table lists a public status
// in. A status this does not name follows, in name order.
var rfcStatusOrder = []string{"Partial", "Experimental", "Supported", "Not supported", "Unsupported"}

// rfcMissingRow is the status an RFC with a gap and no public row is counted
// under. check_status_completeness refuses that tree; the page states it rather
// than showing a blank cell.
const rfcMissingRow = "Missing public row"

// rfcGapClustersShown bounds the gap-cluster table. The tail is a long list of
// RFCs with one or two gaps each, which says less than the ledger already does.
const rfcGapClustersShown = 12

// liveRFCCompliance answers the RFC gate for one checkout. It is a variable so
// a test can state a snapshot rather than walk every summary and every test
// tag in the tree, which answers today's counts and would move a golden page.
var liveRFCCompliance = collectRFCCompliance

// renderRFCCompliance publishes the RFC compliance report, its mirror, and the
// snapshot both were rendered from.
func renderRFCCompliance(paths Paths) ([]string, error) {
	snapshot, err := liveRFCCompliance(paths.Repository)
	if err != nil {
		return nil, err
	}
	links, err := loadPageLinks(paths.Source)
	if err != nil {
		return nil, err
	}
	body, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode %s: %w", rfcComplianceSnapshot, err)
	}
	if err := writeNamedArtifact(paths.Output, rfcComplianceSnapshot, string(body)+"\n"); err != nil {
		return nil, err
	}

	const description = "Generated RFC gate report from requirement summaries, test tags, " +
		"status ledger, audits, and the verification stages that run the gate."
	shell := pageShell{
		Title:       "RFC Compliance Gate Report - Ze",
		Description: description,
		Root:        rfcComplianceRoot,
		Path:        rfcComplianceDest,
		Sidebar:     pageSidebar(rfcComplianceRoot, rfcComplianceDest, links),
	}
	if err := writePublishedPage(paths.Output, rfcComplianceDest,
		shell.render(rfcComplianceBody(snapshot)), rfcComplianceMirror(snapshot)); err != nil {
		return nil, err
	}
	return []string{rfcComplianceRoute}, nil
}

// collectRFCCompliance reads one checkout's RFC gate.
//
// The gate runs twice over the tree: rfc.Check answers the verdict and the open
// issues, and rfc.NewRenderInput answers the public ledger, the recorded audit
// verdicts and their freshness. The second is what the generated ledger page
// already reads, so this page and that page cannot disagree about a row.
func collectRFCCompliance(tree string) (rfcCompliance, error) {
	collected, err := rfc.Collect(tree)
	if err != nil {
		return rfcCompliance{}, err
	}
	input, err := rfc.NewRenderInput(tree, collected, nil, nil)
	if err != nil {
		return rfcCompliance{}, err
	}
	// The exit code restates len(report.Violations), which the page counts.
	report, _ := rfc.Check(tree)
	if report.CannotRun != "" {
		return rfcCompliance{}, fmt.Errorf("the RFC gate cannot run: %s", report.CannotRun)
	}

	var gated []rfc.Requirement
	for _, requirement := range collected.Requirements {
		if requirement.Gated() && collected.Enrolled[requirement.RFC] {
			gated = append(gated, requirement)
		}
	}
	snapshot := rfcCompliance{
		Gate: rfcGate{
			OK:         len(report.Violations) == 0,
			ErrorCount: len(report.Violations),
			GatedMust:  len(gated),
			Enrolled:   len(collected.Enrolled),
			TestTags:   len(collected.Tags),
			Violations: report.Violations,
		},
		Verify: rfcGateStages(),
	}
	snapshot.Gate.Message = rfcGateSummary(report)
	snapshot.Satisfaction, snapshot.Gaps = rfcBuckets(gated, collected.Tags, input.Rows)
	snapshot.Audit = rfcAuditCounts(gated, input.States, len(gated))
	return snapshot, nil
}

// rfcGateSummary answers the gate's own first line, which is its verdict and
// the scale it judged.
//
// It is CUT from the gate's output rather than composed here. The gate already
// states that sentence, and a second author of it would drift: the published
// page would then quote a gate output no run of the gate ever printed.
func rfcGateSummary(report rfc.CheckReport) string {
	if first, _, cut := strings.Cut(report.Text(), "\n"); cut {
		return first
	}
	return strings.TrimSpace(report.Text())
}

// rfcGateStages answers how many pre-commit verification stages run the gate.
//
// The stage population is declared in one place, so this is a registry read
// rather than a search for a call site.
func rfcGateStages() rfcVerify {
	stages := verifyengine.StagesForMode(verifyengine.Mode)
	answer := rfcVerify{Stages: len(stages)}
	for _, stage := range stages {
		if stage.Identity.Command != "rfc" {
			continue
		}
		answer.GateStages++
		answer.Command = strings.TrimSpace("./le " + stage.Identity.Command + " " +
			strings.Join(stage.Identity.Args, " "))
	}
	return answer
}

// rfcBuckets counts how every gated requirement is satisfied, and what the
// public page says about the RFCs declaring a gap.
//
// The gap cluster order is the count, descending, with ties broken by the order
// the summaries were parsed in. That is the order the retired renderer
// published, and it is stable: a Go map's own order is not.
func rfcBuckets(gated []rfc.Requirement, tags []rfc.Tag,
	rows map[string]rfc.LedgerRow) ([]rfcBucket, rfcGaps) {
	polarities := map[string]map[string]bool{}
	for _, tag := range tags {
		if polarities[tag.RID] == nil {
			polarities[tag.RID] = map[string]bool{}
		}
		polarities[tag.RID][tag.Polarity] = true
	}

	counts := map[string]int{}
	gapCounts := map[string]int{}
	var gapOrder []string
	for _, requirement := range gated {
		counts[rfcBucketOf(requirement, polarities[requirement.RID])]++
		if requirement.Annotation == nil || rfcAnnotationBuckets[requirement.Annotation.Kind] != rfcGapBucket {
			continue
		}
		if gapCounts[requirement.RFC] == 0 {
			gapOrder = append(gapOrder, requirement.RFC)
		}
		gapCounts[requirement.RFC]++
	}

	buckets := make([]rfcBucket, 0, len(rfcSatisfaction))
	for _, bucket := range rfcSatisfaction {
		buckets = append(buckets, rfcBucket{Key: bucket.Key, Count: counts[bucket.Key]})
	}
	return buckets, rfcGapsOf(counts[rfcGapBucket], gapCounts, gapOrder, rows)
}

// rfcBucketOf answers the bucket one gated requirement falls in. An annotation
// decides on its own; otherwise the tagged polarities do.
func rfcBucketOf(requirement rfc.Requirement, polarities map[string]bool) string {
	if requirement.Annotation != nil {
		if bucket, named := rfcAnnotationBuckets[requirement.Annotation.Kind]; named {
			return bucket
		}
	}
	switch {
	case len(polarities) > 1:
		return "both_polarities"
	case len(polarities) == 1:
		return "one_polarity_unexcused"
	default:
		return "missing_unexcused"
	}
}

// rfcGapsOf assembles the gap disclosure from the counted gaps and the public
// page's rows.
func rfcGapsOf(requirements int, gapCounts map[string]int, gapOrder []string,
	rows map[string]rfc.LedgerRow) rfcGaps {
	gaps := rfcGaps{Requirements: requirements, RFCs: len(gapOrder)}

	byStatus := map[string]int{}
	for _, stem := range gapOrder {
		byStatus[rfcPublicStatus(rows, stem)]++
	}
	named := map[string]bool{}
	for _, status := range rfcStatusOrder {
		named[status] = true
		if byStatus[status] > 0 {
			gaps.StatusCounts = append(gaps.StatusCounts, rfcStatusCount{Status: status, Count: byStatus[status]})
		}
	}
	var others []string
	for status := range byStatus {
		if !named[status] {
			others = append(others, status)
		}
	}
	sort.Strings(others)
	for _, status := range others {
		gaps.StatusCounts = append(gaps.StatusCounts, rfcStatusCount{Status: status, Count: byStatus[status]})
	}

	disclosed := make([]string, len(gapOrder))
	copy(disclosed, gapOrder)
	sort.Strings(disclosed)
	for _, stem := range disclosed {
		if !strings.HasPrefix(rfcPublicStatus(rows, stem), "Supported") {
			continue
		}
		gaps.SupportedWithRemaining = append(gaps.SupportedWithRemaining,
			rfcRemaining{RFC: rfcDisplayName(stem), Remaining: strings.TrimSpace(rows[stem].Remaining)})
	}

	clusters := make([]rfcGapCluster, 0, len(gapOrder))
	for _, stem := range gapOrder {
		clusters = append(clusters, rfcGapCluster{RFC: rfcDisplayName(stem),
			Count: gapCounts[stem], Status: rfcPublicStatus(rows, stem)})
	}
	// Stable, so an RFC that ties on count keeps the parse order above it.
	sort.SliceStable(clusters, func(left, right int) bool {
		return clusters[left].Count > clusters[right].Count
	})
	if len(clusters) > rfcGapClustersShown {
		clusters = clusters[:rfcGapClustersShown]
	}
	gaps.TopRFCs = clusters
	return gaps
}

// rfcPublicStatus answers what docs/features/rfc-status.md says about one stem.
func rfcPublicStatus(rows map[string]rfc.LedgerRow, stem string) string {
	row, held := rows[stem]
	if !held || row.Status == "" {
		return rfcMissingRow
	}
	return row.Status
}

// rfcDisplayName answers a summary stem the way the page prints it: rfc9012
// becomes RFC 9012, and a draft stem is upper-cased and left alone.
func rfcDisplayName(stem string) string {
	return strings.Replace(rfc.Prefix(stem), "RFC", "RFC ", 1)
}

// rfcAuditCounts counts the recorded verdicts of gated requirements by state.
func rfcAuditCounts(gated []rfc.Requirement, states map[string]rfc.Freshness, total int) rfcAudit {
	byState := map[string]int{}
	for _, requirement := range gated {
		state, held := states[requirement.RID]
		if !held {
			continue
		}
		byState[state.State]++
	}
	audit := rfcAudit{
		Fresh:   byState[rfc.FreshState],
		Shifted: byState[rfc.ShiftedState],
		Stale:   byState[rfc.StaleUnitState] + byState[rfc.StaleRequirementState],
	}
	for _, state := range rfc.FreshnessStates() {
		audit.Verdicts += byState[state]
	}
	if audit.Missing = total - audit.Verdicts; audit.Missing < 0 {
		audit.Missing = 0
	}
	return audit
}

// rfcComplianceBody renders the page under <main>.
func rfcComplianceBody(snapshot rfcCompliance) string {
	var body strings.Builder
	body.WriteString("            <section aria-labelledby=\"rfc-compliance-title\" class=\"md-content reveal cat-observe\">\n")
	body.WriteString(pageHero("RFC Compliance Gate Report",
		"Source: <code>internal/le/rfc</code>, <code>rfc/short/*.md</code>, "+
			"<code>rfc/enrolled.txt</code>, <code>docs/features/rfc-status.md</code>, and "+
			"<code>rfc/audit/*.json</code>.",
		"Quality", ` id="rfc-compliance-title"`, heroClasses) + "\n")
	body.WriteString(rfcComplianceStyle)
	body.WriteString(rfcCardGrid(snapshot))
	body.WriteString(`<div class="section-note reveal"><p><strong>Current gate output:</strong></p>` +
		`<pre class="rfc-command"><code>` + html.EscapeString(snapshot.Gate.Message) +
		"</code></pre></div>\n")

	body.WriteString("<section><h2>Requirement buckets</h2>\n" + rfcSatisfactionHTML(snapshot) + "\n</section>\n")
	body.WriteString("<section><h2>Gap disclosure</h2>\n" + rfcGapDisclosureHTML(snapshot.Gaps) + "\n</section>\n")
	body.WriteString("<section><h2>Top gap clusters</h2>\n" + rfcGapClusterHTML(snapshot.Gaps) + "\n</section>\n")
	body.WriteString("<section><h2>Gate inputs</h2>\n" + rfcInputsHTML(snapshot) + "\n</section>\n")
	body.WriteString("<section><h2>Check results</h2>\n" + rfcCheckHTML(snapshot.Gate) + "\n</section>\n")
	body.WriteString("            </section>\n")
	return body.String()
}

// rfcCardGrid renders the four headline numbers and the gate's wiring.
func rfcCardGrid(snapshot rfcCompliance) string {
	verdict, verdictTone := "OK", rfcToneOK
	if !snapshot.Gate.OK {
		verdict, verdictTone = "RED", rfcToneBad
	}
	guard, guardTone := "ON", rfcToneOK
	if snapshot.Verify.GateStages == 0 {
		guard, guardTone = "OFF", rfcToneBad
	}
	auditTone := rfcToneOK
	switch {
	case snapshot.Audit.Stale > 0:
		auditTone = rfcToneBad
	case snapshot.Audit.Shifted > 0:
		auditTone = rfcToneWarn
	}

	cards := []struct{ Label, Value, Note, Tone string }{
		{"Gate verdict", verdict, plural(snapshot.Gate.ErrorCount, "open gate issue"), verdictTone},
		{"Gated MUSTs", groupThousands(snapshot.Gate.GatedMust),
			groupThousands(snapshot.Gate.Enrolled) + " enrolled RFCs, " +
				groupThousands(snapshot.Gate.TestTags) + " resolved test tags", rfcToneInfo},
		{"Declared gaps", groupThousands(snapshot.Gaps.Requirements),
			"Across " + groupThousands(snapshot.Gaps.RFCs) + " RFCs, all forced into the public ledger", rfcToneWarn},
		{"Pre-commit gate", guard,
			snapshot.Verify.Command + " runs as " + groupThousands(snapshot.Verify.GateStages) +
				" of " + groupThousands(snapshot.Verify.Stages) + " full-mode verify stages", guardTone},
		{"Semantic verdicts", groupThousands(snapshot.Audit.Fresh),
			groupThousands(snapshot.Audit.Shifted) + " shifted, " + groupThousands(snapshot.Audit.Stale) +
				" stale, " + groupThousands(snapshot.Audit.Missing) +
				" missing and therefore not claimed", auditTone},
	}
	var out strings.Builder
	out.WriteString(`<div class="rfc-card-grid reveal">` + "\n")
	for _, card := range cards {
		out.WriteString(`<article class="rfc-card rfc-` + card.Tone + `"><span>` +
			html.EscapeString(card.Label) + "</span><strong>" + html.EscapeString(card.Value) +
			"</strong><p>" + html.EscapeString(card.Note) + "</p></article>\n")
	}
	out.WriteString("</div>\n")
	return out.String()
}

// rfcSatisfactionHTML renders the proportion tape and the bucket table. An
// empty bucket is left out of both: a zero-width tape segment shows nothing and
// a zero row says nothing.
func rfcSatisfactionHTML(snapshot rfcCompliance) string {
	total := snapshot.Gate.GatedMust
	counted := map[string]int{}
	for _, bucket := range snapshot.Satisfaction {
		counted[bucket.Key] = bucket.Count
	}

	var out strings.Builder
	out.WriteString(`<div class="rfc-tape" role="img" aria-label="RFC requirement satisfaction split">` + "\n")
	for _, bucket := range rfcSatisfaction {
		if counted[bucket.Key] == 0 {
			continue
		}
		out.WriteString(`<span class="rfc-tape-` + bucket.Key + `" style="--w: ` +
			strconv.FormatFloat(rfcPercent(counted[bucket.Key], total), 'f', 3, 64) + `%"><b>` +
			html.EscapeString(bucket.Short) + "</b><em>" +
			groupThousands(counted[bucket.Key]) + "</em></span>\n")
	}
	out.WriteString("</div>\n<table>\n" +
		"<thead><tr><th>Bucket</th><th>Count</th><th>Share</th><th>Source condition</th></tr></thead>\n<tbody>\n")
	for _, bucket := range rfcSatisfaction {
		if counted[bucket.Key] == 0 {
			continue
		}
		out.WriteString("<tr><td>" + html.EscapeString(bucket.Label) + "</td><td><strong>" +
			groupThousands(counted[bucket.Key]) + "</strong></td><td>" +
			rfcPercentText(counted[bucket.Key], total) + "</td><td><code>" +
			html.EscapeString(bucket.Condition) + "</code></td></tr>\n")
	}
	out.WriteString("</tbody>\n</table>")
	return out.String()
}

// rfcGapDisclosureHTML renders what the public page says about the RFCs
// declaring a gap, and the Supported rows that still disclose one.
func rfcGapDisclosureHTML(gaps rfcGaps) string {
	var out strings.Builder
	out.WriteString("<table>\n<thead><tr><th>Public status for RFCs with gaps</th><th>RFCs</th></tr></thead>\n<tbody>\n")
	for _, row := range gaps.StatusCounts {
		out.WriteString("<tr><td>" + html.EscapeString(row.Status) + "</td><td><strong>" +
			groupThousands(row.Count) + "</strong></td></tr>\n")
	}
	out.WriteString("</tbody>\n</table>")
	if len(gaps.SupportedWithRemaining) == 0 {
		return out.String()
	}
	out.WriteString("\n" + `<div class="rfc-note-box">` +
		"\n<h3>Supported rows that still disclose a gap</h3>\n<ul>\n")
	for _, row := range gaps.SupportedWithRemaining {
		out.WriteString("<li><strong>" + html.EscapeString(row.RFC) + "</strong>: " +
			html.EscapeString(row.Remaining) + "</li>\n")
	}
	out.WriteString("</ul></div>")
	return out.String()
}

// rfcGapClusterHTML renders the RFCs carrying the most declared gaps.
func rfcGapClusterHTML(gaps rfcGaps) string {
	if len(gaps.TopRFCs) == 0 {
		return "<p>No RFC declares a gap.</p>"
	}
	var out strings.Builder
	out.WriteString("<table>\n<thead><tr><th>RFC</th><th>Declared gaps</th><th>Public status</th></tr></thead>\n<tbody>\n")
	for _, row := range gaps.TopRFCs {
		out.WriteString("<tr><td><code>" + html.EscapeString(row.RFC) + "</code></td><td><strong>" +
			groupThousands(row.Count) + "</strong></td><td>" + html.EscapeString(row.Status) +
			"</td></tr>\n")
	}
	out.WriteString("</tbody>\n</table>")
	return out.String()
}

// rfcInputsHTML renders each input the gate reads, its producer, and what that
// producer answered for this tree.
func rfcInputsHTML(snapshot rfcCompliance) string {
	supported := 0
	for _, row := range snapshot.Gaps.StatusCounts {
		if strings.HasPrefix(row.Status, "Supported") {
			supported += row.Count
		}
	}
	rows := []struct{ Input, Producer, Observed string }{
		{"Requirement source", "rfc/short/*.md",
			groupThousands(snapshot.Gate.GatedMust) + " gated MUST-level requirements"},
		{"Enrollment", "rfc/enrolled.txt", groupThousands(snapshot.Gate.Enrolled) + " enrolled RFCs"},
		{"Test tags", "internal/, pkg/, test/", groupThousands(snapshot.Gate.TestTags) + " resolved tags"},
		{"Public ledger", "docs/features/rfc-status.md",
			groupThousands(snapshot.Gaps.RFCs) + " RFCs with gaps, " + groupThousands(supported) +
				" Supported with Remaining"},
		{"Semantic audits", "rfc/audit/*.json",
			groupThousands(snapshot.Audit.Fresh) + " fresh, " + groupThousands(snapshot.Audit.Shifted) +
				" shifted, " + groupThousands(snapshot.Audit.Stale) + " stale, " +
				groupThousands(snapshot.Audit.Missing) + " missing"},
		{"Pre-commit verification", "internal/le/verify/engine/stages.go",
			snapshot.Verify.Command + ", " + groupThousands(snapshot.Verify.GateStages) + " of " +
				groupThousands(snapshot.Verify.Stages) + " full-mode stages"},
	}
	var out strings.Builder
	out.WriteString("<table>\n<thead><tr><th>Input</th><th>Producer</th><th>Observed value</th></tr></thead>\n<tbody>\n")
	for _, row := range rows {
		out.WriteString("<tr><td>" + html.EscapeString(row.Input) + "</td><td><code>" +
			html.EscapeString(row.Producer) + "</code></td><td>" +
			html.EscapeString(row.Observed) + "</td></tr>\n")
	}
	out.WriteString("</tbody>\n</table>")
	return out.String()
}

// rfcCheckHTML renders the gate's open issues.
//
// The retired page carried a count for each named check. The Go gate answers
// one list rather than a count for each check, so a table of named zeros would
// be a number with no producer behind it. The issues themselves are published
// instead, bounded, with the count of what is left out.
func rfcCheckHTML(gate rfcGate) string {
	var out strings.Builder
	out.WriteString("<p>Generated artifacts: <a href=\"" + rfcComplianceRoot + rfcComplianceSnapshot +
		"\"><code>" + rfcComplianceSnapshot + "</code></a>, <code>" + rfcComplianceDest +
		"</code>, and <code>" + strings.TrimSuffix(rfcComplianceDest, pageIndexFile) + pageMirrorFile +
		"</code>.</p>\n")
	if len(gate.Violations) == 0 {
		out.WriteString("<p>No open issue. Every enrolled MUST-level requirement carries both test " +
			"polarities or an annotation saying why not.</p>")
		return out.String()
	}
	shown := gate.Violations
	if len(shown) > rfcIssuesShown {
		shown = shown[:rfcIssuesShown]
	}
	out.WriteString("<ul>\n")
	for _, violation := range shown {
		out.WriteString(`<li class="rfc-check-bad">` + html.EscapeString(violation) + "</li>\n")
	}
	out.WriteString("</ul>")
	if left := len(gate.Violations) - len(shown); left > 0 {
		out.WriteString("\n<p>... and " + groupThousands(left) + " more.</p>")
	}
	return out.String()
}

// rfcComplianceMirror renders the Markdown sibling.
func rfcComplianceMirror(snapshot rfcCompliance) string {
	counted := map[string]int{}
	for _, bucket := range snapshot.Satisfaction {
		counted[bucket.Key] = bucket.Count
	}

	var mirror strings.Builder
	mirror.WriteString("# RFC Compliance Gate Report\n\n")
	mirror.WriteString("Source: `internal/le/rfc`, `rfc/short/*.md`, `rfc/enrolled.txt`, " +
		"`docs/features/rfc-status.md`, and `rfc/audit/*.json`.\n\n")
	mirror.WriteString("## Current gate output\n\n```\n" + snapshot.Gate.Message + "\n```\n\n")
	mirror.WriteString("| Metric | Value |\n|---|---:|\n")
	for _, row := range []struct {
		Label string
		Value int
	}{
		{"Gate issues", snapshot.Gate.ErrorCount},
		{"Gated MUST-level requirements", snapshot.Gate.GatedMust},
		{"Enrolled RFCs", snapshot.Gate.Enrolled},
		{"Resolved test tags", snapshot.Gate.TestTags},
		{"Declared gaps", snapshot.Gaps.Requirements},
		{"RFCs with declared gaps", snapshot.Gaps.RFCs},
		{"Fresh semantic audit verdicts", snapshot.Audit.Fresh},
		{"Shifted semantic audit verdicts", snapshot.Audit.Shifted},
		{"Stale semantic audit verdicts", snapshot.Audit.Stale},
	} {
		mirror.WriteString("| " + row.Label + " | " + groupThousands(row.Value) + " |\n")
	}

	mirror.WriteString("\n## Requirement buckets\n\n")
	mirror.WriteString("| Bucket | Count | Share | Source condition |\n|---|---:|---:|---|\n")
	for _, bucket := range rfcSatisfaction {
		if counted[bucket.Key] == 0 {
			continue
		}
		mirror.WriteString("| " + bucket.Label + " | " + groupThousands(counted[bucket.Key]) + " | " +
			rfcPercentText(counted[bucket.Key], snapshot.Gate.GatedMust) + " | `" +
			bucket.Condition + "` |\n")
	}

	mirror.WriteString("\n## Gap disclosure\n\n")
	mirror.WriteString("| Public status for RFCs with gaps | RFCs |\n|---|---:|\n")
	for _, row := range snapshot.Gaps.StatusCounts {
		mirror.WriteString("| " + row.Status + " | " + groupThousands(row.Count) + " |\n")
	}
	if len(snapshot.Gaps.SupportedWithRemaining) > 0 {
		mirror.WriteString("\n### Supported rows that still disclose a gap\n\n")
		for _, row := range snapshot.Gaps.SupportedWithRemaining {
			mirror.WriteString("- **" + row.RFC + ":** " + row.Remaining + "\n")
		}
	}

	mirror.WriteString("\n## Top gap clusters\n\n")
	mirror.WriteString("| RFC | Declared gaps | Public status |\n|---|---:|---|\n")
	for _, row := range snapshot.Gaps.TopRFCs {
		mirror.WriteString("| `" + row.RFC + "` | " + groupThousands(row.Count) + " | " + row.Status + " |\n")
	}

	mirror.WriteString("\n## Gate inputs\n\n")
	mirror.WriteString("| Input | Producer | Observed value |\n|---|---|---|\n")
	for _, row := range []struct{ Input, Producer, Observed string }{
		{"Requirement source", "`rfc/short/*.md`",
			groupThousands(snapshot.Gate.GatedMust) + " gated MUST-level requirements"},
		{"Enrollment", "`rfc/enrolled.txt`", groupThousands(snapshot.Gate.Enrolled) + " enrolled RFCs"},
		{"Test tags", "`internal/, pkg/, test/`", groupThousands(snapshot.Gate.TestTags) + " resolved tags"},
		{"Public ledger", "`docs/features/rfc-status.md`",
			groupThousands(snapshot.Gaps.RFCs) + " RFCs with gaps"},
		{"Semantic audits", "`rfc/audit/*.json`",
			groupThousands(snapshot.Audit.Fresh) + " fresh, " + groupThousands(snapshot.Audit.Shifted) +
				" shifted, " + groupThousands(snapshot.Audit.Stale) + " stale, " +
				groupThousands(snapshot.Audit.Missing) + " missing"},
		{"Pre-commit verification", "`internal/le/verify/engine/stages.go`",
			"`" + snapshot.Verify.Command + "`, " + groupThousands(snapshot.Verify.GateStages) +
				" of " + groupThousands(snapshot.Verify.Stages) + " full-mode stages"},
	} {
		mirror.WriteString("| " + row.Input + " | " + row.Producer + " | " + row.Observed + " |\n")
	}

	mirror.WriteString("\n## Check results\n\n")
	if len(snapshot.Gate.Violations) == 0 {
		mirror.WriteString("No open issue. Every enrolled MUST-level requirement carries both test " +
			"polarities or an annotation saying why not.\n")
		return mirror.String()
	}
	shown := snapshot.Gate.Violations
	if len(shown) > rfcIssuesShown {
		shown = shown[:rfcIssuesShown]
	}
	for _, violation := range shown {
		mirror.WriteString("- " + strings.ReplaceAll(violation, "\n", " ") + "\n")
	}
	if left := len(snapshot.Gate.Violations) - len(shown); left > 0 {
		mirror.WriteString("\n... and " + groupThousands(left) + " more.\n")
	}
	return mirror.String()
}

// rfcPercent answers one share, and zero when nothing was counted.
func rfcPercent(count, total int) float64 {
	if total == 0 {
		return 0
	}
	return 100 * float64(count) / float64(total)
}

// rfcPercentText answers one share as the page prints it.
func rfcPercentText(count, total int) string {
	return strconv.FormatFloat(rfcPercent(count, total), 'f', 1, 64) + "%"
}

// groupThousands answers a count with a comma between each group of three
// digits, which is how every number on this page is printed.
func groupThousands(value int) string {
	digits := strconv.Itoa(value)
	sign := ""
	if strings.HasPrefix(digits, "-") {
		sign, digits = "-", digits[1:]
	}
	var out strings.Builder
	for index, digit := range digits {
		if index > 0 && (len(digits)-index)%3 == 0 {
			out.WriteByte(',')
		}
		out.WriteRune(digit)
	}
	return sign + out.String()
}

// rfcComplianceStyle is the page's own stylesheet, recovered from the published
// page. It is inline because these rules serve one page and nothing else links
// them.
const rfcComplianceStyle = `<style>
.rfc-card-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(9rem, 1fr)); gap: 1rem; margin: 1.25rem 0 1.6rem; }
.rfc-card { border-radius: 18px; padding: 1rem 1.1rem; background: var(--panel-strong); border: 1px solid var(--line); box-shadow: 0 1rem 2rem -1.6rem var(--shadow); }
.rfc-card span { display: block; color: var(--muted); font-size: .78rem; font-weight: 800; letter-spacing: .06em; text-transform: uppercase; }
.rfc-card strong { display: block; margin: .3rem 0; font-size: clamp(1.75rem, 4vw, 2.55rem); line-height: 1; color: var(--text); }
.rfc-card p { margin: 0; color: var(--muted); font-size: .92rem; }
.rfc-ok { border-left: 7px solid var(--teal-base); }
.rfc-info { border-left: 7px solid var(--sky-base); }
.rfc-warn { border-left: 7px solid var(--gold-base); }
.rfc-bad { border-left: 7px solid var(--danger-deep); }
.rfc-tape { display: flex; min-height: 3.4rem; margin: 1rem 0 1.2rem; overflow: hidden; border-radius: 999px; border: 1px solid var(--line-strong); background: var(--panel); box-shadow: inset 0 0 0 1px rgba(255,255,255,.75); }
.rfc-tape span { width: var(--w); min-width: 5.2rem; display: flex; flex-direction: column; justify-content: center; gap: .12rem; padding: .45rem .8rem; color: #241431; }
.rfc-tape b { font-size: .82rem; line-height: 1; }
.rfc-tape em { font-style: normal; font-size: .78rem; opacity: .78; }
.rfc-tape-both_polarities { background: var(--teal-chip); }
.rfc-tape-single_polarity { background: var(--sky-chip); }
.rfc-tape-not_applicable { background: var(--grape-chip); }
.rfc-tape-gap { background: var(--gold-chip); }
.rfc-note-box { margin: 1rem 0; padding: 1rem 1.2rem; border-radius: 16px; background: var(--gold-tint); border: 1px solid var(--gold-chip); }
.rfc-note-box h3 { margin-top: 0; }
.rfc-note-box ul { margin-bottom: 0; }
.rfc-check-ok strong { color: var(--teal-deep); }
.rfc-check-bad strong { color: var(--danger-deep); }
.rfc-command { padding: .85rem 1rem; border-radius: 14px; background: var(--term-bg); color: var(--term-text); overflow-x: auto; }
.rfc-command code { color: var(--term-text); }
@media (max-width: 760px) {
  .rfc-tape { flex-direction: column; border-radius: 18px; }
  .rfc-tape span { width: 100% !important; }
}
</style>
`
