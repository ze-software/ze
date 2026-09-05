// Design: docs/architecture/core-design.md -- judging a sign-off against the source
// Overview: rfc.go -- the types, the paths and the closed sets every reader here shares
//
// signoff.go compares one reviewer's walk against the RFC's own text, freshly
// re-derived.
//
// The comparison runs in both directions and each catches a different failure.
// FORWARD catches a MISSED obligation: every derived site is mapped or excluded,
// and every derived field still matches what the source re-derives. REVERSE
// catches an INVENTED one: every gated requirement of the summary is the target
// of some site, or declared unsourced on some section.
//
// Only an artifact with ZERO violations counts as signed. A stale or
// contradicted sign-off must not keep earning drain credit while the basis
// under it has moved.
package rfc

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/spec/specpath"
)

// The Markdown constructs that make text stop being a live claim.
//
// Each falls back to consuming the rest of the input when its terminator is
// missing, so an unbalanced marker strips MORE rather than less. That direction
// is deliberate: FINDING the id is what passes the tripwire, so stripping too
// little fails OPEN and leaves an obligation owed by nobody, while stripping
// too much reddens a gate that names the file and the id for a human to look at.
var (
	mdCommentRE      = regexp.MustCompile(`(?s)<!--.*?-->|<!--.*`)
	mdIndentedCodeRE = regexp.MustCompile(`(?m)^(?: {4}|\t)[^\n]*$`)
	mdStrikeRE       = regexp.MustCompile(`(?m)~~.+?~~|~~.*$`)
	// fenceOpenRE matches a fenced-code opener, both syntaxes. CommonMark
	// accepts three or more backticks or tildes, indented up to three spaces.
	fenceOpenRE = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})")
)

// stripFences removes every fenced code block, closed or not.
//
// Go's RE2 has no backreference and the Python pattern needs one: the closing
// fence must repeat the OPENER, character for character and at least as long.
// So the block is found by scanning lines, and the opener's length is tried
// from longest down to three -- which is the backtracking the Python engine
// does when a five-backtick opener is closed by a three-backtick line.
func stripFences(text string) string {
	lines := strings.Split(text, "\n")
	var out []string
	for i := 0; i < len(lines); i++ {
		found := fenceOpenRE.FindStringSubmatch(lines[i])
		if found == nil {
			out = append(out, lines[i])
			continue
		}
		run := found[1]
		closer := -1
		for size := len(run); size >= 3 && closer < 0; size-- {
			want := run[:size]
			for j := i + 1; j < len(lines); j++ {
				trimmed := strings.TrimLeft(lines[j], " ")
				if len(lines[j])-len(trimmed) <= 3 && strings.HasPrefix(trimmed, want) {
					closer = j
					break
				}
			}
		}
		if closer < 0 {
			// No closer anywhere: the second alternative consumes the rest of
			// the input, which is the strip-more direction stated above.
			out = append(out, " ")
			return strings.Join(out, "\n")
		}
		out = append(out, " ")
		i = closer
	}
	return strings.Join(out, "\n")
}

// liveReservationText answers the part of a destination spec that can RESERVE
// an obligation.
//
// A relocation asserts that a named spec OWES the requirement. Searching the
// raw file answers a weaker question -- do these characters appear anywhere --
// and the two differ exactly where it matters. Commenting the row out, striking
// it through, or leaving the id in a shell example all keep the tripwire green
// while the spec has stopped claiming the work.
//
// Order matters twice. Comments go first, because a comment can hold a fence or
// a strike marker that would otherwise pair with a live one further down.
// Fences go before strikethrough, because three tildes open a fence and two
// open a strike, and the shorter rule applied first splits the delimiter and
// leaves the block behind.
//
// WHAT THIS IS NOT. It is a line-oriented approximation of Markdown, not a
// parser, and the residue falls on both sides. It strips MORE than a renderer
// hides: any four-space line is read as code, so a nested list item stops
// reserving and the gate reddens naming the file. It strips LESS for constructs
// it does not know: an HTML strike tag disowns text and is not removed. The
// known-less cases are the ones worth watching, because that direction leaves
// an obligation owed by nobody.
func liveReservationText(text string) string {
	text = mdCommentRE.ReplaceAllString(text, " ")
	text = stripFences(text)
	text = mdIndentedCodeRE.ReplaceAllString(text, " ")
	return mdStrikeRE.ReplaceAllString(text, " ")
}

// reservesID reports whether text names rid as a WHOLE id, never as a
// substring.
//
// RFC7296-2.19-2 is a prefix of RFC7296-2.19-25, and a neighboring ordinal is
// exactly what a renumbering produces -- so a substring test would let the
// WRONG row keep the tripwire green, which is a fail-open in the one direction
// that matters.
func reservesID(text, rid string) bool {
	for offset := 0; ; {
		at := strings.Index(text[offset:], rid)
		if at < 0 {
			return false
		}
		start := offset + at
		end := start + len(rid)
		if !isIDNeighbourBefore(text, start) && !isIDNeighbourAfter(text, end) {
			return true
		}
		offset = start + 1
	}
}

func isIDNeighbourBefore(text string, start int) bool {
	if start == 0 {
		return false
	}
	char := text[start-1]
	return isWordByte(char) || char == '.' || char == '-'
}

func isIDNeighbourAfter(text string, end int) bool {
	if end >= len(text) {
		return false
	}
	char := text[end]
	return isWordByte(char) || char == '-'
}

func isWordByte(char byte) bool {
	return char == '_' || (char >= '0' && char <= '9') ||
		(char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')
}

// relocationErrors is the tripwire under relocated-to-spec. Every arm DENIES.
//
// The kind's whole claim is that a named spec owes the obligation under a
// reserved id. Nothing about that claim is checkable a year later unless the
// gate re-reads it, so it does: delete the spec, or edit the row out of it, and
// this reds naming the site. When the spec CLOSES the file is removed and this
// reds too, which is correct rather than unfortunate: the obligation has landed
// in the summary by then, so the site is a mapping now.
func relocationErrors(tree string, art Extraction, site ExtractionSite,
	knownIDs map[string]bool, cache map[string]*string) []string {
	var where textbuf.Buffer
	place := where.Str(art.Path).Str(": site ").Str(site.ID).String()
	rel, rid := site.RelocatedTo, site.ReservedID

	if knownIDs[rid] {
		var tb textbuf.Buffer
		return []string{tb.Str(place).Str(" is relocated to ").Str(rel).Str(" reserving ").
			Str(rid).Str(", but rfc/short/").Str(art.Stem).Str(".md still declares ").
			Str(rid).Str(". A relocation asserts the row left this summary: while it is ").
			Str("here, the site is a mapping and must say so, or the obligation is ").
			Str("counted as homed elsewhere while it is also claimed here").String()}
	}

	if _, seen := cache[rel]; !seen {
		// The artifact names a bucket path, and a spec relocated between
		// buckets since it was signed off still exists: resolve the NAME so
		// the pointer is judged on whether the spec is there, not on whether
		// the bucket in the artifact is still the one holding it.
		relative, findErr := specpath.Find(tree, filepath.Base(rel))
		var raw []byte
		var err error
		if findErr != nil {
			err = findErr
		} else {
			raw, err = os.ReadFile(treePath(tree, relative)) // #nosec G304 -- the spec path specpath resolved
		}
		if err != nil {
			cache[rel] = nil
		} else {
			text := replaceInvalidUTF8(string(raw))
			cache[rel] = &text
		}
	}
	text := cache[rel]

	if text == nil {
		var tb textbuf.Buffer
		return []string{tb.Str(place).Str(" is relocated to ").Str(rel).
			Str(", which does not exist or cannot be read. A relocation is a pointer, ").
			Str("not a dismissal: ").Str(rid).Str(" is owed by that spec, so the spec ").
			Str("must be there. If the work landed, map the site to its row in ").
			Str("rfc/short/").Str(art.Stem).Str(".md instead").String()}
	}
	if !reservesID(liveReservationText(*text), rid) {
		var tb textbuf.Buffer
		return []string{tb.Str(place).Str(" reserves ").Str(rid).Str(" in ").Str(rel).
			Str(", and that file no longer names ").Str(rid).
			Str(" in live prose. The obligation is now owed by nobody: put the row ").
			Str("back, or map the site if the requirement has landed in rfc/short/").
			Str(art.Stem).Str(".md. An id inside an HTML comment, a strikethrough span, ").
			Str("a fenced block or an indented code block does not reserve it -- the ").
			Str("first two are how this repository DISOWNS text (ai/rules/planning.md: ").
			Str("superseded spec content is struck through), and the last two are ").
			Str("examples rather than claims").String()}
	}
	return nil
}

// evaluateExtraction judges ONE sign-off against the freshly re-derived
// inventory.
func evaluateExtraction(tree string, art Extraction, inv *Inventory, reqs []Requirement) []string {
	var errs []string
	where := art.Path

	// The census leads, because the per-site detail below it is one line per
	// unclassified sentence and a long RFC has hundreds. A reader who meets
	// that wall learns that something is wrong; this line names the file, the
	// count, and the two moves that end it.
	if sites, sections := art.Unclassified(); sites+sections > 0 {
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(where).Str(": ").Int(int64(sites)).Str(" of ").
			Int(int64(len(art.Sites))).Str(" site(s) and ").Int(int64(sections)).
			Str(" of ").Int(int64(len(art.Sections))).
			Str(" section(s) are UNCLASSIFIED. A generated skeleton is not a sign-off, ").
			Str("and one unclassified artifact reds this gate for the whole corpus. ").
			Str("Classify every one by hand, or move the file back into this session's ").
			Str("scratch until the walk is done: ./le rfc extraction-create stem ").
			Str(art.Stem).Str(" writes an unclassified skeleton to the scratch rather ").
			Str("than to rfc/extraction/, for exactly this reason").String())
	}

	if art.SignedOff == "" {
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(where).
			Str(": 'signed-off' is empty. A skeleton is not a sign-off: record the date ").
			Str("the walk was performed once every site and section is classified").String())
	}
	if art.Reviewer == "" {
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(where).
			Str(": 'reviewer' is empty. A sign-off names who performed the walk").String())
	}
	if art.Register == registerManualWalk && art.RegisterReason == "" {
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(where).
			Str(": a manual-walk sign-off needs a 'register-reason' stating why no ").
			Str("mechanical inventory exists for this source. The gate cannot verify a ").
			Str("manual walk, so the assertion must at least say what it rests on").String())
	}

	if art.SourceSHA != inv.SourceSHA {
		// One accurate error, not a wall. With the source moved, every site and
		// every section would mismatch too, and the only useful message is this
		// one: a false stale costs a re-read, a false fresh ships an unbounded
		// summary.
		var tb textbuf.Buffer
		return append(errs, tb.Str(where).Str(": source-sha no longer matches ").
			Str(inv.SourcePath).Str(". The source text changed under this sign-off, so ").
			Str("the walk no longer bounds what the summary missed. Re-run: ./le rfc ").
			Str("extraction-create stem ").Str(art.Stem).
			Str(", re-classify any site that moved, and bump signed-off").String())
	}
	if art.SourcePath != inv.SourcePath {
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(where).Str(": source-path ").Str(pyRepr(art.SourcePath)).
			Str(" is not where the source text lives (").Str(pyRepr(inv.SourcePath)).
			Str(")").String())
	}
	if registerStrength[art.Register] > registerStrength[inv.Register] {
		gated := 0
		for _, req := range reqs {
			if req.Gated() {
				gated++
			}
		}
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(where).Str(": register ").Str(pyRepr(art.Register)).
			Str(" is STRONGER than the source supports (").Str(pyRepr(inv.Register)).
			Str(": ").Int(int64(inv.KeywordSites)).Str(" capitalised keyword site(s) ").
			Str("against ").Int(int64(gated)).Str(" gated requirement(s) declared). The ").
			Str("register is a property of the source, not a claim the signer may ").
			Str("assert. Sign under ").Str(pyRepr(inv.Register)).Str(" or weaker").String())
	}

	derivedSites := make(map[string]Site, len(inv.Sites))
	for _, site := range inv.Sites {
		derivedSites[site.ID] = site
	}
	artSites := make(map[string]ExtractionSite, len(art.Sites))
	for _, site := range art.Sites {
		artSites[site.ID] = site
	}
	for _, id := range sortedMissing(derivedSites, artSites) {
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(where).Str(": derived site ").Str(id).
			Str(" is absent from the sign-off (").Str(pyRepr(firstRunes(derivedSites[id].Quote, 80))).
			Str("). Re-run ./le rfc extraction-create stem ").Str(art.Stem).
			Str(" and classify it").String())
	}
	for _, id := range sortedMissing(artSites, derivedSites) {
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(where).Str(": site ").Str(id).
			Str(" is not in the derived inventory. Sites are DERIVED from the source ").
			Str("text; a hand-added locator classifies nothing").String())
	}

	byID := make(map[string]Requirement, len(reqs))
	knownIDs := make(map[string]bool, len(reqs))
	for _, req := range reqs {
		byID[req.RID] = req
		knownIDs[req.RID] = true
	}
	mappedTargets := map[string]bool{}
	for _, site := range art.Sites {
		if site.Disposition == DispositionMapped && site.MappedTo != "" {
			mappedTargets[site.MappedTo] = true
		}
	}
	// One read per destination spec, however many sites point at it.
	specCache := map[string]*string{}

	for _, id := range sortedShared(derivedSites, artSites) {
		site, derived := artSites[id], derivedSites[id]
		if site.Quote != derived.Quote {
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(where).Str(": site ").Str(id).
				Str(" quote does not match the source. Derived: ").
				Str(pyRepr(firstRunes(derived.Quote, 90))).Str(". Recorded: ").
				Str(pyRepr(firstRunes(site.Quote, 90))).
				Str(". The quote is a DERIVED field; editing it hides what the reviewer ").
				Str("is meant to judge").String())
		}
		if site.Disposition == "" {
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(where).Str(": site ").Str(id).Str(" is UNCLASSIFIED: ").
				Str(pyRepr(firstRunes(derived.Quote, 110))).
				Str(". Every derived site is mapped to a requirement id or excluded ").
				Str("with a reason").String())
			continue
		}
		if site.MappedTo != "" && !knownIDs[site.MappedTo] {
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(where).Str(": site ").Str(id).Str(" names ").
				Str(site.MappedTo).Str(", which does not exist in rfc/short/").
				Str(art.Stem).Str(".md").String())
			continue // every check below reads the requirement it names
		}

		switch {
		case site.Disposition == DispositionMapped:
			// The site's LEVEL against the row's. Both facts were already here
			// and neither was compared: a sentence quoting a capitalised MUST
			// could be mapped to a SHOULD row and reported as captured, while
			// a SHOULD never gates -- so the obligation was bound by nobody.
			//
			// Only a CAPITALISED keyword triggers it, and the DERIVED quote is
			// what is read. A prose site's lowercase modal asserts nothing
			// about level.
			req := byID[site.MappedTo]
			if siteKeywordRE.MatchString(derived.Quote) && !req.Gated() {
				var tb textbuf.Buffer
				errs = append(errs, tb.Str(where).Str(": site ").Str(id).
					Str(" quotes a MUST-level keyword but maps to ").Str(site.MappedTo).
					Str(" [").Str(req.Level).Str("], which is advisory and never gates: ").
					Str(pyRepr(firstRunes(derived.Quote, 90))).
					Str(". Either the summary row understates the source and its level ").
					Str("is wrong, or this site belongs to a different row -- an ").
					Str("obligation recorded as captured but proven by nothing is the ").
					Str("miss this sign-off exists to make impossible").String())
			}
		case site.ExcludedKind == exclusionDuplicate:
			if !mappedTargets[site.MappedTo] {
				var tb textbuf.Buffer
				errs = append(errs, tb.Str(where).Str(": site ").Str(id).
					Str(" is excluded duplicate-of ").Str(site.MappedTo).
					Str(", but no other site MAPS that id. A chain of duplicates cannot ").
					Str("cover an RFC in which nothing is actually mapped").String())
			}
		case site.ExcludedKind == relocatedToSpec:
			errs = append(errs, relocationErrors(tree, art, site, knownIDs, specCache)...)
		}
	}

	derivedSections := make(map[string]int, len(inv.Sections))
	for _, section := range inv.Sections {
		derivedSections[section.ID] = section.Sites
	}
	artSections := make(map[string]ExtractionSection, len(art.Sections))
	for _, section := range art.Sections {
		artSections[section.ID] = section
	}
	for _, id := range sortedMissing(derivedSections, artSections) {
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(where).Str(": derived section ").Str(id).
			Str(" is absent from the sign-off").String())
	}
	for _, id := range sortedMissing(artSections, derivedSections) {
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(where).Str(": section ").Str(id).
			Str(" is not in the derived section list").String())
	}
	for _, id := range sortedShared(derivedSections, artSections) {
		section := artSections[id]
		if section.Sites != derivedSections[id] {
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(where).Str(": section ").Str(id).Str(" records ").
				Int(int64(section.Sites)).Str(" site(s); the source derives ").
				Int(int64(derivedSections[id])).Str(". The count is a DERIVED field").String())
		}
		if section.Disposition == "" {
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(where).Str(": section ").Str(id).
				Str(" is UNCLASSIFIED. Every section of the source is walked, or ").
				Str("skipped with a kind and a reason").String())
		}
	}

	unsourced := map[string]bool{}
	for _, section := range art.Sections {
		for _, id := range section.UnsourcedIDs {
			unsourced[id] = true
		}
	}
	for _, id := range sortedSet(unsourced) {
		if knownIDs[id] {
			continue
		}
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(where).Str(": unsourced-ids names ").Str(id).
			Str(", which does not exist in rfc/short/").Str(art.Stem).Str(".md").String())
	}
	for _, req := range reqs {
		if !req.Gated() || mappedTargets[req.RID] || unsourced[req.RID] {
			continue
		}
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(where).Str(": ").Str(req.RID).Str(" [").Str(req.Level).
			Str("] is declared by rfc/short/").Str(art.Stem).
			Str(".md but no source site maps to it and no section lists it in ").
			Str("unsourced-ids: ").Str(firstRunes(req.Text, 70)).
			Str(". Either it is backed by a site the walk should map, or it was read ").
			Str("from indicative prose -- say which").String())
	}
	return errs
}

// sortedMissing answers the keys of left that right does not hold, sorted.
func sortedMissing[L any, R any](left map[string]L, right map[string]R) []string {
	var out []string
	for key := range left {
		if _, held := right[key]; !held {
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}

// sortedShared answers the keys both maps hold, sorted.
func sortedShared[L any, R any](left map[string]L, right map[string]R) []string {
	var out []string
	for key := range left {
		if _, held := right[key]; held {
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}

// evaluateExtractions answers the valid sign-offs by stem, and every violation.
func evaluateExtractions(deriver *Deriver, requirements []Requirement) (map[string]Extraction, []string, error) {
	byRFC := map[string][]Requirement{}
	for _, req := range requirements {
		byRFC[req.RFC] = append(byRFC[req.RFC], req)
	}
	artifacts, err := LoadExtractions(deriver.Tree())
	if err != nil {
		return nil, nil, err
	}
	gated := gatedCounts(requirements)
	signed := map[string]Extraction{}
	var errs []string
	for _, stem := range sortedKeysOf(artifacts) {
		art := artifacts[stem]
		inv, err := deriver.Inventory(stem, gated[stem])
		if err != nil {
			return nil, nil, err
		}
		if inv == nil {
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(art.Path).Str(": ").Str(stem).
				Str(" has no source text at rfc/full/").Str(stem).Str(".txt or rfc/drafts/").
				Str(stem).Str(".txt, so the sign-off cannot be re-derived and the bound ").
				Str("it claims cannot be re-checked").String())
			continue
		}
		found := evaluateExtraction(deriver.Tree(), art, inv, byRFC[stem])
		errs = append(errs, found...)
		if len(found) == 0 {
			signed[stem] = art
		}
	}
	return signed, errs, nil
}

func sortedKeysOf[V any](in map[string]V) []string {
	out := make([]string, 0, len(in))
	for key := range in {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

// credited answers the sign-offs that COUNT: the ones whose stem is enrolled.
//
// Every published figure and every comparison is derived from this, never from
// the valid set directly, so credit and backlog always describe the same set.
// They did not: the drain floor compared every valid artifact against a backlog
// of enrolled ones, so a sign-off for a stem nobody enrolled raised the credit
// without lowering the backlog.
//
// Signing BEFORE enrolling stays the normal workflow, because the sign-off is a
// precondition of enrolment. Such an artifact is still parsed and still
// ratcheted, and starts counting the moment its stem enrolls.
func credited(signed map[string]Extraction, enrolled map[string]bool) map[string]Extraction {
	out := map[string]Extraction{}
	for stem := range signed {
		if enrolled[stem] {
			out[stem] = signed[stem]
		}
	}
	return out
}

// uncredited answers the valid sign-offs credited() left out, sorted.
//
// Such a walk is complete and correct, and credited() is right to keep it out
// of the totals: credit and backlog must describe one set. What was missing is
// the REMAINDER. Nothing named it, so a finished walk sat in rfc/extraction/
// counting toward nothing and told nobody -- rfc1035 was signed off on
// 2026-07-30 and appeared in no published figure for a month. Publishing the
// set beside the totals is what stops that, without moving an arithmetic that
// is correct.
func uncredited(signed map[string]Extraction, enrolled map[string]bool) []string {
	out := make([]string, 0)
	for stem := range signed {
		if !enrolled[stem] {
			out = append(out, stem)
		}
	}
	sort.Strings(out)
	return out
}

// registerCounts answers the split with EVERY register present even at zero. A
// register missing from the split reads as "not a thing" rather than as "zero",
// and the split is the counterweight that keeps the credit half honest.
func registerCounts(signed map[string]Extraction) map[string]int {
	counts := map[string]int{}
	for _, name := range Registers() {
		counts[name] = 0
	}
	for stem := range signed {
		counts[signed[stem].Register]++
	}
	return counts
}
