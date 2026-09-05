// Design: docs/architecture/core-design.md -- le's one declaration of the spec layout
//
// Package specpath declares where specs live and answers every question about
// that layout: which directories hold them, which bucket a path belongs to,
// what the whole population is, and where one named spec sits.
//
// The three buckets are release buckets. plan/ holds the work the first release
// ships WITHOUT, plan/immediate/ the work a first-release operator meets as a
// bug, and plan/pre-release/ the work the release cannot go out without.
//
// One declaration, and every other surface derives from it
// (ai/rules/principles.md). The layout used to be re-spelled at a dozen glob
// sites, path joins and regexes across internal/le, so adding plan/immediate/
// and plan/pre-release/ left each of those sites reading plan/ alone. A glob
// and a path join answer an empty population for a directory they do not name,
// and a regex that does not match returns "not a spec": both fail open and
// neither says anything (ai/rules/evidence.md).
package specpath

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// The bucket names, as Bucket answers them and as `./le spec status` prints
// them.
const (
	// After is work the first release ships without.
	After = "after"
	// Immediate is work a first-release operator meets as a bug.
	Immediate = "immediate"
	// PreRelease is work the release cannot go out without.
	PreRelease = "pre-release"
)

// Root is the directory every bucket lives under, and is itself the directory
// of the After bucket.
const Root = "plan"

// namePrefix and nameSuffix bound a spec's file name. A file directly inside a
// bucket directory is a spec when, and only when, its name carries both.
const (
	namePrefix = "spec-"
	nameSuffix = ".md"
)

// buckets is THE declaration: one row per release bucket, in bucket order, each
// naming the bucket and the slash-relative directory that holds it. Dirs,
// Globs, Bucket, All and Find each read this table and nothing else, so a
// fourth bucket is one row here and a rename of one is one edit.
//
// These three directories ARE the spec population. Nothing recurses below one,
// so plan/to-review/ is counted nowhere, exactly as it was when plan/ alone was
// the population (plan/journal/gate-excludes-part-of-its-population.md,
// 2026-08-22). A directory that should be counted is a row here, and adding one
// is the only way to be counted.
var buckets = [...]struct {
	name string
	dir  string
}{
	{After, Root},
	{Immediate, Root + "/immediate"},
	{PreRelease, Root + "/pre-release"},
}

// Dirs answers the bucket directories, slash-relative to the checkout root, in
// bucket order.
func Dirs() []string {
	dirs := make([]string, 0, len(buckets))
	for _, bucket := range buckets {
		dirs = append(dirs, bucket.dir)
	}
	return dirs
}

// Globs answers one spec glob for each bucket directory, slash-relative, in
// bucket order. The glob does not recurse: a spec is a file directly inside a
// bucket directory.
func Globs() []string {
	globs := make([]string, 0, len(buckets))
	for _, bucket := range buckets {
		globs = append(globs, bucket.dir+"/"+namePrefix+"*"+nameSuffix)
	}
	return globs
}

// Bucket answers the release bucket a spec belongs to, from its path relative
// to the checkout root. It answers ok false for every path that is not a
// spec-<stem>.md directly inside a bucket directory: a journal row, a README, a
// learned summary, a spec one directory deeper.
//
// The bool is the refusal and it MUST be read. A path this package cannot place
// has no bucket, and naming one anyway would file a pre-release spec as work
// the release ships without (ai/rules/principles.md).
func Bucket(relPath string) (bucket string, ok bool) {
	clean := path.Clean(filepath.ToSlash(relPath))
	base := path.Base(clean)
	if !strings.HasPrefix(base, namePrefix) || !strings.HasSuffix(base, nameSuffix) {
		return "", false
	}
	dir := path.Dir(clean)
	for _, candidate := range buckets {
		if dir == candidate.dir {
			return candidate.name, true
		}
	}
	return "", false
}

// IsSpec reports whether a root-relative path names a spec in one of the
// buckets. It is Bucket without the name, for the gates and the selectors that
// ask only whether the file is a spec.
func IsSpec(relPath string) bool {
	_, ok := Bucket(relPath)
	return ok
}

// All answers every spec across the buckets, relative to root, slash-separated
// and sorted. It does not recurse below a bucket directory.
//
// It REFUSES a root that holds no plan/ directory. filepath.Glob answers an
// empty list and no error for a pattern whose directory is absent, so a caller
// standing in the wrong tree would read "no specs" as an inventory of an empty
// population rather than as a failure to find one. An absent plan/immediate/ or
// plan/pre-release/ is a different fact and stays an answer: Git tracks no
// empty directory, so a bucket whose last spec closed has no directory left.
func All(root string) ([]string, error) {
	info, err := os.Stat(filepath.Join(root, Root))
	if err != nil {
		return nil, fmt.Errorf("read the spec population under %s: %w", Root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("read the spec population under %s: %s is not a directory", Root, Root)
	}

	var specs []string
	for _, glob := range Globs() {
		matches, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(glob)))
		if err != nil {
			return nil, fmt.Errorf("match the spec population %s: %w", glob, err)
		}
		for _, match := range matches {
			relative, err := filepath.Rel(root, match)
			if err != nil {
				return nil, fmt.Errorf("make %s relative to the checkout root: %w", match, err)
			}
			specs = append(specs, filepath.ToSlash(relative))
		}
	}
	sort.Strings(specs)
	return specs, nil
}

// ErrNoSpec is what Find wraps when no bucket holds the name. A caller for
// which an absent spec is an ANSWER tests for it with errors.Is; every other
// caller reads it as the error it is.
var ErrNoSpec = errors.New("no such spec")

// Find answers the one bucket path that holds name, relative to root. name is a
// bare file name (spec-<stem>.md) or a bare <stem>, because the bucket is what
// this function exists to resolve: a session marker, a CLI argument and a hook
// claim each carry the name alone.
//
// It answers an error for a name no bucket holds and for a name two buckets
// hold, and it never answers an empty path. A caller that read "" as a path
// would open the checkout root, and a caller given the first of two matches
// would act on a bucket chosen by table order (ai/rules/principles.md).
func Find(root, name string) (string, error) {
	if strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("find spec %q: name a spec file, not a path", name)
	}
	base := name
	if !strings.HasSuffix(base, nameSuffix) {
		base += nameSuffix
	}
	if !strings.HasPrefix(base, namePrefix) {
		base = namePrefix + base
	}

	found := make([]string, 0, len(buckets))
	for _, bucket := range buckets {
		relative := bucket.dir + "/" + base
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative)))
		if err == nil && info.Mode().IsRegular() {
			found = append(found, relative)
		}
	}
	switch len(found) {
	case 1:
		return found[0], nil
	case 0:
		return "", fmt.Errorf("find %s under %s: %w", base, strings.Join(Dirs(), ", "), ErrNoSpec)
	default:
		return "", fmt.Errorf("spec %s is in two buckets: %s", base, strings.Join(found, ", "))
	}
}

// Stem answers the <stem> of a spec file name: "spec-foo.md" answers "foo". It
// takes the base name, so a bucket path answers the same stem as the bare name.
func Stem(name string) string {
	base := path.Base(filepath.ToSlash(name))
	return strings.TrimSuffix(strings.TrimPrefix(base, namePrefix), nameSuffix)
}
