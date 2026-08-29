// Design: docs/architecture/core-design.md -- the published repository facts, as one command
//
// Package sitefacts derives the numbers the website publishes ABOUT this
// repository, and writes them into one committed file that the site build
// reads. The build never walks this tree for them.
//
// A published figure is a claim about the repository. A figure derived while a
// site build runs is a claim about whatever happened to be on disk at that
// moment, in a checkout several sessions share, and the two are impossible to
// tell apart in the output. Deriving them here, once, from the tree a person is
// about to commit, is what makes the claim checkable: the file lands in the
// same commit as whatever moved the count.
//
// The precedent is the native test-health producer, which writes
// test/health/latest.json. internal/le/site reads that committed inventory
// instead of recounting it, and this package gives the rest of the published
// repository facts the same shape.
//
// Detail: actions.go -- the command surface. register.go -- the registration.
package sitefacts

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// factsFile is the committed file, relative to the checkout root. It sits
// beside the site's other committed inputs -- website/data/features.json,
// website/data/dependencies.json -- because that is what it is: data the site
// build reads and never writes.
const factsFile = "website/data/repo-facts.json"

// gitTimeout bounds a git run. `git ls-files` and `git status` both answer from
// the index and fork nothing, in well under a second on this checkout, so a run
// past this bound is a git that has stopped rather than one that is slow.
const gitTimeout = 30 * time.Second

// goListTimeout bounds the `go list` run. It loads the package graph of the
// whole module, which is seconds against a warm build cache and minutes against
// a cold one. The retired site producer allowed 60 seconds and published a zero
// when it ran out; this leaves headroom for the cold case and answers an error
// rather than a zero.
const goListTimeout = 2 * time.Minute

// outputMax bounds what the derivation reads back from a command, and with it
// every loop below: the length of an answer is the only thing that can make one
// long. Measured on this checkout, `git ls-files -z -- '*.go'` answers 493 KB
// for 10,562 paths and `go list -json` answers 222 KB for 700 packages, so this
// leaves an order of magnitude of headroom and still refuses a command that
// streams forever.
const outputMax = 8 << 20

// categoryCommitted marks a fact that is a pure function of committed state, so
// a person can regenerate it and a gate can compare it. Every fact records its
// category because they do not all share one: a star count fetched from GitHub
// can never be committed truth, and a reader of the file has to be able to tell
// which kind of claim each number is.
const categoryCommitted = "committed-data"

// designMarker and detailMarkers are the file-header annotations
// docs/contributing/ze-go-style.md asks every Go file to carry. Counting them
// is counting how much of the tree explains itself.
var (
	designMarker  = []byte("// Design:")
	detailMarkers = [][]byte{[]byte("// Detail:"), []byte("// Overview:"), []byte("// Related:")}
)

// annotationSource says where a reader can re-derive the two annotation counts
// by hand, which is what makes the number checkable rather than merely stated.
const annotationSource = "git ls-files '*.go', counting the file-header annotations"

// goPackagesSource says the same for the package count. Both halves are named,
// because either one alone answers a different question: `go list` alone counts
// a package no commit holds, and git alone cannot tell a package from a
// directory that happens to hold a Go file.
const goPackagesSource = "go list ./..., keeping the packages git holds a file for"

// interopTargetsSource and interopScenariosSource say the same for the two
// interop figures. Both count what GIT holds rather than what test/interop/
// contains, and that is the whole change: a scenario directory another session
// is part-way through writing sits on this disk and in no commit, and it moved
// a published number every time a site build walked that directory.
const (
	interopTargetsSource   = "git ls-files test/interop/Dockerfile.*, less Dockerfile.ze, plus the FRR image a run names by variable"
	interopScenariosSource = "git ls-files test/interop/scenarios/, counting the directories git holds a file under"
)

// The paths the two interop counts read, and the two names they treat apart.
// Dockerfile.ze builds ze itself rather than a peer to test against, so it is
// not one of the implementations the figure claims.
const (
	interopRoot      = "test/interop"
	scenarioRoot     = "test/interop/scenarios"
	dockerfilePrefix = "Dockerfile."
	zeDockerfile     = "Dockerfile.ze"
)

// frrTarget is the one interop peer with no Dockerfile of its own: a run names
// the FRR image by variable (FRR_IMAGE), so counting Dockerfiles alone answers
// one short of the implementations ze is tested against.
const frrTarget = 1

// categoryBuiltBinary marks a published fact about this repository that this
// tool cannot commit, because deriving it runs the built ze rather than reading
// what git holds. internal/le/site owns that live derivation.
const categoryBuiltBinary = "built-binary"

// fact is one published number about this repository: what it is a claim about,
// what it is, and where it came from.
type fact struct {
	Category string `json:"category"`
	Value    int    `json:"value"`
	Source   string `json:"source"`
}

// live is a published fact about this repository that is NOT committed data.
// It carries no value, deliberately: a number here would be a claim about a
// binary somebody built, and a binary is not what a commit holds. What it
// carries is the category and the producer, so a reader of this file can tell
// which kind of claim each published number is and none of them looks committed
// by sitting in a committed file.
type live struct {
	Category string `json:"category"`
	Source   string `json:"source"`
}

// facts is the whole committed file. The facts sit under a key of their own so
// that a fact this tool cannot commit -- one derived from a running binary or
// from the network -- can be recorded beside them rather than among them.
//
// A name here is the fact's path in the site's own published site-facts.json,
// so nothing is renamed on the way through. internal/le/site reads
// `repo.design_comments` and renders it from the token specification with the
// same name.
type facts struct {
	Facts map[string]fact `json:"facts"`
	Live  map[string]live `json:"live"`
}

// annotationCounts is what one walk of the tracked Go files answers.
type annotationCounts struct {
	design int
	detail int
}

// interopCounts is what one walk of the tracked interop paths answers. The
// scenario count is published two ways, as the directories a reader sees and as
// every directory there is, so both are derived from one walk.
type interopCounts struct {
	targets      int
	scenarios    int
	scenarioDirs int
}

// goPackage is the part of a `go list -json` record this tool reads: where the
// package is, and where the module holding it is.
type goPackage struct {
	Dir    string
	Module struct {
		Dir string
	}
}

// change is one Go file the working tree and the last commit disagree about.
// The status is git's own two-character spelling, so `??` is a file no commit
// holds and ` M` is one this tree has edited.
type change struct {
	Status string `json:"status"`
	Path   string `json:"path"`
}

// statusPrefix is what a `git status --porcelain` record spends before the
// path: two status characters and the space after them.
const statusPrefix = 3

// derive answers every fact this tool publishes about the checkout at root.
//
// One derivation, called by the action that WRITES the file and by the action
// that CHECKS it, because two derivations over one tree drift by construction:
// the retired site producer and repository producer once disagreed by 30 tests
// when both counted for themselves.
func derive(root string) (facts, error) {
	tracked, err := trackedPaths(root, "*.go")
	if err != nil {
		return facts{}, err
	}

	counts, err := countAnnotations(root, tracked)
	if err != nil {
		return facts{}, err
	}

	packages, err := countGoPackages(root, tracked)
	if err != nil {
		return facts{}, err
	}

	interopPaths, err := trackedPaths(root, interopRoot)
	if err != nil {
		return facts{}, err
	}
	interop := countInterop(interopPaths)

	return facts{Live: liveFacts(), Facts: map[string]fact{
		"repo.design_comments": {
			Category: categoryCommitted,
			Value:    counts.design,
			Source:   annotationSource,
		},
		"repo.detail_comments": {
			Category: categoryCommitted,
			Value:    counts.detail,
			Source:   annotationSource,
		},
		"repo.go_packages": {
			Category: categoryCommitted,
			Value:    packages,
			Source:   goPackagesSource,
		},
		"interop.targets": {
			Category: categoryCommitted,
			Value:    interop.targets,
			Source:   interopTargetsSource,
		},
		"interop.scenarios": {
			Category: categoryCommitted,
			Value:    interop.scenarios,
			Source:   interopScenariosSource,
		},
		"interop.scenario_dirs_raw": {
			Category: categoryCommitted,
			Value:    interop.scenarioDirs,
			Source:   interopScenariosSource,
		},
	}}, nil
}

// liveFacts is every published fact ABOUT this repository that the site derives
// while it builds, and that this tool does not derive at all.
//
// Both run the built ze: internal/le/site asks it for the command surface
// and configuration tree. That makes each one a claim about a binary rather
// than about a commit, so it cannot be regenerated from a tree and cannot be
// gated for staleness the way the facts above can.
//
// They are recorded here anyway, without values. The alternative is a file that
// holds five of the seven published figures and says nothing about the other
// two, which leaves a reader to work out for themselves whether a number they
// cannot find here is uncommitted or forgotten.
func liveFacts() map[string]live {
	return map[string]live{
		"cli_commands": {
			Category: categoryBuiltBinary,
			Source:   "ze help command --json, run against the ze the site build found",
		},
		"config_sections": {
			Category: categoryBuiltBinary,
			Source:   "ze yang tree --json --config, run against the ze the site build found",
		},
	}
}

// countInterop counts the interop peers and the interop scenarios git holds,
// from the tracked paths under test/interop/.
//
// A checkout that holds no interop tree at all answers zero for all three,
// rather than answering the one peer that has no Dockerfile: the figure is a
// claim about a suite this repository does not carry.
func countInterop(paths []string) interopCounts {
	if len(paths) == 0 {
		return interopCounts{}
	}

	var counts interopCounts
	dirs := make(map[string]struct{}, len(paths))
	for _, rel := range paths {
		if dir, base := path.Split(rel); underDir(strings.TrimSuffix(dir, "/"), interopRoot) && strings.TrimSuffix(dir, "/") == interopRoot {
			if strings.HasPrefix(base, dockerfilePrefix) && base != zeDockerfile {
				counts.targets++
			}
			continue
		}
		if name, ok := scenarioDir(rel); ok {
			dirs[name] = struct{}{}
		}
	}
	counts.targets += frrTarget

	for name := range dirs {
		counts.scenarioDirs++
		if !strings.HasPrefix(name, ".") {
			counts.scenarios++
		}
	}
	return counts
}

// scenarioDir answers the scenario a tracked path belongs to, and whether it
// belongs to one at all.
//
// A scenario is a DIRECTORY, so a file sitting directly in scenarios/ names
// none: the path has to carry a separator after the name for the name to be a
// directory git holds something under.
func scenarioDir(rel string) (string, bool) {
	rest, ok := strings.CutPrefix(rel, scenarioRoot)
	if ok {
		rest, ok = strings.CutPrefix(rest, "/")
	}
	if !ok {
		return "", false
	}
	name, _, ok := strings.Cut(rest, "/")
	if !ok {
		return "", false
	}
	return name, true
}

// countAnnotations counts the file-header annotations across the tracked Go
// files named by paths, which are relative to the checkout at root.
//
// The population is what git holds, so a file no commit carries cannot move a
// published number. The content is what the working tree holds, which is the
// deliberate part: a person runs this in the tree they are about to commit, and
// the file they write records that tree.
func countAnnotations(root string, paths []string) (annotationCounts, error) {
	var counts annotationCounts
	for _, rel := range paths {
		text, err := os.ReadFile(filepath.Join(root, rel)) //nolint:gosec // rel is a tracked path of this checkout, named by git ls-files
		if errors.Is(err, fs.ErrNotExist) {
			// git answers from the index, so a file deleted in the working
			// tree and not yet committed is still named here. It is not an
			// error, and it contributes nothing.
			continue
		}
		if err != nil {
			return annotationCounts{}, fmt.Errorf("sitefacts: read %s: %w", rel, err)
		}

		counts.design += bytes.Count(text, designMarker)
		for _, marker := range detailMarkers {
			counts.detail += bytes.Count(text, marker)
		}
	}

	return counts, nil
}

// trackedPaths answers every tracked path of the checkout at root that matches
// one of these pathspecs, relative to it.
//
// One fork for a whole population rather than one for each file, and NUL
// separation rather than newlines, because a path may hold a newline and a
// split on one would then count a fragment of it as a file.
func trackedPaths(root string, patterns ...string) ([]string, error) {
	args := append([]string{"ls-files", "-z", "--"}, patterns...)
	raw, err := output(root, gitTimeout, "git", args...)
	if err != nil {
		return nil, fmt.Errorf("sitefacts: read the tracked %v of %s: %w", patterns, root, err)
	}
	return records(raw), nil
}

// countGoPackages counts the Go packages of the checkout at root that git holds
// a file for. The tracked paths are the ones trackedGoFiles named.
//
// Two questions, and neither half answers both. What is a package is a question
// for the toolchain: it applies build constraints, skips a nested module and a
// vendor tree, and uses the same `go list ./...` population the native site
// builder consumes.
// What the repository holds is a question for git: a directory whose Go files
// are all untracked is a package on this disk and in no commit, so it must not
// move a published number. The count is the packages that pass both.
//
// A directory git holds and the working tree has emptied passes neither, so it
// counts for nothing here. That is the same reading countAnnotations takes of a
// deleted file, and it is why a person runs this in the tree they are about to
// commit rather than in one they are part-way through.
func countGoPackages(root string, tracked []string) (int, error) {
	held := make(map[string]struct{}, len(tracked))
	for _, rel := range tracked {
		held[path.Dir(rel)] = struct{}{}
	}

	listed, err := goPackageDirs(root)
	if err != nil {
		return 0, err
	}

	packages := 0
	for _, dir := range listed {
		if _, ok := held[dir]; ok {
			packages++
		}
	}
	return packages, nil
}

// goPackageDirs answers the directory of every package of the module at root,
// relative to the module root and separated by slashes, as `go list` reports
// them.
//
// The module root comes from `go list` as well, rather than from the root this
// tool was given, because the two spell one directory differently the moment a
// symlink stands between them: a temporary directory on macOS is reached as
// /var and reported as /private/var. Both paths in one answer cannot disagree.
func goPackageDirs(root string) ([]string, error) {
	raw, err := output(root, goListTimeout, "go", "list", "-json=Dir,Module", "./...")
	if err != nil {
		return nil, fmt.Errorf("sitefacts: list the Go packages of %s: %w", root, err)
	}

	// `go list -json` writes one object after another rather than an array, so
	// the decoder reads them as a stream and stops at the end of the input.
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var dirs []string
	for decoder.More() {
		var listed goPackage
		if err := decoder.Decode(&listed); err != nil {
			return nil, fmt.Errorf("sitefacts: read what go list answered for %s: %w", root, err)
		}
		if listed.Module.Dir == "" {
			return nil, fmt.Errorf("sitefacts: go list reported package %s outside a module", listed.Dir)
		}

		rel, err := filepath.Rel(listed.Module.Dir, listed.Dir)
		if err != nil {
			return nil, fmt.Errorf("sitefacts: place package %s under %s: %w", listed.Dir, listed.Module.Dir, err)
		}
		dirs = append(dirs, filepath.ToSlash(rel))
	}
	return dirs, nil
}

// uncommittedGoFiles answers every Go file the working tree at root and the
// last commit disagree about, modified, staged, deleted and untracked alike.
//
// This is what the update warns about before it writes. The facts are derived
// from the tree the person is standing in, and this checkout is shared by
// several sessions at once, so a regeneration can record work that belongs to
// somebody else and nothing in the file it writes would say so
// (plan/journal/concurrent-session-corruption.md).
//
// The pathspec is the population the facts derive from. A fact derived from
// another kind of file adds its pattern here, or the warning goes quiet about
// exactly the change that moved the number.
func uncommittedGoFiles(root string) ([]change, error) {
	raw, err := output(root, gitTimeout, "git", "status", "--porcelain", "-z", "--", "*.go")
	if err != nil {
		return nil, fmt.Errorf("sitefacts: read the working tree state of %s: %w", root, err)
	}

	lines := records(raw)
	changes := make([]change, 0, len(lines))
	for index := 0; index < len(lines); index++ {
		line := lines[index]
		if len(line) <= statusPrefix {
			return nil, fmt.Errorf("sitefacts: git status answered %q, which is too short to carry a status and a path", line)
		}

		status := line[:2]
		changes = append(changes, change{Status: status, Path: line[statusPrefix:]})

		// A rename and a copy spend two records: the path above, then the path
		// it came from. The second is not a change of its own.
		if strings.ContainsAny(status, "RC") {
			index++
		}
	}
	return changes, nil
}

// records splits what a NUL-separated git answer holds, dropping the empty
// record its trailing separator leaves. NUL separation rather than newlines,
// because a path may hold a newline and a split on one would then read a
// fragment of it as a path of its own.
func records(raw []byte) []string {
	answers := make([]string, 0, bytes.Count(raw, []byte{0}))
	for record := range strings.SplitSeq(string(raw), "\x00") {
		if record != "" {
			answers = append(answers, record)
		}
	}
	return answers
}

// output runs one command in the checkout at root and answers what it wrote on
// stdout. Both bounds are stated: a command that has stopped never returns, and
// a command that streams forever fills memory
// (docs/contributing/ze-go-style.md, "A limit on everything").
func output(root string, limit time.Duration, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()

	// name and args are literals of this package, and root is the checkout
	// lepath.Root() discovered or a fixture a test named. Nothing here is
	// reachable from a network peer: le is a build-host tool.
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // the command is a literal of this package; root is the checkout, resolved by lepath.Root
	cmd.Dir = root
	cmd.Stderr = os.Stderr

	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("read what %s answered: %w", name, err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("run %s: %w", name, err)
	}

	raw, readErr := io.ReadAll(io.LimitReader(pipe, outputMax+1))
	waitErr := cmd.Wait()
	switch {
	case readErr != nil:
		return nil, fmt.Errorf("read what %s answered: %w", name, readErr)
	case waitErr != nil:
		return nil, fmt.Errorf("run %s: %w", name, waitErr)
	case len(raw) > outputMax:
		return nil, fmt.Errorf("%s answered more than %d bytes", name, outputMax)
	}
	return raw, nil
}

// write replaces the committed file with these facts and answers the path it
// wrote. Two runs over one commit write the same bytes: every value is derived
// from the tree, the encoder sorts the fact names, and nothing here reads a
// clock.
func write(root string, derived facts) (string, error) {
	raw, err := json.MarshalIndent(derived, "", "  ")
	if err != nil {
		return "", fmt.Errorf("sitefacts: render %s: %w", factsFile, err)
	}
	raw = append(raw, '\n')

	path := filepath.Join(root, factsFile)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return "", fmt.Errorf("sitefacts: write %s: %w", path, err)
	}
	return path, nil
}

// underDir reports whether rel names dir itself or something inside it.
//
// Written out rather than `strings.HasPrefix(rel, dir+"/")` for two reasons.
// c_string_concat (.claude/hooks/pretool-writeedit.py) refuses a `+` beside a
// string literal in any compiled Go file, and `performance.md` is the rule
// behind it: the concatenation allocates a new string on every call to answer a
// question about the one already in hand. This allocates nothing.
func underDir(rel, dir string) bool {
	if rel == dir {
		return true
	}
	return len(rel) > len(dir) && rel[:len(dir)] == dir && rel[len(dir)] == '/'
}
