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
	"errors"
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

// gitTimeout limits each of the three git commands used to build the hash, so
// a wedged git cannot delay a job indefinitely.
//
// On the host's own disk a `git diff HEAD` for this checkout takes hundreds of
// milliseconds. The bound does NOT hold for every machine that sees the
// checkout: in a QEMU guest mounting it over 9p, the stat data in git's index
// matches no file, so git reads every tracked file and the diff runs for
// minutes. On 2026-10-27 that killed `le test firewall` in the guest at
// admission. No guest le measures here: a guest command runs inside the slot
// the host's `test qemu run` holds, which the guest names as its job parent
// (admitQEMURun and guestRunParentSetup, internal/le/test/qemu), and an
// admission inside a parent measures nothing.
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
// internal/le/go/lint/actions.go spells its job label from this constant,
// because the label and the trees declared below are one fact: a label whose
// inputs were declared elsewhere would drift from the work it names.
const LintLabel = "lint"

// lintIgnores names the trees that `le go lint run` does not read.
//
// The declaration EXCLUDES rather than lists, and that direction is the safety
// property. An input nobody thought of is fingerprinted by default, so a
// missing entry costs a duplicate run. An inclusion list fails the other way:
// an input left off it is held identical across a change that could have
// reddened the run, which is a green that could not have been red.
//
// Lint validates its actual go-list source population against this declaration
// before admission can reuse a verdict. A preserved Go file in a nested module,
// testdata, or a Go-hidden directory is not thereby a compiler input.
// CheckLintInputs reads this same declaration; it does not maintain a second
// list of excluded trees.
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

// CheckLintInputs refuses actual checkout-relative compiler inputs omitted from
// the lint fingerprint, including a selected symlink's canonical target and an
// input git ignores. The root MUST be a git work tree. Callers MUST pass the complete producer-selected compiler population before
// admitting work that may attach to a shared verdict. Resolution errors refuse.
func CheckLintInputs(root string, selected map[string]bool) error {
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("resolve lint checkout %s: %w", root, err)
	}
	canonicals := make([]string, 0, len(selected))
	for path := range selected {
		if ignoredTree(path, lintIgnores) {
			return fmt.Errorf("lint input %s belongs to a tree excluded from the lint fingerprint; correct the input declaration before sharing a verdict", path)
		}
		canonical, err := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return fmt.Errorf("resolve lint input %s: %w", path, err)
		}
		relative, err := filepath.Rel(canonicalRoot, canonical)
		if err != nil {
			return fmt.Errorf("locate canonical lint input %s: %w", path, err)
		}
		relative = filepath.ToSlash(relative)
		if relative == ".." {
			return fmt.Errorf("lint input %s resolves outside the fingerprinted checkout: %s", path, canonical)
		}
		if strings.HasPrefix(relative, "../") {
			return fmt.Errorf("lint input %s resolves outside the fingerprinted checkout: %s", path, canonical)
		}
		if ignoredTree(relative, lintIgnores) {
			return fmt.Errorf("lint input %s resolves to excluded compiler input %s; correct the input declaration before sharing a verdict", path, relative)
		}
		canonicals = append(canonicals, relative)
	}
	return refuseGitIgnoredInputs(root, canonicals)
}

// refuseGitIgnoredInputs refuses a selected compiler input that git ignores.
// InputHash reads only the paths git reports (tracked changes and untracked
// files git does not ignore), so an ignored input is outside the lint
// fingerprint: an edit to it would not void a shared verdict. The paths are
// canonical and checkout-relative, because git judges a path beyond a symlink
// as an error and fingerprints the target, not the link's directory.
func refuseGitIgnoredInputs(root string, canonicals []string) error {
	if len(canonicals) == 0 {
		return nil
	}
	slices.Sort(canonicals)
	var input bytes.Buffer
	for _, relative := range canonicals {
		input.WriteString(relative)
		input.WriteByte(0)
	}
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	//nolint:gosec // every argument is a literal; only the directory and stdin vary
	cmd := exec.CommandContext(ctx, "git", "check-ignore", "-z", "--stdin")
	cmd.Dir = root
	cmd.Stdin = &input
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if exit, isExit := errors.AsType[*exec.ExitError](err); isExit {
		// refuse-by-default: guards CheckLintInputs -- exit 1 is git check-ignore's verdict that it ignores no selected input
		if exit.ExitCode() == 1 {
			return nil
		}
	}
	if err != nil {
		return fmt.Errorf("ask git which lint inputs it ignores in %s: %w: %s", root, err, strings.TrimSpace(stderr.String()))
	}
	ignored := nulPaths(out)
	if len(ignored) == 0 {
		return fmt.Errorf("git check-ignore in %s reported ignored lint inputs and named none; rerun the lint", root)
	}
	return fmt.Errorf("lint input %s is ignored by git, so the lint fingerprint never reads it and an edit to it would not void a shared verdict; give its directory its own go.mod to keep it out of lint, or stop ignoring it in .gitignore", strings.Join(ignored, ", "))
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
//
// InputHash refuses a failed Git query or an unreadable input. An absent file
// is a measured deletion; a failed measurement is not a matching generation.
func InputHash(root, label string) (string, error) {
	commit, err := git(root, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("fingerprint %s inputs: read HEAD: %w", label, err)
	}
	sum := sha256.New()
	add(sum, commit)

	ignored, declared := labelIgnores[label]
	if declared {
		if err := writeReadPaths(sum, root, ignored); err != nil {
			return "", fmt.Errorf("fingerprint %s inputs: %w", label, err)
		}
		return hex.EncodeToString(sum.Sum(nil)), nil
	}

	// Undeclared labels retain the whole-tree stream, but a Git failure must
	// refuse admission instead of certifying the remaining fragments.
	diff, err := git(root, "diff", "HEAD")
	if err != nil {
		return "", fmt.Errorf("fingerprint %s inputs: read tracked changes: %w", label, err)
	}
	add(sum, diff)
	untracked, err := git(root, "ls-files", "-o", "--exclude-standard", "-z")
	if err != nil {
		return "", fmt.Errorf("fingerprint %s inputs: list untracked files: %w", label, err)
	}
	paths := strings.Split(string(untracked), "\x00")
	slices.Sort(paths)
	for _, rel := range paths {
		if rel == "" {
			continue
		}
		addText(sum, rel)
		addText(sum, "\n")
		if err := writeInputFileHash(sum, root, rel); err != nil {
			return "", fmt.Errorf("fingerprint %s inputs: %w", label, err)
		}
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
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
func writeReadPaths(sum hash.Hash, root string, ignored []string) error {
	paths, err := changedPaths(root, "HEAD")
	if err != nil {
		return err
	}
	for _, rel := range paths {
		if ignoredTree(rel, ignored) {
			continue
		}
		addText(sum, rel)
		addText(sum, "\n")
		if err := writeInputFileHash(sum, root, rel); err != nil {
			return err
		}
	}
	return nil
}

// writeInputFileHash keeps a proven deletion distinct from an input that
// could not be read. Only the former can participate in a matching digest.
func writeInputFileHash(sum hash.Hash, root, rel string) error {
	fingerprint, err := inputFingerprint(root, rel)
	if err != nil {
		return err
	}
	addText(sum, fingerprint)
	addText(sum, "\n")
	return nil
}

// inputFingerprint answers the content fingerprint of one input, or the
// MISSING sentinel for a path that does not exist, which is a measured
// deletion. A path that exists and cannot be read, or is not a regular file,
// is an error: unlike Fingerprint, an input measurement never turns a failed
// read into a value a later measurement could match.
func inputFingerprint(root, rel string) (string, error) {
	path := filepath.Join(root, filepath.FromSlash(rel))
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return strings.TrimSpace(missingFile), nil
	}
	if err != nil {
		return "", fmt.Errorf("stat input %s: %w", rel, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("fingerprint input %s: not a regular file", rel)
	}
	content, err := fileHash(path)
	if err != nil {
		return "", fmt.Errorf("read input %s: %w", rel, err)
	}
	return content, nil
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

// TreeSnapshot records the whole-tree hash and each dirty path fingerprint at
// one instant.
type TreeSnapshot struct {
	Hash     string
	Manifest map[string]string
	// ManifestErr says Git could not list the dirty paths, so Manifest holds
	// nothing that can be compared. A reader MUST treat such a snapshot as
	// unmeasured: an empty manifest reads as "every path matched its commit",
	// which would certify content no stage read.
	ManifestErr error
}

// SnapshotTree records both fingerprints used by a verification certificate.
// The two views are intentionally produced by this package. Admission and
// verification MUST use one definition of the checkout state.
func SnapshotTree(root string) TreeSnapshot {
	manifest, err := DirtyManifest(root)
	return TreeSnapshot{
		Hash:        TreeHash(root),
		Manifest:    manifest,
		ManifestErr: err,
	}
}

// DirtyManifest fingerprints each path that differs from HEAD.
//
// A deleted path and a non-regular path both carry MISSING. Paths use Git's -z
// spelling, which never quotes a path, so a manifest compares directly with
// what ChangedInputs lists. A tree Git cannot list is an error, never an
// empty manifest: ChangedInputs reads a path absent from a manifest as one that
// matched its commit, so an empty manifest would certify content no stage read.
func DirtyManifest(root string) (map[string]string, error) {
	paths, err := changedPaths(root, "HEAD")
	if err != nil {
		return nil, fmt.Errorf("list the dirty paths for the manifest: %w", err)
	}
	manifest := make(map[string]string, len(paths))
	for _, rel := range paths {
		manifest[rel] = Fingerprint(root, rel)
	}
	return manifest, nil
}

// Fingerprint answers the content fingerprint a manifest row carries for one
// path, whether or not that path differs from HEAD.
//
// It is inputFingerprint with one difference: a path that exists and cannot be
// measured carries the MISSING sentinel too, because a manifest row has no
// place for an error. A reader that must refuse an unreadable input asks
// ChangedInputs, which keeps the error.
func Fingerprint(root, rel string) string {
	fingerprint, err := inputFingerprint(root, rel)
	if err != nil {
		return strings.TrimSpace(missingFile)
	}
	return fingerprint
}

// PathsChangedBetween names every path whose content differs between two
// commits, in Git's -z spelling, which never quotes a path.
//
// The error says the two commits cannot be compared, which is what a commit
// that has left the checkout looks like after a rebase. A caller that cannot
// compare has learned nothing about the paths, so it must not read the empty
// result as "nothing changed".
func PathsChangedBetween(root, from, to string) ([]string, error) {
	out, err := git(root, "diff", "--name-only", "-z", from, to)
	if err != nil {
		return nil, fmt.Errorf("git diff %s %s: %w", from, to, err)
	}
	return nulPaths(out), nil
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
