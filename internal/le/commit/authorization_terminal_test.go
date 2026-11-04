// Design: docs/contributing/committing.md -- terminal-only owner authorization issuance.
package commit

import (
	"errors"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/le/human"
	"github.com/ze-software/ze/internal/le/le/human/humantest"
)

// TestCommitAuthorizationTerminalBoundary reaches Create attest and preparePush
// with real terminals, pipes and closed streams; refusals may write nothing,
// including session directories and replacement message/script artifacts.
func TestCommitAuthorizationTerminalBoundary(t *testing.T) {
	for _, route := range []string{"attest", "push"} {
		t.Run(route, func(t *testing.T) {
			for _, mode := range []string{"terminal", "piped-stdin", "piped-stdout", "piped-both", "closed-stdin", "closed-stdout"} {
				t.Run(mode, func(t *testing.T) {
					humantest.Streams(t, mode)
					for _, existing := range []bool{false, true} {
						name := "new"
						if existing {
							name = "replacement"
						}
						t.Run(name, func(t *testing.T) {
							root := newOwedRepository(t)
							t.Setenv("CLAUDE_CODE_SESSION_ID", "terminal-authorization-fixture")
							trailers := []string(nil)
							if route == "attest" {
								trailers = []string{"Review-owed: demo: no reviewer"}
							}
							owed := commitOwed(t, root, "the change", map[string]string{"a.txt": "a\n"}, nil, trailers...)
							if existing {
								writeCommitFixture(t, root, "tmp/commit-unrelated.sh", "preserve this script\n")
								writeCommitFixture(t, root, "tmp/commit-msg-unrelated.txt", "preserve this message\n")
							}
							before := humantest.Snapshot(t, root)
							var err error
							var command string
							if route == "attest" {
								command = "./le commit create"
								prepared, problem := Create(root, &Options{Subject: "owner attests", Attest: owed,
									AttestWords: "Thomas reviewed this change"})
								err = problem
								if mode == "terminal" {
									if err != nil {
										t.Fatal(err)
									}
									message, err := os.ReadFile(filepath.Join(root, prepared.Message))
									if err != nil || !strings.Contains(string(message), "Owner-attested: ") {
										t.Fatalf("attestation message %q, %v", message, err)
									}
									return
								}
							} else {
								command = "./le commit push"
								prepared, code, problem := preparePush(root, pushTestAuthorisation)
								err = problem
								if mode == "terminal" {
									if err != nil || code != 0 {
										t.Fatalf("push preparation: %d, %v", code, err)
									}
									if !strings.Contains(readScript(t, root, prepared.Script), pushTestAuthorisation) {
										t.Fatal("push script omitted the owner's instruction")
									}
									return
								}
								if code != 2 {
									t.Errorf("terminal refusal exit = %d, want 2", code)
								}
							}
							var refusal *human.Refusal
							if !errors.As(err, &refusal) {
								t.Errorf("%s answered %v, want terminal refusal", route, err)
							} else if strings.Contains(err.Error(), "full verification") || !strings.Contains(err.Error(), command) {
								t.Errorf("authorization refusal = %v", err)
							}
							if !maps.Equal(before, humantest.Snapshot(t, root)) {
								t.Error("refused authorization changed files or directories")
							}
						})
					}
				})
			}
		})
	}
}

// TestPreparedPushScriptRemainsNoninteractive runs the real script renderer's
// output with piped streams and a recording git executable: no repository is
// pushed, and only the already-prepared command is consumed.
func TestPreparedPushScriptRemainsNoninteractive(t *testing.T) {
	root := t.TempDir()
	humantest.Streams(t, "piped-both")
	prepared := pushPrepared{Script: "tmp/push.sh", Target: strings.Repeat("a", 40), Remote: "origin", Branch: "refs/heads/main"}
	text := renderPushScript(root, "12345678", pushTestAuthorisation, &prepared)
	writeCommitFixture(t, root, prepared.Script, text)
	bin := filepath.Join(root, "bin")
	writeCommitFixture(t, root, "bin/git", "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$PUSH_CAPTURE\"\n")
	if err := os.Chmod(filepath.Join(bin, "git"), 0o750); err != nil { //nolint:gosec // executable test fixture, never a real push
		t.Fatal(err)
	}
	capture := filepath.Join(root, "push-arguments")
	command := exec.CommandContext(t.Context(), "bash", filepath.Join(root, prepared.Script))
	command.Dir = root
	command.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"), "PUSH_CAPTURE="+capture)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("prepared script under pipes: %v\n%s", err, output)
	}
	arguments, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	want := "push\norigin\n" + prepared.Target + ":" + prepared.Branch + "\n"
	if string(arguments) != want {
		t.Fatalf("prepared push argv = %q, want %q", arguments, want)
	}
}
