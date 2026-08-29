// Design: docs/architecture/testing/test-health.md -- the three things this does
//
// modes.go holds the three answers: regenerate artifacts, compare structural
// facts with the committed snapshot, and append one KPI sample.
//
// They are ordered by what they touch. `check` writes nothing. `update`
// rewrites three files, every one of them a pure function of committed state.
// `record` APPENDS, which is the one thing here a caller has to mean rather
// than a file that can be rewritten twice for the same answer -- so it
// validates everything the follow-up write needs BEFORE touching the
// append-only history. Appending first meant a failing write left the sample
// recorded, the page stale, and every retry adding another row.
package testhealth

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// record is one whole reading of the tree: the metrics, and the committed
// history the trends are drawn from.
type record struct {
	metrics []Metric
	history []object
}

// objects renders the metrics shared by the artifacts and check.
func (r record) objects() []object {
	out := make([]object, 0, len(r.metrics))
	for _, metric := range r.metrics {
		out = append(out, metric.asObject())
	}
	return out
}

// build reads the tree once and answers every metric, in page order.
func build(root string) (record, error) {
	t := newTree(root)
	floors, err := readQualityFloors(root)
	if err != nil {
		return record{}, err
	}

	headline, unproven, err := collectRFC(t, floors)
	if err != nil {
		return record{}, err
	}
	inert, orphan, err := collectInert(t)
	if err != nil {
		return record{}, err
	}
	inventory, err := collectInventory(t)
	if err != nil {
		return record{}, err
	}
	sleeps, err := collectSleepRatchet(t)
	if err != nil {
		return record{}, err
	}
	negative, err := collectNegativeTests(t, floors)
	if err != nil {
		return record{}, err
	}
	adoption, err := collectAdoption(t)
	if err != nil {
		return record{}, err
	}
	failures, err := collectKnownFailures(t)
	if err != nil {
		return record{}, err
	}

	history, err := loadHistory(t)
	if err != nil {
		return record{}, err
	}
	return record{
		metrics: []Metric{
			headline, unproven, inert, orphan, inventory,
			sleeps, negative, adoption, failures,
		},
		history: history,
	}, nil
}

// loadHistory reads the committed KPI samples, and refuses a line that is not
// JSON: a history nobody can parse is not a history of nothing.
func loadHistory(t *tree) ([]object, error) {
	if !exists(filepath.Join(t.root, filepath.FromSlash(History))) {
		return nil, nil
	}
	body, err := t.readBody(History)
	if err != nil {
		return nil, err
	}

	var rows []object
	for index, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		parsed, parseErr := parseObject([]byte(trimmed))
		if parseErr != nil {
			return nil, collectErrorf("%s line %d is not valid JSON: %w", History, index+1, parseErr)
		}
		rows = append(rows, parsed)
	}
	return rows, nil
}

// kpiRow answers the KPI subset worth storing per sample. Deliberately small.
func kpiRow(metrics []Metric) (object, error) {
	byKey := make(map[string]Metric, len(metrics))
	for _, metric := range metrics {
		byKey[metric.Key] = metric
	}

	density, ok := byKey[keyProofDensity].Data.get("proof_density").(object)
	if !ok {
		return object{}, collectErrorf(
			"the record carries no `rfc-proof-density` ratio, so a sample would record a " +
				"blank where the headline number belongs")
	}
	inert, ok := byKey[keyAssertNothing].Data.get("inert").(object)
	if !ok {
		return object{}, collectErrorf(
			"the record carries no `assert-nothing` ratio, so a sample would record a blank " +
				"where the sensitivity count belongs")
	}
	if !byKey[keyTagOrphan].Data.has("orphan_count") {
		return object{}, collectErrorf("the record carries no `tag-orphan` count")
	}
	row := object{}
	row.set("rfc_proof_numerator", density.get("numerator"))
	row.set("rfc_proof_denominator", density.get("denominator"))
	row.set("rfc_proof_percent", percentOf(density))
	row.set("assert_nothing", inert.get("numerator"))
	row.set("tests_scanned", inert.get("denominator"))
	row.set("tag_orphan", byKey[keyTagOrphan].Data.get("orphan_count"))
	row.set("ci_sleeps", byKey[keySleeps].Data.get("actual"))
	return row, nil
}

// latestJSON answers the exact bytes of the structured artifact, so the write
// and the check agree.
func latestJSON(built record) (string, error) {
	document := object{}
	entries := make([]any, 0, len(built.metrics))
	for _, metric := range built.objects() {
		entries = append(entries, metric)
	}
	document.set("metrics", entries)

	body, err := dumpIndented(document)
	if err != nil {
		return "", collectErrorf("%s cannot be encoded: %w", Latest, err)
	}
	var tb textbuf.Buffer
	return tb.Str(body).Byte('\n').String(), nil
}

// WriteReport is what `update` answers: the three artifacts it rewrote.
type WriteReport struct {
	Page     string `json:"page"`
	Latest   string `json:"latest"`
	Baseline string `json:"baseline"`
}

// Text renders the one line the script printed.
func (w WriteReport) Text() string {
	var tb textbuf.Buffer
	return tb.Str("test-health: wrote ").Str(w.Page).Str(", ").Str(w.Latest).Str(", ").
		Str(w.Baseline).Byte('\n').String()
}

// Write regenerates the page, the structured sibling and both ratchet floors.
func Write(root string, bootstrap bool) (WriteReport, error) {
	built, err := build(root)
	if err != nil {
		return WriteReport{}, err
	}

	// Tighten first, then render because each floor is part of the rendered
	// value. Rendering first would write an immediately stale page.
	row, err := kpiRow(built.metrics)
	if err != nil {
		return WriteReport{}, err
	}
	moved, err := tightenSensitivity(root, row, bootstrap)
	if err != nil {
		return WriteReport{}, err
	}
	qualityMoved, err := tightenQuality(root, built.metrics)
	if err != nil {
		return WriteReport{}, err
	}
	if moved || qualityMoved {
		built, err = build(root)
		if err != nil {
			return WriteReport{}, err
		}
	}

	page := renderMarkdown(built.metrics, built.history)
	if err := writeFile(root, Page, page); err != nil {
		return WriteReport{}, err
	}
	body, err := latestJSON(built)
	if err != nil {
		return WriteReport{}, err
	}
	if err := writeFile(root, Latest, body); err != nil {
		return WriteReport{}, err
	}
	return WriteReport{Page: Page, Latest: Latest, Baseline: Baseline}, nil
}

// writeFile writes one artifact, creating its directory.
func writeFile(root, rel, body string) error {
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return collectErrorf("%s cannot be created: %w", rel, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		return collectErrorf("%s cannot be written: %w", rel, err)
	}
	return nil
}

// CheckReport is what `check` answers: whether the committed snapshot still
// states the tree's structural facts, and what moved when it does not.
type CheckReport struct {
	Latest string `json:"latest"`
	Match  bool   `json:"match"`
	// Missing names an artifact that is not there at all, which is a different
	// failure from a fact that moved.
	Missing string `json:"missing,omitempty"`
	// Unreadable says the committed snapshot could not be read as a record.
	Unreadable string `json:"unreadable,omitempty"`
	// Changes names each checked fact that moved.
	Changes []Change `json:"changes"`
}

// Code answers the check's exit status. A stale page and an absent page both
// give the reader the update action.
func (c CheckReport) Code() int {
	if c.Match {
		return 0
	}
	return 1
}

// Text renders the verdict for a person.
func (c CheckReport) Text() string {
	var tb textbuf.Buffer
	switch {
	case c.Missing != "":
		return tb.Str("test-health: ").Str(c.Missing).
			Str(" does not exist. Run `./le test-health update`.\n").String()
	case c.Unreadable != "":
		return tb.Str("test-health: ").Str(c.Unreadable).Byte('\n').String()
	case c.Match:
		return tb.Str("test-health: structural facts in ").Str(c.Latest).
			Str(" match the tree (volume counters are refreshed by " +
				"`./le test-health update`)\n").String()
	}

	tb.Str("test-health: a STRUCTURAL fact changed without the report being regenerated. " +
		"These are checked because each one is an event, not churn.\n")
	for _, change := range c.Changes {
		tb.Str("  ").Str(change.Fact).Str(":\n")
		if len(change.Gone) > 0 {
			tb.Str("    left the committed list: ").Join(change.Gone, ", ").Byte('\n')
		}
		if len(change.New) > 0 {
			tb.Str("    new in the generated list: ").Join(change.New, ", ").Byte('\n')
		}
		if len(change.Committed) > 0 || len(change.Generated) > 0 {
			tb.Str("    committed: ").Join(change.Committed, ", ").Byte('\n')
			tb.Str("    generated: ").Join(change.Generated, ", ").Byte('\n')
		}
	}
	tb.Str("  Run `./le test-health update` and commit the result.\n")
	return tb.String()
}

// Check compares the committed snapshot's structural facts with the tree.
func Check(root string) (CheckReport, error) {
	built, err := build(root)
	if err != nil {
		return CheckReport{}, err
	}

	report := CheckReport{Latest: Latest, Changes: []Change{}}
	for _, rel := range [...]string{Page, Latest} {
		if !exists(filepath.Join(root, filepath.FromSlash(rel))) {
			report.Missing = rel
			return report, nil
		}
	}

	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(Latest))) // #nosec G304 -- a repository-relative path of the checkout this tool was pointed at
	if err != nil {
		var tb textbuf.Buffer
		report.Unreadable = tb.Str(Latest).Str(" cannot be read: ").Err(err).String()
		return report, nil
	}
	parsed, err := parseObject(raw)
	if err != nil {
		var tb textbuf.Buffer
		report.Unreadable = tb.Str(Latest).Str(" is not valid JSON: ").Err(err).String()
		return report, nil
	}
	entries, isList := parsed.get("metrics").([]any)
	if !isList {
		var tb textbuf.Buffer
		report.Unreadable = tb.Str(Latest).Str(" has no metrics list.").String()
		return report, nil
	}

	committed := make([]object, 0, len(entries))
	for _, entry := range entries {
		metric, isObject := entry.(object)
		if !isObject {
			var tb textbuf.Buffer
			report.Unreadable = tb.Str(Latest).Str(" has a metric that is not an object.").String()
			return report, nil
		}
		committed = append(committed, metric)
	}

	want, err := structuralFacts(built.objects())
	if err != nil {
		return CheckReport{}, err
	}
	got, err := structuralFacts(committed)
	if err != nil {
		return CheckReport{}, err
	}
	if got.Equal(want) {
		report.Match = true
		return report, nil
	}
	report.Changes = Describe(got, want)
	return report, nil
}

// RecordReport is what `record` answers: the sample, and the write that
// followed it.
type RecordReport struct {
	Appended bool        `json:"appended"`
	Sha      string      `json:"sha"`
	History  string      `json:"history"`
	Sample   object      `json:"-"`
	Wrote    WriteReport `json:"wrote"`
}

// Text renders the two lines the script printed.
func (r RecordReport) Text() string {
	var tb textbuf.Buffer
	if r.Appended {
		tb.Str("test-health: recorded one sample in ").Str(r.History).Byte('\n')
	} else {
		tb.Str("test-health: sample identical to the last one at ").Str(r.Sha).
			Str("; nothing appended to ").Str(r.History).Byte('\n')
	}
	return tb.Str(r.Wrote.Text()).String()
}

// Record appends one KPI sample to the committed history, then regenerates the
// page: the trends are rendered from the history, so appending to it makes the
// committed page stale.
//
// The commit sha is a REFUSAL when git cannot answer, which is the fail-open
// the port closes: the script ignored git's exit status and recorded the sample
// under the literal sha "unknown", so a sample nobody can attribute to a commit
// landed in an append-only file with no diagnostic.
func Record(root string, bootstrap bool) (RecordReport, error) {
	built, err := build(root)
	if err != nil {
		return RecordReport{}, err
	}

	// Validate everything the follow-up write needs BEFORE touching the
	// append-only history.
	if !exists(filepath.Join(root, filepath.FromSlash(Baseline))) && !bootstrap {
		return RecordReport{}, collectErrorf(
			"%s does not exist, so the page cannot be regenerated after recording. Restore it "+
				"from git, or pass bootstrap to create it deliberately. Nothing was appended to %s",
			Baseline, History)
	}

	row, err := kpiRow(built.metrics)
	if err != nil {
		return RecordReport{}, err
	}
	t := newTree(root)
	out, err := t.gitOutput("rev-parse", "--short", "HEAD")
	if err != nil {
		return RecordReport{}, collectErrorf(
			"git cannot name HEAD, so a sample would be recorded against a commit nobody can "+
				"identify. Nothing was appended to %s: %w", History, err)
	}
	sha := strings.TrimSpace(out)
	if sha == "" {
		return RecordReport{}, collectErrorf(
			"git named an empty HEAD, so a sample would be recorded against a commit nobody "+
				"can identify. Nothing was appended to %s", History)
	}

	// Wall clock is allowed HERE and nowhere else: history is append-only, so a
	// timestamp cannot make the generated page churn.
	sample := object{}
	sample.set("ts", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	sample.set("sha", sha)
	for _, key := range row.keys {
		sample.set(key, row.get(key))
	}

	// Skip a sample identical to the previous one at the same commit. Recording
	// runs from every mutation target, so re-running at one sha would otherwise
	// stack duplicate points into the sparkline and overstate n -- and n is what
	// the page prints beside every trend to keep it honest.
	if len(built.history) > 0 && sameSample(built.history[len(built.history)-1], sample) {
		wrote, writeErr := Write(root, bootstrap)
		if writeErr != nil {
			return RecordReport{}, writeErr
		}
		return RecordReport{Sha: sha, History: History, Sample: sample, Wrote: wrote}, nil
	}

	line, err := dumpCompact(sample)
	if err != nil {
		return RecordReport{}, collectErrorf("%s cannot be encoded: %w", History, err)
	}
	if err := appendLine(root, History, line); err != nil {
		return RecordReport{}, err
	}

	wrote, err := Write(root, bootstrap)
	if err != nil {
		return RecordReport{}, err
	}
	return RecordReport{
		Appended: true, Sha: sha, History: History, Sample: sample, Wrote: wrote,
	}, nil
}

// appendLine adds one row to the append-only history.
func appendLine(root, rel, line string) error {
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return collectErrorf("%s cannot be created: %w", rel, err)
	}
	handle, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) // #nosec G304 -- a repository-relative path of the checkout this tool was pointed at
	if err != nil {
		return collectErrorf("%s cannot be opened: %w", rel, err)
	}
	var tb textbuf.Buffer
	if _, err := handle.WriteString(tb.Str(line).Byte('\n').String()); err != nil {
		return collectErrorf("%s cannot be appended to: %w", rel, err)
	}
	if err := handle.Close(); err != nil {
		return collectErrorf("%s cannot be closed: %w", rel, err)
	}
	return nil
}

// sameSample reports whether the candidate says the same thing as the last
// recorded row, ignoring the timestamp.
func sameSample(last, candidate object) bool {
	keys := map[string]bool{}
	for _, key := range last.keys {
		keys[key] = true
	}
	for _, key := range candidate.keys {
		keys[key] = true
	}
	delete(keys, "ts")

	names := make([]string, 0, len(keys))
	for key := range keys {
		names = append(names, key)
	}
	sort.Strings(names)

	for _, key := range names {
		if last.has(key) != candidate.has(key) {
			return false
		}
		if !sameValue(last.get(key), candidate.get(key)) {
			return false
		}
	}
	return true
}

// sameValue compares two record values the way Python compares them, which
// counts 1 and 1.0 as one number.
func sameValue(left, right any) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	leftText, leftIsText := left.(string)
	rightText, rightIsText := right.(string)
	if leftIsText || rightIsText {
		return leftIsText && rightIsText && leftText == rightText
	}
	if !isNumber(left) || !isNumber(right) {
		return false
	}
	return numberOf(left) == numberOf(right)
}

// isNumber reports whether a record value is one of the numeric spellings.
func isNumber(value any) bool {
	switch value.(type) {
	case int, float64, pyNum:
		return true
	default:
		return false
	}
}
