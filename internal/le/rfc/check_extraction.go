// Design: docs/architecture/core-design.md -- the rfc area, as one command
// Related: check.go -- the centralized check driver that shares extraction results
// Related: render.go -- the generated pages checked for freshness here
// Related: selftest_state.go -- in-process fixtures for the extraction ratchet and generated pages
//
// check_extraction.go guards extraction monotonicity, the authored drain policy,
// and every generated ledger page. It never writes.
package rfc

import (
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

type drainBudget struct {
	start time.Time
	rate  float64
}

const (
	generatedMissingSuffix = " is missing -- run: make ze-rfc-index-update"
	generatedStaleSuffix   = " is stale vs its sources -- run: make ze-rfc-index-update"
)

func checkExtractionRatchet(tree string, current map[string]Extraction) []string {
	baseline, known := baselineExtractions(tree)
	return checkExtractionRatchetAgainst(current, baseline, known)
}

// checkExtractionRatchetAgainst compares one live extraction set with a HEAD
// snapshot. Keeping the comparison apart from Git lets the in-process selftest
// prove the ratchet without starting another program.
func checkExtractionRatchetAgainst(current map[string]Extraction,
	baseline map[string]baselineExtraction, known bool) []string {
	if !known {
		return nil
	}
	var errs []string
	currentSet := map[string]bool{}
	for stem := range current {
		currentSet[stem] = true
	}
	baselineSet := map[string]bool{}
	for stem := range baseline {
		baselineSet[stem] = true
	}
	for _, stem := range sortedMissing(baselineSet, currentSet) {
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(stem).Str(" had an extraction sign-off at HEAD and has none now. Extraction sign-off is monotonic: an RFC whose source walk bounded its summary cannot stop being bounded. Restore rfc/extraction/").Str(stem).Str(".json").String())
	}
	for _, stem := range sortedShared(baseline, current) {
		was := baseline[stem]
		art := current[stem]
		if art.Excluded() <= was.excluded {
			continue
		}
		if art.ResignReason == "" {
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(art.Path).Str(": exclusions rose from ").Int(int64(was.excluded)).Str(" to ").Int(int64(art.Excluded())).Str(" with no 'resign-reason'. Exclusions are shrink-only: a rise means the walk was redone, so record why, name the reviewer, and bump signed-off").String())
			continue
		}
		if art.SignedOff == was.signedOff {
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(art.Path).Str(": exclusions rose from ").Int(int64(was.excluded)).Str(" to ").Int(int64(art.Excluded())).Str(" with a resign-reason but the same signed-off date (").Str(art.SignedOff).Str("). A re-sign is a new walk; reusing the old date says it did not happen").String())
			continue
		}
		if art.ResignReason == was.resignReason {
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(art.Path).Str(": exclusions rose from ").Int(int64(was.excluded)).Str(" to ").Int(int64(art.Excluded())).Str(", but 'resign-reason' is unchanged from the previous sign-off (").Str(pyRepr(truncateRunes(was.resignReason, 60))).Str("). It is carried forward automatically by make ze-rfc-extraction-create, so an unchanged reason justifies the EARLIER walk, not this one. Say what this walk found that raised the exclusions").String())
		}
	}
	return errs
}

func parseDrainBudget(tree string) (drainBudget, error) {
	const rel = "rfc/drain-budget.txt"
	raw, err := os.ReadFile(treePath(tree, rel)) // #nosec G304 -- the policy under the checkout
	if err != nil {
		var tb textbuf.Buffer
		return drainBudget{}, baselineParseError(tb.Str(rel).Str(": cannot read the drain policy: ").Err(err).Str(". An absent budget does NOT mean 'nothing owed' -- a zero value must never be a valid-looking answer (ai/rules/evidence.md). Create it with a 'start' date and a 'rate' in entries per calendar month").String())
	}
	values := map[string]string{}
	for index, rawLine := range strings.Split(string(raw), "\n") {
		line := strings.TrimSpace(strings.SplitN(rawLine, "#", 2)[0])
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 || (parts[0] != "start" && parts[0] != "rate") {
			var tb textbuf.Buffer
			return drainBudget{}, baselineParseError(tb.Str(rel).Byte(':').Int(int64(index + 1)).Str(": expected '<key> <value>' with key in ['start', 'rate'], got ").Str(pyRepr(line)).Str(". This file carries POLICY ONLY: it may never name an RFC, hold a count, or list stems").String())
		}
		if _, held := values[parts[0]]; held {
			var tb textbuf.Buffer
			return drainBudget{}, baselineParseError(tb.Str(rel).Byte(':').Int(int64(index + 1)).Str(": ").Str(pyRepr(parts[0])).Str(" is set twice").String())
		}
		values[parts[0]] = parts[1]
	}
	var missing []string
	for _, key := range []string{"start", "rate"} {
		if _, held := values[key]; !held {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		var tb textbuf.Buffer
		return drainBudget{}, baselineParseError(tb.Str(rel).Str(": missing ").Str(pyRepr(missing)).Str(". Both are required: 'start' is when the drain clock begins, 'rate' is entries per calendar month (it ships at 0, and only the owner arms it)").String())
	}
	start, err := time.Parse("2006-01-02", values["start"])
	if err != nil {
		var tb textbuf.Buffer
		return drainBudget{}, baselineParseError(tb.Str(rel).Str(": start ").Str(pyRepr(values["start"])).Str(" is not a YYYY-MM-DD date: ").Err(err).String())
	}
	rate, err := strconv.ParseFloat(values["rate"], 64)
	if err != nil {
		var tb textbuf.Buffer
		return drainBudget{}, baselineParseError(tb.Str(rel).Str(": rate ").Str(pyRepr(values["rate"])).Str(" is not a number of entries per calendar month").String())
	}
	if math.IsNaN(rate) || math.IsInf(rate, 0) {
		var tb textbuf.Buffer
		return drainBudget{}, baselineParseError(tb.Str(rel).Str(": rate ").Str(pyRepr(values["rate"])).Str(" is not a finite number of entries per calendar month. A schedule needs a rate arithmetic can compare against a count").String())
	}
	if rate < 0 {
		var tb textbuf.Buffer
		return drainBudget{}, baselineParseError(tb.Str(rel).Str(": rate ").Str(formatRate(rate)).Str(" is negative; a backlog cannot un-drain").String())
	}
	return drainBudget{start: start, rate: rate}, nil
}

func formatRate(rate float64) string {
	text := strconv.FormatFloat(rate, 'f', -1, 64)
	if !strings.Contains(text, ".") {
		text += ".0"
	}
	return text
}

func requiredFloor(start time.Time, rate float64, drainable int, today time.Time) int {
	months := (today.Year()-start.Year())*12 + int(today.Month()-start.Month())
	anniversary := start.Day()
	lastDay := time.Date(today.Year(), today.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if anniversary > lastDay {
		anniversary = lastDay
	}
	if today.Day() < anniversary {
		months--
	}
	if months < 0 {
		months = 0
	}
	return min(drainable, int(math.Ceil(rate*float64(months))))
}

func checkDrainFloor(tree string, enrolled map[string]bool, signed map[string]Extraction,
	today time.Time) []string {
	budget, err := parseDrainBudget(tree)
	if err != nil {
		return []string{err.Error()}
	}
	credited := Credited(signed, enrolled)
	counts := RegisterCounts(credited)
	total := len(credited)
	backlog := len(enrolled) - total
	if budget.rate > float64(len(enrolled)) {
		var tb textbuf.Buffer
		return []string{tb.Str("rfc/drain-budget.txt: rate ").Str(formatRate(budget.rate)).Str(" exceeds the whole enrolled set (").Int(int64(len(enrolled))).Str("); no schedule can be met").String()}
	}
	floor := requiredFloor(budget.start, budget.rate, len(enrolled), today)
	if total >= floor {
		return nil
	}
	var tb textbuf.Buffer
	return []string{tb.Str("rfc/drain-budget.txt: the drain schedule requires ").Int(int64(floor)).Str(" extraction sign-off(s) by now (rate ").Str(formatRate(budget.rate)).Str("/calendar month since ").Str(budget.start.Format("2006-01-02")).Str(", capped at the ").Int(int64(len(enrolled))).Str(" enrolled RFC(s)), and there are ").Int(int64(total)).Str(" (").Str(registerPhrase(counts)).Str("; every register counts, umbrella D6), leaving ").Int(int64(backlog)).Str(" unsigned. Walk another RFC: make ze-rfc-extraction-create STEM=<stem>, then classify every site").String()}
}

func checkLedgerFresh(tree string, collected Collected, rows map[string]LedgerRow,
	dispositions map[string]Disposition) ([]string, error) {
	input, err := NewRenderInput(tree, collected, rows, dispositions)
	if err != nil {
		return nil, err
	}
	var errs []string
	var tb textbuf.Buffer
	index, err := RenderIndex(input)
	if err != nil {
		return nil, err
	}
	index = tb.Str(index).Byte('\n').String()
	current, readErr := os.ReadFile(treePath(tree, ledgerRel)) // #nosec G304 -- generated page under the checkout
	if readErr != nil || string(current) != index {
		errs = append(errs, tb.Reset().Str(ledgerRel).Str(generatedStaleSuffix).String())
	}
	shards := RenderShards(input)
	keep := map[string]bool{}
	for _, stem := range ShardStems(collected.Requirements) {
		keep[stem] = true
		rel := shardRel(stem)
		path := treePath(tree, rel)
		current, readErr := os.ReadFile(path) // #nosec G304 -- generated page under the checkout
		if os.IsNotExist(readErr) {
			errs = append(errs, tb.Reset().Str(rel).Str(generatedMissingSuffix).String())
			continue
		}
		want := tb.Reset().Str(shards[stem]).Byte('\n').Slice()
		if readErr != nil {
			errs = append(errs, tb.Reset().Str(rel).Str(generatedStaleSuffix).String())
			continue
		}
		if string(current) != want {
			errs = append(errs, tb.Reset().Str(rel).Str(generatedStaleSuffix).String())
		}
	}
	prunable, err := PrunableShards(tree, keep)
	if err != nil {
		return nil, err
	}
	for _, stem := range prunable {
		errs = append(errs, tb.Reset().Str(shardRelDir).Byte('/').Str(stem).Str(".md renders no requirement section and the generator no longer owns it -- run: make ze-rfc-index-update").String())
	}
	return errs, nil
}
