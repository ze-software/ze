// Design: reference/README.md -- the non-normative IETF reference tree
// Related: datatracker.go -- the datatracker reads this refresh is built on
// Related: index.go -- the INDEX.tsv format, read and written
// Related: report.go -- what a refresh answers
//
// The refresh is all or nothing up to the filesystem. Every datatracker read
// and every text download finishes in memory before the first file is
// written, so a fetch that fails leaves the tree and the previous index as
// they were. It writes only under reference/ietf/: a document the owner holds
// in rfc/full or rfc/drafts is named by its row and never copied or touched,
// because a file there is a candidate for enrolment and widens RFC validation.

package dataietfreference

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// ietfRel is the tree this refresh owns. Its subdirectories ARE the working
// groups: adding a group is creating its directory, and nothing else names one.
const ietfRel = "reference/ietf"

// indexRel is the index this refresh owns, relative to the tree.
const indexRel = ietfRel + "/INDEX.tsv"

// The two owner-held trees a row can point at, in the order a row prefers them.
const (
	rfcFullRel   = "rfc/full"
	rfcDraftsRel = "rfc/drafts"
)

// The two document kinds, as the datatracker spells them and the index stores them.
const (
	kindDraft = "draft"
	kindRFC   = "rfc"
)

// Refresh rewrites reference/ietf/INDEX.tsv from the datatracker, downloads
// the text of every selected document the reference tree holds and lacks (or,
// for a draft, whose revision moved), and reports what changed against the
// previous index. It never deletes: a file no longer selected is reported.
//
// today dates the index header. Any failed fetch returns an error and writes
// nothing.
func Refresh(ctx context.Context, root string, today time.Time, src Sources) (Report, error) {
	groups, err := workingGroups(root)
	if err != nil {
		return Report{}, err
	}
	previous, err := readIndex(root)
	if err != nil {
		return Report{}, err
	}
	var rows []row
	for _, wg := range groups {
		selected, err := selectGroup(ctx, root, wg, src)
		if err != nil {
			return Report{}, err
		}
		rows = append(rows, selected...)
	}
	slices.SortFunc(rows, compareRows)

	texts, err := downloadTexts(ctx, root, rows, previous, src)
	if err != nil {
		return Report{}, err
	}
	report, err := compare(root, groups, rows, previous, texts)
	if err != nil {
		return Report{}, err
	}
	for _, text := range texts {
		if err := writeAtomic(filepath.Join(root, filepath.FromSlash(text.rel)), text.body); err != nil {
			return Report{}, err
		}
	}
	if err := writeAtomic(filepath.Join(root, filepath.FromSlash(indexRel)), renderIndex(groups, rows, today)); err != nil {
		return Report{}, err
	}
	return report, nil
}

// workingGroups answers the subdirectories of reference/ietf, sorted. A tree
// with none is refused: an empty index would claim the groups publish nothing.
func workingGroups(root string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(ietfRel)))
	if err != nil {
		return nil, fmt.Errorf("list working groups under %s: %w", ietfRel, err)
	}
	var groups []string
	for _, entry := range entries {
		if entry.IsDir() {
			groups = append(groups, entry.Name())
		}
	}
	if len(groups) == 0 {
		return nil, fmt.Errorf("no working-group directory under %s: create reference/ietf/<wg>/ for each group to track", ietfRel)
	}
	return groups, nil
}

// selectGroup reads one working group's documents and answers its rows: every
// RFC whose category is BCP or Informational, and every active draft.
func selectGroup(ctx context.Context, root, wg string, src Sources) ([]row, error) {
	id, err := src.groupID(ctx, wg)
	if err != nil {
		return nil, err
	}
	rfcs, err := src.documents(ctx, id, kindRFC)
	if err != nil {
		return nil, err
	}
	drafts, err := src.documents(ctx, id, kindDraft)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var rows []row
	for i := range rfcs {
		doc := &rfcs[i]
		if seen[doc.Name] {
			continue
		}
		code := lastSegment(doc.StdLevel)
		if code != "bcp" && code != "inf" {
			continue
		}
		seen[doc.Name] = true
		r, err := newRow(root, wg, kindRFC, code, doc)
		if err != nil {
			return nil, err
		}
		r.obsoletedBy, err = src.obsoletedBy(ctx, doc.Name)
		if err != nil {
			return nil, err
		}
		rows = append(rows, r)
	}
	for i := range drafts {
		doc := &drafts[i]
		if seen[doc.Name] {
			continue
		}
		if !doc.active() {
			continue
		}
		seen[doc.Name] = true
		r, err := newRow(root, wg, kindDraft, lastSegment(doc.IntendedStdLevel), doc)
		if err != nil {
			return nil, err
		}
		rows = append(rows, r)
	}
	return rows, nil
}

// newRow builds one index row. An RFC carries no revision.
func newRow(root, wg, kind, code string, doc *document) (row, error) {
	category, err := categoryName(code)
	if err != nil {
		return row{}, fmt.Errorf("%s: %w", doc.Name, err)
	}
	r := row{wg: wg, kind: kind, stem: doc.Name, category: category, title: collapse(doc.Title)}
	if kind == kindDraft {
		r.revision = doc.Rev
	}
	r.location, err = location(root, wg, doc.Name)
	if err != nil {
		return row{}, err
	}
	return r, nil
}

// location answers where the text of stem lives: the owner's rfc/full copy,
// else the owner's rfc/drafts copy, else this working group's reference
// directory, which is the only one the refresh writes.
func location(root, wg, stem string) (string, error) {
	for _, dir := range []string{rfcFullRel, rfcDraftsRel} {
		held, err := present(filepath.Join(root, filepath.FromSlash(dir), stem+".txt"))
		if err != nil {
			return "", err
		}
		if held {
			return dir, nil
		}
	}
	return ietfRel + "/" + wg, nil
}

// present answers whether path exists. A stat that fails for any other reason
// is an error, never "absent".
func present(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("stat %s: %w", path, err)
}

// text is one document body fetched for the reference tree, held until every
// fetch has succeeded.
type text struct {
	key  string
	rel  string
	body []byte
}

// downloadTexts fetches the text of every row held in the reference tree whose
// file is missing, or, for a draft, whose revision differs from the previous
// index's. An RFC text does not change once published, so a present one stays.
func downloadTexts(ctx context.Context, root string, rows []row, previous map[string]row, src Sources) ([]text, error) {
	var texts []text
	for i := range rows {
		r := &rows[i]
		if r.location != ietfRel+"/"+r.wg {
			continue
		}
		rel := r.location + "/" + r.stem + ".txt"
		held, err := present(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return nil, err
		}
		if held && r.kind == kindRFC {
			continue
		}
		if held && previous[r.key()].revision == r.revision {
			continue
		}
		body, err := src.text(ctx, r.kind, r.stem, r.revision)
		if err != nil {
			return nil, err
		}
		texts = append(texts, text{key: r.key(), rel: rel, body: body})
	}
	return texts, nil
}

// compare classifies every row against the previous index, and lists every
// file the reference tree holds that no row selects any longer.
func compare(root string, groups []string, rows []row, previous map[string]row, texts []text) (Report, error) {
	report := Report{Index: indexRel, Rows: len(rows)}
	refetched := map[string]bool{}
	for _, t := range texts {
		refetched[t.key] = true
		report.Downloaded = append(report.Downloaded, t.rel)
	}
	listed := map[string]bool{}
	for i := range rows {
		r := &rows[i]
		key := r.key()
		listed[r.location+"/"+r.stem+".txt"] = true
		old, known := previous[key]
		switch {
		case !known:
			report.Added = append(report.Added, key)
		case old != *r || refetched[key]:
			report.Updated = append(report.Updated, key)
		default:
			report.Unchanged++
		}
	}
	for _, wg := range groups {
		dir := ietfRel + "/" + wg
		entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(dir)))
		if err != nil {
			return Report{}, fmt.Errorf("list %s: %w", dir, err)
		}
		for _, entry := range entries {
			rel := dir + "/" + entry.Name()
			if entry.IsDir() {
				continue
			}
			if !strings.HasSuffix(entry.Name(), ".txt") {
				continue
			}
			if listed[rel] {
				continue
			}
			report.NoLongerListed = append(report.NoLongerListed, rel)
		}
	}
	return report, nil
}

// writeAtomic replaces path through a temporary file and a rename, so a
// reader never sees half a file.
func writeAtomic(path string, body []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".ietf-reference-*")
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	name := tmp.Name()
	_, writeErr := tmp.Write(body)
	closeErr := tmp.Close()
	if err := errors.Join(writeErr, closeErr, os.Chmod(name, 0o644)); err != nil { //nolint:gosec // reference text is read by every developer
		return errors.Join(fmt.Errorf("write %s: %w", path, err), os.Remove(name))
	}
	if err := os.Rename(name, path); err != nil {
		return errors.Join(fmt.Errorf("write %s: %w", path, err), os.Remove(name))
	}
	return nil
}
