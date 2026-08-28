// VALIDATES: AC-15 and AC-16 use cmd/ze artifacts with one le command surface.
// PREVENTS: linking le into normal ze, direct tool roots, or personality drift.
package main

import (
	"bufio"
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
	_ "github.com/ze-software/ze/internal/le"
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

	assertInvocationPair(t, standalone, tagged, 0, nil, []string{"working-tree"})
	assertInvocationPair(t, standalone, tagged, 1, nil, []string{"no-such-tool"})
	assertInvocationPair(t, standalone, tagged, 2, nil, []string{"repository", "no-such-action"})

	direct := invokePersonality(t, tagged, nil, "working-tree")
	if direct.code != 1 || !strings.Contains(direct.stderr, "unknown command") {
		t.Errorf("tagged ze exposed a direct tool root: code=%d stdout=%q stderr=%q",
			direct.code, direct.stdout, direct.stderr)
	}

	for _, format := range []string{"json", "yaml", "table"} {
		t.Run(format, func(t *testing.T) {
			assertInvocationPair(t, standalone, tagged, 0, nil,
				[]string{"working-tree", "|", format})
		})
	}

	fixture := filepath.Join(dir, "stale-checkout")
	writePersonalityFixture(t, fixture)
	env := []string{"ZE_REPO_ROOT=" + fixture}
	assertInvocationPair(t, standalone, tagged, 3, env,
		[]string{"discovery-index", "check", "|", "json"})
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
	}, registry.Meta{Description: "a product-root boundary probe", Mode: "offline", Section: registry.SectionTest})
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
	cmd.Env = append(os.Environ(), extraEnv...)
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

func personalityFeatureTags(t *testing.T, root string) []string {
	t.Helper()
	file, err := os.Open(filepath.Join(root, "feature-gates.txt"))
	if err != nil {
		t.Fatalf("open feature-gates.txt: %v", err)
	}
	defer file.Close() //nolint:errcheck // read-only test input

	var tags []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		tags = append(tags, strings.Fields(line)[0])
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read feature-gates.txt: %v", err)
	}
	if slices.Contains(tags, "ze_le") {
		t.Fatal("feature-gates.txt includes non-default ze_le")
	}
	return tags
}

func writePersonalityFixture(t *testing.T, root string) {
	t.Helper()
	files := map[string]string{
		"ai/PACKAGE-MAP.md":            "stale\n",
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
