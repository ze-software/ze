// Design: docs/contributing/rfc-conformance-gates.md -- the declined refusal
// Related: check.go -- the centralized check driver that runs this stage
// Related: check_reference.go -- the sibling stage for reference/ and shared citesTree helper
// Related: inventory.go -- SourcePath, the one lookup of an RFC's own text
//
// check_declined.go keeps RFCs and Internet-Drafts filed under rfc/declined/
// out of the implementation corpus. A document there will never be implemented
// (rfc/declined/README.md), so it must not also sit under rfc/full/,
// rfc/drafts/, rfc/short/, or rfc/extraction/, and no active summary may cite
// it as a requirement source.
package rfc

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// checkDeclinedSources refuses every active corpus artifact keyed by a stem
// filed under rfc/declined/ (a source text in rfc/full/ or rfc/drafts/, a
// summary in rfc/short/, or an extraction sign-off in rfc/extraction/), and
// refuses any rfc/short/ summary that cites an rfc/declined/ path.
func checkDeclinedSources(tree string, summaries map[string]bool) ([]string, error) {
	declined, err := declinedStems(tree)
	if err != nil {
		return nil, err
	}
	extractions, err := extractionStems(tree)
	if err != nil {
		return nil, err
	}
	var errs []string
	for _, stem := range sortedKeysOf(declined) {
		declinedPath := declined[stem]
		if sourceRel, held := SourcePath(tree, stem); held {
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(sourceRel).Str(": ").Str(stem).
				Str(" is filed as declined at ").Str(declinedPath).
				Str(" and will never be implemented (").Str(declinedReadmeRel).
				Str("). Remove it from ").Str(fullRel).Str("/ or ").Str(draftsRel).
				Str("/, or move it out of ").Str(declinedRel).Str("/ if the owner reverses the decision").String())
		}
		if summaries[stem] {
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(summaryRel).Byte('/').Str(stem).Str(".md: ").Str(stem).
				Str(" is filed as declined at ").Str(declinedPath).
				Str(" and will never be implemented (").Str(declinedReadmeRel).
				Str("). Move this summary under ").Str(declinedRel).Str("/short/ or remove it").String())
		}
		if extractions[stem] {
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(correctionExtractionRel(stem)).Str(": ").Str(stem).
				Str(" is filed as declined at ").Str(declinedPath).
				Str(" and will never be implemented (").Str(declinedReadmeRel).
				Str("). Remove this extraction sign-off").String())
		}
	}
	for _, stem := range sortedSet(summaries) {
		var name textbuf.Buffer
		rel := name.Str(summaryRel).Byte('/').Str(stem).Str(".md").String()
		content, err := readFile(treePath(tree, rel), rel)
		if err != nil {
			return nil, err
		}
		if citesTree(content, declinedRel) {
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(rel).Str(": cites an ").Str(declinedRel).
				Str("/ path. A document there will never be implemented and is never a requirement source (").
				Str(declinedReadmeRel).Str("). Remove the citation").String())
		}
	}
	return errs, nil
}

// declinedStems answers the tree-relative path of every `<stem>.txt` or
// `<stem>.md` (other than README.md) under rfc/declined/, keyed by stem. A
// tree without rfc/declined/ holds none, which is an answer; any other read
// failure is an error, because an unread tree would pass every artifact.
func declinedStems(tree string) (map[string]string, error) {
	out := map[string]string{}
	root := treePath(tree, declinedRel)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		name := entry.Name()
		if strings.EqualFold(name, "README.md") {
			return nil
		}
		stem := strings.TrimSuffix(name, ".txt")
		if stem == name {
			stem = strings.TrimSuffix(name, ".md")
		}
		if stem == name {
			return nil
		}
		if stem == "" {
			return nil
		}
		if _, seen := out[stem]; !seen {
			out[stem] = relTo(tree, path)
		}
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		var tb textbuf.Buffer
		return nil, parseErr(tb.Str(declinedRel).Str(": cannot read: ").Err(err))
	}
	return out, nil
}
