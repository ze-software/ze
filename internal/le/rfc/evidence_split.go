// Design: docs/contributing/rfc-conformance-gates.md -- "The artifacts"
// Overview: evidence_layout.go -- where the per-requirement evidence lives
//
// evidence_split.go is `./le rfc evidence-split`: it converts each per-stem
// evidence file of the retired layout, `rfc/discrimination/<stem>.json` and
// `rfc/audit/<stem>.json`, into one file per requirement under
// `<dir>/<stem>/<requirement-id>.json`. No reader accepts the retired layout:
// every loader refuses such a file and names this command, so a session that
// wrote one with an older le converts it here rather than losing it.
//
// It never deletes the per-stem file. The commit that lands the converted files
// removes it (`remove <path>` in `./le commit create`), so the record and its
// conversion leave and arrive in one commit. `from head` converts the content
// HEAD holds rather than the working tree's, for a file that also carries
// another session's uncommitted edits: what is committed then derives from
// committed content alone.
package rfc

import (
	"bytes"
	"encoding/json"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// splitSourceHead names the HEAD content as the conversion's source.
const splitSourceHead = "head"

// EvidenceSplitReport is what the conversion answers: each per-stem file it
// read, and per file the requirement files it wrote and the ones that already
// held the converted bytes.
type EvidenceSplitReport struct {
	Files []EvidenceSplitFile `json:"files"`
}

// EvidenceSplitFile is one converted per-stem file.
type EvidenceSplitFile struct {
	Retired   string   `json:"retired"`
	Written   []string `json:"written"`
	Unchanged int      `json:"unchanged"`
	// Skipped says why nothing was converted, when nothing was: a file HEAD
	// does not hold, under `from head`.
	Skipped string `json:"skipped,omitempty"`
}

// Text renders the report for a terminal.
func (r EvidenceSplitReport) Text() string {
	var tb textbuf.Buffer
	if len(r.Files) == 0 {
		return tb.Str("no per-stem evidence file is left to convert\n").String()
	}
	for _, file := range r.Files {
		if file.Skipped != "" {
			tb.Str("skipped ").Str(file.Retired).Str(": ").Str(file.Skipped).Byte('\n')
			continue
		}
		tb.Str("converted ").Str(file.Retired).Str(": ").Int(int64(len(file.Written))).
			Str(" file(s) written, ").Int(int64(file.Unchanged)).Str(" unchanged\n")
	}
	return tb.Str("Delete each converted per-stem file in the commit that carries its requirement files ").
		Str("(`remove <path>` in ./le commit create); every loader refuses it until then\n").String()
}

// retiredDiscriminationFile is the per-stem discrimination file this converts.
type retiredDiscriminationFile struct {
	RFC     string                 `json:"rfc"`
	Records []DiscriminationRecord `json:"records"`
}

// splitEvidence converts every per-stem file under both evidence directories.
// source is "" for the working tree, or splitSourceHead.
func splitEvidence(tree, source string) (EvidenceSplitReport, error) {
	var report EvidenceSplitReport
	for _, dir := range []string{discriminationRel, auditRel} {
		retired, err := retiredFiles(tree, dir)
		if err != nil {
			return EvidenceSplitReport{}, err
		}
		for _, rel := range retired {
			raw, held, err := splitSource(tree, rel, source)
			if err != nil {
				return EvidenceSplitReport{}, err
			}
			if !held {
				report.Files = append(report.Files, EvidenceSplitFile{Retired: rel,
					Skipped: "HEAD does not hold it; convert it without `from head`"})
				continue
			}
			stem := strings.TrimSuffix(rel[len(dir)+1:], jsonSuffix)
			bodies, err := splitBodies(dir, stem, rel, raw)
			if err != nil {
				return EvidenceSplitReport{}, err
			}
			converted, err := writeSplit(tree, rel, bodies)
			if err != nil {
				return EvidenceSplitReport{}, err
			}
			report.Files = append(report.Files, converted)
		}
	}
	return report, nil
}

// retiredFiles answers every `<stem>.json` directly under dir, sorted.
func retiredFiles(tree, dir string) ([]string, error) {
	entries, err := os.ReadDir(treePath(tree, dir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		var tb textbuf.Buffer
		return nil, parseErr(tb.Str(dir).Str(": cannot read: ").Err(err))
	}
	var out []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), jsonSuffix) {
			continue
		}
		var tb textbuf.Buffer
		out = append(out, tb.Str(dir).Byte('/').Str(entry.Name()).String())
	}
	slices.Sort(out)
	return out, nil
}

// splitSource answers the bytes to convert, and false when `from head` names a
// file HEAD does not hold.
func splitSource(tree, rel, source string) ([]byte, bool, error) {
	var tb textbuf.Buffer
	if source != splitSourceHead {
		raw, err := os.ReadFile(treePath(tree, rel)) // #nosec G304 -- a listed evidence file
		if err != nil {
			return nil, false, parseErr(tb.Str(rel).Str(": cannot read: ").Err(err))
		}
		return raw, true, nil
	}
	if _, listed := gitOutput(tree, "cat-file", "-e", tb.Str(headRevision).Byte(':').Str(rel).String()); !listed {
		return nil, false, nil
	}
	blobs, known := gitCatBlobs(tree, headRevision, []string{rel})
	if !known {
		var message textbuf.Buffer
		return nil, false, parseErr(message.Str(rel).Str(": git could not read it at HEAD"))
	}
	return []byte(blobs[rel]), true, nil
}

// splitBodies converts one per-stem file into the bytes of each requirement
// file, keyed by path, validating every record and verdict on the way: a
// conversion must never write a file the loader would refuse.
func splitBodies(dir, stem, rel string, raw []byte) (map[string][]byte, error) {
	if dir == discriminationRel {
		return splitDiscrimination(stem, rel, raw)
	}
	return splitAudit(stem, rel, raw)
}

func splitDiscrimination(stem, rel string, raw []byte) (map[string][]byte, error) {
	var tb textbuf.Buffer
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var file retiredDiscriminationFile
	if err := decoder.Decode(&file); err != nil {
		return nil, parseErr(tb.Str(rel).Str(": cannot read: ").Err(err))
	}
	if file.RFC != stem {
		return nil, parseErr(tb.Str(rel).Str(": 'rfc' is ").Quoted(file.RFC).
			Str(" but the filename says ").Quoted(stem))
	}
	byRID := map[string][]DiscriminationRecord{}
	for index := range file.Records {
		if err := validateDiscrimination(&file.Records[index], rel, index+1); err != nil {
			return nil, err
		}
		byRID[file.Records[index].RID] = append(byRID[file.Records[index].RID], file.Records[index])
	}
	out := make(map[string][]byte, len(byRID))
	for rid, records := range byRID {
		target, err := splitTarget(discriminationRel, stem, rid, rel)
		if err != nil {
			return nil, err
		}
		sort.SliceStable(records, func(left, right int) bool { return records[left].Unit < records[right].Unit })
		body, err := renderDiscriminationRecords(records)
		if err != nil {
			return nil, parseErr(tb.Str(target).Str(": cannot render: ").Err(err))
		}
		out[target] = pageBytes(body)
	}
	return out, nil
}

func splitAudit(stem, rel string, raw []byte) (map[string][]byte, error) {
	var tb textbuf.Buffer
	document, order, err := decodeOrdered(raw)
	if err != nil {
		return nil, parseErr(tb.Str(rel).Str(": cannot read: ").Err(err))
	}
	data, isObject := document.(map[string]any)
	if !isObject {
		return nil, parseErr(tb.Str(rel).Str(": expected a JSON object, got ").Str(pyTypeName(document)))
	}
	if named, _ := data[auditFieldRFC].(string); named != stem {
		return nil, parseErr(tb.Str(rel).Str(": 'rfc' is ").Str(pyRepr(data[auditFieldRFC])).
			Str(" but the filename says ").Str(pyRepr(stem)))
	}
	verdicts, isObject := data[auditFieldRequirements].(map[string]any)
	if !isObject {
		return nil, parseErr(tb.Str(rel).Str(": 'requirements' must be an object"))
	}
	verdictOrder := order.child(auditFieldRequirements)
	out := make(map[string][]byte, len(verdicts))
	for _, rid := range verdictOrder.orderOf(verdicts) {
		if err := validateVerdict(rid, verdicts[rid], verdictOrder.child(rid), rel); err != nil {
			return nil, err
		}
		target, err := splitTarget(auditRel, stem, rid, rel)
		if err != nil {
			return nil, err
		}
		out[target] = renderAuditVerdict(verdicts[rid])
	}
	return out, nil
}

// splitTarget answers the requirement file an id converts into, and refuses an
// id that cannot be a file name.
func splitTarget(dir, stem, rid, rel string) (string, error) {
	target := evidenceRel(dir, stem, rid)
	if found, held := evidenceAt(dir, target); held && found.RID == rid {
		return target, nil
	}
	var tb textbuf.Buffer
	return "", parseErr(tb.Str(rel).Str(": requirement ").Quoted(rid).
		Str(" cannot name a file under ").Str(dir).Byte('/').Str(stem).Byte('/'))
}

// writeSplit writes each converted file whose bytes differ from what it holds.
func writeSplit(tree, rel string, bodies map[string][]byte) (EvidenceSplitFile, error) {
	out := EvidenceSplitFile{Retired: rel, Written: []string{}}
	for _, target := range sortedKeysOf(bodies) {
		current, err := os.ReadFile(treePath(tree, target)) // #nosec G304 -- a path this conversion derived
		if err == nil && bytes.Equal(current, bodies[target]) {
			out.Unchanged++
			continue
		}
		if err := writeArtifact(tree, target, bodies[target]); err != nil {
			return EvidenceSplitFile{}, err
		}
		out.Written = append(out.Written, target)
	}
	return out, nil
}
