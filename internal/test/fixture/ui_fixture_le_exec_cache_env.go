// Design: docs/guide/developer-setup.md -- le exec: the launcher the tool shims run
// Related: register_le_exec_cache_env.go -- the registration
// Related: ui_fixture_le_store_trim_answers.go -- the built-le fixture this one follows

package fixture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	leexec "github.com/ze-software/ze/internal/le/exec"
	"github.com/ze-software/ze/internal/le/scratch"
)

// leExecCacheVariables are the four variables `le exec` points under the
// per-user cache, with the directory each one names there.
var leExecCacheVariables = [...][2]string{
	{"GOCACHE", "go-cache"},
	{"GOPLSCACHE", "gopls"},
	{"GOLANGCI_LINT_CACHE", "golangci-lint-cache"},
	{"STATICCHECK_CACHE", "staticcheck-matrix"},
}

// leExecProbe is the program every step runs: it prints its environment.
const leExecProbe = "/usr/bin/env"

// leExecCacheEnv proves `le exec` through the built le binary. It builds le,
// lays out a throwaway checkout (go.mod, feature-gates.txt and the stdpatch
// patches, enough for le to activate a patched GOROOT of its own) and a
// throwaway per-user cache, then runs `le exec /usr/bin/env`:
//
//   - inside the checkout: the four cache variables under the throwaway cache,
//     and GOROOT the patched tree le activated (AC-7);
//   - from a directory outside it: the same four variables, and GOROOT exactly
//     what the caller had, set or unset, with no patched tree on PATH (AC-8);
//   - on `sh -c 'exit 7'`: le's exit status is the program's;
//   - on a shim and on a missing path: exit 1, and a message naming the path
//     and ./le setup, with nothing run (AC-9).
//
// ZE_REPO_ROOT names the throwaway checkout in every step, so the outside step
// still activates the tree, as a shim started from another repository does
// (le finds its own checkout from where it lives). The real checkout and the
// real per-user cache are never named to the le it runs.
func leExecCacheEnv(ctx context.Context) error {
	source, err := leExecSource()
	if err != nil {
		return err
	}
	work, err := os.MkdirTemp("", "le-exec-")
	if err != nil {
		return leExecFailf("create fixture directory: %v", err)
	}
	defer os.RemoveAll(work) //nolint:errcheck // fixture cleanup

	work, err = filepath.EvalSymlinks(work)
	if err != nil {
		return leExecFailf("resolve fixture directory: %v", err)
	}
	lePath, err := leStoreTrimBuild(ctx, source, work)
	if err != nil {
		return err
	}
	checkout := filepath.Join(work, "checkout")
	if err := leExecCheckout(source, checkout); err != nil {
		return err
	}
	outside := filepath.Join(work, "outside")
	if err := os.MkdirAll(outside, 0o750); err != nil {
		return leExecFailf("create the outside directory: %v", err)
	}
	callerGOROOT, err := leExecRealGOROOT(ctx, source)
	if err != nil {
		return err
	}

	perUser := filepath.Join(work, "xdg")
	base := childEnvironment(withoutEnv(envRootedAt(checkout), "GOROOT", "GOTOOLCHAIN"), map[string]string{
		leCacheLinkXDGKey:    perUser,
		"PATH":               leExecPathWithoutTrees(os.Getenv("PATH")),
		scratch.StoreTrimKey: "off",
	})
	withCaller := childEnvironment(base, map[string]string{"GOROOT": callerGOROOT})
	target, err := scratch.CacheTarget(base)
	if err != nil {
		return leExecFailf("derive the per-user cache: %v", err)
	}

	inside, err := leExecEnvironment(ctx, lePath, checkout, withCaller)
	if err != nil {
		return err
	}
	if err := leExecCaches(inside, target, "inside the checkout"); err != nil {
		return err
	}
	if !leExecIsTree(inside["GOROOT"], work) {
		return leExecFailf("inside the checkout GOROOT = %q, want the patched tree le activated under %s", inside["GOROOT"], checkout)
	}

	for _, step := range []struct {
		name    string
		environ []string
		goroot  string
		set     bool
	}{
		{name: "outside, caller GOROOT set", environ: withCaller, goroot: callerGOROOT, set: true},
		{name: "outside, caller GOROOT unset", environ: base, set: false},
	} {
		got, err := leExecEnvironment(ctx, lePath, outside, step.environ)
		if err != nil {
			return err
		}
		if err := leExecCaches(got, target, step.name); err != nil {
			return err
		}
		goroot, set := got["GOROOT"]
		if set != step.set {
			return leExecFailf("%s: GOROOT set is %v (%q), want the caller's set %v", step.name, set, goroot, step.set)
		}
		if goroot != step.goroot {
			return leExecFailf("%s: GOROOT = %q, want the caller's %q", step.name, goroot, step.goroot)
		}
		if _, set := got["GOTOOLCHAIN"]; set {
			return leExecFailf("%s: GOTOOLCHAIN = %q, the caller set none", step.name, got["GOTOOLCHAIN"])
		}
		for _, entry := range filepath.SplitList(got["PATH"]) {
			if leExecIsTree(filepath.Dir(entry), work) {
				return leExecFailf("%s: PATH still carries the patched tree's bin %s", step.name, entry)
			}
		}
	}

	status, err := leExecRun(ctx, lePath, outside, withCaller, "exec", "/bin/sh", "-c", "exit 7")
	if err != nil {
		return err
	}
	if status.exitCode != 7 {
		return leExecFailf("le exec /bin/sh -c 'exit 7' exited %d, want the program's 7: %s", status.exitCode, status.stderr)
	}

	if err := leExecRefusals(ctx, lePath, work, withCaller); err != nil {
		return err
	}
	var verdict textbuf.Buffer
	return verdict.Str("OK\n").StdOut()
}

// leExecRefusals runs `le exec` on a shim and on a missing path, and asserts
// exit 1 with the path and ./le setup named and nothing run: each target would
// print a marker on stdout if it ran.
func leExecRefusals(ctx context.Context, lePath, work string, environ []string) error {
	shim := filepath.Join(work, "shims", "gopls")
	if err := os.MkdirAll(filepath.Dir(shim), 0o750); err != nil {
		return leExecFailf("create the shim directory: %v", err)
	}
	body := "#!/bin/sh\n" + leexec.ShimMarker + "\necho SHIM-RAN\n"
	if err := os.WriteFile(shim, []byte(body), 0o700); err != nil { //nolint:gosec // G306: a shim has to be executable
		return leExecFailf("write the shim: %v", err)
	}
	missing := filepath.Join(work, "gone", "gopls")
	for _, path := range []string{shim, missing} {
		result, err := leExecRun(ctx, lePath, work, environ, "exec", path, "version")
		if err != nil {
			return err
		}
		if result.exitCode != 1 {
			return leExecFailf("le exec %s exited %d, want 1:\n%s", path, result.exitCode, result.stderr)
		}
		if result.stdout != "" {
			return leExecFailf("le exec %s ran something: stdout %q", path, result.stdout)
		}
		for _, want := range []string{path, "./le setup"} {
			if !strings.Contains(result.stderr, want) {
				return leExecFailf("le exec %s: the refusal does not name %q:\n%s", path, want, result.stderr)
			}
		}
	}
	return nil
}

// leExecSource answers the checkout le is built from.
func leExecSource() (string, error) {
	source := os.Getenv("ZE_REPO_ROOT")
	if source == "" {
		return "", leExecFailf("ZE_REPO_ROOT is not set")
	}
	source, err := filepath.Abs(source)
	if err != nil {
		return "", leExecFailf("resolve ZE_REPO_ROOT: %v", err)
	}
	return source, nil
}

// leExecCheckout lays out the smallest checkout le activates a patched GOROOT
// for: the two markers lepath looks for, and the patches with go.mod's pin.
func leExecCheckout(source, checkout string) error {
	patchDir := filepath.Join("internal", "le", "go", "stdpatch", "patches")
	if err := os.MkdirAll(filepath.Join(checkout, patchDir), 0o750); err != nil {
		return leExecFailf("create the throwaway checkout: %v", err)
	}
	patches, err := filepath.Glob(filepath.Join(source, patchDir, "*.patch"))
	if err != nil {
		return leExecFailf("list the stdpatch patches: %v", err)
	}
	copies := []string{fileGoMod, "feature-gates.txt"}
	for _, patch := range patches {
		copies = append(copies, filepath.Join(patchDir, filepath.Base(patch)))
	}
	for _, name := range copies {
		data, err := os.ReadFile(filepath.Join(source, name)) //nolint:gosec // the checkout this fixture runs in
		if err != nil {
			return leExecFailf("read %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(checkout, name), data, 0o600); err != nil {
			return leExecFailf("copy %s: %v", name, err)
		}
	}
	return nil
}

// leExecRealGOROOT answers the real GOROOT of the toolchain the checkout pins,
// the GOROOT a developer's shell carries: `go env GOROOT` run by the go binary
// itself, every link resolved, with no GOROOT inherited, so a tree's bin first
// on PATH cannot answer the tree.
func leExecRealGOROOT(ctx context.Context, source string) (string, error) {
	onPath, err := exec.LookPath("go")
	if err != nil {
		return "", leExecFailf("find go: %v", err)
	}
	goBinary, err := filepath.EvalSymlinks(onPath)
	if err != nil {
		return "", leExecFailf("resolve %s: %v", onPath, err)
	}
	command := exec.CommandContext(ctx, goBinary, "env", "GOROOT") //nolint:gosec // the go on PATH, resolved
	command.Dir = source
	command.Env = withoutEnv(os.Environ(), "GOROOT", "GOTOOLCHAIN")
	out, err := command.Output()
	if err != nil {
		return "", leExecFailf("go env GOROOT: %v", err)
	}
	goroot := strings.TrimSpace(string(out))
	if goroot == "" {
		return "", leExecFailf("go env GOROOT answered nothing")
	}
	return goroot, nil
}

// leExecPathWithoutTrees removes every patched tree's bin from path, so the le
// under test starts from a developer's PATH rather than the runner's.
func leExecPathWithoutTrees(path string) string {
	entries := filepath.SplitList(path)
	kept := make([]string, 0, len(entries))
	for _, entry := range entries {
		if strings.Contains(entry, string(filepath.Separator)+"stdpatch"+string(filepath.Separator)) {
			continue
		}
		kept = append(kept, entry)
	}
	return strings.Join(kept, string(filepath.ListSeparator))
}

// leExecIsTree answers whether goroot is a patched tree of the fixture's own:
// a stdpatch directory under work, links resolved, since the checkout's cache/
// may be a link into the throwaway per-user cache.
func leExecIsTree(goroot, work string) bool {
	if goroot == "" {
		return false
	}
	resolved, err := filepath.EvalSymlinks(goroot)
	if err != nil {
		resolved = goroot
	}
	separator := string(filepath.Separator)
	return strings.HasPrefix(resolved, work+separator) && strings.Contains(resolved, separator+"stdpatch"+separator)
}

// leExecCaches checks the four cache variables name directories under target
// that exist.
func leExecCaches(got map[string]string, target, step string) error {
	for _, variable := range leExecCacheVariables {
		want := filepath.Join(target, variable[1])
		if got[variable[0]] != want {
			return leExecFailf("%s: %s = %q, want %q", step, variable[0], got[variable[0]], want)
		}
		info, err := os.Stat(want)
		if err != nil {
			return leExecFailf("%s: %s names %s, which le exec did not create: %v", step, variable[0], want, err)
		}
		if !info.IsDir() {
			return leExecFailf("%s: %s names %s, which is not a directory", step, variable[0], want)
		}
	}
	return nil
}

// leExecEnvironment runs `le exec /usr/bin/env` in dir and answers the
// environment the probe printed.
func leExecEnvironment(ctx context.Context, lePath, dir string, environ []string) (map[string]string, error) {
	result, err := leExecRun(ctx, lePath, dir, environ, "exec", leExecProbe)
	if err != nil {
		return nil, err
	}
	if result.exitCode != 0 {
		return nil, leExecFailf("le exec %s in %s exited %d:\n%s", leExecProbe, dir, result.exitCode, result.stderr)
	}
	got := make(map[string]string, 64)
	for line := range strings.SplitSeq(result.stdout, "\n") {
		name, value, found := strings.Cut(line, "=")
		if found {
			got[name] = value
		}
	}
	return got, nil
}

// leExecRun runs the built le with args in dir.
func leExecRun(ctx context.Context, lePath, dir string, environ []string, args ...string) (leStoreTrimResult, error) {
	command := exec.CommandContext(ctx, lePath, args...) //nolint:gosec // the fixture chooses the program and its arguments
	command.Dir = dir
	command.Env = environ
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	result := leStoreTrimResult{stdout: stdout.String(), stderr: stderr.String()}
	if err == nil {
		return result, nil
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		result.exitCode = exitErr.ExitCode()
		return result, nil
	}
	return result, leExecFailf("execute le %s: %v", strings.Join(args, " "), err)
}

func leExecFailf(format string, args ...any) error {
	return fmt.Errorf("FAIL: le-exec-cache-env: "+format, args...)
}
