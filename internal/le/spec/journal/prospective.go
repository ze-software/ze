// Design: plan/journal/README.md -- commit validation reads the prospective Git tree.
package specjournal

import (
	"context"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/le/repo/objects"
)

// revisionSource is the immutable baseline and carried tree of one preparation.
// Only ProspectiveBase constructs it after Git has resolved both tree objects.
type revisionSource struct {
	before string
	tree   string
}

// ProspectiveBase resolves the retained baseline and prospective tree once.
// Callers MUST pass this result to both ValidateAgainst and UnownedOpenRows.
// A failed resolution returns no usable immutable source, never a live fallback.
func ProspectiveBase(root, before, tree string) (Base, error) {
	parent, err := objects.ResolveTree(context.Background(), root, before)
	if err != nil {
		return Base{}, err
	}
	carried, err := objects.ResolveTree(context.Background(), root, tree)
	if err != nil {
		return Base{}, err
	}
	return Base{source: &revisionSource{before: parent, tree: carried}}, nil
}

type revisionEntry struct {
	object string
	mode   string
}

func revisionEntries(root, revision string, paths []string) (map[string]revisionEntry, error) {
	entries := make(map[string]revisionEntry)
	if len(paths) == 0 {
		return entries, nil
	}
	args := append([]string{"--literal-pathspecs", "ls-tree", "-r", "-z", revision, "--"}, paths...)
	answer := runGit(root, args...)
	if answer.startErr != nil {
		return nil, answer.startErr
	}
	if answer.code != 0 {
		return nil, gitReadError("read immutable journal tree: ", answer)
	}
	for row := range strings.SplitSeq(string(answer.stdout), "\x00") {
		if row == "" {
			continue
		}
		metadata, name, found := strings.Cut(row, "\t")
		if !found {
			return nil, fmt.Errorf("immutable journal entry has no path: %q", row)
		}
		if strings.ContainsAny(name, "\x00\t\r\n") {
			return nil, fmt.Errorf("immutable journal path contains controls: %q", name)
		}
		fields := strings.Fields(metadata)
		if len(fields) != 3 {
			return nil, fmt.Errorf("immutable journal entry is malformed: %q", row)
		}
		if fields[1] != "blob" {
			continue
		}
		entries[name] = revisionEntry{object: fields[2], mode: fields[0]}
	}
	return entries, nil
}

func readRevisionEntries(root string, entries map[string]revisionEntry) (map[string]string, error) {
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	slices.Sort(names)
	ids := make([]string, 0, len(names))
	for _, name := range names {
		ids = append(ids, entries[name].object)
	}
	blobs, err := objects.Read(context.Background(), root, ids)
	if err != nil {
		return nil, err
	}
	files := make(map[string]string, len(names))
	for index, name := range names {
		files[name] = string(blobs[index])
	}
	return files, nil
}

func revisionFiles(root, revision string, paths []string) (map[string]string, error) {
	entries, err := revisionEntries(root, revision, paths)
	if err != nil {
		return nil, err
	}
	return readRevisionEntries(root, entries)
}

func (base Base) classFiles(root string) ([]string, error) {
	if base.source == nil {
		return classFilesAtHead(root)
	}
	entries, err := revisionEntries(root, base.source.before, []string{journalDir})
	if err != nil {
		return nil, err
	}
	var paths []string
	for name := range entries {
		if !strings.HasSuffix(name, markdownExt) {
			continue
		}
		if strings.HasSuffix(name, "/README.md") {
			continue
		}
		paths = append(paths, name)
	}
	slices.Sort(paths)
	return paths, nil
}

type revisionCaptureSource struct {
	root     string
	revision string
}

func (source revisionCaptureSource) declares(ref funcRef) (bool, error) {
	entries, err := revisionEntries(source.root, source.revision, []string{ref.dir})
	if err != nil {
		return false, err
	}
	for name, entry := range entries {
		directGo := path.Dir(name) == ref.dir && strings.HasSuffix(name, ".go")
		if !directGo {
			delete(entries, name)
			continue
		}
		if entry.mode == "120000" {
			delete(entries, name)
		}
	}
	files, err := readRevisionEntries(source.root, entries)
	if err != nil {
		return false, err
	}
	for _, content := range files {
		if declaresFunc(content, ref.name) {
			return true, nil
		}
	}
	return false, nil
}

func (source revisionCaptureSource) hasPoint(id string) (bool, error) {
	name := pointsDir + id + markdownExt
	entries, err := revisionEntries(source.root, source.revision, []string{name})
	if err != nil {
		return false, err
	}
	entry, found := entries[name]
	if !found {
		return false, nil
	}
	if entry.mode == "120000" {
		return false, nil
	}
	_, err = readRevisionEntries(source.root, entries)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (revisionCaptureSource) where() string { return "in the prospective tree" }

func (base Base) owners(root string) ([]string, error) {
	if base.source == nil {
		return headOwners(root)
	}
	revision := base.source.before
	answer := runGit(root, "grep", "-z", "--name-only", "--fixed-strings", "-e", OwnedRowsHeading, revision, "--", "plan")
	if answer.startErr != nil {
		return nil, answer.startErr
	}
	if answer.code == 1 {
		return nil, nil
	}
	if answer.code != 0 {
		return nil, gitReadError("read immutable journal owners: ", answer)
	}
	var owners []string
	for name := range strings.SplitSeq(string(answer.stdout), "\x00") {
		name = strings.TrimPrefix(name, revision+":")
		if isSpecFile(name) {
			owners = append(owners, name)
		}
	}
	return owners, nil
}
