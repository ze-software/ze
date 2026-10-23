// Design: docs/contributing/rfc-conformance-gates.md -- "The artifacts"
// Overview: rfc.go -- the types, the paths and the closed sets every reader here shares
// Related: evidence_split.go -- converts a file of the retired per-stem layout
//
// evidence_layout.go answers where the per-requirement evidence lives. Every
// discrimination record and every audit verdict is stored in
// `<dir>/<stem>/<requirement-id>.json`, one file per requirement, because the
// records are keyed by requirement and many sessions edit the same RFC at
// once: two sessions proving or judging different requirements then never
// write the same file, so neither has to commit, withhold or carry the other's
// uncommitted hunks.
package rfc

import (
	"os"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// evidenceFile is one per-requirement evidence file: the RFC it belongs to,
// the requirement it holds, and its path relative to the checkout.
type evidenceFile struct {
	Stem string
	RID  string
	Rel  string
}

// evidenceRel answers the file that holds one requirement's evidence under dir.
func evidenceRel(dir, stem, rid string) string {
	var tb textbuf.Buffer
	return tb.Str(dir).Byte('/').Str(stem).Byte('/').Str(rid).Str(jsonSuffix).String()
}

// discriminationRecordRel answers the file that holds one requirement's
// discrimination records.
func discriminationRecordRel(stem, rid string) string {
	return evidenceRel(discriminationRel, stem, rid)
}

// auditRecordRel answers the file that holds one requirement's audit verdict.
func auditRecordRel(stem, rid string) string {
	return evidenceRel(auditRel, stem, rid)
}

// evidenceAt answers the stem and requirement a path names when it is a
// per-requirement file directly under dir, and false for anything else: the
// README, a staging directory, an editor's or writer's dot-named temporary
// file, or a file of the retired per-stem layout.
func evidenceAt(dir, rel string) (evidenceFile, bool) {
	rest, under := strings.CutPrefix(rel, dir+"/")
	if !under {
		return evidenceFile{}, false
	}
	stem, name, nested := strings.Cut(rest, "/")
	if !nested {
		return evidenceFile{}, false
	}
	if strings.Contains(name, "/") {
		return evidenceFile{}, false
	}
	if stem == "" {
		return evidenceFile{}, false
	}
	if strings.HasPrefix(stem, ".") {
		return evidenceFile{}, false
	}
	if strings.HasPrefix(name, ".") {
		return evidenceFile{}, false
	}
	rid, isJSON := strings.CutSuffix(name, jsonSuffix)
	if !isJSON {
		return evidenceFile{}, false
	}
	if rid == "" {
		return evidenceFile{}, false
	}
	return evidenceFile{Stem: stem, RID: rid, Rel: rel}, true
}

// listEvidence answers every per-requirement file under dir in the working
// tree, sorted by path, which is by stem then requirement.
//
// An absent directory is an empty answer. A `.json` file directly under dir is
// REFUSED: it is a file of the retired per-stem layout, and reading past it
// would drop every record it holds from the gate without a word
// (ai/rules/principles.md). The refusal names the command that converts it.
func listEvidence(tree, dir string) ([]evidenceFile, error) {
	entries, err := os.ReadDir(treePath(tree, dir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		var tb textbuf.Buffer
		return nil, parseErr(tb.Str(dir).Str(": cannot read: ").Err(err))
	}
	var out []evidenceFile
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if !entry.IsDir() {
			if strings.HasSuffix(name, jsonSuffix) {
				return nil, retiredLayoutErr(dir, name)
			}
			continue
		}
		var stemRel textbuf.Buffer
		stemDir := stemRel.Str(dir).Byte('/').Str(name).String()
		files, err := os.ReadDir(treePath(tree, stemDir))
		if err != nil {
			var tb textbuf.Buffer
			return nil, parseErr(tb.Str(stemDir).Str(": cannot read: ").Err(err))
		}
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			var fileRel textbuf.Buffer
			if found, held := evidenceAt(dir, fileRel.Str(stemDir).Byte('/').Str(file.Name()).String()); held {
				out = append(out, found)
			}
		}
	}
	slices.SortFunc(out, func(a, b evidenceFile) int { return strings.Compare(a.Rel, b.Rel) })
	return out, nil
}

// retiredLayoutErr refuses a per-stem evidence file and names the conversion.
func retiredLayoutErr(dir, name string) error {
	var tb textbuf.Buffer
	return parseErr(tb.Str(dir).Byte('/').Str(name).
		Str(": a per-stem evidence file of the retired layout. Every record now lives in ").
		Str(dir).Str("/<stem>/<requirement-id>.json, one file per requirement, so two sessions ").
		Str("working different requirements never edit the same file. Convert it with ").
		Str("`./le rfc evidence-split`, then delete it (in a commit, `remove ").Str(dir).Byte('/').
		Str(name).Str("`)"))
}
