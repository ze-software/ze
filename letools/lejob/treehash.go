// Design: docs/architecture/core-design.md -- the fingerprint of the tree a job judges
//
// A job records the tree that it will judge. A second asker uses that job's
// verdict only when the two trees match. If attachment used only the LABEL, a
// run that never saw a session's code would certify that session. The
// Go-commit coverage gate reads that certificate
// (full_verify_coverage in scripts/dev/commit_helper.py).
//
// The Go implementation restates the definition from
// scripts/dev/verify-status.sh. The two implementations must agree byte for
// byte. During migration, jobs admitted by the shell and by this package share
// a registry. A different hash would silently prevent sharing. The hashed
// stream contains the commit, all tracked changes, and each untracked file
// with the hash of its content.

package lejob

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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
// alike. A tree git cannot diff contributes nothing, which is what the shell
// half's `|| true` does.
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
// Paths are sorted in byte order to match `LC_ALL=C sort` in the shell half.
// git lists paths in its own order. The extra sort makes the two streams
// identical regardless of the order selected by git.
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

// git runs one read-only git command in the checkout and returns its stdout.
//
// A failed command returns an error. Each caller above converts the error to
// the stand-in value that the shell half writes. Stderr is discarded because
// the shell half also discards it. A repository question that cannot be
// answered is a fact about the tree. It is not a failure to report to a
// process that waits for a slot.
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
