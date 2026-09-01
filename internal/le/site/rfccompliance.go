// Design: website/AI.md -- the RFC compliance report is internal/le/rfc's own answer
// Detail: health.go holds the other quality page, over internal/le/testhealth.
package site

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
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
	rfcComplianceDirectory    = "quality/" + rfcComplianceProducerName
	rfcComplianceRoute        = "/" + rfcComplianceDirectory + "/"
	rfcComplianceDest         = rfcComplianceDirectory + "/" + pageIndexFile
	rfcComplianceRoot         = "../../"
	// rfcComplianceSnapshot is the machine-readable form of the same answer,
	// linked from the page's own Check results section.
	rfcComplianceSnapshot = "data/rfc-compliance.json"
	// The bucket a gated requirement falls in. Each is named because three
	// tables read it: rfcSatisfaction declares it, rfcStanding groups it into a
	// published share, and rfcLedgerCoverage.Bucket translates a stem's own
	// counters into it. A key spelled three times is a partition three tables
	// can disagree about.
	//
	// rfcGapBucket keeps its own name because the gap disclosure counts it too.
	rfcGapBucket      = "gap"
	rfcBothBucket     = "both_polarities"
	rfcSingleBucket   = "single_polarity"
	rfcOneSideBucket  = "one_polarity_unexcused"
	rfcMissingBucket  = "missing_unexcused"
	rfcNotApplyBucket = "not_applicable"
	// The border color a card takes, one for each thing a number can MEAN.
	//
	// A color on a number is a verdict, so a number with no good or bad
	// direction takes rfcToneNeutral and gets none. Every card states the rule
	// that chose its tone, and rfcToneLegendHTML publishes those rules beside
	// the grid: a reader who cannot say why a card is amber has been given a
	// decoration rather than a fact (owner review, 2026-09-01).
	rfcToneOK      = "ok"
	rfcToneNeutral = "neutral"
	rfcToneWarn    = "warn"
	rfcToneBad     = "bad"
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
	// Findings is the same population in PARTS, straight from the gate. The
	// page renders a table from it rather than parsing Violations back into
	// fields: the checks had the requirement id, the level and the text before
	// they formatted the line, and a second reader of an undeclared format is
	// how a published column starts lying (owner review, 2026-09-01).
	Findings []rfc.Finding `json:"findings,omitempty"`
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
var rfcSatisfaction = []struct {
	Key, Label, Short, Condition string
	// Binds says the obligation binds Ze. A `{not-applicable}` one does not,
	// so it is SCOPE rather than coverage and it is kept out of every ratio's
	// denominator: an obligation that never bound Ze is not an achievement,
	// and counting it flatters the result (owner ruling, 2026-09-01).
	Binds bool
}{
	{Key: rfcBothBucket, Label: "Positive and negative tests", Short: "Test pair",
		Condition: "positive tag + negative tag", Binds: true},
	{Key: rfcSingleBucket, Label: "One polarity plus reason", Short: "Single polarity",
		Condition: "{single-polarity} annotation + required tag", Binds: true},
	{Key: rfcGapBucket, Label: "Declared gap", Short: "Gap",
		Condition: "{gap} annotation + public ledger disclosure", Binds: true},
	{Key: rfcOneSideBucket, Label: "One polarity, unexcused", Short: "Unexcused one side",
		Condition: "tag without annotation", Binds: true},
	{Key: rfcMissingBucket, Label: "Missing, unexcused", Short: "Missing",
		Condition: "no tag, no annotation", Binds: true},
	{Key: rfcNotApplyBucket, Label: "Not applicable", Short: "Not applicable",
		Condition: "{not-applicable} annotation", Binds: false},
}

// rfcAnnotationBuckets maps an annotation kind to the bucket it satisfies.
var rfcAnnotationBuckets = map[string]string{
	rfc.AnnotationNotApplicable: rfcNotApplyBucket, rfc.AnnotationGap: rfcGapBucket,
	rfc.AnnotationSinglePolarity: rfcSingleBucket,
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

// renderRFCCompliance publishes the RFC compliance report, its mirror, the
// snapshot both were rendered from, and one detail page for each summary stem.
//
// One producer writes the index and every child route. checkProducer refuses a
// duplicate name and Coverage refuses a doubled route, so a second producer for
// the detail pages would have to split the route set with this one; one
// producer over one snapshot is fewer moving parts and it is what
// renderPluginCatalog already does with 94 routes.
func renderRFCCompliance(paths Paths) ([]string, error) {
	snapshot, err := liveRFCCompliance(paths.Repository)
	if err != nil {
		return nil, err
	}
	ledger, err := loadRequirementLedger(paths.Output)
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
	if err := removeRetiredRFCPages(paths.Output, ledger); err != nil {
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
		shell.render(rfcComplianceBody(&snapshot, ledger)),
		rfcComplianceMirror(&snapshot, ledger)); err != nil {
		return nil, err
	}

	routes := make([]string, 0, len(ledger.Stems)+1)
	routes = append(routes, rfcComplianceRoute)
	for index := range ledger.Stems {
		route, err := writeRFCDetailPage(paths.Output, &ledger.Stems[index], links)
		if err != nil {
			return nil, err
		}
		routes = append(routes, route)
	}
	return routes, nil
}

// loadRequirementLedger reads the published ledger back and refuses one no page
// could be built from.
//
// A ledger naming no stem, and an entry with no stem of its own, are each
// refused BY NAME rather than skipped. A skipped entry publishes a family with
// a silent hole in it, and an empty ledger publishes an index that links
// nothing while every check the artifact carries passes (ai/rules/principles.md).
func loadRequirementLedger(output string) (rfcLedger, error) {
	var ledger rfcLedger
	if err := readArtifactJSON(output, rfcLedgerFile, &ledger); err != nil {
		return rfcLedger{}, err
	}
	if len(ledger.Stems) == 0 {
		return rfcLedger{}, fmt.Errorf("the published %s names no RFC summary", rfcLedgerFile)
	}
	for index := range ledger.Stems {
		entry := &ledger.Stems[index]
		if entry.Stem == "" {
			return rfcLedger{}, fmt.Errorf("the published %s carries an entry with no stem at position %d",
				rfcLedgerFile, index)
		}
		// A summary that is not enrolled carries the decision that keeps it
		// out. ParseMeta refuses a kind with no reason upstream, so the
		// renderers state it without a fallback; this is the boundary where a
		// truncated or hand-written artifact could reintroduce the state the
		// parser refuses, and it is refused BY NAME rather than dereferenced
		// (ai/rules/principles.md).
		if !entry.Enrolled && entry.Disposition == nil {
			return rfcLedger{}, fmt.Errorf(
				"the published %s says %s is not enrolled and carries no disposition for it; "+
					"every summary declares an `| Enrolment |` kind and its reason",
				rfcLedgerFile, entry.Stem)
		}
	}
	return ledger, nil
}

// removeRetiredRFCPages deletes the page of a summary this build no longer
// publishes, so a withdrawn or renamed summary stops being served.
//
// A page that survives on the incremental seed alone is frozen content with a
// fresh timestamp, and every other check the artifact carries passes it.
func removeRetiredRFCPages(output string, ledger rfcLedger) error {
	live := make(map[string]bool, len(ledger.Stems))
	for index := range ledger.Stems {
		live[ledger.Stems[index].Stem] = true
	}
	root := filepath.Join(output, filepath.FromSlash(rfcComplianceDirectory))
	directory, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range directory {
		if !entry.IsDir() || live[entry.Name()] {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
			return err
		}
	}
	return nil
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
			Findings:   report.Findings,
		},
		Verify: rfcGateStages(),
	}
	snapshot.Gate.Message = rfcGateSummary(&report)
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
func rfcGateSummary(report *rfc.CheckReport) string {
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
		return rfcBothBucket
	case len(polarities) == 1:
		return rfcOneSideBucket
	default:
		return rfcMissingBucket
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
func rfcComplianceBody(snapshot *rfcCompliance, ledger rfcLedger) string {
	var body strings.Builder
	body.WriteString("            <section aria-labeledby=\"rfc-compliance-title\" class=\"md-content reveal cat-observe\">\n")
	body.WriteString(pageHero("RFC Compliance Gate Report",
		"Source: <code>internal/le/rfc</code>, <code>rfc/short/*.md</code>, and "+
			"<code>rfc/audit/*.json</code>.",
		"Quality", ` id="rfc-compliance-title"`, heroClasses) + "\n")
	body.WriteString(rfcComplianceStyle)
	body.WriteString(rfcCardGrid(snapshot, ledger))
	body.WriteString(rfcGateVerdictHTML(snapshot))

	body.WriteString("<section><h2>Requirement buckets</h2>\n" + rfcSatisfactionHTML(snapshot) + "\n</section>\n")
	body.WriteString("<section><h2>Gap disclosure</h2>\n" + rfcGapDisclosureHTML(snapshot.Gaps) + "\n</section>\n")
	body.WriteString("<section><h2>Exclusion disclosure</h2>\n" +
		rfcExclusionDisclosureHTML(ledger) + "\n</section>\n")
	body.WriteString("<section><h2>Top gap clusters</h2>\n" + rfcGapClusterHTML(snapshot.Gaps) + "\n</section>\n")
	body.WriteString("<section><h2>How this is checked</h2>\n" + rfcMechanismHTML(snapshot) +
		"\n</section>\n")
	body.WriteString("<section><h2>Check results</h2>\n" + rfcCheckHTML(snapshot.Gate) + "\n</section>\n")
	body.WriteString("<section><h2>Enrolled RFCs</h2>\n" + rfcEnrolledIndexHTML(ledger) + "\n</section>\n")
	body.WriteString("<section><h2>Summaries that are not enrolled</h2>\n" +
		rfcDeclinedIndexHTML(ledger) + "\n</section>\n")
	body.WriteString("            </section>\n")
	return body.String()
}

// rfcCard is one headline number: what it counts, the number, the sentence
// under it, and the tone that says whether the number is good news.
//
// The index and every per-RFC detail page state their overview through this
// type and the two renderers below, so the family has ONE card, declared once
// (ai/rules/principles.md).
type rfcCard struct {
	Label string
	Value string
	// Count is the arithmetic behind Value, on the card itself: "1,402 of 2,422
	// binding obligations". A percentage with its numerator and denominator in
	// a sentence beside it is a figure a reader has to assemble; the owner
	// asked for both on the card, the count under the percent (2026-09-01). It
	// is empty for a card whose Value is already a count.
	Count string
	Note  string
	Tone  string
	// Rule is what decides this card's tone, in the words a reader gets. It is
	// declared beside the tone it explains, so the two cannot drift, and
	// rfcToneLegendHTML is the only reader of it.
	Rule string
	// Overall says this card is the SCALE every share is taken over rather than
	// a measure of its own. It is what puts the card in the first movement.
	Overall bool
	// Partition says this card's share is one PART of the binding population.
	// The parts sum to the whole, which is what lets a reader add the cards up
	// and land on 100%. The proof ratio is not one of them: its denominator is
	// tagged units, a different population, and a card that let itself be added
	// to the others would answer a number that means nothing.
	Partition bool
}

// rfcCardGroups are the four movements a card grid reads in: the scale, then
// what Ze has, then what is neither, then what Ze owes.
//
// A reader met eight cards in one block and had to work out for themselves
// which numbers were good news (owner review, 2026-09-01). The heading now says
// it. Membership is DERIVED from the tone, except for the scale cards which
// declare themselves, so the headings and the colors cannot disagree: a card in
// "What Ze owes" is red because that is the only way it lands there.
var rfcCardGroups = []struct {
	Heading string
	Lead    string
	Tone    string
	Overall bool
}{
	{Heading: "Overall", Tone: rfcToneNeutral, Overall: true,
		Lead: "the populations every share below is taken over"},
	{Heading: "Positive", Tone: rfcToneOK,
		Lead: "what Ze has"},
	{Heading: "Neutral", Tone: rfcToneNeutral,
		Lead: "measures that are neither good news nor bad"},
	{Heading: "Negative", Tone: rfcToneBad,
		Lead: "what Ze owes"},
}

// rfcCardsIn answers the cards of one movement.
func rfcCardsIn(cards []rfcCard, tone string, overall bool) []rfcCard {
	out := make([]rfcCard, 0, len(cards))
	for _, card := range cards {
		if card.Overall == overall && card.Tone == tone {
			out = append(out, card)
		}
	}
	return out
}

// rfcCardsHTML renders the card grid in its four movements, and the sentence
// that says which of its cards add up.
func rfcCardsHTML(cards []rfcCard, whole int) string {
	var out strings.Builder
	for _, group := range rfcCardGroups {
		held := rfcCardsIn(cards, group.Tone, group.Overall)
		if len(held) == 0 {
			continue
		}
		out.WriteString("<h3>" + html.EscapeString(group.Heading) + "</h3>\n<p>" +
			html.EscapeString(group.Lead) + "</p>\n")
		out.WriteString(`<div class="rfc-card-grid reveal">` + "\n")
		for _, card := range held {
			out.WriteString(`<article class="rfc-card rfc-` + card.Tone + `"><span>` +
				html.EscapeString(card.Label) + "</span><strong>" +
				html.EscapeString(card.Value) + "</strong>")
			if card.Count != "" {
				out.WriteString("<b>" + html.EscapeString(card.Count) + "</b>")
			}
			out.WriteString("<p>" + html.EscapeString(card.Note) + "</p></article>\n")
		}
		out.WriteString("</div>\n")
	}
	out.WriteString("<p>" + html.EscapeString(rfcPartitionNote(cards, whole)) + "</p>\n")
	return out.String()
}

// rfcCardsMirror states the same cards, in the same four movements.
func rfcCardsMirror(cards []rfcCard, whole int) string {
	var out strings.Builder
	for _, group := range rfcCardGroups {
		held := rfcCardsIn(cards, group.Tone, group.Overall)
		if len(held) == 0 {
			continue
		}
		out.WriteString("### " + group.Heading + "\n\n" + group.Lead + "\n\n")
		out.WriteString("| Measure | Value | Count | What it means |\n|---|---:|---|---|\n")
		for _, card := range held {
			out.WriteString("| " + card.Label + " | " + card.Value + " | " +
				rfc.TableCell(rfcOrDash(card.Count)) + " | " + rfc.TableCell(card.Note) + " |\n")
		}
		out.WriteString("\n")
	}
	out.WriteString(rfcPartitionNote(cards, whole) + "\n")
	return out.String()
}

// rfcOrDash answers a value, or the dash an empty cell reads as.
func rfcOrDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

// rfcPartitionNote says which cards add up, and to what.
//
// A reader who adds the shares this page shows must land somewhere they can
// name. Showing two parts of a four-part split and leaving 3.3% unexplained is
// an incomplete report, which is the defect the owner found on 2026-09-01. The
// note states the whole, and it states that the proof ratio is over a different
// population so nobody adds it in.
func rfcPartitionNote(cards []rfcCard, whole int) string {
	parts := 0
	for _, card := range cards {
		if card.Partition {
			parts++
		}
	}
	if parts == 0 || whole == 0 {
		return "No card above is a share of a population, so there is nothing to add up."
	}
	return "The " + plural(parts, "share") + " marked as a part above " +
		"are the whole of the " + groupThousands(whole) +
		" obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of " +
		"TAGGED UNITS, a different population, so it is not one of them."
}

// rfcToneFor answers the tone a count takes: zero is good news, and anything
// else takes the tone the caller says a non-zero number means.
func rfcToneFor(count int, loud string) string {
	if count == 0 {
		return rfcToneOK
	}
	return loud
}

// rfcToneRule is the sentence the legend opens with.
//
// A color names what the measure MEANS, never how well Ze scores on it. The
// number under the label already carries the performance, and a color that
// graded as well as labeled made a reader decode two scales at once: it put an
// amber card on the measure that IS the good news (owner ruling, 2026-09-01).
const rfcToneRule = "A color names what the measure MEANS, not how well Ze scores on it. " +
	"Green is a good outcome at any value, red is a bad one, and neither a population nor a " +
	"scope count is an outcome, so both take no color. The number under the label is what " +
	"says how far Ze has got."

// rfcToneLegendHTML publishes the rule and the reason behind every card's color.
//
// One table, built from the cards themselves, so the rule a reader is given is
// the rule the code applied. A card whose tone is neutral says so, because "no
// direction" is an answer and a blank cell is not.
func rfcToneLegendHTML(cards []rfcCard) string {
	var rows strings.Builder
	for _, card := range cards {
		rows.WriteString(rfcRowCells(html.EscapeString(card.Label),
			`<span class="rfc-swatch rfc-`+card.Tone+`"></span> `+html.EscapeString(card.Tone),
			html.EscapeString(card.Rule)))
	}
	return "<details class=\"rfc-fold\"><summary>How to read the colors</summary>\n<p>" +
		html.EscapeString(rfcToneRule) + "</p>\n" +
		rfcTableHTML(rfcHeadCells("Card", "Tone here", "Why that color"), rows.String()) +
		"\n</details>"
}

// rfcToneLegendMirror states the same rule and the same reasons.
func rfcToneLegendMirror(cards []rfcCard) string {
	var out strings.Builder
	out.WriteString(rfcToneRule + "\n\n")
	out.WriteString(rfcMirrorHead("Card", "Tone here", "Why that color"))
	for _, card := range cards {
		out.WriteString(rfcMirrorRow(card.Label, card.Tone, rfc.TableCell(card.Rule)))
	}
	return out.String()
}

// rfcCardGrid renders the four headline numbers and the gate's wiring.
func rfcCardGrid(snapshot *rfcCompliance, ledger rfcLedger) string {
	cards := rfcComplianceCards(snapshot, ledger)
	return rfcCardsHTML(cards, rfcBindingOf(snapshot).Obligations) +
		rfcToneLegendHTML(cards) + "\n"
}

// rfcLedgerTotals is the whole corpus behind the gated population: how many
// requirements every summary declares, how many summaries there are, and what
// the tagged units of the enrolled ones add up to.
//
// The gate holds a SUBSET, and a reader given only the subset cannot tell
// whether 3,256 is what Ze covers or what exists (owner review, 2026-09-01).
// Derived from the published ledger the index already reads, so no second
// counter is added anywhere.
//
// Units, Records and Escapes are counted over the ENROLLED summaries alone,
// because that is the population every other figure on this page is taken
// over. Mixing the two would answer a ratio no reader could reproduce.
type rfcLedgerTotals struct {
	Requirements int
	Summaries    int
	Units        int
	Records      int
	Escapes      int
	Stale        int
}

// Proven answers the tagged units a LIVE recorded break stands behind. An
// escape claims no break exists and a stale record rests on bytes that have
// moved, so neither is one.
func (t rfcLedgerTotals) Proven() int { return t.Records - t.Escapes - t.Stale }

// rfcTotalsOf counts them.
func rfcTotalsOf(ledger rfcLedger) rfcLedgerTotals {
	totals := rfcLedgerTotals{Summaries: len(ledger.Stems)}
	for index := range ledger.Stems {
		entry := &ledger.Stems[index]
		totals.Requirements += entry.Coverage.Requirements
		if !entry.Enrolled {
			continue
		}
		totals.Units += entry.Coverage.Units
		totals.Records += entry.Coverage.Records
		totals.Escapes += entry.Coverage.Escapes
		totals.Stale += entry.Coverage.Stale
	}
	return totals
}

// rfcStanding groups the binding buckets into the ratios the cards publish.
//
// EVERY binding bucket appears in exactly one group. That is what makes the
// ratios partition the denominator, so a reader who adds the shares lands on
// 100% rather than on 96.7% with nowhere to look for the rest -- which is what
// the owner found on 2026-09-01. TestTheRatioCardsPartitionTheirDenominator
// holds the property rather than trusting this table.
//
// Good says the measure names a GOOD outcome. A card's color names what its
// measure MEANS, never how well Ze scores on it: the number under the label
// already carries the performance, and a color that graded as well as labeled
// made a reader decode two scales at once (owner ruling, 2026-09-01).
var rfcStanding = []struct {
	Label   string
	Keys    []string
	Good    bool
	Meaning string
	Rule    string
}{
	{Label: "Tested both ways", Keys: []string{rfcBothBucket}, Good: true,
		Meaning: "a positive test proves Ze does what the requirement demands and a negative " +
			"one proves it refuses what the requirement forbids",
		Rule: "green at every value: a test pair is the outcome this gate exists to produce, " +
			"and the share under the label is what says how far Ze has got"},
	{Label: "One polarity plus reason", Keys: []string{rfcSingleBucket}, Good: true,
		Meaning: "the requirement admits no counter-case, so one polarity plus a recorded " +
			"reason is the whole proof available for it",
		Rule: "green at every value: where no counter-case exists, one polarity IS the " +
			"complete answer, and a recorded reason is what the gate demands beside it"},
	{Label: "One polarity, unexcused", Keys: []string{rfcOneSideBucket}, Good: false,
		Meaning: "one direction is tested, the other is neither tested nor excused, and " +
			"nothing states which",
		Rule: "green at zero, RED above it: half a proof with no reason for the other half"},
	{Label: "No test at all", Keys: []string{rfcGapBucket, rfcMissingBucket}, Good: false,
		Meaning: "no test carries the requirement id, whether or not a gap states why",
		Rule: "green at zero, RED above it: a binding obligation nothing exercises is a claim " +
			"with nothing behind it, whether or not a reason is stated"},
}

// rfcBinding is the population that actually binds Ze, and how it is answered.
//
// Every ratio this page leads with is taken over Obligations, never over the
// gated count: the gated count includes 834 obligations a `{not-applicable}`
// annotation says never bound Ze, and a denominator carrying them makes the
// answer look better than it is (owner ruling, 2026-09-01).
type rfcBinding struct {
	Gated       int
	OutOfScope  int
	Obligations int
	Pairs       int
	NoTest      int
}

// Bucket answers this summary's count for one index bucket key.
//
// It is the one translation between the two shapes the same partition is
// counted in: the index counts by bucket key over the whole corpus, a stem page
// counts by annotation and polarity over its own requirements, and rfcStanding
// groups them for both. Without it the grouping would be written twice and the
// two pages could disagree about what a card means.
func (c rfcLedgerCoverage) Bucket(key string) int {
	switch key {
	case rfcBothBucket:
		return c.Both
	case rfcSingleBucket:
		return c.SinglePolarity
	case rfcOneSideBucket:
		return c.One
	case rfcGapBucket:
		return c.GatedGaps
	case rfcMissingBucket:
		return c.Missing
	default:
		return 0
	}
}

// rfcStandingCards answers one card per rfcStanding group, over a population
// and a counter for it.
//
// One builder for the index and for every stem page, so the two cannot publish
// different partitions of the same idea (ai/rules/principles.md).
func rfcStandingCards(whole int, countOf func(string) int) []rfcCard {
	cards := make([]rfcCard, 0, len(rfcStanding))
	for _, group := range rfcStanding {
		part := 0
		for _, key := range group.Keys {
			part += countOf(key)
		}
		tone := rfcToneOK
		if !group.Good {
			tone = rfcToneFor(part, rfcToneBad)
		}
		cards = append(cards, rfcCard{Label: group.Label,
			Value: rfcPercentText(part, whole),
			Count: groupThousands(part) + " of " + groupThousands(whole) +
				" binding obligations",
			Note: group.Meaning, Tone: tone, Rule: group.Rule, Partition: true})
	}
	return cards
}

// rfcBindingOf splits the gated population into what binds Ze and what does
// not, from the buckets the snapshot already carries.
func rfcBindingOf(snapshot *rfcCompliance) rfcBinding {
	counted := map[string]int{}
	for _, bucket := range snapshot.Satisfaction {
		counted[bucket.Key] = bucket.Count
	}
	split := rfcBinding{Gated: snapshot.Gate.GatedMust, Pairs: counted[rfcBothBucket]}
	for _, bucket := range rfcSatisfaction {
		if bucket.Binds {
			split.Obligations += counted[bucket.Key]
			continue
		}
		split.OutOfScope += counted[bucket.Key]
	}
	split.NoTest = counted[rfcGapBucket] + counted[rfcMissingBucket]
	return split
}

// rfcComplianceCards answers the index page's own headline numbers, each with
// the rule that chose its tone.
//
// SCALE first, then STANDING. The grid opens with the population the gate holds
// and the part of it that does not bind Ze, then the three shares that
// partition what does, then the proof ratio over its own denominator. The
// population leads by owner amendment of 2026-09-01, which supersedes the
// ratio-first order of the ruling earlier the same day; what has not changed is
// WHY that rule existed, so `Gated MUSTs` is labeled as scale, carries the
// neutral tone and the sentence saying a population is not a result, and the
// coverage shares sit in the same grid immediately after it.
func rfcComplianceCards(snapshot *rfcCompliance, ledger rfcLedger) []rfcCard {
	totals := rfcTotalsOf(ledger)
	split := rfcBindingOf(snapshot)

	cards := []rfcCard{
		{Label: "Gated MUSTs", Value: groupThousands(split.Gated), Overall: true,
			Count: groupThousands(totals.Requirements) + " extracted from " +
				groupThousands(totals.Summaries) + " summaries",
			Note: "MUST-level requirements the gate HOLDS, across " +
				groupThousands(snapshot.Gate.Enrolled) + " enrolled RFCs. A population, not a " +
				"result: the shares beside it are what says how Ze stands",
			Tone: rfcToneNeutral,
			Rule: "no color: a population is a scale, and a larger one is neither good news " +
				"nor bad. It is the accounting total"},
		{Label: "Out of scope", Value: groupThousands(split.OutOfScope), Overall: true,
			Count: "of " + groupThousands(split.Gated) + " gated MUSTs",
			Note: "a {not-applicable} annotation says the obligation does not bind Ze. Scope, " +
				"not coverage: it is in no share below",
			Tone: rfcToneNeutral,
			Rule: "no color: an obligation that never bound Ze is neither an achievement nor a " +
				"failure, and counting it either way would be a claim"},
	}
	cards = append(cards, rfcStandingCards(split.Obligations, func(key string) int {
		for _, bucket := range snapshot.Satisfaction {
			if bucket.Key == key {
				return bucket.Count
			}
		}
		return 0
	})...)
	return append(cards,
		rfcProofCard(totals.Proven(), totals.Units, totals.Escapes, totals.Stale,
			" in enrolled RFCs"),
		rfcCard{Label: "Gate verdict", Value: rfcGateWord(snapshot.Gate.OK),
			Count: plural(snapshot.Gate.ErrorCount, "open gate issue"),
			Note:  "whether ./le rfc check passes over this tree",
			Tone:  rfcGateTone(snapshot.Gate.OK),
			Rule:  "the verdict IS the value: green when the gate passes, red when it does not"},
		rfcCard{Label: "Semantic verdicts", Value: groupThousands(snapshot.Audit.Fresh),
			Count: groupThousands(snapshot.Audit.Shifted) + " shifted, " +
				groupThousands(snapshot.Audit.Stale) + " stale, " +
				groupThousands(snapshot.Audit.Missing) + " missing",
			Note: "requirements a reader has judged and whose judgement is still current. A " +
				"missing verdict is not claimed, and the shifted and stale ones are named on " +
				"their own RFC's page",
			Tone: rfcToneNeutral,
			Rule: "no color: a count of judgements recorded is a scale rather than an outcome, " +
				"and the shifted and stale counts beside it are the states that need reading"})
}

// rfcProofCard answers the proof ratio, which is over TAGGED UNITS and is
// therefore not one of the shares that partition the binding obligations.
//
// The card says what a recorded break IS, because a reader is right to ask
// whether a red observed once at authoring time is a current property. It is
// not re-run. What `verifyOneDiscrimination` (internal/le/rfc/discriminate.go)
// re-checks on every run is that nothing the red rested on has moved: the
// tagged unit still exists and still carries the tag, and the unit's behavior,
// the tag's claim and the producer's behavior still hash to what the record
// stored. A record whose ground moved is a LAPSED proof, counted apart here and
// named under its requirement id on its RFC's page.
func rfcProofCard(proven, units, escapes, stale int, where string) rfcCard {
	count := groupThousands(proven) + " of " + groupThousands(units) + " tagged units" + where
	if escapes+stale > 0 {
		count += ", " + groupThousands(escapes) + " escaped and " + groupThousands(stale) +
			" lapsed"
	}
	return rfcCard{Label: "Proven by a recorded break", Value: rfcPercentText(proven, units),
		Count: count,
		Note: "a red was observed once under a recorded procedure, and the unit, the claim " +
			"and the producer it rested on still hash to what was recorded. The break is not " +
			"re-run. A test pair is not a proof until one has been observed",
		Tone: rfcToneOK,
		Rule: "green at every value: an observed break is the outcome the discrimination gate " +
			"exists to produce. The denominator is TAGGED UNITS, not obligations, so this " +
			"share is not one of the parts above"}
}

// rfcGateWord and rfcGateTone answer the verdict the card and the status block
// both carry, so the two cannot say different things about one gate run.
func rfcGateWord(ok bool) string {
	if ok {
		return "OK"
	}
	return "RED"
}

func rfcGateTone(ok bool) string {
	if ok {
		return rfcToneOK
	}
	return rfcToneBad
}

// rfcGateVerdictHTML renders the gate's verdict as the STATUS it is.
//
// It was published as a `<pre>` block until 2026-09-01, which gave a one-line
// verdict terminal styling and the copy button website/assets/js/site.js
// attaches to every `pre`. Both told a reader this was console output to paste
// into a shell. It is not: it is what the gate answered for this tree, and the
// only thing here a reader can run is the invocation that reproduces it (owner
// review, 2026-09-01).
func rfcGateVerdictHTML(snapshot *rfcCompliance) string {
	return `<div class="rfc-verdict rfc-` + rfcGateTone(snapshot.Gate.OK) + `">` + "\n" +
		"<p><span>Gate verdict</span><strong>" + rfcGateWord(snapshot.Gate.OK) +
		"</strong></p>\n<p>" + html.EscapeString(rfcGateVerdictText(snapshot)) +
		"</p>\n<p>Reproduce it with <code>" + html.EscapeString(snapshot.Verify.Command) +
		"</code>. The gate's own line reads <code>" + html.EscapeString(snapshot.Gate.Message) +
		"</code>.</p>\n</div>\n"
}

// rfcGateVerdictText says what the verdict means, in the page's own voice.
func rfcGateVerdictText(snapshot *rfcCompliance) string {
	if snapshot.Gate.OK {
		return "Every enrolled MUST-level requirement carries both test polarities or an " +
			"annotation saying why not."
	}
	return plural(snapshot.Gate.ErrorCount, "open gate issue") +
		". Check results below names them, up to the " + strconv.Itoa(rfcIssuesShown) +
		" this page inlines."
}

// rfcSatisfactionHTML renders the proportion tape, its key, and the bucket
// table with the totals that prove the accounting.
//
// The shares are over the BINDING population, never over the gated count. The
// gated count carries 834 obligations a `{not-applicable}` annotation says
// never bound Ze, and a share taken over it answers a question nobody asked:
// "of everything we looked at, how much did we cover", where the honest one is
// "of everything that binds us, how much do we cover" (owner ruling,
// 2026-09-01). The not-applicable row is BELOW the binding total, marked as
// scope, and the gated total is below that.
//
// The tape carries no text of its own. A bucket at 4.5% of the width had a
// label wider than its segment, so the small buckets -- which are the ones that
// matter -- were the unreadable ones. The tape is the proportion, the key
// beneath it is the words, and every bucket is legible at any width.
//
// An empty bucket is left out of the tape and its key: a zero-width segment
// shows nothing and a zero row says nothing. Every bucket the vocabulary
// declares keeps its table row, so the accounting cannot read as complete while
// a bucket is missing from it.
func rfcSatisfactionHTML(snapshot *rfcCompliance) string {
	counted := map[string]int{}
	for _, bucket := range snapshot.Satisfaction {
		counted[bucket.Key] = bucket.Count
	}
	split := rfcBindingOf(snapshot)

	var out strings.Builder
	out.WriteString(`<div class="rfc-tape" role="img" aria-label="How the obligations that bind Ze are answered">` + "\n")
	for _, bucket := range rfcSatisfaction {
		if !bucket.Binds || counted[bucket.Key] == 0 {
			continue
		}
		out.WriteString(`<span class="rfc-tape-` + bucket.Key + `" style="--w: ` +
			strconv.FormatFloat(rfcPercent(counted[bucket.Key], split.Obligations), 'f', 3, 64) +
			`%"></span>` + "\n")
	}
	out.WriteString("</div>\n<ul class=\"rfc-tape-key\">\n")
	for _, bucket := range rfcSatisfaction {
		if !bucket.Binds || counted[bucket.Key] == 0 {
			continue
		}
		out.WriteString(`<li><span class="rfc-swatch rfc-tape-` + bucket.Key + `"></span> ` +
			html.EscapeString(bucket.Label) + ": <strong>" + groupThousands(counted[bucket.Key]) +
			"</strong> (" + rfcPercentText(counted[bucket.Key], split.Obligations) + ")</li>\n")
	}
	out.WriteString("</ul>\n<p>" + html.EscapeString(rfcScopeNote(split)) + "</p>\n")
	out.WriteString(rfcTableHTML(rfcHeadCells("Bucket", "Count", "Share of binding",
		"Source condition"), rfcSatisfactionRows(counted, split)))
	return out.String()
}

// rfcSatisfactionRows answers the bucket rows, the binding total, the scope row
// and the accounting total, in that order.
func rfcSatisfactionRows(counted map[string]int, split rfcBinding) string {
	var rows strings.Builder
	accounted := 0
	for _, bucket := range rfcSatisfaction {
		if !bucket.Binds {
			continue
		}
		accounted += counted[bucket.Key]
		rows.WriteString(rfcRowCells(html.EscapeString(bucket.Label),
			"<strong>"+groupThousands(counted[bucket.Key])+"</strong>",
			rfcPercentText(counted[bucket.Key], split.Obligations),
			"<code>"+html.EscapeString(bucket.Condition)+"</code>"))
	}
	rows.WriteString(`<tr class="rfc-total"><td><strong>` +
		html.EscapeString(rfcBindingLabel) + `</strong></td><td><strong>` +
		groupThousands(accounted) + "</strong></td><td>" +
		rfcPercentText(accounted, split.Obligations) + "</td><td>" +
		html.EscapeString(rfcAccountedNote(accounted, split.Obligations)) + "</td></tr>\n")
	for _, bucket := range rfcSatisfaction {
		if bucket.Binds {
			continue
		}
		rows.WriteString(rfcRowCells(html.EscapeString(bucket.Label),
			"<strong>"+groupThousands(counted[bucket.Key])+"</strong>", "-",
			"<code>"+html.EscapeString(bucket.Condition)+"</code>: "+
				html.EscapeString(rfcScopeCell)))
	}
	rows.WriteString(`<tr class="rfc-total"><td><strong>` +
		html.EscapeString(rfcGatedLabel) + `</strong></td><td><strong>` +
		groupThousands(split.Gated) + "</strong></td><td>-</td><td>" +
		html.EscapeString(rfcGatedNote(split)) + "</td></tr>\n")
	return rows.String()
}

// The three labels and the two sentences the accounting rows carry.
const (
	rfcBindingLabel = "Obligations that bind Ze"
	rfcGatedLabel   = "Gated MUST-level requirements"
	rfcScopeCell    = "the obligation does not bind Ze, so it is scope rather than coverage"
)

// rfcAccountedNote says whether the binding buckets account for the binding
// population.
//
// A mismatch is STATED rather than hidden. The buckets are meant to partition
// it, so a difference is a defect in the bucketing and a page that printed only
// the sum would let it pass.
func rfcAccountedNote(accounted, obligations int) string {
	if accounted == obligations {
		return "every obligation that binds Ze falls in exactly one bucket above"
	}
	return "the buckets account for " + groupThousands(accounted) + " of " +
		groupThousands(obligations) + ", so " + groupThousands(obligations-accounted) +
		" fall in none: the bucketing is incomplete"
}

// rfcGatedNote states the accounting total as the sum it is.
func rfcGatedNote(split rfcBinding) string {
	return "the accounting total: " + groupThousands(split.Obligations) +
		" that bind Ze plus " + groupThousands(split.OutOfScope) + " that do not"
}

// rfcScopeNote says what the bar leaves out, so a reader is never shown a
// proportion whose population they cannot name.
func rfcScopeNote(split rfcBinding) string {
	if split.OutOfScope == 0 {
		return "The bar is every gated MUST-level requirement: none of them is out of scope."
	}
	return "The bar is the " + groupThousands(split.Obligations) +
		" obligations that bind Ze. " + groupThousands(split.OutOfScope) +
		" further gated MUSTs are {not-applicable}: they do not bind Ze, they are not in the " +
		"bar, and they are counted apart below."
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

// rfcCheckHTML renders the gate's open findings as a TABLE.
//
// It was one bullet per finding until 2026-09-01, each a single line of the
// shape `<file>:<line>: <RID> [<LEVEL>] <issue>: "<text>" (§<section>)`, which
// truncated on screen. The columns come from rfc.Finding, which carries the
// parts the checks had before they formatted them, so nothing here parses that
// line back apart. A finding with no requirement in hand -- about a file, a
// ratchet, a ledger row -- states itself in the one column it fills.
//
// The requirement id links to its own page and row. Those pages exist now,
// which is what makes the table worth more than the list.
func rfcCheckHTML(gate rfcGate) string {
	var out strings.Builder
	if len(gate.Findings) == 0 {
		out.WriteString("<p>No open issue. Every enrolled MUST-level requirement carries both test " +
			"polarities or an annotation saying why not.</p>")
		return out.String()
	}
	shown := gate.Findings
	if len(shown) > rfcIssuesShown {
		shown = shown[:rfcIssuesShown]
	}
	var rows strings.Builder
	for index := range shown {
		finding := &shown[index]
		rows.WriteString(rfcRowCells(
			html.EscapeString(rfcFindingRFC(finding)),
			rfcFindingRequirementHTML(finding),
			html.EscapeString(rfcOrDash(finding.Level)),
			html.EscapeString(rfcFindingIssue(finding)),
			html.EscapeString(rfcOrDash(finding.Text))))
	}
	out.WriteString(rfcTableHTML(rfcHeadCells("RFC", "Requirement", "Level", "What is wrong",
		"The requirement"), rows.String()))
	if left := len(gate.Findings) - len(shown); left > 0 {
		out.WriteString("\n<p>" + html.EscapeString(rfcFindingsLeftOut(left)) + "</p>")
	}
	return out.String()
}

// rfcCheckMirror states the same table.
func rfcCheckMirror(gate rfcGate) string {
	if len(gate.Findings) == 0 {
		return "No open issue. Every enrolled MUST-level requirement carries both test " +
			"polarities or an annotation saying why not.\n"
	}
	shown := gate.Findings
	if len(shown) > rfcIssuesShown {
		shown = shown[:rfcIssuesShown]
	}
	var out strings.Builder
	out.WriteString(rfcMirrorHead("RFC", "Requirement", "Level", "What is wrong",
		"The requirement"))
	for index := range shown {
		finding := &shown[index]
		out.WriteString(rfcMirrorRow(rfcFindingRFC(finding),
			rfcFindingRequirementMirror(finding), rfcOrDash(finding.Level),
			rfc.TableCell(rfcFindingIssue(finding)), rfc.TableCell(rfcOrDash(finding.Text))))
	}
	if left := len(gate.Findings) - len(shown); left > 0 {
		out.WriteString("\n" + rfcFindingsLeftOut(left) + "\n")
	}
	return out.String()
}

// rfcFindingsLeftOut says how many findings the page does not inline.
//
// A bound with no count beside it publishes 25 issues and hides the rest, which
// is the aggregate this family exists to replace.
func rfcFindingsLeftOut(left int) string {
	return plural(left, "further finding") +
		" not shown here. The whole list is in " + rfcComplianceSnapshot +
		", and each one is on its own RFC's page under the requirement it names."
}

// rfcFindingRFC answers the summary stem a finding was raised against, from the
// requirement id the check carried.
func rfcFindingRFC(finding *rfc.Finding) string {
	if stem := rfcFindingStem(finding); stem != "" {
		return rfcDisplayName(stem)
	}
	return "-"
}

// rfcFindingStem answers the stem a finding's requirement belongs to, from the
// summary path the check recorded. Empty for a finding about no requirement.
func rfcFindingStem(finding *rfc.Finding) string {
	const prefix = "rfc/short/"
	if finding.RID == "" || !strings.HasPrefix(finding.Where, prefix) {
		return ""
	}
	file, _, _ := strings.Cut(strings.TrimPrefix(finding.Where, prefix), ":")
	return strings.TrimSuffix(file, ".md")
}

// rfcFindingRequirementHTML links a finding's requirement to its own row on its
// own page, and states the raising site for a finding about no requirement.
func rfcFindingRequirementHTML(finding *rfc.Finding) string {
	if finding.RID == "" {
		return "<code>" + html.EscapeString(rfcOrDash(finding.Where)) + "</code>"
	}
	stem := rfcFindingStem(finding)
	if stem == "" {
		return "<code>" + html.EscapeString(finding.RID) + "</code>"
	}
	return `<a href="` + html.EscapeString(rfcStemHref(stem)+"#"+rfcAnchor(finding.RID)) +
		`"><code>` + html.EscapeString(finding.RID) + "</code></a>"
}

// rfcFindingRequirementMirror states the same link.
func rfcFindingRequirementMirror(finding *rfc.Finding) string {
	if finding.RID == "" {
		return "`" + rfcOrDash(finding.Where) + "`"
	}
	stem := rfcFindingStem(finding)
	if stem == "" {
		return "`" + finding.RID + "`"
	}
	return "[`" + finding.RID + "`](" + rfcStemHref(stem) + pageMirrorFile + "#" +
		rfcAnchor(finding.RID) + ")"
}

// rfcFindingIssue answers what is wrong. A finding that carries no parts states
// its whole message here, because that message IS what is wrong about it.
func rfcFindingIssue(finding *rfc.Finding) string {
	if strings.TrimSpace(finding.Issue) != "" {
		return finding.Issue
	}
	return finding.Message
}

// rfcComplianceMirror renders the Markdown sibling.
func rfcComplianceMirror(snapshot *rfcCompliance, ledger rfcLedger) string {
	counted := map[string]int{}
	for _, bucket := range snapshot.Satisfaction {
		counted[bucket.Key] = bucket.Count
	}

	var mirror strings.Builder
	mirror.WriteString("# RFC Compliance Gate Report\n\n")
	mirror.WriteString("Source: `internal/le/rfc`, `rfc/short/*.md`, and " +
		"`rfc/audit/*.json`.\n\n")
	mirror.WriteString("## Gate verdict\n\n" + rfcGateWord(snapshot.Gate.OK) + ". " +
		rfcGateVerdictText(snapshot) + " Reproduce it with `" + snapshot.Verify.Command +
		"`. The gate's own line reads `" + snapshot.Gate.Message + "`.\n\n")
	cards := rfcComplianceCards(snapshot, ledger)
	mirror.WriteString(rfcCardsMirror(cards, rfcBindingOf(snapshot).Obligations) + "\n" +
		rfcToneLegendMirror(cards) + "\n")
	mirror.WriteString("| Metric | Value |\n|---|---:|\n")
	for _, row := range []struct {
		Label string
		Value int
	}{
		{"Gate issues", snapshot.Gate.ErrorCount},
		{"Gated MUST-level requirements", snapshot.Gate.GatedMust},
		{"Enrolled RFCs", snapshot.Gate.Enrolled},
		{"Resolved test tags", snapshot.Gate.TestTags},
		{rfcDeclaredGapsLabel, snapshot.Gaps.Requirements},
		{"RFCs with declared gaps", snapshot.Gaps.RFCs},
		{"Fresh semantic audit verdicts", snapshot.Audit.Fresh},
		{"Shifted semantic audit verdicts", snapshot.Audit.Shifted},
		{"Stale semantic audit verdicts", snapshot.Audit.Stale},
	} {
		mirror.WriteString("| " + row.Label + " | " + groupThousands(row.Value) + " |\n")
	}

	split := rfcBindingOf(snapshot)
	mirror.WriteString("\n## Requirement buckets\n\n")
	mirror.WriteString(rfcScopeNote(split) + "\n\n")
	mirror.WriteString("| Bucket | Count | Share of binding | Source condition |\n|---|---:|---:|---|\n")
	accounted := 0
	for _, bucket := range rfcSatisfaction {
		if !bucket.Binds {
			continue
		}
		accounted += counted[bucket.Key]
		mirror.WriteString("| " + bucket.Label + " | " + groupThousands(counted[bucket.Key]) + " | " +
			rfcPercentText(counted[bucket.Key], split.Obligations) + " | `" +
			bucket.Condition + "` |\n")
	}
	mirror.WriteString("| **" + rfcBindingLabel + "** | **" + groupThousands(accounted) + "** | " +
		rfcPercentText(accounted, split.Obligations) + " | " +
		rfc.TableCell(rfcAccountedNote(accounted, split.Obligations)) + " |\n")
	for _, bucket := range rfcSatisfaction {
		if bucket.Binds {
			continue
		}
		mirror.WriteString("| " + bucket.Label + " | " + groupThousands(counted[bucket.Key]) +
			" | - | `" + bucket.Condition + "`: " + rfc.TableCell(rfcScopeCell) + " |\n")
	}
	mirror.WriteString("| **" + rfcGatedLabel + "** | **" + groupThousands(split.Gated) +
		"** | - | " + rfc.TableCell(rfcGatedNote(split)) + " |\n")

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

	mirror.WriteString("\n## Exclusion disclosure\n\n")
	mirror.WriteString(rfcExclusionDisclosureMirror(ledger))

	mirror.WriteString("\n## Top gap clusters\n\n")
	mirror.WriteString("| RFC | Declared gaps | Public status |\n|---|---:|---|\n")
	for _, row := range snapshot.Gaps.TopRFCs {
		mirror.WriteString("| `" + row.RFC + "` | " + groupThousands(row.Count) + " | " + row.Status + " |\n")
	}

	mirror.WriteString("\n## How this is checked\n\n")
	mirror.WriteString(rfcMechanismMirror(snapshot))

	mirror.WriteString("\n## Check results\n\n")
	mirror.WriteString(rfcCheckMirror(snapshot.Gate))
	mirror.WriteString(rfcIndexMirror(ledger))
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

// rfcComplianceStyle is the RFC family's own stylesheet, recovered from the
// published page. It is inline because these rules serve the index and its
// detail pages and nothing else links them.
//
// .rfc-table-wrap is the scrolling container these pages wrap every table in.
// The site convention is the one .cmd-eq-table-wrap already holds for the
// command family: wide content scrolls inside its own box, and the page body
// never scrolls sideways. A test path is one unbreakable token, so a table of
// them overflows without it.
const rfcComplianceStyle = `<style>
.rfc-table-wrap { overflow-x: auto; }
.rfc-id-list { margin: .6rem 0; line-height: 1.9; }
.rfc-verdict { margin: 1rem 0 1.6rem; padding: 1rem 1.2rem; border-radius: 18px; background: var(--panel-strong); border: 1px solid var(--line); box-shadow: 0 1rem 2rem -1.6rem var(--shadow); }
.rfc-verdict p { margin: 0 0 .4rem; color: var(--muted); }
.rfc-verdict p:last-child { margin-bottom: 0; }
.rfc-verdict span { display: block; color: var(--muted); font-size: .78rem; font-weight: 800; letter-spacing: .06em; text-transform: uppercase; }
.rfc-verdict strong { display: block; margin: .2rem 0 .5rem; font-size: clamp(1.5rem, 3vw, 2rem); line-height: 1; color: var(--text); }
.rfc-card-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(13rem, 1fr)); grid-auto-rows: 1fr; gap: 1rem; margin: 1.25rem 0 .8rem; }
.rfc-card { display: flex; flex-direction: column; border-radius: 18px; padding: 1rem 1.1rem; background: var(--panel-strong); border: 1px solid var(--line); box-shadow: 0 1rem 2rem -1.6rem var(--shadow); }
.rfc-card span { display: block; color: var(--muted); font-size: .78rem; font-weight: 800; letter-spacing: .06em; text-transform: uppercase; }
.rfc-card strong { display: block; margin: .3rem 0 .15rem; font-size: clamp(1.75rem, 4vw, 2.55rem); line-height: 1; color: var(--text); }
.rfc-card b { display: block; margin: 0 0 .5rem; color: var(--text); font-size: .9rem; font-weight: 700; }
.rfc-card p { margin: auto 0 0; padding-top: .35rem; color: var(--muted); font-size: .92rem; }
.rfc-ok { border-left: 7px solid var(--teal-base); }
.rfc-neutral { border-left: 7px solid var(--line-strong); }
.rfc-warn { border-left: 7px solid var(--gold-base); }
.rfc-bad { border-left: 7px solid var(--danger-deep); }
.rfc-tape { display: flex; height: 1.5rem; margin: 1rem 0 .8rem; overflow: hidden; border-radius: 999px; border: 1px solid var(--line-strong); background: var(--panel); box-shadow: inset 0 0 0 1px rgba(255,255,255,.75); }
.rfc-tape span { width: var(--w); min-width: 0; display: block; }
.rfc-tape-key { display: flex; flex-wrap: wrap; gap: .35rem 1.4rem; margin: 0 0 1.4rem; padding: 0; list-style: none; font-size: .92rem; }
.rfc-tape-key li { display: flex; align-items: center; gap: .45rem; }
.rfc-swatch { display: inline-block; width: .85rem; height: .85rem; border-radius: .25rem; border: 1px solid var(--line-strong); vertical-align: -.1rem; }
.rfc-tape-both_polarities { background: var(--teal-chip); }
.rfc-tape-single_polarity { background: var(--sky-chip); }
.rfc-tape-not_applicable { background: var(--grape-chip); }
.rfc-tape-gap { background: var(--gold-chip); }
.rfc-tape-one_polarity_unexcused { background: var(--gold-base); }
.rfc-tape-missing_unexcused { background: var(--danger-deep); }
.rfc-swatch.rfc-ok { background: var(--teal-base); }
.rfc-swatch.rfc-neutral { background: var(--line-strong); }
.rfc-swatch.rfc-warn { background: var(--gold-base); }
.rfc-swatch.rfc-bad { background: var(--danger-deep); }
.rfc-total td { border-top: 2px solid var(--line-strong); }
.rfc-prose { max-width: 46rem; }
ul.rfc-prose { margin: .5rem 0 1rem; padding-left: 1.2rem; }
ul.rfc-prose li { margin: .35rem 0; line-height: 1.6; }
p.rfc-prose { margin: .5rem 0; line-height: 1.6; }
.rfc-fold { margin: .6rem 0; }
.rfc-fold > summary { cursor: pointer; font-weight: 700; }
.rfc-span > td { padding-bottom: .2rem; border-bottom: 0; }
.rfc-requirements td:first-child { position: static; background: transparent; font-weight: 400; }
.rfc-requirements .rfc-span > td { padding-top: .9rem; }
.rfc-subject { display: block; margin-top: .25rem; max-width: 52rem; }
.rfc-tests { display: grid; gap: .15rem .8rem; }
.rfc-tests-row { display: grid; grid-template-columns: 5rem 9.5rem minmax(0, 1fr); gap: .8rem; align-items: baseline; }
.rfc-tests-row > span:first-child { color: var(--muted); font-size: .88rem; }
.rfc-mark { margin: .45rem 0 0; color: var(--muted); max-width: 52rem; }
@media (max-width: 760px) {
  .rfc-tests-row { grid-template-columns: 1fr; gap: 0; padding-bottom: .35rem; }
}
.rfc-text > summary { cursor: pointer; color: var(--muted); font-size: .85rem; }
.rfc-text > p { margin: .35rem 0 0; max-width: 44rem; }
.rfc-note-box { margin: 1rem 0; padding: 1rem 1.2rem; border-radius: 16px; background: var(--gold-tint); border: 1px solid var(--gold-chip); }
.rfc-note-box h3 { margin-top: 0; }
.rfc-note-box ul { margin-bottom: 0; }
.rfc-check-ok strong { color: var(--teal-deep); }
.rfc-check-bad strong { color: var(--danger-deep); }
</style>
`

// rfcStemHref reaches one summary's detail page from this index.
func rfcStemHref(stem string) string { return stem + "/" }

// rfcEnrolledIndexHTML links every enrolled summary, with the public status and
// the gated MUST count each one carries.
//
// Every stem of rfc/short is in this table or in the declined one, so the
// family has no page a reader can reach only through search. 39 of the 190
// stems carry no row in docs/features/rfc-status.md, which is why the index of
// this family is here rather than on the mirror of that page.
func rfcEnrolledIndexHTML(ledger rfcLedger) string {
	rows := rfcIndexRows(ledger, true)
	if len(rows) == 0 {
		return "<p>No summary is enrolled.</p>"
	}
	var body strings.Builder
	for index := range rows {
		entry := rows[index]
		body.WriteString(rfcRowCells(
			"<a href=\""+html.EscapeString(rfcStemHref(entry.Stem))+"\"><code>"+
				html.EscapeString(entry.Display)+"</code></a> "+html.EscapeString(entry.Title),
			html.EscapeString(rfcIndexStatus(entry)),
			strconv.Itoa(entry.Coverage.Gated),
			strconv.Itoa(entry.Coverage.Gaps),
			strconv.Itoa(entry.Coverage.Missing)))
	}
	return rfcTableHTML(rfcHeadCells("RFC", "Public status", "Gated MUSTs",
		rfcDeclaredGapsLabel, "Gated with no test"), body.String())
}

// rfcDeclinedIndexHTML links every summary that is not enrolled, with the kind
// and the reason rfc/not-enrolled.txt declares for it.
func rfcDeclinedIndexHTML(ledger rfcLedger) string {
	rows := rfcIndexRows(ledger, false)
	if len(rows) == 0 {
		return "<p>Every summary is enrolled.</p>"
	}
	var body strings.Builder
	for index := range rows {
		entry := rows[index]
		body.WriteString(rfcRowCells(
			"<a href=\""+html.EscapeString(rfcStemHref(entry.Stem))+"\"><code>"+
				html.EscapeString(entry.Display)+"</code></a> "+html.EscapeString(entry.Title),
			html.EscapeString(rfcIndexDispositionKind(entry)),
			html.EscapeString(rfcIndexDispositionReason(entry))))
	}
	return rfcTableHTML(rfcHeadCells("RFC", "Disposition", "Reason"), body.String())
}

// rfcIndexMirror states both link tables in the Markdown sibling.
func rfcIndexMirror(ledger rfcLedger) string {
	var out strings.Builder
	out.WriteString("\n## Enrolled RFCs\n\n")
	enrolled := rfcIndexRows(ledger, true)
	if len(enrolled) == 0 {
		out.WriteString("No summary is enrolled.\n")
	} else {
		out.WriteString("| RFC | Public status | Gated MUSTs | Declared gaps | " +
			"Gated with no test |\n|---|---|---:|---:|---:|\n")
		for index := range enrolled {
			entry := enrolled[index]
			out.WriteString("| [`" + entry.Display + "`](" + rfcStemHref(entry.Stem) +
				pageMirrorFile + ") " + rfc.TableCell(entry.Title) + " | " +
				rfc.TableCell(rfcIndexStatus(entry)) + " | " +
				strconv.Itoa(entry.Coverage.Gated) + " | " +
				strconv.Itoa(entry.Coverage.Gaps) + " | " +
				strconv.Itoa(entry.Coverage.Missing) + " |\n")
		}
	}
	out.WriteString("\n## Summaries that are not enrolled\n\n")
	declined := rfcIndexRows(ledger, false)
	if len(declined) == 0 {
		out.WriteString("Every summary is enrolled.\n")
		return out.String()
	}
	out.WriteString("| RFC | Disposition | Reason |\n|---|---|---|\n")
	for index := range declined {
		entry := declined[index]
		out.WriteString("| [`" + entry.Display + "`](" + rfcStemHref(entry.Stem) +
			pageMirrorFile + ") " + rfc.TableCell(entry.Title) + " | " +
			rfcIndexDispositionKind(entry) + " | " +
			rfc.TableCell(rfcIndexDispositionReason(entry)) + " |\n")
	}
	return out.String()
}

// rfcIndexRows answers one half of the ledger, in stem order.
func rfcIndexRows(ledger rfcLedger, enrolled bool) []*rfcLedgerStem {
	rows := make([]*rfcLedgerStem, 0, len(ledger.Stems))
	for index := range ledger.Stems {
		if ledger.Stems[index].Enrolled == enrolled {
			rows = append(rows, &ledger.Stems[index])
		}
	}
	return rows
}

// rfcIndexStatus answers what docs/features/rfc-status.md says about one stem,
// and says plainly that it carries no row rather than showing a blank cell.
func rfcIndexStatus(entry *rfcLedgerStem) string {
	if entry.PublicStatus == "" {
		return rfcMissingRow
	}
	return entry.PublicStatus
}

// rfcIndexDispositionKind answers which decision keeps one summary out of the
// gate: the `| Enrolment |` kind its own Meta table declares.
func rfcIndexDispositionKind(entry *rfcLedgerStem) string {
	return entry.Disposition.Kind
}

// rfcIndexDispositionReason answers why one summary is not enrolled.
//
// No fallback: ParseMeta refuses a summary with no `| Enrolment |` row and
// refuses a kind with no `| Enrolment reason |` beside it (readEnrolment,
// internal/le/rfc/meta.go), so a summary that reaches this renderer carries
// both. A branch for a state the parser refuses is dead code that reads like a
// real case (ai/rules/principles.md).
func rfcIndexDispositionReason(entry *rfcLedgerStem) string {
	return entry.Disposition.Reason
}

// rfcExclusions is what the extraction sign-offs declined to map, aggregated
// across the corpus.
//
// Two mechanisms take an obligation off the gated ledger and the index
// published only one of them. `{not-applicable}` annotates a requirement that
// EXISTS, and the Out of scope card carries it. An excluded site never becomes
// a requirement at all: a reviewer walked the RFC's own text sentence by
// sentence and declined to map that one. A page showing the first and not the
// second answers "what does Ze not do" with half the answer (owner review,
// 2026-09-01).
type rfcExclusions struct {
	// Signed and Stems count the summaries that carry a sign-off and the
	// summaries there are. The ratio is on the page's face, because a count
	// over a third of the corpus read as a count over the corpus is the
	// flattery this family exists to prevent.
	Signed         int
	SignedEnrolled int
	Stems          int
	Enrolled       int
	Mapped         int
	// Sites counts every declined sentence. Debt counts the subset that is an
	// obligation Ze owes, and it is stated APART: summing it into "declined"
	// publishes a debt as scope (owner review, 2026-09-01).
	Sites int
	Debt  int
	// Relocated is every one of those obligations, whole.
	Relocated []rfcRelocated
	// Kinds is one row per kind the vocabulary declares, in its own order, and
	// carries a zero row rather than dropping it: a kind nobody used is a fact
	// about the corpus.
	Kinds []rfcExclusionKind
}

// rfcExclusionKind is one kind, its count, and the summaries that used it.
type rfcExclusionKind struct {
	Kind    string
	Meaning string
	// Group is ExclusionScope or ExclusionDebt, from the vocabulary. It decides
	// which section the kind is published under, so a seventh kind lands in one
	// of the two or reddens TestTheExclusionGroupsPartitionTheVocabulary.
	Group string
	Sites int
	Stems []string
	// Suspect says the repository's own rule treats this kind as presumed
	// wrong until it is justified.
	Suspect bool
}

// rfcRelocated is one obligation Ze OWES: the sentence, the id reserved for it,
// and the spec that owns it.
//
// It is carried whole rather than counted, because 17 of the corpus's 511
// exclusions are these and they say something the other 494 do not. `./le rfc
// check` refuses the sign-off unless the named spec exists and still reserves
// the id, so each row is tracked work rather than an obligation that went away.
type rfcRelocated struct {
	Stem  string
	Site  string
	ID    string
	Spec  string
	Quote string
}

// rfcExclusionsOf aggregates every sign-off in the published ledger.
func rfcExclusionsOf(ledger rfcLedger) rfcExclusions {
	out := rfcExclusions{Stems: len(ledger.Stems)}
	sites := map[string]int{}
	stems := map[string][]string{}
	for index := range ledger.Stems {
		entry := &ledger.Stems[index]
		if entry.Enrolled {
			out.Enrolled++
		}
		if entry.Extraction == nil {
			continue
		}
		out.Signed++
		if entry.Enrolled {
			out.SignedEnrolled++
		}
		out.Mapped += entry.Extraction.Mapped
		seen := map[string]bool{}
		for _, site := range entry.Extraction.Exclusions {
			out.Sites++
			sites[site.Kind]++
			if group, held := rfc.ExclusionKindGroup(site.Kind); held && group == rfc.ExclusionDebt {
				out.Debt++
				out.Relocated = append(out.Relocated, rfcRelocated{Stem: entry.Stem,
					Site: site.ID, ID: site.ReservedID, Spec: site.RelocatedTo,
					Quote: site.Quote})
			}
			if seen[site.Kind] {
				continue
			}
			seen[site.Kind] = true
			stems[site.Kind] = append(stems[site.Kind], entry.Stem)
		}
	}
	for _, kind := range rfc.ExclusionKinds() {
		meaning, _ := rfc.ExclusionKindMeaning(kind)
		group, _ := rfc.ExclusionKindGroup(kind)
		out.Kinds = append(out.Kinds, rfcExclusionKind{Kind: kind, Meaning: meaning,
			Group: group, Sites: sites[kind], Stems: stems[kind],
			Suspect: rfc.ExclusionPresumedWrong(kind)})
	}
	sort.SliceStable(out.Kinds, func(left, right int) bool {
		return out.Kinds[left].Sites > out.Kinds[right].Sites
	})
	return out
}

// rfcExclusionCoverage says how much of the corpus this count covers, on the
// section's own face.
func rfcExclusionCoverage(exclusions rfcExclusions) string {
	return groupThousands(exclusions.Signed) + " of " + groupThousands(exclusions.Stems) +
		" summaries carry an extraction sign-off, " + groupThousands(exclusions.SignedEnrolled) +
		" of them among the " + groupThousands(exclusions.Enrolled) +
		" enrolled. The other " + groupThousands(exclusions.Stems-exclusions.Signed) +
		" have no exclusion ledger at all, so what follows counts the walks that HAVE been " +
		"done and is not the whole picture."
}

// rfcExclusionCaution is the sentence the presumed-wrong kind carries.
const rfcExclusionCaution = "ai/rules/rfc-compliance.md treats binds-another-role as " +
	"PRESUMED WRONG until it is justified: Ze rarely implements one side of a protocol, so an " +
	"obligation addressed to \"the sender\" or \"the receiver\" almost always binds it, and the " +
	"label reads as \"not our problem\" where the truth is usually \"our problem, unbuilt\". " +
	"Each one is justified on its own RFC's page, under Extraction sign-off, and the " +
	"justification MUST name the role, show Ze never acts as it, and cite the producer that " +
	"would act as it if Ze did."

// rfcExclusionDisclosureHTML renders what the walks declined to map, in the two
// groups the kinds MEAN.
//
// Five kinds say the obligation never bound Ze. `relocated-to-spec` says the
// opposite: it is real, unbuilt, and a named spec owes it. Publishing the two
// under one heading files a debt as scope, which is the defect the owner found
// on 2026-09-01. The split comes from the vocabulary, so a seventh kind lands
// in one group or reddens a test rather than defaulting to scope.
func rfcExclusionDisclosureHTML(ledger rfcLedger) string {
	exclusions := rfcExclusionsOf(ledger)
	var out strings.Builder
	out.WriteString("<p>" + html.EscapeString(rfcExclusionIntro(exclusions)) + "</p>\n")
	out.WriteString("<p>" + html.EscapeString(rfcExclusionCoverage(exclusions)) + "</p>\n")
	if exclusions.Sites == 0 {
		out.WriteString("<p>" + html.EscapeString("No sign-off has declined a sentence.") + "</p>")
		return out.String()
	}
	var rows strings.Builder
	for _, kind := range exclusions.Kinds {
		rows.WriteString(rfcRowCells("<code>"+html.EscapeString(kind.Kind)+"</code>",
			html.EscapeString(rfcExclusionGroupWord(kind.Group)),
			"<strong>"+groupThousands(kind.Sites)+"</strong>",
			strconv.Itoa(len(kind.Stems)),
			html.EscapeString(kind.Meaning)))
	}
	rows.WriteString(`<tr class="rfc-total"><td><strong>` +
		html.EscapeString(rfcExcludedLabel) + `</strong></td><td>-</td><td><strong>` +
		groupThousands(exclusions.Sites) + "</strong></td><td>-</td><td>" +
		html.EscapeString(rfcExclusionTotalNote(exclusions)) + "</td></tr>\n")
	out.WriteString(rfcTableHTML(rfcHeadCells("Excluded kind", "Means", "Sites", "Summaries",
		"What it means"), rows.String()))
	for _, kind := range exclusions.Kinds {
		if len(kind.Stems) == 0 || kind.Group == rfc.ExclusionDebt {
			continue
		}
		out.WriteString("\n<p class=\"rfc-id-list\"><strong>" + html.EscapeString(kind.Kind) +
			" (" + groupThousands(kind.Sites) + "):</strong> " + rfcStemLinksHTML(kind.Stems))
		if kind.Suspect {
			out.WriteString("<br />" + html.EscapeString(rfcExclusionCaution))
		}
		out.WriteString("</p>")
	}
	out.WriteString("\n<h3>" + html.EscapeString(rfcDebtHeading) + "</h3>\n<p>" +
		html.EscapeString(rfcDebtLead(exclusions)) + "</p>\n")
	if len(exclusions.Relocated) == 0 {
		out.WriteString("<p>" + html.EscapeString("No sign-off has relocated an obligation to a "+
			"spec.") + "</p>")
		return out.String()
	}
	var debt strings.Builder
	for index := range exclusions.Relocated {
		row := &exclusions.Relocated[index]
		debt.WriteString(rfcRowCells(
			`<a href="`+html.EscapeString(rfcStemHref(row.Stem))+`"><code>`+
				html.EscapeString(rfcDisplayName(row.Stem))+"</code></a>",
			"<code>"+html.EscapeString(rfcOrUnstated(row.ID))+"</code>",
			html.EscapeString(rfcOrUnstated(row.Quote)),
			"<code>"+html.EscapeString(rfcOrUnstated(row.Spec))+"</code>"))
	}
	out.WriteString(rfcTableHTML(rfcHeadCells("RFC", "Reserved id", "The obligation",
		"The spec that owes it"), debt.String()))
	return out.String()
}

// The heading and the lead the relocated obligations carry.
const rfcDebtHeading = "Obligations relocated to a spec"

// rfcDebtLead says what this table is, in the words that keep it out of scope.
func rfcDebtLead(exclusions rfcExclusions) string {
	return "These " + plural(exclusions.Debt, "sentence") + " are NOT scope. Each is an " +
		"obligation Ze owes and has not built, moved to a named spec that reserves a " +
		"requirement id for it. ./le rfc check refuses the sign-off unless that spec exists " +
		"and still reserves the id, so each row is tracked work rather than an obligation " +
		"that went away."
}

// rfcExclusionGroupWord says what a group MEANS, in a cell.
func rfcExclusionGroupWord(group string) string {
	if group == rfc.ExclusionDebt {
		return "Ze owes it"
	}
	return "never bound Ze"
}

// rfcExclusionDisclosureMirror states the same counts, the same summaries and
// the same relocated obligations.
func rfcExclusionDisclosureMirror(ledger rfcLedger) string {
	exclusions := rfcExclusionsOf(ledger)
	var out strings.Builder
	out.WriteString(rfcExclusionIntro(exclusions) + "\n\n")
	out.WriteString(rfcExclusionCoverage(exclusions) + "\n\n")
	if exclusions.Sites == 0 {
		return out.String() + "No sign-off has declined a sentence.\n"
	}
	out.WriteString(rfcMirrorHead("Excluded kind", "Means", "Sites", "Summaries",
		"What it means"))
	for _, kind := range exclusions.Kinds {
		out.WriteString(rfcMirrorRow("`"+kind.Kind+"`", rfcExclusionGroupWord(kind.Group),
			groupThousands(kind.Sites), strconv.Itoa(len(kind.Stems)),
			rfc.TableCell(kind.Meaning)))
	}
	out.WriteString(rfcMirrorRow("**"+rfcExcludedLabel+"**", "-",
		"**"+groupThousands(exclusions.Sites)+"**", "-",
		rfc.TableCell(rfcExclusionTotalNote(exclusions))))
	for _, kind := range exclusions.Kinds {
		if len(kind.Stems) == 0 || kind.Group == rfc.ExclusionDebt {
			continue
		}
		out.WriteString("\n**" + kind.Kind + " (" + groupThousands(kind.Sites) + "):** " +
			rfcStemLinksMirror(kind.Stems) + "\n")
		if kind.Suspect {
			out.WriteString("\n" + rfcExclusionCaution + "\n")
		}
	}
	out.WriteString("\n### " + rfcDebtHeading + "\n\n" + rfcDebtLead(exclusions) + "\n\n")
	if len(exclusions.Relocated) == 0 {
		return out.String() + "No sign-off has relocated an obligation to a spec.\n"
	}
	out.WriteString(rfcMirrorHead("RFC", "Reserved id", "The obligation",
		"The spec that owes it"))
	for index := range exclusions.Relocated {
		row := &exclusions.Relocated[index]
		out.WriteString(rfcMirrorRow(
			"[`"+rfcDisplayName(row.Stem)+"`]("+rfcStemHref(row.Stem)+pageMirrorFile+")",
			"`"+rfcOrUnstated(row.ID)+"`", rfc.TableCell(rfcOrUnstated(row.Quote)),
			"`"+rfcOrUnstated(row.Spec)+"`"))
	}
	return out.String()
}

// rfcExcludedLabel names the total row.
const rfcExcludedLabel = "Sentences declined"

// rfcExclusionIntro says what an exclusion IS, and how it differs from the
// {not-applicable} count the card grid already carries.
func rfcExclusionIntro(exclusions rfcExclusions) string {
	return "A reviewer walks an RFC's own text sentence by sentence and decides which " +
		"sentences become requirements. One that does not is EXCLUDED, with a kind and a " +
		"reason, and it never reaches the gated ledger at all. That is a different mechanism " +
		"from the Out of scope card above, which counts requirements that exist and carry a " +
		"{not-applicable} annotation. Across the sign-offs done so far, " +
		groupThousands(exclusions.Mapped) + " sentences were mapped to a requirement and " +
		groupThousands(exclusions.Sites) + " were declined. " +
		groupThousands(exclusions.Debt) + " of those declines are not scope at all: they are " +
		"obligations Ze OWES, relocated to a named spec, and they are stated apart below."
}

// rfcExclusionTotalNote states the ratio the total row is part of.
func rfcExclusionTotalNote(exclusions rfcExclusions) string {
	return groupThousands(exclusions.Sites-exclusions.Debt) + " say the obligation never " +
		"bound Ze and " + groupThousands(exclusions.Debt) + " say Ze owes it, of " +
		groupThousands(exclusions.Mapped+exclusions.Sites) +
		" normative sentences the walks found"
}

// rfcStemLinksHTML and rfcStemLinksMirror name summaries as links to their own
// pages, where every exclusion carries the reason that justifies it.
func rfcStemLinksHTML(stems []string) string {
	parts := make([]string, 0, len(stems))
	for _, stem := range stems {
		parts = append(parts, `<a href="`+html.EscapeString(rfcStemHref(stem))+`"><code>`+
			html.EscapeString(rfcDisplayName(stem))+"</code></a>")
	}
	return strings.Join(parts, ", ")
}

func rfcStemLinksMirror(stems []string) string {
	parts := make([]string, 0, len(stems))
	for _, stem := range stems {
		parts = append(parts, "[`"+rfcDisplayName(stem)+"`]("+rfcStemHref(stem)+pageMirrorFile+")")
	}
	return strings.Join(parts, ", ")
}

// rfcMechanismHTML renders how the gate is checked, apart from what it found.
//
// `Pre-commit gate / ON` sat in the card grid until 2026-09-01, beside counts
// and ratios. It is not a measure: it answers "how is this enforced" rather
// than "where does Ze stand", and so do the stage count, the reproduce command,
// the inputs and the published artifacts. A reader who wants the mechanism now
// finds all of it in one place, and the grid holds only measures (owner
// review, 2026-09-01).
func rfcMechanismHTML(snapshot *rfcCompliance) string {
	var rows strings.Builder
	for _, row := range rfcMechanismRows(snapshot) {
		rows.WriteString(rfcRowCells(html.EscapeString(row[0]), row[1],
			html.EscapeString(row[2])))
	}
	return "<p>" + html.EscapeString(rfcMechanismLead(snapshot)) + "</p>\n" +
		rfcTableHTML(rfcHeadCells("Input", "Producer", "What it answered here"), rows.String())
}

// rfcMechanismMirror states the same mechanism.
func rfcMechanismMirror(snapshot *rfcCompliance) string {
	var out strings.Builder
	out.WriteString(rfcMechanismLead(snapshot) + "\n\n")
	out.WriteString(rfcMirrorHead("Input", "Producer", "What it answered here"))
	for _, row := range rfcMechanismRows(snapshot) {
		out.WriteString(rfcMirrorRow(row[0], rfc.TableCell(rfcPlain(row[1])),
			rfc.TableCell(row[2])))
	}
	return out.String()
}

// rfcMechanismLead says whether the gate runs before a commit is verified,
// which is the fact the retired card carried.
func rfcMechanismLead(snapshot *rfcCompliance) string {
	if snapshot.Verify.GateStages == 0 {
		return "NO verification stage runs this gate, so nothing on this page is enforced " +
			"before a commit is verified."
	}
	return "This gate runs before a commit is verified: " + snapshot.Verify.Command +
		" is " + plural(snapshot.Verify.GateStages, "stage") + " of the " +
		groupThousands(snapshot.Verify.Stages) + " that ./le verify current mode full runs."
}

// rfcMechanismRows answers what the gate reads, what runs it, and what it
// publishes, as (term, markup) pairs both renderings share.
func rfcMechanismRows(snapshot *rfcCompliance) [][3]string {
	supported := 0
	for _, row := range snapshot.Gaps.StatusCounts {
		if strings.HasPrefix(row.Status, "Supported") {
			supported += row.Count
		}
	}
	return [][3]string{
		{"Reproduce it", "<code>" + html.EscapeString(snapshot.Verify.Command) + "</code>",
			groupThousands(snapshot.Verify.GateStages) + " of " +
				groupThousands(snapshot.Verify.Stages) + " full-mode verify stages run it"},
		{"Requirement source", "<code>rfc/short/*.md</code>",
			groupThousands(snapshot.Gate.GatedMust) + " gated MUST-level requirements"},
		{rfcEnrolmentLabel, "<code>rfc/short/*.md</code>, the <code>| Enrolment |</code> Meta row",
			groupThousands(snapshot.Gate.Enrolled) + " enrolled RFCs"},
		{rfcTestTagsLabel, "<code>internal/</code>, <code>pkg/</code>, <code>test/</code>",
			groupThousands(snapshot.Gate.TestTags) + " resolved tags"},
		{"Public ledger", "<code>rfc/short/*.md</code>, the <code>| Support |</code> Meta row",
			groupThousands(snapshot.Gaps.RFCs) + " RFCs with gaps, " + groupThousands(supported) +
				" Supported with Remaining"},
		{"Semantic audits", "<code>rfc/audit/*.json</code>",
			groupThousands(snapshot.Audit.Fresh) + " fresh, " +
				groupThousands(snapshot.Audit.Shifted) + " shifted, " +
				groupThousands(snapshot.Audit.Stale) + " stale, " +
				groupThousands(snapshot.Audit.Missing) + " missing"},
		{"Pre-commit verification", "<code>internal/le/verify/engine/stages.go</code>",
			snapshot.Verify.Command + ", " + groupThousands(snapshot.Verify.GateStages) +
				" of " + groupThousands(snapshot.Verify.Stages) + " full-mode stages"},
		{"Published artifacts",
			"<code>" + html.EscapeString(rfcComplianceSnapshot) + "</code>, <code>" +
				html.EscapeString(rfcLedgerFile) + "</code>",
			"the same answers this page renders, machine-readable"},
	}
}
