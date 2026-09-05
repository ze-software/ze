// Design: docs/architecture/core-design.md -- the authored half of a sign-off
// Overview: rfc.go -- the types, the paths and the closed sets every reader here shares
//
// artifact.go reads rfc/extraction/<stem>.json, the record of one reviewer's
// walk over an RFC's own text.
//
// Every enum is closed and every authored field is required, so a malformed
// artifact is a parse error the driver turns into a clean exit 2 rather than a
// traceback. An UNCLASSIFIED disposition is NOT a parse error: it is a legal
// skeleton state that the CHECK refuses, which is what makes generating
// skeletons unable to produce a pass.
package rfc

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// extractionSchemaVersion is the artifact shape this reader accepts.
const extractionSchemaVersion = 1

// relocatedToSpec is the one exclusion kind that does NOT say "this sentence
// binds nobody". It says the opposite: the obligation is owed, by a named spec,
// under an id reserved there.
const relocatedToSpec = "relocated-to-spec"

// The keys an extraction artifact declares. Each one is read by the parser,
// listed in a closed key set, and written by the selftest fixture, so a typo in
// any one of the three reads as a field nobody wrote.
const (
	keySchemaVersion = "schema-version"
	keyStem          = "stem"
	keyReviewer      = "reviewer"
	keySignedOff     = "signed-off"
	keySites         = "sites"
	keyDisposition   = "disposition"
	keyReason        = "reason"
	keyQuote         = "quote"
)

// The two dispositions a walked section can carry.
const (
	dispositionWalked  = "walked"
	dispositionSkipped = "skipped"
)

// The two site dispositions and the one exclusion kind that other files read
// by name. Spelled once, because a second spelling is a second place for a
// reader and the closed set below to drift apart.
//
// Both dispositions are EXPORTED, because internal/le/site publishes the
// exclusions of every sign-off and has to select them. Deriving the word by
// elimination out of SiteDispositions would put a second rule beside this one.
const (
	DispositionMapped   = "mapped"
	DispositionExcluded = "excluded"
	exclusionDuplicate  = "duplicate-of"
)

// exclusionKinds is the closed vocabulary, each kind beside what it says about
// the sentence the walk declined to map.
//
// The meaning is stated HERE, beside the kind, because a reader who meets
// `binds-another-role` on a published page cannot act on the word alone, and
// because the site aggregates these counts across the corpus. A second table of
// sentences elsewhere would be a copy of this set with nothing to arbitrate a
// disagreement (ai/rules/principles.md), which is the shape auditVerdicts
// already avoids.
//
// feature-out-of-scope is a DECISION, and a gap is an ISSUE. A gap says Ze owes
// the behavior and does not produce it, so it stays on the ledger until the
// behavior exists. This says the obligation never bound Ze at all. The absent
// FEATURE is still recorded, as an implementation gap a later scope decision
// can revisit, and never as a conformance gap (ai/rules/rfc-compliance.md,
// owner directive 2026-08-31).
// The two things an exclusion can MEAN. Every kind declares one, and there is
// no default: a kind added without a group is a kind whose meaning nobody
// decided, and it would read as scope by omission -- which is the flattery
// failure this ledger exists to prevent.
//
// ExclusionScope says the obligation never bound Ze. ExclusionDebt says the
// opposite: it is real, Ze owes it, it is unbuilt, and a named spec owns it.
// Summing the two into one "declined" count publishes a debt as scope.
const (
	ExclusionScope = "scope"
	ExclusionDebt  = "debt"
)

// exclusionKind is what one kind says and which of the two it means.
type exclusionKind struct {
	Meaning string
	Group   string
}

var exclusionKinds = map[string]exclusionKind{
	"not-a-requirement": {Group: ExclusionScope,
		Meaning: "the sentence states a fact or describes another document, and directs no " +
			"implementation"},
	"binds-another-role": {Group: ExclusionScope,
		Meaning: "the obligation is addressed to a role Ze never acts as"},
	exclusionDuplicate: {Group: ExclusionScope,
		Meaning: "the same obligation is already captured under another requirement id"},
	"cross-document": {Group: ExclusionScope,
		Meaning: "the obligation belongs to another document that this one only cites"},
	"advisory-in-context": {Group: ExclusionScope,
		Meaning: "the sentence advises on applying a rule stated elsewhere and adds no " +
			"obligation of its own"},
	"feature-out-of-scope": {Group: ExclusionScope,
		Meaning: "the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this " +
			"obligation is conditional on offering it"},
	relocatedToSpec: {Group: ExclusionDebt,
		Meaning: "the obligation is real and unbuilt, and a named spec owes it"},
}

// ExclusionKinds answers them sorted.
func ExclusionKinds() []string { return sortedKeysOf(exclusionKinds) }

// ExclusionKindMeaning answers what one exclusion kind says, and false for a
// kind outside the vocabulary.
//
// A caller that publishes an exclusion owes the reader the sentence. These
// words are the project's own and mean nothing to a reader outside it.
func ExclusionKindMeaning(kind string) (string, bool) {
	entry, held := exclusionKinds[kind]
	return entry.Meaning, held
}

// ExclusionKindGroup answers whether one kind means SCOPE or DEBT, and false
// for a kind outside the vocabulary.
//
// The two must never be summed. `relocated-to-spec` is an obligation Ze owes
// and has not built, and checkExtraction refuses the sign-off unless the named
// spec exists and still reserves the requirement id, so it is tracked work
// rather than an obligation that went away.
func ExclusionKindGroup(kind string) (string, bool) {
	entry, held := exclusionKinds[kind]
	return entry.Group, held
}

// ExclusionPresumedWrong answers whether the repository's own rule treats this
// kind as suspect until it is justified.
//
// `binds-another-role` is PRESUMED WRONG (ai/rules/rfc-compliance.md, owner
// directive 2026-08-31): Ze rarely implements one side of a protocol, so an
// obligation addressed to "the sender" or "the receiver" almost always binds
// it, and the label reads on the public ledger as "not our problem" where the
// truth is usually "our problem, unbuilt". A page that published the count
// without that context would repeat the flattery the ledger exists to prevent.
func ExclusionPresumedWrong(kind string) bool { return kind == "binds-another-role" }

// dispositionKinds is what each un-enrolled kind SAYS, for a reader outside
// this project.
//
// The kinds themselves are declared once, in ledger.go, and enrolmentKinds in
// meta.go is the closed set the parser holds authors to. What this adds is the
// sentence, for the same reason exclusionKinds carries one: a published page
// that prints `source-restricted` and stops has told a reader nothing.
// TestEveryDispositionKindCarriesItsMeaning holds this set against the parser's.
var dispositionKinds = map[string]string{
	dispositionNonNormative: "the document imposes no MUST-level obligation on an " +
		"implementation, so there is nothing to gate",
	dispositionBacklog: "the requirements have not been extracted from the document yet; " +
		"this is work owed rather than a decision",
	dispositionBlocked: "something outside the summary stops the extraction, and it is " +
		"named in the reason",
	dispositionSourceRestricted: "the standard's own text may not be redistributed, so this " +
		"repository cannot hold it and no checklist can ever be bounded against it",
	dispositionOutOfScope: "the requirements ARE extracted and the owner decided not to " +
		"offer the feature for now, so the absence is a scope decision rather than a " +
		"conformance gap",
}

// DispositionKinds answers the un-enrolled kinds, sorted.
func DispositionKinds() []string { return sortedKeysOf(dispositionKinds) }

// DispositionKindMeaning answers what one un-enrolled kind says, and false for
// a kind outside the vocabulary.
func DispositionKindMeaning(kind string) (string, bool) {
	meaning, held := dispositionKinds[kind]
	return meaning, held
}

var sectionSkipKinds = map[string]bool{
	"front-matter": true, "references": true, "iana": true,
	"acknowledgements": true, "appendix-non-normative": true,
}

// SectionSkipKinds answers them sorted.
func SectionSkipKinds() []string { return sortedKeys(sectionSkipKinds) }

var siteDispositions = map[string]bool{DispositionMapped: true, DispositionExcluded: true}

// SiteDispositions answers them sorted.
func SiteDispositions() []string { return sortedKeys(siteDispositions) }

var sectionDispositions = map[string]bool{dispositionWalked: true, dispositionSkipped: true}

// SectionDispositions answers them sorted.
func SectionDispositions() []string { return sortedKeys(sectionDispositions) }

// specPathRE refuses everything that is not a spec. plan/known-failures/ and
// plan/learned/ are homes the compliance rule names as NOT a decision
// procedure, and this shape refuses both, along with any path that leaves the
// repository. A spec in ANY release bucket is accepted.
var specPathRE = regexp.MustCompile(reSpecPath())

func reSpecPath() string {
	var tb textbuf.Buffer
	tb.Str(`^(?:`)
	for index, dir := range specDirNames() {
		if index != 0 {
			tb.Byte('|')
		}
		tb.Str(regexp.QuoteMeta(dir))
	}
	return tb.Str(`)/spec-[a-z0-9][a-z0-9._-]*\.md$`).String()
}

var artifactKeys = map[string]bool{
	keySchemaVersion: true, keyStem: true, "register": true, "register-reason": true,
	"source-path": true, "source-sha": true, keySignedOff: true, keyReviewer: true,
	"resign-reason": true, "sections": true, keySites: true,
}

var siteKeys = map[string]bool{
	"id": true, keyQuote: true, keyDisposition: true, "mapped-to": true,
	"excluded-kind": true, keyReason: true, "relocated-to": true, "reserved-id": true,
}

var sectionKeys = map[string]bool{
	"id": true, keySites: true, keyDisposition: true, "skip-kind": true,
	keyReason: true, "unsourced-ids": true,
}

// ExtractionSite is one classified sentence of the source.
type ExtractionSite struct {
	ID    string `json:"id"`
	Quote string `json:"quote"`
	// Disposition is empty for an unclassified site, which is a legal skeleton
	// state and a check failure.
	Disposition  string `json:"disposition,omitempty"`
	MappedTo     string `json:"mapped-to,omitempty"`
	ExcludedKind string `json:"excluded-kind,omitempty"`
	Reason       string `json:"reason,omitempty"`
	// RelocatedTo and ReservedID are the relocated-to-spec fields: the spec
	// that owes the obligation, and the id it reserves for it.
	RelocatedTo string `json:"relocated-to,omitempty"`
	ReservedID  string `json:"reserved-id,omitempty"`
}

// ExtractionSection is one section of the source and what the reviewer did
// with it.
type ExtractionSection struct {
	ID           string   `json:"id"`
	Sites        int      `json:"sites"`
	Disposition  string   `json:"disposition,omitempty"`
	SkipKind     string   `json:"skip-kind,omitempty"`
	Reason       string   `json:"reason,omitempty"`
	UnsourcedIDs []string `json:"unsourced-ids,omitempty"`
}

// sectionTitleMax bounds what reads as a section's own name rather than as
// prose about the walk.
//
// A reviewer opens a section's reason with the section's title where the
// document gives one: "Constructing the Next Hop field. The only section that
// binds a BGP speaker." Past this width the opening sentence is an account of
// what the reviewer did, and a page that printed it as a title would name the
// section wrongly. The widest real title in rfc/extraction/ is 47 characters.
const sectionTitleMax = 60

// sectionNarrationArticles are the words a narrating sentence puts after its
// verb. "Read THE ECN section" is an account of the walk; "Constructing THE
// Next Hop field" is a name, and the two are told apart by the tense of the
// word before the article, never by this list alone.
var sectionNarrationArticles = map[string]bool{
	"the": true, "a": true, "an": true, "its": true,
	"this": true, "these": true, "every": true, "each": true,
}

// sectionNarrationIrregulars are the irregular past tenses a reviewer writes
// when narrating what they did to a section. A regular one ends in "ed" and
// needs no list.
var sectionNarrationIrregulars = map[string]bool{
	"read": true, "took": true, "left": true, "found": true, "saw": true,
	"went": true, "held": true, "kept": true, "chose": true, "wrote": true,
	"understood": true, "sought": true, "brought": true,
}

// narratesTheWalk answers whether a lead is an account of what the reviewer did
// rather than the section's name.
//
// The shape is a past-tense verb followed by an article: "Read the ECN
// section", "Walked the nonce section". Both halves are load-bearing. Past
// tense alone refuses real titles -- "Deprecated MRT Types", "Combined mode
// algorithms", "Failed authentication" -- and an article alone refuses the
// gerund titles an RFC is full of: "Constructing the Next Hop field", "Using
// the cache", "Rekeying the IKE SA". No title in the 1,038 this checkout
// derives is past tense followed by an article, and 17 of the narrating leads
// are.
//
// It is a grammatical shape and not a list of observed strings, so a reviewer
// who writes "Checked the ..." or "Skipped the ..." tomorrow is refused by the
// same rule that refuses "Walked" today.
func narratesTheWalk(lead string) bool {
	words := strings.Fields(lead)
	if len(words) < 2 {
		return false
	}
	first := strings.ToLower(words[0])
	if !sectionNarrationArticles[strings.ToLower(strings.Trim(words[1], ",;:"))] {
		return false
	}
	return sectionNarrationIrregulars[first] || strings.HasSuffix(first, "ed")
}

// sectionNotASection is how a record says the numbered section does not exist
// in the source. It is a verdict about the register, never a name.
const sectionNotASection = "not a section"

// sectionPartDeterminers and sectionPartNouns are the two halves of a lead
// whose SUBJECT is the document part rather than the topic: "The section gives
// KEYMAT and three ordering bullets", "One paragraph introducing the four usage
// scenarios below it".
//
// A section's name says what the section is ABOUT. A sentence whose subject is
// the section is the reviewer describing it from outside, and printing it as
// the name tells a reader the RFC contains a section called "The section
// shows...".
var (
	sectionPartDeterminers = map[string]bool{
		"the": true, "one": true, "a": true, "an": true, "this": true,
	}
	sectionPartNouns = map[string]bool{
		"section": true, "introduction": true, "paragraph": true, "heading": true,
		"chapter": true, "appendix": true, "abstract": true, "preamble": true,
	}
)

// describesTheDocumentPart answers whether the lead takes the document part as
// its subject.
//
// Three words at least. "The section" alone is a name for want of a better one
// and says nothing false; "The section gives KEYMAT" is a sentence about it.
// Real titles that open with an article keep their own subject -- "The Route
// Target Attribute", "The Echo Function and Asymmetry" -- and are untouched,
// because the word after the article is the topic rather than a part of the
// document.
func describesTheDocumentPart(lead string) bool {
	words := strings.Fields(lead)
	if len(words) < 3 {
		return false
	}
	return sectionPartDeterminers[strings.ToLower(words[0])] &&
		sectionPartNouns[strings.ToLower(strings.Trim(words[1], ",;:"))]
}

// Title answers the section's own name, taken from the opening sentence of the
// reason, and the empty string where the record states none.
//
// A caller that prints a section number owes the reader the name beside it: the
// number alone is a place to go and look up. The empty string is what a caller
// falls back on, so a lead this cannot read as a name is refused rather than
// published as one.
//
// The derivation is a CONVENTION, not a declaration. No record carries a title
// field; the convention is that a reviewer opens the reason with the section's
// name, and 1,015 of the 1,038 leads this checkout derives do. Where one wrote
// prose instead, the page published that prose as the section's name: rfc7296
// section 2.10 read "Walked the nonce section" and rfc1035 section 25 read "NOT
// A SECTION", both live on the artifact until 2026-09-02 (independent review).
// Three shapes are refused here, and each is the reviewer speaking about their
// own walk rather than naming the section.
func (s ExtractionSection) Title() string {
	lead, _, _ := strings.Cut(strings.TrimSpace(s.Reason), ". ")
	lead = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(lead), "."))
	if lead == "" || len(lead) > sectionTitleMax {
		return ""
	}
	if narratesTheWalk(lead) {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(lead), sectionNotASection) {
		return ""
	}
	if describesTheDocumentPart(lead) {
		return ""
	}
	return lead
}

// Extraction is one whole sign-off record.
type Extraction struct {
	Stem           string              `json:"stem"`
	Register       string              `json:"register"`
	SourcePath     string              `json:"source-path"`
	SourceSHA      string              `json:"source-sha"`
	SignedOff      string              `json:"signed-off,omitempty"`
	Reviewer       string              `json:"reviewer,omitempty"`
	ResignReason   string              `json:"resign-reason,omitempty"`
	RegisterReason string              `json:"register-reason,omitempty"`
	Sections       []ExtractionSection `json:"sections"`
	Sites          []ExtractionSite    `json:"sites"`
	Path           string              `json:"path"`
}

// Relocated answers the relocated-to-spec SUBSET of Excluded, never a
// disposition of its own.
//
// Excluded stays the number both the ratchet and the published ratio read,
// because it is one: this walk declined to map the sentence. Counting the
// subset apart is what lets a reviewer tell a homed obligation from a dismissed
// sentence.
func (e Extraction) Relocated() int { return e.countSites(DispositionExcluded, relocatedToSpec) }

// Mapped counts the sites this walk tied to a requirement id.
func (e Extraction) Mapped() int { return e.countSites(DispositionMapped, "") }

// Excluded counts the sites this walk declined to map, relocations included.
//
// A relocation is counted here, and in the published exclusion ratio, because
// it is one: this walk did decline to map the sentence. It is counted APART in
// the ledger's relocation note, because a homed obligation and a dismissed
// sentence are not the same fact.
func (e Extraction) Excluded() int { return e.countSites(DispositionExcluded, "") }

// Unclassified counts the sites and the sections this artifact leaves with no
// disposition.
//
// A generated skeleton is all of both, and a walk in progress is some of
// either, so the pair separates "nobody has started" from "three sentences are
// left". The check leads with this census because the per-site detail that
// follows it can run to a thousand lines on a long RFC, and a wall of them
// tells an author less than one sentence naming the file and the count.
func (e Extraction) Unclassified() (sites, sections int) {
	for _, site := range e.Sites {
		if site.Disposition == "" {
			sites++
		}
	}
	for _, section := range e.Sections {
		if section.Disposition == "" {
			sections++
		}
	}
	return sites, sections
}

func (e Extraction) countSites(disposition, kind string) int {
	n := 0
	for _, site := range e.Sites {
		if site.Disposition == disposition && (kind == "" || site.ExcludedKind == kind) {
			n++
		}
	}
	return n
}

// strField reads a string field. A field that is not required accepts
// absent-or-empty as "", but still refuses a wrong TYPE: "not filled in yet" is
// a legal state, 42 never is.
func strField(obj map[string]any, key, where string, required bool) (string, error) {
	value := obj[key]
	text, isText := value.(string)
	if !required && (value == nil || (isText && strings.TrimSpace(text) == "")) {
		return "", nil
	}
	if !isText || strings.TrimSpace(text) == "" {
		var tb textbuf.Buffer
		return "", parseErr(tb.Str(where).Str(": ").Str(pyRepr(key)).
			Str(" must be a non-empty string, got ").Str(pyRepr(value)))
	}
	return strings.TrimSpace(text), nil
}

// rejectUnknownKeys refuses a typo'd key, which would otherwise read as an
// ABSENT field. Every field here is authored, and an absent authored field must
// never pass silently.
func rejectUnknownKeys(obj map[string]any, allowed map[string]bool, where string) error {
	var unknown []string
	for key := range obj {
		if !allowed[key] {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	sort.Strings(unknown)
	var tb textbuf.Buffer
	return parseErr(tb.Str(where).Str(": unknown key(s) ").Str(pyRepr(unknown)).
		Str("; expected one of ").Str(pyRepr(sortedKeys(allowed))))
}

// relocationFields answers the destination spec and the reserved id of a
// relocated-to-spec exclusion.
//
// Both are required and both are shape-checked, because this kind is the one
// that does not dismiss its sentence: it says somebody else is bound, over
// there. A pointer whose target nothing can resolve is the shrug the kind
// exists not to be.
func relocationFields(entry map[string]any, where, stem string) (string, string, error) {
	var tb textbuf.Buffer
	rel, isText := entry["relocated-to"].(string)
	if !isText || !specPathRE.MatchString(strings.TrimSpace(rel)) {
		return "", "", parseErr(tb.Str(where).Str(": ").Str(relocatedToSpec).
			Str(" needs a 'relocated-to' naming the spec that owes this obligation, as ").
			Str("<bucket>/spec-<name>.md with <bucket> one of ").
			Str(strings.Join(specDirNames(), ", ")).Str("; got ").Str(pyRepr(entry["relocated-to"])).
			Str(". A known-failure file, a learned summary and any document outside a ").
			Str("release bucket are none of them a spec ").
			Str("(ai/rules/rfc-compliance.md)"))
	}
	var want textbuf.Buffer
	prefix := want.Str(Prefix(stem)).Byte('-').String()
	rid, isRID := entry["reserved-id"].(string)
	trimmed := strings.TrimSpace(rid)
	if !isRID || !strings.HasPrefix(trimmed, prefix) || !idRE.MatchString(trimmed) {
		return "", "", parseErr(tb.Str(where).Str(": ").Str(relocatedToSpec).
			Str(" needs a 'reserved-id', the requirement id the destination spec ").
			Str("reserves for this obligation, as ").Str(prefix).Str("<section>-<n>; got ").
			Str(pyRepr(entry["reserved-id"])).
			Str(". Without it the relocation points at a document rather than at a row, ").
			Str("and the spec could satisfy the gate while owing nothing"))
	}
	return strings.TrimSpace(rel), trimmed, nil
}

// ParseExtractionArtifact reads and validates one rfc/extraction/<stem>.json.
func ParseExtractionArtifact(tree, path string) (Extraction, error) {
	rel := relTo(tree, path)
	wantStem := strings.TrimSuffix(filepath.Base(path), ".json")
	var tb textbuf.Buffer

	raw, err := os.ReadFile(path) // #nosec G304 -- a path under the checkout this gate judges
	if err != nil {
		return Extraction{}, parseErr(tb.Str(rel).Str(": cannot read: ").Err(err))
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	// UseNumber keeps the literal an author wrote, which is what tells an int
	// from a float. A decoded float64 has already lost the difference, and the
	// site count refuses a float on purpose.
	decoder.UseNumber()
	var document any
	if err := decoder.Decode(&document); err != nil {
		return Extraction{}, parseErr(tb.Str(rel).Str(": cannot read: ").Err(err))
	}
	data, isObject := document.(map[string]any)
	if !isObject {
		return Extraction{}, parseErr(tb.Str(rel).Str(": expected a JSON object, got ").
			Str(pyTypeName(document)))
	}
	if err := rejectUnknownKeys(data, artifactKeys, rel); err != nil {
		return Extraction{}, err
	}

	if !equalsSchemaVersion(data[keySchemaVersion]) {
		return Extraction{}, parseErr(tb.Str(rel).Str(": schema-version must be ").
			Int(extractionSchemaVersion).Str(", got ").Str(pyRepr(data[keySchemaVersion])))
	}

	stem, err := strField(data, keyStem, rel, true)
	if err != nil {
		return Extraction{}, err
	}
	if stem != wantStem {
		return Extraction{}, parseErr(tb.Str(rel).Str(": stem ").Str(pyRepr(stem)).
			Str(" does not match the filename (").Str(pyRepr(wantStem)).
			Str("). The artifact names the RFC it signs off; the two can never drift apart"))
	}

	register, _ := data["register"].(string)
	if !knownRegister(register) {
		return Extraction{}, parseErr(tb.Str(rel).Str(": register ").
			Str(pyRepr(data["register"])).
			Str(" is missing, empty or unknown; expected one of ").Str(pyRepr(Registers())).
			Str(". It does NOT default to the strong grade: that would publish the ").
			Str("weakest sign-off as the strongest"))
	}

	// signed-off, reviewer and register-reason are required to SIGN OFF, not to
	// parse: an unsigned skeleton is a legal intermediate state, and having the
	// writer invent a date and a reviewer so its own output parses would
	// fabricate a sign-off record for a walk nobody performed.
	art := Extraction{Stem: stem, Register: register, Path: rel}
	for _, field := range []struct {
		key  string
		into *string
	}{
		{"register-reason", &art.RegisterReason},
		{keySignedOff, &art.SignedOff},
		{keyReviewer, &art.Reviewer},
		{"resign-reason", &art.ResignReason},
	} {
		value, err := strField(data, field.key, rel, false)
		if err != nil {
			return Extraction{}, err
		}
		*field.into = value
	}

	if art.Sections, err = parseSections(data, rel); err != nil {
		return Extraction{}, err
	}
	if art.Sites, err = parseSites(data, rel, stem); err != nil {
		return Extraction{}, err
	}
	if art.SourcePath, err = strField(data, "source-path", rel, true); err != nil {
		return Extraction{}, err
	}
	if art.SourceSHA, err = strField(data, "source-sha", rel, true); err != nil {
		return Extraction{}, err
	}
	return art, nil
}

// equalsSchemaVersion reads the version the way Python's `!=` does, which
// accepts 1, 1.0 and true alike. Reproduced rather than tightened: this reader
// answers what the script answers, and a stricter version check is a change to
// what the gate accepts rather than a port of it.
func equalsSchemaVersion(value any) bool {
	switch typed := value.(type) {
	case json.Number:
		asFloat, err := typed.Float64()
		return err == nil && asFloat == float64(extractionSchemaVersion)
	case bool:
		return typed && extractionSchemaVersion == 1
	}
	return false
}

func knownRegister(name string) bool { return slices.Contains(Registers(), name) }

func parseSections(data map[string]any, rel string) ([]ExtractionSection, error) {
	var tb textbuf.Buffer
	rawSections, isList := data["sections"].([]any)
	if !isList {
		return nil, parseErr(tb.Str(rel).Str(": 'sections' must be a list"))
	}
	out := make([]ExtractionSection, 0, len(rawSections))
	seen := map[string]bool{}
	for _, item := range rawSections {
		entry, isObject := item.(map[string]any)
		if !isObject {
			tb.Reset()
			return nil, parseErr(tb.Str(rel).Str(": each section must be an object, got ").
				Str(pyRepr(item)))
		}
		if err := rejectUnknownKeys(entry, sectionKeys, rel); err != nil {
			return nil, err
		}
		id, err := strField(entry, "id", rel, true)
		if err != nil {
			return nil, err
		}
		if seen[id] {
			tb.Reset()
			return nil, parseErr(tb.Str(rel).Str(": duplicate section ").Str(pyRepr(id)))
		}
		seen[id] = true
		var where textbuf.Buffer
		place := where.Str(rel).Str(": section ").Str(id).String()

		count, ok := nonNegativeInt(entry[keySites])
		if !ok {
			tb.Reset()
			return nil, parseErr(tb.Str(place).Str(": 'sites' must be a non-negative integer"))
		}
		disposition, err := dispositionOf(entry, sectionDispositions, SectionDispositions(), place)
		if err != nil {
			return nil, err
		}
		reason, err := strField(entry, keyReason, place, false)
		if err != nil {
			return nil, err
		}
		skipKind := ""
		if disposition == dispositionSkipped {
			skipKind, _ = entry["skip-kind"].(string)
			if !sectionSkipKinds[skipKind] {
				tb.Reset()
				return nil, parseErr(tb.Str(place).Str(": skipped needs a 'skip-kind' from ").
					Str(pyRepr(SectionSkipKinds())).Str(", got ").
					Str(pyRepr(orEmptyString(entry["skip-kind"]))))
			}
			if reason == "" {
				tb.Reset()
				return nil, parseErr(tb.Str(place).Str(": skipped needs a non-empty 'reason'"))
			}
		}
		unsourced, err := stringList(entry["unsourced-ids"], place)
		if err != nil {
			return nil, err
		}
		out = append(out, ExtractionSection{
			ID: id, Sites: count, Disposition: disposition,
			SkipKind: skipKind, Reason: reason, UnsourcedIDs: unsourced,
		})
	}
	return out, nil
}

func parseSites(data map[string]any, rel, stem string) ([]ExtractionSite, error) {
	var tb textbuf.Buffer
	rawSites, isList := data[keySites].([]any)
	if !isList {
		return nil, parseErr(tb.Str(rel).Str(": 'sites' must be a list"))
	}
	out := make([]ExtractionSite, 0, len(rawSites))
	seen := map[string]bool{}
	for _, item := range rawSites {
		entry, isObject := item.(map[string]any)
		if !isObject {
			tb.Reset()
			return nil, parseErr(tb.Str(rel).Str(": each site must be an object, got ").
				Str(pyRepr(item)))
		}
		if err := rejectUnknownKeys(entry, siteKeys, rel); err != nil {
			return nil, err
		}
		id, err := strField(entry, "id", rel, true)
		if err != nil {
			return nil, err
		}
		if seen[id] {
			tb.Reset()
			return nil, parseErr(tb.Str(rel).Str(": duplicate site locator ").Str(pyRepr(id)))
		}
		seen[id] = true
		var where textbuf.Buffer
		place := where.Str(rel).Str(": site ").Str(id).String()

		site := ExtractionSite{ID: id}
		if site.Quote, err = strField(entry, keyQuote, place, true); err != nil {
			return nil, err
		}
		if site.Disposition, err = dispositionOf(entry, siteDispositions, SiteDispositions(), place); err != nil {
			return nil, err
		}
		if site.Reason, err = strField(entry, keyReason, place, false); err != nil {
			return nil, err
		}
		switch site.Disposition {
		case DispositionMapped:
			if site.MappedTo, err = strField(entry, "mapped-to", place, true); err != nil {
				return nil, err
			}
		case DispositionExcluded:
			site.ExcludedKind, _ = entry["excluded-kind"].(string)
			if _, held := exclusionKinds[site.ExcludedKind]; !held {
				tb.Reset()
				return nil, parseErr(tb.Str(place).
					Str(": excluded needs an 'excluded-kind' from ").
					Str(pyRepr(ExclusionKinds())).Str(", got ").
					Str(pyRepr(orEmptyString(entry["excluded-kind"]))))
			}
			if site.Reason == "" {
				tb.Reset()
				return nil, parseErr(tb.Str(place).
					Str(": excluded needs a non-empty 'reason'. A bare exclusion is an ").
					Str("escape hatch; say why this sentence binds nothing"))
			}
			switch site.ExcludedKind {
			case exclusionDuplicate:
				// mapped-to means "the requirement id this site relates to":
				// for a mapping, the id this site PROVES; for a duplicate, the
				// id already captured elsewhere. Naming it is what makes the
				// duplicate checkable at all -- a duplicate that names nothing
				// cannot be compared against anything, and a chain of such
				// could cover an RFC in which nothing is actually mapped.
				if site.MappedTo, err = strField(entry, "mapped-to", place, true); err != nil {
					return nil, err
				}
			case relocatedToSpec:
				if site.RelocatedTo, site.ReservedID, err = relocationFields(entry, place, stem); err != nil {
					return nil, err
				}
			}
		}
		if site.ExcludedKind != relocatedToSpec &&
			(entry["relocated-to"] != nil || entry["reserved-id"] != nil) {
			// Authored where they mean nothing, both fields would be read by
			// nobody and reported by nobody: the same failure a typo'd key is,
			// one level up.
			tb.Reset()
			return nil, parseErr(tb.Str(place).
				Str(": 'relocated-to' and 'reserved-id' mean something only on a ").
				Str(relocatedToSpec).Str(" exclusion. Here they are silently ignored, ").
				Str("and a silently ignored authored field is indistinguishable from ").
				Str("one nobody wrote"))
		}
		out = append(out, site)
	}
	return out, nil
}

// dispositionOf reads a disposition field, where absent means unclassified.
func dispositionOf(entry map[string]any, known map[string]bool, sorted []string, place string) (string, error) {
	value := entry[keyDisposition]
	if value == nil {
		return "", nil
	}
	text, isText := value.(string)
	if !isText || !known[text] {
		var tb textbuf.Buffer
		return "", parseErr(tb.Str(place).Str(": disposition ").Str(pyRepr(value)).
			Str(" is not one of ").Str(pyRepr(sorted)).Str(" (null means unclassified)"))
	}
	return text, nil
}

// nonNegativeInt reads an integer field, refusing a float and a bool the way
// Python's isinstance pair does.
func nonNegativeInt(value any) (int, bool) {
	number, isNumber := value.(json.Number)
	if !isNumber {
		return 0, false
	}
	asInt, err := number.Int64()
	if err != nil || asInt < 0 {
		return 0, false
	}
	return int(asInt), true
}

// stringList reads a list of non-empty strings, where absent means none.
func stringList(value any, place string) ([]string, error) {
	if value == nil {
		return nil, nil
	}
	items, isList := value.([]any)
	if !isList {
		return nil, listRefusal(place)
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		text, isText := item.(string)
		if !isText || strings.TrimSpace(text) == "" {
			return nil, listRefusal(place)
		}
		out = append(out, strings.TrimSpace(text))
	}
	return out, nil
}

func listRefusal(place string) error {
	var tb textbuf.Buffer
	return parseErr(tb.Str(place).Str(": 'unsourced-ids' must be a list of requirement ids"))
}

// orEmptyString reproduces Python's `x or ""` for a field whose falsy values
// all render as the empty string in the message that refuses them.
func orEmptyString(value any) any {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		if typed == "" {
			return ""
		}
		return typed
	case bool:
		if !typed {
			return ""
		}
		return typed
	case json.Number:
		if asFloat, err := typed.Float64(); err == nil && asFloat == 0 {
			return ""
		}
		return typed
	case []any:
		if len(typed) == 0 {
			return ""
		}
		return typed
	case map[string]any:
		if len(typed) == 0 {
			return ""
		}
		return typed
	}
	return value
}

// ExtractionStems answers every stem under rfc/extraction/.
func ExtractionStems(tree string) (map[string]bool, error) {
	dir := treePath(tree, extractionRel)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		var tb textbuf.Buffer
		return nil, parseErr(tb.Str(relTo(tree, dir)).Str(": cannot read: ").Err(err))
	}
	out := map[string]bool{}
	for _, entry := range entries {
		if name := entry.Name(); strings.HasSuffix(name, ".json") {
			out[strings.TrimSuffix(name, ".json")] = true
		}
	}
	return out, nil
}

// LoadExtractions answers every artifact under rfc/extraction/, parsed. It
// stops at the first malformed one.
func LoadExtractions(tree string) (map[string]Extraction, error) {
	stems, err := ExtractionStems(tree)
	if err != nil {
		return nil, err
	}
	out := make(map[string]Extraction, len(stems))
	for _, stem := range sortedSet(stems) {
		var name textbuf.Buffer
		path := treePath(tree, extractionRel, name.Str(stem).Str(".json").String())
		art, err := ParseExtractionArtifact(tree, path)
		if err != nil {
			return nil, err
		}
		out[stem] = art
	}
	return out, nil
}
