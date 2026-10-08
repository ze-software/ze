// Design: docs/contributing/rfc-conformance-gates.md -- the reference refusal
// Related: check.go -- the centralized check driver that runs this stage
// Related: inventory.go -- SourcePath, the one lookup of an RFC's own text
//
// check_reference.go keeps the non-normative documents under reference/ out of
// the requirement corpus. Only the owner moves a document from there into
// rfc/full/ or rfc/drafts/, and that move is what enrolls it as a source.
package rfc

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// checkReferenceSources refuses every requirement artifact keyed by a stem
// whose only text is under reference/: a summary under rfc/short/ and an
// extraction sign-off under rfc/extraction/. It also refuses a summary that
// cites a reference/ path, whatever rfc/full/ or rfc/drafts/ hold, because the
// citation names a non-normative document as the source of its rows.
//
// The text lookup is SourcePath, the same one every quote check reads, so a
// stem this stage accepts is a stem whose rows are quoted against enrolled text.
func checkReferenceSources(tree string, summaries map[string]bool) ([]string, error) {
	held, err := referenceTexts(tree)
	if err != nil {
		return nil, err
	}
	extractions, err := extractionStems(tree)
	if err != nil {
		return nil, err
	}
	var errs []string
	for _, stem := range sortedSet(summaries) {
		var name textbuf.Buffer
		rel := name.Str(summaryRel).Byte('/').Str(stem).Str(".md").String()
		if message, refused := referenceOnly(tree, held, rel, stem); refused {
			errs = append(errs, message)
		}
		content, err := readFile(treePath(tree, rel), rel)
		if err != nil {
			return nil, err
		}
		if citesReference(content) {
			var tb textbuf.Buffer
			errs = append(errs, tb.Str(rel).Str(": cites a ").Str(referenceRel).
				Str("/ path. A document there is non-normative and is never the source of a summary or a requirement row (").
				Str(referenceReadmeRel).Str("). Quote ").Str(fullRel).Str("/ or ").Str(draftsRel).
				Str("/, where only the owner moves a document").String())
		}
	}
	for _, stem := range sortedSet(extractions) {
		if message, refused := referenceOnly(tree, held, correctionExtractionRel(stem), stem); refused {
			errs = append(errs, message)
		}
	}
	return errs, nil
}

// referenceOnly answers the refusal for an artifact whose stem has a text under
// reference/ and none under rfc/full/ or rfc/drafts/.
func referenceOnly(tree string, held map[string]string, rel, stem string) (string, bool) {
	copyRel, present := held[stem]
	if !present {
		return "", false
	}
	if _, enrolled := SourcePath(tree, stem); enrolled {
		return "", false
	}
	var tb textbuf.Buffer
	return tb.Str(rel).Str(": the only text of ").Str(stem).Str(" is ").Str(copyRel).
		Str(", a non-normative document that is never a requirement source (").Str(referenceReadmeRel).
		Str("). Only the owner moves it to ").Str(fullRel).Byte('/').Str(stem).Str(".txt or ").
		Str(draftsRel).Byte('/').Str(stem).Str(".txt; until then remove this artifact").String(), true
}

// referenceTexts answers the tree-relative path of every `<stem>.txt` under
// reference/, keyed by stem. A tree without reference/ holds none, which is an
// answer; any other read failure is an error, because an unread tree would
// pass every artifact.
func referenceTexts(tree string) (map[string]string, error) {
	out := map[string]string{}
	root := treePath(tree, referenceRel)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".txt") {
			return nil
		}
		stem := strings.TrimSuffix(name, ".txt")
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
		return nil, parseErr(tb.Str(referenceRel).Str(": cannot read: ").Err(err))
	}
	return out, nil
}

// citesReference answers whether a text names a path under the top-level
// reference/ tree. The match starts a path token, so a reference/ segment
// inside a longer path or a URL names another tree and does not count.
func citesReference(content string) bool {
	const token = referenceRel + "/"
	for offset := 0; ; {
		index := strings.Index(content[offset:], token)
		if index < 0 {
			return false
		}
		at := offset + index
		if at == 0 || !pathByte(content[at-1]) {
			return true
		}
		offset = at + len(token)
	}
}

// pathByte answers whether a byte can sit inside a path token, so a reference/
// it precedes is a segment of a longer path rather than its start.
func pathByte(b byte) bool {
	if b >= 'a' && b <= 'z' {
		return true
	}
	if b >= 'A' && b <= 'Z' {
		return true
	}
	if b >= '0' && b <= '9' {
		return true
	}
	return strings.IndexByte("./-_", b) >= 0
}
