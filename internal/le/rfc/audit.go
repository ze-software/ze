// Design: docs/architecture/core-design.md -- the audit record's schema
// Overview: rfc.go -- the types, the paths and the closed sets every reader here shares
//
// audit.go reads rfc/audit/<stem>.json: the per-requirement verdicts the
// ze-rfc-audit skill records, and the fingerprints that say which text each
// verdict judged. freshness.go decides what those fingerprints mean and
// reseal.go rewrites them.
//
// The whole file is a VALIDATING parse. Before the schema existed the body was
// a bare json load returning `requirements`, with no field-presence check, no
// enum check and no type check, and the verdict vocabulary had already drifted
// unnoticed because nothing looked.
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
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// auditRel is where the verdicts live.
const auditRel = "rfc/audit"

// The verdict vocabulary of ai/skills/ze-rfc-audit.md, as a closed enum. A
// fifth word is drift, and drift in this field is a compliance claim nobody can
// read.
//
// `not-applicable` is owner ruling OR-1 (2026-07-29): a requirement with
// genuinely no reachable code path had no legal state, because `enforced` with
// an empty tests map is refused and the code-map remedy is open only to
// `unimplemented`.
const (
	VerdictEnforced      = "enforced"
	VerdictWeak          = "weak"
	VerdictWrong         = "wrong"
	VerdictUnimplemented = "unimplemented"
	VerdictNotApplicable = "not-applicable"
)

// auditVerdicts is the closed vocabulary, each word beside what it says about
// the tests bound to a requirement.
//
// The meaning is stated HERE, beside the word, because a reader who meets
// `weak` on a published page cannot act on the word alone. A second table of
// sentences elsewhere would be a copy of this set with nothing to arbitrate a
// disagreement (ai/rules/principles.md).
var auditVerdicts = map[string]string{
	VerdictEnforced:      "the tests do what the requirement demands",
	VerdictWeak:          "the tests pass over code that does not enforce the requirement",
	VerdictWrong:         "the tests assert something other than what the requirement demands",
	VerdictUnimplemented: "no code path enforces the requirement",
	VerdictNotApplicable: "the requirement has no reachable code path in Ze",
}

// AuditVerdicts answers the closed vocabulary, sorted. The fixture pins it
// by VALUE rather than by count, which is what kills a one-word mutation in a
// set no output ever prints.
func AuditVerdicts() []string { return sortedKeysOf(auditVerdicts) }

// AuditVerdictMeaning answers what one verdict word says, and false for a word
// outside the vocabulary.
//
// A caller that publishes a verdict owes the reader the sentence: the word on
// its own carries no meaning to anyone who has not read the audit skill.
func AuditVerdictMeaning(verdict string) (string, bool) {
	meaning, held := auditVerdicts[verdict]
	return meaning, held
}

var auditFileKeys = map[string]bool{
	"rfc": true, "audited": true, "requirements": true,
	"reaudit_note": true, "reaudit_history": true,
}

// The three verdict fields whose keys become filesystem reads: the tests that
// prove the requirement, the units they judged, and the production code.
const (
	fingerprintTests = "tests"
	fingerprintUnits = "units"
	fingerprintCode  = "code"
)

var verdictKeys = map[string]bool{
	"verdict": true, "note": true, "requirement_sha": true,
	fingerprintTests: true, fingerprintUnits: true, fingerprintCode: true,
	"upgrade_reason": true, "no_code_path": true,
}

// fingerprintMaps are the three fields whose keys become filesystem reads.
var fingerprintMaps = [...]string{fingerprintTests, fingerprintUnits, fingerprintCode}

// A fingerprint key names a SYMBOL, never a location: `<repo-relative
// path>::<FuncName>` for a Go function, or `<repo-relative path>` alone when
// the whole file is the unit. A stored line is kept current by no generator, so
// it rots at the next edit above it; a symbol is recovered by NAME at read
// time, so a line shift is invisible and a rename is a real change to what was
// judged.
//
// A second and later tag inside ONE unit take a `#2`, `#3` ordinal. That is
// what keeps the identity a multiset: without it two tags in one function mint
// one key, deleting one of them leaves the map identical, and the verdict reads
// FRESH over a test that is gone. `#1` is not a legal spelling.
//
// The trailing `\n?` is Python's `$`, which matches before ONE final newline
// where Go's matches only at the end of the text. Reproduced rather than
// tightened: this reader answers what the script answers, and the retired form
// below is where the difference is visible -- its `\d+` cannot swallow the
// newline the way the path's character class can.
var (
	fingerprintKeyRE = regexp.MustCompile(
		`^(?P<file>[^:#\x00]+)(?:::(?P<symbol>[A-Za-z_][A-Za-z0-9_]*))?(?:#(?:[2-9]|[1-9][0-9]+))?\n?$`)
	retiredKeyRE = regexp.MustCompile(`^(?P<file>[^:\x00]+):(?P<line>\d+)\n?$`)
)

// shaRE matches the whole string, so a 17-character `<16 hex>\n` is refused by
// the very check meant to catch an invalid value.
var shaRE = regexp.MustCompile(`^[0-9a-f]{16}$`)

// fingerprintKey splits `path::Symbol`, `path::Symbol#2` or a bare `path`.
//
// It answers (path, symbol) with an empty symbol for a file-scoped key. The
// ordinal is validated and then dropped: it distinguishes two tags inside ONE
// unit, and both resolve to that same unit. Path safety is judged FIRST, over
// whichever shape matched, so a key naming `/etc/passwd` is refused for the
// reason that matters even when its other half is also malformed.
func fingerprintKey(key, where string) (string, string, error) {
	var tb textbuf.Buffer
	match := fingerprintKeyRE.FindStringSubmatch(key)
	var retired []string
	if match == nil {
		retired = retiredKeyRE.FindStringSubmatch(key)
	}
	matched := match
	if matched == nil {
		matched = retired
	}
	if len(matched) > 1 {
		if !insideRepo(matched[1]) {
			return "", "", parseErr(tb.Str(where).Str(": fingerprint key ").Str(pyRepr(key)).
				Str(" names a path outside the repository. A verdict is authored input, ").
				Str("not a trusted path source"))
		}
	}
	if retired != nil {
		return "", "", parseErr(tb.Str(where).Str(": fingerprint key ").Str(pyRepr(key)).
			Str(" is the retired '<path>:<line>' form. A key names the SYMBOL it ").
			Str("fingerprints, not where that symbol sat: write '").Str(retired[1]).
			Str("::<FuncName>', or '").Str(retired[1]).
			Str("' alone when the whole file is the unit. A stored line is current ").
			Str("only until the next edit above it"))
	}
	if match == nil {
		return "", "", parseErr(tb.Str(where).Str(": fingerprint key ").Str(pyRepr(key)).
			Str(" is not '<repo-relative-path>::<FuncName>' or a bare ").
			Str("'<repo-relative-path>', either of which may carry a '#2' or higher ").
			Str("ordinal when one unit holds more than one tag"))
	}
	return match[1], match[2], nil
}

// slashSegments reports whether any slash-separated segment of a path is `..`,
// which is the traversal a key must never carry.
func slashSegments(rel string) bool {
	return slices.Contains(strings.Split(rel, "/"), "..")
}

// insideRepo reports whether a path from AUTHORED input stays in the checkout.
//
// Three shapes leave it: an absolute path, a home-relative path, and any `..`
// segment. Stated positively, so a caller reads "inside" rather than inverting
// three negatives, and declared once because every authored path that reaches a
// tree read or a tree write owes the same answer -- a fingerprint key in a
// record, and the file path in a gomu report, which becomes an overlay key and,
// on the interop carrier, an os.WriteFile target (ai/rules/principles.md).
func insideRepo(rel string) bool {
	if strings.HasPrefix(rel, "/") {
		return false
	}
	if strings.HasPrefix(rel, "~") {
		return false
	}
	return !slashSegments(rel)
}

// keyFile answers the file half of a fingerprint key, for a caller that wants
// no validation.
//
// Every shape is read here, symbol and ordinal alike, and an unparseable key
// comes back whole rather than refusing: the two callers are comparing and
// reporting, not opening.
func keyFile(key string) string {
	head, _, _ := strings.Cut(key, "::")
	head, _, _ = strings.Cut(head, "#")
	return head
}

// validateSHA checks one recorded fingerprint for SHAPE, and not merely for
// non-emptiness.
//
// Length alone is not the check: a value truncated and then padded to 16
// characters has the right length and is still not a fingerprint, so the
// charset is validated too.
func validateSHA(value any, where string) error {
	text, isText := value.(string)
	if isText && shaRE.MatchString(text) {
		return nil
	}
	var got textbuf.Buffer
	if isText {
		got.Str("a ").Int(int64(utf8.RuneCountInString(text))).Str("-character string")
	} else {
		got.Str("a ").Str(pyTypeName(value))
	}
	var tb textbuf.Buffer
	return parseErr(tb.Str(where).Str(": ").Str(pyRepr(value)).
		Str(" is not a fingerprint. Expected exactly ").Int(shaHexLen).
		Str(" lowercase hex characters, as produced by requirement_sha()/test_sha(); got ").
		Str(got.String()).
		Str(". Replace the recorded value with the computed one -- a hand-edited ").
		Str("fingerprint is never valid, and `./le rfc reseal` cannot repair it ").
		Str("because it loads through this same check"))
}

// validateSHAMap checks one fingerprint map. Absent reads as empty; a wrong
// TYPE never does.
//
// The zero-value trap this closes: a `tests` field holding a string, a list or
// a map of maps used to flow straight into an equality comparison, where any of
// them compares unequal to the computed shas and reports as STALE -- a real
// defect wearing the costume of a routine re-read.
//
// The keys are read in DOCUMENT order, which is what decides the first refusal
// when more than one is malformed. A JSON object cannot carry a non-string key,
// so the module's non-string-key refusal is unreachable from either half.
func validateSHAMap(verdict map[string]any, order *keyOrder, key, where string) error {
	value := verdict[key]
	if value == nil {
		return nil
	}
	var tb textbuf.Buffer
	nested, isObject := value.(map[string]any)
	if !isObject {
		return parseErr(tb.Str(where).Str(": ").Str(pyRepr(key)).
			Str(" must be an object, got ").Str(pyTypeName(value)))
	}
	var place textbuf.Buffer
	inner := place.Str(where).Str(": ").Str(key).String()
	for _, name := range order.child(key).orderOf(nested) {
		if _, _, err := fingerprintKey(name, inner); err != nil {
			return err
		}
		var at textbuf.Buffer
		if err := validateSHA(nested[name], at.Str(inner).Byte('[').Str(pyRepr(name)).Byte(']').String()); err != nil {
			return err
		}
	}
	return nil
}

// validateVerdict checks one verdict's STRUCTURE.
//
// Structure only: the cross-referential rules (does the rid exist, do the tags
// cover both polarities, does the annotation agree) need the requirements and
// are reported as violations instead. A refusal here aborts the whole run,
// while a violation lets the other 900 be seen.
func validateVerdict(rid string, verdict any, order *keyOrder, where string) error {
	var tb textbuf.Buffer
	var place textbuf.Buffer
	at := place.Str(where).Str(": ").Str(rid).String()

	data, isObject := verdict.(map[string]any)
	if !isObject {
		return parseErr(tb.Str(where).Str(": ").Str(rid).Str(" must be an object, got ").
			Str(pyTypeName(verdict)))
	}
	if err := rejectUnknownKeys(data, verdictKeys, at); err != nil {
		return err
	}
	value, _ := data["verdict"].(string)
	if _, known := auditVerdicts[value]; !known {
		return parseErr(tb.Str(at).Str(" has verdict ").Str(pyRepr(data["verdict"])).
			Str(", which is not one of ").Str(pyRepr(AuditVerdicts())).
			Str(". The vocabulary is closed (ai/skills/ze-rfc-audit.md): a fifth word is ").
			Str("drift, and drift in this field is a compliance claim nobody can read"))
	}
	if _, err := strField(data, "note", at, true); err != nil {
		return err
	}
	// strField runs first, so a wrong TYPE keeps its own message; the shape
	// check then reads the RAW value, NOT strField's return, because that
	// return is stripped while the record keeps what was written and the
	// freshness rule compares THAT. Checking the stripped form passed a
	// `<16 hex>\n` value and then let it read STALE_REQUIREMENT forever.
	if _, err := strField(data, "requirement_sha", at, true); err != nil {
		return err
	}
	var sha textbuf.Buffer
	if err := validateSHA(data["requirement_sha"], sha.Str(at).Str(": 'requirement_sha'").String()); err != nil {
		return err
	}
	for _, name := range fingerprintMaps {
		if err := validateSHAMap(data, order, name, at); err != nil {
			return err
		}
	}
	if data["upgrade_reason"] != nil {
		if _, err := strField(data, "upgrade_reason", at, false); err != nil {
			return err
		}
	}
	// `no_code_path` means exactly one thing, so it may only appear where it
	// means it. A field that sits unread on the other four verdicts is a field
	// an author can believe they filled in.
	if _, held := data["no_code_path"]; held {
		if value != VerdictNotApplicable {
			return parseErr(tb.Str(at).Str(" carries 'no_code_path' with verdict ").
				Str(pyRepr(data["verdict"])).
				Str(". That field states why no reachable code path exists and is only ").
				Str("meaningful on ").Str(pyRepr(VerdictNotApplicable)))
		}
		// required=false on purpose: absent-or-empty stays the REPORTED
		// violation that names what to write, and only a wrong TYPE refuses.
		if _, err := strField(data, "no_code_path", at, false); err != nil {
			return err
		}
	}
	return nil
}

// Audit is one rfc/audit/<stem>.json, loaded and validated.
//
// Document holds the whole decoded file so the writer can rewrite it without a
// second parse, and Verdicts is the `requirements` object with its authored key
// order preserved, which is what decides which malformed verdict is named first.
type Audit struct {
	Document map[string]any
	Verdicts map[string]any
	Order    []string
	order    *keyOrder
}

// Verdict answers one requirement's recorded verdict, and whether it has one.
func (a Audit) Verdict(rid string) (map[string]any, bool) {
	found, isObject := a.Verdicts[rid].(map[string]any)
	return found, isObject
}

// VerdictRecord is one recorded judgement, typed.
//
// Verdict answers the untyped document, because the writer rewrites it and the
// freshness rule compares the raw fields. A READER outside this package wants
// the fields instead, and reaching into `map[string]any` for them would spell
// the six keys a second time -- which is where the vocabulary drifted before
// the schema existed (ai/rules/principles.md).
type VerdictRecord struct {
	Verdict string `json:"verdict"`
	Note    string `json:"note"`
	// RequirementSHA fingerprints the obligation's own text as it read when
	// the verdict was recorded.
	RequirementSHA string `json:"requirement-sha"`
	// Tests, Units and Code are the three fingerprint maps, keyed by the
	// symbol each one names.
	Tests map[string]string `json:"tests,omitempty"`
	Units map[string]string `json:"units,omitempty"`
	Code  map[string]string `json:"code,omitempty"`
	// UpgradeReason is why a previous verdict was raised, and NoCodePath is
	// why no reachable code path exists. Each is empty where the record wrote
	// none, and NoCodePath is legal only on a not-applicable verdict.
	UpgradeReason string `json:"upgrade-reason,omitempty"`
	NoCodePath    string `json:"no-code-path,omitempty"`
}

// Record answers one requirement's recorded verdict as fields, and whether it
// has one.
//
// A verdict that reaches here has passed the validating load, so every field is
// the type this reads. A document that reached it without one answers the zero
// value for that field alone, which is the state the record is in.
func (a Audit) Record(rid string) (VerdictRecord, bool) {
	verdict, held := a.Verdict(rid)
	if !held {
		return VerdictRecord{}, false
	}
	record := VerdictRecord{
		Verdict: verdictValue(verdict),
		Tests:   recordedMap(verdict, fingerprintTests),
		Units:   recordedMap(verdict, fingerprintUnits),
		Code:    recordedMap(verdict, fingerprintCode),
	}
	record.Note, _ = verdict["note"].(string)
	record.RequirementSHA, _ = verdict["requirement_sha"].(string)
	record.UpgradeReason, _ = verdict["upgrade_reason"].(string)
	record.NoCodePath, _ = verdict["no_code_path"].(string)
	return record, true
}

// auditStems answers every stem with an rfc/audit/<stem>.json.
//
// The direction the freshness check never walked: it iterates REQUIREMENTS and
// asks each for its verdict, so an audit file for a stem that is not enrolled,
// or has no summary at all, was read by nothing and reported by nothing.
func auditStems(tree string) (map[string]bool, error) {
	entries, err := os.ReadDir(treePath(tree, auditRel))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		var tb textbuf.Buffer
		return nil, parseErr(tb.Str(auditRel).Str(": cannot read: ").Err(err))
	}
	out := map[string]bool{}
	for _, entry := range entries {
		if stem, isJSON := strings.CutSuffix(entry.Name(), ".json"); isJSON {
			out[stem] = true
		}
	}
	return out, nil
}

// loadAudit reads and VALIDATES one rfc/audit/<stem>.json.
//
// An absent file is an empty answer rather than a refusal: most enrolled RFCs
// have never been audited, and that is a state the ledger reports rather than a
// tree the gate cannot read.
func loadAudit(tree, rfcStem string) (Audit, error) {
	var name textbuf.Buffer
	path := treePath(tree, auditRel, name.Str(rfcStem).Str(".json").String())
	var relBuf textbuf.Buffer
	rel := relBuf.Str(auditRel).Byte('/').Str(rfcStem).Str(".json").String()
	var tb textbuf.Buffer

	raw, err := os.ReadFile(path) // #nosec G304 -- a path under the checkout this gate judges
	if err != nil {
		if os.IsNotExist(err) {
			return Audit{}, nil
		}
		return Audit{}, parseErr(tb.Str(rel).Str(": cannot read: ").Err(err))
	}
	document, order, err := decodeOrdered(raw)
	if err != nil {
		return Audit{}, parseErr(tb.Str(rel).Str(": cannot read: ").Err(err))
	}
	data, isObject := document.(map[string]any)
	if !isObject {
		return Audit{}, parseErr(tb.Str(rel).Str(": expected a JSON object, got ").
			Str(pyTypeName(document)))
	}
	if err := rejectUnknownKeys(data, auditFileKeys, rel); err != nil {
		return Audit{}, err
	}
	stem, err := strField(data, "rfc", rel, true)
	if err != nil {
		return Audit{}, err
	}
	if stem != rfcStem {
		return Audit{}, parseErr(tb.Str(rel).Str(": 'rfc' is ").Str(pyRepr(stem)).
			Str(" but the filename says ").Str(pyRepr(rfcStem)).
			Str(". The record names the RFC it judges; the two can never drift apart"))
	}
	if _, err := strField(data, "audited", rel, true); err != nil {
		return Audit{}, err
	}
	if history, held := data["reaudit_history"]; held && history != nil {
		if !isStringList(history) {
			return Audit{}, parseErr(tb.Str(rel).Str(": 'reaudit_history' must be a list of strings"))
		}
	}
	verdicts, isObject := data["requirements"].(map[string]any)
	if !isObject {
		return Audit{}, parseErr(tb.Str(rel).Str(": 'requirements' must be an object"))
	}
	verdictOrder := order.child("requirements")
	for _, rid := range verdictOrder.orderOf(verdicts) {
		if err := validateVerdict(rid, verdicts[rid], verdictOrder.child(rid), rel); err != nil {
			return Audit{}, err
		}
	}
	return Audit{
		Document: data,
		Verdicts: verdicts,
		Order:    verdictOrder.orderOf(verdicts),
		order:    verdictOrder,
	}, nil
}

// isStringList reports whether a decoded value is a JSON array of strings.
func isStringList(value any) bool {
	items, isList := value.([]any)
	if !isList {
		return false
	}
	for _, one := range items {
		if _, isText := one.(string); !isText {
			return false
		}
	}
	return true
}

// loadAudits reads every enrolled RFC's verdicts.
//
// ONE load point, so the validating parse cannot be reached by one consumer and
// bypassed by another, and so a 166-RFC run pays for each file once instead of
// once per check.
func loadAudits(tree string, enrolled map[string]bool) (map[string]Audit, error) {
	out := make(map[string]Audit, len(enrolled))
	for _, stem := range sortedSet(enrolled) {
		found, err := loadAudit(tree, stem)
		if err != nil {
			return nil, err
		}
		out[stem] = found
	}
	return out, nil
}

// ─── The authored key order ─────────────────────────────────────────────────

// keyOrder mirrors a decoded JSON document, holding the order every object's
// keys were written in.
//
// It exists because the module reports the FIRST defect it meets while walking
// a dict, and a Python dict walks in insertion order while a Go map walks in no
// order at all. Two halves that refuse the same tree for a different reason
// would still both refuse, so this is a message fidelity rather than a verdict:
// it is carried because a parity test compares the pages, and a page that named
// a different one of two malformed verdicts run to run would be untestable.
type keyOrder struct {
	keys     []string
	children map[string]*keyOrder
}

// noOrder is the answer for a value that is not an object: an array, a string,
// a number, a boolean or a null. Every accessor below is nil-safe and
// empty-safe, so one shared node serves them all and no reader has to tell an
// absent order from an order over nothing.
var noOrder = &keyOrder{}

// child answers the order recorded for one nested object, or nil.
func (k *keyOrder) child(key string) *keyOrder {
	if k == nil {
		return nil
	}
	return k.children[key]
}

// orderOf answers the keys of a decoded object in the order they were written,
// falling back to sorted order for a node this walk never saw.
func (k *keyOrder) orderOf(object map[string]any) []string {
	out := make([]string, 0, len(object))
	if k != nil {
		for _, key := range k.keys {
			if _, held := object[key]; held {
				out = append(out, key)
			}
		}
		if len(out) == len(object) {
			return out
		}
		out = out[:0]
	}
	for key := range object {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

// decodeOrdered decodes a JSON document and, in a second pass over the same
// bytes, records the key order of every object in it.
//
// UseNumber keeps the literal an author wrote, which is what tells an int from
// a float: a decoded float64 has already lost the difference.
func decodeOrdered(raw []byte) (any, *keyOrder, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var document any
	if err := decoder.Decode(&document); err != nil {
		return nil, nil, err
	}
	order, err := scanKeyOrder(json.NewDecoder(bytes.NewReader(raw)))
	if err != nil {
		return nil, nil, err
	}
	return document, order, nil
}

// scanKeyOrder walks the token stream and answers the key order tree.
//
// A duplicate key keeps its FIRST position, which is where Python's dict keeps
// it while taking the LAST value.
func scanKeyOrder(decoder *json.Decoder) (*keyOrder, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delim, isDelim := token.(json.Delim)
	if !isDelim {
		return noOrder, nil
	}
	switch delim {
	case '{':
		node := &keyOrder{children: map[string]*keyOrder{}}
		seen := map[string]bool{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			key, _ := keyToken.(string)
			if !seen[key] {
				seen[key] = true
				node.keys = append(node.keys, key)
			}
			child, err := scanKeyOrder(decoder)
			if err != nil {
				return nil, err
			}
			node.children[key] = child
		}
		if _, err := decoder.Token(); err != nil {
			return nil, err
		}
		return node, nil
	case '[':
		// An array's own order is its indexes, and no object this schema
		// carries sits inside one. The elements are still walked, because the
		// token stream has to be consumed to reach what follows.
		for decoder.More() {
			if _, err := scanKeyOrder(decoder); err != nil {
				return nil, err
			}
		}
		if _, err := decoder.Token(); err != nil {
			return nil, err
		}
		return noOrder, nil
	}
	return noOrder, nil
}

// auditPath answers where one RFC's verdicts live.
func auditPath(tree, rfcStem string) string {
	var name textbuf.Buffer
	return filepath.Join(treePath(tree, auditRel), name.Str(rfcStem).Str(".json").String())
}
