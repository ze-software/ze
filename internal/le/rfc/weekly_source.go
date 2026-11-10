// Design: docs/contributing/rfc-conformance-gates.md -- a range's movement, read by `le weekly report`
// Related: tree.go -- the rows each endpoint's own le answers through `rfc tree | json`
// Related: discriminate.go -- discriminationStatus, what `rfc discriminate stem <stem> | json` answers
// Related: discriminate_observe.go -- interopScenarioOf, which names a checker's scenario
//
// weekly_source.go answers the three RFC sections of `le weekly report`: the
// MUST figure and the requirements proven or lost, the interop scenarios that
// gained a proving record, and the documents signed off.
//
// Every figure at an endpoint comes from THAT endpoint's own le, compiled from
// its export and run as a subprocess on it. Today's corpus readers refuse a
// past tree whose summary format changed since (spec-weekly-event-report, A-3),
// and a copy of an older reader would be a second declaration of what that
// revision's ledger published. The endpoint's le is the producer its own
// ledger ran, so its answer is the published figure by construction (R-1).
// Only the interop mapping runs today's code, interopScenarioOf over the `to`
// export, because it reads checker source and never a summary.

package rfc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/repo/export"
	"github.com/ze-software/ze/internal/le/weekly"
)

// The three sections this package answers in `le weekly report`.
const (
	weeklyProvenSection     = "rfc-proven"
	weeklyInteropSection    = "rfc-interop"
	weeklyExtractionSection = "rfc-extraction"
)

// weeklyLeBuildNameKey is the launcher's build-name key, declared by
// envLeBuildName in cmd/ze/le_build_name.go, a main package nothing can
// import. The report removes it from the child's environment: an endpoint's le
// is not the build `./le --name` asked for, and would refuse to answer.
const weeklyLeBuildNameKey = "ze.le.build.name"

// endpointLe runs one endpoint's own le with args (the words after the
// program name, without the pipe chain) and answers what its `| json`
// rendering printed.
type endpointLe func(ctx context.Context, args ...string) ([]byte, error)

// endpointLeOf readies the le of the endpoint exported at tree, writing what
// it builds under workDir; keyword is `from` or `to`, for the error. A test
// replaces it with an in-process answer over a fixture tree; production
// compiles that tree's own le (builtEndpointLe).
var endpointLeOf = builtEndpointLe

// builtEndpointLe compiles the endpoint's le from its export with
// export.BuildLe, into a directory of its own under workDir, so the binary is
// removed with the run. The binary is named `le`, the name that selects the le
// personality of cmd/ze.
func builtEndpointLe(ctx context.Context, keyword, tree, workDir string) (endpointLe, error) {
	dir, err := os.MkdirTemp(workDir, "rfc-le-"+keyword+"-")
	if err != nil {
		return nil, fmt.Errorf("create the directory for the %s endpoint's le under %s: %w", keyword, workDir, err)
	}
	binary := filepath.Join(dir, "le")
	if err := export.BuildLe(ctx, tree, binary); err != nil {
		return nil, fmt.Errorf("build the %s endpoint's own le from its export %s: %w", keyword, tree, err)
	}
	return func(ctx context.Context, args ...string) ([]byte, error) {
		return runEndpointLe(ctx, binary, tree, args)
	}, nil
}

// runEndpointLe runs binary on the export at tree and answers its stdout.
//
// The pipe bar is its own argv word, as le's dispatcher splits it
// (leroot.splitChain). The child judges the export alone: the repository root
// is named by ZE_REPO_ROOT, every other spelling of that key and the
// launcher's build name are removed, and the working directory is the export.
// The child may write its own derived files (the RFC ledger renders) into
// that export, as it would in a checkout; no source reads them.
func runEndpointLe(ctx context.Context, binary, tree string, args []string) ([]byte, error) {
	argv := append(slices.Clone(args), "|", "json")
	child := exec.CommandContext(ctx, binary, argv...) //nolint:gosec // the le this run built from the endpoint's export, at a path it chose
	child.Dir = tree
	child.Env = append(env.Without(os.Environ(), lepath.RootKey, weeklyLeBuildNameKey), "ZE_REPO_ROOT="+tree)

	var stdout, stderr bytes.Buffer
	child.Stdout = &stdout
	child.Stderr = &stderr
	if err := child.Run(); err != nil {
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str("le ").Join(argv, " ").Str(" on ").Str(tree).Str(": ").Err(err).
			Str(": ").Str(strings.TrimSpace(stderr.String())).String())
	}
	return stdout.Bytes(), nil
}

// weeklyRow is one document as an endpoint's `rfc tree | json` row states it.
type weeklyRow struct {
	summary         bool
	walked          bool
	mandatoryProven int
	mandatoryTotal  int
	optionalProven  int
	optionalTotal   int
}

// weeklyEndpoint is what one endpoint's own le answered.
type weeklyEndpoint struct {
	// rows is every tree row, by stem.
	rows map[string]weeklyRow
	// proving is the covers a verified proving record carries, over the
	// compared stems only, each with its record.
	proving map[Cover]*DiscriminationRecord
}

// weeklyRange is both endpoints, and the stems compared record by record.
type weeklyRange struct {
	from, to weeklyEndpoint
	// compared is the stems whose records were read at both endpoints: those
	// whose proven counts differ, or whose record files changed in the range.
	compared []string
}

// weeklyRanges holds the last range answered, so the three sections of one
// report share one build of each endpoint's le: each build compiles a whole
// cmd/ze. One entry, replaced when another range is asked, so a long-lived
// process holds no more than one range. The error is kept too: a build that
// failed fails all three sections with its reason, without building again.
// Safe for concurrent use; a second caller waits for the first build.
var weeklyRanges struct {
	sync.Mutex
	key    string
	answer *weeklyRange
	err    error
}

// weeklyRangeFor answers the range of events, built once per run.
func weeklyRangeFor(ctx context.Context, events weekly.EventRange) (*weeklyRange, error) {
	key := events.FromDir + "\x00" + events.ToDir
	weeklyRanges.Lock()
	defer weeklyRanges.Unlock()

	if weeklyRanges.key == key {
		return weeklyRanges.answer, weeklyRanges.err
	}
	answer, err := weeklyRangeOf(ctx, events)
	weeklyRanges.key, weeklyRanges.answer, weeklyRanges.err = key, answer, err
	return answer, err
}

// weeklyRangeOf builds and asks each endpoint's le, `from` then `to`, one
// after the other: each build is a CPU-intensive job.
func weeklyRangeOf(ctx context.Context, events weekly.EventRange) (*weeklyRange, error) {
	fromLe, err := endpointLeOf(ctx, "from", events.FromDir, events.WorkDir)
	if err != nil {
		return nil, err
	}
	fromRows, err := weeklyTreeRows(ctx, fromLe, "from")
	if err != nil {
		return nil, err
	}
	toLe, err := endpointLeOf(ctx, "to", events.ToDir, events.WorkDir)
	if err != nil {
		return nil, err
	}
	toRows, err := weeklyTreeRows(ctx, toLe, "to")
	if err != nil {
		return nil, err
	}
	changed, err := changedRecordStems(ctx, events)
	if err != nil {
		return nil, err
	}

	out := &weeklyRange{from: weeklyEndpoint{rows: fromRows}, to: weeklyEndpoint{rows: toRows},
		compared: comparedStems(fromRows, toRows, changed)}
	out.from.proving, err = weeklyProving(ctx, fromLe, "from", fromRows, out.compared)
	if err != nil {
		return nil, err
	}
	out.to.proving, err = weeklyProving(ctx, toLe, "to", toRows, out.compared)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// weeklyTreeJSON is the part of `rfc tree | json` the report reads. Every
// field is a pointer, so a key the endpoint's le does not publish is refused
// by name rather than read as zero.
type weeklyTreeJSON struct {
	Rows *[]weeklyTreeRowJSON `json:"rows"`
}

type weeklyTreeRowJSON struct {
	Stem            *string `json:"stem"`
	Summary         *bool   `json:"summary"`
	Walked          *bool   `json:"walked"`
	MandatoryProven *int    `json:"mandatory-proven"`
	MandatoryTotal  *int    `json:"mandatory-total"`
	OptionalProven  *int    `json:"optional-proven"`
	OptionalTotal   *int    `json:"optional-total"`
}

// weeklyTreeRows asks one endpoint's le for its tree and answers its rows.
func weeklyTreeRows(ctx context.Context, le endpointLe, keyword string) (map[string]weeklyRow, error) {
	raw, err := le(ctx, "rfc", "tree")
	if err != nil {
		return nil, err
	}
	var decoded weeklyTreeJSON
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("the %s endpoint's `le rfc tree | json` is not the JSON the report reads: %w", keyword, err)
	}
	if decoded.Rows == nil {
		return nil, weeklyShapeErr(keyword, "rfc tree", "rows", "the answer")
	}
	rows := make(map[string]weeklyRow, len(*decoded.Rows))
	for index := range *decoded.Rows {
		row, stem, err := weeklyRowOf(&(*decoded.Rows)[index], keyword)
		if err != nil {
			return nil, err
		}
		if _, held := rows[stem]; held {
			var tb textbuf.Buffer
			return nil, errors.New(tb.Str("the ").Str(keyword).Str(" endpoint's `le rfc tree | json` answers ").
				Str(stem).Str(" twice, so its figure cannot be read off one row").String())
		}
		rows[stem] = row
	}
	return rows, nil
}

// weeklyRowOf checks every key the report reads is present, then answers the
// row and its stem.
func weeklyRowOf(row *weeklyTreeRowJSON, keyword string) (weeklyRow, string, error) {
	if row.Stem == nil {
		return weeklyRow{}, "", weeklyShapeErr(keyword, "rfc tree", "stem", "a row")
	}
	where := "the row " + *row.Stem
	for _, required := range []struct {
		key     string
		missing bool
	}{
		{"summary", row.Summary == nil},
		{"walked", row.Walked == nil},
		{"mandatory-proven", row.MandatoryProven == nil},
		{"mandatory-total", row.MandatoryTotal == nil},
		{"optional-proven", row.OptionalProven == nil},
		{"optional-total", row.OptionalTotal == nil},
	} {
		if required.missing {
			return weeklyRow{}, "", weeklyShapeErr(keyword, "rfc tree", required.key, where)
		}
	}
	return weeklyRow{summary: *row.Summary, walked: *row.Walked,
		mandatoryProven: *row.MandatoryProven, mandatoryTotal: *row.MandatoryTotal,
		optionalProven: *row.OptionalProven, optionalTotal: *row.OptionalTotal}, *row.Stem, nil
}

// weeklyShapeErr names a key an endpoint's answer does not carry.
func weeklyShapeErr(keyword, command, key, where string) error {
	var tb textbuf.Buffer
	return errors.New(tb.Str("the ").Str(keyword).Str(" endpoint's `le ").Str(command).
		Str(" | json` carries no `").Str(key).Str("` key in ").Str(where).
		Str(", so the report cannot read that revision's figure; a missing key is never read as zero").String())
}

// changedRecordStems answers the stems with a record file under
// rfc/discrimination/ that differs between the two commits.
func changedRecordStems(ctx context.Context, events weekly.EventRange) (map[string]bool, error) {
	diff := exec.CommandContext(ctx, "git", "-C", events.Repo, "diff", "--no-renames", "--name-only", "-z", //nolint:gosec // both commits were resolved to full IDs by the report
		events.FromCommit, events.ToCommit, "--", discriminationRel+"/")
	var stderr bytes.Buffer
	diff.Stderr = &stderr
	raw, err := diff.Output()
	if err != nil {
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str("git diff of ").Str(discriminationRel).Str("/ between ").Str(events.FromCommit).
			Str(" and ").Str(events.ToCommit).Str(": ").Err(err).Str(": ").Str(strings.TrimSpace(stderr.String())).String())
	}
	stems := map[string]bool{}
	for path := range strings.SplitSeq(string(raw), "\x00") {
		rest, under := strings.CutPrefix(path, discriminationRel+"/")
		if !under {
			continue
		}
		stem, _, nested := strings.Cut(rest, "/")
		if !nested {
			continue
		}
		stems[stem] = true
	}
	return stems, nil
}

// comparedStems answers, sorted, the stems whose records are worth reading:
// a document present at one endpoint only, or whose proven counts differ, or
// whose record files changed. A stem whose proven SET changed while both
// counts stayed equal and no record file moved is not among them, and the
// section's note says so.
func comparedStems(from, to map[string]weeklyRow, changed map[string]bool) []string {
	stems := map[string]bool{}
	for stem := range changed {
		stems[stem] = true
	}
	for stem, before := range from {
		after, held := to[stem]
		if !held {
			stems[stem] = true
			continue
		}
		if before.mandatoryProven != after.mandatoryProven {
			stems[stem] = true
			continue
		}
		if before.optionalProven != after.optionalProven {
			stems[stem] = true
		}
	}
	for stem := range to {
		if _, held := from[stem]; !held {
			stems[stem] = true
		}
	}
	return sortedSet(stems)
}

// weeklyDiscriminateJSON is the part of `rfc discriminate stem <stem> | json`
// the report reads: the records that still verify against that tree.
type weeklyDiscriminateJSON struct {
	Records *[]weeklyRecordJSON `json:"records"`
}

type weeklyRecordJSON struct {
	RID      *string `json:"rid"`
	Polarity *string `json:"polarity"`
	Unit     *string `json:"unit"`
	Route    *string `json:"route"`
}

// weeklyProving asks one endpoint's le for the verified records of each
// compared stem that endpoint holds a summary for, and answers the covers a
// proving record carries: a verified record whose route is not the escape,
// the predicate provenRequirements counts. A stem with no summary there has no
// record that endpoint's le would verify, so it is not asked.
func weeklyProving(ctx context.Context, le endpointLe, keyword string, rows map[string]weeklyRow,
	compared []string) (map[Cover]*DiscriminationRecord, error) {
	proving := map[Cover]*DiscriminationRecord{}
	for _, stem := range compared {
		if !rows[stem].summary {
			continue
		}
		raw, err := le(ctx, "rfc", "discriminate", keyStem, stem)
		if err != nil {
			return nil, err
		}
		var decoded weeklyDiscriminateJSON
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return nil, fmt.Errorf("the %s endpoint's `le rfc discriminate stem %s | json` is not the JSON the report reads: %w",
				keyword, stem, err)
		}
		command := "rfc discriminate stem " + stem
		if decoded.Records == nil {
			return nil, weeklyShapeErr(keyword, command, "records", "the answer")
		}
		for index := range *decoded.Records {
			record, err := weeklyRecordOf(&(*decoded.Records)[index], keyword, command, stem)
			if err != nil {
				return nil, err
			}
			if !record.Proves() {
				continue
			}
			proving[record.Cover()] = &record
		}
	}
	return proving, nil
}

// weeklyRecordOf checks every key the report reads is present, then answers
// the record, its Source the file it lives in.
func weeklyRecordOf(record *weeklyRecordJSON, keyword, command, stem string) (DiscriminationRecord, error) {
	for _, required := range []struct {
		key     string
		missing bool
	}{
		{"rid", record.RID == nil},
		{"polarity", record.Polarity == nil},
		{"unit", record.Unit == nil},
		{"route", record.Route == nil},
	} {
		if required.missing {
			return DiscriminationRecord{}, weeklyShapeErr(keyword, command, required.key, "a record")
		}
	}
	return DiscriminationRecord{RID: *record.RID, Polarity: *record.Polarity, Unit: *record.Unit,
		Route: *record.Route, Source: discriminationRel + "/" + stem + "/" + *record.RID + jsonSuffix}, nil
}

// answerWeeklyProven answers the rfc-proven section: the MUST figure at each
// endpoint, then the requirements proven at `to` and not at `from`, and those
// lost.
func answerWeeklyProven(ctx context.Context, events weekly.EventRange) (weekly.SectionBody, error) {
	answer, err := weeklyRangeFor(ctx, events)
	if err != nil {
		return weekly.SectionBody{}, err
	}
	gained := provenRIDs(answer.to.proving, answer.from.proving)
	lost := provenRIDs(answer.from.proving, answer.to.proving)
	return weekly.SectionBody{
		Note: "MUSTs proven by a verified recorded break over the MUSTs owed, summed over the walked documents " +
			"as each endpoint's own `le rfc tree` answers them, the sum the ledger's rollup takes. " +
			"The requirement lists name ids, at any level, with a verified proving record at one endpoint and " +
			"not the other. They are read only for the documents compared record by record: those whose proven " +
			"counts differ between the endpoints, or whose rfc/discrimination/<stem>/ files changed in the range. " +
			"A document whose proven set changed while both counts stayed equal and no record file changed is not listed.",
		Facts: []weekly.Fact{
			{Name: "MUSTs proven at from", Value: mustFigure(answer.from.rows)},
			{Name: "MUSTs proven at to", Value: mustFigure(answer.to.rows)},
			{Name: "requirements newly proven", Value: strconv.Itoa(len(gained)), Items: gained},
			{Name: "requirements no longer proven", Value: strconv.Itoa(len(lost)), Items: lost},
			{Name: "documents compared record by record", Value: strconv.Itoa(len(answer.compared)), Items: answer.compared},
		},
	}, nil
}

// mustFigure sums the walked rows' MUST counts, as rollupOf does: an
// unwalked row's list is unbounded, so its rows count nowhere.
func mustFigure(rows map[string]weeklyRow) string {
	proven, total := 0, 0
	for _, row := range rows {
		if !row.walked {
			continue
		}
		proven += row.mandatoryProven
		total += row.mandatoryTotal
	}
	return strconv.Itoa(proven) + " of " + strconv.Itoa(total)
}

// provenRIDs answers, sorted, the requirement ids some cover in held proves
// and no cover in other does.
func provenRIDs(held, other map[Cover]*DiscriminationRecord) []string {
	elsewhere := map[string]bool{}
	for cover := range other {
		elsewhere[cover.RID] = true
	}
	rids := map[string]bool{}
	for cover := range held {
		if !elsewhere[cover.RID] {
			rids[cover.RID] = true
		}
	}
	return sortedSet(rids)
}

// answerWeeklyInterop answers the rfc-interop section: the scenarios whose
// checker gained a proving record in the range, and each interop record that
// names no scenario, with the reason.
func answerWeeklyInterop(ctx context.Context, events weekly.EventRange) (weekly.SectionBody, error) {
	answer, err := weeklyRangeFor(ctx, events)
	if err != nil {
		return weekly.SectionBody{}, err
	}
	scenarios, unmapped, err := gainedScenarios(events.ToDir, answer)
	if err != nil {
		return weekly.SectionBody{}, err
	}
	return weekly.SectionBody{
		Note: "Interop scenarios whose checker gained a verified proving record in the range, each named by the " +
			"checker's `const name` in the `to` tree. The records are those the rfc-proven section compared, " +
			"so the same bound applies. An interop record whose checker names no scenario is listed with the reason.",
		Facts: []weekly.Fact{
			{Name: "scenarios that gained a proving record", Value: strconv.Itoa(len(scenarios)), Items: scenarios},
			{Name: "interop records naming no scenario", Value: strconv.Itoa(len(unmapped)), Items: unmapped},
		},
	}, nil
}

// gainedScenarios maps every cover proven at `to` and not at `from` whose unit
// is an interop carrier to its scenario, `<suite> <scenario>`, through
// interopScenarioOf over the `to` export. A unit no scenario answers for is
// listed as `<unit>: <reason>`. Both lists are sorted, the scenarios once each.
func gainedScenarios(toDir string, answer *weeklyRange) (scenarios, unmapped []string, err error) {
	table, err := carriers(toDir)
	if err != nil {
		return nil, nil, fmt.Errorf("read the carrier table of the to export: %w", err)
	}
	reader := newSourceReader(toDir)
	index := newScopeIndex()
	named := map[string]bool{}
	missed := map[string]bool{}
	for cover, record := range answer.to.proving {
		if _, before := answer.from.proving[cover]; before {
			continue
		}
		rel := keyFile(record.Unit)
		carrier, held := CarrierFor(rel, table)
		if !held {
			continue
		}
		if carrier.Kind != kindInterop {
			continue
		}
		scenario, reason := scenarioOfRecord(toDir, reader, index, record, rel)
		if reason != "" {
			missed[record.Unit+": "+reason] = true
			continue
		}
		named[scenario] = true
	}
	return sortedSet(named), sortedSet(missed), nil
}

// scenarioOfRecord answers `<suite> <scenario>` for one interop record, or the
// reason none can be named.
func scenarioOfRecord(toDir string, reader *sourceReader, index *scopeIndex, record *DiscriminationRecord,
	rel string) (string, string) {
	suite, err := interopSuiteOf(rel)
	if err != nil {
		return "", err.Error()
	}
	scenario, err := interopScenarioOf(toDir, reader, index, record, suite)
	if err != nil {
		return "", err.Error()
	}
	return suite.Suite + " " + scenario, ""
}

// answerWeeklyExtraction answers the rfc-extraction section: the documents
// signed off at `to` and not at `from`, and those no longer signed off.
func answerWeeklyExtraction(ctx context.Context, events weekly.EventRange) (weekly.SectionBody, error) {
	answer, err := weeklyRangeFor(ctx, events)
	if err != nil {
		return weekly.SectionBody{}, err
	}
	signed := walkedOnly(answer.to.rows, answer.from.rows)
	unsigned := walkedOnly(answer.from.rows, answer.to.rows)
	return weekly.SectionBody{
		Note: "Documents whose requirement list is signed off against the RFC text (walked) at one endpoint and " +
			"not the other, as each endpoint's own `le rfc tree` answers them.",
		Facts: []weekly.Fact{
			{Name: "documents signed off in the range", Value: strconv.Itoa(len(signed)), Items: signed},
			{Name: "documents no longer signed off", Value: strconv.Itoa(len(unsigned)), Items: unsigned},
		},
	}, nil
}

// walkedOnly answers, sorted, the stems walked in held and not walked in
// other, where a stem other does not carry is not walked there.
func walkedOnly(held, other map[string]weeklyRow) []string {
	stems := map[string]bool{}
	for stem, row := range held {
		if !row.walked {
			continue
		}
		if other[stem].walked {
			continue
		}
		stems[stem] = true
	}
	return sortedSet(stems)
}
