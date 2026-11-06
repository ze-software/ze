package site

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
	repoinventory "github.com/ze-software/ze/internal/le/repo/inventory"
)

// stubPluginRegistries replaces both registry readers for one test and
// restores them after it.
func stubPluginRegistries(t *testing.T, inProcess, container func(string) ([]repoinventory.Plugin, error)) {
	t.Helper()
	previousInProcess, previousContainer := inProcessPluginRegistry, containerPluginRegistry
	t.Cleanup(func() {
		inProcessPluginRegistry, containerPluginRegistry = previousInProcess, previousContainer
	})
	inProcessPluginRegistry, containerPluginRegistry = inProcess, container
}

// TestNonLinuxBuildNeverReadsTheHostRegistry proves a build on any host but
// Linux publishes the container's registry and never calls the in-process one,
// whose registry misses every plugin registered from a Linux-only file. Method:
// stub both readers, record the in-process calls, and ask for each host OS.
func TestNonLinuxBuildNeverReadsTheHostRegistry(t *testing.T) {
	inProcessCalls := 0
	stubPluginRegistries(t,
		func(string) ([]repoinventory.Plugin, error) {
			inProcessCalls++
			return []repoinventory.Plugin{{Name: "host"}}, nil
		},
		func(string) ([]repoinventory.Plugin, error) {
			return []repoinventory.Plugin{{Name: "linux-container"}}, nil
		},
	)

	for _, goos := range []string{"darwin", "freebsd", "windows", "openbsd"} {
		plugins, err := pluginRegistryFor(goos)("/checkout")
		if err != nil {
			t.Fatalf("%s: %v", goos, err)
		}
		if len(plugins) != 1 || plugins[0].Name != "linux-container" {
			t.Fatalf("%s published %+v, want the container's registry", goos, plugins)
		}
	}
	if inProcessCalls != 0 {
		t.Fatalf("a non-Linux build read the host registry %d times", inProcessCalls)
	}

	plugins, err := pluginRegistryFor("linux")("/checkout")
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 1 || plugins[0].Name != "host" {
		t.Fatalf("linux published %+v, want its own registry", plugins)
	}
}

// failingRunner answers every docker command with an error, as a host with no
// Docker running does.
type failingRunner struct{ calls [][]string }

func (r *failingRunner) Run(_ context.Context, command interoplab.ProcessCommand) (interoplab.ProcessResult, error) {
	r.calls = append(r.calls, command.Arguments)
	return interoplab.ProcessResult{ExitCode: 1, Stderr: "Cannot connect to the Docker daemon"}, errors.New("exit status 1")
}

// TestLinuxRegistryRefusesWithoutDocker proves a host with no Docker stops the
// build with the problem, the reason and the fix, before it compiles or stages
// anything, and with no registry to fall back on. Method: a fixture Docker
// whose every command fails, over an empty checkout.
func TestLinuxRegistryRefusesWithoutDocker(t *testing.T) {
	runner := &failingRunner{}
	root := t.TempDir()

	plugins, err := readLinuxPluginRegistry(context.Background(), interoplab.NewFixtureDocker(runner), root)
	if err == nil {
		t.Fatalf("no Docker answered %+v, want a refusal", plugins)
	}
	if plugins != nil {
		t.Fatalf("a refusal still answered %+v", plugins)
	}
	for _, want := range []string{"Docker does not answer", "register only on Linux", "docker info", "./le site build"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("refusal %q does not name %q", err, want)
		}
	}
	if len(runner.calls) != 1 {
		t.Fatalf("ran %d docker commands after Docker did not answer: %v", len(runner.calls), runner.calls)
	}
	if _, statErr := os.Stat(filepath.Join(root, "tmp")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("staged into the checkout before Docker answered: %v", statErr)
	}
}

// TestLinuxRegistryContainerRunsTheStagedLe proves the container runs the le
// staged in the checkout, over the checkout mounted read-only and worked in,
// and asks it for the inventory as JSON. Method: read the one-shot spec.
func TestLinuxRegistryContainerRunsTheStagedLe(t *testing.T) {
	spec := linuxRegistryContainer("/home/u/ze", "tmp/site-linux-le-1/le")
	if spec.Image != interoplab.StaticBinaryImage {
		t.Fatalf("image %q", spec.Image)
	}
	if len(spec.Mounts) != 1 || spec.Mounts[0] != (interoplab.Mount{Source: "/home/u/ze", Target: "/src", ReadOnly: true}) {
		t.Fatalf("mounts %+v", spec.Mounts)
	}
	if strings.Join(spec.Arguments, " ") != "--workdir /src" {
		t.Fatalf("arguments %q", spec.Arguments)
	}
	if got := strings.Join(spec.Command, " "); got != "/src/tmp/site-linux-le-1/le repo inventory | json" {
		t.Fatalf("command %q", got)
	}
	if spec.Timeout <= 0 {
		t.Fatalf("timeout %v", spec.Timeout)
	}
}

// TestInventoryPluginsReadsThePluginList proves the container's answer is read
// as the inventory's plugin list, and that an empty list or another document
// is refused rather than published as an empty catalog. Method: three bodies.
func TestInventoryPluginsReadsThePluginList(t *testing.T) {
	plugins, err := inventoryPlugins([]byte(`{"generated":"x","plugins":[{"name":"memlock","source-dir":"internal/plugins/memlock","has-yang":false}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 1 || plugins[0].Name != "memlock" || plugins[0].SourceDir != "internal/plugins/memlock" {
		t.Fatalf("plugins %+v", plugins)
	}

	for _, body := range []string{`{"plugins":[]}`, `{}`, `not json`} {
		if refused, err := inventoryPlugins([]byte(body)); err == nil {
			t.Fatalf("%q answered %+v, want a refusal", body, refused)
		}
	}
}

// TestBuildPublishesTheHostAwareRegistry proves the reader a build publishes
// (livePluginRegistry) is the host-aware one: on this host it reaches the
// reader pluginRegistryFor picks, the container's anywhere but Linux. Method:
// stub both readers with distinct answers and call the unstubbed build seam; a
// build wired straight to the in-process registry calls neither stub.
func TestBuildPublishesTheHostAwareRegistry(t *testing.T) {
	stubPluginRegistries(t,
		func(string) ([]repoinventory.Plugin, error) {
			return []repoinventory.Plugin{{Name: "in-process"}}, nil
		},
		func(string) ([]repoinventory.Plugin, error) {
			return []repoinventory.Plugin{{Name: "linux-container"}}, nil
		},
	)
	want := "linux-container"
	if runtime.GOOS == shippedGOOS {
		want = "in-process"
	}

	plugins, err := livePluginRegistry(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 1 || plugins[0].Name != want {
		t.Fatalf("a build on %s published %d plugins, first %+v; want only %q", runtime.GOOS, len(plugins), plugins, want)
	}
}

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
	liveDaemon = func(string, string) (string, error) {
		compiled = true
		return "", errors.New("the daemon compile ran")
	}

	err := refreshNativeSurfaces(Paths{Repository: t.TempDir(), Source: t.TempDir(), Output: t.TempDir()})
	if !errors.Is(err, refusal) {
		t.Fatalf("refresh answered %v, want the registry refusal", err)
	}
	if compiled {
		t.Fatal("the daemon compiled before the plugin registry was read")
	}
}
