// Design: docs/architecture/core-design.md -- the rfc area, as one command
// Related: check.go -- the centralized check driver that orders these ratchets
// Related: check_baseline.go -- the committed snapshots each ratchet compares
//
// check_ratchets.go holds the monotonic checks over requirements and evidence.
package rfc

import (
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const minCorrectionQuote = 24

var (
	correctionOpenRE  = regexp.MustCompile(`^Correction\s+(\d{4}-\d{2}-\d{2})\s*:`)
	correctionRIDRE   = regexp.MustCompile("`([A-Za-z0-9][A-Za-z0-9.\\-]*-\\d+)`")
	correctionQuoteRE = regexp.MustCompile(`"([^"]+)"`)
)

func requirementWhere(req Requirement) string {
	if req.Source == "" {
		return req.RID
	}
	var tb textbuf.Buffer
	return tb.Str(req.Source).Byte(':').Int(int64(req.Line)).String()
}

func highWater(ids map[string]bool) map[string]int {
	out := map[string]int{}
	for rid := range ids {
		match := idRE.FindStringSubmatch(rid)
		if match == nil {
			continue
		}
		ordinal, err := strconv.Atoi(match[2])
		if err == nil && ordinal > out[match[1]] {
			out[match[1]] = ordinal
		}
	}
	return out
}

func checkIDAllocation(requirements []Requirement, baseline map[string]bool) []string {
	marks := highWater(baseline)
	var errs []string
	for _, req := range requirements {
		match := idRE.FindStringSubmatch(req.RID)
		if match == nil {
			continue
		}
		ordinal, err := strconv.Atoi(match[2])
		if err != nil {
			continue
		}
		mark, held := marks[match[1]]
		if !held || ordinal > mark || baseline[req.RID] {
			continue
		}
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(req.Source).Byte(':').Int(int64(req.Line)).Str(": ").
			Str(req.RID).Str(" reuses a retired id. ").Str(match[1]).Str(" has allocated up to -").
			Int(int64(mark)).Str("; a new requirement must take -").Int(int64(mark+1)).
			Str(" or higher. Reusing an id silently re-points every test tagged ").Str(req.RID).
			Str(" at a different obligation.").String())
	}
	return errs
}

func checkEnrolment(tree string, current, baseline, summaries, newly, signed map[string]bool) []string {
	var errs []string
	if len(current) == 0 {
		errs = append(errs, "nothing is enrolled: rfc/enrolled.txt is empty or missing. The gate refuses to report clean while enforcing nothing (ai/rules/evidence.md)")
	}
	for _, rfc := range sortedMissing(baseline, current) {
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(rfc).Str(" was un-enrolled. Enrolment is monotonic: an RFC whose MUSTs were gated cannot stop being gated. Restore it in rfc/enrolled.txt").String())
	}
	for _, rfc := range sortedMissing(current, summaries) {
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(rfc).Str(" is enrolled but rfc/short/").Str(rfc).Str(".md does not exist -- there is no requirement list to enforce").String())
	}
	for _, rfc := range sortedSet(current) {
		if _, held := sourceKeywordCount(tree, rfc); held {
			continue
		}
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(rfc).Str(" is enrolled but there is no source text at rfc/full/").Str(rfc).
			Str(".txt or rfc/drafts/").Str(rfc).Str(".txt -- without it the summary is validated only against itself, so a requirement the RFC does not contain cannot be caught and a requirement it does contain can be missing invisibly. Fetch the source (https://www.rfc-editor.org/rfc/").Str(rfc).
			Str(".txt for an RFC; the datatracker archive for a draft) before enrolling").String())
	}
	for _, rfc := range sortedSet(newly) {
		if signed[rfc] {
			continue
		}
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(rfc).Str(" is newly enrolled with no valid extraction sign-off at rfc/extraction/").Str(rfc).
			Str(".json. Enrolling gates the requirements the summary LISTS; nothing bounds what it MISSED until the source text has been walked site by site (ai/rules/rfc-compliance.md, Extraction Completeness). Run: ./le rfc extraction-create stem ").Str(rfc).
			Str(", then classify every site and section. RFCs enrolled before this gate existed are grandfathered and unaffected").String())
	}
	return errs
}

func checkCoverageRatchet(requirements []Requirement, tags []Tag, enrolled map[string]bool,
	baseline map[string]map[string]bool, baselineEnrolled map[string]bool) []string {
	current := baselinePolarities(tags)
	seen := map[string]bool{}
	var errs []string
	for _, req := range requirements {
		if !enrolled[req.RFC] || !baselineEnrolled[req.RFC] || seen[req.RID] {
			continue
		}
		was := baseline[req.RID]
		var lost []string
		for polarity := range was {
			if !current[req.RID][polarity] {
				lost = append(lost, polarity)
			}
		}
		if len(lost) == 0 {
			continue
		}
		sort.Strings(lost)
		seen[req.RID] = true
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(requirementWhere(req)).Str(": ").Str(req.RID).
			Str(" is no longer proven -- the ").Str(strings.Join(lost, "/")).
			Str(" test(s) that covered it at HEAD are gone. Coverage is monotonic: evidence that existed cannot quietly stop existing. Restore the test, or retire the requirement id if the obligation itself is gone. An annotation does not substitute for proof that was already there").String())
	}
	return errs
}

func checkEvidenceRatchet(requirements []Requirement, tags []Tag, enrolled map[string]bool,
	carriers []Carrier, baseline map[string]map[string]bool, baselineEnrolled map[string]bool) []string {
	current := nonunitEvidence(tags, carriers)
	seen := map[string]bool{}
	var errs []string
	for _, req := range requirements {
		if !enrolled[req.RFC] || !baselineEnrolled[req.RFC] || seen[req.RID] {
			continue
		}
		var lost, kept []string
		for label := range baseline[req.RID] {
			if !current[req.RID][label] {
				lost = append(lost, label)
			}
		}
		if len(lost) == 0 {
			continue
		}
		for label := range current[req.RID] {
			kept = append(kept, label)
		}
		sort.Strings(lost)
		sort.Strings(kept)
		still := "nothing but unit tests"
		if len(kept) > 0 {
			still = strings.Join(kept, ", ")
		}
		seen[req.RID] = true
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(requirementWhere(req)).Str(": ").Str(req.RID).
			Str(" has lost its ").Str(strings.Join(lost, "/")).Str(" evidence -- the test(s) of that kind that proved it at HEAD are gone, leaving ").Str(still).
			Str(". Non-unit evidence is monotonic and each tier ratchets on its own: a unit test proves the algorithm, a functional test proves the daemon exposes the behavior, an interop test proves a peer accepts it, and a nightly-tier binding never substitutes for a verify-tier one. Restore the test, or retire the requirement id if the obligation itself is gone. No annotation satisfies this").String())
	}
	return errs
}

func checkRetiredRequirements(requirements []Requirement, enrolled, baselineIDs,
	baselineEnrolled, stems, baselineStems map[string]bool, parseByStem map[string]string) []string {
	live := map[string]bool{}
	for _, req := range requirements {
		live[req.RID] = true
	}
	known := map[string]bool{}
	for stem := range stems {
		known[stem] = true
	}
	for stem := range baselineStems {
		known[stem] = true
	}
	ordered := sortedSet(known)
	sort.Slice(ordered, func(i, j int) bool { return len(ordered[i]) > len(ordered[j]) })
	silent := map[string]bool{}
	for stem := range parseByStem {
		silent[stem] = true
	}
	for stem := range baselineStems {
		if !stems[stem] {
			silent[stem] = true
		}
	}
	var errs []string
	for _, rid := range sortedSet(baselineIDs) {
		if live[rid] {
			continue
		}
		stem := ""
		for _, candidate := range ordered {
			if hasRIDStem(rid, candidate) {
				stem = candidate
				break
			}
		}
		if stem == "" || !enrolled[stem] || !baselineEnrolled[stem] || silent[stem] {
			continue
		}
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(rid).Str(" was in rfc/short/").Str(stem).
			Str(".md at HEAD and is now gone. Requirement ids are permanent: deleting the line retires the obligation silently, which is exactly the move that makes a compliance claim rot. Restore the line (edit its TEXT under the same id if the wording was wrong), and annotate it if it is not met").String())
	}
	return errs
}

func parseCorrections(text string) []correction {
	lines := strings.Split(text, "\n")
	var out []correction
	for start := 0; start < len(lines); {
		if strings.TrimSpace(lines[start]) == "" {
			start++
			continue
		}
		end := start
		for end < len(lines) && strings.TrimSpace(lines[end]) != "" {
			end++
		}
		body := make([]string, 0, end-start)
		for _, line := range lines[start:end] {
			body = append(body, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), ">")))
		}
		open := correctionOpenRE.FindStringSubmatch(body[0])
		if open != nil {
			joined := strings.Join(body, " ")
			correction := correction{Date: open[1], Line: start + 1}
			for _, match := range correctionRIDRE.FindAllStringSubmatch(joined, -1) {
				correction.RIDs = append(correction.RIDs, match[1])
			}
			for _, match := range correctionQuoteRE.FindAllStringSubmatch(joined, -1) {
				correction.Quotes = append(correction.Quotes, match[1])
			}
			out = append(out, correction)
		}
		start = end
	}
	return out
}

func squashWhitespace(text string) string { return strings.Join(strings.Fields(text), " ") }

func correctionAuthorizes(rid string, corrections []correction, source string) bool {
	haystack := squashWhitespace(source)
	for _, correction := range corrections {
		named := false
		for _, one := range correction.RIDs {
			if one == rid {
				named = true
			}
		}
		if !named {
			continue
		}
		for _, quote := range correction.Quotes {
			quote = squashWhitespace(quote)
			if len(quote) >= minCorrectionQuote && strings.Contains(haystack, quote) {
				return true
			}
		}
	}
	return false
}

func checkLevelRatchet(tree string, requirements []Requirement, enrolled map[string]bool,
	levels map[string]string, baselineEnrolled map[string]bool) []string {
	seen := map[string]bool{}
	corrections := map[string][]correction{}
	sources := map[string]string{}
	var errs []string
	for _, req := range requirements {
		was, held := levels[req.RID]
		if !enrolled[req.RFC] || !baselineEnrolled[req.RFC] || !held || !gatedLevels[was] || req.Gated() || seen[req.RID] {
			continue
		}
		seen[req.RID] = true
		if _, loaded := corrections[req.RFC]; !loaded {
			text, _ := os.ReadFile(treePath(tree, summaryRelOf(req.RFC))) // #nosec G304 -- a summary under the checkout
			corrections[req.RFC] = parseCorrections(string(text))
			sources[req.RFC], _ = SourceText(tree, req.RFC)
		}
		var tb textbuf.Buffer
		section := tb.Str("section ").Str(req.Section).String()
		if req.Section == noSection {
			section = "no section cited"
		}
		if sources[req.RFC] == "" {
			errs = append(errs, tb.Reset().Str(requirementWhere(req)).Str(": ").Str(req.RID).Str(" (").Str(section).
				Str(") moved [").Str(was).Str("] -> [").Str(req.Level).
				Str("] and the RFC's own text is not in the repository, so no correction can be checked against it. Fetch it to rfc/full/").Str(req.RFC).Str(".txt or rfc/drafts/").Str(req.RFC).Str(".txt, then record the correction").String())
			continue
		}
		if correctionAuthorizes(req.RID, corrections[req.RFC], sources[req.RFC]) {
			continue
		}
		errs = append(errs, tb.Reset().Str(requirementWhere(req)).Str(": ").Str(req.RID).Str(" (").Str(section).
			Str(") moved [").Str(was).Str("] -> [").Str(req.Level).
			Str("] and left the gated MUST-level population with nothing recorded. Gating is monotonic: the row keeps its id and its tests, so no other ratchet sees the loss, while every coverage obligation attached to it disappears. Record the correction in rfc/short/").Str(req.RFC).
			Str(".md as a paragraph opening 'Correction <YYYY-MM-DD>:', naming `").Str(req.RID).
			Str("` and quoting, in double quotes, at least 24 characters of the RFC sentence that states the lower strength. If the RFC does say MUST, restore the level instead").String())
	}
	return errs
}

func checkNewSummaries(deriver *Deriver, stems, baselineStems, enrolled map[string]bool,
	requirements []Requirement, parseByStem map[string]string, baselineKnown bool) []string {
	if !baselineKnown || len(baselineStems) == 0 {
		return nil
	}
	gated := gatedCounts(requirements)
	var errs []string
	for _, stem := range sortedMissing(stems, baselineStems) {
		if enrolled[stem] {
			continue
		}
		if problem := parseByStem[stem]; problem != "" {
			var tb textbuf.Buffer
			errs = append(errs, tb.Str("rfc/short/").Str(stem).Str(".md is new and does not parse: ").Str(problem).Str(". A new summary must parse before it can be enrolled").String())
			continue
		}
		if gated[stem] > 0 {
			var tb textbuf.Buffer
			errs = append(errs, tb.Str("rfc/short/").Str(stem).Str(".md is new and declares ").Int(int64(gated[stem])).
				Str(" gated MUST-level requirement(s), but is not in rfc/enrolled.txt -- so none of them is checked. Enroll it (see .claude/skills/ze-rfc/SKILL.md), classifying each requirement as tested, {single-polarity}, {gap} or {not-applicable}").String())
			continue
		}
		inventory, err := deriver.Inventory(stem, gated[stem])
		if err != nil || inventory == nil || inventory.KeywordSites == 0 {
			continue
		}
		var tb textbuf.Buffer
		errs = append(errs, tb.Str("rfc/short/").Str(stem).Str(".md is new and declares NO MUST-level requirement, but the source text has ").Int(int64(inventory.KeywordSites)).
			Str(" MUST-level keyword(s) outside any RFC 2119 key-words paragraph this gate recognizes. An absent summary is indistinguishable from a compliant one. Enroll the stem, with the obligations extracted; when those keywords are not requirements on a speaker (a key-words paragraph written \"keywords\", or \"defined in RFC 2119\", is invisible to the exclusion), record that at each site as not-a-requirement in rfc/extraction/").Str(stem).
			Str(".json -- the sign-off enrolling requires anyway").String())
	}
	return errs
}
