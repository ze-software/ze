// Design: docs/contributing/spec-workflow.md -- the roadmap Profiles column.
// Related: roadmap.go -- Collect, which places each item through rfcCorpus.
// Related: rfccorpus/register.go -- registers the reader of the RFC corpus.

package roadmap

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"hash"
	"path"
	"slices"
	"strings"

	specstatus "github.com/ze-software/ze/internal/le/spec/status"
)

// objectBlob is the git object type of a file.
const objectBlob = "blob"

// rfcDirs are the corpus directories the Profiles column reads, each listed
// one level deep, as rfc.LoadRequirementStems, rfc.LoadProfiles and
// graph.Load read them from a checkout.
var rfcDirs = []string{"rfc/short", "rfc/profiles", "rfc/full", "rfc/drafts"}

// Corpus is what one revision's rfc/ tree says about `RFC gaps` items.
type Corpus struct {
	// RequirementStems maps each requirement id a summary lists to its stem.
	RequirementStems map[string]string
	// SummaryStems holds the stem of every rfc/short/ summary.
	SummaryStems map[string]bool
	// ProfilesOf names, for each stem, every use-case profile whose closure
	// holds it.
	ProfilesOf map[string][]string
	// ProfileCount is the number of use-case profiles the revision declares.
	ProfileCount int
}

// CorpusReader builds the Corpus from one revision's corpus files, keyed by
// path (rfc/short/<stem>.md, rfc/profiles/<name>.md, rfc/full/<stem>.txt,
// rfc/drafts/<stem>.txt). It refuses a corpus it cannot fully read.
type CorpusReader func(files map[string]string) (Corpus, error)

// corpusReader is the registered reader. The RFC corpus parser lives in
// internal/le/rfc, whose dependencies reach most le namespaces; it registers
// from internal/le/spec/roadmap/rfccorpus so this package's own tests link
// none of them.
var corpusReader CorpusReader

// errNoCorpusReader is the answer when no reader is linked into the binary:
// the gaps are not judged, never judged empty.
var errNoCorpusReader = errors.New("no RFC corpus reader is linked into this binary; " +
	"blank-import github.com/ze-software/ze/internal/le/spec/roadmap/rfccorpus")

// RegisterCorpusReader installs the reader Collect places `RFC gaps` items
// with. It MUST be called once, from an init() in a register.go.
func RegisterCorpusReader(reader CorpusReader) {
	if reader == nil {
		panic("BUG: roadmap.RegisterCorpusReader: nil reader")
	}
	if corpusReader != nil {
		panic("BUG: roadmap.RegisterCorpusReader: called twice")
	}
	corpusReader = reader
}

// rfcCorpus is a loaded Corpus, or the reason it could not be loaded, with
// the files it was read from: they are inputs of the snapshot as much as the
// specs are, so they join its input digest.
type rfcCorpus struct {
	corpus   Corpus
	files    map[string]string
	unjudged error
}

// digest writes every corpus file read into the snapshot's input digest,
// sorted by path, with the same length framing Collect gives a spec.
func (loaded *rfcCorpus) digest(sum hash.Hash) {
	paths := make([]string, 0, len(loaded.files))
	for rel := range loaded.files {
		paths = append(paths, rel)
	}
	slices.Sort(paths)
	for _, rel := range paths {
		body := loaded.files[rel]
		fmt.Fprintf(sum, "%d:%s%d:", len(rel), rel, len(body)) //nolint:errcheck // hash.Hash writes cannot fail.
		sum.Write([]byte(body))                                //nolint:errcheck // hash.Hash writes cannot fail.
	}
}

// loadRFCCorpus reads the corpus from revision's objects, never the working
// tree, so the column describes the same commit as the rest of the snapshot.
func loadRFCCorpus(ctx context.Context, root, revision string) *rfcCorpus {
	if corpusReader == nil {
		return &rfcCorpus{unjudged: errNoCorpusReader}
	}
	files, err := rfcBlobs(ctx, root, revision)
	if err != nil {
		return &rfcCorpus{unjudged: err}
	}
	corpus, err := corpusReader(files)
	if err != nil {
		return &rfcCorpus{files: files, unjudged: err}
	}
	return &rfcCorpus{corpus: corpus, files: files}
}

// place fills item.Profiles from item.RFCGaps, and states as a diagnostic
// every item the corpus cannot place: an unknown id or stem, a revision with
// no use-case profile, or a corpus that did not load.
func (loaded *rfcCorpus) place(item *Item) {
	if loaded.unjudged != nil {
		item.Diagnostics = append(item.Diagnostics, "RFC gaps unjudged, the revision's rfc/ corpus did not load: "+loaded.unjudged.Error())
		return
	}
	corpus := &loaded.corpus
	stems, unknown := specstatus.ResolveRFCGaps(item.RFCGaps, corpus.RequirementStems, corpus.SummaryStems)
	for _, gap := range unknown {
		item.Diagnostics = append(item.Diagnostics, specstatus.UnknownRFCGap(item.Path, gap))
	}
	if len(stems) == 0 {
		return
	}
	if corpus.ProfileCount == 0 {
		item.Diagnostics = append(item.Diagnostics, "RFC gaps unplaced: this revision declares no use-case profile under rfc/profiles/")
		return
	}
	for _, stem := range stems {
		for _, profile := range corpus.ProfilesOf[stem] {
			if !slices.Contains(item.Profiles, profile) {
				item.Profiles = append(item.Profiles, profile)
			}
		}
	}
	slices.Sort(item.Profiles)
}

// rfcBlobs answers every summary, profile and RFC text of revision, keyed by
// its path. A directory the revision does not hold contributes nothing, as an
// absent directory does for the checkout readers.
func rfcBlobs(ctx context.Context, root, revision string) (map[string]string, error) {
	args := make([]string, 0, 5+len(rfcDirs))
	args = append(args, "ls-tree", "-z", "--full-tree", revision, "--")
	for _, dir := range rfcDirs {
		args = append(args, dir+"/")
	}
	output, err := git(ctx, root, nil, args...)
	if err != nil {
		return nil, fmt.Errorf("%w: %s rfc/: %w", ErrRead, revision, err)
	}
	entries := make([]treeEntry, 0)
	for record := range bytes.SplitSeq(output, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		header, name, ok := bytes.Cut(record, []byte{'\t'})
		if !ok {
			return nil, fmt.Errorf("%w: malformed rfc/ tree entry at %s", ErrRead, revision)
		}
		fields := strings.Fields(string(header))
		if len(fields) != 3 {
			return nil, fmt.Errorf("%w: malformed rfc/ tree header at %s", ErrRead, revision)
		}
		if fields[1] != objectBlob {
			continue
		}
		rel := string(name)
		if !corpusFile(rel) {
			continue
		}
		entries = append(entries, treeEntry{path: rel, object: fields[2]})
	}
	blobs, err := readBlobs(ctx, root, entries)
	if err != nil {
		return nil, err
	}
	files := make(map[string]string, len(entries))
	for index, entry := range entries {
		files[entry.path] = string(blobs[index])
	}
	return files, nil
}

// corpusFile is whether rel is a file the checkout readers would read: a
// `.md` summary or profile, or a `.txt` RFC or draft text.
func corpusFile(rel string) bool {
	dir, name := path.Split(rel)
	switch dir {
	case "rfc/short/", "rfc/profiles/":
		return strings.HasSuffix(name, ".md")
	case "rfc/full/", "rfc/drafts/":
		return strings.HasSuffix(name, ".txt")
	}
	return false
}
