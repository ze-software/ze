// Design: docs/contributing/gh-pages.md -- the plugin catalog publishes the Linux build's registry
// Related: build.go -- publishPluginRegistry, the one caller
// Related: ../interoplab/zebuild.go -- StageBinaries, the linux le cross-compile this reuses
//
// pluginregistry.go answers which plugin registrations a build publishes.
//
// Ze ships only on Linux, and some plugins register from a file built only on
// Linux (memlock, iface-dhcp and iface-ra when this was written). An le built
// for any other host never runs their init(), so its own registry is SMALLER
// than the shipped one, with no error. A Mac build that published it would
// drop those plugins from the catalog, llms.txt, the search index and every
// page that reads data/plugin-registry.json. So a build on Linux reads its own
// registry, and a build anywhere else reads the registry of a linux le run in
// a container. When no container can run, the build stops: it never falls
// back to the host's registry.

package site

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/internal/le/linuxle"
	repoinventory "github.com/ze-software/ze/internal/le/repo/inventory"
)

// shippedGOOS is the one operating system Ze ships on, so the one whose plugin
// registry a build publishes.
const shippedGOOS = "linux"

// linuxRegistryMount is where the checkout appears inside the container.
const linuxRegistryMount = "/src"

// linuxRegistryRunTimeout bounds the container run, which compiles nothing:
// the linux le is staged before it starts.
const linuxRegistryRunTimeout = 2 * time.Minute

// inProcessPluginRegistry reads this process's own registrations, which are
// the shipped ones only when this process runs on shippedGOOS. It is a variable
// so a test can prove a build elsewhere never calls it.
var inProcessPluginRegistry = repoinventory.Plugins

// containerPluginRegistry reads the registrations of a linux le run in a
// container. It is a variable so a test can state its answer rather than run
// Docker.
var containerPluginRegistry = linuxPluginRegistry

// pluginRegistryFor answers the reader of the plugin registry a build on goos
// publishes: the in-process registry on shippedGOOS, the container's anywhere
// else.
func pluginRegistryFor(goos string) func(string) ([]repoinventory.Plugin, error) {
	if goos == shippedGOOS {
		return inProcessPluginRegistry
	}
	return containerPluginRegistry
}

// hostPluginRegistry reads the plugin registry a build on this host publishes.
func hostPluginRegistry(root string) ([]repoinventory.Plugin, error) {
	return pluginRegistryFor(runtime.GOOS)(root)
}

// linuxPluginRegistry reads the plugin registry of a linux le, run in a
// container on this machine's Docker.
func linuxPluginRegistry(root string) ([]repoinventory.Plugin, error) {
	return readLinuxPluginRegistry(context.Background(), interoplab.NewBuildToolDocker(), root)
}

// readLinuxPluginRegistry checks Docker answers, cross-compiles a linux le
// into the checkout's tmp/ with the lab recipe (interoplab.StageBinaries, the
// tags of linuxle.Base and every feature gate, for the daemon's architecture),
// runs `le repo inventory | json` in a container over the checkout mounted
// read-only, and answers the plugins it printed.
//
// The le is staged inside the checkout because the checkout is the one host
// directory every Docker on a Mac shares with its VM, where a system temporary
// directory is not.
func readLinuxPluginRegistry(ctx context.Context, docker *interoplab.Docker, root string) (plugins []repoinventory.Plugin, err error) {
	if err := docker.Probe(ctx); err != nil {
		return nil, linuxRegistryRefusal("Docker does not answer", err, startDockerFix)
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return nil, linuxRegistryRefusal("the checkout path cannot be made absolute for the container mount", err, causeFix)
	}

	scratch := filepath.Join(root, "tmp")
	if err := os.MkdirAll(scratch, 0o750); err != nil {
		return nil, linuxRegistryRefusal("the checkout's tmp/ directory, where the linux le is staged, cannot be created", err, causeFix)
	}
	stage, err := os.MkdirTemp(scratch, "site-linux-le-*")
	if err != nil {
		return nil, linuxRegistryRefusal("the staging directory for the linux le cannot be created", err, causeFix)
	}
	defer func() { err = errors.Join(err, os.RemoveAll(stage)) }()

	relative, err := filepath.Rel(root, filepath.Join(stage, linuxle.Name))
	if err != nil {
		return nil, linuxRegistryRefusal("the staged linux le is not inside the checkout the container mounts", err, causeFix)
	}
	output := filepath.ToSlash(relative)
	build := interoplab.StageBinaries(root, false, interoplab.LabBinary{Name: linuxle.Name, Base: linuxle.Base, Output: output})
	if err := build(ctx, docker); err != nil {
		return nil, linuxRegistryRefusal("the linux le did not build", err, causeFix)
	}

	result, err := docker.RunOneShot(ctx, linuxRegistryContainer(root, output))
	if err != nil {
		return nil, linuxRegistryRefusal("the linux le did not answer `repo inventory | json`", err, causeFix)
	}
	return inventoryPlugins([]byte(result.Stdout))
}

// linuxRegistryContainer answers the one-shot container that prints the
// inventory: the checkout mounted read-only, worked in, so the le finds the
// checkout from its working directory (lepath.Root).
func linuxRegistryContainer(root, output string) interoplab.OneShotContainer {
	return interoplab.OneShotContainer{
		Image:     interoplab.StaticBinaryImage,
		Mounts:    []interoplab.Mount{{Source: root, Target: linuxRegistryMount, ReadOnly: true}},
		Arguments: []string{"--workdir", linuxRegistryMount},
		Command:   []string{path.Join(linuxRegistryMount, output), "repo", "inventory", "|", "json"},
		Timeout:   linuxRegistryRunTimeout,
	}
}

// inventoryPlugins answers the plugins of a `le repo inventory | json`
// document. A document with no plugin is refused: a Ze build always registers
// some, so none means the le answered something other than the inventory.
func inventoryPlugins(raw []byte) ([]repoinventory.Plugin, error) {
	var inventory struct {
		Plugins []repoinventory.Plugin `json:"plugins"`
	}
	if err := json.Unmarshal(raw, &inventory); err != nil {
		return nil, linuxRegistryRefusal("the linux le printed something that is not the inventory", err, causeFix)
	}
	if len(inventory.Plugins) == 0 {
		return nil, linuxRegistryRefusal("the linux le listed no plugin", errors.New("its inventory holds an empty plugin list"), causeFix)
	}
	return inventory.Plugins, nil
}

// startDockerFix is the next step when Docker does not answer.
const startDockerFix = "start Docker (on a Mac, `colima start` or Docker Desktop), check it answers with\n" +
	"  docker info\nthen run the build again:\n  ./le site build"

// causeFix is the next step when Docker answers and the linux le fails.
const causeFix = "correct the cause named above, then run the build again:\n  ./le site build"

// linuxRegistryRefusal answers why a build off Linux stopped: the problem,
// why the build needs a container at all, and the step that fixes it.
func linuxRegistryRefusal(problem string, cause error, fix string) error {
	var tb textbuf.Buffer
	tb.Str("site build: cannot read the Linux plugin registry: ").Str(problem).Str(": ").Err(cause).
		Str(".\nWhy: this host runs ").Str(runtime.GOOS).
		Str(", and some plugins register only on Linux, where Ze ships. The build reads the registry ").
		Str("from a linux le run in a Docker container, and never publishes this host's smaller one.").
		Str("\nFix: ").Str(fix)
	return errors.New(tb.String())
}
