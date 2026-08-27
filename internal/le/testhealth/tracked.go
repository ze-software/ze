// Design: docs/architecture/testing/test-health.md -- which files count
//
// tracked.go answers which files the page is a function of. The answer is
// GIT'S INDEX, never a working-tree listing.
//
// Without it, an untracked work-in-progress test moved the published counts,
// and the developer who regenerated the page then committed numbers that a
// clean CI checkout could not reproduce -- the staleness gate went red for
// everyone.
//
// The ratchet is deliberately NOT filtered this way: internal/le/testsensitivity
// scans the working tree so an inert test is caught before it is committed,
// rather than blaming the next unrelated change.
package testhealth

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// subprocessDeadline bounds every git call. A hung `git log` would otherwise
// hang the whole verify run with no diagnostic.
const subprocessDeadline = 600 * time.Second

// excludedParts are the path components a counted file may not sit under.
// vendor/ and node_modules/ are third-party, and testdata/ holds fixtures
// rather than tests.
var excludedParts = map[string]bool{"vendor": true, "testdata": true, "node_modules": true}

// tree is one checkout, with the index cached for the run.
//
// The cache is per tree rather than global: a test points two runs at two
// fixtures in one process, and a global cache would answer the first tree's
// index for the second.
type tree struct {
	root    string
	tracked map[string]bool
}

// newTree names the checkout every collector reads.
func newTree(root string) *tree { return &tree{root: root} }

// trackedFiles answers the repository-relative paths git has in its index.
//
// It FAILS CLOSED twice. A git that cannot run is a refusal rather than an
// empty set, and an index that lists nothing is a broken query rather than an
// empty repository: every count on this page is a filter over this set, so an
// empty answer publishes zero of everything and reads as the goal state.
func (t *tree) trackedFiles() (map[string]bool, error) {
	if t.tracked != nil {
		return t.tracked, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), subprocessDeadline)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "-C", t.root, "ls-files", "-z") // #nosec G204 -- the checkout path this tool was pointed at
	out, err := cmd.Output()
	if err != nil {
		return nil, collectErrorf("git ls-files failed in %s: %w", t.root, err)
	}

	files := make(map[string]bool)
	for name := range strings.SplitSeq(string(out), "\x00") {
		if name != "" {
			files[name] = true
		}
	}
	if len(files) == 0 {
		return nil, collectErrorf("git ls-files listed nothing in %s", t.root)
	}
	t.tracked = files
	return files, nil
}

// trackedMatching answers the tracked files under one subtree ending in
// suffix, in a stable order.
//
// It reads git's INDEX, so a file deleted or moved in the working tree is still
// listed until that deletion is staged. Those entries are skipped: there is no
// content left to count, and on the clean checkout these counts are meant to
// describe, the deletion is committed and the entry is gone. Without the skip
// every developer mid-refactor gets a read failure from the caller, before they
// are able to commit.
func (t *tree) trackedMatching(subtree, suffix string) ([]string, error) {
	files, err := t.trackedFiles()
	if err != nil {
		return nil, err
	}

	var tb textbuf.Buffer
	prefix := tb.Str(subtree).Byte('/').String()

	var out []string
	for name := range files {
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
			continue
		}
		if hasExcludedPart(name) {
			continue
		}
		if !exists(filepath.Join(t.root, filepath.FromSlash(name))) {
			continue
		}
		out = append(out, name)
	}
	sort.Slice(out, func(i, j int) bool { return lessByPathParts(out[i], out[j]) })
	return out, nil
}

// hasExcludedPart reports whether any component of a path is excluded.
func hasExcludedPart(name string) bool {
	for part := range strings.SplitSeq(name, "/") {
		if excludedParts[part] {
			return true
		}
	}
	return false
}

// lessByPathParts orders two paths the way Python orders two Path objects,
// which is COMPONENT BY COMPONENT rather than byte by byte.
//
// The difference is live in this tree: "cmd/ze" sorts before "cmd/ze-gok"
// there and after it here, because `-` is 0x2d and `/` is 0x2f. The order
// reaches the page through the negative-test table, whose ranking is a STABLE
// sort over a ratio and therefore breaks its many ties on the order the files
// arrived in.
func lessByPathParts(left, right string) bool {
	leftParts, rightParts := strings.Split(left, "/"), strings.Split(right, "/")
	for index := 0; index < len(leftParts) && index < len(rightParts); index++ {
		if leftParts[index] != rightParts[index] {
			return leftParts[index] < rightParts[index]
		}
	}
	return len(leftParts) < len(rightParts)
}

// exists reports whether the path is there at all, which is what Python's
// Path.exists answers.
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// readText answers one repository-relative file's content, and refuses when it
// is not there.
func (t *tree) readText(rel string) (string, error) {
	if !exists(filepath.Join(t.root, filepath.FromSlash(rel))) {
		return "", collectErrorf("%s does not exist", rel)
	}
	return t.readBody(rel)
}

// readBody answers a file the tree listed, dropping the bytes that are not
// valid UTF-8 the way Python's `errors="ignore"` drops them.
//
// A file that is PRESENT and unreadable is an error rather than an empty
// string: every metric here counts what a file holds, and a lower count is
// what passing looks like.
func (t *tree) readBody(rel string) (string, error) {
	body, err := os.ReadFile(filepath.Join(t.root, filepath.FromSlash(rel))) // #nosec G304 -- a repository-relative path this package listed from git's index
	if err != nil {
		return "", collectErrorf("%s cannot be read: %w", rel, err)
	}
	return decodeIgnoringErrors(body), nil
}

// decodeIgnoringErrors drops every byte that begins no valid UTF-8 sequence,
// which is what Python's `errors="ignore"` does when it decodes.
//
// Go strings hold bytes rather than runes, so a plain conversion would keep the
// invalid bytes and every count over the text would differ from the script's on
// the one file that carries them.
func decodeIgnoringErrors(body []byte) string {
	if utf8.Valid(body) {
		return string(body)
	}
	kept := make([]byte, 0, len(body))
	for index := 0; index < len(body); {
		char, size := utf8.DecodeRune(body[index:])
		if char == utf8.RuneError && size == 1 {
			index++
			continue
		}
		kept = append(kept, body[index:index+size]...)
		index += size
	}
	return string(kept)
}

// gitOutput runs one git query in the tree and answers its output. A git that
// could not answer is an error: every caller here is measuring, and an empty
// answer would be a measurement of nothing.
func (t *tree) gitOutput(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), subprocessDeadline)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...) // #nosec G204 -- this package's own fixed argument lists
	cmd.Dir = t.root
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
