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

	"github.com/ze-software/ze/internal/core/textbuf"
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
	// rfcUnmappedBucket is where a gated requirement goes when it carries an
	// annotation this page has no bucket for. It is deliberately NOT one of
	// rfcSatisfaction, so nothing publishes it as a share: it leaves a hole in
	// the accounting that both notes state, which is what a page can be honest
	// about.
	//
	// Falling through to the polarity switch is what it did until 2026-09-02,
	// and that was worse than losing the requirement. A {single-polarity}
	// requirement whose kind lost its bucket carries one tag, so the switch put
	// it in "One polarity, unexcused" -- a red-tone card -- and the index
	// would have republished every {single-polarity} obligation of the corpus
	// as unexcused, with every test green (independent review). A visible hole
	// is a defect a reader can see. A
	// silent move into a worse-looking bucket is a false statement about the
	// requirements in it.
	rfcUnmappedBucket = "unmapped_annotation"
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
	// The two cards that carry a percentage and are NOT parts of the
	// partition. Named here because rfcPartitionNote names them and their
	// builders set them, and a label spelled twice is a sentence that stops
	// matching the card it describes.
	rfcProvenShareLabel = "Proven by test"
	rfcProofCardLabel   = "Proven by a recorded break"
	// The label the non-binding bucket carries. The partition row, the tone
	// legend and the bucket table each name it, and a label spelled three times
	// is a sentence that stops matching the card it describes.
	rfcNotApplyLabel = "Not applicable"
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
	Gate rfcGate `json:"gate"`
	// Share is the published proof share, exactly as internal/le/rfc answers
	// it. The page states it; it derives no share of its own.
	Share        rfcShare    `json:"share"`
	Satisfaction []rfcBucket `json:"satisfaction"`
	Gaps         rfcGaps     `json:"gaps"`
	Audit        rfcAudit    `json:"audit"`
	Verify       rfcVerify   `json:"verify"`
	// Unmapped counts the gated requirements whose annotation kind has no
	// bucket. It is zero in a healthy tree and the page states it when it is
	// not, because those requirements are in no published share.
	Unmapped int `json:"unmapped-annotations,omitempty"`
}

// rfcShare is the ONE answer to "how much of what Ze implements is proven by
// test", carried here in the shape rfc.ProvenShareOf returns it.
//
// Three surfaces state it and all three read that producer, so they cannot
// disagree. Until 2026-09-02 this page answered 58.1% over a denominator that
// dropped every {not-applicable} obligation, /quality/health/ answered 43.2%
// over every enrolled RFC, and the home page published two absolute counts with
// no denominator at all (owner directive, 2026-09-01).
//
// Percent is the producer's own rendering rather than a second formatting of
// Proven over Gated, because a page that re-rounds the ratio can print a
// different last digit from the producer that owns it.
type rfcShare struct {
	Proven         int    `json:"proven"`
	Gated          int    `json:"gated"`
	RFCs           int    `json:"rfcs"`
	Inspected      int    `json:"inspected"`
	GatedInspected int    `json:"gated-inspected"`
	Percent        string `json:"percent"`
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
	// so it is SCOPE rather than coverage, and the page says so in words: the
	// bucket's Source condition cell, the scale card and the bucket's own
	// share each name it.
	//
	// It is NOT a filter on any denominator. It was one until 2026-09-02, and
	// what it removed was the evidence: an obligation annotated away left the
	// tape, the key, the cards and the total, and every share around it rose
	// with nothing on the page to say why (owner decision, 2026-09-02).
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
	{Key: rfcNotApplyBucket, Label: rfcNotApplyLabel, Short: rfcNotApplyLabel,
		Condition: "{not-applicable} annotation", Binds: false},
}

// rfcAnnotationBucket answers the bucket one annotation kind satisfies, and
// whether this page knows the kind at all.
//
// It is the ONE mapping. The index counts buckets over the whole corpus and a
// stem page splits its own Annotated total by kind, and both read this, so the
// two cannot disagree about what an annotation means. It was a map beside a
// second switch in rfcLedgerCoverageOf until 2026-09-02, and the switch had no
// default: a kind added to rfc.AnnotationKinds and not given a bucket fell out
// of every counter while the gate went on counting it, so the shares summed to
// less than their whole with every test green (independent review).
//
// The second return is the refusal. A caller MUST NOT read the empty string as
// a bucket: an unknown kind is a mapping this page has not been taught, and it
// is counted as one rather than dropped (ai/rules/principles.md).
func rfcAnnotationBucket(kind string) (string, bool) {
	switch kind {
	case rfc.AnnotationNotApplicable:
		return rfcNotApplyBucket, true
	case rfc.AnnotationGap:
		return rfcGapBucket, true
	case rfc.AnnotationSinglePolarity:
		return rfcSingleBucket, true
	default:
		return "", false
	}
}

// rfcStatusOrder is the order the gap-disclosure table lists a public status
// in. A status this does not name follows, in name order.
var rfcStatusOrder = []string{"Partial", "Experimental", "Supported", "Not supported", "Unsupported"}

// rfcMissingRow is the status an RFC with a gap and no public row is counted
// under.
//
// It reads as a DECISION rather than as an omission, because that is what it
// now is. The public row is the summary's own `| Support |` Meta row, readSupport
// (`internal/le/rfc/meta.go`) refuses a Meta table that carries no such row, and
// a summary that writes `-` there declares that it renders no row. So a row is
// never simply forgotten, and the cell says which of the two a reader is looking
// at. It does NOT say the row is unguarded: whether a row may be DELETED from a
// summary that keeps its enrolment is a question for the gate, not for this
// page, and this page states no verdict on it.
const rfcMissingRow = "No public row declared"

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
	written := make(map[string]bool, len(ledger.Stems))
	for index := range ledger.Stems {
		route, err := writeRFCDetailPage(paths.Output, &ledger.Stems[index], links)
		if err != nil {
			return nil, err
		}
		written[ledger.Stems[index].Stem] = true
		routes = append(routes, route)
	}
	// After the writing, so the live set is what this run PUT there rather
	// than what it meant to.
	if err := removeRetiredRFCPages(paths.Output, written); err != nil {
		return nil, err
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
//
// The removal is keyed on the MARKER this family writes, never on a name that
// is merely absent from the live set. The output of a real build is the
// published checkout, so a name test deletes any directory under this prefix
// that another producer, or an author, put there (independent review,
// 2026-09-01).
func removeRetiredRFCPages(output string, live map[string]bool) error {
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
		page, err := os.ReadFile(filepath.Join(root, entry.Name(), pageIndexFile)) //nolint:gosec // a site build reads the artifact it was pointed at
		if err != nil {
			// No page of ours to retire. On a real build the output is the
			// published checkout, so a directory this producer never wrote is
			// somebody else's work and it is left alone
			// (ai/rules/never-destroy-work.md).
			continue
		}
		if !strings.Contains(string(page), rfcDetailMarker) {
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

	// The carriers argument is nil because no field of a ProvenShare reads
	// one: rfc.CoverageRows takes them to decide NightlyOnly, which the share
	// does not count. The home page and /quality/health/ pass nil for the same
	// reason, so the three surfaces cannot answer different numbers.
	share, err := rfc.ProvenShareOf(collected.Metas, collected.Requirements, collected.Tags, nil)
	if err != nil {
		return rfcCompliance{}, err
	}

	// TWO populations, and every figure below names which one it is over.
	//
	// inspected is what the GATE holds: every gated MUST of an enrolled
	// summary. It is the gate's own scale and the set the recorded audit
	// verdicts are judged over.
	//
	// implemented is the set the published SHARES are taken over: the gated
	// MUSTs of the RFCs Ze implements, which is the population
	// rfc.ProvenShareOf uses. Holding Ze to the obligations of a document its
	// own public row says it does not implement measures a decision rather than
	// a defect. rfc.Implements is the one definition of that set, read here
	// rather than restated (ai/rules/principles.md).
	var inspected, implemented []rfc.Requirement
	for _, requirement := range collected.Requirements {
		if !requirement.Gated() {
			continue
		}
		meta, held := collected.Metas[requirement.RFC]
		if !held || !meta.Enrolled() {
			continue
		}
		inspected = append(inspected, requirement)
		if rfc.Implements(meta) {
			implemented = append(implemented, requirement)
		}
	}
	snapshot := rfcCompliance{
		Gate: rfcGate{
			OK:         len(report.Violations) == 0,
			ErrorCount: len(report.Violations),
			GatedMust:  len(inspected),
			Enrolled:   len(collected.Enrolled),
			TestTags:   len(collected.Tags),
			Violations: report.Violations,
			Findings:   report.Findings,
		},
		Share: rfcShare{Proven: share.Proven, Gated: share.Gated, RFCs: share.RFCs,
			Inspected: share.Inspected, GatedInspected: share.GatedInspected,
			Percent: share.Percent()},
		Verify: rfcGateStages(),
	}
	snapshot.Gate.Message = rfcGateSummary(&report)
	snapshot.Satisfaction, snapshot.Gaps, snapshot.Unmapped = rfcBuckets(implemented,
		collected.Tags, input.Rows)
	snapshot.Audit = rfcAuditCounts(inspected, input.States, len(inspected))
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
//
// The third answer is the requirements whose annotation kind has no bucket.
// They are in NO published share, so the accounting is short by that many and
// both notes say so.
func rfcBuckets(gated []rfc.Requirement, tags []rfc.Tag,
	rows map[string]rfc.LedgerRow) ([]rfcBucket, rfcGaps, int) {
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
		if requirement.Annotation == nil {
			continue
		}
		if bucket, _ := rfcAnnotationBucket(requirement.Annotation.Kind); bucket != rfcGapBucket {
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
	return buckets, rfcGapsOf(counts[rfcGapBucket], gapCounts, gapOrder, rows),
		counts[rfcUnmappedBucket]
}

// rfcBucketOf answers the bucket one gated requirement falls in. An annotation
// decides on its own; otherwise the tagged polarities do.
//
// An annotation with no bucket decides too, and it decides on rfcUnmappedBucket
// rather than letting the polarities answer. A requirement the summary EXCUSED
// must never be counted as one nobody excused, and the polarity switch cannot
// tell the difference: it sees one tag and says "one polarity, unexcused".
func rfcBucketOf(requirement rfc.Requirement, polarities map[string]bool) string {
	if requirement.Annotation != nil {
		bucket, known := rfcAnnotationBucket(requirement.Annotation.Kind)
		if !known {
			return rfcUnmappedBucket
		}
		return bucket
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

// rfcPublicStatus answers what the public page says about one stem, which is
// what that stem's own `| Support |` Meta row declares.
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
	var body textbuf.Buffer
	body.Str("            <section aria-labeledby=\"rfc-compliance-title\" class=\"md-content reveal cat-observe\">\n")
	body.Str(pageHero("RFC Compliance Gate Report",
		"Source: <code>internal/le/rfc</code>, <code>rfc/short/*.md</code>, and "+
			"<code>rfc/audit/*.json</code>.",
		"Quality", ` id="rfc-compliance-title"`, heroClasses)).Byte('\n')
	body.Str(rfcComplianceStyle)
	body.Str(rfcCardGrid(snapshot, ledger))
	body.Str(rfcGateVerdictHTML(snapshot))

	body.Str("<section><h2>Requirement buckets</h2>\n").Str(rfcSatisfactionHTML(snapshot)).Str("\n</section>\n")
	body.Str("<section><h2>Gap disclosure</h2>\n").Str(rfcGapDisclosureHTML(snapshot.Gaps)).Str("\n</section>\n")
	body.Str("<section><h2>Exclusion disclosure</h2>\n").
		Str(rfcExclusionDisclosureHTML(ledger)).Str("\n</section>\n")
	body.Str("<section><h2>Top gap clusters</h2>\n").Str(rfcGapClusterHTML(snapshot.Gaps)).Str("\n</section>\n")
	body.Str("<section><h2>How this is checked</h2>\n").Str(rfcMechanismHTML(snapshot)).
		Str("\n</section>\n")
	body.Str("<section><h2>Check results</h2>\n").Str(rfcCheckHTML(snapshot.Gate)).Str("\n</section>\n")
	body.Str("<section><h2>Enrolled RFCs</h2>\n").Str(rfcEnrolledIndexHTML(ledger)).Str("\n</section>\n")
	body.Str("<section><h2>Summaries that are not enrolled</h2>\n").
		Str(rfcDeclinedIndexHTML(ledger)).Str("\n</section>\n")
	body.Str("            </section>\n")
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
	// Part is the numerator behind Value on a partition card, so the sum a
	// reader is told to make is one the page can make first.
	//
	// Value is a percentage and Count is a sentence, and rfcPartitionNote read
	// neither: it counted how many cards were MARKED as parts and said they add
	// to 100% whatever they added to (independent review, 2026-09-02).
	Part int
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
	var out textbuf.Buffer
	for _, group := range rfcCardGroups {
		held := rfcCardsIn(cards, group.Tone, group.Overall)
		if len(held) == 0 {
			continue
		}
		out.Str("<h3>").Str(html.EscapeString(group.Heading)).Str("</h3>\n<p>").
			Str(html.EscapeString(group.Lead)).Str("</p>\n")
		out.Str(`<div class="rfc-card-grid reveal">`).Byte('\n')
		for _, card := range held {
			out.Str(`<article class="rfc-card rfc-`).Str(card.Tone).Str(`"><span>`).
				Str(html.EscapeString(card.Label)).Str("</span><strong>").
				Str(html.EscapeString(card.Value)).Str("</strong>")
			if card.Count != "" {
				out.Str("<b>").Str(html.EscapeString(card.Count)).Str("</b>")
			}
			out.Str("<p>").Str(html.EscapeString(card.Note)).Str("</p></article>\n")
		}
		out.Str("</div>\n")
	}
	out.Str("<p>").Str(html.EscapeString(rfcPartitionNote(cards, whole))).Str("</p>\n")
	return out.String()
}

// rfcCardsMirror states the same cards, in the same four movements.
func rfcCardsMirror(cards []rfcCard, whole int) string {
	var out textbuf.Buffer
	for _, group := range rfcCardGroups {
		held := rfcCardsIn(cards, group.Tone, group.Overall)
		if len(held) == 0 {
			continue
		}
		out.Str("### ").Str(group.Heading).Str("\n\n").Str(group.Lead).Str("\n\n")
		out.Str("| Measure | Value | Count | What it means |\n|---|---:|---|---|\n")
		for _, card := range held {
			out.Str("| ").Str(card.Label).Str(" | ").Str(card.Value).Str(" | ").
				Str(rfc.TableCell(rfcOrDash(card.Count))).Str(" | ").Str(rfc.TableCell(card.Note)).Str(" |\n")
		}
		out.Byte('\n')
	}
	out.Str(rfcPartitionNote(cards, whole)).Byte('\n')
	return out.String()
}

// rfcOrDash answers a value, or the dash an empty cell reads as.
func rfcOrDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

// rfcPartitionNote says which cards add up, to what, and whether they do.
//
// It counted how many cards were MARKED as a part and then said they add to
// 100%, without reading one of their values: the sentence was true by
// assertion, and a partition that had lost a share published it unchanged
// (independent review, 2026-09-02). It now sums Part and states the shortfall
// where there is one.
//
// A reader who adds the shares this page shows must land somewhere they can
// name. Showing two parts of a four-part split and leaving 3.3% unexplained is
// an incomplete report, which is the defect the owner found on 2026-09-01. The
// note states the whole, and it states that the proof ratio is over a different
// population so nobody adds it in.
func rfcPartitionNote(cards []rfcCard, whole int) string {
	parts, sum := 0, 0
	for _, card := range cards {
		if !card.Partition {
			continue
		}
		parts++
		sum += card.Part
	}
	if parts == 0 || whole == 0 {
		return "No card above is a share of a population, so there is nothing to add up."
	}
	// The tail names the cards that are NOT parts, and it names only the ones
	// this grid actually carries: a stem page has no headline share, and a
	// sentence about a card a reader cannot see is a sentence about nothing.
	tail := ""
	if rfcGridHas(cards, rfcProvenShareLabel) {
		tail += " " + rfcProvenShareLabel + " is the first two of them added together, so it " +
			"is not a part of its own."
	}
	if rfcGridHas(cards, rfcProofCardLabel) {
		tail += " " + rfcProofCardLabel + " is a share of TAGGED UNITS, a different " +
			"population, so it is not one of them."
	}
	if sum != whole {
		return "The " + plural(parts, "share") + " marked as a part above account for " +
			groupThousands(sum) + " of the " + groupThousands(whole) +
			" gated MUSTs, so " + groupThousands(whole-sum) +
			" fall in none of them: they do NOT add to 100%." + tail
	}
	return "The " + plural(parts, "share") + " marked as a part above " +
		"are the whole of the " + groupThousands(whole) +
		" gated MUSTs: they add to 100%." + tail
}

// rfcGridHas answers whether a grid carries the card with one label.
func rfcGridHas(cards []rfcCard, label string) bool {
	for _, card := range cards {
		if card.Label == label {
			return true
		}
	}
	return false
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
	var rows textbuf.Buffer
	for _, card := range cards {
		rows.Str(rfcRowCells(html.EscapeString(card.Label),
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
	var out textbuf.Buffer
	out.Str(rfcToneRule).Str("\n\n")
	out.Str(rfcMirrorHead("Card", "Tone here", "Why that color"))
	for _, card := range cards {
		out.Str(rfcMirrorRow(card.Label, card.Tone, rfc.TableCell(card.Rule)))
	}
	return out.String()
}

// rfcCardGrid renders the four headline numbers and the gate's wiring.
func rfcCardGrid(snapshot *rfcCompliance, ledger rfcLedger) string {
	cards := rfcComplianceCards(snapshot, ledger)
	return rfcCardsHTML(cards, rfcBindingOf(snapshot).Gated) +
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

// rfcStanding groups the gated buckets into the ratios the cards publish.
//
// EVERY bucket appears in exactly one group, including the not-applicable one.
// That is what makes the ratios partition the denominator, so a reader who adds
// the shares lands on 100% rather than on 96.7% with nowhere to look for the
// rest -- which is what the owner found on 2026-09-01.
// TestTheRatioCardsPartitionTheirDenominator holds the property rather than
// trusting this table.
//
// The denominator is the GATED count. It was the gated count less the
// not-applicable obligations until 2026-09-02, and that subtraction existed
// only to take those obligations out of view: an obligation annotated away
// left the page, and the shares above it rose. The owner ruled against
// removing them, so they are a NAMED slice here instead, and the two green
// slices added together are the proof share this site publishes.
//
// Tone is the color a NON-ZERO value takes, and a card's color names what its
// measure MEANS rather than how well Ze scores on it: the number under the
// label already carries the performance, and a color that graded as well as
// labeled made a reader decode two scales at once (owner ruling, 2026-09-01).
// A group whose tone is red reads GREEN at zero, because none of that outcome
// is the good news; rfcStandingCards is where that one exception lives.
var rfcStanding = []struct {
	Label   string
	Keys    []string
	Tone    string
	Meaning string
	Rule    string
}{
	{Label: "Tested both ways", Keys: []string{rfcBothBucket}, Tone: rfcToneOK,
		Meaning: "a positive test proves Ze does what the requirement demands and a negative " +
			"one proves it refuses what the requirement forbids",
		Rule: "green at every value: a test pair is the outcome this gate exists to produce, " +
			"and the share under the label is what says how far Ze has got"},
	{Label: "One polarity plus reason", Keys: []string{rfcSingleBucket}, Tone: rfcToneOK,
		Meaning: "the requirement admits no counter-case, so one polarity plus a recorded " +
			"reason is the whole proof available for it",
		Rule: "green at every value: where no counter-case exists, one polarity IS the " +
			"complete answer, and a recorded reason is what the gate demands beside it"},
	{Label: "One polarity, unexcused", Keys: []string{rfcOneSideBucket}, Tone: rfcToneBad,
		Meaning: "one direction is tested, the other is neither tested nor excused, and " +
			"nothing states which",
		Rule: "green at zero, RED above it: half a proof with no reason for the other half"},
	{Label: "No test at all", Keys: []string{rfcGapBucket, rfcMissingBucket}, Tone: rfcToneBad,
		Meaning: "no test carries the requirement id, whether or not a gap states why",
		Rule: "green at zero, RED above it: a binding obligation nothing exercises is a claim " +
			"with nothing behind it, whether or not a reason is stated"},
	{Label: rfcNotApplyLabel, Keys: []string{rfcNotApplyBucket}, Tone: rfcToneNeutral,
		Meaning: "a {not-applicable} annotation says the obligation does not bind Ze, so no " +
			"test is owed for it. It stays in the denominator every share here is taken over",
		Rule: "no color: an obligation that never bound Ze is neither an achievement nor a " +
			"failure, and counting it either way would be a claim"},
}

// rfcBinding is the gated population of the RFCs Ze implements, and how it is
// answered.
//
// EVERY share on this page is taken over Gated, which the producer answered and
// no bucket here produced. There was a second population until 2026-09-02,
// `Gated - OutOfScope`, and the five shares were taken over that one: a
// `{not-applicable}` annotation moved its obligation out of the denominator,
// out of the tape and out of every card, and each share above it rose. The
// owner ruled against removing them from view, so not-applicable is a NAMED
// slice of the partition now and the subtraction is gone.
//
// OutOfScope survives because the scale card still states how many of the gated
// MUSTs that slice holds. It is a count the page shows, no longer a subtrahend.
type rfcBinding struct {
	Gated      int
	OutOfScope int
	// Obligations is the sum of the buckets whose obligation binds Ze. Nothing
	// on the page divides by it; TestTheBucketTableAccountsForEveryGatedRequirement
	// holds it against a count taken straight from the vocabulary, which is
	// what says rfcBindingOf read the snapshot correctly.
	Obligations int
	// Unmapped counts the gated requirements in no bucket at all, which is the
	// one way the buckets can fail to account for Gated.
	Unmapped int
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
	case rfcNotApplyBucket:
		return c.NotApplicable
	default:
		return 0
	}
}

// rfcStandingCards answers one card per rfcStanding group, over a population
// and a counter for it.
//
// One builder for the index and for every stem page, so the two cannot publish
// different partitions of the same idea (ai/rules/principles.md). The whole is
// the GATED count on both: the corpus page passes the share's own denominator
// and a stem page passes its own gated total, and neither takes anything out of
// it first.
//
// The tone is the group's, with ONE exception: a group that reads red above
// zero reads green AT zero, because no unexcused half-proof and no untested
// obligation is the outcome the gate exists to produce.
func rfcStandingCards(whole int, countOf func(string) int) []rfcCard {
	cards := make([]rfcCard, 0, len(rfcStanding))
	for _, group := range rfcStanding {
		part := 0
		for _, key := range group.Keys {
			part += countOf(key)
		}
		tone := group.Tone
		if tone == rfcToneBad {
			tone = rfcToneFor(part, rfcToneBad)
		}
		cards = append(cards, rfcCard{Label: group.Label,
			Value: rfcPercentText(part, whole),
			Count: groupThousands(part) + " of " + groupThousands(whole) + " gated MUSTs",
			Note:  group.Meaning, Tone: tone, Rule: group.Rule, Partition: true, Part: part})
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
	split := rfcBinding{Gated: snapshot.Share.Gated, Unmapped: snapshot.Unmapped}
	for _, bucket := range rfcSatisfaction {
		if bucket.Binds {
			split.Obligations += counted[bucket.Key]
			continue
		}
		split.OutOfScope += counted[bucket.Key]
	}
	return split
}

// rfcComplianceCards answers the index page's own headline numbers, each with
// the rule that chose its tone.
//
// SCALE first, then STANDING. The grid opens with the population the gate holds
// and the part of it that does not bind Ze, then the headline share, then the
// five shares that partition that same population, then the proof ratio over
// its own denominator. The
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
			Note: "MUST-level requirements the gate HOLDS, across the " +
				groupThousands(snapshot.Share.RFCs) + " RFCs Ze implements, out of " +
				groupThousands(snapshot.Share.GatedInspected) + " across the " +
				groupThousands(snapshot.Share.Inspected) + " RFCs inspected. A population, not " +
				"a result: the shares beside it are what says how Ze stands",
			Tone: rfcToneNeutral,
			Rule: "no color: a population is a scale, and a larger one is neither good news " +
				"nor bad. It is the accounting total"},
		{Label: "Out of scope", Value: groupThousands(split.OutOfScope), Overall: true,
			Count: "of " + groupThousands(split.Gated) + " gated MUSTs",
			Note: "a {not-applicable} annotation says the obligation does not bind Ze. Scope, " +
				"not coverage, and it is the Not applicable share below: it stays in the " +
				"denominator every share on this page is taken over",
			Tone: rfcToneNeutral,
			Rule: "no color: an obligation that never bound Ze is neither an achievement nor a " +
				"failure, and counting it either way would be a claim"},
		rfcProvenShareCard(snapshot.Share),
	}
	// The whole is the gate's own gated total less what the buckets put out of
	// scope, and NOT split.Obligations, which is the sum of the binding buckets
	// the parts are drawn from. Passing that sum made the partition note
	// compare a number with itself, the same tautology rfcAccountedNote carried
	// (independent review, 2026-09-01 and 2026-09-02). The two are equal today
	// and the note is what says so.
	cards = append(cards, rfcStandingCards(split.Gated, func(key string) int {
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

// rfcProvenShareCard answers the ONE published share: how much of what Ze
// implements is proven by a test bound to the requirement id.
//
// It states the producer's own percentage and both populations it is drawn
// from, because an absolute number alone counts obligations JUDGED and is read
// as obligations MET (owner directive, 2026-09-01). It is NOT marked as a part
// of the partition: it is two of those parts added together, so counting it
// again would put the same obligations in the sum twice.
//
// Its numerator IS those two buckets summed and its denominator IS theirs, so
// it is those two slices of the partition added together and stated as one
// number. TestTheHeadlineShareIsTheTwoProvenCardsSummed holds the numerator
// half of that, and the card's own note tells a reader the same thing.
func rfcProvenShareCard(share rfcShare) rfcCard {
	// Overall stays false: this card is a MEASURE rather than one of the
	// populations the shares are taken over, so it reads in the Positive
	// movement beside the other green cards.
	return rfcCard{Label: rfcProvenShareLabel, Value: share.Percent + "%",
		Count: groupThousands(share.Proven) + " of " + groupThousands(share.Gated) +
			" gated MUSTs across the " + groupThousands(share.RFCs) + " RFCs Ze implements",
		Note: "the share this site publishes everywhere: Tested both ways and One polarity " +
			"plus reason added together, two of the five shares that partition the same " +
			"denominator. That denominator keeps the {not-applicable} obligations, so " +
			"annotating a requirement away cannot raise it",
		Tone: rfcToneOK,
		Rule: "green at every value: a proven obligation is the outcome this gate exists to " +
			"produce, and the number under the label is what says how far Ze has got"}
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
	return rfcCard{Label: rfcProofCardLabel, Value: rfcPercentText(proven, units),
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
// table with the total that proves the accounting.
//
// The shares are over the GATED population, every bucket included. They were
// over `gated - not-applicable` until 2026-09-02, which took 642 annotated
// obligations out of the tape, out of the key and out of every denominator: a
// requirement annotated away left the picture, and each share around it rose.
// The owner ruled against removing them from view, so the not-applicable bucket
// is a segment of the tape like any other and its row sits with the rest.
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

	var out textbuf.Buffer
	out.Str(`<div class="rfc-tape" role="img" aria-label="How every gated MUST is answered">`).Byte('\n')
	for _, bucket := range rfcSatisfaction {
		if counted[bucket.Key] == 0 {
			continue
		}
		out.Str(`<span class="rfc-tape-`).Str(bucket.Key).Str(`" style="--w: `).
			Float(rfcPercent(counted[bucket.Key], split.Gated), 3).
			Str(`%"></span>`).Byte('\n')
	}
	out.Str("</div>\n<ul class=\"rfc-tape-key\">\n")
	for _, bucket := range rfcSatisfaction {
		if counted[bucket.Key] == 0 {
			continue
		}
		out.Str(`<li><span class="rfc-swatch rfc-tape-`).Str(bucket.Key).Str(`"></span> `).
			Str(html.EscapeString(bucket.Label)).Str(": <strong>").Str(groupThousands(counted[bucket.Key])).
			Str("</strong> (").Str(rfcPercentText(counted[bucket.Key], split.Gated)).Str(")</li>\n")
	}
	out.Str("</ul>\n<p>").Str(html.EscapeString(rfcScopeNote(split))).Str("</p>\n")
	out.Str(rfcTableHTML(rfcHeadCells("Bucket", "Count", "Share of gated",
		"Source condition"), rfcSatisfactionRows(counted, split)))
	return out.String()
}

// rfcSatisfactionRows answers one row per bucket and the accounting total.
//
// ONE pass over the vocabulary, in its own order. There were two passes and two
// totals until 2026-09-02, because the not-applicable bucket was published
// below a subtotal it had been subtracted out of. It is a row like the others
// now, and its Source condition still says the obligation does not bind Ze.
func rfcSatisfactionRows(counted map[string]int, split rfcBinding) string {
	var rows textbuf.Buffer
	accounted := 0
	for _, bucket := range rfcSatisfaction {
		accounted += counted[bucket.Key]
		condition := "<code>" + html.EscapeString(bucket.Condition) + "</code>"
		if !bucket.Binds {
			condition += ": " + html.EscapeString(rfcScopeCell)
		}
		rows.Str(rfcRowCells(html.EscapeString(bucket.Label),
			"<strong>"+groupThousands(counted[bucket.Key])+"</strong>",
			rfcPercentText(counted[bucket.Key], split.Gated), condition))
	}
	rows.Str(`<tr class="rfc-total"><td><strong>`).
		Str(html.EscapeString(rfcGatedLabel)).Str(`</strong></td><td><strong>`).
		Str(groupThousands(split.Gated)).Str("</strong></td><td>").
		Str(rfcPercentText(accounted, split.Gated)).Str("</td><td>").
		Str(html.EscapeString(rfcAccountedNote(split, accounted))).Str("</td></tr>\n")
	return rows.String()
}

// The label and the sentence the accounting rows carry.
const (
	rfcGatedLabel = "Gated MUST-level requirements"
	rfcScopeCell  = "the obligation does not bind Ze, so it is scope rather than coverage"
)

// rfcAccountedNote says whether the buckets account for the gated population.
//
// A mismatch is STATED rather than hidden. The buckets are meant to partition
// it, so a difference is a defect in the bucketing and a page that printed only
// the sum would let it pass.
func rfcAccountedNote(split rfcBinding, accounted int) string {
	obligations := split.Gated
	if accounted == obligations {
		return "every gated MUST falls in exactly one bucket above, the " +
			groupThousands(split.OutOfScope) + " that do not bind Ze included. This total is " +
			"the denominator of every share above it"
	}
	note := "the buckets account for " + groupThousands(accounted) + " of " +
		groupThousands(obligations) + ", so " + groupThousands(obligations-accounted) +
		" fall in none: the bucketing is incomplete"
	if split.Unmapped == 0 {
		return note
	}
	return note + ". " + groupThousands(split.Unmapped) +
		" of them carry an annotation kind this page has no bucket for, so they are " +
		"counted apart rather than moved into a bucket that would misdescribe them"
}

// rfcScopeNote says what the bar counts, so a reader is never shown a
// proportion whose population they cannot name.
//
// It used to say what the bar LEAVES OUT, because the not-applicable
// obligations were not in it. They are a segment of it now.
func rfcScopeNote(split rfcBinding) string {
	if split.OutOfScope == 0 {
		return "The bar is every gated MUST-level requirement: none of them is out of scope."
	}
	return "The bar is every one of the " + groupThousands(split.Gated) +
		" gated MUST-level requirements. " + groupThousands(split.OutOfScope) +
		" of them are {not-applicable}: they do not bind Ze, and they are a named segment of " +
		"the bar rather than an omission from it."
}

// rfcGapDisclosureHTML renders what the public page says about the RFCs
// declaring a gap, and the Supported rows that still disclose one.
func rfcGapDisclosureHTML(gaps rfcGaps) string {
	var rows textbuf.Buffer
	for _, row := range gaps.StatusCounts {
		rows.Str(rfcRowCells(html.EscapeString(row.Status),
			"<strong>"+groupThousands(row.Count)+"</strong>"))
	}
	var out textbuf.Buffer
	out.Str(rfcTableHTML(rfcHeadCells("Public status for RFCs with gaps", "RFCs"),
		rows.String()))
	if len(gaps.SupportedWithRemaining) == 0 {
		return out.String()
	}
	out.Byte('\n').Str(`<div class="rfc-note-box">`).
		Str("\n<h3>Supported rows that still disclose a gap</h3>\n<ul>\n")
	for _, row := range gaps.SupportedWithRemaining {
		out.Str("<li><strong>").Str(html.EscapeString(row.RFC)).Str("</strong>: ").
			Str(html.EscapeString(row.Remaining)).Str("</li>\n")
	}
	out.Str("</ul></div>")
	return out.String()
}

// rfcGapClusterHTML renders the RFCs carrying the most declared gaps.
func rfcGapClusterHTML(gaps rfcGaps) string {
	if len(gaps.TopRFCs) == 0 {
		return "<p>No RFC declares a gap.</p>"
	}
	var rows textbuf.Buffer
	for _, row := range gaps.TopRFCs {
		rows.Str(rfcRowCells("<code>"+html.EscapeString(row.RFC)+"</code>",
			"<strong>"+groupThousands(row.Count)+"</strong>",
			html.EscapeString(row.Status)))
	}
	return rfcTableHTML(rfcHeadCells("RFC", rfcDeclaredGapsLabel, "Public status"),
		rows.String())
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
	var out textbuf.Buffer
	if len(gate.Findings) == 0 {
		out.Str("<p>No open issue. Every enrolled MUST-level requirement carries both test ").
			Str("polarities or an annotation saying why not.</p>")
		return out.String()
	}
	shown := gate.Findings
	if len(shown) > rfcIssuesShown {
		shown = shown[:rfcIssuesShown]
	}
	var rows textbuf.Buffer
	for index := range shown {
		finding := &shown[index]
		rows.Str(rfcRowCells(
			html.EscapeString(rfcFindingRFC(finding)),
			rfcFindingRequirementHTML(finding),
			html.EscapeString(rfcOrDash(finding.Level)),
			html.EscapeString(rfcFindingIssue(finding)),
			html.EscapeString(rfcOrDash(finding.Text))))
	}
	out.Str(rfcTableHTML(rfcHeadCells("RFC", "Requirement", "Level", "What is wrong",
		"The requirement"), rows.String()))
	if left := len(gate.Findings) - len(shown); left > 0 {
		out.Str("\n<p>").Str(html.EscapeString(rfcFindingsLeftOut(left))).Str("</p>")
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
	var out textbuf.Buffer
	out.Str(rfcMirrorHead("RFC", "Requirement", "Level", "What is wrong",
		"The requirement"))
	for index := range shown {
		finding := &shown[index]
		out.Str(rfcMirrorRow(rfcFindingRFC(finding),
			rfcFindingRequirementMirror(finding), rfcOrDash(finding.Level),
			rfc.TableCell(rfcFindingIssue(finding)), rfc.TableCell(rfcOrDash(finding.Text))))
	}
	if left := len(gate.Findings) - len(shown); left > 0 {
		out.Byte('\n').Str(rfcFindingsLeftOut(left)).Byte('\n')
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

	var mirror textbuf.Buffer
	mirror.Str("# RFC Compliance Gate Report\n\n")
	mirror.Str("Source: `internal/le/rfc`, `rfc/short/*.md`, and ").
		Str("`rfc/audit/*.json`.\n\n")
	mirror.Str("## Gate verdict\n\n").Str(rfcGateWord(snapshot.Gate.OK)).Str(". ").
		Str(rfcGateVerdictText(snapshot)).Str(" Reproduce it with `").Str(snapshot.Verify.Command).
		Str("`. The gate's own line reads `").Str(snapshot.Gate.Message).Str("`.\n\n")
	cards := rfcComplianceCards(snapshot, ledger)
	mirror.Str(rfcCardsMirror(cards, rfcBindingOf(snapshot).Gated)).Byte('\n').
		Str(rfcToneLegendMirror(cards)).Byte('\n')
	mirror.Str("| Metric | Value |\n|---|---:|\n")
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
		mirror.Str("| ").Str(row.Label).Str(" | ").Str(groupThousands(row.Value)).Str(" |\n")
	}

	split := rfcBindingOf(snapshot)
	mirror.Str("\n## Requirement buckets\n\n")
	mirror.Str(rfcScopeNote(split)).Str("\n\n")
	mirror.Str("| Bucket | Count | Share of gated | Source condition |\n|---|---:|---:|---|\n")
	accounted := 0
	for _, bucket := range rfcSatisfaction {
		accounted += counted[bucket.Key]
		condition := "`" + bucket.Condition + "`"
		if !bucket.Binds {
			condition += ": " + rfc.TableCell(rfcScopeCell)
		}
		mirror.Str("| ").Str(bucket.Label).Str(" | ").Str(groupThousands(counted[bucket.Key])).
			Str(" | ").Str(rfcPercentText(counted[bucket.Key], split.Gated)).Str(" | ").Str(condition).Str(" |\n")
	}
	mirror.Str("| **").Str(rfcGatedLabel).Str("** | **").Str(groupThousands(split.Gated)).
		Str("** | ").Str(rfcPercentText(accounted, split.Gated)).Str(" | ").
		Str(rfc.TableCell(rfcAccountedNote(split, accounted))).Str(" |\n")

	mirror.Str("\n## Gap disclosure\n\n")
	mirror.Str("| Public status for RFCs with gaps | RFCs |\n|---|---:|\n")
	for _, row := range snapshot.Gaps.StatusCounts {
		mirror.Str("| ").Str(row.Status).Str(" | ").Str(groupThousands(row.Count)).Str(" |\n")
	}
	if len(snapshot.Gaps.SupportedWithRemaining) > 0 {
		mirror.Str("\n### Supported rows that still disclose a gap\n\n")
		for _, row := range snapshot.Gaps.SupportedWithRemaining {
			mirror.Str("- **").Str(row.RFC).Str(":** ").Str(row.Remaining).Byte('\n')
		}
	}

	mirror.Str("\n## Exclusion disclosure\n\n")
	mirror.Str(rfcExclusionDisclosureMirror(ledger))

	mirror.Str("\n## Top gap clusters\n\n")
	mirror.Str("| RFC | Declared gaps | Public status |\n|---|---:|---|\n")
	for _, row := range snapshot.Gaps.TopRFCs {
		mirror.Str("| `").Str(row.RFC).Str("` | ").Str(groupThousands(row.Count)).Str(" | ").Str(row.Status).Str(" |\n")
	}

	mirror.Str("\n## How this is checked\n\n")
	mirror.Str(rfcMechanismMirror(snapshot))

	mirror.Str("\n## Check results\n\n")
	mirror.Str(rfcCheckMirror(snapshot.Gate))
	mirror.Str(rfcIndexMirror(ledger))
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
	var out textbuf.Buffer
	for index, digit := range digits {
		if index > 0 && (len(digits)-index)%3 == 0 {
			out.Byte(',')
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
.rfc-table td:first-child { font-weight: 400; }
.rfc-table .rfc-span > td { padding-top: .9rem; }
.rfc-subject-id { white-space: nowrap; vertical-align: top; }
.rfc-subject { display: block; max-width: 52rem; }
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
	var body textbuf.Buffer
	for index := range rows {
		entry := rows[index]
		body.Str(rfcRowCells(
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
// and the reason its own `## Meta` table declares.
//
// The legend above the table says what each kind MEANS, for the same reason the
// exclusion section carries one: `source-restricted` and `out-of-scope` are
// this project's words and a reader outside it cannot act on either.
func rfcDeclinedIndexHTML(ledger rfcLedger) string {
	rows := rfcIndexRows(ledger, false)
	if len(rows) == 0 {
		return "<p>Every summary is enrolled.</p>"
	}
	var body textbuf.Buffer
	for index := range rows {
		entry := rows[index]
		body.Str(rfcRowCells(
			"<a href=\""+html.EscapeString(rfcStemHref(entry.Stem))+"\"><code>"+
				html.EscapeString(entry.Display)+"</code></a> "+html.EscapeString(entry.Title),
			html.EscapeString(rfcIndexDispositionKind(entry)),
			html.EscapeString(rfcIndexDispositionReason(entry))))
	}
	return rfcDispositionLegendHTML(rows) +
		rfcTableHTML(rfcHeadCells("RFC", "Disposition", "Reason"), body.String())
}

// rfcDispositionsUsed answers the kinds this index actually shows, in the
// vocabulary's own order, so a kind nothing uses gets no line.
func rfcDispositionsUsed(rows []*rfcLedgerStem) []string {
	held := map[string]bool{}
	for _, entry := range rows {
		held[entry.Disposition.Kind] = true
	}
	var used []string
	for _, kind := range rfc.DispositionKinds() {
		if held[kind] {
			used = append(used, kind)
		}
	}
	for _, entry := range rows {
		if _, known := rfc.DispositionKindMeaning(entry.Disposition.Kind); !known {
			used = append(used, entry.Disposition.Kind)
			break
		}
	}
	return used
}

// rfcDispositionLegendHTML says what each kind on the table below means.
func rfcDispositionLegendHTML(rows []*rfcLedgerStem) string {
	var out textbuf.Buffer
	out.Str("<ul>\n")
	for _, kind := range rfcDispositionsUsed(rows) {
		out.Str("<li><code>").Str(html.EscapeString(kind)).Str("</code>: ").
			Str(html.EscapeString(rfcDispositionMeaning(kind))).Str("</li>\n")
	}
	out.Str("</ul>\n")
	return out.String()
}

// rfcDispositionLegendMirror states the same legend.
func rfcDispositionLegendMirror(rows []*rfcLedgerStem) string {
	var out textbuf.Buffer
	for _, kind := range rfcDispositionsUsed(rows) {
		out.Str("- `").Str(kind).Str("`: ").Str(rfcDispositionMeaning(kind)).Byte('\n')
	}
	out.Byte('\n')
	return out.String()
}

// rfcIndexMirror states both link tables in the Markdown sibling.
func rfcIndexMirror(ledger rfcLedger) string {
	var out textbuf.Buffer
	out.Str("\n## Enrolled RFCs\n\n")
	enrolled := rfcIndexRows(ledger, true)
	if len(enrolled) == 0 {
		out.Str("No summary is enrolled.\n")
	} else {
		out.Str("| RFC | Public status | Gated MUSTs | Declared gaps | ").
			Str("Gated with no test |\n|---|---|---:|---:|---:|\n")
		for index := range enrolled {
			entry := enrolled[index]
			out.Str("| [`").Str(entry.Display).Str("`](").Str(rfcStemHref(entry.Stem)).
				Str(pageMirrorFile).Str(") ").Str(rfc.TableCell(entry.Title)).Str(" | ").
				Str(rfc.TableCell(rfcIndexStatus(entry))).Str(" | ").
				Int(int64(entry.Coverage.Gated)).Str(" | ").
				Int(int64(entry.Coverage.Gaps)).Str(" | ").
				Int(int64(entry.Coverage.Missing)).Str(" |\n")
		}
	}
	out.Str("\n## Summaries that are not enrolled\n\n")
	declined := rfcIndexRows(ledger, false)
	if len(declined) == 0 {
		out.Str("Every summary is enrolled.\n")
		return out.String()
	}
	out.Str(rfcDispositionLegendMirror(declined))
	out.Str("| RFC | Disposition | Reason |\n|---|---|---|\n")
	for index := range declined {
		entry := declined[index]
		out.Str("| [`").Str(entry.Display).Str("`](").Str(rfcStemHref(entry.Stem)).
			Str(pageMirrorFile).Str(") ").Str(rfc.TableCell(entry.Title)).Str(" | ").
			Str(rfcIndexDispositionKind(entry)).Str(" | ").
			Str(rfc.TableCell(rfcIndexDispositionReason(entry))).Str(" |\n")
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
	var out textbuf.Buffer
	out.Str("<p>").Str(html.EscapeString(rfcExclusionIntro(exclusions))).Str("</p>\n")
	out.Str("<p>").Str(html.EscapeString(rfcExclusionCoverage(exclusions))).Str("</p>\n")
	if exclusions.Sites == 0 {
		out.Str("<p>").Str(html.EscapeString("No sign-off has declined a sentence.")).Str("</p>")
		return out.String()
	}
	var rows textbuf.Buffer
	for _, kind := range exclusions.Kinds {
		rows.Str(rfcRowCells("<code>"+html.EscapeString(kind.Kind)+"</code>",
			html.EscapeString(rfcExclusionGroupWord(kind.Group)),
			"<strong>"+groupThousands(kind.Sites)+"</strong>",
			strconv.Itoa(len(kind.Stems)),
			html.EscapeString(kind.Meaning)))
	}
	rows.Str(`<tr class="rfc-total"><td><strong>`).
		Str(html.EscapeString(rfcExcludedLabel)).Str(`</strong></td><td>-</td><td><strong>`).
		Str(groupThousands(exclusions.Sites)).Str("</strong></td><td>-</td><td>").
		Str(html.EscapeString(rfcExclusionTotalNote(exclusions))).Str("</td></tr>\n")
	out.Str(rfcTableHTML(rfcHeadCells("Excluded kind", "Means", "Sites", "Summaries",
		"What it means"), rows.String()))
	for _, kind := range exclusions.Kinds {
		if len(kind.Stems) == 0 || kind.Group == rfc.ExclusionDebt {
			continue
		}
		out.Str("\n<p class=\"rfc-id-list\"><strong>").Str(html.EscapeString(kind.Kind)).
			Str(" (").Str(groupThousands(kind.Sites)).Str("):</strong> ").Str(rfcStemLinksHTML(kind.Stems))
		if kind.Suspect {
			out.Str("<br />").Str(html.EscapeString(rfcExclusionCaution))
		}
		out.Str("</p>")
	}
	out.Str("\n<h3>").Str(html.EscapeString(rfcDebtHeading)).Str("</h3>\n<p>").
		Str(html.EscapeString(rfcDebtLead(exclusions))).Str("</p>\n")
	if len(exclusions.Relocated) == 0 {
		out.Str("<p>").Str(html.EscapeString("No sign-off has relocated an obligation to a " +
			"spec.")).Str("</p>")
		return out.String()
	}
	var debt textbuf.Buffer
	for index := range exclusions.Relocated {
		row := &exclusions.Relocated[index]
		debt.Str(rfcRowCells(
			`<a href="`+html.EscapeString(rfcStemHref(row.Stem))+`"><code>`+
				html.EscapeString(rfcDisplayName(row.Stem))+"</code></a>",
			"<code>"+html.EscapeString(rfcOrUnstated(row.ID))+"</code>",
			html.EscapeString(rfcOrUnstated(row.Quote)),
			"<code>"+html.EscapeString(rfcOrUnstated(row.Spec))+"</code>"))
	}
	out.Str(rfcTableHTML(rfcHeadCells("RFC", "Reserved id", "The obligation",
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
	var out textbuf.Buffer
	out.Str(rfcExclusionIntro(exclusions)).Str("\n\n")
	out.Str(rfcExclusionCoverage(exclusions)).Str("\n\n")
	if exclusions.Sites == 0 {
		return out.String() + "No sign-off has declined a sentence.\n"
	}
	out.Str(rfcMirrorHead("Excluded kind", "Means", "Sites", "Summaries",
		"What it means"))
	for _, kind := range exclusions.Kinds {
		out.Str(rfcMirrorRow("`"+kind.Kind+"`", rfcExclusionGroupWord(kind.Group),
			groupThousands(kind.Sites), strconv.Itoa(len(kind.Stems)),
			rfc.TableCell(kind.Meaning)))
	}
	out.Str(rfcMirrorRow("**"+rfcExcludedLabel+"**", "-",
		"**"+groupThousands(exclusions.Sites)+"**", "-",
		rfc.TableCell(rfcExclusionTotalNote(exclusions))))
	for _, kind := range exclusions.Kinds {
		if len(kind.Stems) == 0 || kind.Group == rfc.ExclusionDebt {
			continue
		}
		out.Str("\n**").Str(kind.Kind).Str(" (").Str(groupThousands(kind.Sites)).Str("):** ").
			Str(rfcStemLinksMirror(kind.Stems)).Byte('\n')
		if kind.Suspect {
			out.Byte('\n').Str(rfcExclusionCaution).Byte('\n')
		}
	}
	out.Str("\n### ").Str(rfcDebtHeading).Str("\n\n").Str(rfcDebtLead(exclusions)).Str("\n\n")
	if len(exclusions.Relocated) == 0 {
		return out.String() + "No sign-off has relocated an obligation to a spec.\n"
	}
	out.Str(rfcMirrorHead("RFC", "Reserved id", "The obligation",
		"The spec that owes it"))
	for index := range exclusions.Relocated {
		row := &exclusions.Relocated[index]
		out.Str(rfcMirrorRow(
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
	var rows textbuf.Buffer
	for _, row := range rfcMechanismRows(snapshot) {
		rows.Str(rfcRowCells(html.EscapeString(row[0]), row[1],
			html.EscapeString(row[2])))
	}
	return "<p>" + html.EscapeString(rfcMechanismLead(snapshot)) + "</p>\n" +
		rfcTableHTML(rfcHeadCells("Input", "Producer", "What it answered here"), rows.String())
}

// rfcMechanismMirror states the same mechanism.
func rfcMechanismMirror(snapshot *rfcCompliance) string {
	var out textbuf.Buffer
	out.Str(rfcMechanismLead(snapshot)).Str("\n\n")
	out.Str(rfcMirrorHead("Input", "Producer", "What it answered here"))
	for _, row := range rfcMechanismRows(snapshot) {
		out.Str(rfcMirrorRow(row[0], rfc.TableCell(rfcPlain(row[1])),
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
