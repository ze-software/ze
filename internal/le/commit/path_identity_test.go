// Design: docs/contributing/committing.md -- raw Git path identity through publication.
package commit

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestPreparedPublicationPreservesRawGitPaths drives whole-file snapshots and
// hunk selections through Create, script block decoding, publication, history
// enumeration and shared-index repair. The path is never Git's quoted spelling.
func TestPreparedPublicationPreservesRawGitPaths(t *testing.T) {
	for _, path := range []string{"space name.txt", "double\"quote.txt", "back\\slash.txt", "résumé.txt", " leading and trailing .txt "} {
		for _, selection := range []string{"whole file", "hunks"} {
			t.Run(path+"/"+selection, func(t *testing.T) {
				root := newCommitRepository(t)
				configureCommitAuthor(t, root)
				t.Setenv("CLAUDE_CODE_SESSION_ID", "raw-git-path-publication")
				writeCommitFixture(t, root, path, "before\n")
				runCommitGit(t, root, "add", "--", path)
				runCommitGit(t, root, "commit", "-q", "-m", "raw path baseline")
				writeCommitFixture(t, root, path, "selected\n")
				options := Options{Subject: "preserve exact path identity", SpecWork: "raw-path-proof"}
				if selection == "whole file" {
					options.Files = []string{path}
				} else {
					patch := runCommitGitOutput(t, root, "diff", "--", path)
					writeCommitFixture(t, root, "chosen.patch", patch)
					options.Hunks = []HunkSelection{{Path: path, Patch: "chosen.patch"}}
				}
				prepared, err := Create(root, &options)
				if err != nil {
					t.Fatalf("prepare %q: %v", path, err)
				}
				script, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(prepared.Script)))
				if err != nil {
					t.Fatal(err)
				}
				paths, err := scriptPaths(prepared.Script, string(script))
				if err != nil {
					t.Fatal(err)
				}
				if !slices.Equal(paths, []string{path}) {
					t.Fatalf("script paths = %q, want exact %q", paths, path)
				}
				writeCommitFixture(t, root, "foreign.txt", "peer staging\n")
				runCommitGit(t, root, "add", "--", "foreign.txt")
				writeCommitFixture(t, root, path, "selected\npeer edit remains\n")
				runCommitScript(t, root, prepared.Script)
				if got := runCommitGitOutput(t, root, "show", "HEAD:"+path); got != "selected\n" {
					t.Fatalf("published blob = %q, want selected bytes", got)
				}
				got, err := os.ReadFile(filepath.Join(root, path))
				if err != nil {
					t.Fatal(err)
				}
				if string(got) != "selected\npeer edit remains\n" {
					t.Fatalf("publication changed another session's edit: %q", got)
				}
				if staged := runCommitGitOutput(t, root, "diff", "--cached", "--name-only", "-z"); staged != "foreign.txt\x00" {
					t.Fatalf("shared index = %q, want only foreign.txt", staged)
				}
				population, err := specWorkPopulation(root, "raw-path-proof", "HEAD")
				if err != nil {
					t.Fatal(err)
				}
				if !slices.Equal(population.Paths, []string{path}) {
					t.Fatalf("Spec-work history paths = %q, want exact %q", population.Paths, path)
				}
			})
		}
	}
}

// TestRawGitPathRemovalPreservesConcurrentBytes exercises the removal decoder
// and post-publication guard for raw names, including a same-content mode edit.
func TestRawGitPathRemovalPreservesConcurrentBytes(t *testing.T) {
	for _, path := range []string{"double\"quote.txt", "back\\slash.txt", "résumé.txt"} {
		for _, drift := range []string{"unchanged", "bytes", "mode"} {
			t.Run(path+"/"+drift, func(t *testing.T) {
				root := newCommitRepository(t)
				configureCommitAuthor(t, root)
				t.Setenv("CLAUDE_CODE_SESSION_ID", "raw-git-path-removal")
				writeCommitFixture(t, root, path, "remove only these bytes\n")
				runCommitGit(t, root, "add", "--", path)
				runCommitGit(t, root, "commit", "-q", "-m", "raw removal baseline")
				prepared, err := Create(root, &Options{Subject: "remove exact raw path", Remove: []string{path}})
				if err != nil {
					t.Fatal(err)
				}
				if drift == "bytes" {
					writeCommitFixture(t, root, path, "another session's replacement\n")
				}
				if drift == "mode" {
					if err := os.Chmod(filepath.Join(root, path), 0o700); err != nil { //nolint:gosec // executable mode is the concurrent change this fixture preserves
						t.Fatal(err)
					}
				}
				runCommitScript(t, root, prepared.Script)
				if code, _ := gitExit(root, "cat-file", "-e", "HEAD:"+path); code == 0 {
					t.Fatal("removed raw path remains in published tree")
				}
				info, err := os.Stat(filepath.Join(root, path))
				if drift == "unchanged" {
					if !os.IsNotExist(err) {
						t.Fatalf("unchanged removed path remains: %v", err)
					}
					return
				}
				if err != nil {
					t.Fatalf("concurrent %s change was deleted: %v", drift, err)
				}
				if drift == "mode" && info.Mode().Perm()&0o111 == 0 {
					t.Fatal("concurrent executable mode was lost")
				}
				if drift == "bytes" {
					body, err := os.ReadFile(filepath.Join(root, path))
					if err != nil {
						t.Fatal(err)
					}
					if !strings.Contains(string(body), "another session's replacement") {
						t.Fatalf("concurrent bytes changed: %q", body)
					}
				}
			})
		}
	}
}
