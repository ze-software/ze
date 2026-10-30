// Design: docs/architecture/core-design.md -- the RFC conformance gate, as one command
// Detail: summary.go -- the registry of obligations, read off rfc/short
// Detail: tags.go -- the tests that prove them, read off the three test roots
// Detail: carriers.go -- whether anything executes a test, and what that is worth
// Detail: pytokens.go -- where a scenario check's comments are, without Python
// Detail: inventory.go -- the derived walk over an RFC's own text
// Detail: artifact.go -- the authored sign-off over that walk
// Detail: signoff.go -- judging the one against the other
// Detail: goscope.go -- the tagged unit, the text one tag governs
// Detail: audit.go -- the recorded verdicts, and the schema they satisfy
// Detail: discriminate.go -- the recorded break one tag's unit was seen to fail under
// Detail: freshness.go -- which of those verdicts is still current
// Detail: reseal.go -- the one writer of rfc/audit/
// Detail: status.go -- the envelope the drain quota consumes
// Detail: ledger.go -- the three authored pages the generated ones quote back
// Detail: coverage.go -- the counting half, with no markup in it at all
// Detail: render.go -- the two generated pages, and write.go -- the one writer of them
// Detail: actions.go -- the command surface, and register.go -- how le finds it
// Detail: pyfmt.go -- the Python spellings a message carries
//
// Package rfc binds every MUST-level requirement of an enrolled RFC to the
// tests that enforce it, and carries the ratchets in
// ai/rules/rfc-compliance.md.
//
// Messages that are part of the established contract use the Python spellings
// implemented in pyfmt.go rather than Go's %q.
package rfc

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	specpath "github.com/ze-software/ze/internal/le/spec/path"
)

// area is the word a developer types, and the prefix leaction removes from
// every gate name in the table. `ze-rfc-check` becomes `le rfc check`.
const area = "rfc"

// The tree-relative locations this gate reads. Slash-separated, joined through
// filepath.FromSlash at each use, so one spelling serves the reader and the
// message that names the file.
const (
	summaryRel  = "rfc/short"
	enrolledRel = "rfc/enrolled.txt"
	// graphRel is the dependency graph, one row per in-corpus edge, rendered
	// by graph.Graph.TSV from the RFC texts.
	graphRel          = "rfc/graph.tsv"
	extractionRel     = "rfc/extraction"
	discriminationRel = "rfc/discrimination"
	fullRel           = "rfc/full"
	draftsRel         = "rfc/drafts"
	// referenceRel holds non-normative IETF documents that are never a
	// requirement source; referenceReadmeRel states that rule.
	referenceRel       = "reference"
	referenceReadmeRel = "reference/README.md"
)

// specDirNames lists current spec buckets for derived-feed invalidation.
// Artifact recognition uses specpath.HistoricalDirs separately, because signed
// records retain their original paths when specs move between buckets.
func specDirNames() []string { return specpath.Dirs() }

// testRoots are the source trees that can contain requirement tags.
var testRoots = [...]string{"cmd", "internal", "pkg", "test"}

// The RFC 2119 levels, each spelling declared once: gatedLevels and
// advisoryLevels sort them into the two figures, levelForce (quote_backfill.go)
// ranks them, and bcp14Keywords (notapplicable_roles.go) reads them out of
// both sets. levelMust is the gated keyword a checklist row carries most often,
// and the one the fixtures and the ratchet messages spell.
const (
	levelMust           = "MUST"
	levelMustNot        = "MUST NOT"
	levelShall          = "SHALL"
	levelShallNot       = "SHALL NOT"
	levelRequired       = "REQUIRED"
	levelShould         = "SHOULD"
	levelShouldNot      = "SHOULD NOT"
	levelRecommended    = "RECOMMENDED"
	levelNotRecommended = "NOT RECOMMENDED"
	levelMay            = "MAY"
	levelOptional       = "OPTIONAL"
)

// The RFC 2119 keywords that create an obligation the gate enforces.
// SHOULD/MAY are listed in the ledger and may be tagged, but never gate.
var gatedLevels = map[string]bool{
	levelMust:     true,
	levelMustNot:  true,
	levelShall:    true,
	levelShallNot: true,
	levelRequired: true,
}

// IsGatedLevel reports whether an RFC 2119 keyword creates an obligation this
// repository's gates enforce.
//
// It is exported because internal/le/test/health partitions the published ledger
// on the same set and must cover exactly the rows the ledger's totals cover. It
// kept its own copy of these five keywords until 2026-08-29, under a comment
// saying it "mirrors the RFC gate's own gated set" -- a stated obligation to
// agree with nothing enforcing it, which is the shape that let the commit
// gate's structural-stage list drift out of the verifier's.
func IsGatedLevel(level string) bool { return gatedLevels[level] }

var advisoryLevels = map[string]bool{
	levelShould:         true,
	levelShouldNot:      true,
	levelMay:            true,
	levelRecommended:    true,
	levelNotRecommended: true,
	levelOptional:       true,
}

// gatedLevelNames answers the MUST-level keyword set, sorted. The fixture pins
// it by VALUE rather than by count, which is the only thing that
// kills a one-word mutation in a set an output comparison never prints.
func gatedLevelNames() []string { return sortedKeys(gatedLevels) }

// advisoryLevelNames answers the SHOULD-level keyword set, sorted.
func advisoryLevelNames() []string { return sortedKeys(advisoryLevels) }

// MandatoryKeywords answers the keywords whose rows count in the mandatory
// figure, sorted. It is exported so the site's profile cards name the set the
// figure counts rather than a copy of it.
func MandatoryKeywords() []string { return gatedLevelNames() }

// OptionalKeywords answers the keywords whose rows count in the optional
// figure, sorted: every level a row may carry that does not gate.
func OptionalKeywords() []string { return advisoryLevelNames() }

// The two directions a tag can prove: that the code does what the requirement
// demands, and that it refuses what the requirement forbids.
const (
	PolarityPositive = "positive"
	PolarityNegative = "negative"
)

// polarities are the two directions a tag can prove.
var polarities = map[string]bool{PolarityPositive: true, PolarityNegative: true}

// Polarities answers them sorted, for the value comparison.
func Polarities() []string { return sortedKeys(polarities) }

// How a discrimination record was produced: the break was generated by a
// mutation operator, the break was a producer function disabled by hand, or no
// break exists and the record says so.
//
// A closed set, because free text lets "there is nothing to break here" read as
// "this tag is proven". RouteNoBreak is the ESCAPE and is counted apart from
// the two proof routes wherever a count is published.
const (
	RouteMutant  = "mutant"
	RouteRevert  = "revert"
	RouteNoBreak = "no-break"
)

var discriminationRoutes = map[string]bool{
	RouteMutant:  true,
	RouteRevert:  true,
	RouteNoBreak: true,
}

// DiscriminationRoutes answers them sorted, for a refusal message.
func DiscriminationRoutes() []string { return sortedKeys(discriminationRoutes) }

// Why a no-break record says no break exists.
//
// A closed vocabulary, and each reason CLAIMS A FACT the gate goes and checks,
// in the shape checkSuperseded's four dispositions already have. An
// unconditioned reason is the blanket opt-out the escape exists to avoid: a
// {gap} is cheap, and "there is nothing to break here" with nothing behind it
// would be cheaper, so the escaped count would climb faster than the proven
// one (R-9).
const (
	// escapeForeign: the behavior is produced by an implementation this
	// repository does not build, so no edit here can falsify the claim.
	// CHECKED: the carrier kind is interop, and the record names no producer,
	// because there is none in this tree to name.
	escapeForeign = "foreign-producer"
	// escapeDeclaration: the named producer file holds no function body -- a
	// table, an embed, a registration list. CHECKED: the producer key is a bare
	// path and goFuncSpans finds no function in it.
	escapeDeclaration = "declaration-only"
	// escapeGenerated: the named producer is generated, so a break is undone by
	// the next generator run and can never be re-observed. CHECKED: the
	// producer's file carries the `Code generated ... DO NOT EDIT.` marker Go
	// defines.
	escapeGenerated = "generated-producer"
	// escapeKernel: the Linux kernel produces the behavior from the packet
	// alone, and no state Ze installs decides it, so no break of Ze's own code
	// falsifies the claim. A break of the KERNEL could, and that is the
	// patched-kernel route the record names as its LaterRoute. CHECKED: the
	// tagged unit is a Go unit whose file builds only for Linux, the record
	// names no producer in this tree, the RFC summary declares a lower layer
	// and its reason names the record's kernel function, and the record cites
	// a span of the tag's own claim.
	escapeKernel = "kernel-producer"
)

var escapeReasons = map[string]bool{
	escapeForeign:     true,
	escapeDeclaration: true,
	escapeGenerated:   true,
	escapeKernel:      true,
}

// laterPatchedKernel is the proof route a kernel-producer escape could still
// take: a clean run on the runtime kernel and a broken run on a kernel carrying
// one patch that breaks the producing function. The route is not built yet;
// plan/post-release/kernel/spec-rfc-kernel-patch-discrimination-route.md
// designs it, and every kernel-producer escape is its population.
const laterPatchedKernel = "patched-kernel"

// laterRoutes is the closed set a record's LaterRoute names: a route, not
// built yet, that could prove what the escape leaves unproven.
var laterRoutes = map[string]bool{
	laterPatchedKernel: true,
}

// laterRouteNames answers them sorted, for a refusal message.
func laterRouteNames() []string { return sortedKeys(laterRoutes) }

// escapeReasonNames answers them sorted, for a refusal message.
func escapeReasonNames() []string { return sortedKeys(escapeReasons) }

// annotationKinds are the `{...}` kinds that say something about Ze's
// COVERAGE. SupersededKind is named apart because it says something about the
// DOCUMENT, and the two registers must never share a slot: had superseded
// joined this set, marking a requirement would have EVICTED its {gap} and a
// document's obsolescence would have become a way out of the gated population.
// The annotation kinds a checklist line can carry. Named, because
// AnnotationSinglePolarity is read in three places -- the parser that demands a
// polarity beside it, the coverage rule that treats it as complete cover, and
// the audit schema that lets one test carry an `enforced` verdict -- and a
// literal spelled three times is a rule three files can disagree about.
//
// AnnotationLowerLayer says a layer UNDER Ze performs the behavior, on state Ze
// installs into that layer. The owner ruling of 2026-08-31 counts such a
// requirement MET and asks for a test at the boundary Ze owns; this kind is for
// the requirements where that boundary carries nothing the behavior reads, so
// there is no value to assert. Sixteen RFC 4302 obligations are the case it was
// added for (2026-09-03): Linux XFRM builds every AH packet, and no field of
// the SA Ze installs decides that the RESERVED field is zero.
//
// It is not a softer {not-applicable}. That one says the obligation never bound
// Ze; this one says it binds, is met, and is not proven BY ZE -- so it stays
// inside the gated denominator and out of the proven numerator
// (provenshare.go), exactly where {gap} sits. What stops it becoming the next
// blanket exemption is that its reason CLAIMS A FACT the gate checks: the layer
// and, as `<path>.go::<Symbol>`, the producer that installs into it.
//
// AnnotationFeatureDeclined says the obligation is CONDITIONAL on a feature the
// RFC makes optional, and Ze declined that feature, so the condition is false
// and the obligation does not bind. RFC4302-2.5.1-1 is the case it was added
// for (owner approval, 2026-09-03): an Extended Sequence Number "MUST be
// negotiated by an SA management protocol", Ze negotiates no AH SA and uses no
// ESN, so {gap} would accuse Ze of owing behavior it does not owe and
// {lower-layer} would claim a negotiation no layer performs.
//
// It is the coverage register's word for the decision the extraction register
// already records as `feature-out-of-scope` (exclusionKinds, artifact.go). The
// two are spelled apart on purpose: the site's `out-of-scope` counter already
// means {not-applicable} there, so one word would name two partitions on one
// page. Both mean "the RFC makes a feature OPTIONAL, Ze decided not to offer
// it, and this obligation is conditional on offering it".
//
// What stops it becoming the next {not-applicable} is the same discipline
// {lower-layer} carries: its reason claims two FACTS the gate checks. The
// QUOTED sentence that makes the feature optional is held against the RFC's own
// text in rfc/full/, and the producer that does the narrower thing Ze chose is
// held against this checkout (checkFeatureDeclined, check_core.go).
//
// AnnotationRollup says the row ASSERTS nothing of its own: it is true exactly
// when every row it names is true. RFC4302-5-1 and RFC4302-5-2 are the case it
// was added for (2026-09-14): "MUST fully implement the AH syntax and
// processing described here" is met by the other rows of that summary and not
// by any producer, so {gap} would accuse Ze of owing behavior its constituents
// already meet, {lower-layer} refuses a rollup by name, and no tagged test can
// carry it: the test would claim more than its body checks.
//
// Its status is DERIVED at check time from the targets' own state and never
// written by an author: met when every target is met, a gap while any target
// is a gap, unproven while any target is unproven (rollupDeriver.derive,
// check_core.go). A target is a requirement id or a summary stem, and one the
// corpus cannot show is refused (checkRollupTargets), exactly as a
// {lower-layer} producer the tree cannot show is. The other five kinds each
// sit in the gated denominator because the obligation is Ze's; this one sits
// outside it, because every obligation it carries is already counted once
// under its own id (CoverageRows, coverage.go). For the same reason the gate
// raises no finding for a rollup: its derived state and the cause are
// published on the row, and the row that owes the work is reported once,
// under its own id (owner decision, 2026-09-15).
const (
	AnnotationNotApplicable   = "not-applicable"
	AnnotationGap             = "gap"
	AnnotationPartial         = "partial"
	AnnotationSinglePolarity  = "single-polarity"
	AnnotationLowerLayer      = "lower-layer"
	AnnotationFeatureDeclined = "feature-declined"
	AnnotationRollup          = "rollup"
	// AnnotationLinuxLimit names a verified kernel primitive Linux cannot
	// provide. The obligation still binds and remains unmet.
	AnnotationLinuxLimit = "linux-limit"
)

// AnnotationTraits is the shared meaning of a coverage annotation. Values
// returned by AnnotationTraitsOf are copies; changing one cannot change the
// declaration. The zero value grants no proof or audit permission.
type AnnotationTraits struct {
	Counts    bool
	Proves    bool
	BarsTests bool
	// BarsSupported forbids a public conformance promise even when Remaining
	// discloses the unmet obligation.
	BarsSupported      bool
	Unmet              bool
	RemainingRequired  bool
	SinglePolarity     bool
	Gap                bool
	Partial            bool
	Rollup             bool
	RollupState        RollupState
	Bucket             string
	AuditUnimplemented bool
	AuditNotApplicable bool
	CeilingExcluded    bool
	CeilingLabel       string
}

// annotationKinds declares each kind once, including its specialized parser.
// A nil parser means the common non-empty reason is the entire payload.
var annotationKinds = map[string]struct {
	traits AnnotationTraits
	parse  func(rest, where string) (*Annotation, error)
	format string
}{
	AnnotationNotApplicable: {
		traits: AnnotationTraits{Counts: true, BarsTests: true, RollupState: RollupMet,
			Bucket: "not_applicable", AuditUnimplemented: true, AuditNotApplicable: true,
			CeilingExcluded: true, CeilingLabel: "Not applicable (classification can be revisited)"},
		parse: parseNotApplicable,
	},
	AnnotationGap: {
		traits: AnnotationTraits{Counts: true, BarsTests: true, Unmet: true, Gap: true,
			RollupState: RollupGap, Bucket: "gap", AuditUnimplemented: true},
	},
	AnnotationPartial: {
		traits: AnnotationTraits{Counts: true, Unmet: true, RemainingRequired: true,
			Partial: true, RollupState: RollupGap, Bucket: "partial-scope"},
		parse: parsePartial,
	},
	AnnotationSinglePolarity: {
		traits: AnnotationTraits{Counts: true, Proves: true, SinglePolarity: true,
			RollupState: RollupUnproven, Bucket: "single_polarity"},
		parse: parseSinglePolarity,
	},
	AnnotationLowerLayer: {
		traits: AnnotationTraits{Counts: true, BarsTests: true,
			RollupState: RollupMet, Bucket: "lower_layer",
			CeilingExcluded: true, CeilingLabel: "Met by a lower layer"},
		parse: parseLowerLayer,
	},
	AnnotationFeatureDeclined: {
		traits: AnnotationTraits{Counts: true, BarsTests: true, RollupState: RollupMet,
			Bucket: "feature_declined", AuditNotApplicable: true,
			CeilingExcluded: true, CeilingLabel: "Optional feature not offered (classification can be revisited)"},
		parse: parseFeatureDeclined,
	},
	AnnotationRollup: {
		traits: AnnotationTraits{BarsTests: true, Rollup: true,
			RollupState: RollupUnproven, Bucket: "rollup"},
		parse: parseRollup,
	},
	AnnotationLinuxLimit: {
		traits: AnnotationTraits{Counts: true, BarsTests: true, BarsSupported: true, Unmet: true, RemainingRequired: true,
			RollupState: RollupGap, Bucket: "linux_limit", AuditUnimplemented: true,
			CeilingExcluded: true, CeilingLabel: "Linux cannot do this"},
		parse:  parseLinuxLimit,
		format: linuxLimitFormat,
	},
}

// AnnotationKinds answers the declared kinds sorted.
func AnnotationKinds() []string { return sortedKeysOf(annotationKinds) }

// AnnotationTraitsOf answers the declared meaning and whether kind is known.
// Callers MUST handle an unknown kind explicitly: it cannot excuse a row,
// remove it from a denominator, or grant proof or audit credit.
func AnnotationTraitsOf(kind string) (AnnotationTraits, bool) {
	declaration, known := annotationKinds[kind]
	return declaration.traits, known
}

const SupersededKind = "superseded"

// UpdatedByKind is the row marker naming the requirement of an UPDATING RFC
// that rewrites this row. It orders the row; it changes no count.
const UpdatedByKind = "updated-by"

// What a superseded document's obligation became. A closed set, because free
// text lets "the RFC was replaced" read as "Ze need not comply".
const (
	successorRestated    = "restated"
	successorDropped     = "dropped"
	successorUnextracted = "unextracted"
	successorUnresolved  = "unresolved"
)

var successorDispositions = map[string]bool{
	successorRestated:    true,
	successorDropped:     true,
	successorUnextracted: true,
	successorUnresolved:  true,
}

// successorDispositionNames answers them sorted.
func successorDispositionNames() []string { return sortedKeys(successorDispositions) }

// successorTargeted are the two dispositions that name something.
var successorTargeted = map[string]bool{successorRestated: true, successorUnextracted: true}

// noSection is the anchor a requirement citing no section of its own takes.
// Deliberately conspicuous: it is a summary defect to fix, not a resting state.
const noSection = "x"

// tagMarker is the literal every tag in every carrier contains. It is the cheap
// pre-filter that tells "this file certainly holds no tag" from "this file
// might", so the expensive answer is only computed where it can change the
// verdict. Both call sites read the constant; neither re-spells the string.
const tagMarker = "RFC requirement:"

// shaHexLen is the width of every fingerprint this package records, in hex
// characters.
const shaHexLen = 16

// tagPunct is the trailing punctuation an author legitimately writes around a
// tag. `godot` requires a Go doc comment's last line to end in a period, so a
// tag placed last becomes "RFC7606-2-1 negative." -- rejecting that would make
// the lint rule and the tag convention contradict each other.
const tagPunct = ".,;:"

// levelAlternation is every RFC 2119 keyword, longest first, so "MUST NOT"
// wins over "MUST" and "NOT RECOMMENDED" over "RECOMMENDED".
//
// Built in an init rather than written out, because the set above is what the
// value comparison against Python reads: a keyword added to one and not the
// other would leave the alternation and the set disagreeing about the
// population.
var levelAlternation = buildLevelAlternation()

func buildLevelAlternation() string {
	all := append(gatedLevelNames(), advisoryLevelNames()...)
	// Longest first. Equal lengths keep sorted order, which is what Python's
	// sorted(key=len, reverse=True) does: the sort is stable there too.
	for i := 1; i < len(all); i++ {
		for j := i; j > 0 && len(all[j]) > len(all[j-1]); j-- {
			all[j], all[j-1] = all[j-1], all[j]
		}
	}
	var tb textbuf.Buffer
	for i, level := range all {
		if i > 0 {
			tb.Byte('|')
		}
		tb.Str(regexp.QuoteMeta(level))
	}
	return tb.String()
}

// The patterns that read a Compliance Checklist line.
//
// The two that splice in the shared levelAlternation are assembled with
// textbuf by a function rather than written inline.
var (
	// checklistPattern parses the whole line. Sections carry lowercase letters
	// (S3.b, S7.11), so the id must too.
	checklistRE = regexp.MustCompile(reChecklist())
	// firstTagRE tells "this line is trying to be a requirement and is
	// malformed" from "this line is prose" and from an ad-hoc implementation
	// checklist entry ([FORMAT], [IPSEC]).
	firstTagRE = regexp.MustCompile(`^-\s*\[[ xX]\]\s*\[(?P<tag>[^\]]*)\]`)
	// levelBracketRE finds any bracketed RFC 2119 keyword anywhere on the
	// line. Its presence means "this line is a compliance requirement",
	// independent of whether the id parses -- which is what makes a malformed
	// id an ERROR instead of a silent skip.
	levelBracketRE = regexp.MustCompile(reLevelBracket())
	// idRE splits on the LAST hyphen for the ordinal; everything between the
	// RFC prefix and that hyphen is the section, so dotted (5.3), lettered
	// (3.b) and deep (9.1.2.2) sections all work.
	idRE = regexp.MustCompile(`^(?P<head>.+)-(?P<ord>\d+)$`)
	// trailingParenRE finds the last parenthetical on the line: by convention
	// that is where the section is cited.
	trailingParenRE = regexp.MustCompile(`\((?P<body>[^()]*)\)[^()]*$`)
	// annotationRE matches ONE trailing `{...}` group.
	annotationRE = regexp.MustCompile(`\{(?P<body>[^{}]*)\}\s*$`)
)

func reChecklist() string {
	var tb textbuf.Buffer
	return tb.Str(`^-\s*\[(?P<box>[ xX])\]\s*`).
		Str(`(?:\[(?P<rid>[A-Za-z0-9][A-Za-z0-9.\-]*-\d+)\]\s*)?`).
		Str(`\[(?P<level>`).Str(levelAlternation).Str(`)\]\s*`).
		Str(`(?P<rest>.*)$`).String()
}

func reLevelBracket() string {
	var tb textbuf.Buffer
	return tb.Str(`\[(?:`).Str(levelAlternation).Str(`)\]`).String()
}

// ParseError says the input is malformed. It is always raised and never
// swallowed: a silently skipped MUST is a false green.
//
// It is a distinct type because the drivers separate "this tree is wrong" from
// "I could not read the tree", and both exit 2 while only one names a file.
type ParseError struct{ msg string }

func (e *ParseError) Error() string { return e.msg }

// parseErr builds one, from a buffer the caller has already filled.
func parseErr(tb *textbuf.Buffer) error { return &ParseError{msg: tb.String()} }

// isParseError reports whether err came from malformed input rather than from
// an unreadable tree.
func isParseError(err error) bool {
	var pe *ParseError
	return errors.As(err, &pe)
}

// Annotation is a `{kind: reason}` marker on a requirement line: why this
// requirement owes less than a positive and a negative test.
// Annotation is one coverage disposition a checklist line carries.
//
// Polarity, Layer, Quote, Producer and Targets are each read by ONE kind and
// empty on every other, except Producer, which two kinds name: the parser
// fills them where that kind's format demands them, so a reader that has the
// kind never re-parses the reason to recover them.
type Annotation struct {
	Kind     string `json:"kind"`
	Polarity string `json:"polarity,omitempty"`
	// LimitID names the Linux limitations register entry, without copying its evidence.
	LimitID string `json:"limit-id,omitempty"`
	// Layer names what performs the behavior under Ze. It is {lower-layer}'s
	// alone, and a layer claim nothing here can falsify is the blanket
	// exemption that kind exists to avoid being.
	Layer string `json:"layer,omitempty"`
	// Quote is the RFC's own sentence that removes the row from Ze's count, and
	// two kinds carry one. On {feature-declined} it makes the feature optional;
	// on {not-applicable} it scopes the obligation to a role. Both are held
	// against the section the row cites in rfc/full/<stem>.txt, or for
	// {feature-declined} the section QuoteSection names
	// ((*evidenceReader).gap): the kind rests on the DOCUMENT saying so, which
	// is a fact rather than a judgement. Empty on a {not-applicable} row
	// written before that kind carried evidence.
	Quote string `json:"quote,omitempty"`
	// QuoteSection is the section a {feature-declined} payload names for its
	// Quote (`§<section> "<quote>"; <reason>`), when the RFC makes the feature
	// optional in a section other than the one holding the conditional
	// obligation. The Quote is then held against that section rather than the
	// row's (featureQuoteGap). It is a section id (featureSectionIDRE) and
	// must be an exact heading of the RFC, never resolved to an ancestor.
	// Empty means the row's own section or its subsections, and a named
	// section is refused as redundant whenever the quote lies in the row's own
	// section. It is {feature-declined}'s alone.
	QuoteSection string `json:"quote-section,omitempty"`
	// Heading is the feature words of a {feature-declined} heading form
	// (`heading "<feature words>"; ze never ...`): the declined feature as the
	// title of the row's own section, or of one of its ancestors, names it. A
	// heading that names the feature scopes every obligation under it the way
	// a conditional does, the never clause shows Ze never uses that feature
	// (featureHeadingGap), and the summary's `Declined features` row names the
	// owner decision behind it (checkDeclinedFeatureHeadings). Quote is empty
	// when it is set. It is {feature-declined}'s alone.
	Heading string `json:"heading,omitempty"`
	// BoundRole is the role the Quote binds, which the quoted sentence names
	// and which is none of the roles the summary's Meta table declares Ze
	// fills (parseNotApplicable, checkNotApplicableRoles). It is
	// {not-applicable}'s alone.
	BoundRole string `json:"bound-role,omitempty"`
	// LayerPath is where a reader checks the component Layer names: a kernel
	// CONFIG_ option Ze's kernel config enables (linux:CONFIG_IPV6), a Go
	// standard library package (go:crypto/x509) or a vendored module directory
	// (vendor/<module>). It is {lower-layer}'s alone, and empty on a row
	// written before that kind carried it ((*evidenceReader).layerPathGap).
	LayerPath string `json:"layer-path,omitempty"`
	// Tested and Gap locate distinct clauses inside the complete parent quote.
	// Partial tags prove only Tested; every other obligation remains unproven.
	Tested string `json:"tested,omitempty"`
	Gap    string `json:"gap,omitempty"`
	// Producer is the `<path>.go::<Symbol>` a reason names, and
	// checkLowerLayerProducer and checkFeatureDeclined each hold it against the
	// tree. {lower-layer} names the function that installs into the layer;
	// {feature-declined} names the function that does the narrower thing Ze
	// chose. Both die the same way, when the code is renamed or deleted under
	// the annotation.
	Producer string `json:"producer,omitempty"`
	// Targets are the rows a rollup derives from, each a requirement id or a
	// summary stem, in the order the author wrote them. It is {rollup}'s
	// alone: checkRollupTargets holds every one against the corpus and
	// rollupDeriver reads their state, so a target nobody can find is a
	// refusal rather than prose.
	Targets []string `json:"targets,omitempty"`
	Reason  string   `json:"reason"`
}

// Successor is where one requirement of a superseded document now lives.
//
// Target is the successor's requirement id under `restated` and the
// successor's section under `unextracted`; `dropped` and `unresolved` have
// nothing to name, which is what they exist to say.
type Successor struct {
	Disposition string `json:"disposition"`
	Target      string `json:"target,omitempty"`
	Reason      string `json:"reason"`
}

// UpdatedBy names the requirement of an updating RFC that rewrites one row of
// the RFC it updates: replaces, narrows, relaxes or strengthens it.
//
// Target is exactly one requirement id of the updater, and Reason says what the
// updater does to the row. The row stays gated and counted in its own RFC.
type UpdatedBy struct {
	Target string `json:"target"`
	Reason string `json:"reason"`
}

// Requirement is one Compliance Checklist line.
type Requirement struct {
	RFC     string `json:"rfc"`
	RID     string `json:"rid"`
	Level   string `json:"level"`
	Text    string `json:"text"`
	Section string `json:"section"`
	// Annotation is nil when the line carries no coverage annotation.
	Annotation *Annotation `json:"annotation,omitempty"`
	Source     string      `json:"source"`
	Line       int         `json:"line"`
	// Ticked records a hand-written "- [x]". Recorded, not obeyed: coverage is
	// DERIVED from test tags, so a tick is someone's claim, not evidence.
	Ticked bool `json:"ticked"`
	// Derived is the state a {rollup} row takes from the rows it names, and
	// RollupNone on every other row. rollupDeriver.fill writes it, for evaluate
	// and for deriveRollups (check_core.go). A reader that finds RollupNone on
	// a rollup row is reading a requirement nobody has evaluated, never a
	// rollup with no state.
	Derived RollupState `json:"derived,omitempty"`
	// Superseded is where this obligation now lives, when the document stating
	// it has been obsoleted. Its OWN field, never a member of Annotation.
	Superseded *Successor `json:"superseded,omitempty"`
	// UpdatedBy is the updating RFC's requirement that rewrites this row, and
	// nil when no updater touches it. Its OWN field, like Superseded.
	UpdatedBy *UpdatedBy `json:"updated-by,omitempty"`
	// DerivedCause is the sentence naming the first target that decided a
	// derived gap or unproven state, and empty on a met rollup and on every
	// other row. The gate raises no finding for a rollup, so this is the one
	// place the cause is published: the Proof column and the site print it.
	DerivedCause string `json:"derived-cause,omitempty"`
}

// Gated reports whether this requirement's level creates an obligation the
// gate enforces.
func (r Requirement) Gated() bool { return gatedLevels[r.Level] }

// Rollup reports whether this requirement is annotated {rollup}: a row that
// asserts nothing of its own and is left out of every gated population.
func (r Requirement) Rollup() bool {
	if r.Annotation == nil {
		return false
	}
	traits, known := AnnotationTraitsOf(r.Annotation.Kind)
	return known && traits.Rollup
}

// DerivedMark answers what a page prints for a rollup row's derived state: the
// state, then the cause where one decided it (`gap: RFC4302-2.5-5 is annotated
// {gap}`), so a reader lands on the row that owes the work.
//
// A rollup that reaches a page with no derived state is a renderer that read
// the row before deriveRollups ran, which NewRenderInput does for every
// renderer. That is a defect and not a fourth state, so it stops here rather
// than printing a blank a reader would take for data (ai/rules/principles.md).
func (r Requirement) DerivedMark() string {
	if r.Derived == RollupNone {
		panic("BUG: " + r.RID + " is a {rollup} row rendered before deriveRollups filled it")
	}
	if r.DerivedCause == "" {
		return r.Derived.String()
	}
	return r.Derived.String() + ": " + r.DerivedCause
}

// RollupState is what the gate derives for a {rollup} row from the rows it
// names, and what each named row contributes to that derivation.
//
// The zero value is RollupNone, "not a rollup", so a Requirement nobody
// derived cannot read as met. Nothing compares the three real states by
// value: rollupDeriver.answer settles a gap before an unproven row by reading
// the targets in author order, because a rollup over one gap and nine
// untested rows owes the gap first.
type RollupState uint8

const (
	RollupNone RollupState = iota
	// RollupMet says every named row is met: proven in both polarities, or in
	// its one under {single-polarity}, or excused by {lower-layer},
	// {feature-declined} or {not-applicable}, or a met rollup.
	RollupMet
	// RollupGap says a named row is annotated {gap}, or is a gap rollup.
	RollupGap
	// RollupUnproven says no named row is a gap and one is not met: no test,
	// one polarity, a target the gate holds no state for, or a rollup whose
	// derivation leads into a cycle.
	RollupUnproven
)

// String answers the word the ledger prints for a derived state, and nothing
// for RollupNone: a row that is not a rollup has no derived state to print.
func (s RollupState) String() string {
	switch s {
	case RollupMet:
		return "met"
	case RollupGap:
		return "gap"
	case RollupUnproven:
		return "unproven"
	case RollupNone:
		return ""
	default:
		panic("BUG: unexpected rollup state")
	}
}

// correctionKind says which change a paragraph of rfc/corrections/<stem>.md
// authorizes. The zero value authorizes nothing, so a paragraph whose opener
// was not recognized can never pass as either kind.
type correctionKind uint8

const (
	// The zero value is no kind, so an unrecognized opener authorizes nothing.
	_ correctionKind = iota
	// correctionLevel opens with `Correction <date>:` and authorizes a row
	// leaving the gated level population (checkLevelRatchet).
	correctionLevel
	// correctionRetirement opens with `Retired <date>:` and authorizes an
	// enrolled id disappearing from its summary (checkRetiredRequirements).
	correctionRetirement
)

// correction is one dated paragraph of rfc/corrections/<stem>.md: the recorded
// authorisation for a change to the rows it names. Quotes holds every
// double-quoted span in the paragraph and Sections every § reference, both
// unverified: the check that consumes the paragraph judges them.
type correction struct {
	Kind     correctionKind `json:"kind"`
	Date     string         `json:"date"`
	RIDs     []string       `json:"rids"`
	Quotes   []string       `json:"quotes"`
	Sections []string       `json:"sections"`
	Line     int            `json:"line"`
}

// Tag is one `RFC requirement: <ID> <polarity>` comment found in a test.
//
// Claim is the prose half: what the tag advertises its unit demonstrates. Every
// gate read the structured half and stopped until 2026-08-31, so a tag could
// promise an assertion its body never makes. discriminate.go fingerprints the
// claim as its own field of a record, which is what makes rewording it stale
// the proof rather than widen it.
//
// Gap marks the other shape, `RFC requirement: <ID> gap`: a test that asserts
// the RFC-correct behavior Ze does not have yet, through rfcgap.Demonstrate.
// It proves no polarity, so Polarity is empty and Collect keeps it out of the
// proof corpus every coverage reader walks (splitGapTags).
//
// Demonstration is the unit key (`path::Func`) of the test function around a
// gap tag that calls rfcgap.Demonstrate with the tag's own id. Empty on a gap
// tag says the scan found no such call, and the gate refuses the tag
// (gapTagRefusal): a gap tag nothing runs is a description, not a demonstration.
type Tag struct {
	RID           string `json:"rid"`
	Polarity      string `json:"polarity"`
	Gap           bool   `json:"gap,omitempty"`
	Demonstration string `json:"demonstration,omitempty"`
	File          string `json:"file"`
	Line          int    `json:"line"`
	Claim         string `json:"claim,omitempty"`
}

// Prefix answers the id prefix of a summary stem: rfc7606 -> RFC7606,
// draft-foo-bar -> DRAFT-FOO-BAR.
func Prefix(stem string) string { return strings.ToUpper(stem) }

func hasRIDStem(rid, stem string) bool {
	prefix := Prefix(stem)
	if len(prefix) < len(rid) {
		if strings.HasPrefix(rid, prefix) {
			return rid[len(prefix)] == '-'
		}
	}
	return false
}

// treePath joins a slash-separated tree-relative path onto the checkout.
func treePath(tree string, rel ...string) string {
	parts := make([]string, 0, len(rel)+1)
	parts = append(parts, tree)
	for _, one := range rel {
		parts = append(parts, filepath.FromSlash(one))
	}
	return filepath.Join(parts...)
}

// relTo answers a path relative to the tree, slash-separated, so a message
// names the file the way a developer types it. It answers the input unchanged
// when the two share no prefix, which is what os.path.relpath would not do --
// and a message naming an absolute path is better than one naming a walk of
// `..` segments.
func relTo(tree, path string) string {
	rel, err := filepath.Rel(tree, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

// readFile answers a file's text, or a ParseError naming it. errors.Join is not
// used: the message is compared against the script's, which names the path and
// the operating system's reason and nothing else.
func readFile(path, rel string) (string, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- a path under the checkout this gate judges
	if err != nil {
		var tb textbuf.Buffer
		return "", parseErr(tb.Str(rel).Str(": cannot read: ").Err(err))
	}
	return string(raw), nil
}
