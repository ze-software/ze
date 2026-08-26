// Design: docs/architecture/core-design.md -- the six habits, as data
// Overview: ste.go -- the checker these tables serve
//
// These narrow lists come unchanged from ste_check.py. They belong to Ze, not
// the standard. ASD-STE100 part 2 contains approximately 900 approved words and
// is copyright (c) ASD 2025. This repository has no reproduction right. The
// lists implement the six habits for Ze's vocabulary. Reviewers can trust a
// narrow list more than a wide list with false positives.
//
// Each list is an ORDERED slice because order affects two operations. Compiled
// alternatives sort by length, longest first. Python's stable sort preserves
// dictionary order for equal lengths. Thus, declaration order decides which of
// the two eight-character phrases supplies the finding.
package ste

import (
	"regexp"
	"sort"
	"strings"
)

// wordFix contains one list entry: current prose and its replacement.
type wordFix struct {
	Word string
	Fix  string
}

// The lists intentionally contain British spellings and repeated fixes. The
// checker must find `utilise`, `endeavour`, and `neighbour`, so changing these
// entries would remove their detection. Repeated fix text is advice beside its
// word, not a constant that needs indirection.
//
// plainWords is the plain word for an action, and the formal words that mean
// the same. This is the standard's own core discipline: when English gives
// several words for one action, keep the shortest common one. A domain verb
// that no plainer word replaces is deliberately absent, so `implement`,
// `negotiate`, `withdraw` and `advertise` are never flagged. Reported under
// habit 1, because one concept keeping one name is the same rule seen from the
// other side.
//
//nolint:misspell,goconst // the spellings are the detection, and the fixes are a table
var plainWords = []wordFix{
	{"initiate", "start"},
	{"initiates", "starts"},
	{"commence", "start"},
	{"commences", "starts"},
	{"terminate", "stop"},
	{"terminates", "stops"},
	{"cease", "stop"},
	{"ceases", "stops"},
	{"obtain", "get"},
	{"obtains", "gets"},
	{"acquire", "get"},
	{"acquires", "gets"},
	{"utilize", "use"},
	{"utilizes", "uses"},
	{"utilise", "use"},
	{"assist", "help"},
	{"assists", "helps"},
	{"facilitate", "help"},
	{"facilitates", "helps"},
	{"ascertain", "make sure"},
	{"eliminate", "remove"},
	{"eliminates", "removes"},
	{"additional", "more"},
	{"acceptable", "permitted"},
	{"endeavor", "try"},
	{"endeavour", "try"},
	{"prior to", "before"},
	{"subsequent to", "after"},
	{"in order to", "to"},
}

// plainWordIdentifiers holds a plain word that is ALSO a protocol identifier.
// `Cease` is the BGP NOTIFICATION error code of RFC 4271 Section 4.5, so `the
// Cease/MaxPrefixes Data field` names a wire field rather than the verb. The
// capital letter is the tell, and a sentence-initial capital is not, so the
// guard reads what comes before the word.
var plainWordIdentifiers = []string{"Cease"}

// termSet is one concept and the names this repository does NOT use for it.
type termSet struct {
	Canonical string
	Rotations []string
}

var termSets = []termSet{
	{"peer", []string{"neighbour", "neighbor"}}, //nolint:misspell // the rotation is what the checker looks for
	{"plugin", []string{"plug-in"}},
	{"hostname", []string{"host name"}},
	{"filename", []string{"file name"}},
	{"dataplane", []string{"data plane", "data-plane"}},
	{"runtime", []string{"run-time"}},
	{"startup", []string{"start-up"}},
	{"nftables", []string{"nft tables"}},
}

// hedges contains habit 2. In the controlled dictionary, `may` maps to CAN and
// `should` maps to MUST. `usually` is APPROVED, so it replaces `generally` and
// `normally`.
//
//nolint:goconst // a repeated fix is a table of advice, not a constant waiting to be named
var hedges = []wordFix{
	{"may", "CAN for a possibility, MUST for an obligation"},
	{"might", "CAN, or state the condition"},
	{"could", "CAN, or state the condition"},
	{"should", "MUST"},
	{"probably", "state the fact, or say you do not know"},
	{"possibly", "state the fact, or say you do not know"},
	{"perhaps", "state the fact, or say you do not know"},
	{"arguably", "state the fact, or say you do not know"},
	{"seemingly", "state what the code does"},
	{"apparently", "state what the code does"},
	{"presumably", "state what the code does"},
	{"generally", "usually (the approved word)"},
	{"normally", "usually (the approved word)"},
	{"typically", "usually (the approved word)"},
	{"essentially", "delete it, then state the fact"},
	{"basically", "delete it, then state the fact"},
	{"hopefully", "delete it, then state the fact"},
	{"ideally", "state the requirement"},
	{"simply", "delete it"},
	{"easily", "delete it, or give the number"},
	{"somewhat", "give the number"},
	{"relatively", "give the number"},
	{"fairly", "give the number"},
}

// hedgePhrases are the multi-word hedges.
//
//nolint:goconst // as above: the advice belongs beside the phrase it answers
var hedgePhrases = []wordFix{
	{"in most cases", "usually"},
	{"in general", "usually"},
	{"more or less", "give the number"},
	{"it seems", "state what the code does"},
	{"seems to", "state what the code does"},
	{"appears to", "state what the code does"},
	{"we believe", "state the fact, or cite the source"},
	{"we think", "state the fact, or cite the source"},
	{"tend to", "usually"},
	{"tends to", "usually"},
	{"kind of", "delete it"},
	{"sort of", "delete it"},
	{"pretty much", "delete it"},
}

// nominalizations are habit 3: the noun, and the verb to use instead. Only
// nouns whose verb is unambiguous.
var nominalizations = []wordFix{
	{"installation", "install"},
	{"validation", "validate"},
	{"verification", "verify"},
	{"configuration", "configure"},
	{"initialization", "initialize"},
	{"registration", "register"},
	{"allocation", "allocate"},
	{"negotiation", "negotiate"},
	{"activation", "activate"},
	{"deactivation", "deactivate"},
	{"cancellation", "cancel"},
	{"deletion", "delete"},
	{"creation", "create"},
	{"execution", "execute"},
	{"modification", "modify"},
	{"transmission", "transmit"},
	{"distribution", "distribute"},
	{"generation", "generate"},
	{"calculation", "calculate"},
	{"notification", "notify"},
	{"implementation", "implement"},
	{"replacement", "replace"},
	{"movement", "move"},
	{"measurement", "measure"},
	{"removal", "remove"},
	{"comparison", "compare"},
	{"examination", "examine"},
	{"inspection", "inspect"},
	{"selection", "select"},
	{"computation", "compute"},
	{"evaluation", "evaluate"},
	{"migration", "migrate"},
	{"conversion", "convert"},
	{"insertion", "insert"},
	{"extraction", "extract"},
	{"resolution", "resolve"},
	{"submission", "submit"},
}

// marketing are habit 4.
var marketing = []string{
	"powerful", "seamless", "seamlessly", "blazing", "blazingly",
	"lightning-fast", "effortless", "effortlessly", "feature-rich",
	"cutting-edge", "state-of-the-art", "world-class", "best-in-class",
	"battle-tested", "enterprise-grade", "industrial-strength", "bulletproof",
	"rock-solid", "turnkey", "next-generation", "revolutionary",
	"game-changing", "unparalleled", "unmatched", "amazing", "incredible",
	"stunning", "insanely", "super-fast", "ultra-fast", "buttery", "silky",
}

// The run-on bounds, habit 5. STE Rule 5.1 governs a numbered step, Rule 6.3 a
// description, and Rule 6.6 a paragraph.
const (
	maxProcedural            = 20
	maxDescriptive           = 25
	maxSentencesPerParagraph = 6
)

// phrasalVerbs are habit 6: two-word verbs whose meaning is not the sum of
// their parts. `set up` is the verb and `setup` is the noun, so the spaced form
// is always the verb. `shutdown`, `teardown` and `backoff` are technical nouns
// (STE Rules 1.5, 1.6) and stay correct as one word.
var phrasalVerbs = []wordFix{
	{"spin up", "start"},
	{"spun up", "started"},
	{"spins up", "starts"},
	{"kick off", "start"},
	{"kicks off", "starts"},
	{"kicked off", "started"},
	{"figure out", "find, or diagnose"},
	{"figures out", "finds"},
	{"figured out", "found"},
	{"get rid of", "delete, or remove"},
	{"gets rid of", "deletes"},
	{"put out", "extinguish"},
	{"give off", "release"},
	{"gives off", "releases"},
	{"come up with", "write, or select"},
	{"comes up with", "writes"},
	{"look into", "examine, or diagnose"},
	{"looks into", "examines"},
	{"end up", "become, or result in"},
	{"ends up", "becomes"},
	{"sort out", "correct, or resolve"},
	{"work out", "calculate, or resolve"},
	{"take care of", "do, or manage"},
	{"takes care of", "does"},
	{"tear down", "stop, or remove"},
	{"tears down", "stops"},
	{"set up", "install, prepare, or configure"},
	{"sets up", "installs"},
}

// latinAbbreviations are Recommendation 6: a Latin abbreviation confuses a
// reader who does not know it. Reported under hedging, because both habits make
// the reader supply meaning the writer withheld.
var latinAbbreviations = []wordFix{
	{"e.g.", "for example"},
	{"i.e.", "that is"},
	{"etc.", "and so on"},
	{"viz.", "namely"},
	{"cf.", "compare"},
	{"et al.", "and others"},
}

// notGerund contains words with an `-ing` suffix that are not gerunds. The
// gerund pattern accepts any lowercase word with this suffix. An indefinite
// pronoun after one of five prepositions would otherwise look like a gerund
// clause. `string` matters most because Ze often discusses string parsing.
var notGerund = []string{
	"anything", "everything", "nothing", "something",
	"bring", "during", "king", "ring", "spring", "sting", "string", "swing",
	"thing", "wing",
	"ceiling", "evening", "morning",
}

// ─── The lists, as scanners ─────────────────────────────────────────────────

// wordsOf answers the Word field of every entry, in table order.
func wordsOf(list []wordFix) []string {
	out := make([]string, len(list))
	for i, entry := range list {
		out[i] = entry.Word
	}
	return out
}

// fixOf answers the replacement for one matched list entry. These short lists
// use a scan. No second map exists to disagree with the source slice.
func fixOf(list []wordFix, word string) string {
	for _, entry := range list {
		if entry.Word == word {
			return entry.Fix
		}
	}
	return ""
}

var (
	hedgeWordList   = newLiteralList(wordsOf(hedges))
	hedgePhraseList = newLiteralList(wordsOf(hedgePhrases))
	marketingList   = newLiteralList(marketing)
	phrasalList     = newLiteralList(wordsOf(phrasalVerbs))
	plainList       = newLiteralList(wordsOf(plainWords))
	rotationList    = map[string]literalList{}
	canonicalList   = map[string]literalList{}
	lightVerbRe     *regexp.Regexp
	frozenOfRe      *regexp.Regexp
	nominalizedVerb = map[string]string{}
)

func init() {
	for _, set := range termSets {
		canonicalList[set.Canonical] = newLiteralList([]string{set.Canonical})
		for _, rotation := range set.Rotations {
			rotationList[rotation] = newLiteralList([]string{rotation})
		}
	}
	for _, entry := range nominalizations {
		nominalizedVerb[entry.Word] = entry.Fix
	}

	// "Do a check of the battery" is STE, because "check" is not an approved
	// verb (STE Rule 3.7 gives that exact example). So the trigger is never the
	// light verb alone: it is a light verb, or a preposition, bound to a
	// nominalization whose verb IS available.
	nouns := make([]string, 0, len(nominalizations))
	for _, entry := range nominalizations {
		nouns = append(nouns, entry.Word)
	}
	sort.Strings(nouns)
	joined := strings.Join(nouns, "|")

	var light strings.Builder
	light.WriteString(`(?i)(do|does|did|doing|perform|performs|performed|performing|make|makes|made`)
	light.WriteString(`|making|conduct|conducts|conducted|carry out|carries out|carried out){SP}+`)
	light.WriteString(`(?:a|an|the|any|its|their)?{SP}*(`)
	light.WriteString(joined)
	light.WriteString(`)`)
	lightVerbRe = mustPattern(light.String())

	var frozen strings.Builder
	frozen.WriteString(`(?i)(before|after|during|upon){SP}+the{SP}+(`)
	frozen.WriteString(joined)
	frozen.WriteString(`){SP}+of`)
	frozenOfRe = mustPattern(frozen.String())
}
