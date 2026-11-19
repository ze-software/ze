// Design: docs/contributing/rfc-conformance-gates.md -- the commit gate over the records a commit stales
// Overview: discriminate.go -- the record, its fingerprints and the verifier this gate reuses
// Related: register.go -- where this gate registers into the changecheck seam
//
// discriminate_commit.go is the commit gate for the discrimination records:
// `./le commit create` refuses a commit that changes a tagged test or its
// claim without carrying the record `./le rfc record` re-took
// (ai/META-DECISIONS.md, "Test changes"). The proof belongs to the test, so a
// commit that changes only the code the test checks, the producer the record
// broke included, stales nothing and this gate reads nothing for it. It judges
// what the commit itself does, so it is a refusal and never a red: a record the
// parent already held stale is debt the commit did not make, and it refuses
// nothing here.

package rfc

import (
	"errors"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/commit/changecheck"
)

// ze point: rfc-compliance/directives/prove-a-new-tag-discriminates-its-claim
// judgeStaleRecords answers the refusal for every discrimination record this
// commit stales, or "" when it stales none.
//
// A record is judged only when the commit can have staled it: its tagged unit
// sits in a carrier file the commit carries or removes, or the commit carries
// or removes the record's own file. A producer file is not one of those: a
// change to it leaves the unchanged test catching what it caught.
// It is staled when it verifies against the parent and does not verify against
// what the commit leaves. The record the commit leaves is the one its After
// side holds for the same requirement, polarity and unit, so a re-taken record
// the commit carries is judged instead of the old one, and clears the refusal
// exactly when its fingerprints match the code the commit leaves.
//
// A record whose tag the commit removes dies with it (ProofUnitGone,
// ProofTagGone) and is accepted here: removing a tag is the judge's to approve,
// through the RFC-tagged test change gate. A record the commit deletes while
// its tag stays is refused, because that takes a proof off the ledger and
// leaves the claim behind it.
//
// The verdict is verifyOneDiscrimination's over each side, so this gate holds
// no second fingerprint and no second definition of stale.
func judgeStaleRecords(change changecheck.Change) (string, error) {
	gate := newRecordGate(change)
	if !gate.touchesAnyRecord() {
		return "", nil
	}
	if change.BeforeUnder == nil {
		return "", errors.New("the commit gate supplied no batch reader of the parent's records, " +
			"so the RFC proof records this commit stales cannot be found")
	}
	held, err := change.BeforeUnder(discriminationRel)
	if err != nil {
		return "", err
	}
	refusals, err := gate.judge(held)
	if err != nil {
		return "", err
	}
	if len(refusals) == 0 {
		return "", nil
	}
	return staleRecordsText(refusals), nil
}

// recordGate is one commit as this gate reads it: the files it changes, the
// parent and the tree it leaves, each read lazily through the seam.
type recordGate struct {
	change changecheck.Change
	// units are the changed carrier paths a tagged unit can sit in. A record's
	// producer is not judged, so the files producers sit in are not collected.
	units map[string]bool
	// records are the changed paths under rfc/discrimination.
	records map[string]bool
	carried map[string]bool
	removed map[string]bool
	table   []Carrier
	index   *scopeIndex
	before  *recordSide
	after   *recordSide
}

// recordSide is one side of the commit: a reader over its files, the tagged
// units of each file read so far, and the first read that failed.
type recordSide struct {
	reader *sourceReader
	covers map[string]map[Cover][]Tag
	failed error
}

func newRecordGate(change changecheck.Change) *recordGate {
	gate := &recordGate{
		change:  change,
		units:   map[string]bool{},
		records: map[string]bool{},
		carried: map[string]bool{},
		removed: map[string]bool{},
		// The carrier KIND is a property of the path, so no schedule is read:
		// verifyDiscrimination resolves carriers the same way.
		table: carriersFor(FunctionalSuites(), nil),
		index: newScopeIndex(),
	}
	for _, rel := range change.Paths {
		gate.carried[rel] = true
		gate.classify(rel)
	}
	for _, rel := range change.Removed {
		gate.removed[rel] = true
		gate.classify(rel)
	}
	gate.before = &recordSide{covers: map[string]map[Cover][]Tag{}}
	gate.before.reader = newSideReader(func(rel string) *string { return gate.before.fetch(change.Before, rel) })
	gate.after = &recordSide{covers: map[string]map[Cover][]Tag{}}
	gate.after.reader = newSideReader(gate.afterText)
	return gate
}

// touchesAnyRecord answers whether the commit changes a carrier a tagged unit
// can sit in or a record file. A commit that changes neither stales no record,
// so the gate reads nothing for it: a commit that changes only producer code
// lands here.
func (g *recordGate) touchesAnyRecord() bool {
	if len(g.units) > 0 {
		return true
	}
	return len(g.records) > 0
}

// classify files one changed path as a record file, a carrier a tagged unit can
// sit in, or neither. A Go file that is no carrier holds producers only, and a
// producer change stales no record.
func (g *recordGate) classify(rel string) {
	if _, record := evidenceAt(discriminationRel, rel); record {
		g.records[rel] = true
		return
	}
	if _, carrier := CarrierFor(rel, g.table); carrier {
		g.units[rel] = true
	}
}

// afterText reads one path of the tree the commit leaves: absent where the
// commit removes it, the carried selection where the commit carries it, and
// the parent's text everywhere else. The seam's After reads the working tree
// for a path the commit does not carry, which holds other sessions' edits.
func (g *recordGate) afterText(rel string) *string {
	if g.removed[rel] {
		return nil
	}
	if g.carried[rel] {
		return g.after.fetch(g.change.After, rel)
	}
	return g.before.reader.read(rel)
}

// fetch reads one path through a seam side, keeping the first failure so the
// gate answers it as an error rather than as a file the side does not hold.
func (s *recordSide) fetch(side changecheck.Side, rel string) *string {
	text, present, err := side(rel)
	if err != nil {
		if s.failed == nil {
			s.failed = err
		}
		return nil
	}
	if !present {
		return nil
	}
	return &text
}

// coversOf answers the tagged units of one file on this side, gap tags left
// out because they prove nothing, keyed as tagCoversIn keys them.
func (s *recordSide) coversOf(rel string, table []Carrier, index *scopeIndex) map[Cover][]Tag {
	if found, held := s.covers[rel]; held {
		return found
	}
	found := map[Cover][]Tag{}
	if text := s.reader.read(rel); text != nil {
		proof, _ := splitGapTags(scanTagsTolerant(*text, rel, table))
		found = coversOfTags(index, proof, map[string]string{rel: *text})
	}
	s.covers[rel] = found
	return found
}

// verdict answers what this side says about one record.
func (g *recordGate) verdict(side *recordSide, record *DiscriminationRecord) (DiscriminationVerdict, error) {
	covers := side.coversOf(keyFile(record.Unit), g.table, g.index)
	verdict, err := verifyOneDiscrimination(side.reader, g.index, g.table, covers, record)
	if err != nil {
		return DiscriminationVerdict{}, err
	}
	if err := g.failed(); err != nil {
		return DiscriminationVerdict{}, err
	}
	return verdict, nil
}

// failed answers the first read of either side that failed. The tree the
// commit leaves reads the parent for every path the commit does not change,
// so a failure on the parent can surface while that tree is read.
func (g *recordGate) failed() error {
	if g.before.failed != nil {
		return g.before.failed
	}
	return g.after.failed
}

// concerns is the cheap filter over one record file of the parent: the commit
// changes the file itself, or its raw text spells a changed carrier. A file
// that does neither holds no record this commit can stale, so only the files
// that do are decoded.
func (g *recordGate) concerns(rel, text string) bool {
	if g.records[rel] {
		return true
	}
	for unit := range g.units {
		if strings.Contains(text, unit) {
			return true
		}
	}
	return false
}

// judges answers whether one record of a file the commit concerns is judged:
// the commit changes the record's own file, or the carrier its tagged unit
// sits in. A changed file that only its producer names is not enough, so a
// record file that spells a changed path in its producer key is decoded and
// its records then pass here unjudged.
func (g *recordGate) judges(rel string, record *DiscriminationRecord) bool {
	if g.records[rel] {
		return true
	}
	return g.units[keyFile(record.Unit)]
}

// staleRecord is one record the commit stales, and what it left.
type staleRecord struct {
	record  DiscriminationRecord
	stem    string
	verdict DiscriminationVerdict
	// withdrawn is a record the commit deletes while its tag stays.
	withdrawn bool
	// unreadable is the load error of the record file the commit carries.
	unreadable error
}

// judge answers every record of the parent's corpus held that this commit
// stales, in record-file order.
func (g *recordGate) judge(held map[string]string) ([]staleRecord, error) {
	files := make([]string, 0, len(held))
	for rel := range held {
		files = append(files, rel)
	}
	slices.Sort(files)
	var out []staleRecord
	for _, rel := range files {
		file, isRecord := evidenceAt(discriminationRel, rel)
		if !isRecord {
			continue
		}
		if !g.concerns(rel, held[rel]) {
			continue
		}
		found, err := g.judgeFile(file, held[rel])
		if err != nil {
			return nil, err
		}
		out = append(out, found...)
	}
	return out, nil
}

// judgeFile answers the records of one requirement's file that this commit
// stales.
//
// A parent file that does not load proves nothing at the parent, so the
// commit stales none of it; `./le rfc check` refuses that file on its own.
func (g *recordGate) judgeFile(file evidenceFile, parentText string) ([]staleRecord, error) {
	parent, err := decodeDiscrimination(parentText, file)
	if err != nil {
		return nil, nil //nolint:nilerr // an unloadable parent file is debt the commit did not make
	}
	left, leftErr := g.leftRecords(file)
	if err := g.failed(); err != nil {
		return nil, err
	}
	var out []staleRecord
	for position := range parent {
		record := &parent[position]
		if !g.judges(file.Rel, record) {
			continue
		}
		was, err := g.verdict(g.before, record)
		if err != nil {
			return nil, err
		}
		if !was.Verified() {
			continue
		}
		found, stale, err := g.judgeLeft(file, record, left, leftErr)
		if err != nil {
			return nil, err
		}
		if stale {
			out = append(out, found)
		}
	}
	return out, nil
}

// leftRecords answers the records the commit leaves in one requirement's
// file, nil where it leaves no file, and the load error of a carried file that
// does not load.
func (g *recordGate) leftRecords(file evidenceFile) ([]DiscriminationRecord, error) {
	text := g.afterText(file.Rel)
	if text == nil {
		return nil, nil
	}
	return decodeDiscrimination(*text, file)
}

// judgeLeft judges one record the parent proved against what the commit
// leaves, and answers whether the commit stales it.
func (g *recordGate) judgeLeft(file evidenceFile, record *DiscriminationRecord,
	left []DiscriminationRecord, leftErr error) (staleRecord, bool, error) {
	found := staleRecord{record: *record, stem: file.Stem}
	if leftErr != nil {
		found.unreadable = leftErr
		return found, true, nil //nolint:nilerr // a carried record that does not load is a refusal naming its error, not a gate failure
	}
	cover := record.Cover()
	position := -1
	// Indexed rather than ranged by value: a record is 176 bytes.
	for index := range left {
		if left[index].Cover() == cover {
			position = index
			break
		}
	}
	if position < 0 {
		now, err := g.verdict(g.after, record)
		if err != nil {
			return found, false, err
		}
		if now.removable() {
			return found, false, nil
		}
		found.verdict, found.withdrawn = now, true
		return found, true, nil
	}
	now, err := g.verdict(g.after, &left[position])
	if err != nil {
		return found, false, err
	}
	if now.Verified() {
		return found, false, nil
	}
	if now.removable() {
		return found, false, nil
	}
	found.record, found.verdict = left[position], now
	return found, true, nil
}

// staleRecordsText renders the refusal: what is stale, why it matters, and
// the command that re-takes each record (ai/rules/user-facing-errors.md).
func staleRecordsText(stale []staleRecord) string {
	var tb textbuf.Buffer
	tb.Str("This commit stales ").Int(int64(len(stale))).Str(" RFC proof record(s). A record proves ").
		Str("that its tagged test was seen to go red under a break of the code it checks, and its ").
		Str("fingerprints say which test and which claim that was. This commit changes the test or ").
		Str("its claim, or deletes the record while the tag stays, so the record would publish a ").
		Str("proof nobody observed. Carry the record `./le rfc record` re-takes in this commit ").
		Str("(the main thread runs it; an agent lists it as owed), or remove the tag with the ").
		Str("ze-test-judge's approval.\n")
	for position := range stale {
		tb.Str(staleRecordText(&stale[position]))
	}
	return strings.TrimSuffix(tb.String(), "\n")
}

// staleRecordText renders one stale record and its re-take.
func staleRecordText(stale *staleRecord) string {
	record := &stale.record
	var tb textbuf.Buffer
	tb.Str("- ").Str(discriminationRecordRel(stale.stem, record.RID)).Str(": the ").Str(record.Route).
		Str(" record for ").Str(record.RID).Byte(' ').Str(record.Polarity).Str(" at ").Str(record.Unit)
	if stale.unreadable != nil {
		return tb.Str(" cannot be read in the file this commit carries: ").Err(stale.unreadable).
			Str(". Repair the file, or re-take the record:\n    ").Str(retakeLines(stale)).Byte('\n').String()
	}
	if stale.withdrawn {
		tb.Str(" is deleted by this commit while its tag stays, which takes a proof off the ledger ").
			Str("and leaves the claim behind it. Restore the record, re-take it, or remove the tag:\n    ")
		return tb.Str(retakeLines(stale)).Byte('\n').String()
	}
	verdict := &stale.verdict
	tb.Str(" goes stale (").Str(verdict.State).Str(", ").Str(driftedKey(verdict)).Str("): ").
		Str(verdict.Detail).Str(". Re-take it on the route it was recorded on")
	return tb.Str(":\n    ").Str(retakeLines(stale)).Byte('\n').String()
}

// retakeLines answers the command lines that re-take one record on the route
// it was recorded on, with every argument the record holds written out from it.
func retakeLines(stale *staleRecord) string {
	record := &stale.record
	var tb textbuf.Buffer
	head := "./le rfc record " + keyID + " " + recordWord(record.RID) + " " + keyPolarity + " " +
		recordWord(record.Polarity) + " " + keyUnit + " " + recordWord(record.Unit) + " " + keyRoute + " "
	switch record.Route {
	case RouteRevert:
		tb.Str(head).Str(RouteRevert).Byte(' ').Str(keyProducer).Byte(' ').Str(recordWord(record.Producer))
		appendCitation(&tb, record)
	case RouteMutant:
		report := "<gomu-report.json>"
		tb.Str("./le rfc discriminate stem ").Str(recordWord(stale.stem)).Byte(' ').Str(keyReport).
			Byte(' ').Str(report).Str("\n    ")
		tb.Str(head).Str(RouteMutant).Byte(' ').Str(keyReport).Byte(' ').Str(report)
		for range 1 + len(record.Paired) {
			tb.Byte(' ').Str(keyMutant).Str(" <file:line:column#n>")
		}
		appendCitation(&tb, record)
		tb.Str("\n    The record's break was ").Quoted(record.Break).Str(" in ").Str(record.Producer)
		for _, paired := range record.Paired {
			tb.Str(", with ").Quoted(paired.Break).Str(" in ").Str(paired.Producer)
		}
		tb.Str(": take the proposed mutant that makes the same break in the code as the commit leaves it.")
	case RouteNoBreak:
		tb.Str(head).Str(RouteNoBreak).Byte(' ').Str(keyReason).Byte(' ').Str(recordWord(record.Reason))
		if record.Producer != "" {
			tb.Byte(' ').Str(keyProducer).Byte(' ').Str(recordWord(record.Producer))
		}
		appendCitation(&tb, record)
		for _, function := range record.KernelFunctions {
			tb.Byte(' ').Str(keyKernelFunction).Byte(' ').Str(recordWord(function))
		}
		if record.LaterRoute != "" {
			tb.Byte(' ').Str(keyLaterRoute).Byte(' ').Str(recordWord(record.LaterRoute))
		}
		for _, excluded := range record.ZeStateExcluded {
			tb.Byte(' ').Str(keyZeStateExcluded).Byte(' ').Str(recordWord(excluded.Producer + " " + excluded.Reason))
		}
	default:
		// A route outside the closed set never loads (validateDiscrimination),
		// so no record reaching here can hold one.
		panic("BUG: retakeLines: a loaded record with route " + record.Route)
	}
	return tb.String()
}

// appendCitation writes the citation a functional or interop record rests on.
func appendCitation(tb *textbuf.Buffer, record *DiscriminationRecord) {
	if record.Citation == "" {
		return
	}
	tb.Byte(' ').Str(keyCitation).Byte(' ').Str(recordWord(record.Citation))
}

// recordWord answers one argument as a shell reads it back: bare when every
// byte is one a shell leaves alone, single-quoted otherwise.
func recordWord(value string) string {
	plain := value != ""
	for index := range len(value) {
		if !shellPlainByte(value[index]) {
			plain = false
			break
		}
	}
	if plain {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

// shellPlainByte reports whether a shell reads one byte as itself.
func shellPlainByte(char byte) bool {
	if goNameByte(char) {
		return true
	}
	return strings.IndexByte("./:#@%+=,-", char) >= 0
}
