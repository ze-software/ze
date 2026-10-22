// Design: docs/architecture/core-design.md -- le's one declaration of the spec layout
//
// Package specpath declares where specs live and answers every question about
// that layout: which directories hold them, which bucket a path belongs to,
// what the whole population is, and where one named spec sits.
//
// Current placement and historical interpretation are separate contracts. Current
// inventory refuses obsolete locations; committed history retains its original
// classifications rather than guessing where an old work item belongs today.
package specpath

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// Category names identify current work queues, independently of lifecycle status.
const (
	Fixes       = "fixes"
	Validation  = "validation"
	Delivery    = "delivery"
	Features    = "features"
	Engineering = "engineering"
	Proposals   = "proposals"
)

// Root is the directory every category lives under.
const Root = "plan"

// namePrefix and nameSuffix bound a spec's file name. A file directly inside a
// bucket directory is a spec when, and only when, its name carries both.
const (
	namePrefix = "spec-"
	nameSuffix = ".md"
)

// Category declares a queue's identity, location, display label and release obligation.
type Category struct {
	Name     string `json:"name"`
	Dir      string `json:"dir"`
	Label    string `json:"label"`
	Required bool   `json:"required"`
}

// buckets is the one current declaration. Inventory, counts and presentation
// derive from these rows; proposals are visible without being release commitments.
var buckets = [...]Category{
	{Fixes, Root + "/release/fixes", "Correctness fixes", true},
	{Validation, Root + "/release/validation", "Release validation", true},
	{Delivery, Root + "/release/delivery", "Release delivery", true},
	{Features, Root + "/post-release/features", "Post-release features", false},
	{Engineering, Root + "/post-release/engineering", "Post-release engineering", false},
	{Proposals, Root + "/proposals", "Unapproved proposals", false},
}

// historicalBuckets describes the old population, not a current fallback. In
// particular, "after" never implies that an old spec was an absent feature.
var historicalBuckets = [...]Category{
	{"after", Root, "Historical optional work", false},
	{"immediate", Root + "/immediate", "Historical operator-visible obligations", true},
	{"pre-release", Root + "/pre-release", "Historical release obligations", true},
}

// historicalProposal was recognizable work but excluded from the old inventory.
var historicalProposal = Category{Name: "to-review", Dir: Root + "/to-review"}

// Categories returns the current declaration in reporting order. The caller owns
// the slice, so modifying a report cannot change subsequent inventory semantics.
func Categories() []Category { return slices.Clone(buckets[:]) }

// HistoricalCategories returns only the retired layout for committed snapshots.
// It MUST NOT be used to admit current filesystem placements.
func HistoricalCategories() []Category { return slices.Clone(historicalBuckets[:]) }

// HistoricalDirs lists recognized citation and relocation locations, not the
// current inventory. Current filesystem callers MUST use Dirs.
func HistoricalDirs() []string {
	dirs := Dirs()
	for _, category := range historicalBuckets {
		dirs = append(dirs, category.Dir)
	}
	return append(dirs, historicalProposal.Dir)
}

// Dirs answers the bucket directories, slash-relative to the checkout root, in
// bucket order.
func Dirs() []string {
	dirs := make([]string, 0, len(buckets))
	for _, bucket := range buckets {
		dirs = append(dirs, bucket.Dir)
	}
	return dirs
}

// Globs answers one spec glob for each bucket directory, slash-relative, in
// bucket order. The glob does not recurse: a spec is a file directly inside a
// bucket directory.
func Globs() []string {
	globs := make([]string, 0, len(buckets))
	for _, bucket := range buckets {
		globs = append(globs, bucket.Dir+"/"+namePrefix+"*"+nameSuffix)
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
	return bucketIn(relPath, buckets[:])
}

// HistoricalBucket recognizes committed sources, including old-to-new relocations.
// Callers inspecting current files MUST use Bucket instead.
// It also recognizes the formerly uncounted to-review directory for relocation;
// HistoricalCategories deliberately excludes that directory from old reports.
func HistoricalBucket(relPath string) (string, bool) {
	if bucket, ok := Bucket(relPath); ok {
		return bucket, true
	}
	if bucket, ok := bucketIn(relPath, historicalBuckets[:]); ok {
		return bucket, true
	}
	return bucketIn(relPath, []Category{historicalProposal})
}

func bucketIn(relPath string, categories []Category) (string, bool) {
	slash := filepath.ToSlash(relPath)
	for component := range strings.SplitSeq(slash, "/") {
		if component == ".." {
			return "", false
		}
	}
	clean := path.Clean(slash)
	base := path.Base(clean)
	if !strings.HasPrefix(base, namePrefix) {
		return "", false
	}
	if !strings.HasSuffix(base, nameSuffix) {
		return "", false
	}
	dir := path.Dir(clean)
	for _, candidate := range categories {
		if dir == candidate.Dir {
			return candidate.Name, true
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

// All answers every current spec, relative to root, slash-separated and sorted.
// Missing empty buckets are valid because Git tracks no empty directories.
// An absent plan, an unreadable bucket, an obsolete placement or a duplicated
// identity is an error, never an apparently empty or partial inventory.
func All(root string) ([]string, error) {
	info, err := os.Stat(filepath.Join(root, Root))
	if err != nil {
		return nil, fmt.Errorf("read the spec population under %s: %w", Root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("read the spec population under %s: %s is not a directory", Root, Root)
	}

	for _, category := range historicalBuckets {
		if err := refuseObsolete(root, category.Dir); err != nil {
			return nil, err
		}
	}
	if err := refuseObsolete(root, historicalProposal.Dir); err != nil {
		return nil, err
	}
	var specs []string
	identities := make(map[string]string)
	for _, category := range buckets {
		entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(category.Dir)))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read spec bucket %s: %w", category.Dir, err)
		}
		for _, entry := range entries {
			relative := category.Dir + "/" + entry.Name()
			if !IsSpec(relative) {
				continue
			}
			if previous, exists := identities[entry.Name()]; exists {
				return nil, fmt.Errorf("spec %s is in two buckets: %s, %s", entry.Name(), previous, relative)
			}
			identities[entry.Name()] = relative
			specs = append(specs, relative)
		}
	}
	slices.Sort(specs)
	return specs, nil
}

func refuseObsolete(root, directory string) error {
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(directory)))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect obsolete spec location %s: %w", directory, err)
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), namePrefix) {
			continue
		}
		if !strings.HasSuffix(entry.Name(), nameSuffix) {
			continue
		}
		return fmt.Errorf("obsolete spec placement %s/%s: move the work item into a declared category under %s", directory, entry.Name(), strings.Join(Dirs(), ", "))
	}
	return nil
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

	var found []string
	for _, directory := range HistoricalDirs() {
		relative := directory + "/" + base
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative)))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("find %s: %w", relative, err)
		}
		if !IsSpec(relative) {
			return "", fmt.Errorf("obsolete spec placement %s: move it into a declared current category", relative)
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("find %s: not a regular spec file", relative)
		}
		found = append(found, relative)
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
