package site

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	repoinventory "github.com/ze-software/ze/internal/le/repo/inventory"
)

func committedRegistryFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	run := func(input string, args ...string) {
		t.Helper()
		command := exec.CommandContext(t.Context(), "git", append([]string{"-C", root}, args...)...)
		command.Stdin = strings.NewReader(input)
		command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("fixture git %v: %v: %s", args, err, output)
		}
	}
	run("", "init", "-q", "--initial-branch=main")
	files := map[string]string{
		"go.mod":            "module github.com/ze-software/ze\n\ngo 1.26\n",
		"feature-gates.txt": "ze_example internal/plugins/example\n",
		"cmd/ze/main.go":    "//go:build ze_le && ze_example\npackage main\nimport _ \"github.com/ze-software/ze/internal/plugins/example\"\nfunc main() {}\n",
		"internal/component/plugin/registry/registry.go": "package registry\ntype Registration struct { Name, Description string; RunEngine func() }\nfunc Register(Registration) error { return nil }\n",
		"internal/plugins/example/register_linux.go":     "package example\nimport r \"github.com/ze-software/ze/internal/component/plugin/registry\"\nfunc engine() {}\nfunc init() { _ = r.Register(r.Registration{Name: \"committed-linux\", Description: \"committed description\", RunEngine: engine}) }\n",
	}
	var stream strings.Builder
	stream.WriteString("commit refs/heads/main\ncommitter Fixture <fixture@example.invalid> 1700000000 +0000\ndata 7\nfixture\n")
	paths := make([]string, 0, len(files))
	for name := range files {
		paths = append(paths, name)
	}
	slices.Sort(paths)
	for _, name := range paths {
		body := files[name]
		fmt.Fprintf(&stream, "M 100644 inline %s\ndata %d\n%s\n", strconv.Quote(name), len(body), body)
	}
	stream.WriteString("\ndone\n")
	run(stream.String(), "fast-import", "--quiet")
	return root
}

// TestBuildPublishesCommittedLinuxRegistryWithoutHostExecution drives the actual
// publication entry over committed Linux-only source. Dirty files and a simulated
// Darwin environment cannot alter the published bytes or require Docker/staging.
func TestBuildPublishesCommittedLinuxRegistryWithoutHostExecution(t *testing.T) {
	t.Setenv("GOOS", "darwin")
	root := committedRegistryFixture(t)
	if err := os.MkdirAll(filepath.Join(root, "internal", "plugins", "example"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "plugins", "example", "register_linux.go"), []byte("package example\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	output := t.TempDir()
	if err := publishPluginRegistry(Paths{Repository: root, Output: output}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(output, "data", "plugin-registry.json"))
	if err != nil {
		t.Fatal(err)
	}
	var plugins []registryPlugin
	if err := json.Unmarshal(raw, &plugins); err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 1 || plugins[0].Name != "committed-linux" || plugins[0].Description != "committed description" {
		t.Fatalf("published host/dirty inventory: %s", raw)
	}
	if _, err := os.Stat(filepath.Join(root, "tmp")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("inventory staged executable source into checkout: %v", err)
	}
}

// TestCommittedRegistryRefusesUnreadablePopulation keeps publication fail-closed
// when committed membership is unavailable rather than accepting an empty list.
func TestCommittedRegistryRefusesUnreadablePopulation(t *testing.T) {
	root := t.TempDir()
	if plugins, err := livePluginRegistry(root); err == nil || plugins != nil {
		t.Fatalf("unavailable committed registry answered %v, %v", plugins, err)
	}
	output := t.TempDir()
	if err := publishPluginRegistry(Paths{Repository: root, Output: output}); err == nil {
		t.Fatal("unavailable registry was published")
	}
	if _, err := os.Stat(filepath.Join(output, "data", "plugin-registry.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("refusal published a registry file: %v", err)
	}
}

// EXISTING TEST BELOW IS RETAINED BYTE-FOR-BYTE (comments included).
// TestRegistryRefusalStopsTheBuildBeforeTheDaemonCompile proves a registry
// that cannot be read (no Docker, off Linux) stops the native refresh before
// the five-minute daemon compile. Method: a failing registry reader and a
// daemon compile that records whether it ran.
func TestRegistryRefusalStopsTheBuildBeforeTheDaemonCompile(t *testing.T) {
	previousRegistry, previousDaemon := livePluginRegistry, liveDaemon
	t.Cleanup(func() { livePluginRegistry, liveDaemon = previousRegistry, previousDaemon })
	refusal := errors.New("Docker does not answer")
	livePluginRegistry = func(string) ([]repoinventory.Plugin, error) { return nil, refusal }
	compiled := false
	liveDaemon = func(context.Context, string, string) (string, error) {
		compiled = true
		return "", errors.New("the daemon compile ran")
	}

	err := refreshNativeSurfaces(t.Context(), Paths{Repository: t.TempDir(), Source: t.TempDir(), Output: t.TempDir()})
	if !errors.Is(err, refusal) {
		t.Fatalf("refresh answered %v, want the registry refusal", err)
	}
	if compiled {
		t.Fatal("the daemon compiled before the plugin registry was read")
	}
}

// TestCommittedRegistryRefusesReadableEmptyPopulation preserves the old parser
// guard through the replacement producer: the committed entry and import graph
// are valid and readable, but no reachable init registers a plugin.
func TestCommittedRegistryRefusesReadableEmptyPopulation(t *testing.T) {
	root := committedRegistryFixture(t)
	headCommand := exec.CommandContext(t.Context(), "git", "-C", root, "rev-parse", "HEAD")
	head, err := headCommand.Output()
	if err != nil {
		t.Fatal(err)
	}
	body := "package example\n"
	stream := "commit refs/heads/main\ncommitter Fixture <fixture@example.invalid> 1700000001 +0000\ndata 5\nempty\nfrom " + strings.TrimSpace(string(head)) + "\nM 100644 inline internal/plugins/example/register_linux.go\ndata " + strconv.Itoa(len(body)) + "\n" + body + "\n\ndone\n"
	command := exec.CommandContext(t.Context(), "git", "-C", root, "fast-import", "--quiet")
	command.Stdin = strings.NewReader(stream)
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	if raw, err := command.CombinedOutput(); err != nil {
		t.Fatalf("empty committed fixture: %v: %s", err, raw)
	}
	plugins, err := livePluginRegistry(root)
	if err == nil || plugins != nil {
		t.Fatalf("readable empty population answered %+v: %v", plugins, err)
	}
	if !strings.Contains(err.Error(), "registers no plugins") {
		t.Fatalf("empty fixture failed before empty-population guard: %v", err)
	}
	output := t.TempDir()
	err = publishPluginRegistry(Paths{Repository: root, Output: output})
	if err == nil || !strings.Contains(err.Error(), "registers no plugins") {
		t.Fatalf("publication did not preserve empty-population refusal: %v", err)
	}
	if _, err := os.Stat(filepath.Join(output, "data", "plugin-registry.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("empty refusal published registry artifact: %v", err)
	}
}
