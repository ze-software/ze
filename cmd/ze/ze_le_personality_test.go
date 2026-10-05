// VALIDATES: AC-15 and AC-16 use cmd/ze artifacts with one le command surface.
// PREVENTS: linking le into normal ze, direct tool roots, or personality drift.
package main

import (
	"bytes"
	"context"
	"debug/buildinfo"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/command/registry"
	coreenv "github.com/ze-software/ze/internal/core/env"
	_ "github.com/ze-software/ze/internal/le"
	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
)

const internalLeImport = "github.com/ze-software/ze/internal/le"

const leArtifactTimeout = 10 * time.Minute

func TestNormalZeLinksNoInternalLe(t *testing.T) {
	root := personalityRepoRoot(t)
	for _, pkg := range personalityDeps(t, root, normalZeTags(t, root)) {
		if pkg == internalLeImport || strings.HasPrefix(pkg, internalLeImport+"/") {
			t.Errorf("normal ze links development package %s", pkg)
		}
	}
}

func TestStandaloneLeAndZeLeHaveIdenticalSurface(t *testing.T) {
	root := personalityRepoRoot(t)
	featureTags := personalityFeatureTags(t, root)
	standaloneTags := append([]string{"ze_le"}, featureTags...)
	taggedTags := append([]string{"ze_core", "ze_distro", "ze_le"}, featureTags...)

	standaloneDeps := internalLeDeps(personalityDeps(t, root, standaloneTags))
	taggedDeps := internalLeDeps(personalityDeps(t, root, taggedTags))
	if len(standaloneDeps) == 0 {
		t.Fatal("standalone le links no internal/le package")
	}
	if !slices.Equal(standaloneDeps, taggedDeps) {
		t.Fatalf("internal/le dependency sets differ:\nstandalone: %v\ntagged ze: %v", standaloneDeps, taggedDeps)
	}

	dir := t.TempDir()
	standalone := filepath.Join(dir, "le")
	tagged := filepath.Join(dir, "ze")
	buildPersonality(t, root, standalone, standaloneTags)
	buildPersonality(t, root, tagged, taggedTags)
	for _, binary := range []string{standalone, tagged} {
		info, err := buildinfo.ReadFile(binary)
		if err != nil {
			t.Fatalf("read %s build info: %v", binary, err)
		}
		if info.Path != "github.com/ze-software/ze/cmd/ze" {
			t.Errorf("%s main package = %q, want cmd/ze", binary, info.Path)
		}
	}

	leHelp := invokePersonality(t, standalone, nil, "--help")
	zeHelp := invokePersonality(t, tagged, nil, "le", "--help")
	if leHelp.code != 0 || zeHelp.code != 0 {
		t.Fatalf("help codes: le=%d ze-le=%d\nle: %s%s\nze: %s%s",
			leHelp.code, zeHelp.code, leHelp.stdout, leHelp.stderr, zeHelp.stdout, zeHelp.stderr)
	}
	if leHelp.stdout != zeHelp.stdout {
		t.Errorf("help stdout differs:\nle: %q\nze le: %q", leHelp.stdout, zeHelp.stdout)
	}
	if leHelp.stderr != strings.ReplaceAll(zeHelp.stderr, "ze le", "le") {
		t.Errorf("help inventories differ:\nle:\n%s\nze le:\n%s", leHelp.stderr, zeHelp.stderr)
	}

	// Both real binaries read one stable Git population, never the checkout
	// another test or session may change between invocations.
	workingTree := filepath.Join(dir, "working-tree")
	writePersonalityWorkingTree(t, workingTree)
	workingTreeEnv := []string{"ZE_REPO_ROOT=" + workingTree}
	assertInvocationPair(t, standalone, tagged, 0, workingTreeEnv, []string{"repo", "working-tree"})
	assertInvocationPair(t, standalone, tagged, 1, nil, []string{"no-such-tool"})
	assertInvocationPair(t, standalone, tagged, 2, nil, []string{"repo", "no-such-action"})

	direct := invokePersonality(t, tagged, nil, "repo", "working-tree")
	if direct.code != 1 || !strings.Contains(direct.stderr, "unknown command") {
		t.Errorf("tagged ze exposed a direct tool root: code=%d stdout=%q stderr=%q",
			direct.code, direct.stdout, direct.stderr)
	}

	for _, format := range []string{"json", "yaml", "table"} {
		t.Run(format, func(t *testing.T) {
			assertInvocationPair(t, standalone, tagged, 0, workingTreeEnv,
				[]string{"repo", "working-tree", "|", format})
		})
	}

	// A generator failure carries its own exit code through both personalities,
	// which is the half a zero-exit pair cannot show. The fixture holds Go to
	// walk and no ai/ directory to write the map into, so the generator answers
	// 1 rather than refusing the arguments.
	fixture := filepath.Join(dir, "no-ai-checkout")
	writePersonalityFixture(t, fixture)
	env := []string{"ZE_REPO_ROOT=" + fixture}
	assertInvocationPair(t, standalone, tagged, 1, env,
		[]string{"repo", "package-map", "update", "|", "json"})
}

// TestPersonalityChildOwnsItsBuildIdentity builds a real le fixture under
// conflicting outer identities, then proves explicit child guards still apply.
func TestPersonalityChildOwnsItsBuildIdentity(t *testing.T) {
	t.Cleanup(coreenv.ResetCache)
	root := personalityRepoRoot(t)
	for _, name := range []string{
		"ZE_REPO_ROOT", "ze.repo.root", "Ze.RePo_Root",
		"ZE_LE_BUILD_NAME", "ze.le.build.name", "Ze.Le_Build.Name",
	} {
		t.Setenv(name, "outer-"+name)
	}
	binary := filepath.Join(t.TempDir(), "le")
	tags := append([]string{"ze_le"}, personalityFeatureTags(t, root)...)
	buildPersonality(t, root, binary, tags)
	for _, one := range []struct {
		name string
		env  []string
		code int
	}{
		{name: "fixture owns its identity", code: 0},
		{name: "matching explicit identity", env: []string{"ze.le.build.name=le"}, code: 0},
		{name: "mismatching explicit identity", env: []string{"ZE_LE_BUILD_NAME=another-build"}, code: 2},
	} {
		t.Run(one.name, func(t *testing.T) {
			got := invokePersonality(t, binary, one.env, "--help")
			if got.code != one.code {
				t.Fatalf("child exit = %d, want %d: %s%s", got.code, one.code, got.stdout, got.stderr)
			}
			if one.code == 2 {
				if !strings.Contains(got.stderr, "another-build") {
					t.Errorf("guard refusal omitted the requested identity: %q", got.stderr)
				}
			}
		})
	}
}

// TestLeDispatchesNoProductCommand preserves the standalone boundary: a root
// owned by ze must not become reachable because the le process shares the
// registry.
func TestLeDispatchesNoProductCommand(t *testing.T) {
	const name = "le-product-root-probe"
	ran := registerProductRootProbe(name)
	savedArgs := slices.Clone(os.Args)
	os.Args = []string{"le"}
	t.Cleanup(func() { os.Args = savedArgs })

	if code := defaultDispatch([]string{name}); code != 1 {
		t.Errorf("standalone le product-root refusal code = %d, want 1", code)
	}
	if *ran {
		t.Error("standalone le ran a product root")
	}
}

// TestTheCrossingRefusesZesOwnCommands preserves the tagged crossing boundary:
// the explicit `ze le` root dispatches development tools only.
func TestTheCrossingRefusesZesOwnCommands(t *testing.T) {
	const name = "crossing-product-root-probe"
	ran := registerProductRootProbe(name)
	crossing := registry.LookupRoot("le")
	if crossing == nil {
		t.Fatal("the test process has no le crossing root")
	}
	if code := crossing(&registry.RuntimeContext{}, []string{name}); code != 1 {
		t.Errorf("ze le product-root refusal code = %d, want 1", code)
	}
	if *ran {
		t.Error("ze le ran a product root")
	}
}

func registerProductRootProbe(name string) *bool {
	ran := new(bool)
	registry.MustRegisterRootHandler(name, func(*registry.RuntimeContext, []string) int {
		*ran = true
		return 73
	}, registry.Meta{ShortHelp: "a product-root boundary probe", Mode: "offline", Section: registry.SectionTest})
	return ran
}

type personalityResult struct {
	stdout string
	stderr string
	code   int
}

func assertInvocationPair(
	t *testing.T,
	standalone string,
	tagged string,
	wantCode int,
	env []string,
	args []string,
) {
	t.Helper()
	leResult := invokePersonality(t, standalone, env, args...)
	zeArgs := append([]string{"le"}, args...)
	zeResult := invokePersonality(t, tagged, env, zeArgs...)
	if leResult.code != wantCode || zeResult.code != wantCode {
		t.Errorf("%v codes: le=%d ze-le=%d, want %d", args, leResult.code, zeResult.code, wantCode)
	}
	if leResult.stdout != zeResult.stdout {
		t.Errorf("%v stdout differs:\nle: %q\nze le: %q", args, leResult.stdout, zeResult.stdout)
	}
	if leResult.stderr != strings.ReplaceAll(zeResult.stderr, "ze le", "le") {
		t.Errorf("%v stderr differs after program-name normalization:\nle: %q\nze le: %q",
			args, leResult.stderr, zeResult.stderr)
	}
}

func invokePersonality(t *testing.T, binary string, extraEnv []string, args ...string) personalityResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), leArtifactTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = append(launcherEnv(), extraEnv...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if cmd.ProcessState == nil {
		t.Fatalf("run %s %v: %v", binary, args, err)
	}
	return personalityResult{stdout: stdout.String(), stderr: stderr.String(), code: cmd.ProcessState.ExitCode()}
}

func buildPersonality(t *testing.T, root, output string, tags []string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), leArtifactTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-tags", strings.Join(tags, ","), "-o", output, "./cmd/ze")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build cmd/ze with %v: %v\n%s", tags, err, out)
	}
}

func personalityDeps(t *testing.T, root string, tags []string) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), leArtifactTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "list", "-deps", "-tags", strings.Join(tags, ","), "./cmd/ze")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("list cmd/ze dependencies with %v: %v\n%s", tags, err, out)
	}
	packages := strings.Fields(string(out))
	slices.Sort(packages)
	return packages
}

func internalLeDeps(packages []string) []string {
	selected := make([]string, 0, len(packages))
	for _, pkg := range packages {
		if pkg == internalLeImport || strings.HasPrefix(pkg, internalLeImport+"/") {
			selected = append(selected, pkg)
		}
	}
	return selected
}

func normalZeTags(t *testing.T, root string) []string {
	t.Helper()
	return append([]string{"ze_core", "ze_distro"}, personalityFeatureTags(t, root)...)
}

// personalityFeatureTags answers the gates a normal ze carries, through
// featuretags, the one reader of feature-gates.txt. ze_le is a PERSONALITY and
// never a gate: a manifest that declared it would put le's own commands in
// every shipped binary.
func personalityFeatureTags(t *testing.T, root string) []string {
	t.Helper()
	tags, err := repofeaturetags.DaemonTags(root)
	if err != nil {
		t.Fatalf("read the feature manifest: %v", err)
	}
	if slices.Contains(tags, "ze_le") {
		t.Fatal("feature-gates.txt includes non-default ze_le")
	}
	return tags
}

// writePersonalityWorkingTree commits an original file and leaves both a tracked
// modification and an untracked file for the output formatters to describe.
func writePersonalityWorkingTree(t *testing.T, root string) {
	t.Helper()
	writePersonalityFixture(t, root)
	// Runtime scratch files follow the real checkout's ignore policy; they are
	// not source changes for either personality.
	ignore, err := os.ReadFile(filepath.Join(personalityRepoRoot(t), ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), ignore, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "--quiet"},
		{"add", "."},
		{"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid",
			"-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "Original fixture"},
	} {
		cmd := exec.CommandContext(t.Context(), "git", args...)
		cmd.Dir = root
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	for relative, body := range map[string]string{
		"internal/core/thing/thing.go": "// Package thing changed.\npackage thing\n",
		"internal/core/thing/new.go":   "package thing\n",
	} {
		if err := os.WriteFile(filepath.Join(root, relative), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func writePersonalityFixture(t *testing.T, root string) {
	t.Helper()
	files := map[string]string{
		"internal/core/thing/thing.go": "// Package thing does a thing.\npackage thing\n",
	}
	for relative, content := range files {
		path := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create fixture directory: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write fixture %s: %v", relative, err)
		}
	}
}

func personalityRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "feature-gates.txt")); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no Ze checkout above test directory")
		}
		dir = parent
	}
}
