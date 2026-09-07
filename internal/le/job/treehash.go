// Design: docs/architecture/core-design.md -- the fingerprint of the tree a job judges
//
// A job records the tree that it will judge. A second asker uses that job's
// verdict only when the two trees match. If attachment used only the LABEL, a
// run that never saw a session's code would certify that session. The
// Go-commit coverage gate reads that certificate
// (full_verify_coverage in internal/le/commit/actions.go).
//
// This package is the single definition shared by job admission and native
// verification certificates. The hashed stream contains the commit, all
// tracked changes, and each untracked file with the hash of its content.
//
// Two fingerprints live here and they answer different questions. TreeHash
// describes the WHOLE checkout and is what a certificate asserts. InputHash
// describes the inputs ONE label's work reads and is what admission compares,
// because a job that shares another job's verdict needs the inputs to match
// rather than the checkout.

package job

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Unknown is the tree hash for a job whose tree was not measured. It matches
// no value, including another Unknown. An unmeasured tree is not a matching
// tree. This implementation does not produce Unknown because it can always
// compute a hash. The shell half can write it, and the attach rule reads it.
const Unknown = "unknown"

// gitTimeout limits each of the three git commands used to build the hash. A
// `git diff HEAD` for this checkout usually takes hundreds of milliseconds.
// A command that exceeds this limit is wedged, not slow. Such a command must
// not delay a job indefinitely.
const gitTimeout = 60 * time.Second

// noHead stands in for the commit when git cannot name one. It is what the
// shell half writes, so an empty repository still hashes to the same value on
// both sides.
const noHead = "NO_HEAD\n"

// missingFile substitutes for the content of an untracked path that is not a
// regular file. Such a path can be a directory, a dangling symlink, or a file
// removed between the listing and the read.
const missingFile = "MISSING\n"

// TreeHash returns the fingerprint of the checkout at root.
//
// It never fails. If git cannot describe a tree, TreeHash uses the stand-in
// values above to calculate the hash. Without a hash, a caller would have to
// refuse every job or admit every job. Neither choice is a valid admission
// decision.
func TreeHash(root string) string {
	sum := sha256.New()

	writeCommit(sum, root)
	writeDiff(sum, root)
	writeUntracked(sum, root)

	return hex.EncodeToString(sum.Sum(nil))
}

// LintLabel is the name every lint on this machine claims in the registry.
// internal/le/verify/lint/actions.go spells its job label from this constant,
// because the label and the trees declared below are one fact: a label whose
// inputs were declared elsewhere would drift from the work it names.
const LintLabel = "lint"

// lintIgnores names the trees that `le verify lint run` does not read.
//
// The declaration EXCLUDES rather than lists, and that direction is the safety
// property. An input nobody thought of is fingerprinted by default, so a
// missing entry costs a duplicate run. An inclusion list fails the other way:
// an input left off it is held identical across a change that could have
// reddened the run, which is a green that could not have been red.
//
// Lint loads Go packages, so a file reaches its verdict by one of two routes.
// It is Go source that a pass type-checks, or it is embedded into a package by
// a //go:embed directive. A //go:embed pattern cannot leave the directory of
// the package that writes it, so a tree holding no Go file at all reaches
// neither route. Every tree below holds none, and
// TestTheTreesTheLintLabelIgnoresHoldNoGoFile keeps it that way.
//
// These are the trees several sessions write while a lint runs: journal rows,
// specs, rules and pages. They are why two identical lints almost never shared
// before this list existed.
var lintIgnores = []string{
	".claude/",
	"ai/",
	"backups/",
	"docs/",
	"plan/",
	"rfc/",
	"website/",
}

// labelIgnores answers the trees one label's work does not read. A label with
// no entry is fingerprinted over the whole checkout, which is where every
// label started: it shares least, and it is never wrong about what it read.
var labelIgnores = map[string][]string{
	LintLabel: lintIgnores,
}

// InputHash returns the fingerprint of the inputs one label's work reads.
//
// Admission compares this value rather than TreeHash. A second asker uses a
// running job's verdict only when both judge the same inputs, and on a
// checkout that nine sessions write the whole-tree hash almost never holds
// still: a journal row written by somebody else answered "different tree" for
// two jobs doing identical work over identical Go source.
//
// A verification certificate keeps the whole-tree answer, because it asserts
// something about the whole tree. SnapshotTree is unchanged
// (docs/architecture/testing/verify-freshness-scope.md).
func InputHash(root, label string) string {
	ignored, declared := labelIgnores[label]
	if !declared {
		return TreeHash(root)
	}

	sum := sha256.New()

	writeCommit(sum, root)
	writeReadPaths(sum, root, ignored)

	return hex.EncodeToString(sum.Sum(nil))
}

// writeReadPaths puts each changed path this label reads into the stream: the
// path, and then the hash of its content.
//
// The paths are sorted, so the fingerprint does not depend on the order git
// listed them in, and each one is written once: a path that git reports as
// both changed and untracked is one input.
//
// A whole diff cannot be hashed here the way writeDiff hashes one, because a
// diff is one blob and this fingerprint has to drop the paths inside it that
// the label does not read.
func writeReadPaths(sum hash.Hash, root string, ignored []string) {
	paths := dirtyPaths(root)
	slices.Sort(paths)
	paths = slices.Compact(paths)

	for _, rel := range paths {
		if ignoredTree(rel, ignored) {
			continue
		}
		addText(sum, rel)
		addText(sum, "\n")
		writeFileHash(sum, filepath.Join(root, filepath.FromSlash(rel)))
	}
}

// ignoredTree reports whether one changed path lies under a tree this label
// does not read. Git spells every path with forward slashes and every entry
// ends in one, so the prefix test needs no conversion and cannot match a
// sibling whose name merely starts the same way.
func ignoredTree(rel string, ignored []string) bool {
	for _, tree := range ignored {
		if strings.HasPrefix(rel, tree) {
			return true
		}
	}
	return false
}

// dirtyPaths lists every path that differs from HEAD: the tracked changes
// first, then the untracked files. A path can appear twice, and each caller
// says what it does with the repeat.
func dirtyPaths(root string) []string {
	tracked, _ := git(root, "diff", "HEAD", "--name-only")
	untracked, _ := git(root, "ls-files", "-o", "--exclude-standard")
	return append(nonEmptyLines(tracked), nonEmptyLines(untracked)...)
}

// TreeSnapshot records the whole-tree hash and each dirty path fingerprint at
// one instant.
type TreeSnapshot struct {
	Hash     string
	Manifest map[string]string
}

// SnapshotTree records both fingerprints used by a verification certificate.
// The two views are intentionally produced by this package. Admission and
// verification MUST use one definition of the checkout state.
func SnapshotTree(root string) TreeSnapshot {
	return TreeSnapshot{
		Hash:     TreeHash(root),
		Manifest: DirtyManifest(root),
	}
}

// DirtyManifest fingerprints each path that differs from HEAD.
//
// A deleted path and a non-regular path both carry MISSING. Paths use the
// spelling that Git prints because the status file format predates this port
// and both its readers use that spelling.
func DirtyManifest(root string) map[string]string {
	paths := dirtyPaths(root)

	manifest := make(map[string]string, len(paths))
	for _, rel := range paths {
		if _, exists := manifest[rel]; exists {
			continue
		}
		manifest[rel] = Fingerprint(root, rel)
	}
	return manifest
}

// Fingerprint answers the content fingerprint a manifest row carries for one
// path, whether or not that path differs from HEAD.
//
// A dirty manifest can only speak about the paths that were dirty when it was
// taken, and a reader comparing an old manifest against today's checkout needs
// the same measurement for a path that has since gone clean. It is the same
// answer either way: the content decides it, and where the content is stored
// does not.
//
// A path that is absent, or that is not a regular file, carries the MISSING
// sentinel a manifest row uses, so the two views compare directly.
func Fingerprint(root, rel string) string {
	path := filepath.Join(root, filepath.FromSlash(rel))
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return strings.TrimSpace(missingFile)
	}
	fingerprint, err := fileHash(path)
	if err != nil {
		return strings.TrimSpace(missingFile)
	}
	return fingerprint
}

// PathsChangedBetween names every path whose content differs between two
// commits, in git's own spelling.
//
// The error says the two commits cannot be compared, which is what a commit
// that has left the checkout looks like after a rebase. A caller that cannot
// compare has learned nothing about the paths, so it must not read the empty
// result as "nothing changed".
func PathsChangedBetween(root, from, to string) ([]string, error) {
	out, err := git(root, "diff", "--name-only", from, to)
	if err != nil {
		return nil, fmt.Errorf("git diff %s %s: %w", from, to, err)
	}
	return nonEmptyLines(out), nil
}

// Head returns the current commit, or Unknown when Git cannot name one.
func Head(root string) string {
	out, err := git(root, "rev-parse", "HEAD")
	if err != nil {
		return Unknown
	}
	head := strings.TrimSpace(string(out))
	if head == "" {
		return Unknown
	}
	return head
}

func nonEmptyLines(out []byte) []string {
	lines := strings.Split(string(trimNewline(out)), "\n")
	kept := lines[:0]
	for _, line := range lines {
		if line != "" {
			kept = append(kept, line)
		}
	}
	return kept
}

// add puts bytes into the stream being hashed.
//
// hash.Hash documents that Write never returns an error, and this is the one
// place in the package that relies on it. Every other caller goes through
// here, so the reliance is stated once rather than discarded eight times.
func add(sum hash.Hash, text []byte) {
	if _, err := sum.Write(text); err != nil {
		panic("BUG: hash.Hash.Write answered an error, which its contract forbids")
	}
}

// addText is add for a string, which is what most of the stream is.
func addText(sum hash.Hash, text string) {
	add(sum, []byte(text))
}

// writeCommit puts the commit this tree sits on into the stream, or the
// stand-in when git cannot name one.
func writeCommit(sum hash.Hash, root string) {
	out, err := git(root, "rev-parse", "HEAD")
	if err != nil {
		addText(sum, noHead)
		return
	}
	add(sum, out)
}

// writeDiff puts every tracked change into the stream, staged and unstaged
// alike. A tree Git cannot diff contributes nothing, matching the established
// fingerprint definition.
func writeDiff(sum hash.Hash, root string) {
	out, err := git(root, "diff", "HEAD")
	if err != nil {
		return
	}
	add(sum, out)
}

// writeUntracked adds each untracked file to the stream. It adds the path and
// then the hash of the file content.
//
// Ignored files are excluded. Therefore, the registry can remain under tmp/
// and the hash stays unchanged when a job is admitted.
//
// Paths are sorted in byte order. Git lists paths in its own order, so the
// explicit sort makes the fingerprint deterministic.
func writeUntracked(sum hash.Hash, root string) {
	out, err := git(root, "ls-files", "-o", "--exclude-standard")
	if err != nil {
		return
	}

	paths := strings.Split(string(trimNewline(out)), "\n")
	slices.Sort(paths)

	for _, path := range paths {
		if path == "" {
			continue
		}
		addText(sum, path)
		addText(sum, "\n")
		writeFileHash(sum, filepath.Join(root, path))
	}
}

// writeFileHash puts the hash of one file's content into the stream, or the
// stand-in when the path is not a regular file.
func writeFileHash(sum hash.Hash, path string) {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		addText(sum, missingFile)
		return
	}

	content, err := fileHash(path)
	if err != nil {
		addText(sum, missingFile)
		return
	}
	addText(sum, content)
	addText(sum, "\n")
}

// fileHash answers the hex hash of one file's content.
func fileHash(path string) (string, error) {
	file, err := os.Open(path) //nolint:gosec // the path came from git's own listing of this checkout
	if err != nil {
		return "", err
	}

	sum := sha256.New()
	_, copyErr := io.Copy(sum, file)
	closeErr := file.Close()

	if copyErr != nil {
		return "", copyErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}

// git runs one read-only Git command in the checkout and returns its stdout.
//
// A failed command returns an error. Each caller above converts the error to
// the fingerprint's fail-closed stand-in value. Stderr is discarded because a
// repository question that cannot be answered is a fact about the tree, not a
// failure to report to a process waiting for admission.
func git(root string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	//nolint:gosec // every argument is a literal in this file; only the directory varies
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	return cmd.Output()
}

// trimNewline drops one trailing newline, which is what a shell command
// substitution does to the output it captures.
func trimNewline(out []byte) []byte {
	return bytes.TrimRight(out, "\n")
}
