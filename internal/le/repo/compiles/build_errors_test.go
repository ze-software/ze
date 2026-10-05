// Design: docs/architecture/testing/tracked-build-gate.md -- failures remain diagnosable.
// Goal: distinguish process failures and timeouts from the guards being tested.
// Method: drive Build and the selftest case runner with isolated fixtures and expired contexts.
package repocompiles

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestBuildReportsAnExpiredDeadline keeps a silent exec failure visible without sleeping.
func TestBuildReportsAnExpiredDeadline(t *testing.T) {
	env := fixture(t)
	ctx, cancel := context.WithDeadline(t.Context(), time.Unix(0, 0))
	defer cancel()
	result := Build(ctx, env.dir, okFlavor, nil, 1)
	if result.OK {
		t.Fatal("an expired build reported success")
	}
	if !strings.Contains(result.Output, context.DeadlineExceeded.Error()) {
		t.Fatalf("lost deadline diagnostic: %q", result.Output)
	}
	if !strings.Contains(result.Output, "go build:") {
		t.Fatalf("lost failed operation: %q", result.Output)
	}
}

// TestBuildReportsSilentExecutionFailures covers startup and nonzero exit with no output.
func TestBuildReportsSilentExecutionFailures(t *testing.T) {
	for _, silentExit := range []bool{false, true} {
		name := "missing-command"
		if silentExit {
			name = "silent-exit"
		}
		t.Run(name, func(t *testing.T) {
			bin := t.TempDir()
			if silentExit {
				if err := os.WriteFile(filepath.Join(bin, "go"), []byte("#!/bin/sh\nexit 7\n"), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", bin)
			ctx, cancel := context.WithTimeout(t.Context(), selftestDeadline)
			defer cancel()
			result := Build(ctx, t.TempDir(), okFlavor, nil, 1)
			if result.OK {
				t.Fatal("failed process reported success")
			}
			want := "executable file not found"
			if silentExit {
				want = "exit status 7"
			}
			if !strings.Contains(result.Output, want) {
				t.Fatalf("failure = %q, want %q", result.Output, want)
			}
		})
	}
}

// TestBuildRetainsCompilerDiagnostics preserves the underlying source error beside the exec error.
func TestBuildRetainsCompilerDiagnostics(t *testing.T) {
	env := fixture(t)
	if err := writeFile(filepath.Join(env.dir, "cmd/probe/broken.go"), "package main\nvar broken = missingProducer\n"); err != nil {
		t.Fatal(err)
	}
	result := Build(env.ctx, env.dir, okFlavor, nil, 1)
	if result.OK {
		t.Fatal("broken source compiled")
	}
	for _, want := range []string{"go build:", "undefined: missingProducer"} {
		if !strings.Contains(result.Output, want) {
			t.Errorf("failure = %q, missing %q", result.Output, want)
		}
	}
}

// TestBuildOwnsItsModuleEnvironment proves flags and workspaces cannot change the fixture's build.
func TestBuildOwnsItsModuleEnvironment(t *testing.T) {
	env := fixture(t)
	goenv := filepath.Join(t.TempDir(), "goenv")
	if err := writeFile(goenv, "GOFLAGS=-invalid-build-fixture-flag\n"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOENV", goenv)
	t.Setenv("GOFLAGS", "-invalid-build-fixture-flag")
	t.Setenv("GOWORK", filepath.Join(env.dir, "absent.work"))
	t.Setenv("GOTOOLCHAIN", "local")
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOSUMDB", "off")
	result := Build(env.ctx, env.dir, okFlavor, nil, 1)
	if !result.OK {
		t.Fatalf("ambient settings changed the fixture build: %s", result.Output)
	}
}

// TestSelftestCasesOwnTheirDeadlines proves one finished case cannot expire a later HEAD query.
func TestSelftestCasesOwnTheirDeadlines(t *testing.T) {
	env := fixture(t)
	var previous context.Context
	first := runSelftestCase(t.Context(), env, selftestCase{
		name: "first",
		check: func(current selftestEnv) string {
			previous = current.ctx
			if _, ok := previous.Deadline(); !ok {
				return "case has no deadline"
			}
			return ""
		},
	})
	if !first.Passed {
		t.Fatalf("first case: %+v", first)
	}
	if !errors.Is(previous.Err(), context.Canceled) {
		t.Fatalf("case did not release its context: %v", previous.Err())
	}
	second := runSelftestCase(t.Context(), env, selftestCase{
		name: "later-head-query",
		check: func(current selftestEnv) string {
			if current.ctx == previous {
				return "case reused its predecessor's context"
			}
			tracked, err := commitHasPath(current.ctx, current.probe, "HEAD", "vendor/modules.txt")
			if err != nil {
				return err.Error()
			}
			if !tracked {
				return "later query lost the committed probe file"
			}
			return ""
		},
	})
	if !second.Passed {
		t.Fatalf("second case: %+v", second)
	}
}

// TestSelftestCannotCountATimeoutAsAnExpectedRefusal rejects a negative probe's false pass.
func TestSelftestCannotCountATimeoutAsAnExpectedRefusal(t *testing.T) {
	ctx, cancel := context.WithDeadline(t.Context(), time.Unix(0, 0))
	defer cancel()
	result := runSelftestCase(ctx, selftestEnv{}, selftestCase{
		name: "expired-negative-probe",
		check: func(current selftestEnv) string {
			if current.ctx.Err() == nil {
				return "probe unexpectedly received a live context"
			}
			return ""
		},
	})
	if result.Passed {
		t.Fatal("timeout counted as a passing negative probe")
	}
	if !strings.Contains(result.Detail, context.DeadlineExceeded.Error()) {
		t.Fatalf("timeout not named: %+v", result)
	}
}

// TestFixtureSetupRejectsAnExpiredContext proves fixture creation is bounded separately.
func TestFixtureSetupRejectsAnExpiredContext(t *testing.T) {
	ctx, cancel := context.WithDeadline(t.Context(), time.Unix(0, 0))
	defer cancel()
	_, err := WriteFixture(ctx, t.TempDir())
	if err == nil {
		t.Fatal("expired setup created a committed fixture")
	}
	if !strings.Contains(err.Error(), context.DeadlineExceeded.Error()) {
		t.Fatalf("setup lost deadline error: %v", err)
	}
}

// TestFixtureSetupIgnoresInheritedGitState creates a real HEAD despite foreign Git overrides.
func TestFixtureSetupIgnoresInheritedGitState(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GIT_DIR", filepath.Join(root, "foreign.git"))
	t.Setenv("GIT_WORK_TREE", filepath.Join(root, "foreign-tree"))
	t.Setenv("GIT_INDEX_FILE", filepath.Join(root, "foreign-index"))
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "commit.gpgSign")
	t.Setenv("GIT_CONFIG_VALUE_0", "true")
	ctx, cancel := context.WithTimeout(t.Context(), selftestDeadline)
	defer cancel()
	fixture, err := WriteFixture(ctx, filepath.Join(root, "fixture"))
	if err != nil {
		t.Fatalf("isolated setup: %v", err)
	}
	cmd := exec.CommandContext(ctx, "git", "-C", fixture.probe, "ls-tree", "--name-only", "HEAD", "--", "vendor/modules.txt")
	cmd.Env = selftestGitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("read isolated HEAD: %v: %s", err, out)
	}
	if strings.TrimSpace(string(out)) != "vendor/modules.txt" {
		t.Fatalf("probe commit lost its tracked file: %q", out)
	}
	if _, err := os.Stat(filepath.Join(root, "foreign-index")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("setup touched the caller's index: %v", err)
	}
}

// TestBuildLinksTheCommittedTree proves extraction does not borrow a working-tree link fix.
func TestBuildLinksTheCommittedTree(t *testing.T) {
	fixture := fixture(t)
	const brokenMain = "package main\nimport _ \"unsafe\"\n" +
		"//go:linkname missing example.invalid/missing.symbol\n" +
		"func missing()\nfunc main() { missing() }\n"
	if err := writeFile(filepath.Join(fixture.probe, "cmd/probe/main.go"), brokenMain); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(filepath.Join(fixture.probe, "cmd/probe/gated.go"), fixtureFiles["cmd/probe/gated.go"]); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"add", "--", "cmd/probe"},
		{"-c", "core.hooksPath=" + os.DevNull, "commit", "--quiet", "-m", "unresolved link"},
	} {
		cmd := exec.CommandContext(fixture.ctx, "git", append([]string{"-C", fixture.probe}, args...)...)
		cmd.Env = selftestGitEnv()
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("commit link fixture: %v: %s", err, out)
		}
	}
	if err := writeFile(filepath.Join(fixture.probe, "cmd/probe/main.go"), fixtureFiles["cmd/probe/main.go"]); err != nil {
		t.Fatal(err)
	}
	working := Build(fixture.ctx, fixture.probe, okFlavor, nil, 1)
	if !working.OK {
		t.Fatalf("working-tree link fix is not coherent: %s", working.Output)
	}
	dest := t.TempDir()
	if err := extract(fixture.ctx, fixture.probe, "HEAD", dest); err != nil {
		t.Fatalf("extract committed link fixture: %v", err)
	}
	committed := Build(fixture.ctx, dest, okFlavor, nil, 1)
	if committed.OK {
		t.Fatal("committed unresolved link borrowed the working-tree fix")
	}
	if !strings.Contains(committed.Output, "relocation target example.invalid/missing.symbol not defined") {
		t.Fatalf("failure was not the committed unresolved link: %s", committed.Output)
	}
}

// TestBuildIgnoresBrokenEnclosingVCS drives every selftest case inside a real
// repository whose index is corrupted after a successful status control. The
// unrelated index must neither refuse coherent Go nor mask a vacuity diagnostic.
func TestBuildIgnoresBrokenEnclosingVCS(t *testing.T) {
	env := fixture(t)
	env.dir = filepath.Join(env.probe, "extracted")
	for rel, body := range fixtureFiles {
		if err := writeFile(filepath.Join(env.dir, rel), body); err != nil {
			t.Fatal(err)
		}
	}

	control := exec.CommandContext(env.ctx, "git", "-C", env.probe, "status", "--porcelain")
	control.Env = selftestGitEnv()
	if out, err := control.CombinedOutput(); err != nil {
		t.Fatalf("healthy enclosing repository: %v: %s", err, out)
	}
	if err := writeFile(filepath.Join(env.probe, ".git", "index"), "invalid Git index\n"); err != nil {
		t.Fatal(err)
	}
	broken := exec.CommandContext(env.ctx, "git", "-C", env.probe, "status", "--porcelain")
	broken.Env = selftestGitEnv()
	out, err := broken.CombinedOutput()
	if err == nil {
		t.Fatal("corrupt index did not break the enclosing repository's status")
	}
	if !strings.Contains(string(out), "index") {
		t.Fatalf("status failed for a reason other than the corrupt index: %v: %s", err, out)
	}
	if err := env.ctx.Err(); err != nil {
		t.Fatalf("index failure was not judged before the deadline: %v", err)
	}

	for _, testCase := range selftestCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := runSelftestCase(t.Context(), env, testCase)
			if !result.Passed {
				t.Fatal(result.Detail)
			}
		})
	}
}
