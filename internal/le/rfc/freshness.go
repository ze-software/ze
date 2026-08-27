// Design: docs/architecture/core-design.md -- what makes a recorded verdict current
// Overview: rfc.go -- the types, the paths and the closed sets every reader here shares
//
// freshness.go decides which of four states a recorded audit verdict is in.
// The record it judges is read by audit.go, and the unit each fingerprint
// names is defined by goscope.go.
//
// Replacing a boolean is the whole point. `fresh` and `stale` collapsed a
// mechanical re-stamp -- nothing about what a test asserts moved -- into the
// same signal as a real judgement change, and the only remedy either offered
// was a human re-read. At fleet scale that trains the reflex of re-stamping
// whenever the gate goes red, and the reflex is what fails.
//
// Every rule here is biased to over-trigger: a false 'stale' costs a re-read, a
// false 'fresh' ships a test that no longer enforces its requirement.
package rfc

import (
	"maps"
	"os"
	"sort"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The four freshness states, mutually exclusive and total.
const (
	// FreshState: nothing the verdict judged has moved.
	FreshState = "fresh"
	// ShiftedState: the units are identical and the enclosing file moved. This
	// one is mechanically re-sealable.
	ShiftedState = "shifted"
	// StaleUnitState: the tagged unit itself, or cited producer code, changed.
	StaleUnitState = "stale-unit"
	// StaleRequirementState: the RFC obligation's own text changed.
	StaleRequirementState = "stale-requirement"
)

// FreshnessStates answers the four, in the order the module declares them, for
// the value comparison against it.
func FreshnessStates() []string {
	return []string{FreshState, ShiftedState, StaleUnitState, StaleRequirementState}
}

// Freshness is one verdict's state and the fingerprint keys that moved.
type Freshness struct {
	State string   `json:"state"`
	Moved []string `json:"moved"`
}

// sourceReader reads each tracked file at most once per run.
//
// It keeps "unreadable" apart from "empty", because the two mean different
// things to the two fingerprint producers: an unreadable file stores "" in the
// tag-level map, which is safe there because it compares unequal to whatever
// was recorded, and it is REFUSED by the unit-level one, where the same value
// would be a legitimate-looking answer.
type sourceReader struct {
	tree  string
	cache map[string]*string
}

func newSourceReader(tree string) *sourceReader {
	return &sourceReader{tree: tree, cache: map[string]*string{}}
}

// read answers a file's text, or nil when it could not be read.
func (r *sourceReader) read(rel string) *string {
	found, held := r.cache[rel]
	if held {
		return found
	}
	raw, err := os.ReadFile(treePath(r.tree, rel)) // #nosec G304 -- a path the verdict names, validated by fingerprintKey
	if err != nil {
		r.cache[rel] = nil
		return nil
	}
	// errors="replace" is the module's spelling: a file with a bad byte is
	// still fingerprinted rather than refused, and Go's []byte to string
	// conversion keeps the same bytes, which hash the same way.
	text := string(raw)
	r.cache[rel] = &text
	return &text
}

// text answers a file's content with "" for one that could not be read.
func (r *sourceReader) text(rel string) string {
	if found := r.read(rel); found != nil {
		return *found
	}
	return ""
}

// mintTagKeys answers one key per tag, in tag order: `path::FuncName`,
// `path::FuncName#2`, ... or a bare path.
//
// The ONE place the key form is minted, and the reason it takes a whole
// SEQUENCE rather than one tag: the ordinal counts tags WITHIN a unit, so it
// cannot be derived from a tag alone. Two tags inside one function share a
// symbol, and without the ordinal they would share a key too -- which makes the
// map compare equal after one of them is deleted, and a verdict over a test
// that no longer exists reads FRESH.
func mintTagKeys(tags []Tag, index *scopeIndex, contentOf func(string) string) []string {
	seen := map[string]int{}
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		name := index.funcNameAt(tag.File, contentOf(tag.File), tag.Line)
		var tb textbuf.Buffer
		base := tag.File
		if name != "" {
			base = tb.Str(tag.File).Str("::").Str(name).String()
		}
		seen[base]++
		if seen[base] == 1 {
			out = append(out, base)
			continue
		}
		var ordinal textbuf.Buffer
		out = append(out, ordinal.Str(base).Byte('#').Int(int64(seen[base])).String())
	}
	return out
}

// tagKeys answers the keys a requirement's tags fingerprint under.
//
// Each tagged file is read once: the enclosing function's NAME is a property of
// the file, not of the tag, so it cannot be derived from the Tag alone the way
// a file and a line could.
func tagKeys(tags []Tag, reader *sourceReader, index *scopeIndex) []string {
	return mintTagKeys(tags, index, reader.text)
}

// taggedUnitSHAs fingerprints each tagged test, keyed by the symbol the tag
// sits in. The VALUE is the whole enclosing FILE, coarse on purpose:
// over-triggering costs a re-read, under-triggering ships a verdict for a test
// that has since changed.
func taggedUnitSHAs(tags []Tag, reader *sourceReader, index *scopeIndex) map[string]string {
	keys := mintTagKeys(tags, index, reader.text)
	out := make(map[string]string, len(keys))
	for i, key := range keys {
		content := reader.read(tags[i].File)
		if content == nil {
			out[key] = ""
			continue
		}
		out[key] = RequirementSHA(*content)
	}
	return out
}

// unitSHAs fingerprints the UNIT each key NAMES, keyed by that same string.
//
// The unit is one top-level Go function (its doc comment through its closing
// brace) or the whole file, decided by the key itself. This is the fix for the
// false-stale class: hashing the whole enclosing file sent a verdict stale on
// any edit anywhere in a tagged file and on any line shift, and six of a
// pending sixteen commits to the one existing audit file were mechanical
// re-stamps where nothing about what a test asserts had changed.
//
// An EMPTY unit is an error, never a hash input. Hashing "" would give every
// unreadable file the same fingerprint, so a deleted test would read as
// unchanged -- a false FRESH, the one catastrophic outcome. The unit is found
// by NAME, so a key survives every edit above the function it names and dies
// exactly when that function is renamed or removed.
func unitSHAs(keys []string, reader *sourceReader, index *scopeIndex, where string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		rel, symbol, err := fingerprintKey(key, where)
		if err != nil {
			return nil, err
		}
		content := reader.text(rel)
		var tb textbuf.Buffer
		if content == "" {
			return nil, parseErr(tb.Str(where).Str(": ").Str(key).Str(" names ").Str(rel).
				Str(", which is missing or empty. There is no unit to fingerprint, and ").
				Str("hashing nothing would make every missing file look identical and ").
				Str("therefore unchanged"))
		}
		text := content
		if symbol != "" {
			found := index.funcTexts(content, symbol)
			if len(found) != 1 {
				var why textbuf.Buffer
				if len(found) == 0 {
					why.Str("no top-level function declares it")
				} else {
					why.Int(int64(len(found))).Str(" top-level functions declare it")
				}
				return nil, parseErr(tb.Str(where).Str(": ").Str(key).Str(" names symbol ").
					Str(pyRepr(symbol)).Str(" in ").Str(rel).Str(", where ").Str(why.String()).
					Str(". A verdict judged that function, so a rename or a removal is a ").
					Str("change to what was judged: re-read it with the ze-rfc-audit skill ").
					Str("(ai/skills/ze-rfc-audit.md) rather than re-pointing the key"))
			}
			text = found[0]
		}
		if text == "" {
			return nil, parseErr(tb.Str(where).Str(": ").Str(key).
				Str(" resolved to an empty unit in ").Str(rel).
				Str("; refusing to fingerprint an empty extraction"))
		}
		out[key] = RequirementSHA(text)
	}
	return out, nil
}

// recordedMap answers one RECORDED fingerprint map, with absent and empty as
// the same state.
//
// The load-time reader established that normalisation ("absent reads as
// empty"), and this is the same one at COMPARISON time. They did not always
// agree: the freshness rule compared the raw field against a computed empty
// map, and in Python a missing key is not an empty dict. The cost was a
// permanently red gate on the one state owner ruling OR-1 created -- a
// `not-applicable` verdict cites no test, the skill tells the author to omit
// the field, and the omitted spelling then read stale forever with a message
// that was false in all three of its clauses.
//
// A wrong TYPE reads as empty here rather than refusing: the load already
// refused it, and this sits on the LEDGER RENDER path where refusing would take
// down a report that must stay readable.
func recordedMap(verdict map[string]any, key string) map[string]string {
	nested, isObject := verdict[key].(map[string]any)
	if !isObject {
		return map[string]string{}
	}
	out := make(map[string]string, len(nested))
	for name, value := range nested {
		text, isText := value.(string)
		if !isText {
			// Unreachable through the load, which refuses a non-string value.
			// Reproduced anyway: a value that is not a fingerprint compares
			// unequal to every computed one, which is more checking.
			continue
		}
		out[name] = text
	}
	return out
}

// unitIdentity answers a unit map as a multiset of (file, unit-sha) -- the
// identity a RENAME does not preserve.
//
// Comparing the maps key by key was wrong while a key held a line: prepending a
// nine-line header changed every key in the file, so every verdict in it
// compared unequal and reported stale while the recorded shas were identical.
// That IS the false-stale class the four states exist to remove.
//
// A COUNT, not a set. Two keys resolving to one (file, sha) pair must not
// collapse, or dropping one of them would read as unchanged -- a false FRESH.
func unitIdentity(fingerprints map[string]string) map[[2]string]int {
	out := map[[2]string]int{}
	for key, sha := range fingerprints {
		out[[2]string{keyFile(key), sha}]++
	}
	return out
}

// movedKeys answers the keys whose recorded fingerprint differs from the
// computed one, sorted, falling back to the symmetric difference of the two key
// sets when every shared key agrees.
func movedKeys(recorded, current map[string]string) []string {
	var out []string
	for key := range union(recorded, current) {
		if recorded[key] != current[key] {
			out = append(out, key)
		}
	}
	if len(out) == 0 {
		for key := range union(recorded, current) {
			_, inRecorded := recorded[key]
			_, inCurrent := current[key]
			if inRecorded != inCurrent {
				out = append(out, key)
			}
		}
	}
	sort.Strings(out)
	return out
}

// union answers the key set of two fingerprint maps.
func union(left, right map[string]string) map[string]bool {
	out := make(map[string]bool, len(left)+len(right))
	for key := range left {
		out[key] = true
	}
	for key := range right {
		out[key] = true
	}
	return out
}

// verdictFreshness answers which of the four states a verdict is in, and which
// keys moved.
//
// THE freshness rule -- there is no second spelling of it. Order matters and is
// the same over-triggering bias: the requirement's own text is checked first
// (re-reading the RFC invalidates every judgement under it), then the units,
// then the producing code, and only then the file-level shift, so a real
// judgement change can never be reported as the cheap mechanical case.
//
// A verdict with no `units` (recorded before unit fingerprints existed) takes
// exactly the old file-level rule, applied inline below, so pre-existing
// records keep behaving as they did and the migration is a backfill rather than
// a re-judgement.
func verdictFreshness(verdict map[string]any, reqSHA string,
	testSHAs, unitFingerprints, codeFingerprints map[string]string) Freshness {
	recordedSHA, _ := verdict["requirement_sha"].(string)
	if recordedSHA != reqSHA {
		return Freshness{State: StaleRequirementState}
	}

	tests := recordedMap(verdict, "tests")
	code := recordedMap(verdict, "code")
	units := recordedMap(verdict, "units")

	if len(code) > 0 {
		if !maps.Equal(unitIdentity(code), unitIdentity(codeFingerprints)) {
			return Freshness{State: StaleUnitState, Moved: movedKeys(code, codeFingerprints)}
		}
	}
	if len(units) == 0 {
		if maps.Equal(tests, testSHAs) {
			return Freshness{State: FreshState}
		}
		return Freshness{State: StaleUnitState}
	}
	if !maps.Equal(unitIdentity(units), unitIdentity(unitFingerprints)) {
		return Freshness{State: StaleUnitState, Moved: movedKeys(units, unitFingerprints)}
	}
	if !maps.Equal(tests, testSHAs) {
		return Freshness{State: ShiftedState, Moved: movedKeys(tests, testSHAs)}
	}
	return Freshness{State: FreshState}
}

// AuditFreshnessInput is the whole tree the derivation reads.
type AuditFreshnessInput struct {
	Tree         string
	Requirements []Requirement
	Tags         []Tag
	Enrolled     map[string]bool
	Audits       map[string]Audit
}

// AuditFreshness answers the state of every requirement carrying a verdict.
//
// Derived once and shared by the freshness check, the ledger's proven count and
// the re-seal, so the three can never disagree about which verdicts are
// current.
//
// An unresolvable fingerprint degrades to stale-unit rather than propagating. A
// file the gate cannot read is NOT "unchanged": naming the keys it could not
// resolve sends the verdict for a re-read, which is more checking, never less.
// Refusing here instead would take the LEDGER RENDER down with it -- a report is
// not a gate, and a cited producer that has been deleted must still be
// reportable rather than crashing every consumer of the ledger.
func AuditFreshness(in AuditFreshnessInput) map[string]Freshness {
	reader := newSourceReader(in.Tree)
	index := newScopeIndex()
	byRID := map[string][]Tag{}
	for _, tag := range in.Tags {
		byRID[tag.RID] = append(byRID[tag.RID], tag)
	}

	out := map[string]Freshness{}
	for _, req := range in.Requirements {
		if !in.Enrolled[req.RFC] {
			continue
		}
		audit := in.Audits[req.RFC]
		verdict, held := audit.Verdict(req.RID)
		if !held || len(verdict) == 0 {
			continue
		}
		keys := tagKeys(byRID[req.RID], reader, index)
		codeKeys := verdictCodeKeys(audit, req.RID, verdict)

		units, err := unitSHAs(keys, reader, index, auditWhere(req, "tests"))
		var code map[string]string
		if err == nil {
			code, err = unitSHAs(codeKeys, reader, index, auditWhere(req, "code"))
		}
		if err != nil {
			unresolved := append(append([]string{}, keys...), codeKeys...)
			sort.Strings(unresolved)
			out[req.RID] = Freshness{State: StaleUnitState, Moved: dedupe(unresolved)}
			continue
		}
		out[req.RID] = verdictFreshness(verdict, RequirementSHA(req.Text),
			taggedUnitSHAs(byRID[req.RID], reader, index), units, code)
	}
	return out
}

// auditWhere names the record and the field a refusal came from.
func auditWhere(req Requirement, field string) string {
	var tb textbuf.Buffer
	return tb.Str(auditRel).Byte('/').Str(req.RFC).Str(".json: ").Str(req.RID).
		Byte(' ').Str(field).String()
}

// verdictCodeKeys answers the cited producer keys of one verdict, in the order
// the record wrote them, which is what decides the first refusal.
func verdictCodeKeys(audit Audit, rid string, verdict map[string]any) []string {
	nested, isObject := verdict["code"].(map[string]any)
	if !isObject {
		return nil
	}
	return audit.order.child(rid).child("code").orderOf(nested)
}

// dedupe answers a sorted slice with its repeats removed, which is what a
// Python set union of two key lists answers.
func dedupe(sorted []string) []string {
	out := sorted[:0]
	for i, one := range sorted {
		if i == 0 || sorted[i-1] != one {
			out = append(out, one)
		}
	}
	return out
}
