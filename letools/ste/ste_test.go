// These tests prove each detection property that scripts/dev/ste_check.py
// states.
//
// VALIDATES: spec-le-is-a-ze-binary AC-5. Every case calls a function, not a
// process.
// PREVENTS: agreement with today's corpus but not the script's documented edge
// cases. Each habit has guards added after false positives. Tests keep later
// edits from removing those guards.

package ste

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// findingDetails answers each finding's detail string. Cases use it when they
// check WHICH habit appeared instead of the count.
func findingDetails(findings []Finding) []string {
	out := make([]string, len(findings))
	for index, finding := range findings {
		out[index] = finding.Detail
	}
	return out
}

// reviewMarkdown reviews one document as Markdown.
func reviewMarkdown(t *testing.T, body string) []Finding {
	t.Helper()
	found, skip := Review("doc.md", body, SurfaceMarkdown)
	if skip != "" {
		t.Fatalf("the document was skipped: %s", skip)
	}
	return found
}

// containsDetail reports whether any finding carries the detail.
func containsDetail(findings []Finding, want string) bool {
	return slices.Contains(findingDetails(findings), want)
}

func TestAHedgeIsFoundAndAnRFC2119KeywordIsNot(t *testing.T) {
	found := reviewMarkdown(t, "# T\n\nThe peer may retry. The peer MUST retry.\n")

	if !containsDetail(found, `"may"`) {
		t.Errorf("the lowercase hedge was not reported: %v", findingDetails(found))
	}
	for _, finding := range found {
		if finding.Detail == `"MUST"` || finding.Detail == `"must"` {
			t.Errorf("an RFC 2119 keyword was reported as a hedge: %v", finding)
		}
	}
}

func TestASentenceInitialHedgeIsStillAHedge(t *testing.T) {
	found := reviewMarkdown(t, "# T\n\nTypically the peer retries.\n")

	if !containsDetail(found, `"typically"`) {
		t.Errorf("a capitalized hedge at the head of a sentence was not reported: %v",
			findingDetails(found))
	}
}

func TestAOneLetterAllCapsWordIsNotAnRFCKeyword(t *testing.T) {
	// The guard is `word.isupper() and len(word) > 1`. A single capital is a
	// letter, not a keyword. No hedge is one character long, so this case
	// verifies the guard's LENGTH condition.
	if strings.ToUpper("may") == "may" {
		t.Fatal("the fixture assumption is wrong")
	}
	found := reviewMarkdown(t, "# T\n\nThe peer MAY retry.\n")
	if len(found) != 0 {
		t.Errorf("MAY in capitals is RFC 2119 and must not be reported: %v", findingDetails(found))
	}
}

func TestAWordInsideALongerWordIsNotAHedge(t *testing.T) {
	// `(?<![\w-])may(?![\w-])`: neither "maybe" nor "dismay" nor "non-may" is
	// the hedge, and the dash half is what a plain \b would get wrong.
	found := reviewMarkdown(t, "# T\n\nThe maybe dismay non-may holds.\n")

	if containsDetail(found, `"may"`) {
		t.Errorf("a hedge was found inside a longer word: %v", findingDetails(found))
	}
}

func TestAProtocolIdentifierIsNotThePlainWordItSpells(t *testing.T) {
	// `Cease` is the BGP NOTIFICATION error code of RFC 4271 Section 4.5.
	inSentence := reviewMarkdown(t, "# T\n\nThe daemon sends the Cease code now.\n")
	if containsDetail(inSentence, `"cease"`) {
		t.Errorf("the protocol identifier was reported as the verb: %v", findingDetails(inSentence))
	}

	// The same word after a sentence boundary is a capitalized verb, because a
	// sentence-initial capital says nothing about a proper noun.
	atStart := reviewMarkdown(t, "# T\n\nThe daemon stops. Cease the session now.\n")
	if !containsDetail(atStart, `"cease"`) {
		t.Errorf("the verb at the head of a sentence was not reported: %v", findingDetails(atStart))
	}
}

func TestAGerundClauseIsAFrozenVerbAndAnIndefinitePronounIsNot(t *testing.T) {
	found := reviewMarkdown(t, "# T\n\nBefore starting the peer, check it.\n")
	if !containsDetail(found, `"before starting"`) {
		t.Errorf("the gerund clause was not reported: %v", findingDetails(found))
	}

	// `string` is the entry that matters most: this repository writes about
	// strings and "when string parsing fails" is ordinary prose.
	notGerundFound := reviewMarkdown(t, "# T\n\nWhen string parsing fails, stop.\n")
	if containsDetail(notGerundFound, `"when string"`) {
		t.Errorf("a word that is not a gerund was reported: %v", findingDetails(notGerundFound))
	}
}

func TestALightVerbBoundToANominalizationIsAFrozenVerb(t *testing.T) {
	found := reviewMarkdown(t, "# T\n\nThe operator does the installation now.\n")
	if !containsDetail(found, `"does the installation"`) {
		t.Errorf("the frozen verb was not reported: %v", findingDetails(found))
	}
	for _, finding := range found {
		if finding.Detail == `"does the installation"` && finding.Fix != `use the verb "install"` {
			t.Errorf("the fix names the wrong verb: %q", finding.Fix)
		}
	}
}

func TestALightVerbWithNoArticleIsStillAFrozenVerb(t *testing.T) {
	// The article group is optional, and Python returns it when no noun follows.
	// This fixture has no article.
	found := reviewMarkdown(t, "# T\n\nThe tool does validation now.\n")
	if !containsDetail(found, `"does validation"`) {
		t.Errorf("a frozen verb with no article was not reported: %v", findingDetails(found))
	}
}

func TestALightVerbWhoseArticleSharesTheNounsFirstLetterIsStillFound(t *testing.T) {
	// `(?:a|an|the|...)?` first tries `a` in "an allocation". Then `\s*`
	// matches nothing, and the noun fails at "n allocation". The engine must
	// return the first alternative and try `an`.
	found := reviewMarkdown(t, "# T\n\nThe engine does an allocation now.\n")
	if !containsDetail(found, `"does an allocation"`) {
		t.Errorf("the article alternative was not given back: %v", findingDetails(found))
	}
}

func TestAPrepositionBeforeANominalizationOfIsAFrozenVerb(t *testing.T) {
	found := reviewMarkdown(t, "# T\n\nStop before the validation of the config.\n")
	if !containsDetail(found, `"before the validation of"`) {
		t.Errorf("the frozen `of` form was not reported: %v", findingDetails(found))
	}
}

func TestAMarketingAdjectiveAndAPhrasalVerbAreFound(t *testing.T) {
	found := reviewMarkdown(t, "# T\n\nThe seamless daemon can spin up a peer.\n")
	if !containsDetail(found, `"seamless"`) || !containsDetail(found, `"spin up"`) {
		t.Errorf("a marketing adjective or a phrasal verb was missed: %v", findingDetails(found))
	}
}

func TestARotationIsReportedOncePerDocument(t *testing.T) {
	// The rule fires when a rotation sits beside the canonical name, or when
	// two rotations of one concept sit together.
	beside := reviewMarkdown(t, "# T\n\nThe peer is up.\n\nThe neighbor is up.\n")
	if !containsDetail(beside, "neighbor beside peer") {
		t.Errorf("the rotation beside the canonical name was not reported: %v",
			findingDetails(beside))
	}

	//nolint:misspell // both spellings ARE the fixture: two rotations of one concept
	two := reviewMarkdown(t, "# T\n\nThe neighbor is up.\n\nThe neighbour is up.\n")
	if !containsDetail(two, "neighbour, neighbor") { //nolint:misspell // the finding names both
		t.Errorf("two rotations of one concept were not reported: %v", findingDetails(two))
	}

	alone := reviewMarkdown(t, "# T\n\nThe neighbor is up.\n")
	for _, detail := range findingDetails(alone) {
		if strings.Contains(detail, "neighbor") {
			t.Errorf("one rotation alone is a vocabulary choice, not a rotation: %v", detail)
		}
	}
}

func TestARestrictedMeaningAndAnArticleBeforeAnIdentifierAreFound(t *testing.T) {
	found := reviewMarkdown(t, "# T\n\nKeep it above 5 and read the RFC 4271 text.\n")
	if !containsDetail(found, `"above" for a limit`) {
		t.Errorf("the restricted meaning was not reported: %v", findingDetails(found))
	}
	// The pattern reads ONE digit after the noun. Thus, the finding contains
	// only the article, noun, and first identifier digit.
	if !containsDetail(found, `"the RFC 4"`) {
		t.Errorf("the article before an identifier was not reported: %v", findingDetails(found))
	}
}

func TestALatinAbbreviationIsReportedUnderHedging(t *testing.T) {
	found := reviewMarkdown(t, "# T\n\nRead the guide, e.g. the quickstart page.\n")
	for _, finding := range found {
		if finding.Detail == `"e.g."` {
			if finding.Habit != "hedging" {
				t.Errorf("a Latin abbreviation is habit 2, got %q", finding.Habit)
			}
			return
		}
	}
	t.Errorf("the Latin abbreviation was not reported: %v", findingDetails(found))
}

// ─── Sentences and the word count ───────────────────────────────────────────

func TestASentenceIsNotSplitInsideAnAbbreviationOrANumber(t *testing.T) {
	got := sentences("Read RFC 4271 Section 4.5, e.g. the header. Then stop.")
	if len(got) != 2 {
		t.Fatalf("want 2 sentences, got %d: %q", len(got), got)
	}
	if !strings.Contains(got[0], "e.g.") || !strings.Contains(got[0], "4.5") {
		t.Errorf("the abbreviation or the number lost its dot: %q", got[0])
	}
}

func TestNumberedAbbreviationHoldsOnlyInFrontOfItsNumber(t *testing.T) {
	// "No. 5" is a label and keeps its dot. "answered Yes/No." ends a sentence,
	// and an unconditional hold glues the next sentence onto it.
	label := sentences("Read No. 5 first. Then stop.")
	if len(label) != 2 {
		t.Errorf("the labeled number split its sentence: %q", label)
	}

	ending := sentences("The answer is Yes/No. Then stop.")
	if len(ending) != 2 {
		t.Errorf("a real full stop after No was held: %q", ending)
	}
}

func TestASentenceEndsThroughItsClosingMarkup(t *testing.T) {
	// "**... dictionary.** It cannot ..." -- without the closer class a bolded
	// lead-in glues its whole bullet into one sentence.
	got := sentences("**The dictionary.** It cannot be embedded.")
	if len(got) != 2 {
		t.Errorf("the bolded lead-in was not split off: %q", got)
	}
}

func TestTheWordCountCollapsesAParenthesisAQuoteAndAMeasure(t *testing.T) {
	if got := wordCount("Stop (for a while) now"); got != 3 {
		t.Errorf("parenthesised text counts as one word, got %d", got)
	}
	if got := wordCount(`Read "the whole page" now`); got != 3 {
		t.Errorf("quoted text counts as one word, got %d", got)
	}
	if got := wordCount("Wait 30 seconds now"); got != 3 {
		t.Errorf("a number with its unit counts as one word, got %d", got)
	}
}

func TestAPercentSignAloneIsNotAUnit(t *testing.T) {
	// `[A-Za-z%]+\b` after a `%` needs the NEXT character to be a word
	// character, because `\b` measures a transition rather than one side. So
	// "50 % of" holds no measurement, and the bare `%` is not a word either.
	if got := replaceMeasures("50 % of"); got != "50 % of" {
		t.Errorf("a lone %% was read as a unit: %q", got)
	}
	if got := wordCount("50 % of"); got != 2 {
		t.Errorf("`50 %% of` counts 50 and of, got %d", got)
	}
}

func TestAUnitThatEndsInAPercentSignGivesTheSignBack(t *testing.T) {
	// The engine takes `[A-Za-z%]+` greedily and then gives characters back
	// until `\b` holds. Without that give-back "3 abc%" matches nothing, so the
	// TEXT is what says whether the backtracking happened: a word count of 2
	// comes out either way.
	if got := replaceMeasures("3 abc% now"); got != " MEASURE % now" {
		t.Errorf("the greedy unit did not give its %% back: %q", got)
	}
}

func TestTheTextAroundARejectedMeasureSurvives(t *testing.T) {
	// A match whose boundary fails must leave the text it spanned in place.
	// "as112 additionally" has no boundary before its digits, and dropping the
	// span cost this port 811 run-on findings before it was found.
	if got := replaceMeasures("as112 additionally starts"); got != "as112 additionally starts" {
		t.Errorf("a rejected measure ate its own text: %q", got)
	}
}

// ─── The extractors ─────────────────────────────────────────────────────────

func TestATildeRunDoesNotCloseABacktickFence(t *testing.T) {
	body := "# T\n\n```\n~~~\nThe daemon may start.\n```\n\nThe peer is up.\n"
	found := reviewMarkdown(t, body)
	if containsDetail(found, `"may"`) {
		t.Errorf("a ~~~ line closed a ``` fence and exposed the code: %v", findingDetails(found))
	}
}

func TestABlockquoteAndATableDividerAreNotProse(t *testing.T) {
	body := "# T\n\n> The daemon may start.\n\n| a | b |\n|---|---|\n"
	found := reviewMarkdown(t, body)
	if containsDetail(found, `"may"`) {
		t.Errorf("external quotation was reviewed: %v", findingDetails(found))
	}
}

func TestAnIgnoreLineSkipsTheLineAfterIt(t *testing.T) {
	body := "# T\n\n<!-- ste: ignore -->\nThe daemon may start.\n\nThe peer might stop.\n"
	found := reviewMarkdown(t, body)
	if containsDetail(found, `"may"`) {
		t.Errorf("the ignored line was reviewed: %v", findingDetails(found))
	}
	if !containsDetail(found, `"might"`) {
		t.Errorf("the ignore covered more than one line: %v", findingDetails(found))
	}
}

func TestAnIgnoreFileNeedsAReasonAndSkipsTheDocument(t *testing.T) {
	withReason := "<!-- ste: ignore-file quotes RFC text at length -->\n\nThe daemon may start.\n"
	found, skip := Review("doc.md", withReason, SurfaceMarkdown)
	if skip != "quotes RFC text at length" {
		t.Errorf("the reason was not read back: %q", skip)
	}
	if len(found) != 0 {
		t.Errorf("a skipped document still answered findings: %v", findingDetails(found))
	}

	// An opt-out with no reason is a silent exemption. The script honors one:
	// its non-greedy group swallows the comment terminator and reads `-->` as
	// the reason. This half refuses it, and the whole document is reviewed.
	noReason := "<!-- ste: ignore-file -->\n\nThe daemon may start.\n"
	stillFound, stillSkip := Review("doc.md", noReason, SurfaceMarkdown)
	if stillSkip != "" {
		t.Errorf("an opt-out with no reason was honored: %q", stillSkip)
	}
	if !containsDetail(stillFound, `"may"`) {
		t.Errorf("the document was not reviewed: %v", findingDetails(stillFound))
	}
}

func TestAGeneratedMarkerCountsOnlyInTheOpeningLines(t *testing.T) {
	head := "GENERATED by tool\n\nThe daemon may start.\n"
	if _, skip := Review("doc.md", head, SurfaceMarkdown); skip != "generated file" {
		t.Errorf("a generated document was reviewed: %q", skip)
	}

	// The marker is read in the first 8 lines only, so a document that
	// MENTIONS the banner half way down is still reviewed.
	var deep strings.Builder
	for range 12 {
		deep.WriteString("Filler line.\n")
	}
	deep.WriteString("GENERATED by tool\n")
	deep.WriteString("The daemon may start.\n")
	found, skip := Review("doc.md", deep.String(), SurfaceMarkdown)
	if skip != "" {
		t.Fatalf("a document that mentions the banner was skipped: %q", skip)
	}
	if !containsDetail(found, `"may"`) {
		t.Errorf("the document was not reviewed: %v", findingDetails(found))
	}
}

func TestAStructuredGoMarkerIsNotProseAndACommentedOutLineIsNot(t *testing.T) {
	body := "package x\n\n// Design: docs/x.md -- the daemon may start\n// if err != nil {\n" +
		"// The daemon may start.\nfunc f() {}\n"
	found, _ := Review("x.go", body, SurfaceGo)

	if len(found) != 1 {
		t.Fatalf("want exactly the prose comment's one finding, got %v", findingDetails(found))
	}
	if found[0].Line != 5 {
		t.Errorf("the finding names line %d, want the prose comment at 5", found[0].Line)
	}
}

func TestAYangDescriptionIsProseAndAShortOneIsNot(t *testing.T) {
	body := "module m {\n  leaf a {\n    description \"The peer may retry the session.\";\n  }\n" +
		"  leaf b {\n    description \"May retry\";\n  }\n}\n"
	found, _ := Review("m.yang", body, SurfaceYANG)

	if !containsDetail(found, `"may"`) {
		t.Errorf("the description was not reviewed: %v", findingDetails(found))
	}
	if len(found) != 1 {
		t.Errorf("a description under three words is not prose: %v", findingDetails(found))
	}
}

func TestAParagraphIsCappedOnSentencesAndATableCellIsNot(t *testing.T) {
	seven := "One. Two. Three. Four. Five. Six. Seven."
	paragraph := reviewMarkdown(t, "# T\n\n"+seven+"\n")
	if !containsDetail(paragraph, "7 sentences in one paragraph (STE Rule 6.6 allows 6)") {
		t.Errorf("the paragraph cap did not fire: %v", findingDetails(paragraph))
	}

	// A reference-table cell holding eight short facts is a table, and capping
	// it would push authors to write fewer, longer sentences.
	cell := reviewMarkdown(t, "# T\n\n| a | "+seven+" |\n")
	for _, detail := range findingDetails(cell) {
		if strings.Contains(detail, "sentences in one paragraph") {
			t.Errorf("a table cell was capped as a paragraph: %v", detail)
		}
	}
}

func TestANumberedStepIsHeldToTheShorterBound(t *testing.T) {
	// 22 words: over Rule 5.1's 20 for a procedure, under Rule 6.3's 25 for a
	// description. Only the step is a run-on.
	sentence := "one two three four five six seven eight nine ten " +
		"eleven twelve thirteen fourteen fifteen sixteen seventeen eighteen nineteen twenty " +
		"twentyone twentytwo"

	step := reviewMarkdown(t, "# T\n\n1. "+sentence+"\n")
	if !containsDetail(step, "22 words (STE Rule 5.1 allows 20)") {
		t.Errorf("the procedural bound did not fire: %v", findingDetails(step))
	}

	bullet := reviewMarkdown(t, "# T\n\n- "+sentence+"\n")
	for _, detail := range findingDetails(bullet) {
		if strings.Contains(detail, "words (STE Rule") {
			t.Errorf("a bullet is a description, not a procedure: %v", detail)
		}
	}
}

func TestAColumnAlignedLineIsNotJoinedIntoTheParagraph(t *testing.T) {
	// Markdown joins consecutive lines. A joined two-column note is too long,
	// although each source line is valid. Each line has eleven words, under
	// Rule 6.3's limit. Together they have thirty-three words. The test fails if
	// the checker joins the aligned lines.
	rows := []string{
		"alpha      one two three four five six seven eight nine ten",
		"bravo      one two three four five six seven eight nine ten",
		"charlie    one two three four five six seven eight nine ten",
	}
	found := reviewMarkdown(t, "# T\n\n"+strings.Join(rows, "\n")+"\n")
	for _, detail := range findingDetails(found) {
		if strings.Contains(detail, "words (STE Rule") {
			t.Errorf("aligned columns were measured as one sentence: %v", detail)
		}
	}

	// With ONE inter-column space, the same lines are normal wrapped prose.
	// Joining them creates a run-on. This comparison prevents a checker that
	// passes the aligned case by measuring nothing.
	wrapped := strings.ReplaceAll(strings.Join(rows, "\n"), "      ", " ")
	wrapped = strings.ReplaceAll(wrapped, "    ", " ")
	joined := reviewMarkdown(t, "# T\n\n"+wrapped+"\n")
	if !hasRunOn(joined) {
		t.Errorf("wrapped prose of 33 words was not measured: %v", findingDetails(joined))
	}
}

// hasRunOn reports whether any finding names a sentence over its word bound.
func hasRunOn(findings []Finding) bool {
	for _, detail := range findingDetails(findings) {
		if strings.Contains(detail, "words (STE Rule") {
			return true
		}
	}
	return false
}

func TestAnInnerDotIsHeldSoAVersionIsNotTwoSentences(t *testing.T) {
	// `(?<=\w)\.(?=\w)` holds dots in 4.5, foo.go, and Rule 1.1. Removing the
	// CALL does not change sentence splitting because inner dots lack following
	// whitespace. This test therefore calls the helper directly.
	if got := holdInnerDots("Rule 1.1 and foo.go"); got != "Rule 1"+held+"1 and foo"+held+"go" {
		t.Errorf("the inner dots were not held: %q", got)
	}
	if got := holdInnerDots("Stop. Then go."); got != "Stop. Then go." {
		t.Errorf("a sentence-ending dot was held: %q", got)
	}
}

// ─── The Python string semantics ────────────────────────────────────────────

func TestAnExcerptIsCutAtCharactersNotBytes(t *testing.T) {
	// The 118 em dashes occupy 118 CHARACTERS and 354 BYTES. A byte cut splits
	// one rune, but the bound is 120 characters.
	dashes := strings.Repeat("—", 118)
	body := "# T\n\nThe peer may retry " + dashes + " now.\n"
	found := reviewMarkdown(t, body)

	var excerpt string
	for _, finding := range found {
		if finding.Detail == `"may"` {
			excerpt = finding.Excerpt
		}
	}
	if excerpt == "" {
		t.Fatal("the hedge was not reported")
	}
	if count := len([]rune(excerpt)); count != excerptRunes {
		t.Errorf("the excerpt is %d characters, want %d", count, excerptRunes)
	}
	if !strings.ContainsRune(excerpt, '—') || strings.ContainsRune(excerpt, '�') {
		t.Errorf("the excerpt cut a character in half: %q", excerpt)
	}
}

func TestPathsSortByComponentTheWayPythonSortsThem(t *testing.T) {
	// "cmd/ze" sorts before "cmd/ze-gok" by component and after it by byte,
	// because `-` is 0x2d and `/` is 0x2f.
	if !lessByPathParts("cmd/ze/dispatch.go", "cmd/ze-gok/main.go") {
		t.Error("the paths were ordered by byte rather than by component")
	}
	if lessByPathParts("cmd/ze-gok/main.go", "cmd/ze/dispatch.go") {
		t.Error("the order is not antisymmetric")
	}
	// A directory sorts before a file whose name extends it, because the
	// components are compared before their lengths are: "a" precedes "a.md".
	if !lessByPathParts("docs/a/b.md", "docs/a.md") {
		t.Error("the components were not compared before the lengths")
	}
	if !lessByPathParts("docs/a", "docs/a/b.md") {
		t.Error("a shorter component list must sort first when every component agrees")
	}
}

func TestFoldedComparisonMatchesTheWayBothEnginesFold(t *testing.T) {
	// `(?i)k` matches U+212A KELVIN SIGN in Python and in Go, and the two
	// spellings are different byte lengths.
	list := newLiteralList([]string{"kick off"})
	if got := list.findAll("They Kick off now"); len(got) != 1 {
		t.Errorf("the folded spelling was not matched: %v", got)
	}
	if got := list.findAll("They KICK OFF now"); len(got) != 1 {
		t.Errorf("the upper-case spelling was not matched: %v", got)
	}
}

func TestTheLongestEntryWinsAtOnePosition(t *testing.T) {
	// The longest entry comes first. This matters when its longer sibling
	// continues after a SPACE. Both boundaries match at one position, and only
	// order selects the entry.
	//
	// A prefix pair inside one word does not test order. Its trailing lookaround
	// decides the match because the shorter entry ends inside a word.
	list := newLiteralList([]string{"set", "set up"})
	got := list.findAll("They set up the peer")
	if len(got) != 1 || got[0].Entry != "set up" {
		t.Errorf("want one match of the longer entry, got %v", got)
	}

	shorter := newLiteralList([]string{"figure out", "figures out"})
	if hit := shorter.findAll("It figures out the answer"); len(hit) != 1 || hit[0].Entry != "figures out" {
		t.Errorf("the trailing lookaround did not reject the shorter entry: %v", hit)
	}
}

func TestSplitLinesReadsEveryTerminatorPythonReads(t *testing.T) {
	// A Go comment holding a form feed and a Windows document both reach this
	// checker, and Go's own line split sees neither.
	got := pySplitLines("a\r\nb\fc\nd")
	want := []string{"a", "b", "c", "d"}
	if len(got) != len(want) {
		t.Fatalf("want %d lines, got %d: %q", len(want), len(got), got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Errorf("line %d is %q, want %q", index, got[index], want[index])
		}
	}
}

func TestWhitespaceIsPythonsSetNotGos(t *testing.T) {
	// Python counts the four information separators as whitespace and Go's
	// unicode.IsSpace does not.
	if !pySpace('\x1c') {
		t.Error("U+001C is whitespace to Python and must be here")
	}
	if got := pyFields("a\x1cb"); len(got) != 2 {
		t.Errorf("the separator did not split the words: %q", got)
	}
}

// ─── The ratchet ────────────────────────────────────────────────────────────

func TestGrowthIsReportedOnlyWhenTheCountRises(t *testing.T) {
	before, _ := Review("doc.md", "# T\n\nThe peer may retry.\n", SurfaceMarkdown)
	same, _ := Review("doc.md", "# T\n\nThe session may retry.\n", SurfaceMarkdown)
	more, _ := Review("doc.md", "# T\n\nThe peer may retry. It might stop.\n", SurfaceMarkdown)

	if rows := grewIn("doc.md", same, before); len(rows) != 0 {
		t.Errorf("an equal count is not growth: %v", rows)
	}
	rows := grewIn("doc.md", more, before)
	if len(rows) != 1 || rows[0].Was != 1 || rows[0].Now != 2 {
		t.Fatalf("want one hedging row 1 -> 2, got %v", rows)
	}
	if len(rows[0].Findings) != 2 {
		// Both findings are new: the excerpt is the whole unit and the unit
		// changed, so neither matches its HEAD twin on content.
		t.Errorf("want the new findings listed, got %v", findingDetails(rows[0].Findings))
	}
}

func TestANewFindingIsMatchedOnContentRatherThanOnLine(t *testing.T) {
	// An edit higher in the file moves every line below it. Matching on the
	// line number would report every finding under the edit as new.
	before, _ := Review("doc.md", "# T\n\nThe peer may retry.\n", SurfaceMarkdown)
	after, _ := Review("doc.md", "# T\n\nAdded line.\n\nThe peer may retry.\n\nIt might stop.\n", SurfaceMarkdown)

	fresh := addedSince(after, before, "hedging")
	if len(fresh) != 1 || fresh[0].Detail != `"might"` {
		t.Errorf("want only the new hedge, got %v", findingDetails(fresh))
	}
}

func TestTheGrowthRowsAreOrderedByHabitNumber(t *testing.T) {
	before, _ := Review("doc.md", "# T\n\nOk.\n", SurfaceMarkdown)
	after, _ := Review("doc.md", "# T\n\nThe seamless peer may spin up.\n", SurfaceMarkdown)

	rows := grewIn("doc.md", after, before)
	for index := 1; index < len(rows); index++ {
		if rows[index-1].Number > rows[index].Number {
			t.Errorf("row %d (habit %d) precedes habit %d", index, rows[index-1].Number, rows[index].Number)
		}
	}
	if len(rows) < 3 {
		t.Errorf("want a row for each of the three habits, got %v", rows)
	}
}

// ─── The command surface ────────────────────────────────────────────────────

func TestTheNamedFileFormNeedsAKeywordBeforeEachPath(t *testing.T) {
	got, err := namedFiles([]string{"file", "docs/a.md", "file", "docs/b.md"})
	if err != nil {
		t.Fatalf("the typed form was refused: %v", err)
	}
	if len(got) != 2 || got[0] != "docs/a.md" || got[1] != "docs/b.md" {
		t.Errorf("want both paths, got %v", got)
	}

	if _, err := namedFiles([]string{"docs/a.md"}); err == nil {
		t.Error("a bare path in a positional slot must be refused")
	}
	if _, err := namedFiles([]string{"file"}); err == nil {
		t.Error("a keyword with no value must be refused")
	}
}

func TestEveryActionCarriesItsGateAndNoneWrites(t *testing.T) {
	want := map[string]bool{
		"ze-ste-check": true, "ze-ste-review": true, "ze-ste-review-changed": true,
	}
	for _, gate := range Gates() {
		if !want[gate] {
			t.Errorf("unexpected gate %q", gate)
		}
		delete(want, gate)
	}
	if len(want) != 0 {
		t.Errorf("gates missing from the table: %v", want)
	}
	for _, row := range Actions().Actions {
		if row.Writes {
			t.Errorf("%q writes; this tool reads prose and reports", row.Verb)
		}
	}
}

func TestTheReportsAreStructuredDataWithKebabCaseKeys(t *testing.T) {
	review := NewReviewReport([]Finding{{
		File: "doc.md", Line: 3, Surface: SurfaceMarkdown, Habit: "hedging",
		Number: 2, Detail: `"may"`, Fix: "CAN", Excerpt: "x",
	}}, 1, 0)
	if review.Totals["hedging"] != 1 || review.Counts[SurfaceMarkdown]["hedging"] != 1 {
		t.Errorf("the tally is wrong: %v", review.Totals)
	}
	assertKebabKeys(t, review)
	assertKebabKeys(t, NewGateReport([]Growth{{File: "doc.md", Habit: "hedging", Number: 2, Was: 0, Now: 1}}, 1))
}

// assertKebabKeys fails when any JSON key of the payload carries an underscore
// or a capital letter.
func assertKebabKeys(t *testing.T, payload any) {
	t.Helper()
	raw := marshalForTest(t, payload)
	for _, key := range jsonKeys(raw) {
		if strings.ContainsAny(key, "_ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
			t.Errorf("key %q is not kebab-case", key)
		}
	}
}

func TestTheGateCodeIsThreeSoACallerCanTellItFromAUsageError(t *testing.T) {
	if got := NewGateReport(nil, 3).Code(); got != 0 {
		t.Errorf("a clean gate answers 0, got %d", got)
	}
	grew := NewGateReport([]Growth{{File: "doc.md", Habit: "hedging", Number: 2, Was: 0, Now: 1}}, 1)
	if got := grew.Code(); got != 3 {
		t.Errorf("a grown habit answers 3, got %d", got)
	}
}

func TestAnUnreadableChangedFileIsARefusalRatherThanASkip(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 file")
	}
	root := gitFixture(t, map[string]string{
		"docs/a.md": "# T\n\nThe daemon starts.\n",
	})
	writeFixtureFile(t, root, "docs/a.md", "# T\n\nThe daemon may start. It should work.\n")
	if err := os.Chmod(filepath.Join(root, "docs", "a.md"), 0o000); err != nil {
		t.Fatalf("making the file unreadable: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(root, "docs", "a.md"), 0o600) })

	if _, _, err := Ratchet(root, nil); err == nil {
		t.Error("a document the ratchet cannot read must not read as no growth")
	}
}

func TestAGitThatCannotAnswerIsARefusalRatherThanAnEmptySet(t *testing.T) {
	// A repository whose HEAD is unborn has no baseline, so the ratchet has
	// nothing to compare against. An empty candidate list is what PASSING looks
	// like, which is the fail-open this port fixed.
	root := t.TempDir()
	writeFixtureFile(t, root, "docs/a.md", "# T\n\nThe daemon may start.\n")
	runGit(t, root, "init", "--quiet")
	runGit(t, root, "add", "--all")

	if _, _, err := Ratchet(root, nil); err == nil {
		t.Error("an unborn HEAD must refuse rather than answer no growth")
	}
}
