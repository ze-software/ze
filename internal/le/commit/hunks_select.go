// Design: docs/contributing/committing.md -- committing some hunks of a shared file
// Related: hunks/hunks.go -- builds the selected blob in a private index

package commit

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/le/commit/hunks"
)

// keywordHunks names one path and the patch holding the hunks of it a commit
// carries.
const keywordHunks = "hunks"

// hunkSelectedDetail is the verification state of a commit carrying selected
// hunks. The certificate's manifest hashes the working-tree file, which holds
// hunks the commit does not carry, so a FRESH answer would vouch for content
// that is not being committed.
const hunkSelectedDetail = "a hunks path commits content the working tree does not hold, so the certificate cannot vouch for it"

// HunkSelection is one `hunks <path> <patch-file>` pair as the caller gave it.
type HunkSelection struct {
	Path  string
	Patch string
}

// hunkSelections pairs the values of the repeated hunks keyword.
func hunkSelections(values []string) []HunkSelection {
	selections := make([]HunkSelection, 0, len(values)/2)
	for index := 0; index+1 < len(values); index += 2 {
		selections = append(selections, HunkSelection{Path: values[index], Patch: values[index+1]})
	}
	return selections
}

// normalizeHunkPaths validates the path of each hunks pair exactly as a file
// path is validated, and answers the pairs with their normalized paths, in the
// order given. Create calls it beside the file and remove paths, before
// anything is written, so a path no route can commit is refused with nothing
// recorded under tmp/. Pairs are not deduplicated: a path named twice is
// refused by selectHunks, never merged.
func normalizeHunkPaths(root string, raw []HunkSelection) ([]HunkSelection, error) {
	pairs := make([]HunkSelection, 0, len(raw))
	for _, pair := range raw {
		path, err := normalizePath(root, pair.Path)
		if err != nil {
			return nil, err
		}
		if err := validateAddPath(root, path); err != nil {
			return nil, err
		}
		pairs = append(pairs, HunkSelection{Path: path, Patch: pair.Patch})
	}
	return pairs, nil
}

// selectHunks builds the blob each hunks pair commits against head. Its pairs
// MUST come from normalizeHunkPaths, which validated every path. It answers the
// selected paths, in the order given, and the Selection every gate reads them
// through. A path also named by file, or named by hunks twice, is refused: it
// would carry two contents.
func selectHunks(root, head string, files []string, pairs []HunkSelection) ([]string, hunks.Selection, error) {
	if len(pairs) == 0 {
		return nil, nil, nil
	}
	whole := make(map[string]bool, len(files))
	for _, path := range files {
		whole[path] = true
	}
	paths := make([]string, 0, len(pairs))
	selection := make(hunks.Selection, len(pairs))
	for _, pair := range pairs {
		path := pair.Path
		if whole[path] {
			return nil, nil, errors.New("hunks " + path + ": the path is also named by file, which commits the whole file; " +
				"name it once, with file for the whole file or hunks for some of its hunks")
		}
		if _, twice := selection[path]; twice {
			return nil, nil, errors.New("hunks " + path + ": the path is named by hunks twice; " +
				"put every hunk this commit carries for it in one patch")
		}
		patch := pair.Patch
		if !filepath.IsAbs(patch) {
			patch = filepath.Join(root, filepath.FromSlash(patch))
		}
		info, err := os.Stat(patch)
		if err != nil {
			return nil, nil, errors.New("hunks " + path + ": cannot read the patch " + pair.Patch + ": " + err.Error() +
				"\n  write it with `git diff -- " + path + " > <patch>` and delete the hunks this commit must not carry")
		}
		if info.IsDir() {
			return nil, nil, errors.New("hunks " + path + ": the patch " + pair.Patch + " is a directory; name the patch file")
		}
		entry, err := hunks.Apply(root, head, path, patch)
		if err != nil {
			return nil, nil, err
		}
		selection[path] = hunks.Selected{Entry: entry, Patch: patch}
		paths = append(paths, path)
	}
	return paths, selection, nil
}
