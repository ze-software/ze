// Design: docs/architecture/testing/tracked-build-gate.md -- the vacuity guards, proved before use
//
// selftest.go proves the vacuity guards can still FAIL, before the gate is
// allowed to judge the live tree.
//
// It drives Build, the function that CONSUMES the guards, rather than the
// helpers it calls. A selftest over the helpers alone stays green when a case
// is deleted from Build's switch, which is exactly the edit that would disarm
// the gate.
//
// The table is declared ONCE and read twice: `le repository-tracked-build
// selftest` runs it, and the package test runs the same rows so a failure names
// the case rather than a count.

package trackedbuild

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/leroot"
)

// selftestDeadline bounds the whole selftest. It compiles a one-package module
// twice and creates a throwaway git repository, all of which are seconds.
const selftestDeadline = time.Minute

// fixtureFiles is the module every build case is pointed at: one package, one
// file the probe tag selects and one it does not.
var fixtureFiles = map[string]string{
	"go.mod":             "module example.invalid/selftest\n\ngo 1.21\n",
	"feature-gates.txt":  "ze_probe\tinternal/probe\n",
	"cmd/probe/main.go":  "package main\n\nfunc main() {}\n",
	"cmd/probe/gated.go": "//go:build ze_probe\n\npackage main\n\nfunc gated() {}\n",
}

// okFlavor is the coherent flavor: its tags select its anchor file.
var okFlavor = Flavor{
	Name: "selftest-ok", Tags: []string{"ze_probe"},
	Anchor: "./cmd/probe", AnchorFiles: []string{"gated.go"},
	Why: "the coherent case, which is what tells a working guard from one that always fails",
}

// selftestEnv is what every case is handed: the fixture module, a throwaway git
// repository, and the deadline they all run under.
type selftestEnv struct {
	ctx context.Context
	// dir is the fixture module. It holds go.mod and no vendor/, which is what
	// makes it a partial extraction against the probe repository.
	dir string
	// bare is an empty directory: no go.mod, no feature manifest.
	bare string
	// probe is a git repository whose one commit tracks vendor/modules.txt.
	probe string
}

// selftestCase is one property the gate must have. check answers the empty
// string when it holds, and what the failure means otherwise.
type selftestCase struct {
	name  string
	check func(env selftestEnv) string
}

// selftestCases is the whole selftest.
var selftestCases = []selftestCase{
	{
		name: "coherent-flavor-builds",
		check: func(env selftestEnv) string {
			result := Build(env.ctx, env.dir, okFlavor, nil, 1)
			if result.OK {
				return ""
			}
			var tb textbuf.Buffer
			return tb.Str("build refused a coherent flavor: ").Str(result.Output).String()
		},
	},
	{
		name: "vacuous-tags-refused",
		check: func(env selftestEnv) string {
			// Same tree, a tag that selects nothing. `go build ./...` still
			// exits 0 over it, so this is the fail-open the anchor guard exists
			// to close.
			vacuous := Flavor{
				Name: "selftest-vacuous", Tags: []string{"ze_absent"},
				Anchor: "./cmd/probe", AnchorFiles: []string{"gated.go"}, Why: "the vacuous case",
			}
			result := Build(env.ctx, env.dir, vacuous, nil, 1)
			if result.OK {
				return "build accepted a flavor whose tags select none of its anchor files"
			}
			if strings.Contains(result.Output, "gated.go") {
				return ""
			}
			var tb textbuf.Buffer
			return tb.Str("the anchor failure does not name the unselected file: ").Str(result.Output).String()
		},
	},
	{
		name: "package-floor-refuses",
		check: func(env selftestEnv) string {
			if Build(env.ctx, env.dir, okFlavor, nil, 99).OK {
				return "build accepted a tree below the package floor"
			}
			return ""
		},
	},
	{
		name: "feature-manifest-required",
		check: func(env selftestEnv) string {
			// A tree with no manifest must be refused rather than answered with
			// an empty tag set, which would build every flavor feature-free.
			if _, err := featureTags(env.bare); err == nil {
				return "FeatureTags accepted a tree with no feature-gates.txt"
			}
			return ""
		},
	},
	{
		name: "go-mod-required",
		check: func(env selftestEnv) string {
			if err := sanityCheck(env.ctx, env.dir, "HEAD", env.bare); err == nil {
				return "SanityCheck accepted a tree with no go.mod"
			}
			return ""
		},
	},
	{
		name: "missing-anchor-package",
		check: func(env selftestEnv) string {
			// Deleting Build's `case anchorErr != nil` leaves `missing` nil, so
			// the switch falls through the file check, past the floor (the
			// ./... count is real) and into `default: result.OK = true`. That
			// is a fail-open, and this is what refuses it.
			gone := Flavor{
				Name: "selftest-no-anchor", Tags: []string{"ze_probe"},
				Anchor: "./cmd/absent", AnchorFiles: []string{"main.go"}, Why: "the absent-anchor case",
			}
			result := Build(env.ctx, env.dir, gone, nil, 1)
			if result.OK {
				return "build accepted a flavor whose anchor package does not exist"
			}
			if strings.Contains(result.Output, "anchor package did not resolve") {
				return ""
			}
			var tb textbuf.Buffer
			return tb.Str("a missing anchor package was misdiagnosed: ").Str(result.Output).String()
		},
	},
	{
		name: "partial-extraction-refused",
		check: func(env selftestEnv) string {
			// The one branch no fixture repository can reach by accident,
			// because `git archive` always yields what the commit holds. The
			// probe repository is created for it rather than reusing the live
			// checkout: a selftest that read the caller's cwd, HEAD and
			// vendor/ would fail in a shallow clone, a non-vendored checkout,
			// or when run from another directory, none of which says anything
			// about the guard.
			tracked, err := commitHasPath(env.ctx, env.probe, "HEAD", "vendor/modules.txt")
			if err != nil {
				var tb textbuf.Buffer
				return tb.Str("CommitHasPath over the probe repository: ").Err(err).String()
			}
			if !tracked {
				return "CommitHasPath did not find vendor/modules.txt in a commit that holds it, " +
					"so the partial-extraction guard is disarmed"
			}
			absent, err := commitHasPath(env.ctx, env.probe, "HEAD", "no/such/path.txt")
			if err != nil {
				var tb textbuf.Buffer
				return tb.Str("CommitHasPath over an absent path: ").Err(err).String()
			}
			if absent {
				return "CommitHasPath reported a path the commit does not hold"
			}
			// env.dir holds go.mod but no vendor/, which is the partial
			// extraction.
			if err := sanityCheck(env.ctx, env.probe, "HEAD", env.dir); err == nil {
				return "SanityCheck accepted a tree with no vendor/ against a commit that tracks it"
			}
			return ""
		},
	},
}

// WriteFixture writes the fixture module, the empty tree and the probe git
// repository under root, and answers the environment the cases run in.
func WriteFixture(ctx context.Context, root string) (selftestEnv, error) {
	env := selftestEnv{
		ctx:   ctx,
		dir:   filepath.Join(root, "module"),
		bare:  filepath.Join(root, "bare"),
		probe: filepath.Join(root, "probe-repo"),
	}

	for rel, body := range fixtureFiles {
		if err := writeFile(filepath.Join(env.dir, rel), body); err != nil {
			return selftestEnv{}, err
		}
	}
	if err := os.MkdirAll(env.bare, 0o750); err != nil {
		return selftestEnv{}, err
	}
	if err := writeFile(filepath.Join(env.probe, "go.mod"), "module example.invalid/probe\n\ngo 1.21\n"); err != nil {
		return selftestEnv{}, err
	}
	if err := writeFile(filepath.Join(env.probe, "vendor", "modules.txt"), "# probe\n"); err != nil {
		return selftestEnv{}, err
	}

	for _, args := range [][]string{
		{"init", "--quiet"},
		{"add", "--", "go.mod", "vendor/modules.txt"},
		{"commit", "--quiet", "-m", "probe"},
	} {
		cmd := exec.CommandContext(ctx, "git", append([]string{"-C", env.probe}, args...)...) //nolint:gosec // fixed argument list
		// No global or system git config, so the probe inherits no commit
		// signing, which would fail in a throwaway repository.
		// textbuf rather than `+`: `performance.md` bans building strings by
		// concatenation, and c_string_concat enforces it on every compiled file.
		var gk, sk textbuf.Buffer
		cmd.Env = append(os.Environ(),
			gk.Str("GIT_CONFIG_GLOBAL=").Str(os.DevNull).String(),
			sk.Str("GIT_CONFIG_SYSTEM=").Str(os.DevNull).String(),
			"GIT_AUTHOR_NAME=selftest", "GIT_AUTHOR_EMAIL=selftest@example.invalid",
			"GIT_COMMITTER_NAME=selftest", "GIT_COMMITTER_EMAIL=selftest@example.invalid")
		if out, err := cmd.CombinedOutput(); err != nil {
			var tb textbuf.Buffer
			return selftestEnv{}, errors.New(tb.Str("probe repo git ").Join(args, " ").Str(": ").
				Err(err).Str(": ").Str(strings.TrimSpace(string(out))).String())
		}
	}
	return env, nil
}

// writeFile writes one fixture file, creating its directory.
func writeFile(path, body string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(body), 0o600)
}

// Selftest writes the fixtures and answers one row per case.
//
// The error is a fixture that could not be written, which is a different fact
// from a guard that stopped firing, so it is answered apart from the rows
// rather than as one more failing case.
func Selftest(root string) (leroot.SelftestReport, error) {
	ctx, cancel := context.WithTimeout(context.Background(), selftestDeadline)
	defer cancel()

	// Under the checkout's own scratch, never the system temp directory: this
	// repository keeps its scratch inside the tree so it is visible to the
	// operator.
	base := filepath.Join(root, "tmp")
	if err := os.MkdirAll(base, 0o750); err != nil {
		return leroot.SelftestReport{}, err
	}
	dir, err := os.MkdirTemp(base, "tracked-build-selftest")
	if err != nil {
		return leroot.SelftestReport{}, err
	}
	defer os.RemoveAll(dir) //nolint:errcheck // temp fixture

	fixture, err := WriteFixture(ctx, dir)
	if err != nil {
		return leroot.SelftestReport{}, err
	}

	results := make([]leroot.SelftestResult, 0, len(selftestCases))
	for _, testCase := range selftestCases {
		if detail := testCase.check(fixture); detail != "" {
			results = append(results, leroot.Fail(testCase.name, detail))
			continue
		}
		results = append(results, leroot.Pass(testCase.name))
	}

	return leroot.NewSelftestReport(
		"tracked-build: selftest OK",
		"tracked-build: SELFTEST FAILED:",
		results...,
	), nil
}

// runSelftest is the `le repository-tracked-build selftest` action.
func runSelftest() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := Selftest(root)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	// 2 rather than 1: the script answered 2 for a selftest failure, and a
	// caller that reads "the guards are disarmed" apart from "the commit does
	// not compile" keeps reading them apart.
	return report, report.Code(2)
}
