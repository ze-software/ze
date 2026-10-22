// Design: docs/contributing/spec-workflow.md -- committed release roadmap.
// Package roadmap reports the spec population of one immutable Git tree.
package roadmap

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"

	specpath "github.com/ze-software/ze/internal/le/spec/path"
	specstatus "github.com/ze-software/ze/internal/le/spec/status"
)

// Qualification states the release boundary without claiming readiness from counts.
const Qualification = "Release fixes, validation and delivery are required; post-release work and unapproved proposals are not release commitments. Dependencies take precedence over priority."

// Named failures distinguish an unreadable population from an empty one.
var (
	ErrRevision  = errors.New("roadmap revision unavailable")
	ErrPlan      = errors.New("roadmap plan tree missing")
	ErrRead      = errors.New("roadmap tree read failed")
	ErrDuplicate = errors.New("roadmap duplicate spec identity")
)

// Item contains only the metadata published by the roadmap, never a spec body.
type Item struct {
	Name          string   `json:"name"`
	Title         string   `json:"title"`
	Path          string   `json:"path"`
	Bucket        string   `json:"bucket"`
	Status        string   `json:"status"`
	Priority      string   `json:"priority"`
	ReleaseReason string   `json:"release-reason"`
	Depends       string   `json:"depends"`
	Phase         string   `json:"phase"`
	Set           string   `json:"set"`
	Updated       string   `json:"updated"`
	SourceURL     string   `json:"source-url"`
	Diagnostics   []string `json:"diagnostics"`
}

// Counts count files, including every malformed and parked item.
type Counts struct {
	Total      int            `json:"total"`
	Required   int            `json:"required"`
	NiceToHave int            `json:"nice-to-have"`
	Buckets    map[string]int `json:"buckets"`
	Statuses   map[string]int `json:"statuses"`
}

// Snapshot is the reproducible input shared by the repository, site, and news.
type Snapshot struct {
	SchemaVersion int                 `json:"schema-version"`
	Revision      string              `json:"revision"`
	InputDigest   string              `json:"input-digest"`
	Qualification string              `json:"qualification"`
	Measure       string              `json:"measure"`
	Counts        Counts              `json:"counts"`
	Items         []Item              `json:"items"`
	Categories    []specpath.Category `json:"categories"`
}

// Collect resolves revision once, then reads only objects reachable from that commit.
// An empty revision explicitly selects committed HEAD. No working-tree files are read.
func Collect(ctx context.Context, root, revision string) (Snapshot, error) {
	resolved, err := resolve(ctx, root, revision)
	if err != nil {
		return Snapshot{}, err
	}
	entries, err := tree(ctx, root, resolved)
	if err != nil {
		return Snapshot{}, err
	}
	blobs, err := readBlobs(ctx, root, entries)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot := Snapshot{
		SchemaVersion: 2, Revision: resolved, Qualification: Qualification,
		Measure: "spec work items; counts do not measure effort or release readiness",
		Items:   make([]Item, 0, len(entries)),
		Counts:  Counts{Buckets: make(map[string]int), Statuses: make(map[string]int)},
	}
	snapshot.Categories = snapshotCategories(entries)
	for _, category := range snapshot.Categories {
		if category.Dir == specpath.Root {
			snapshot.Qualification = "Historical inventory: original directory classifications are preserved, not mapped to current queues. Counts do not establish release readiness."
			break
		}
	}
	order := make(map[string]int, len(snapshot.Categories))
	required := make(map[string]bool, len(snapshot.Categories))
	for index, category := range snapshot.Categories {
		snapshot.Counts.Buckets[category.Name] = 0
		order[category.Name] = index
		required[category.Name] = category.Required
	}
	identities := make(map[string]string, len(entries))
	digest := sha256.New()
	for index, entry := range entries {
		parsed, err := specstatus.ParseHistorical(blobs[index], entry.path, nil)
		if err != nil {
			return Snapshot{}, fmt.Errorf("%w: %s: %w", ErrRead, entry.path, err)
		}
		if previous, exists := identities[parsed.Name]; exists {
			return Snapshot{}, fmt.Errorf("%w: %s and %s", ErrDuplicate, previous, entry.path)
		}
		identities[parsed.Name] = entry.path
		// Hash writes cannot fail. Length framing prevents path/content ambiguity.
		fmt.Fprintf(digest, "%d:%s%d:", len(entry.path), entry.path, len(blobs[index])) //nolint:errcheck // hash.Hash writes cannot fail.
		digest.Write(blobs[index])                                                      //nolint:errcheck // hash.Hash writes cannot fail.
		item := Item{
			Name: parsed.Name, Title: parsed.Title, Path: parsed.Path, Bucket: parsed.Bucket,
			Status: parsed.Status, Depends: parsed.Depends, Phase: parsed.Phase,
			Priority: parsed.Priority, ReleaseReason: parsed.ReleaseReason,
			Set: parsed.Set, Updated: parsed.Updated, SourceURL: sourceURL(resolved, entry.path),
			Diagnostics: make([]string, 0),
		}
		itemDiagnostics(&item)
		snapshot.Items = append(snapshot.Items, item)
		snapshot.Counts.Total++
		snapshot.Counts.Buckets[item.Bucket]++
		snapshot.Counts.Statuses[item.Status]++
		if required[item.Bucket] {
			snapshot.Counts.Required++
		} else {
			snapshot.Counts.NiceToHave++
		}
	}
	snapshot.InputDigest = hex.EncodeToString(digest.Sum(nil))
	slices.SortFunc(snapshot.Items, func(left, right Item) int {
		if result := cmp.Compare(priorityOrder(left.Priority), priorityOrder(right.Priority)); result != 0 {
			return result
		}
		if result := cmp.Compare(order[left.Bucket], order[right.Bucket]); result != 0 {
			return result
		}
		if result := cmp.Compare(specstatus.StatusOrder(left.Status), specstatus.StatusOrder(right.Status)); result != 0 {
			return result
		}
		return cmp.Compare(left.Name, right.Name)
	})
	return snapshot, nil
}

// A retired layout keeps its actual category names and obligations. A tree with
// both layouts reports both rather than silently relabeling old work.
func snapshotCategories(entries []treeEntry) []specpath.Category {
	current, historical := false, false
	for _, entry := range entries {
		if specpath.IsSpec(entry.path) {
			current = true
		} else {
			historical = true
		}
	}
	if !historical {
		return specpath.Categories()
	}
	categories := specpath.HistoricalCategories()
	if current {
		categories = append(categories, specpath.Categories()...)
	}
	return categories
}

func priorityOrder(priority string) int {
	switch priority {
	case "P0":
		return 0
	case "P1":
		return 1
	case "P2":
		return 2
	case "P3":
		return 3
	default:
		return 4
	}
}

func itemDiagnostics(item *Item) {
	// The parse markers are checked before the vocabulary, because Parse writes
	// them in place of a Status the spec failed to declare.
	switch {
	case item.Status == specstatus.StatusUnparsed:
		item.Diagnostics = append(item.Diagnostics, "metadata table unavailable")
	case item.Status == specstatus.StatusUnknown:
		item.Diagnostics = append(item.Diagnostics, "status unavailable")
	case !specstatus.Declared(item.Status):
		item.Diagnostics = append(item.Diagnostics, "unrecognized declared status: "+item.Status)
	}
	if item.Title == "" {
		item.Diagnostics = append(item.Diagnostics, "title unavailable")
	}
	if _, err := time.Parse(time.DateOnly, item.Updated); err != nil {
		item.Diagnostics = append(item.Diagnostics, "updated date unavailable or invalid")
	}
	// The new metadata contract does not rewrite what old commits were required
	// to declare. Current-layout files keep missing or invalid values visible.
	if specpath.IsSpec(item.Path) {
		if priorityOrder(item.Priority) == 4 {
			item.Diagnostics = append(item.Diagnostics, "priority unavailable or invalid")
		}
		if strings.TrimSpace(item.ReleaseReason) == "" {
			item.Diagnostics = append(item.Diagnostics, "release reason unavailable")
		}
	}
}

func resolve(ctx context.Context, root, revision string) (string, error) {
	if revision == "" {
		revision = "HEAD"
	}
	if strings.HasPrefix(revision, "-") {
		return "", fmt.Errorf("%w: invalid reference %q", ErrRevision, revision)
	}
	output, err := git(ctx, root, nil, "rev-parse", "--verify", "--end-of-options", revision+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("%w: %q; fetch the requested history: %w", ErrRevision, revision, err)
	}
	return strings.TrimSpace(string(output)), nil
}

type treeEntry struct {
	path   string
	object string
}

func tree(ctx context.Context, root, revision string) ([]treeEntry, error) {
	output, err := git(ctx, root, nil, "ls-tree", "-r", "-t", "-z", "--full-tree", revision, "--", specpath.Root)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrRead, revision, err)
	}
	entries := make([]treeEntry, 0)
	planFound := false
	for record := range bytes.SplitSeq(output, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		header, name, ok := bytes.Cut(record, []byte{'\t'})
		if !ok {
			return nil, fmt.Errorf("%w: malformed tree entry at %s", ErrRead, revision)
		}
		fields := strings.Fields(string(header))
		if len(fields) != 3 {
			return nil, fmt.Errorf("%w: malformed tree header at %s", ErrRead, revision)
		}
		rel := string(name)
		if rel == specpath.Root {
			planFound = fields[1] == "tree"
		}
		bucket, recognized := specpath.HistoricalBucket(rel)
		if !recognized {
			continue
		}
		// Old proposals were not part of the historical roadmap population.
		if bucket == "to-review" {
			continue
		}
		if fields[1] != "blob" {
			return nil, fmt.Errorf("%w: %s is not a regular spec blob", ErrRead, rel)
		}
		if fields[0] != "100644" {
			if fields[0] != "100755" {
				return nil, fmt.Errorf("%w: %s has unsupported mode %s", ErrRead, rel, fields[0])
			}
		}
		entries = append(entries, treeEntry{path: rel, object: fields[2]})
	}
	if !planFound {
		return nil, fmt.Errorf("%w: %s has no plan/ directory", ErrPlan, revision)
	}
	// Explicit path ordering also fixes the digest order independently of Git output.
	slices.SortFunc(entries, func(left, right treeEntry) int { return cmp.Compare(left.path, right.path) })
	return entries, nil
}

func readBlobs(ctx context.Context, root string, entries []treeEntry) ([][]byte, error) {
	blobs := make([][]byte, 0, len(entries))
	if len(entries) == 0 {
		return blobs, nil
	}
	var request strings.Builder
	for _, entry := range entries {
		request.WriteString(entry.object)
		request.WriteByte('\n')
	}
	output, err := git(ctx, root, strings.NewReader(request.String()), "cat-file", "--batch")
	if err != nil {
		return nil, fmt.Errorf("%w: spec blobs: %w", ErrRead, err)
	}
	for _, entry := range entries {
		header, rest, found := bytes.Cut(output, []byte{'\n'})
		if !found {
			return nil, fmt.Errorf("%w: missing blob header for %s", ErrRead, entry.path)
		}
		fields := strings.Fields(string(header))
		if len(fields) != 3 {
			return nil, fmt.Errorf("%w: unavailable blob for %s", ErrRead, entry.path)
		}
		if fields[0] != entry.object {
			return nil, fmt.Errorf("%w: wrong blob for %s", ErrRead, entry.path)
		}
		if fields[1] != "blob" {
			return nil, fmt.Errorf("%w: wrong object type for %s", ErrRead, entry.path)
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, fmt.Errorf("%w: invalid blob length for %s: %w", ErrRead, entry.path, err)
		}
		if size < 0 {
			return nil, fmt.Errorf("%w: negative blob length for %s", ErrRead, entry.path)
		}
		if size >= len(rest) {
			return nil, fmt.Errorf("%w: truncated blob for %s", ErrRead, entry.path)
		}
		if rest[size] != '\n' {
			return nil, fmt.Errorf("%w: invalid blob terminator for %s", ErrRead, entry.path)
		}
		blobs = append(blobs, rest[:size])
		output = rest[size+1:]
	}
	if len(output) != 0 {
		return nil, fmt.Errorf("%w: unexpected trailing blob output", ErrRead)
	}
	return blobs, nil
}

func git(ctx context.Context, root string, input *strings.Reader, args ...string) ([]byte, error) {
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	command := exec.CommandContext(bounded, "git", append([]string{"--no-replace-objects", "-C", root}, args...)...) //nolint:gosec // Internal Git subcommands use argv, not a shell; revision arguments are validated.
	if input != nil {
		command.Stdin = input
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return output, nil
}

func sourceURL(revision, rel string) string {
	return "https://github.com/ze-software/ze/blob/" + revision + "/" + escapedPath(rel)
}

func escapedPath(rel string) string {
	parts := strings.Split(rel, "/")
	for index := range parts {
		parts[index] = url.PathEscape(parts[index])
	}
	return strings.Join(parts, "/")
}
