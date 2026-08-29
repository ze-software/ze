// Design: website/AI.md -- a build stages sources into an isolated Pages artifact
package site

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/docvalid"
)

const sourceListTimeout = 30 * time.Second

// BuildOptions controls one deterministic site build.
type BuildOptions struct {
	Repository string
	Output     string
	Partial    bool
}

// BuildReport identifies the inputs and artifact produced by Build.
//
// Unstamped names every public page that carries no footer for the publication
// stamp to live in. A build reports them rather than failing, because a page
// without a footer is a page defect and not a broken build.
//
// Coverage names every published route no producer wrote and every route two
// producers wrote. A build reports them rather than failing, for the same
// reason: the artifact it produced is the evidence, and refusing to produce it
// would remove the evidence.
type BuildReport struct {
	SourceDigest string   `json:"source-digest"`
	Output       string   `json:"output"`
	Files        int      `json:"files"`
	Published    string   `json:"published"`
	Carried      int      `json:"carried-stamps"`
	Unstamped    []string `json:"unstamped,omitempty"`
	Coverage     Coverage `json:"coverage"`
}

// Build stages the website source tree, preserves the last complete artifact as
// an incremental seed, refreshes native generated surfaces, runs every
// registered page producer, removes every source-only input from deployment,
// and stamps the publication time into the footer of every published page.
func Build(options BuildOptions) (BuildReport, error) {
	paths, err := resolvePaths(options.Repository, options.Output)
	if err != nil {
		return BuildReport{}, err
	}
	files, err := trackedAndUntrackedSourceFiles(paths)
	if err != nil {
		return BuildReport{}, err
	}
	digest, err := sourceDigest(paths.Source, files)
	if err != nil {
		return BuildReport{}, err
	}

	// The artifact as it was last published both seeds this build and answers
	// which pages the build left alone. Take it before anything writes into the
	// output, because seeding cleans the output first.
	previous, releasePrevious, err := snapshotArtifact(paths)
	if err != nil {
		return BuildReport{}, err
	}
	defer releasePrevious()

	if options.Partial {
		if info, statErr := os.Stat(paths.Output); statErr != nil || !info.IsDir() {
			return BuildReport{}, fmt.Errorf("partial build requires an existing full artifact: %s", paths.Output)
		}
	} else if err := seedArtifact(paths, previous); err != nil {
		return BuildReport{}, err
	}
	if err := stageSources(paths.Source, paths.Output, files); err != nil {
		return BuildReport{}, err
	}
	if err := refreshNativeSurfaces(paths); err != nil {
		return BuildReport{}, err
	}

	// Every page a producer writes is written before the artifact is trimmed
	// and stamped, so a produced page is trimmed and stamped like any other.
	// The record of what they wrote goes to the checkout, never to the artifact,
	// because the artifact is published.
	claims, err := renderProducers(paths)
	if err != nil {
		return BuildReport{}, err
	}
	if err := writeProducerRecord(paths, claims); err != nil {
		return BuildReport{}, err
	}

	if err := removeSourceOnly(paths.Output); err != nil {
		return BuildReport{}, err
	}
	published := buildClock()
	unstamped, err := stampArtifact(paths.Output, published)
	if err != nil {
		return BuildReport{}, err
	}
	carried, err := carryPublicationStamps(previous, paths.Output)
	if err != nil {
		return BuildReport{}, err
	}

	// Coverage is read from the finished artifact, so a page the trimming
	// removed is not counted as published and a page a producer wrote is.
	coverage, err := coverageOf(paths.Output, claims)
	if err != nil {
		return BuildReport{}, err
	}

	return BuildReport{
		SourceDigest: digest,
		Output:       paths.Output,
		Files:        len(files),
		Published:    publishedDisplay(published),
		Carried:      carried,
		Unstamped:    unstamped,
		Coverage:     coverage,
	}, nil
}

func trackedAndUntrackedSourceFiles(paths Paths) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), sourceListTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, "git", "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	command.Dir = paths.Source
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("list website sources: %w", err)
	}
	var files []string
	for raw := range bytes.SplitSeq(output, []byte{0}) {
		if len(raw) == 0 {
			continue
		}
		name := filepath.Clean(string(raw))
		info, statErr := os.Lstat(filepath.Join(paths.Source, name))
		if statErr == nil && (info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0) {
			files = append(files, name)
		}
	}
	sort.Strings(files)
	return files, nil
}

// sourceDigest computes the build-boundary digest used to detect source races.
func sourceDigest(source string, files []string) (string, error) {
	hash := sha256.New()
	names := append([]string(nil), files...)
	sort.Strings(names)
	for _, name := range names {
		path := filepath.Join(source, name)
		info, err := os.Lstat(path)
		if err != nil {
			return "", err
		}
		hash.Write([]byte(filepath.ToSlash(name)))
		hash.Write([]byte{0})
		if info.Mode()&os.ModeSymlink != 0 {
			target, readErr := os.Readlink(path)
			if readErr != nil {
				return "", readErr
			}
			hash.Write([]byte(filepath.ToSlash(target)))
		} else {
			content, readErr := os.ReadFile(path) //nolint:gosec // a site build reads the checkout it was pointed at
			if readErr != nil {
				return "", readErr
			}
			hash.Write(content)
		}
		hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// publishable reports whether one artifact-relative path belongs in a
// deployment. Git metadata and every source-only input stay out.
func publishable(name string) bool {
	return name != gitMetadataDir && !strings.HasPrefix(name, ".git/") && !isSourceOnly(name)
}

// lastPublished names the directory holding the artifact as it was last
// published: the Pages checkout beside the repository, which a default build
// also writes into. An empty name means there is no previous artifact.
func lastPublished(paths Paths) (string, error) {
	published := filepath.Join(filepath.Dir(paths.Repository), "gh-pages")
	info, err := os.Stat(published)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("site artifact is not a directory: %s", published)
	}
	return published, nil
}

// snapshotArtifact answers a readable copy of the last published artifact and
// the function that releases it.
//
// The copy is needed only when the build writes over that artifact, because
// seeding cleans the output first. A build that writes elsewhere reads the
// published tree where it stands.
func snapshotArtifact(paths Paths) (string, func(), error) {
	release := func() {}
	published, err := lastPublished(paths)
	if err != nil || published == "" {
		return "", release, err
	}
	if filepath.Clean(published) != filepath.Clean(paths.Output) {
		return published, release, nil
	}
	snapshot, err := os.MkdirTemp(filepath.Dir(paths.Output), ".site-artifact-")
	if err != nil {
		return "", release, fmt.Errorf("create site artifact snapshot: %w", err)
	}
	// The snapshot is a scratch directory beside the artifact. A failed removal
	// leaves it behind and must not fail a build that otherwise succeeded.
	release = func() { _ = os.RemoveAll(snapshot) }
	if err := copyTree(published, snapshot, publishable); err != nil {
		release()
		return "", func() {}, fmt.Errorf("snapshot site artifact: %w", err)
	}
	return snapshot, release, nil
}

// seedArtifact empties the output and lays the last published artifact back
// down, so a build that changes three pages leaves the rest as they were.
func seedArtifact(paths Paths, previous string) error {
	if err := cleanArtifact(paths.Output); err != nil {
		return err
	}
	if previous == "" {
		return nil
	}
	return copyTree(previous, paths.Output, publishable)
}

func cleanArtifact(root string) error {
	if err := os.MkdirAll(root, 0o755); err != nil { //nolint:gosec // published web content: a web server, often another account, serves these bytes
		return err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == gitMetadataDir {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func stageSources(source, output string, files []string) error {
	for _, name := range files {
		if err := copyPath(filepath.Join(source, name), filepath.Join(output, name)); err != nil {
			return fmt.Errorf("stage %s: %w", name, err)
		}
	}
	return nil
}

func copyTree(source, target string, keep func(string) bool) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return os.MkdirAll(target, 0o755) //nolint:gosec // published web content: a web server, often another account, serves these bytes
		}
		name := filepath.ToSlash(relative)
		if !keep(name) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return os.MkdirAll(filepath.Join(target, relative), 0o755) //nolint:gosec // published web content: a web server, often another account, serves these bytes
		}
		return copyPath(path, filepath.Join(target, relative))
	})
}

func copyPath(source, target string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil { //nolint:gosec // published web content: a web server, often another account, serves these bytes
		return err
	}
	_ = os.RemoveAll(target)
	if info.Mode()&os.ModeSymlink != 0 {
		value, readErr := os.Readlink(source)
		if readErr != nil {
			return readErr
		}
		return os.Symlink(value, target)
	}
	content, err := os.ReadFile(source) //nolint:gosec // a site build reads the checkout it was pointed at
	if err != nil {
		return err
	}
	return os.WriteFile(target, content, info.Mode().Perm())
}

// refreshNativeSurfaces republishes the generated surfaces a build owns.
//
// The asset bundles are rendered on EVERY build, and the command PAGES are
// bootstrapped only when they are absent. The two guards look alike and the
// answers are opposite, so the reason is stated here rather than left for a
// reader to infer from the shape.
//
// renderCSS expands the @import chain and minifies a real stylesheet, and
// renderJS writes the authored script: each produces the artifact a reader
// downloads. Guarding them on absence meant a seeded bundle survived every
// build, so no edit under website/assets/css or website/assets/js ever reached
// a reader.
//
// refreshCommandSurfaces keeps its guard because its renderer emits a contract
// fixture rather than a page. Removing that one is what commit 9f45348a7 did,
// and it overwrote 396 published pages with fragments. The guard goes when a
// producer writes those pages, not before.
func refreshNativeSurfaces(paths Paths) error {
	if err := refreshCommandSurfaces(paths); err != nil {
		return err
	}
	if err := renderCSS(paths.Source, paths.Output); err != nil {
		return err
	}
	if err := renderJS(paths.Source, paths.Output); err != nil {
		return err
	}
	if _, err := refreshTalks(paths.Repository, filepath.Join(paths.Output, "talks")); err != nil {
		return err
	}
	return nil
}

// liveCommandCatalog answers the command catalog a build publishes. It is a
// variable so a test can state a catalog rather than build the daemon, which
// the real reader does with `go run ./cmd/ze`.
var liveCommandCatalog = docvalid.LiveCommandCatalog

// refreshCommandSurfaces republishes the command catalog, and bootstraps the
// pages derived from it only when they are absent.
//
// The catalog IS read from the binary on every build. It is derived data with
// no other producer since the interpreter cutover, so a build is the right
// place to write it.
//
// The PAGES are a different matter, and the asymmetry is deliberate.
// docvalid.RenderCommandSurfaces emits the contract FIXTURE that the
// documentation drift check compares a published page against: a bare doctype,
// a body, and one definition list. It carries no head, no title, no meta, no
// navigation, no stylesheet, and on a command-equivalents page none of the
// vendor equivalents the page exists for. Publishing it overwrites a 10KB page
// with 481 bytes. So it runs only to give a fresh checkout something rather
// than nothing, which is the role it has always had.
//
// The consequence is stated rather than hidden: the published pages have no Go
// producer. The Python renderer that wrote them was retired at the interpreter
// cutover and nothing replaced it, so the drift the checker reports against
// those pages is TRUE, and this function does not fix it. Making the check
// green by publishing the fixture is what commit 9f45348a7 did, and it cost
// 396 pages.
func refreshCommandSurfaces(paths Paths) error {
	raw, err := liveCommandCatalog(paths.Repository)
	if err != nil {
		return err
	}
	catalog := filepath.Join(paths.Output, "data", "cli-commands.json")
	if err := os.MkdirAll(filepath.Dir(catalog), 0o755); err != nil { //nolint:gosec // published web content: a web server, often another account, serves these bytes
		return err
	}
	if err := os.WriteFile(catalog, raw, 0o644); err != nil { //nolint:gosec // published web content: a web server, often another account, serves these bytes
		return fmt.Errorf("publish command catalog %s: %w", catalog, err)
	}

	primary := filepath.Join(paths.Output, "reference", "cli", "index.html")
	if _, err := os.Stat(primary); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return docvalid.RenderCommandSurfaces(paths.Output, raw)
}

func removeSourceOnly(root string) error {
	var removals []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root || strings.Contains(filepath.ToSlash(path), "/.git/") {
			return nil
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if isSourceOnly(relative) {
			removals = append(removals, path)
			if entry.IsDir() {
				return filepath.SkipDir
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Slice(removals, func(i, j int) bool { return len(removals[i]) > len(removals[j]) })
	for _, path := range removals {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return nil
}

// The output keyword this build accepts, and the repository metadata directory
// it preserves inside an artifact tree.
const (
	keywordOutput  = "output"
	keywordInput   = "input"
	gitMetadataDir = ".git"
)
