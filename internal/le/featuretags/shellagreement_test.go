// Design: ai/rules/principles.md -- a fact declared once; where a copy is
// unavoidable, a check compares the copy against its source.
//
// feature-gates.txt has exactly two readers, and the second one is necessary.
// bin/le does not exist before the launcher builds it. The bootstrap therefore
// cannot ask a Go helper which tags to compile itself with. feature-tags.sh is
// that shell reader. This file holds it to the same answer as readTags.

package featuretags

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/lepath"
)

// shellTags answers what feature-tags.sh reads out of the manifest under root,
// joined with separator. The launchers call it exactly this way.
func shellTags(t *testing.T, checkout, root, separator string) (string, error) {
	t.Helper()

	script := ". \"$1\"/feature-tags.sh && feature_tags \"$2\"/feature-gates.txt \"$3\""
	// A shell is the SUBJECT here. This test runs the launchers' own reader, so
	// the interpreter is the thing under test rather than a way to reach one.
	cmd := exec.CommandContext(t.Context(), "/bin/sh", "-c", script, "sh", checkout, root, separator)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// writeManifest puts a feature-gates.txt under a fresh root and answers it.
func writeManifest(t *testing.T, body string) string {
	t.Helper()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "feature-gates.txt"), []byte(body), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return root
}

// VALIDATES: feature-tags.sh and readTags answer the same gate tags, in the same order.
// PREVENTS: a launcher that builds a binary with a feature set the Go tooling never selects.
// TestTheShellWalkAndTheGoReaderAnswerTheSameTags holds the two readers to one
// answer. It reads the checkout's real manifest, and one carrying every shape
// the parse must handle. Those shapes are a comment, a blank line, a repeated
// tag, a line with no tag, and an order that is not sorted.
func TestTheShellWalkAndTheGoReaderAnswerTheSameTags(t *testing.T) {
	checkout, err := lepath.Root()
	if err != nil {
		t.Fatalf("find the checkout: %v", err)
	}

	roots := map[string]string{
		"the checkout's own manifest": checkout,
		"comments, blanks, repeats and an unsorted order": writeManifest(t, strings.Join([]string{
			"# ze_comment internal/comment",
			"",
			"ze_web    internal/component/web",
			"ze_bgp    internal/component/bgp",
			"ze_bgp    internal/component/bgp/rib",
			"   ",
			"ze_anomaly internal/plugins/anomaly",
			"",
		}, "\n")),
	}

	for name, root := range roots {
		t.Run(name, func(t *testing.T) {
			want, err := DaemonTags(root)
			if err != nil {
				t.Fatalf("DaemonTags: %v", err)
			}

			got, err := shellTags(t, checkout, root, ",")
			if err != nil {
				t.Fatalf("feature-tags.sh: %v", err)
			}
			if got != strings.Join(want, ",") {
				t.Errorf("the two readers disagree\nshell: %s\ngo:    %s", got, strings.Join(want, ","))
			}

			spaced, err := shellTags(t, checkout, root, " ")
			if err != nil {
				t.Fatalf("feature-tags.sh with a space separator: %v", err)
			}
			if spaced != strings.Join(want, " ") {
				t.Errorf("the separator is not the caller's\nshell: %s\ngo:    %s", spaced, strings.Join(want, " "))
			}
		})
	}
}

// VALIDATES: a manifest declaring no gate stops the shell reader as it already stops the Go reader.
// PREVENTS: a silent build of a daemon with every feature compiled out, which fails later as an unknown config keyword.
// TestAManifestWithNoGateStopsBothReaders proves the shell reader refuses the
// empty answer the Go reader already refuses. An empty tag list compiles every
// feature out, and a build that carries no feature is not distinguishable from
// one whose feature is broken.
func TestAManifestWithNoGateStopsBothReaders(t *testing.T) {
	checkout, err := lepath.Root()
	if err != nil {
		t.Fatalf("find the checkout: %v", err)
	}

	root := writeManifest(t, "# every line here is a comment\n\n")

	if _, err := DaemonTags(root); err == nil {
		t.Error("DaemonTags accepted a manifest declaring no gate")
	}
	if out, err := shellTags(t, checkout, root, ","); err == nil {
		t.Errorf("feature-tags.sh accepted a manifest declaring no gate, answering %q", out)
	}
}

// runsAShell reports whether a shell executes the named file. The set is the
// two launchers, an image recipe, a shell library, a make recipe, and a
// functional test with inline commands. Prose that quotes an awk is not a
// parser. The walk below therefore reads only these.
func runsAShell(name string) bool {
	switch name {
	case "ze", "le", "Makefile":
		return true
	}
	return strings.HasPrefix(name, "Dockerfile") ||
		strings.HasSuffix(name, ".sh") ||
		strings.HasSuffix(name, ".ci")
}

// VALIDATES: feature-tags.sh is the only shell file that walks feature-gates.txt.
// PREVENTS: a second awk in a launcher or an image, which drifts from the Go reader.
// TestOnlyOneShellFileParsesTheFeatureManifest walks the checkout for a second
// shell walk over the manifest. Before feature-tags.sh, the launchers and the
// images each carried their own awk. A copy that nothing compares is what lets
// one image ship a feature set another does not.
func TestOnlyOneShellFileParsesTheFeatureManifest(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("find the checkout: %v", err)
	}

	skipped := map[string]bool{
		".git": true, "bin": true, "cache": true, "node_modules": true,
		"tmp": true, "vendor": true, "backups": true, "third_party": true,
		"docs": true, "plan": true, "rfc": true, "website": true, "ai": true,
	}

	var offenders []string
	// A path this cannot read stops the walk rather than passing it. A check
	// that skips what it cannot read reports a clean tree it never looked at.
	walk := func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if skipped[entry.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if !runsAShell(entry.Name()) {
			return nil
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if rel == "feature-tags.sh" {
			return nil
		}

		body, readErr := os.ReadFile(path) //nolint:gosec // the test reads the checkout it found
		if readErr != nil {
			return readErr
		}
		for line := range strings.SplitSeq(string(body), "\n") {
			if strings.Contains(line, "feature-gates.txt") && strings.Contains(line, "awk") {
				offenders = append(offenders, rel)
				break
			}
		}
		return nil
	}

	if err := filepath.WalkDir(root, walk); err != nil {
		t.Fatalf("walk the checkout: %v", err)
	}
	if len(offenders) != 0 {
		t.Errorf("a second shell walk over feature-gates.txt lives in %v; call feature_tags from feature-tags.sh instead", offenders)
	}
}
