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
		errs = append(errs, "nothing is enrolled: no summary under rfc/short/ declares `| Enrolment | enrolled |`. The gate refuses to report clean while enforcing nothing (ai/rules/evidence.md)")
	}
	for _, rfc := range sortedMissing(baseline, current) {
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(rfc).Str(" was un-enrolled. Enrolment is monotonic: an RFC whose MUSTs were gated cannot stop being gated. Restore `| Enrolment | enrolled |` in its summary's Meta table").String())
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
	requirements []Requirement, parseByStem map[string]string, baselineKnown bool,
	metas map[string]Meta) []string {
	if !baselineKnown || len(baselineStems) == 0 {
		return nil
	}
	gated := gatedCounts(requirements)
	var errs []string
	for _, stem := range sortedMissing(stems, baselineStems) {
		if enrolled[stem] {
			continue
		}
		// An out-of-scope summary is the one un-enrolled shape that is
		// allowed to declare gated MUSTs. It declares them precisely so
		// they are not lost: the extraction is done and the FEATURE is
		// what the owner declined, so demanding enrolment here would
		// pressure the next author to delete the checklist instead, which
		// is the failure checkRetiredRequirements exists to prevent.
		// checkOutOfScope holds the other half: such a summary may not
		// claim public support.
		if metas[stem].OutOfScope() {
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
				Str(" gated MUST-level requirement(s), but does not declare `| Enrolment | enrolled |` -- so none of them is checked. Enroll it in its own Meta table (see .claude/skills/ze-rfc/SKILL.md), classifying each requirement as tested, {single-polarity}, {gap}, {lower-layer}, {feature-declined} or {not-applicable}").String())
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

// discriminationInput is everything the ninth ratchet compares: what the tree
// proves, what it claims, and what HEAD held of both.
//
// A struct rather than eight parameters, in the shape auditFreshnessInput
// already has: the ratchet reads five sets and a caller passing them
// positionally can swap two maps of the same type with nothing to catch it.
type discriminationInput struct {
	Verdicts     []DiscriminationVerdict
	Requirements []Requirement
	// Gated is the requirement ids a tag can owe a proof of: MUST-level, and
	// declared by an enrolled RFC.
	Gated map[string]bool
	// Covers is every tagged unit in the working tree, with the tags that sit
	// in it.
	Covers map[Cover][]Tag
	// HeadCovers is that same set at the tip commit, carrying the tags so a
	// violation can name a file and a line. The obligation is computed FROM it
	// and never from the working tree, so one session's uncommitted tag is
	// nobody else's violation.
	HeadCovers map[Cover][]Tag
	// PriorCovers is the cover set of the commit BEFORE the tip, and PriorKnown
	// is false where git could not answer. A tag the tip commit added against
	// it owes its proof in that commit.
	PriorCovers map[Cover]bool
	PriorKnown  bool
	// BacklogCovers is the cover set of the pushed branch and BacklogRef names
	// it, empty when that ref does not resolve. The unproven backlog is
	// MEASURED against it and never billed.
	BacklogCovers map[Cover]bool
	BacklogRef    string
	// HeadRecords is the recorded set at HEAD, and HeadKnown is false when git
	// could not answer, where the monotonic branch judges nothing.
	HeadRecords map[Cover]bool
	HeadKnown   bool
	// Carriers decides which proof route a new tag's violation names.
	Carriers []Carrier
	// Sources reads the working tree and Index resolves a unit key inside one
	// text, so a stale record can be compared with what HEAD holds for the same
	// file and a tagged unit can be read at both revisions.
	Sources *sourceReader
	Index   *scopeIndex
	// HeadSources reads the HEAD text of every file a record fingerprints, and
	// HeadBlobsKnown is false when git could not answer, where the stale branch
	// judges nothing. A reader rather than the map, because the stale branch
	// resolves a fingerprint key inside that text and resolveKeyText takes the
	// same lookup for either revision.
	HeadSources    *sourceReader
	HeadBlobsKnown bool
	// HeadTagBlobs is the HEAD text of every file carrying a tag at HEAD. It is
	// read by the changed-unit MEASUREMENT, which reports and never refuses.
	HeadTagBlobs map[string]string
}

// checkDiscriminationRatchet judges the recorded proofs against the corpus.
//
// Five refusals. One names a requirement no summary declares, so it proves an
// obligation nobody has written down. One claims a tagged unit a second record
// already claims, and a duplicate is not cosmetic: the proven count is
// published, and two rows for one unit inflate it. The third is the one that
// makes the artifact worth having -- a record whose stored fingerprints no
// longer match COMMITTED code. Nothing observed the red over the code that was
// committed, so the record is a claim rather than a proof, and a hand-written
// record is refused by the same rule that catches a real drift.
//
// HEAD decides that one, not the working tree (owner decision, 2026-08-31). A
// record staled by an edit nobody has committed is REPORTED instead: several
// sessions share this checkout, one session's uncommitted edit to a producer
// would otherwise red the gate for all of them, and clearing an interop record
// costs a 576-second re-record. The author still meets the violation, at their
// own commit, where the drift is theirs. It is never counted as proven in the
// meantime, because an unverified verdict is counted by nothing.
//
// The last two are the obligation itself. A tagged unit the TIP COMMIT added
// owes its proof in that commit, and a record that was committed and has been
// deleted while its tag stands takes back a proof the ledger published. Both
// compare two committed revisions and both judge nothing where git cannot
// answer, which is what keeps the ratchet silent on work that touched no tag
// and on every other session's uncommitted edits.
//
// A record whose TAG is gone is none of these. It is REMOVABLE: the record dies
// with the tag it proves, so there is nothing left for it to be wrong about,
// and check.go reports it rather than refusing it.
//
// Records arrive in file order, so the FIRST offending row is the one named.
func checkDiscriminationRatchet(in discriminationInput) []string {
	declared := make(map[string]bool, len(in.Requirements))
	for _, req := range in.Requirements {
		declared[req.RID] = true
	}

	claimed := make(map[Cover]string, len(in.Verdicts))
	recorded := make(map[Cover]bool, len(in.Verdicts))
	var errs []string
	// Indexed rather than ranged by value: a verdict carries a whole record, and
	// the linter counts the copy at every iteration.
	for index := range in.Verdicts {
		verdict := &in.Verdicts[index]
		record := &verdict.Record
		recorded[record.Cover()] = true
		var tb textbuf.Buffer
		tb.Str(record.Source).Str(": the record for ").Str(record.RID).Byte(' ').Str(record.Polarity).
			Str(" at ").Str(record.Unit)
		if !declared[record.RID] {
			errs = append(errs, tb.Str(" names a requirement no summary in rfc/short/ declares. A proof of an obligation nobody wrote down proves nothing, so the record is refused rather than counted").String())
			continue
		}
		key := record.Cover()
		if first, held := claimed[key]; held {
			errs = append(errs, tb.Str(" repeats the claim ").Str(first).
				Str(" already carries. Each requirement, polarity and tagged unit is proven once; a second row inflates the published proven count without proving anything twice").String())
			continue
		}
		claimed[key] = record.Source
		if verdict.removable() {
			continue
		}
		if !verdict.Verified() && driftIsCommitted(in, verdict) {
			errs = append(errs, discriminationStaleError(verdict))
		}
	}

	errs = append(errs, discriminationOwedErrors(in)...)
	return append(errs, discriminationWithdrawnErrors(in, recorded)...)
}

// owedAgainst answers the tags the tip commit added against one baseline and
// has not proven.
//
// Two baselines are asked of the same predicate: the commit before the tip,
// which is the obligation, and the pushed branch, which is the published
// backlog. A second predicate for the measurement would let the number drift
// from the rule it measures.
func (in discriminationInput) owedAgainst(baseline map[Cover]bool, known bool) []Tag {
	return discriminationOwedTags(in.HeadCovers, baseline, known, in.Verdicts, in.Gated)
}

// discriminationOwed answers the tags the TIP COMMIT owes a proof for.
func discriminationOwed(in discriminationInput) []Tag {
	return in.owedAgainst(in.PriorCovers, in.PriorKnown)
}

// discriminationBacklog counts the tagged units the unpushed commits added
// without proving, and answers nil when the branch it measures against was not
// read.
//
// A MEASUREMENT and never a violation. The unpushed set on this checkout is
// hundreds of commits deep, its tags were added by sessions that have finished,
// and nobody can clear it inside the change in hand: billing it is the R-8
// failure at the scale that gets a ratchet removed rather than obeyed. The
// obligation stays on the tip commit, which is the one change its author can
// still record a proof in, and this line says how much sits behind it.
//
// An unresolvable ref answers nil, and the report prints no line rather than a
// figure taken against nothing.
func discriminationBacklog(in discriminationInput) *int {
	if in.BacklogRef == "" {
		return nil
	}
	count := len(in.owedAgainst(in.BacklogCovers, true))
	return &count
}

// discriminationOwedErrors refuses a tagged unit the tip commit added and did
// not prove.
//
// The violation names the file and the line so the author can open it, and the
// proof route its carrier kind admits so the next command is obvious. A
// generated break reaches unit tests only, which is why the route a functional
// or an interop tag gets is the recorded revert rather than the mutant.
func discriminationOwedErrors(in discriminationInput) []string {
	owed := discriminationOwed(in)
	errs := make([]string, 0, len(owed))
	for _, tag := range owed {
		carrier, carried := CarrierFor(tag.File, in.Carriers)
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(tag.File).Byte(':').Int(int64(tag.Line)).
			Str(": the tag for ").Str(tag.RID).Byte(' ').Str(tag.Polarity).
			Str(" was added by the commit under test and carries no discrimination proof. ").
			Str("A tag states what its test demonstrates and nothing reads that sentence, so ").
			Str("the proof is a recorded break ").
			Str("under which this tagged unit itself goes red. Record one with `./le rfc discriminate-record` ").
			Str("by the ").Str(proofRouteFor(carrier, carried)).String())
	}
	return errs
}

// proofRouteFor names the route one carrier kind admits, and says why.
func proofRouteFor(carrier Carrier, carried bool) string {
	if carried && carrier.Kind != kindUnit {
		var tb textbuf.Buffer
		return tb.Str(RouteRevert).Str(" route with a citation, because gomu mutates unit tests only and no generated break reaches a ").
			Str(carrier.Kind).Str(" carrier").String()
	}
	var tb textbuf.Buffer
	return tb.Str(RouteMutant).Str(" route from a gomu report, or by the ").Str(RouteRevert).
		Str(" route naming a producer to disable").String()
}

// discriminationWithdrawnErrors refuses the deletion of a committed proof whose
// tag is still in the tree.
//
// The proven set is monotonic. A record deleted beside the tag it proved is the
// orphan case and is legal; a record deleted while the tag stands takes a proof
// off the published ledger and leaves the claim behind it.
func discriminationWithdrawnErrors(in discriminationInput, recorded map[Cover]bool) []string {
	if !in.HeadKnown {
		return nil
	}
	var withdrawn []Cover
	for key := range in.HeadRecords {
		if recorded[key] || len(in.Covers[key]) == 0 {
			continue
		}
		withdrawn = append(withdrawn, key)
	}
	sort.Slice(withdrawn, func(left, right int) bool {
		if withdrawn[left].Unit != withdrawn[right].Unit {
			return withdrawn[left].Unit < withdrawn[right].Unit
		}
		if withdrawn[left].RID != withdrawn[right].RID {
			return withdrawn[left].RID < withdrawn[right].RID
		}
		return withdrawn[left].Polarity < withdrawn[right].Polarity
	})

	errs := make([]string, 0, len(withdrawn))
	for _, key := range withdrawn {
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(discriminationRel).Str(": the record for ").Str(key.RID).
			Byte(' ').Str(key.Polarity).Str(" at ").Str(key.Unit).
			Str(" was committed and is gone from the tree while that tag is still there. The proven ").
			Str("set only goes up: a record dies with the tag it proves, so deleting one beside a ").
			Str("standing tag takes a proof off the published ledger and leaves the claim behind it. ").
			Str("Restore the record, or delete the tag it proved").String())
	}
	return errs
}

// discriminationStaleError says which fingerprint moved and what was recorded
// against it.
//
// It names the break and the producer as well as the unit, because the reader's
// next action is to judge whether that break still discriminates the claim, and
// re-running it is what `./le rfc discriminate` does. The gate itself replays
// nothing.
func discriminationStaleError(verdict *DiscriminationVerdict) string {
	record := &verdict.Record
	var tb textbuf.Buffer
	tb.Str(record.Source).Str(": the ").Str(record.Route).Str(" record for ").Str(record.RID).
		Byte(' ').Str(record.Polarity).Str(" at ").Str(record.Unit).
		Str(" no longer verifies (").Str(verdict.State).Str("): ").Str(verdict.Detail)
	if record.Proves() {
		tb.Str(". The break ").Quoted(record.Break).Str(" was observed to redden that unit ").
			Str("when applied to ").Str(record.Producer)
	}
	return tb.Str(". `./le rfc check` replays nothing: it compares the fingerprints the ").
		Str("proof was taken against, so a moved fingerprint means the red was never ").
		Str("re-observed over the code that is there now. Re-record it with ").
		Str("`./le rfc discriminate`, or delete the record with the tag it proved").String()
}

// driftedKey answers which fingerprint key of a non-verifying record moved.
//
// The producer for a record whose PRODUCER moved, and the tagged unit for
// everything else. An escape that names no producer -- foreign-producer, whose
// facts are the carrier kind and the citation -- is judged over its unit for the
// same reason: that is the text its check reads.
func driftedKey(record *DiscriminationRecord, state string) string {
	if state == ProofProducerGone || state == ProofProducerChanged {
		return record.Producer
	}
	if state == ProofEscapeUnfounded && record.Producer != "" {
		return record.Producer
	}
	return record.Unit
}

// driftIsCommitted answers whether a stale record's drift is in the COMMIT
// rather than in somebody's working tree.
//
// The comparison is the record's OWN key at both revisions, resolved by
// resolveKeyText and normalized by behaviorBytes, which is what the fingerprint
// it went stale against was taken over. A unit whose behavior at HEAD equals its
// behavior here explains no drift, so the record was already wrong about the
// commit. One that differs carries an uncommitted edit, and that edit is what
// staled the record.
//
// The key and not its FILE. A record fingerprints one function, so comparing
// whole files answers a wider question than the record asks: any unrelated
// uncommitted edit anywhere else in the producer's file made a genuinely
// committed drift look like somebody's working copy, and the violation the
// author owed was downgraded to a report. discriminationChangedUnits already
// resolves the key for the same reason.
//
// Where git cannot answer, nothing is committed as far as this gate can tell,
// and it judges nothing -- the rule every other branch of this ratchet follows.
// A key that does not parse is the same answer: the record is refused elsewhere
// for that, and guessing here would red the tree over somebody else's edit.
func driftIsCommitted(in discriminationInput, verdict *DiscriminationVerdict) bool {
	if !in.HeadBlobsKnown || in.HeadSources == nil || in.Sources == nil || in.Index == nil {
		return false
	}
	record := &verdict.Record
	key := driftedKey(record, verdict.State)
	rel := keyFile(key)
	was, atHead, err := resolveKeyText(in.HeadSources.read, in.Index, key, record.Source)
	if err != nil {
		return false
	}
	now, here, err := resolveKeyText(in.Sources.read, in.Index, key, record.Source)
	if err != nil {
		return false
	}
	if !atHead {
		// Absent at HEAD and absent here: no working-tree edit explains the
		// drift, so the commit does. Absent at HEAD and present here is a unit
		// this session added, which is uncommitted by definition.
		return !here
	}
	if !here {
		return false
	}
	return behaviorSHA(rel, was) == behaviorSHA(rel, now)
}

// discriminationDrifted names the records an UNCOMMITTED edit contradicts.
//
// Reported rather than refused, and never counted as proven: the tree holds code
// the stored red was not observed over, so the record answers nothing right now,
// and saying so is what keeps an unreliable observation from reading as a proof
// (ai/rules/principles.md).
func discriminationDrifted(in discriminationInput) []string {
	var out []string
	for index := range in.Verdicts {
		verdict := &in.Verdicts[index]
		if verdict.Verified() || verdict.removable() || driftIsCommitted(in, verdict) {
			continue
		}
		var tb textbuf.Buffer
		out = append(out, tb.Str(verdict.Record.RID).Byte(' ').Str(verdict.Record.Polarity).
			Str(" at ").Str(verdict.Record.Unit).Str(" (").Str(verdict.State).Byte(')').String())
	}
	return out
}

// discriminationChangedUnits MEASURES the tagged units whose behavior changed
// since HEAD and that carry no verifying record.
//
// The spec answers "which tags owe a proof" twice. R-2 reads it wide: a tag
// added since HEAD, OR a tagged unit whose behavior changed. AC-3 reads it
// narrow, because its violation names "the stale record", which only a unit that
// already has one can have. Phase 4 implemented the narrow one, so a
// grandfathered tagged test can be gutted today and nothing bills it.
//
// The owner's decision of 2026-08-31 is to DETECT the wide set and PUBLISH it,
// and to enforce nothing yet. The count is what says whether enforcing it is
// affordable, and a ratchet that reds the tree over a backlog nobody has
// measured gets removed rather than obeyed. Nothing here is a violation and the
// exit code is untouched.
//
// ChangedTags is the predicate rather than a second one, so a comment-only,
// whitespace-only or Go import-only edit answers nothing. The population is the
// narrow obligation's own: a GATED requirement of an enrolled RFC, on a unit
// that HEAD already carried.
//
// The second answer is the units the walk could NOT read at both revisions, and
// it is published beside the first. 3,802 of the in-scope tags key on a
// function rather than on a whole file, so the resolver is what almost every
// unit goes through, and a resolver that answered nothing would report zero
// changed units over a corpus it never opened. A scan that cannot see is not a
// clean corpus (ai/rules/principles.md).
func discriminationChangedUnits(in discriminationInput) (changed, unresolved int) {
	if len(in.HeadTagBlobs) == 0 || in.Sources == nil {
		return 0, 0
	}
	head := newTextReader(in.HeadTagBlobs)
	proven := provenCovers(in.Verdicts)
	for key := range in.Covers {
		if proven[key] || len(in.HeadCovers[key]) == 0 || !in.Gated[key.RID] {
			continue
		}
		rel := keyFile(key.Unit)
		was, atHead, err := resolveKeyText(head.read, in.Index, key.Unit, rel)
		if err != nil || !atHead {
			unresolved++
			continue
		}
		now, here, err := resolveKeyText(in.Sources.read, in.Index, key.Unit, rel)
		if err != nil || !here {
			unresolved++
			continue
		}
		if len(ChangedTags(rel, was, now)) > 0 {
			changed++
		}
	}
	return changed, unresolved
}
