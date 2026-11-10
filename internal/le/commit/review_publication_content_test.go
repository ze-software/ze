// Design: docs/contributing/spec-workflow.md -- native publication reviews the content it carries.
package commit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestNativeReviewPublishesEarlierWholeAndHunkSnapshots records the actual
// immutable source of an earlier block, then leaves concurrent bytes on disk.
// The generated checker must review the earlier block's tree, not those bytes.
func TestNativeReviewPublishesEarlierWholeAndHunkSnapshots(t *testing.T) {
	for _, kind := range []string{"whole", "hunks"} {
		t.Run(kind, func(t *testing.T) {
			root, code, spec := runtimeReviewFixture(t)
			const selected = "#!/bin/sh\nprintf 'selected earlier bytes\\n'\n"
			const concurrent = selected + "# unrelated concurrent checkout bytes\n"
			options := &Options{Subject: "earlier reviewed content", SpecWork: "runtime-review"}
			if kind == "hunks" {
				patch := writeHunkPatch(t, root, code, selected, concurrent)
				options.Hunks = []HunkSelection{{Path: code, Patch: patch}}
			} else {
				writeCommitFixture(t, root, code, selected)
				options.Files = []string{code}
			}
			earlier, err := Create(root, options)
			if err != nil {
				t.Fatal(err)
			}
			script, err := os.ReadFile(filepath.Join(root, earlier.Script))
			if err != nil {
				t.Fatal(err)
			}
			blob := ""
			if err := walkScriptBlocks(earlier.Script, string(script), func(entry string) error {
				object, path, err := snapshotEntryFields(entry)
				if path == code {
					blob = object
				}
				return err
			}, func([]string) {}); err != nil {
				t.Fatal(err)
			}
			if blob == "" {
				t.Fatal("earlier block carries no selected source")
			}
			recordFixtureReview(t, root, "spec", "runtime-review", "verdict", "clean", "rounds", "1",
				"blob", code, blob, "mode", code, "100644")
			writeCommitFixture(t, root, code, concurrent)
			closure, err := Create(root, &Options{Subject: "close earlier reviewed work", Remove: []string{spec}, Append: true, NoNews: "fixture: no operator-visible change"})
			if err != nil {
				t.Fatal(err)
			}
			command := exec.CommandContext(t.Context(), "bash", filepath.Join(root, closure.Script))
			command.Dir = root
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("native %s review publication refused: %v\n%s", kind, err, output)
			}
			if got := runCommitGitOutput(t, root, "show", "HEAD:"+code); got != selected {
				t.Fatalf("published %q, want immutable selected bytes", got)
			}
			body, err := os.ReadFile(filepath.Join(root, code))
			if err != nil {
				t.Fatal(err)
			}
			if string(body) != concurrent {
				t.Fatalf("publication changed unrelated checkout bytes: %q", body)
			}
		})
	}
}

// TestNativeReviewPinsExecutableMode publishes an explicitly reviewed mode,
// while preserving a later checkout-only mode change.
func TestNativeReviewPinsExecutableMode(t *testing.T) {
	root, code, spec := runtimeReviewFixture(t)
	if err := os.Chmod(filepath.Join(root, code), 0o700); err != nil { //nolint:gosec // the reviewed executable mode is the fixture input
		t.Fatal(err)
	}
	blob := strings.TrimSpace(runCommitGitOutput(t, root, "hash-object", "-w", "--", code))
	recordFixtureReview(t, root, "spec", "runtime-review", "verdict", "clean", "rounds", "1",
		"blob", code, blob, "mode", code, "100755")
	prepared, err := Create(root, &Options{Subject: "close reviewed executable", Files: []string{code}, Remove: []string{spec}, NoNews: "fixture: no operator-visible change"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(root, code), 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(t.Context(), "bash", filepath.Join(root, prepared.Script))
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("native mode review failed: %v\n%s", err, output)
	}
	if got := runCommitGitOutput(t, root, "ls-tree", "HEAD", "--", code); !strings.HasPrefix(got, "100755 blob ") {
		t.Fatalf("published mode is not the explicitly reviewed executable mode: %q", got)
	}
	info, err := os.Stat(filepath.Join(root, code))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 != 0 {
		t.Fatalf("publication rewrote concurrent checkout mode: %v", info.Mode())
	}
}
