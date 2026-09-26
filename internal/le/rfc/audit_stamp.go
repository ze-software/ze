// Design: docs/architecture/core-design.md -- the one writer of rfc/audit/
// Overview: rfc.go -- the types, the paths and the closed sets every reader here shares
// Related: reseal.go -- the writer this shares, and the re-stamp this never performs
//
// audit_stamp.go is `./le rfc audit-stamp`: it adds the verdicts an author has
// just judged to rfc/audit/<stem>.json. The author writes the judgement in a
// pending file outside rfc/audit/: the verdict word, the note, the producer keys
// a `code` map cites, and `no_code_path`. This fills the fingerprints the audit
// skill forbids computing by hand -- requirement_sha from the row's text, `tests`
// and `units` from the row's tags, a sha for each cited producer -- and merges
// the stamped verdicts into the audit file.
//
// The pending file lives OUTSIDE rfc/audit/ because an unstamped verdict there
// fails the validating load, and that load feeds `./le rfc check` and the
// derived rfc/enrolled.txt every session's shell hook builds. One half-written
// verdict would stop every session in the checkout.
//
// A requirement that already has a verdict is refused, never re-stamped:
// re-fingerprinting it would declare a judgement fresh that nobody re-made.
// reseal owns the mechanical case and /ze-rfc-audit owns every other one.
package rfc

import (
	"os"
	"time"

	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// keyFrom is the parameter naming the pending verdicts file.
const keyFrom = "from"

// pendingFileKeys are the keys a pending file may carry. The re-stamp notes
// belong to reseal.
var pendingFileKeys = map[string]bool{
	auditFieldRFC: true, auditFieldAudited: true, auditFieldRequirements: true,
}

// pendingVerdicts is one pending file, read: its verdicts in the order the
// author wrote them, and the date the author judged them on.
type pendingVerdicts struct {
	Verdicts map[string]any
	Order    []string
	Audited  string
}

// authoredVerdictKeys are the fields an author writes. Every other verdict
// field is computed by this command or belongs to a re-judgement.
var authoredVerdictKeys = map[string]bool{
	verdictFieldVerdict: true, verdictFieldNote: true, fingerprintCode: true, "no_code_path": true,
}

// AuditStampReport is what the command answers: the stem, and the verdicts it
// stamped in the pending file's order.
type AuditStampReport struct {
	Stem    string   `json:"stem"`
	Stamped []string `json:"stamped"`
}

// Text renders one line per stamped verdict, then the count.
func (r AuditStampReport) Text() string {
	var tb textbuf.Buffer
	for _, rid := range r.Stamped {
		tb.Str("stamped ").Str(rid).Byte('\n')
	}
	return tb.Str("stamped ").Int(int64(len(r.Stamped))).Str(" new verdict(s) into ").Str(auditRel).
		Byte('/').Str(r.Stem).Str(".json. The ledger now needs: ./le rfc index-update\n").String()
}

// auditStamp stamps every verdict of one pending file and merges them into the
// stem's audit file, creating it when absent.
//
// All or nothing: a refusal of any entry names every refused id and writes
// nothing, so a half-merged file never lands and the author fixes the whole
// list in one pass. The write itself is staged and renamed by replaceAudit,
// which validates the merged file before it replaces the old one.
func auditStamp(tree, rfcStem, fromPath string, now time.Time) (AuditStampReport, error) {
	collected, err := Collect(tree)
	if err != nil {
		return AuditStampReport{}, err
	}
	if !collected.Enrolled[rfcStem] {
		var tb textbuf.Buffer
		return AuditStampReport{}, parseErr(tb.Str(pyRepr(rfcStem)).
			Str(" is not an enrolled stem. `./le rfc check` refuses an audit file for a stem ").
			Str("it does not gate, so a verdict for it has nowhere durable to live"))
	}
	pending, err := readPendingVerdicts(tree, rfcStem, fromPath, now)
	if err != nil {
		return AuditStampReport{}, err
	}
	audit, err := loadAudit(tree, rfcStem)
	if err != nil {
		return AuditStampReport{}, err
	}
	if audit.Document == nil {
		audit.Verdicts = map[string]any{}
		audit.Document = map[string]any{auditFieldRFC: rfcStem, auditFieldRequirements: audit.Verdicts}
	}

	rows := map[string]Requirement{}
	for _, req := range collected.Requirements {
		if req.RFC == rfcStem {
			rows[req.RID] = req
		}
	}
	byRID := map[string][]Tag{}
	for _, tag := range collected.Tags {
		byRID[tag.RID] = append(byRID[tag.RID], tag)
	}
	reader := newSourceReader(tree)
	index := newScopeIndex()

	var refused []string
	for _, rid := range pending.Order {
		_, judged := audit.Verdicts[rid]
		req, held := rows[rid]
		why := stampRefusal(rid, pending.Verdicts[rid], judged, held, len(byRID[rid]), rfcStem)
		if why != "" {
			refused = append(refused, why)
			continue
		}
		verdict, _ := pending.Verdicts[rid].(map[string]any)
		if err := stampFingerprints(verdict, req, byRID[rid], reader, index); err != nil {
			refused = append(refused, err.Error())
		}
	}
	if len(refused) > 0 {
		var tb textbuf.Buffer
		tb.Str(relTo(tree, fromPath)).Str(": refused ").Int(int64(len(refused))).
			Str(" verdict(s), nothing written to ").Str(auditRel).Byte('/').Str(rfcStem).Str(".json:")
		for _, why := range refused {
			tb.Str("\n  ").Str(why)
		}
		return AuditStampReport{}, parseErr(&tb)
	}

	for _, rid := range pending.Order {
		audit.Verdicts[rid] = pending.Verdicts[rid]
		audit.Order = append(audit.Order, rid)
	}
	audit.Document[auditFieldAudited] = pending.Audited
	// Made here rather than when the file was found absent, so a refusal
	// leaves no empty directory behind.
	if err := os.MkdirAll(treePath(tree, auditRel), 0o750); err != nil {
		var tb textbuf.Buffer
		return AuditStampReport{}, parseErr(tb.Str(auditRel).Str(": cannot create: ").Err(err))
	}
	if err := replaceAudit(tree, rfcStem, audit); err != nil {
		return AuditStampReport{}, err
	}
	return AuditStampReport{Stem: rfcStem, Stamped: pending.Order}, nil
}

// readPendingVerdicts reads the pending file and answers its verdicts in the
// order the author wrote them.
//
// The file is shaped like an audit file so an author writes one form, and it
// must name the stem the command was given: a file for another RFC merged here
// would judge rows it never read. Its `audited` date, when present, is the day
// the author judged the verdicts and becomes the audit file's date; absent, the
// stamp's own day is.
func readPendingVerdicts(tree, rfcStem, fromPath string, now time.Time) (pendingVerdicts, error) {
	rel := relTo(tree, fromPath)
	var tb textbuf.Buffer
	raw, err := cliio.ReadFile(fromPath)
	if err != nil {
		return pendingVerdicts{}, parseErr(tb.Str(rel).Str(": cannot read: ").Err(err))
	}
	document, order, err := decodeOrdered(raw)
	if err != nil {
		return pendingVerdicts{}, parseErr(tb.Str(rel).Str(": cannot read: ").Err(err))
	}
	data, isObject := document.(map[string]any)
	if !isObject {
		return pendingVerdicts{}, parseErr(tb.Str(rel).Str(": expected a JSON object, got ").
			Str(pyTypeName(document)))
	}
	if err := rejectUnknownKeys(data, pendingFileKeys, rel); err != nil {
		return pendingVerdicts{}, err
	}
	stem, _ := data["rfc"].(string)
	if stem != rfcStem {
		return pendingVerdicts{}, parseErr(tb.Str(rel).Str(": 'rfc' is ").Str(pyRepr(data["rfc"])).
			Str(" but the command names ").Str(pyRepr(rfcStem)).
			Str(". A pending file judges one RFC's rows, and it merges into that RFC's file alone"))
	}
	verdicts, isObject := data["requirements"].(map[string]any)
	if !isObject {
		return pendingVerdicts{}, parseErr(tb.Str(rel).Str(": 'requirements' must be an object"))
	}
	if len(verdicts) == 0 {
		return pendingVerdicts{}, parseErr(tb.Str(rel).Str(": 'requirements' names no verdict. A pending ").
			Str("file that judges nothing cannot be told from one whose verdicts went to the wrong key"))
	}
	audited := now.Format("2006-01-02")
	if data[auditFieldAudited] != nil {
		if audited, err = strField(data, auditFieldAudited, rel, true); err != nil {
			return pendingVerdicts{}, err
		}
	}
	return pendingVerdicts{
		Verdicts: verdicts,
		Order:    order.child("requirements").orderOf(verdicts),
		Audited:  audited,
	}, nil
}

// stampRefusal answers why one pending verdict cannot be stamped, or "" when it
// can.
//
// Each refusal is an entry the gate would read as a claim nobody can check, or
// as a judgement nobody made: a row that already has a verdict, a row that does
// not exist, a field the author does not own, a word outside the closed
// vocabulary, `enforced` over no test, and `not-applicable` over a row a test
// exercises. The remaining schema -- a non-empty note, `no_code_path` only on
// `not-applicable` -- is judged by validateVerdict when the merged file is
// staged, before anything is written.
func stampRefusal(rid string, entry any, judged, held bool, tagCount int, rfcStem string) string {
	var tb textbuf.Buffer
	if judged {
		return tb.Str(rid).Str(" already has a verdict in ").Str(auditRel).Byte('/').Str(rfcStem).
			Str(".json. Re-judging a recorded verdict is /ze-rfc-audit's work, not a stamp").String()
	}
	if !held {
		return tb.Str(rid).Str(" is not a requirement row of rfc/short/").Str(rfcStem).
			Str(".md. A verdict judges a row, so fix the id or add the row first").String()
	}
	verdict, isObject := entry.(map[string]any)
	if !isObject {
		return tb.Str(rid).Str(" must be an object, got ").Str(pyTypeName(entry)).String()
	}
	for _, key := range sortedKeysOf(verdict) {
		if !authoredVerdictKeys[key] {
			return tb.Str(rid).Str(" carries ").Str(pyRepr(key)).Str(", which an author does not ").
				Str("write here: the fingerprints are computed by this command, and the rest belong ").
				Str("to a re-judgement. Write only ").Str(pyRepr(sortedKeysOf(authoredVerdictKeys))).String()
		}
	}
	if code, isMap := verdict[fingerprintCode].(map[string]any); isMap {
		for _, key := range sortedKeysOf(code) {
			if code[key] != "" {
				return tb.Str(rid).Str(" code ").Str(pyRepr(key)).Str(" carries a value. Leave it ").
					Str("empty: a fingerprint is computed, never written by hand").String()
			}
		}
	}
	word, _ := verdict["verdict"].(string)
	if _, known := auditVerdicts[word]; !known {
		return tb.Str(rid).Str(" has verdict ").Str(pyRepr(verdict["verdict"])).
			Str(", which is not one of ").Str(pyRepr(AuditVerdicts())).
			Str(". The vocabulary is closed (ai/skills/ze-rfc-audit.md)").String()
	}
	if word == VerdictEnforced && tagCount == 0 {
		return tb.Str(rid).Str(" is 'enforced' but no `RFC requirement:` tag names it, so there is ").
			Str("no test to fingerprint. Tag the tests first, or record the verdict the evidence ").
			Str("supports").String()
	}
	if word == VerdictNotApplicable && tagCount > 0 {
		return tb.Str(rid).Str(" is 'not-applicable' but ").Int(int64(tagCount)).
			Str(" tag(s) name it. If a test can exercise it, a reachable code path exists, and ").
			Str("a verdict citing no test would read stale over the tests that do").String()
	}
	return ""
}

// stampFingerprints writes the fingerprints of one pending verdict in place.
//
// `not-applicable` cites no test, so it records no `tests` and no `units`,
// which the gate reads as the same state as empty ones. Every other verdict
// records one entry per tag, keyed as the freshness rule keys them, so the
// first `./le rfc check` after the stamp reads it fresh.
func stampFingerprints(verdict map[string]any, req Requirement, tags []Tag,
	reader *sourceReader, index *scopeIndex) error {
	verdict["requirement_sha"] = RequirementSHA(req.Text)
	if verdict["verdict"] != VerdictNotApplicable && len(tags) > 0 {
		units, err := unitSHAs(tagKeys(tags, reader, index), reader, index, auditWhere(req, fingerprintUnits))
		if err != nil {
			return err
		}
		verdict[fingerprintTests] = anyMap(taggedUnitSHAs(tags, reader, index))
		verdict[fingerprintUnits] = anyMap(units)
	}
	code, isObject := verdict[fingerprintCode].(map[string]any)
	if !isObject {
		return nil
	}
	for _, key := range sortedKeysOf(code) {
		sha, err := unitSHAs([]string{key}, reader, index, auditWhere(req, fingerprintCode))
		if err != nil {
			return err
		}
		code[key] = sha[key]
	}
	return nil
}
