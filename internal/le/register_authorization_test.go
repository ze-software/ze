// Design: docs/contributing/running-commands.md -- authorization precedes every root startup writer.
package le

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/le/human/humantest"
)

const authorizationRootChild = "ZE_TEST_LE_AUTHORIZATION_ROOT_CHILD"
const authorizationRootSession = "root-authorization-fixture"

// TestRootAuthorizationPrecedesStartupWrites enters the complete composition
// root in a fresh process and cold root for each issuance route, with neither
// stdin nor stdout a terminal. Every directory, file, mode and byte must survive
// refusal, not merely the final approval artifact. Repairs are decoded only;
// neither an owner action nor the printed repair is executed.
func TestRootAuthorizationPrecedesStartupWrites(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"rfc-owner-ruling", []string{"rfc", "approve", "unit", "pkg.TestX", "reason", "owner ruling 2026-11-03 accepts this fixture"}},
		{"commit-attestation", []string{"commit", "create", "subject", "owner authorization fixture", "attest", strings.Repeat("1", 40), "Thomas reviewed this fixture"}},
		{"push-authorization", []string{"commit", "push", "the owner ordered this push in the fixture"}},
		{"owner-review", []string{"spec", "review", "record", "spec", "demo", "verdict", "clean", "rounds", "1", "file", "pkg/a.go", "owner-authorised", "Thomas asked for this pass"}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := authorizationColdRoot(t)
			before := humantest.Snapshot(t, root)
			response := authorizationRootRun(t, root, 2, test.args)
			if !strings.Contains(strings.ToLower(response), "terminal") {
				t.Errorf("not a terminal refusal: %s", response)
			}
			if !maps.Equal(before, humantest.Snapshot(t, root)) {
				t.Error("authorization refusal changed the cold root before admission")
			}
			var repair string
			for line := range strings.SplitSeq(response, "\n") {
				if strings.HasPrefix(strings.TrimSpace(line), "env ") {
					repair = strings.TrimSpace(line)
				}
			}
			if repair == "" {
				t.Fatalf("refusal contains no context-preserving repair: %s", response)
			}
			arguments := humantest.RetryArguments(t, errors.New(repair))
			want := append([]string{"./le"}, test.args...)
			if !slices.Equal(arguments, want) {
				t.Errorf("repair argv = %q, want %q", arguments, want)
			}
			if got := os.Getenv("ZE_REPO_ROOT"); got != root {
				t.Errorf("repair root = %q, want %q", got, root)
			}
			if got := os.Getenv("CLAUDE_CODE_SESSION_ID"); got != authorizationRootSession {
				t.Errorf("repair session = %q, want raw harness %q", got, authorizationRootSession)
			}
		})
	}
}

// TestRootOrdinaryAuthorizationControlsStayNoninteractive proves the complete
// startup path still reaches the ordinary judge and review writers. Each
// control must publish its actual fixture artifact, not merely avoid refusal.
func TestRootOrdinaryAuthorizationControlsStayNoninteractive(t *testing.T) {
	cases := []struct {
		name, artifact string
		args           []string
	}{
		{"ordinary-judge", "| pkg.TestX | independent judge approved this fixture |", []string{"rfc", "approve", "unit", "pkg.TestX", "reason", "independent judge approved this fixture"}},
		{"ordinary-review", "ze-review spec=demo verdict=clean", []string{"spec", "review", "record", "spec", "demo", "verdict", "clean", "rounds", "1", "file", "pkg/a.go"}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := authorizationColdRoot(t)
			response := authorizationRootRun(t, root, 0, test.args)
			if strings.Contains(response, "requires a terminal") {
				t.Errorf("ordinary action acquired an owner gate: %s", response)
			}
			found := false
			err := filepath.WalkDir(filepath.Join(root, "tmp"), func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() {
					return nil
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				if bytes.Contains(body, []byte(test.artifact)) {
					found = true
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if !found {
				t.Errorf("ordinary action published no artifact containing %q: %s", test.artifact, response)
			}
		})
	}
}

// TestAuthorizationRootChild is the subprocess entry. It calls run, the handler
// registered for both native le and ze le, without replacing any startup seam.
func TestAuthorizationRootChild(t *testing.T) {
	if os.Getenv(authorizationRootChild) == "" {
		return
	}
	separator := slices.Index(os.Args, "--")
	if separator < 0 {
		t.Fatal("root child received no argument separator")
	}
	env.ResetCache()
	code := run(nil, os.Args[separator+1:])
	t.Log("ROOT-EXIT: " + strconv.Itoa(code))
}

func authorizationColdRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "cold authorization root")
	if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o750); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"pkg/a.go":      "package a\nfunc A() {}\n",
		"pkg/a_test.go": "package a\nimport \"testing\"\nfunc TestX(t *testing.T) {}\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func authorizationRootRun(t *testing.T, root string, want int, arguments []string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	args := []string{"-test.run=^TestAuthorizationRootChild$", "-test.v", "--"}
	child := exec.CommandContext(ctx, os.Args[0], append(args, arguments...)...)
	// Cmd's nil stdin is the null device, and CombinedOutput supplies pipes.
	// Override both supported repository spellings so inherited settings cannot
	// redirect the cold-root proof into another checkout.
	child.Env = append(os.Environ(), authorizationRootChild+"=1", "ZE_REPO_ROOT="+root,
		"ze.repo.root="+root, "CLAUDE_CODE_SESSION_ID="+authorizationRootSession,
		"ZE_AGENT_ID=", "ze.agent.id=")
	output, err := child.CombinedOutput()
	if err != nil {
		t.Fatalf("root child failed: %v\n%s", err, output)
	}
	response := string(output)
	if !strings.Contains(response, "ROOT-EXIT: "+strconv.Itoa(want)+"\n") {
		t.Errorf("root action did not return %d:\n%s", want, response)
	}
	return response
}
