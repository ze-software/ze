package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/le/speccitation"
)

// VALIDATES: scripts/dev/spec-citation-check.py and internal/le/speccitation emit
// the same bytes and code for every citation outcome, including the live tree.
// PREVENTS: the integration swap routing the Make gate to a native scanner with
// different baseline, warning, malformed-input, ordering or exit semantics.

func specCitationParityRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("repository root: %v", err)
	}
	return root
}

func specCitationParityFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "plan"), 0o750); err != nil {
		t.Fatalf("mkdir plan: %v", err)
	}
	for relative, body := range files {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", relative, err)
		}
	}
	return root
}

func specCitationParityPython(t *testing.T, root string, productionArgv bool) (string, int) {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is not on PATH")
	}
	var argv []string
	if productionArgv {
		argv = []string{"scripts/dev/spec-citation-check.py"}
	} else {
		repository := specCitationParityRoot(t)
		argv = []string{
			filepath.Join(repository, "scripts", "dev", "spec-citation-check.py"),
			"--repo",
			root,
		}
	}
	cmd := exec.CommandContext(t.Context(), "python3", argv...) //nolint:gosec // the retiring producer under comparison
	cmd.Dir = root
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("spec-citation-check.py: %v", err)
		}
		code = exit.ExitCode()
	}
	return string(output), code
}

func specCitationParityGo(t *testing.T, root string) (string, int) {
	t.Helper()
	report, err := speccitation.Scan(root)
	if err != nil {
		t.Fatalf("speccitation.Scan: %v", err)
	}
	code := 0
	if len(report.Dangling) > 0 {
		code = 1
	}
	return report.Text(), code
}

func specCitationParityEqual(t *testing.T, root, want string, wantCode int) {
	t.Helper()
	pythonOutput, pythonCode := specCitationParityPython(t, root, false)
	goOutput, goCode := specCitationParityGo(t, root)
	if pythonCode != wantCode {
		t.Errorf("Python code = %d, want %d\n%s", pythonCode, wantCode, pythonOutput)
	}
	if goCode != wantCode {
		t.Errorf("Go code = %d, want %d\n%s", goCode, wantCode, goOutput)
	}
	if pythonOutput != want {
		t.Errorf("Python output:\n%s\nwant:\n%s", pythonOutput, want)
	}
	if goOutput != want {
		t.Errorf("Go output:\n%s\nwant:\n%s", goOutput, want)
	}
}

func TestSpecCitationParityFixtures(t *testing.T) {
	failureSuffix := " which is absent on disk (not in baseline)\n"
	cases := []struct {
		name  string
		files map[string]string
		code  int
		want  string
	}{
		{
			name: "valid",
			files: map[string]string{
				"plan/spec-a.md": "See `plan/spec-b.md`.\n",
				"plan/spec-b.md": "Present.\n",
			},
			want: "spec-citation-check OK (2 specs, 0 baselined dangling)\n",
		},
		{
			name: "dangling",
			files: map[string]string{
				"plan/spec-a.md": "first\nSee `plan/spec-gone.md`.\n",
			},
			code: 1,
			want: "spec-citation-check FAILED: dangling plan/spec-*.md references\n" +
				"  plan/spec-a.md:2: references plan/spec-gone.md" + failureSuffix +
				"\n1 dangling reference(s). Either fix the citing reference, or -- if the target is legitimately gone -- add it to plan/.citation-baseline (or run spec-citation-check.py --write-baseline).\n",
		},
		{
			name: "baselined",
			files: map[string]string{
				"plan/spec-a.md":          "See `plan/spec-gone.md`.\n",
				"plan/.citation-baseline": "# known\nplan/spec-gone.md\n",
			},
			want: "spec-citation-check OK (1 specs, 1 baselined dangling)\n",
		},
		{
			name: "baseline growth",
			files: map[string]string{
				"plan/spec-a.md":          "`plan/spec-old.md` then `plan/spec-new.md`.\n",
				"plan/.citation-baseline": "plan/spec-old.md\n",
			},
			code: 1,
			want: "spec-citation-check FAILED: dangling plan/spec-*.md references\n" +
				"  plan/spec-a.md:1: references plan/spec-new.md" + failureSuffix +
				"\n1 dangling reference(s). Either fix the citing reference, or -- if the target is legitimately gone -- add it to plan/.citation-baseline (or run spec-citation-check.py --write-baseline).\n",
		},
		{
			name: "baseline shrink",
			files: map[string]string{
				"plan/spec-a.md":          "No references.\n",
				"plan/.citation-baseline": "# empty after cleanup\n",
			},
			want: "spec-citation-check OK (1 specs, 0 baselined dangling)\n",
		},
		{
			name: "token drift warning",
			files: map[string]string{
				"plan/spec-a.md": "The guard `oldToken` lives at `src/foo.go:2`.\n",
				"src/foo.go":     "package foo\nnewToken := 1\n",
			},
			want: "WARN plan/spec-a.md:1: citation `src/foo.go:2` no longer shows token `oldToken` on that line (line-token drift)\n" +
				"spec-citation-check OK (1 specs, 0 baselined dangling, 1 line-token WARN)\n",
		},
		{
			name: "malformed citation",
			files: map[string]string{
				"plan/spec-a.md": "The guard `oldToken` lives at `src/foo.go:two`.\n",
				"src/foo.go":     "package foo\nnewToken := 1\n",
			},
			want: "spec-citation-check OK (1 specs, 0 baselined dangling)\n",
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := specCitationParityFixture(t, test.files)
			specCitationParityEqual(t, root, test.want, test.code)
		})
	}
}

// Goal: prove exact parity over the population the Make gate reads. Method: run
// the Python producer with its registry argv and the Go scanner over one root.
func TestSpecCitationLiveTreeOutputAndCodeParity(t *testing.T) {
	root := specCitationParityRoot(t)
	pythonOutput, pythonCode := specCitationParityPython(t, root, true)
	goOutput, goCode := specCitationParityGo(t, root)
	if pythonCode != goCode {
		t.Fatalf("codes differ: Python=%d Go=%d\nPython:\n%s\nGo:\n%s", pythonCode, goCode, pythonOutput, goOutput)
	}
	if pythonOutput != goOutput {
		t.Errorf("live outputs differ\nPython:\n%s\nGo:\n%s", pythonOutput, goOutput)
	}
}
