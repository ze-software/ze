// Design: docs/architecture/core-design.md -- the vendored-web tools, as commands
//
// Package vendorweb is the two halves of one contract. third_party/web/ is the
// source of truth for every vendored web asset, and each consumer package keeps
// its own copy because //go:embed cannot reach outside its own package. The
// sync writes those copies; the check gates them.
//
// Three commands come out of it, one per gate the Python le declares:
// vendor-web-sync writes, vendor-web-check compares, and
// vendor-web-update-report asks the npm registry what is newer. register.go
// wires all three.
//
// This file holds what the comparison reads: which directories under
// third_party/web/ are vendored packages, which directories under internal/ are
// consumers, and which consumer subscribes to which package. Both lists are
// DERIVED from the two trees. The sync writes from tables it declares itself
// (sync.go), and the asymmetry is deliberate: a package the sync was never told
// to copy is exactly what the check's UNSYNCED verdict exists to find.
package vendorweb

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// vendorDir is the source of truth, relative to the repository root.
const vendorDir = "third_party/web"

// consumerRoot is the tree walked for consumer asset directories.
const consumerRoot = "internal"

// consumerDirName is the directory name a consumer keeps its embedded assets
// in. Every row of MANIFEST.md's Consumers table uses it.
const consumerDirName = "assets"

// gitTimeout bounds the one subprocess this package runs. `git check-ignore`
// over a handful of paths answers in milliseconds, so a run past this is a git
// that has stopped rather than one that is slow.
const gitTimeout = 30 * time.Second

// ignoredReason is why a vendor-directory entry was passed over. It is the
// value half of CheckReport.Skipped, so a reader of `| json` gets the reason
// rather than a bare path.
const ignoredReason = "git ignores it, so it is not vendored content"

// vendorPackage is one directory under third_party/web/ and the file names it
// holds. The directory is the unit a consumer subscribes to, so an asset that
// belongs to one consumer alone gets its own directory (swagger-ui/ is that
// case). third_party/web/MANIFEST.md states the same contract.
type vendorPackage struct {
	dir   string   // directory name, e.g. "htmx"
	files []string // file names, sorted
}

// vendorPackages reads the source of truth. It fails when the tree holds no
// package, and when two packages hold one file name: a consumer copy is matched
// by name, so an ambiguous name has no single source to compare against.
//
// skipped names the entries it passed over, with the reason. The script this
// replaces printed those lines as it walked; they are recorded here so the
// answer carries them and the rendering stays the caller's choice.
func vendorPackages(root string) (pkgs []vendorPackage, skipped map[string]string, err error) {
	base := filepath.Join(root, vendorDir)
	skipped = map[string]string{}

	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, skipped, fmt.Errorf("read %s: %w", base, err)
	}

	owner := map[string]string{}

	ignored, err := ignoredNames(root, entries)
	if err != nil {
		return nil, skipped, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue // MANIFEST.md, and anything else at the top level
		}
		// An ignored directory is not a vendored package. Git ignore patterns
		// are unanchored, so a directory named `build` here can match a rule
		// written for another tree. A tracked package cannot match:
		// `git add` refuses an ignored path without -f, and check-ignore
		// consults the index. The skip is announced below so an unrelated rule
		// cannot silently remove a directory from the synchronization
		// population.
		if ignored[entry.Name()] {
			skipped[path.Join(vendorDir, entry.Name())] = ignoredReason
			continue
		}

		dir := filepath.Join(base, entry.Name())

		files, readErr := os.ReadDir(dir)
		if readErr != nil {
			return nil, skipped, fmt.Errorf("read %s: %w", dir, readErr)
		}

		pkg := vendorPackage{dir: entry.Name()}
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			if first, seen := owner[file.Name()]; seen {
				return nil, skipped, fmt.Errorf("%s is vendored twice, in %s/ and %s/; a consumer copy of that name has no single source", file.Name(), first, entry.Name())
			}
			owner[file.Name()] = entry.Name()
			pkg.files = append(pkg.files, file.Name())
		}

		if len(pkg.files) == 0 {
			continue
		}

		sort.Strings(pkg.files)
		pkgs = append(pkgs, pkg)
	}

	if len(pkgs) == 0 {
		return nil, skipped, fmt.Errorf("%s holds no vendored package, so nothing could be compared", base)
	}

	return pkgs, skipped, nil
}

// consumerDirs returns every consumer asset directory, relative to root.
//
// The list is DERIVED, not written down. A hand-written list is the failure
// this program exists to catch, one level up: a consumer added to the sync and
// forgotten here would hold copies that nothing gates.
func consumerDirs(root string) ([]string, error) {
	base := filepath.Join(root, consumerRoot)

	var dirs []string

	err := filepath.WalkDir(base, func(found string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() {
			return nil
		}

		name := d.Name()
		if name == "testdata" || name == "node_modules" || (len(name) > 1 && name[0] == '.') {
			return fs.SkipDir
		}
		if name != consumerDirName {
			return nil
		}

		rel, relErr := filepath.Rel(root, found)
		if relErr != nil {
			return relErr
		}
		dirs = append(dirs, rel)

		return fs.SkipDir
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", base, err)
	}

	if len(dirs) == 0 {
		return nil, fmt.Errorf("%s holds no %s/ directory, so nothing could be compared", base, consumerDirName)
	}

	sort.Strings(dirs)

	return dirs, nil
}

// isCheckIgnoreAnswer reports whether a `git check-ignore` exit code is an ANSWER
// rather than a failure. 1 is "nothing matched" and 128 is "not a git repository";
// both tell us what we asked. Named so the two numbers read as what they mean at
// the call site instead of as a compound condition.
func isCheckIgnoreAnswer(code int) bool {
	return code == 1 || code == 128
}

// ignoredNames names the entries of the vendor directory that git ignores. It asks
// git rather than matching names, so a cache directory nobody predicted is excluded
// on the same evidence as the one that prompted this: the repository already says it
// is not content. Outside a git checkout the answer is "nothing is ignored", which
// is the reading that reports more rather than less.
func ignoredNames(root string, entries []os.DirEntry) (map[string]bool, error) {
	ignored := map[string]bool{}
	if len(entries) == 0 {
		return ignored, nil
	}

	var stdin bytes.Buffer
	for _, entry := range entries {
		stdin.WriteString(path.Join(vendorDir, entry.Name())) //nolint:errcheck // bytes.Buffer never fails
		stdin.WriteByte('\n')                                 //nolint:errcheck // bytes.Buffer never fails
	}

	// The bound is what the script had no way to express: a git that never
	// answers held the gate open with no diagnosis. internal/le/parity bounds its
	// own subprocess the same way.
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	// root is the checkout lepath.Root() discovered or a fixture a test named.
	// Nothing here is reachable from a network peer: le is a build-host tool.
	cmd := exec.CommandContext(ctx, "git", "-C", root, "check-ignore", "--stdin") //nolint:gosec // root is the checkout, resolved by lepath.Root
	cmd.Stdin = &stdin
	out, err := cmd.Output()
	// Exit 1 is "nothing matched" and exit 128 is "not a git repository". Both are
	// answers, not failures. Anything else is a broken invocation and is reported.
	//
	// One guard per fact, rather than one condition holding three: a reader deciding
	// whether every case is covered should not have to hold "did it exit at all",
	// "is it an ExitError" and "which code" at once (docs/contributing/ze-go-style.md,
	// "Control flow a reader can simulate").
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			return nil, fmt.Errorf("git check-ignore in %s: %w", root, err)
		}
		if !isCheckIgnoreAnswer(exit.ExitCode()) {
			return nil, fmt.Errorf("git check-ignore in %s: %w", root, err)
		}
	}

	for line := range strings.SplitSeq(string(out), "\n") {
		if line == "" {
			continue
		}
		ignored[path.Base(strings.TrimSuffix(line, "/"))] = true
	}
	return ignored, nil
}

// subscribes reports whether a consumer holds any file of one vendor package.
func subscribes(root, consumer string, pkg vendorPackage) bool {
	for _, name := range pkg.files {
		if _, err := os.Stat(filepath.Join(root, consumer, name)); err == nil {
			return true
		}
	}

	return false
}
