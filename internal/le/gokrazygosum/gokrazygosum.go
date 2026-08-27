// Design: docs/architecture/appliance/gokrazy-build-pins.md -- the packed go.sum gate
//
// Package gokrazygosum compares the tracked `gokrazy/ze/builddir/**/go.sum`
// files with the root module's go.sum. A go.sum line is `module version hash`,
// so the same (module, version) carrying two DIFFERENT hashes says one of the
// two files records a module content that the other would refuse.
//
// The check is narrow by construction, and both exclusions are deliberate. A
// (module, version) present only in a builddir file is normal: those modules
// are the third-party programs gokrazy packs, and they depend on things the
// root module does not. Version SKEW is normal too, because the builddirs are
// independent modules. Only the one condition that cannot be legitimate is
// reported.
//
// It matters because nothing else reads those files. No build outside the
// gokrazy image build opens them, so a drift there surfaces nowhere and ships
// in the image.
//
// Detail: report.go holds the answer, register.go the registration.
package gokrazygosum

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/leroot"
)

// The two paths this gate is about, relative to the tree it judges.
//
// rootGosum is both the root module's file and the base name every builddir
// file carries, which is why one constant serves the root read and the suffix
// test: they are the same file name in two places.
const (
	builddirPrefix = "gokrazy/ze/builddir/"
	rootGosum      = "go.sum"
)

// name is the word this command is typed as.
const name = "gokrazy-gosum"

// Answer is the `le gokrazy-gosum` command. The tree is the checkout, so there
// is nothing to type after the name.
func Answer(args []string) (any, int) {
	if len(args) > 0 {
		return nil, leroot.RefuseArgument(name, args[0])
	}

	tree, err := lepath.Root()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: gokrazy-gosum: %v\n", err) //nolint:errcheck // CLI output
		return nil, 2
	}

	report, err := Check(tree)
	if err != nil {
		// 2 rather than 1: the gate could not read what it judges, which a
		// caller reads apart from a tree that holds a conflict.
		fmt.Fprintf(os.Stderr, "error: gokrazy-gosum: %v\n", err) //nolint:errcheck // CLI output
		return nil, 2
	}

	if len(report.Conflicts) > 0 {
		return report, 1
	}
	return report, 0
}

// entry is one go.sum line's identity. A go.sum carries two lines per module,
// one for the zip and one for the `/go.mod`, and the version string as written
// keeps them apart: `v1.2.3` and `v1.2.3/go.mod` are two entries and are
// compared separately.
type entry struct {
	Module  string
	Version string
}

// TrackedGosums answers the tracked builddir go.sum paths under tree.
//
// It asks git rather than walking, because the question is what the repository
// SHIPS. An untracked go.sum left in a builddir by a local build is not part of
// the image, and this gate has nothing to say about it.
func TrackedGosums(tree string) ([]string, error) {
	// The query is bounded by git itself rather than by a timeout. `git
	// ls-files` over a checkout is local work with no network and no lock this
	// process waits behind, and a timeout would turn a slow filesystem into a
	// report that there is nothing to check.
	cmd := exec.Command("git", "-C", tree, "ls-files") //nolint:gosec,noctx // a build tool queries the checkout it was pointed at
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var found []string
	for path := range strings.SplitSeq(string(out), "\n") {
		if strings.HasPrefix(path, builddirPrefix) && strings.HasSuffix(path, rootGosum) {
			found = append(found, path)
		}
	}
	// git ls-files answers sorted paths, so nothing is sorted here. A second
	// sort would say this code depends on git's order, which it does not: what
	// it depends on is the order being STABLE, and re-sorting hides a change in
	// git rather than surviving one.
	return found, nil
}

// readGosum reads one go.sum file into its entries, keeping file order.
//
// The order is carried because it reaches the answer: a conflict list is
// rendered and compared in the order the file states its modules, and a Go map
// randomizes that on every run.
func readGosum(path string) ([]entry, map[entry]string, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- a tracked go.sum path under the checkout
	if err != nil {
		return nil, nil, err
	}

	order := make([]entry, 0, 64)
	hash := make(map[entry]string, 64)
	for line := range strings.SplitSeq(string(data), "\n") {
		parts := strings.Fields(line)
		// A go.sum line is exactly three fields. Anything else is a blank
		// line, or a form this gate has no opinion about.
		if len(parts) != 3 {
			continue
		}
		key := entry{Module: parts[0], Version: parts[1]}
		if _, seen := hash[key]; !seen {
			order = append(order, key)
		}
		// A repeated key takes the LAST hash, which is what the assignment in
		// the script this ports does.
		hash[key] = parts[2]
	}
	return order, hash, nil
}

// Check compares every tracked builddir go.sum against the root module's.
//
// An error means the gate could not read what it judges, which is a different
// fact from a tree that holds a conflict, and the caller answers a different
// exit code for it.
func Check(tree string) (Report, error) {
	files, err := TrackedGosums(tree)
	if err != nil {
		return Report{}, err
	}

	report := Report{Files: len(files)}
	if len(files) == 0 {
		// Not an error: the builddir can be retired, or this can run over a
		// checkout that does not carry it. The count says so, and Text says it
		// in words, so a zero-file run cannot read as a clean one.
		return report, nil
	}

	_, rootHash, err := readGosum(filepath.Join(tree, rootGosum))
	if err != nil {
		return Report{}, err
	}

	for _, rel := range files {
		order, hash, err := readGosum(filepath.Join(tree, rel))
		if err != nil {
			return Report{}, err
		}
		for _, key := range order {
			root, shared := rootHash[key]
			if !shared {
				continue
			}
			report.Shared++
			if root == hash[key] {
				continue
			}
			report.Conflicts = append(report.Conflicts, Conflict{
				Path:         rel,
				Module:       key.Module,
				Version:      key.Version,
				RootHash:     root,
				BuilddirHash: hash[key],
			})
		}
	}
	return report, nil
}
