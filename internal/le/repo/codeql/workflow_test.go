// Design: docs/architecture/testing/ci-workflows.md -- CodeQL's admitted Go population.
package codeql

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	normalizedenv "github.com/ze-software/ze/internal/core/env"
	leroot "github.com/ze-software/ze/internal/le/le/root"
	// Dependencies register other areas. Link their real parent namespaces, as
	// the native composition and existing dispatcher fixtures do.
	_ "github.com/ze-software/ze/internal/le/ai"
	_ "github.com/ze-software/ze/internal/le/arch"
	_ "github.com/ze-software/ze/internal/le/build"
	_ "github.com/ze-software/ze/internal/le/chaos"
	_ "github.com/ze-software/ze/internal/le/cli"
	_ "github.com/ze-software/ze/internal/le/config"
	_ "github.com/ze-software/ze/internal/le/data"
	_ "github.com/ze-software/ze/internal/le/doc"
	_ "github.com/ze-software/ze/internal/le/go"
	_ "github.com/ze-software/ze/internal/le/lab"
	_ "github.com/ze-software/ze/internal/le/plugin"
	_ "github.com/ze-software/ze/internal/le/repo"
	_ "github.com/ze-software/ze/internal/le/setup/docker"
	_ "github.com/ze-software/ze/internal/le/site"
	_ "github.com/ze-software/ze/internal/le/spec"
	_ "github.com/ze-software/ze/internal/le/test"
	_ "github.com/ze-software/ze/internal/le/web"
	_ "github.com/ze-software/ze/internal/le/yang"
)

// TestWorkflowTrackedPopulation executes the actual workflow build body against
// tiny Git repositories. The compiler, Git index and dependency loader are real;
// the test-binary launcher dispatches the registered native action, not an echo.
func TestWorkflowTrackedPopulation(t *testing.T) {
	cases := []struct {
		name        string
		files       map[string]string
		link        bool
		wantFailure string
	}{
		{"cache is not compiled", map[string]string{
			"gokrazy/modcache/sentinel/cache.go": "package sentinel\nvar _ = codeqlCacheMustNotCompile\n",
		}, false, ""},
		{"new authored root is compiled", map[string]string{
			"newroot/sentinel.go": "//go:build ze_setup\n\npackage sentinel\nvar _ = codeqlAuthoredSentinel\n",
		}, false, "codeqlAuthoredSentinel"},
		{"authored gokrazy is compiled", map[string]string{
			"gokrazy/authored/sentinel.go": "//go:build ze_setup\n\npackage sentinel\nvar _ = codeqlGokrazySentinel\n",
		}, false, "codeqlGokrazySentinel"},
		{"unknown tracked form refuses", map[string]string{
			"newroot/.hidden.go": "package sentinel\n",
		}, false, "unclassified tracked Go"},
		{"canonical dependency into cache refuses", map[string]string{
			"authored/import.go":                 "package authored\nimport _ \"example.test/codeql/bridge\"\n",
			"gokrazy/modcache/dependency/dep.go": "package dependency\n",
		}, true, "gokrazy/modcache"},
	}
	body := workflowBuildBody(t)
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := fixtureRepository(t, test.files)
			if test.link {
				if err := os.Symlink("gokrazy/modcache/dependency", filepath.Join(root, "bridge")); err != nil {
					t.Fatal(err)
				}
			}
			installFixtureLauncher(t, root)
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			cmd := exec.CommandContext(ctx, "bash", "-euo", "pipefail", "-c", body)
			cmd.Dir = root
			cmd.Env = fixtureEnvironment(t, root)
			out, err := cmd.CombinedOutput()
			if test.wantFailure == "" {
				if err != nil {
					t.Fatalf("workflow must build authored packages without cache extraction: %v\n%s", err, out)
				}
				return
			}
			if err == nil {
				t.Fatalf("workflow admitted a population that must fail with %q", test.wantFailure)
			}
			if !strings.Contains(string(out), test.wantFailure) {
				t.Fatalf("workflow failed for the wrong reason, want %q: %v\n%s", test.wantFailure, err, out)
			}
		})
	}
}

// TestFixtureCLI lets the workflow invoke the real in-process registry from its
// fixture checkout. It does not replace Git, go list or go build.
func TestFixtureCLI(t *testing.T) {
	if os.Getenv("ZE_CODEQL_FIXTURE_CHILD") != "1" {
		return
	}
	for index, arg := range os.Args {
		if arg == "--" {
			os.Exit(leroot.Dispatch("le", os.Args[index+1:]))
		}
	}
	t.Fatal("fixture launcher did not supply its command boundary")
}

func fixtureRepository(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	fixtureWrite(t, root, "go.mod", "module example.test/codeql\n\ngo 1.27\n")
	fixtureWrite(t, root, "authored/base.go", "package authored\n")
	// These are intentionally invalid source, classified as vendor, fixture data
	// and a tool pin rather than buildable authored packages.
	fixtureWrite(t, root, "vendor/example.test/unused/unused.go", "not Go source\n")
	fixtureWrite(t, root, "testdata/broken.go", "not Go source\n")
	fixtureWrite(t, root, "tools.go", "//go:build tools\n\npackage tools\n")
	for path, content := range files {
		fixtureWrite(t, root, path, content)
	}
	fixtureGit(t, root, "init", "--quiet")
	fixtureGit(t, root, "add", "--", ".")
	return root
}

func fixtureWrite(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func fixtureGit(t *testing.T, root string, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func installFixtureLauncher(t *testing.T, root string) {
	t.Helper()
	path := filepath.Join(root, "le")
	body := "#!/bin/sh\nexec \"$ZE_CODEQL_FIXTURE_BINARY\" -test.run '^TestFixtureCLI$' -- \"$@\"\n"
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
}

func fixtureEnvironment(t *testing.T, root string) []string {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return append(normalizedenv.Without(os.Environ(), "ze.repo.root"), "ZE_REPO_ROOT="+root,
		"GOENV=off", "GOFLAGS=", "GOWORK=off",
		"ZE_CODEQL_FIXTURE_CHILD=1", "ZE_CODEQL_FIXTURE_BINARY="+binary)
}

func workflowBuildBody(t *testing.T) string {
	t.Helper()
	return workflowStepBody(t, "Build Go shipped feature combinations")
}

func workflowStepBody(t *testing.T, name string) string {
	t.Helper()
	for _, step := range workflowSteps(t) {
		if step.Name == name {
			return step.Run
		}
	}
	t.Fatalf("CodeQL workflow has no %q step", name)
	return ""
}

type workflowStep struct {
	Name string `yaml:"name"`
	Run  string `yaml:"run"`
}

func workflowSteps(t *testing.T) []workflowStep {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate the workflow fixture source")
	}
	path := filepath.Join(filepath.Dir(file), "../../../..", ".github/workflows/codeql.yml")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			Steps []workflowStep `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(source, &workflow); err != nil {
		t.Fatal(err)
	}
	return workflow.Jobs["analyze"].Steps
}
