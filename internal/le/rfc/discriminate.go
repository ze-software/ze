// Design: docs/architecture/core-design.md -- the recorded proof behind one RFC tag
// Overview: rfc.go -- the types, the paths and the closed sets every reader here shares
// Detail: check_ratchets.go -- the ratchet that judges these records
// Related: goscope.go -- UnitAt, the one definition of the tagged unit a record names
//
// An RFC tag is `RFC requirement: <ID> <polarity>` followed by prose stating
// what the test demonstrates. tags.go reads the structured half. Nothing can
// read the prose half, so a tag can advertise an assertion its body never
// makes and every gate stays green.
//
// discriminate.go reads what replaces reading that prose: a record that the
// tagged unit was OBSERVED to fail under a named break of the code the claim
// rests on. "The prose is true" is unfalsifiable by a machine; "this break
// makes this unit red" is decidable and replayable.
package rfc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// keyID is the keyword the discriminate action takes a requirement id under.
const keyID = "id"

// jsonSuffix selects the artifact files in the discrimination tree, so a
// README beside them is walked past rather than parsed as a record.
const jsonSuffix = ".json"

// What the tree says about one recorded proof now. Mutually exclusive and
// total: verifyDiscrimination answers exactly one of these for every record.
const (
	// ProofVerified: both fingerprints match, so the tree still holds the code
	// the red was observed against.
	ProofVerified = "verified"
	// ProofUnitGone: the tagged unit the break reddened is no longer in the tree.
	ProofUnitGone = "unit-gone"
	// ProofTagGone: the unit is still there and no longer carries the tag this
	// record proves.
	ProofTagGone = "tag-gone"
	// ProofClaimChanged: the tag is there and what it CLAIMS was reworded, so
	// the recorded red proves the old sentence and not the new one.
	ProofClaimChanged = "claim-changed"
	// ProofUnitChanged: the tagged unit is there and its behavior changed.
	ProofUnitChanged = "unit-changed"
	// ProofProducerGone: the code the break was applied to is no longer in the tree.
	ProofProducerGone = "producer-gone"
	// ProofProducerChanged: the producer is there and its behavior changed.
	ProofProducerChanged = "producer-changed"
	// ProofCitationGone: the assertion the record rests on is not in the
	// carrier the record names.
	ProofCitationGone = "citation-gone"
	// ProofEscapeUnfounded: the escape's reason claims a fact about the tree,
	// and the tree no longer holds it.
	ProofEscapeUnfounded = "escape-unfounded"
)

// DiscriminationRecord is one recorded break and the tagged unit it reddened.
//
// Unit is the tagged unit key, so the record dies with the tag it proves:
// rename the test and the key stops matching. Route says how the break was
// produced, or that no break exists. Producer names the code the break was
// applied to, and Break states the break itself, which no gate parses and a
// reviewer reads.
//
// The three fingerprints are what makes a stored proof re-checkable without
// running anything. Two are the behavior hash of the unit their key names,
// taken when the red was observed; the third is the hash of what the tag
// claims. A hash that no longer matches says the observation was never repeated
// over THIS code, or was never made about THIS claim, and neither is a proof.
type DiscriminationRecord struct {
	RID      string `json:"rid"`
	Polarity string `json:"polarity"`
	Unit     string `json:"unit"`
	UnitSHA  string `json:"unit-sha"`
	// ClaimSHA fingerprints the PROSE the tag advertises, apart from the unit's
	// behavior. behaviorBytes strips comments and a claim is a comment, so
	// without a field of its own a sealed proof survives a reworded claim: the
	// widened sentence would be published as proven with no code edit at all
	// (owner decision, 2026-08-31).
	ClaimSHA    string `json:"claim-sha"`
	Route       string `json:"route"`
	Producer    string `json:"producer,omitempty"`
	ProducerSHA string `json:"producer-sha,omitempty"`
	Break       string `json:"break,omitempty"`
	// Citation names the assertion a functional or interop proof rests on: a
	// numbered `fail(N, ...)` site for an interop checker, and a directive line
	// for a `.ci`. Neither carrier is reachable by a generated break, so the
	// citation is what ties the recorded red to a named assertion rather than
	// to the suite as a whole.
	Citation string `json:"citation,omitempty"`
	// Reason is why no break exists, for the escape only. A closed vocabulary,
	// and each reason's precondition is checked against the tree.
	Reason string `json:"reason,omitempty"`
	// Source is the artifact file this record was read from. Derived from the
	// path at load time, never authored, so it is not part of the schema.
	Source string `json:"-"`
}

// Proves answers whether this record claims a proof rather than the escape.
//
// RouteNoBreak is the ESCAPE and is counted apart from the two proof routes
// wherever a count is published: a claim that no break exists is debt rather
// than evidence.
func (r DiscriminationRecord) Proves() bool { return r.Route != RouteNoBreak }

// discriminationFile is one rfc/discrimination/<stem>.json: the RFC it proves
// and the records that prove it.
type discriminationFile struct {
	RFC     string                 `json:"rfc"`
	Records []DiscriminationRecord `json:"records"`
}

// Cover is what one record proves: one requirement, in one direction, in one
// TAGGED UNIT.
//
// A typed key rather than a joined string, so no separator can collide with a
// path and no two readers can spell the join differently.
//
// The unit rather than its file, since 2026-08-31. Phase 1 keyed on the carrier
// file because resolving each tag's unit costs a file read, and a file key
// bills nothing for a second tag on the same requirement in the same file: an
// author could add a test tagged with a requirement that file already proves
// elsewhere and owe no proof at all. That is the over-claim route the gate
// exists to close, so the read is paid.
type Cover struct {
	RID      string `json:"rid"`
	Polarity string `json:"polarity"`
	Unit     string `json:"unit"`
}

// Cover answers the coverage this record carries.
func (r DiscriminationRecord) Cover() Cover {
	return Cover{RID: r.RID, Polarity: r.Polarity, Unit: r.Unit}
}

// unitKeyAt mints the fingerprint key of the unit one tag sits in.
//
// UnitAt through scopeIndex is the one definition of the tagged unit, so this
// adds no second answer to that question: it turns the scope that walk reports
// into the key form fingerprintKey parses. A tag outside exactly one top-level
// function is file-scoped, and the bare path IS the key there.
func unitKeyAt(index *scopeIndex, path, content string, line int) string {
	name := index.funcNameAt(path, content, line)
	if name == "" {
		return path
	}
	var tb textbuf.Buffer
	return tb.Str(path).Str("::").Str(name).String()
}

// tagCovers resolves every tag to the unit it sits in and groups the tags by
// what they prove.
//
// One walk, three consumers: the claim fingerprint a record is verified
// against, the obligation a tag new since HEAD owes, and the orphan exemption
// that tells a record whose TAG is gone from one that no longer verifies.
// Resolving a unit costs a file read and a span walk, so the answer is computed
// once and shared.
//
// A tagged file that cannot be read is an error rather than a file-scoped
// guess. The guess would move the tag off the unit key its record names, and
// the record would then read as an orphan and be quietly forgiven -- which is
// the fail-open shape (ai/rules/principles.md).
func tagCovers(reader *sourceReader, index *scopeIndex, tags []Tag) (map[Cover][]Tag, error) {
	out := make(map[Cover][]Tag, len(tags))
	for _, tag := range tags {
		content := reader.read(tag.File)
		if content == nil {
			var tb textbuf.Buffer
			return nil, parseErr(tb.Str(tag.File).Byte(':').Int(int64(tag.Line)).
				Str(": cannot read the file this tag was scanned from, so the unit it sits ").
				Str("in cannot be resolved and its recorded proof cannot be matched to it"))
		}
		key := Cover{RID: tag.RID, Polarity: tag.Polarity,
			Unit: unitKeyAt(index, tag.File, *content, tag.Line)}
		out[key] = append(out[key], tag)
	}
	return out, nil
}

// tagCoversIn scans one tree and answers the same map, for a caller holding
// nothing but a checkout.
func tagCoversIn(tree string) (map[Cover][]Tag, error) {
	tags, err := scanTreeWith(tree, coverCarriers(tree))
	if err != nil {
		return nil, err
	}
	return tagCovers(newSourceReader(tree), newScopeIndex(), tags)
}

// coverCarriers answers the table a cover walk needs.
//
// A cover key reads a carrier's KIND, which is a property of the path, and
// never its TIER, which is derived from the workflow schedule. A tree with no
// .github/workflows can therefore still be walked for its tagged units, and
// refusing there would leave every fixture unable to resolve its own -- the
// same choice, for the same reason, verifyDiscrimination makes about the table
// it resolves carriers with.
func coverCarriers(tree string) []Carrier {
	if table, err := carriers(tree); err == nil {
		return table
	}
	return carriersFor(FunctionalSuites(), nil)
}

// claimSHA fingerprints what the tags of one cover CLAIM.
//
// Its own field of a record, apart from the unit's behavior hash, because
// behaviorBytes strips comments and an RFC tag's claim IS a comment. Without
// this a sealed proof survives a reworded claim: an author proves a modest
// claim, widens the sentence with no code edit at all, and the gate publishes
// the wider claim as proven (owner decision, 2026-08-31).
//
// The tags are hashed in line order, because one unit can carry several tags
// for one requirement and polarity and each of them claims something.
func claimSHA(tags []Tag) string {
	var tb textbuf.Buffer
	for index := range tags {
		if index > 0 {
			tb.Byte('\n')
		}
		tb.Str(tags[index].Claim)
	}
	sum := sha256.Sum256([]byte(tb.String()))
	return hex.EncodeToString(sum[:])[:shaHexLen]
}

// loadDiscrimination reads every recorded proof in the corpus, in file order.
//
// An absent artifact tree is an empty answer: most tags have never been
// proven, and an unproven tag is a backlog the ledger publishes rather than a
// tree the gate cannot read. A file that EXISTS and cannot be read is the
// opposite and is refused, because a corrupt record must never read as a clean
// corpus (ai/rules/principles.md).
func loadDiscrimination(tree string) ([]DiscriminationRecord, error) {
	entries, err := os.ReadDir(treePath(tree, discriminationRel))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		var tb textbuf.Buffer
		return nil, parseErr(tb.Str(discriminationRel).Str(": cannot read: ").Err(err))
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), jsonSuffix) {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	var out []DiscriminationRecord
	for _, name := range names {
		found, loadErr := loadDiscriminationFile(tree, name)
		if loadErr != nil {
			return nil, loadErr
		}
		out = append(out, found...)
	}
	return out, nil
}

// loadDiscriminationFile reads and VALIDATES one artifact file.
//
// Unknown keys are refused rather than ignored, at both levels: a record whose
// route was typed as `rout` would otherwise load with an empty route, and a
// half-read record is the shape a false proof takes.
func loadDiscriminationFile(tree, name string) ([]DiscriminationRecord, error) {
	var relBuf textbuf.Buffer
	rel := relBuf.Str(discriminationRel).Byte('/').Str(name).String()
	raw, err := readFile(treePath(tree, discriminationRel, name), rel)
	if err != nil {
		return nil, err
	}

	var tb textbuf.Buffer
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	var file discriminationFile
	if decodeErr := decoder.Decode(&file); decodeErr != nil {
		return nil, parseErr(tb.Str(rel).Str(": cannot read: ").Err(decodeErr))
	}
	if decoder.More() {
		return nil, parseErr(tb.Str(rel).Str(": trailing content after the JSON object"))
	}
	stem := strings.TrimSuffix(name, jsonSuffix)
	if file.RFC != stem {
		return nil, parseErr(tb.Str(rel).Str(": 'rfc' is ").Quoted(file.RFC).
			Str(" but the filename says ").Quoted(stem).
			Str(". The record names the RFC it proves; the two can never drift apart"))
	}
	for index := range file.Records {
		file.Records[index].Source = rel
		if err := validateDiscrimination(file.Records[index], rel, index+1); err != nil {
			return nil, err
		}
	}
	return file.Records, nil
}

// validateDiscrimination refuses a record that cannot mean what it says.
//
// position is 1-based, because it is read by a person counting rows in a file.
//
// The two keys are parsed by fingerprintKey, the ONE key parser this package
// has. A record therefore inherits its path-safety refusal and, with it, the
// refusal of the retired `<path>:<line>` form -- which is the shape a record
// must never take, because a stored line is current only until the next edit
// above it.
func validateDiscrimination(record DiscriminationRecord, rel string, position int) error {
	var tb textbuf.Buffer
	tb.Str(rel).Str(": record ").Int(int64(position))
	if record.RID == "" {
		return parseErr(tb.Str(" names no requirement: 'rid' is empty"))
	}
	where := tb.Str(" for ").Str(record.RID).String()
	if !polarities[record.Polarity] {
		return parseErr(tb.Str(" has polarity ").Quoted(record.Polarity).
			Str(", want one of: ").Join(Polarities(), ", "))
	}
	if _, _, err := fingerprintKey(record.Unit, where); err != nil {
		return err
	}
	if err := validateBehaviorSHA(record.UnitSHA, where, "unit-sha"); err != nil {
		return err
	}
	if err := validateBehaviorSHA(record.ClaimSHA, where, "claim-sha"); err != nil {
		return err
	}
	if !discriminationRoutes[record.Route] {
		return parseErr(tb.Str(" has route ").Quoted(record.Route).
			Str(", want one of: ").Join(DiscriminationRoutes(), ", "))
	}
	if !record.Proves() {
		return validateEscapeShape(record, where)
	}
	if record.Reason != "" {
		return parseErr(tb.Reset().Str(where).Str(" takes the ").Str(record.Route).
			Str(" route and carries the escape reason ").Quoted(record.Reason).
			Str(". A reason says why NO break exists; this record states one"))
	}
	symbol, err := producerSymbol(record.Producer, where, record.Route)
	if err != nil {
		return err
	}
	if symbol == "" {
		return parseErr(tb.Reset().Str(where).Str(" has producer ").Quoted(record.Producer).
			Str(", which names a whole file. A break is applied to a FUNCTION, so the ").
			Str("producer key carries the symbol it was applied to"))
	}
	if err := validateBehaviorSHA(record.ProducerSHA, where, "producer-sha"); err != nil {
		return err
	}
	if strings.TrimSpace(record.Break) == "" {
		return parseErr(tb.Reset().Str(where).Str(" takes the ").Str(record.Route).
			Str(" route and states no break. The record exists so a reviewer can read what ").
			Str("was done to the producer; a proof nobody can read is a claim"))
	}
	return nil
}

// producerSymbol answers the function half of a proof record's producer key.
func producerSymbol(producer, where, route string) (string, error) {
	if producer == "" {
		var tb textbuf.Buffer
		return "", parseErr(tb.Str(where).Str(" takes the ").Str(route).
			Str(" route and names no producer. A break is a change to code the claim rests ").
			Str("on, so the record names that code as <path>::<FuncName>"))
	}
	_, symbol, err := fingerprintKey(producer, where)
	return symbol, err
}

// validateEscapeShape refuses an escape that cannot be checked, or that reads
// as a proof.
//
// A no-break record states that no break exists, so a break text or a producer
// fingerprint on it is a proof half-written, and counting it as debt while it
// reads as evidence is the confusion the escape exists to avoid.
//
// Every escape names what ties it to THIS claim, and which field carries the
// tie depends on whether there is code here to point at. escapeDeclaration and
// escapeGenerated each claim something about the CODE the claim rests on, so
// they name that code as the producer and carry no citation: their tie is the
// claim naming what that file declares. escapeForeign claims there is no such
// code in this tree, so it names no producer and CITES the assertion its own
// checker makes instead. A record carrying neither tie is the blanket opt-out
// this vocabulary exists to refuse (R-9), and discriminate_escape.go is where
// each claim is checked against the tree.
func validateEscapeShape(record DiscriminationRecord, where string) error {
	var tb textbuf.Buffer
	tb.Str(where).Str(" takes the ").Str(RouteNoBreak).Str(" route and carries ")
	switch {
	case record.ProducerSHA != "":
		return parseErr(tb.Str("a producer fingerprint. The escape states that NO break exists"))
	case record.Break != "":
		return parseErr(tb.Str("a break. The escape states that NO break exists"))
	}
	if !escapeReasons[record.Reason] {
		return parseErr(tb.Reset().Str(where).Str(" takes the ").Str(RouteNoBreak).
			Str(" route and gives the reason ").Quoted(record.Reason).
			Str(", want one of: ").Join(escapeReasonNames(), ", ").
			Str(". Each reason claims a fact this gate goes and checks; free text would ").
			Str("let \"there is nothing to break here\" read as \"this tag is proven\""))
	}
	if record.Reason == escapeForeign {
		return validateForeignEscape(record, where)
	}
	if record.Citation != "" {
		return parseErr(tb.Reset().Str(where).Str(" is ").Str(record.Reason).
			Str(" and cites the assertion ").Quoted(record.Citation).
			Str(". A citation is the tie only where no producer can be named. This reason ").
			Str("names one, and its tie is the claim naming what that file declares"))
	}
	if record.Producer == "" {
		return parseErr(tb.Reset().Str(where).Str(" is ").Str(record.Reason).
			Str(" and names no producer. That reason claims something about the code the ").
			Str("claim rests on, so the record names that code and the gate goes and reads it"))
	}
	_, _, err := fingerprintKey(record.Producer, where)
	return err
}

// validateForeignEscape refuses the foreign escape that names nothing.
//
// The reason says the behavior is produced outside this repository, so there is
// no producer here to name and the record MUST NOT name one. What it must carry
// instead is the citation: the carrier kind is a property of all 37 interop tags
// at once, so on its own it discharges every one of them equally, and the
// assertion number is the only field that says which claim this escape is about.
func validateForeignEscape(record DiscriminationRecord, where string) error {
	var tb textbuf.Buffer
	if record.Producer != "" {
		return parseErr(tb.Str(where).Str(" is ").Str(escapeForeign).
			Str(" and names the producer ").Quoted(record.Producer).
			Str(". That reason says the behavior is produced OUTSIDE this repository, ").
			Str("so there is no producer here to name"))
	}
	if record.Citation != "" {
		return nil
	}
	return parseErr(tb.Str(where).Str(" is ").Str(escapeForeign).
		Str(" and cites no assertion. No code in this tree produces the behavior, so the ").
		Str("assertion this checker makes about it is the only thing that ties the escape to ").
		Str("THIS claim: cite the `fail(N, ...)` number the checker writes out"))
}

// validateBehaviorSHA refuses a fingerprint that is not one.
//
// Length alone is not the check, for the reason validateSHA already states: a
// value padded to the right width is still not a fingerprint, so the charset is
// judged too. shaRE is that shape, declared once and shared.
func validateBehaviorSHA(value, where, field string) error {
	if shaRE.MatchString(value) {
		return nil
	}
	var tb textbuf.Buffer
	return parseErr(tb.Str(where).Str(" has ").Str(field).Str(" ").Quoted(value).
		Str(", which is not a fingerprint. Expected exactly ").Int(shaHexLen).
		Str(" lowercase hex characters, as ./le rfc discriminate computes them from the ").
		Str("unit's behavior. A hand-written fingerprint is never valid: it would publish a ").
		Str("red nobody observed"))
}

// behaviorSHA fingerprints what a unit DOES.
//
// behaviorBytes is the input, not normalize: it strips comments as well as
// whitespace, so a reworded comment inside a tagged test cannot void a proof,
// and a changed assertion always does. It is also the predicate ChangedTags
// already uses to answer "did this unit's behavior change", so the record goes
// stale exactly when the obligation says the unit moved. Two normalizations
// would be two answers to one question.
func behaviorSHA(path, text string) string {
	sum := sha256.Sum256([]byte(behaviorBytes(path, text)))
	return hex.EncodeToString(sum[:])[:shaHexLen]
}

// sealDiscrimination answers record with its fingerprints taken from tree.
//
// The ONE place a fingerprint is minted. Everything that writes a record goes
// through it, so a stored hash is always taken over the same text the verifier
// will read back. The first version of this package had two ways to compute one
// -- whole file here, resolved unit there -- and every record it wrote was
// stale on the day it was written.
//
// A key that does not resolve is an error rather than an empty hash: sealing
// nothing would give every missing unit one fingerprint, and they would all
// compare equal to each other. The same holds for the claim: a cover carrying
// no tag has nothing to seal, and an empty claim hash would seal every one of
// them identically.
func sealDiscrimination(tree string, covers map[Cover][]Tag,
	record DiscriminationRecord) (DiscriminationRecord, error) {
	reader := newSourceReader(tree)
	index := newScopeIndex()

	unit, held, err := resolveKeyText(reader.read, index, record.Unit, record.Source)
	if err != nil {
		return record, err
	}
	if !held {
		return record, unresolvedKeyErr(record, record.Unit, "tagged unit")
	}
	tagged := covers[record.Cover()]
	if len(tagged) == 0 {
		return record, unresolvedKeyErr(record, record.Unit, "tag")
	}
	record.UnitSHA = behaviorSHA(keyFile(record.Unit), unit)
	record.ClaimSHA = claimSHA(tagged)
	if !record.Proves() {
		return record, nil
	}

	producer, producerHeld, err := resolveKeyText(reader.read, index, record.Producer, record.Source)
	if err != nil {
		return record, err
	}
	if !producerHeld {
		return record, unresolvedKeyErr(record, record.Producer, "producer")
	}
	record.ProducerSHA = behaviorSHA(keyFile(record.Producer), producer)
	return record, nil
}

// unresolvedKeyErr says which key a record names that the tree does not hold.
func unresolvedKeyErr(record DiscriminationRecord, key, role string) error {
	var tb textbuf.Buffer
	return parseErr(tb.Str("the record for ").Str(record.RID).Byte(' ').Str(record.Polarity).
		Str(" names the ").Str(role).Byte(' ').Quoted(key).
		Str(", which this tree does not hold. There is nothing to fingerprint, and an empty ").
		Str("hash would make every absent unit look identical"))
}

// DiscriminationVerdict is one record and what the tree says about it now.
type DiscriminationVerdict struct {
	Record DiscriminationRecord `json:"record"`
	State  string               `json:"state"`
	// Detail names what moved, for the violation a reader has to act on.
	Detail string `json:"detail,omitempty"`
}

// Verified answers whether the tree still holds the code the red was observed
// against. A reader outside this package publishes proof and escape apart, and
// this is the one question that decides which.
func (v DiscriminationVerdict) Verified() bool { return v.State == ProofVerified }

// removable answers whether this record's TAG is gone from the tree.
//
// A record dies with the tag it proves, so a record whose tag was deleted or
// renamed has nothing left to be wrong about: it is reported for removal, never
// refused (AC-4). A record whose tag is STILL THERE and no longer verifies is
// the opposite -- a published claim the tree contradicts -- and that one is a
// violation.
func (v DiscriminationVerdict) removable() bool {
	return v.State == ProofUnitGone || v.State == ProofTagGone
}

// verifyDiscrimination re-checks every stored proof against the working tree.
//
// It executes NOTHING: no mutant, no `go test`, no scenario. `./le rfc check`
// is the third stage of every full verify, and an interop scenario alone costs
// 21 to 150 seconds warm. What it does is compare the fingerprints the proof
// was taken against, which is what checkAuditFreshness already does for an
// audit verdict.
//
// The consequence is stated rather than hidden: a verified record says the red
// WAS observed and the code it was observed over has not moved since. It does
// not say the red would happen again on a machine that never ran it.
func verifyDiscrimination(tree string, records []DiscriminationRecord,
	covers map[Cover][]Tag) ([]DiscriminationVerdict, error) {
	// carriersFor with no schedule, rather than carriers(tree): only the KIND
	// of a carrier is read here, and kind is a property of the path. The
	// schedule decides the TIER, so asking for it would make verification fail
	// on a tree that carries no .github/workflows and answer nothing more.
	table := carriersFor(FunctionalSuites(), nil)
	reader := newSourceReader(tree)
	index := newScopeIndex()
	verdicts := make([]DiscriminationVerdict, 0, len(records))
	// Indexed rather than ranged by value: a record is 176 bytes, and the
	// linter counts the copy at every iteration.
	for position := range records {
		verdict, err := verifyOneDiscrimination(reader, index, table, covers, records[position])
		if err != nil {
			return nil, err
		}
		verdicts = append(verdicts, verdict)
	}
	return verdicts, nil
}

// verifyOneDiscrimination answers the state of one record.
//
// The order is the order a reader can act on. The unit comes first, because a
// record whose tagged unit is gone has nothing left to prove and naming its
// producer would send the reader to the wrong file. The TAG comes next, for the
// same reason one step down: a unit that no longer carries the tag has nothing
// left to prove either, and both of those are removable rather than wrong. Only
// then do the fingerprints of a record that still has something to prove get
// compared.
func verifyOneDiscrimination(reader *sourceReader, index *scopeIndex, table []Carrier,
	covers map[Cover][]Tag, record DiscriminationRecord) (DiscriminationVerdict, error) {
	unit, held, err := resolveKeyText(reader.read, index, record.Unit, record.Source)
	if err != nil {
		return DiscriminationVerdict{}, err
	}
	if !held {
		return DiscriminationVerdict{Record: record, State: ProofUnitGone,
			Detail: "the tagged unit is no longer in the tree"}, nil
	}
	tagged := covers[record.Cover()]
	if len(tagged) == 0 {
		return DiscriminationVerdict{Record: record, State: ProofTagGone,
			Detail: "the unit is still in the tree and no longer carries this tag"}, nil
	}
	if behaviorSHA(keyFile(record.Unit), unit) != record.UnitSHA {
		return DiscriminationVerdict{Record: record, State: ProofUnitChanged,
			Detail: "the tagged unit's behavior changed since the red was observed"}, nil
	}
	if claimSHA(tagged) != record.ClaimSHA {
		return DiscriminationVerdict{Record: record, State: ProofClaimChanged,
			Detail: "the tag's claim was reworded, and a red observed against the old " +
				"sentence proves nothing about the new one"}, nil
	}
	carrier, carried := CarrierFor(keyFile(record.Unit), table)
	if !record.Proves() {
		escape := escapeCheck{reader: reader, index: index, carrier: carrier, carried: carried,
			unit: unit, tagged: tagged, record: record}
		return escape.verdict(), nil
	}

	producer, producerHeld, err := resolveKeyText(reader.read, index, record.Producer, record.Source)
	if err != nil {
		return DiscriminationVerdict{}, err
	}
	if !producerHeld {
		return DiscriminationVerdict{Record: record, State: ProofProducerGone,
			Detail: "the producer the break was applied to is no longer in the tree"}, nil
	}
	if behaviorSHA(keyFile(record.Producer), producer) != record.ProducerSHA {
		return DiscriminationVerdict{Record: record, State: ProofProducerChanged,
			Detail: "the producer's behavior changed since the break was applied to it"}, nil
	}
	if state, detail := citationState(carrier, carried, record, unit); state != ProofVerified {
		return DiscriminationVerdict{Record: record, State: state, Detail: detail}, nil
	}
	return DiscriminationVerdict{Record: record, State: ProofVerified}, nil
}

// citationState judges the assertion a functional or interop proof rests on.
//
// A generated break cannot reach either carrier: gomu runs unit tests only, and
// no operator rewrites a scenario. What ties the recorded red to a named
// assertion instead is the citation, so those two kinds owe one and the gate
// goes and looks for it. A unit record owes none, and carrying one would put an
// unchecked string beside a checked proof.
//
// The interop checkers already number their assertions through `fail(N, cause)`
// and render `assertion %d`, so the citation is that number and the count of
// numbered sites in the tagged unit is its bound. A `.ci` has no numbering, so
// the citation is the directive line itself and the file is searched for it.
func citationState(carrier Carrier, carried bool, record DiscriminationRecord, unit string) (string, string) {
	if !carried || carrier.Kind == kindUnit {
		if record.Citation == "" {
			return ProofVerified, ""
		}
		return ProofCitationGone, "a unit record cites an assertion, and nothing checks it: " +
			"its proof is the break, which the gate does check"
	}
	if record.Citation == "" {
		var tb textbuf.Buffer
		return ProofCitationGone, tb.Str("a ").Str(carrier.Kind).
			Str(" record cites no assertion. No generated break reaches that carrier, so the ").
			Str("citation is what ties the recorded red to one assertion rather than to the ").
			Str("whole suite").String()
	}
	if carrier.Kind == kindInterop {
		return interopCitationState(record.Citation, unit)
	}
	if strings.Contains(unit, record.Citation) {
		return ProofVerified, ""
	}
	var tb textbuf.Buffer
	return ProofCitationGone, tb.Str("the cited directive ").Quoted(record.Citation).
		Str(" is not in ").Str(keyFile(record.Unit)).String()
}

// interopCitationState bounds a cited assertion by the numbered sites the
// tagged checker holds.
func interopCitationState(citation, unit string) (string, string) {
	var tb textbuf.Buffer
	cited, err := strconv.Atoi(citation)
	if err != nil {
		return ProofCitationGone, tb.Str("the citation ").Quoted(citation).
			Str(" is not an assertion number. An interop checker numbers its assertions ").
			Str("through `fail(N, cause)` and renders `assertion %d`, so the citation is that N").String()
	}
	sites := interopAssertionRE.FindAllStringSubmatch(unit, -1)
	numbered := make([]string, 0, len(sites))
	for _, site := range sites {
		if site[1] == citation {
			return ProofVerified, ""
		}
		numbered = append(numbered, site[1])
	}
	return ProofCitationGone, tb.Str("assertion ").Int(int64(cited)).
		Str(" is not one of the `fail(N, ...)` sites the cited checker holds (").
		Join(numbered, ", ").Byte(')').String()
}

// interopAssertionRE matches one LITERALLY numbered assertion site in an
// interop checker.
//
// The numbering was built for the error message `checkerFailure` renders, and a
// citation points at it rather than at a line, so an edit above the assertion
// cannot rot the record.
//
// The set is what a citation is checked against, not its size, because the
// checkers number some assertions by expression: `fail(index+2, err)` inside a
// loop over four observers covers 2 through 5, and counting literals would then
// bound a 9-assertion checker at 4. The consequence is stated rather than
// hidden: an assertion whose number is computed cannot be cited, and a checker
// that wants one of its own citable writes the number out.
var interopAssertionRE = regexp.MustCompile(`\bfail\(\s*(\d+)\s*,`)

// resolveKeyText answers the text one fingerprint key names, and false when the
// tree no longer holds it.
//
// An empty unit is never an answer, for the reason unitSHAs states: hashing
// nothing gives every missing file one fingerprint, so a deleted test would
// read as unchanged. Here that would publish a proof of a test that is gone.
//
// The error return is for a key that does not parse. Every key loaded through
// validateDiscrimination already did, so an error here says the record reached
// this walk without passing that gate, and refusing is the only answer that
// cannot publish a proof nobody checked.
//
// A lookup rather than a reader, because the same key is resolved against two
// texts of one file: the working tree, through sourceReader.read, and HEAD,
// through the blobs check_baseline.go already reads. One resolver for both keeps
// "what text does this key name" a single answer.
func resolveKeyText(lookup func(string) *string, index *scopeIndex,
	key, where string) (string, bool, error) {
	rel, symbol, err := fingerprintKey(key, where)
	if err != nil {
		return "", false, err
	}
	content := lookup(rel)
	if content == nil || *content == "" {
		return "", false, nil
	}
	if symbol == "" {
		return *content, true, nil
	}
	found := index.funcTexts(*content, symbol)
	if len(found) != 1 || found[0] == "" {
		return "", false, nil
	}
	return found[0], true, nil
}

// discriminationRouteCounts answers how many records prove and how many escape.
//
// A record that no longer VERIFIES is neither. It is refused by the ratchet, so
// the counts are never published beside it, and counting it here anyway would
// leave the one number a reader trusts derivable from an unchecked claim
// (ai/rules/principles.md).
//
// Proof and escape are counted APART wherever they are published, because a
// claim that no break exists is debt rather than evidence (R-9).
func discriminationRouteCounts(verdicts []DiscriminationVerdict) (proven, escaped int) {
	for index := range verdicts {
		verdict := &verdicts[index]
		if !verdict.Verified() {
			continue
		}
		if verdict.Record.Proves() {
			proven++
			continue
		}
		escaped++
	}
	return proven, escaped
}

// provenCovers answers the coverage the VERIFYING records carry.
//
// A record that no longer verifies covers nothing, so its cover is absent here:
// reading a stale record as coverage is the fail-open shape, and both the
// obligation and the changed-unit measurement ask this one question.
func provenCovers(verdicts []DiscriminationVerdict) map[Cover]bool {
	out := make(map[Cover]bool, len(verdicts))
	for index := range verdicts {
		if verdicts[index].Verified() {
			out[verdicts[index].Record.Cover()] = true
		}
	}
	return out
}

// discriminationOwedTags answers one tag per tagged unit that head added
// against baseline and has not proven, in file and line order.
//
// Both sides are COMMITTED. head is the tip commit's cover set, so a tag
// sitting only in somebody's working tree is outside this answer entirely:
// several sessions share this checkout, and billing a tag nobody has committed
// reds the gate for every one of them over an edit that is not theirs, which is
// the failure R-8 names. The author still meets it, at their own commit, where
// `./le verify worktree` checks that commit out detached and the tag it added
// IS the tip.
//
// The baseline decides what the answer MEANS, and two callers pass two: the
// commit before the tip, which is the obligation the ratchet bills, and the
// pushed branch, which is the backlog the report publishes. One predicate for
// both, so the measurement cannot drift from the rule it measures.
//
// A baseline that could not be read accuses nobody, which is why known is a
// parameter rather than a length test: a revision holding no tags at all and a
// revision git could not open are opposite answers.
//
// Only a GATED requirement of an ENROLLED RFC obliges. This gate exists for the
// MUST-level population, so a tag on a SHOULD is evidence nobody was owed and
// a proof nobody is owed either.
//
// A record that no longer verifies covers nothing, so the tag it named is owed
// again. Reading a stale record as coverage is the fail-open shape.
//
// One tag per COVER, because the violation is about the unit: a unit carrying
// three tags for one requirement and polarity owes one record, not three.
func discriminationOwedTags(head map[Cover][]Tag, baseline map[Cover]bool, known bool,
	verdicts []DiscriminationVerdict, gated map[string]bool) []Tag {
	if !known {
		return nil
	}
	proven := provenCovers(verdicts)

	var owed []Tag
	for key, tags := range head {
		if baseline[key] || proven[key] || !gated[key.RID] || len(tags) == 0 {
			continue
		}
		owed = append(owed, tags[0])
	}
	sort.Slice(owed, func(left, right int) bool {
		if owed[left].File != owed[right].File {
			return owed[left].File < owed[right].File
		}
		return owed[left].Line < owed[right].Line
	})
	return owed
}

// discriminationStatus is what one selector has proven and what it still owes.
//
// Stale is kept apart from Records because a record that no longer verifies has
// proven nothing: its tag is back in Unproven, and reading it as a proof is the
// answer this artifact exists to remove. `./le rfc check` says what moved.
type discriminationStatus struct {
	Selector string                 `json:"selector"`
	Records  []DiscriminationRecord `json:"records"`
	Stale    []DiscriminationRecord `json:"stale"`
	Unproven []Tag                  `json:"unproven"`
	// Candidates are the breaks that could prove the unproven unit tags, filled
	// only when a gomu report was named. Proposing is reading: this answer
	// writes nothing, and `./le rfc discriminate-record` is what records one.
	Candidates []DiscriminationCandidate `json:"candidates"`
}

// discriminationStatusOf answers the state of one RFC stem or one requirement id.
//
// selected decides the population, so one walk of the corpus serves both
// keywords and neither can drift from the other.
func discriminationStatusOf(tree, selector, report string,
	selected func(rid string) bool) (discriminationStatus, error) {
	records, err := loadDiscrimination(tree)
	if err != nil {
		return discriminationStatus{}, err
	}
	covers, err := tagCoversIn(tree)
	if err != nil {
		return discriminationStatus{}, err
	}
	verdicts, err := verifyDiscrimination(tree, records, covers)
	if err != nil {
		return discriminationStatus{}, err
	}

	status := discriminationStatus{Selector: selector, Records: []DiscriminationRecord{},
		Stale: []DiscriminationRecord{}, Unproven: []Tag{},
		Candidates: []DiscriminationCandidate{}}
	covered := map[Cover]bool{}
	for index := range verdicts {
		verdict := &verdicts[index]
		if !selected(verdict.Record.RID) {
			continue
		}
		if !verdict.Verified() {
			status.Stale = append(status.Stale, verdict.Record)
			continue
		}
		status.Records = append(status.Records, verdict.Record)
		covered[verdict.Record.Cover()] = true
	}
	for key, tagged := range covers {
		if !selected(key.RID) || covered[key] {
			continue
		}
		status.Unproven = append(status.Unproven, tagged...)
	}
	sort.Slice(status.Unproven, func(left, right int) bool {
		if status.Unproven[left].File != status.Unproven[right].File {
			return status.Unproven[left].File < status.Unproven[right].File
		}
		return status.Unproven[left].Line < status.Unproven[right].Line
	})
	if report == "" {
		return status, nil
	}
	status.Candidates, err = proposeBreaks(tree, report, status.Unproven)
	if err != nil {
		return discriminationStatus{}, err
	}
	return status, nil
}
