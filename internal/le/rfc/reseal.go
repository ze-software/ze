// Design: docs/architecture/core-design.md -- the one writer of rfc/audit/
// Overview: rfc.go -- the types, the paths and the closed sets every reader here shares
//
// reseal.go is `./le rfc reseal`: the ONLY thing that writes rfc/audit/
// without a human editing it. freshness.go says which verdicts merely
// shifted, and audit.go holds the schema a re-stamp must still satisfy.
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
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// resealNote is what the re-stamp records about itself, on every file it
// rewrites.
const resealNote = "Mechanical re-stamp by `./le rfc reseal`. Every verdict re-stamped below was in the " +
	"SHIFTED state: its recorded unit fingerprints -- the enclosing top-level function of each " +
	"tagged test -- were byte-identical to the tree, and only the file around them had moved (a " +
	"line shift, a sibling test, an import rewrite). Nothing about what any of these tests " +
	"asserts changed, so no verdict was re-judged and only the `tests` map was rewritten. A " +
	"verdict whose unit, cited producer code, or requirement text moved was REFUSED and left " +
	"stale for a human. The proof is the code in verdict_freshness(), not this note."

// stagingPrefix names the directory a rewrite is staged in.
const stagingPrefix = ".staging-"

// stagingStaleSeconds is how old an abandoned staging directory must be before
// the sweep removes it.
//
// AGE-GATED rather than unconditional. An unconditional sweep would delete a
// CONCURRENT run's in-flight staging directory, and that run would then fail to
// read its own file -- a crash where the shipped code exits cleanly. A live
// staging directory exists for milliseconds; an abandoned one is minutes old
// before anyone runs the writer again.
const stagingStaleSeconds = 3600

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
		Str(" refused. The ledger now needs: ./le rfc index-update\n").String()
}

// resealTree re-stamps every SHIFTED verdict in the checkout and answers what it
// did.
func resealTree(tree string) (ResealReport, error) {
	return reseal(tree, nil, resealNote)
}

// ResealWithProof re-stamps mechanically shifted verdicts after the caller
// independently proves every named file changed only in its allowed way.
//
// The predicate can only make resealing stricter. It also admits the
// transitional stale-unit verdict with no recorded units, because that legacy
// record has no unit fingerprint to compare and therefore needs the caller's
// independent per-file proof. A false proof refuses only the verdict that names
// the file.
func ResealWithProof(tree string, prove func(string) bool, note string) (ResealReport, error) {
	if note == "" {
		note = resealNote
	}
	return reseal(tree, prove, note)
}

func reseal(tree string, prove func(string) bool, note string) (ResealReport, error) {
	collected, err := Collect(tree)
	if err != nil {
		return ResealReport{}, err
	}
	audits, err := loadAudits(tree, collected.Enrolled)
	if err != nil {
		return ResealReport{}, err
	}
	states := AuditFreshness(AuditFreshnessInput{
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
			transitional := state.State == StaleUnitState && len(verdictFingerprintKeys(verdict["units"])) == 0
			if state.State != ShiftedState && !(transitional && prove != nil) {
				var message textbuf.Buffer
				report.Refused = append(report.Refused, message.Str(rfcStem).Byte(' ').
					Str(req.RID).Str(": ").Str(state.State).
					Str(", a human must re-read it").String())
				continue
			}
			fresh := taggedUnitSHAs(byRID[req.RID], reader, index)
			if prove != nil {
				files := map[string]bool{}
				for _, key := range verdictFingerprintKeys(verdict["tests"]) {
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
			verdict["tests"] = anyMap(fresh)
			var message textbuf.Buffer
			report.Resealed = append(report.Resealed,
				message.Str(rfcStem).Byte(' ').Str(req.RID).String())
			touched = true
		}
		if touched {
			if err := writeAudit(tree, rfcStem, audit, note); err != nil {
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

// writeAudit rewrites one audit file, preserving the previous re-stamp note
// into history.
//
// Staged beside the target then renamed: a refusal or a kill must leave the
// reviewer's existing evidence file untouched. The STAGED BYTES are re-read
// through the validating parser before they land, so a defect in this writer
// can never commit a record that later makes every check say "cannot run". Both
// are checked -- the in-memory document first, since it still refuses things a
// JSON round trip would launder.
func writeAudit(tree, rfcStem string, audit Audit, note string) error {
	data := audit.Document
	// An earlier re-stamp's reasoning is evidence about the same verdicts.
	// Overwriting it would delete the record of why they were trusted then.
	if previous, isText := data["reaudit_note"].(string); isText && previous != "" {
		history, _ := data["reaudit_history"].([]any)
		data["reaudit_history"] = append(history, previous)
	}
	data["reaudit_note"] = note

	var staged textbuf.Buffer
	rel := staged.Str(auditRel).Byte('/').Str(rfcStem).Str(".json (staged)").String()
	for _, rid := range audit.Order {
		if err := validateVerdict(rid, audit.Verdicts[rid], audit.order.child(rid), rel); err != nil {
			return err
		}
	}

	dir := treePath(tree, auditRel)
	sweepStaleStaging(dir)
	staging, err := os.MkdirTemp(dir, stagingPrefix)
	if err != nil {
		var tb textbuf.Buffer
		return parseErr(tb.Str(auditRel).Str(": cannot stage a rewrite: ").Err(err))
	}
	defer func() { _ = os.RemoveAll(staging) }()

	var name textbuf.Buffer
	target := name.Str(rfcStem).Str(".json").String()
	path := filepath.Join(staging, target)
	var body textbuf.Buffer
	if err := os.WriteFile(path, []byte(body.Str(pyDump(data)).Byte('\n').String()), 0o600); err != nil {
		var tb textbuf.Buffer
		return parseErr(tb.Str(rel).Str(": cannot write: ").Err(err))
	}
	raw, err := os.ReadFile(path) // #nosec G304 -- the file this function just wrote
	if err != nil {
		var tb textbuf.Buffer
		return parseErr(tb.Str(rel).Str(": cannot read: ").Err(err))
	}
	reread, order, err := decodeOrdered(raw)
	if err != nil {
		var tb textbuf.Buffer
		return parseErr(tb.Str(rel).Str(": cannot read: ").Err(err))
	}
	document, isObject := reread.(map[string]any)
	if !isObject {
		var tb textbuf.Buffer
		return parseErr(tb.Str(rel).Str(": expected a JSON object, got ").Str(pyTypeName(reread)))
	}
	verdicts, isObject := document["requirements"].(map[string]any)
	if isObject {
		verdictOrder := order.child("requirements")
		for _, rid := range verdictOrder.orderOf(verdicts) {
			if err := validateVerdict(rid, verdicts[rid], verdictOrder.child(rid), rel); err != nil {
				return err
			}
		}
	}
	if err := os.Rename(path, auditPath(tree, rfcStem)); err != nil {
		var tb textbuf.Buffer
		return parseErr(tb.Str(auditRel).Byte('/').Str(target).Str(": cannot replace: ").Err(err))
	}
	return nil
}

// sweepStaleStaging removes abandoned staging directories left by a killed run.
//
// Best-effort: this is hygiene, and a sweep that cannot read the directory must
// never stop a write.
func sweepStaleStaging(dir string) {
	cutoff := time.Now().Add(-stagingStaleSeconds * time.Second)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() || !hasStagingPrefix(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		_ = os.RemoveAll(filepath.Join(dir, entry.Name()))
	}
}

// hasStagingPrefix reports whether a directory name is one of this writer's.
func hasStagingPrefix(name string) bool {
	return len(name) >= len(stagingPrefix) && name[:len(stagingPrefix)] == stagingPrefix
}
