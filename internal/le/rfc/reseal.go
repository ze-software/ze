// Design: docs/architecture/core-design.md -- the one writer of rfc/audit/
// Overview: rfc.go -- the types, the paths and the closed sets every reader here shares
//
// reseal.go is `./le rfc reseal`: the ONLY thing that re-stamps a verdict
// already in rfc/audit/ without a human editing it. freshness.go says which
// verdicts merely shifted, and audit.go holds the schema a re-stamp must still
// satisfy. audit_stamp.go adds the verdicts an author has just judged, and
// refuses a requirement that already has one.
//
// Deliberately not folded into the coverage gate (a check that writes cannot be
// trusted to report) nor into the ledger generator (which runs routinely, for
// reasons that have nothing to do with an audit, so re-sealing there would
// automate the blind re-stamp reflex this exists to remove). Owner ruling
// 2026-07-29.
//
// It re-stamps the verdicts whose tagged UNITS did not change, and only those.
// A verdict whose unit, cited producer code, or requirement text moved is
// REFUSED and left stale for a human.
package rfc

import (
	"bytes"
	"errors"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ResealReport is what the command answers: the verdicts it re-stamped and the
// ones it refused, each named with the state that refused it.
type ResealReport struct {
	Resealed []string `json:"resealed"`
	Refused  []string `json:"refused"`
}

// Text renders the page the script printed, without its color.
func (r ResealReport) Text() string {
	var tb textbuf.Buffer
	for _, line := range r.Refused {
		tb.Str("refused ").Str(line).Byte('\n')
	}
	if len(r.Resealed) == 0 {
		return tb.Str("nothing to re-seal: no verdict is in the 'shifted' state (").
			Int(int64(len(r.Refused))).Str(" refused)\n").String()
	}
	for _, line := range r.Resealed {
		tb.Str("re-stamped ").Str(line).Byte('\n')
	}
	return tb.Str("re-sealed ").Int(int64(len(r.Resealed))).Str(" shifted verdict(s); ").
		Int(int64(len(r.Refused))).
		Str(" refused. The ledger now needs: ./le rfc update\n").String()
}

// resealScope names the stems one reseal may write.
//
// A shifted verdict is any verdict whose tagged file moved around an unchanged
// unit, and in a checkout several sessions edit, files move under every RFC all
// day. A reseal scoped to the whole corpus therefore re-stamps verdicts nobody
// asked about, and leaves their files modified in a tree where nobody owns them.
// The scope is the caller's list, so a reseal writes only the stems named.
type resealScope struct {
	// every admits each enrolled stem. Only ResealWithProof sets it, because its
	// per-file proof callback is what bounds what it may write.
	every bool
	// stems are the stems the caller named, each of which must be enrolled.
	stems map[string]bool
}

// admits reports whether the scope lets the reseal write stem.
func (s resealScope) admits(stem string) bool {
	if s.every {
		return true
	}
	return s.stems[stem]
}

// resealStems re-stamps the SHIFTED verdicts of the named stems, and only
// those, and answers what it did. A verdict of any other stem is neither read
// for refusal nor written, whatever its state.
func resealStems(tree string, stems []string) (ResealReport, error) {
	if len(stems) == 0 {
		return ResealReport{}, errors.New("rfc reseal requires stem <stem>: name each RFC " +
			"whose verdicts your own edit shifted, for example `./le rfc reseal stem rfc4271`, " +
			"so verdicts of RFCs you did not touch stay as they are")
	}
	named := make(map[string]bool, len(stems))
	for _, stem := range stems {
		named[stem] = true
	}
	return reseal(tree, resealScope{stems: named}, nil)
}

// ResealWithProof re-stamps mechanically shifted verdicts after the caller
// independently proves every named file changed only in its allowed way.
//
// The predicate can only make resealing stricter. It also admits the
// transitional stale-unit verdict with no recorded units, because that legacy
// record has no unit fingerprint to compare and therefore needs the caller's
// independent per-file proof. A false proof refuses only the verdict that names
// the file. It walks every enrolled stem: a module rename moves files under
// every RFC at once, and the proof, not a stem list, bounds what it writes.
func ResealWithProof(tree string, prove func(string) bool) (ResealReport, error) {
	return reseal(tree, resealScope{every: true}, prove)
}

func reseal(tree string, scope resealScope, prove func(string) bool) (ResealReport, error) {
	collected, err := Collect(tree)
	if err != nil {
		return ResealReport{}, err
	}
	for _, stem := range sortedSet(scope.stems) {
		if collected.Enrolled[stem] {
			continue
		}
		var message textbuf.Buffer
		return ResealReport{}, errors.New(message.Str("rfc reseal: ").Str(stem).
			Str(" is not an enrolled RFC, so it holds no verdict to re-seal: name the stem ").
			Str("of a summary under rfc/short/ whose Enrolment is enrolled").String())
	}
	audits, err := loadAudits(tree, collected.Enrolled)
	if err != nil {
		return ResealReport{}, err
	}
	states := auditFreshness(auditFreshnessInput{
		Tree: tree, Requirements: collected.Requirements, Tags: collected.Tags,
		Enrolled: collected.Enrolled, Audits: audits,
	})
	byRID := map[string][]Tag{}
	for _, tag := range collected.Tags {
		byRID[tag.RID] = append(byRID[tag.RID], tag)
	}
	reader := newSourceReader(tree)
	index := newScopeIndex()
	proven := map[string]bool{}

	report := ResealReport{Resealed: []string{}, Refused: []string{}}
	for _, rfcStem := range sortedSet(collected.Enrolled) {
		if !scope.admits(rfcStem) {
			continue
		}
		audit := audits[rfcStem]
		if len(audit.Verdicts) == 0 {
			continue
		}
		touched := false
		for _, req := range collected.Requirements {
			verdict, held := audit.Verdict(req.RID)
			if req.RFC != rfcStem || !held {
				continue
			}
			state, known := states[req.RID]
			if !known {
				state = Freshness{State: FreshState}
			}
			if state.State == FreshState {
				continue
			}
			transitional := state.State == StaleUnitState && len(verdictFingerprintKeys(verdict[fingerprintUnits])) == 0
			// A shifted unit re-seals from its own text. A stale-unit verdict
			// that fingerprinted no unit is transitional, and it re-seals only
			// when the caller supplies the proof callback.
			resealable := state.State == ShiftedState || (transitional && prove != nil)
			if !resealable {
				var message textbuf.Buffer
				report.Refused = append(report.Refused, message.Str(rfcStem).Byte(' ').
					Str(req.RID).Str(": ").Str(state.State).
					Str(", a human must re-read it").String())
				continue
			}
			fresh := taggedUnitSHAs(byRID[req.RID], reader, index)
			if prove != nil {
				files := map[string]bool{}
				for _, key := range verdictFingerprintKeys(verdict[fingerprintTests]) {
					files[keyFile(key)] = true
				}
				for key := range fresh {
					files[keyFile(key)] = true
				}
				var unproven []string
				for _, rel := range sortedSet(files) {
					held, checked := proven[rel]
					if !checked {
						held = prove(rel)
						proven[rel] = held
					}
					if !held {
						unproven = append(unproven, rel)
					}
				}
				if len(unproven) > 0 {
					var message textbuf.Buffer
					report.Refused = append(report.Refused, message.Str(rfcStem).Byte(' ').
						Str(req.RID).Str(": more than the caller's proof changed in ").
						Str(strings.Join(unproven, ", ")).String())
					continue
				}
			}
			verdict[fingerprintTests] = anyMap(fresh)
			var message textbuf.Buffer
			report.Resealed = append(report.Resealed,
				message.Str(rfcStem).Byte(' ').Str(req.RID).String())
			touched = true
		}
		if touched {
			if err := replaceAudit(tree, rfcStem, audit); err != nil {
				return ResealReport{}, err
			}
		}
	}
	return report, nil
}

func verdictFingerprintKeys(value any) []string {
	fingerprints, isMap := value.(map[string]any)
	if !isMap {
		return nil
	}
	return sortedKeysOf(fingerprints)
}

// anyMap widens a fingerprint map so it can sit back in the decoded document
// the writer renders.
func anyMap(in map[string]string) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

// replaceAudit validates every verdict of one RFC, then writes each verdict
// whose rendered bytes differ from its file.
//
// One file per requirement, so a re-stamp or a stamp touches only the
// requirements it changed, and a session judging another requirement of the
// same RFC never finds this one's verdict in a file it must commit. Every
// verdict is validated before any is written, so a refusal writes nothing.
func replaceAudit(tree, rfcStem string, audit Audit) error {
	for _, rid := range audit.Order {
		if err := validateVerdict(rid, audit.Verdicts[rid], audit.order.child(rid), auditRecordRel(rfcStem, rid)); err != nil {
			return err
		}
	}
	for _, rid := range audit.Order {
		if err := writeAuditVerdict(tree, rfcStem, rid, audit.Verdicts[rid]); err != nil {
			return err
		}
	}
	return nil
}

// writeAuditVerdict writes one verdict into its own file, unless the file
// already holds those bytes.
//
// The RENDERED BYTES are re-read through the validating parser before they
// land, so a defect in this writer can never commit a record that later makes
// every check say "cannot run". The write itself is atomic (writeArtifact).
func writeAuditVerdict(tree, rfcStem, rid string, verdict any) error {
	rel := auditRecordRel(rfcStem, rid)
	body := renderAuditVerdict(verdict)
	var tb textbuf.Buffer
	reread, order, err := decodeOrdered(body)
	if err != nil {
		return parseErr(tb.Str(rel).Str(" (rendered): cannot read: ").Err(err))
	}
	if err := validateVerdict(rid, reread, order, rel+" (rendered)"); err != nil {
		return err
	}
	current, err := os.ReadFile(auditRecordPath(tree, rfcStem, rid)) // #nosec G304 -- the file this function replaces
	if err == nil && bytes.Equal(current, body) {
		return nil
	}
	return writeArtifact(tree, rel, body)
}

// renderAuditVerdict is one verdict as its file holds it. The writer and the
// conversion from the retired layout share it, so both produce the same bytes.
func renderAuditVerdict(verdict any) []byte {
	var body textbuf.Buffer
	return []byte(body.Str(pyDump(verdict)).Byte('\n').String())
}
