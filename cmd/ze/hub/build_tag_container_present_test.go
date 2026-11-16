// Design: ai/rules/plugins.md -- ze_container present (compile-out) validation
//
//go:build ze_container

package hub

// VALIDATES: with the ze_container build tag (a default ze and appliance
// feature) the container plugin is registered, its config root parses, and
// the binary links the vendored runtime library without the three parts the
// patch series removes from the link: the systemd control group driver, the
// cgroup v1 manager and the checkpoint client.
// PREVENTS: a regression where ze_container is set but the plugin is not
// wired (the generated all_ze_container.go blank import dropped), or where a
// module bump or a lost patch links code ze never runs.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	pluginreg "github.com/ze-software/ze/internal/component/plugin/registry"
)

func TestBuildTag_Container_Present(t *testing.T) {
	if !pluginreg.Has("container") {
		t.Fatal("ze_container build: container plugin not registered")
	}
}

// TestBuildTag_Container_PresentParsesContainerConfig proves the plugin's
// schema reached the build: the container root and its instance list parse.
func TestBuildTag_Container_PresentParsesContainerConfig(t *testing.T) {
	const cfg = `container {
	instance web {
		enabled false;
	}
}
`
	if _, err := zeconfig.ParseTreeWithYANG(cfg, nil); err != nil {
		t.Fatalf("ze_container build refused a container config: %v", err)
	}
}

// TestBuildTag_Container_PresentBinaryLinksNoUnusedRuntime proves what the
// patch series removes stays out of the link. Method: build ze with the
// container gate, list its symbols, require the runtime library to be there
// (so an empty or wrong listing cannot pass) and each removed part to be
// absent.
func TestBuildTag_Container_PresentBinaryLinksNoUnusedRuntime(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the ze binary (slow); skipped under -short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	repoRoot := filepath.Join("..", "..", "..")

	bin := filepath.Join(t.TempDir(), "ze-container")
	cmd := exec.CommandContext(ctx, "go", "build", "-tags", "ze_core,ze_container", "-o", bin, "./cmd/ze")
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build -tags ze_core,ze_container failed: %v\n%s", err, out)
	}
	out, err := exec.CommandContext(ctx, "go", "tool", "nm", bin).CombinedOutput()
	if err != nil {
		t.Fatalf("go tool nm failed: %v\n%s", err, out)
	}
	syms := string(out)

	for _, linked := range []string{
		"internal/plugins/container",
		"github.com/opencontainers/runc/libcontainer",
	} {
		if !strings.Contains(syms, linked) {
			t.Fatalf("ze_container build: binary holds no symbol matching %q, so the listing proves nothing", linked)
		}
	}

	// The trailing dot ends the package path: the cgroup v1 manager is the
	// package cgroups/fs, and the cgroup v2 manager ze uses is cgroups/fs2.
	removed := []string{
		"github.com/opencontainers/cgroups/systemd.",
		"github.com/opencontainers/cgroups/fs.",
		"github.com/opencontainers/cgroups/manager.",
		"github.com/coreos/go-systemd",
		"github.com/checkpoint-restore/go-criu",
	}
	for line := range strings.SplitSeq(syms, "\n") {
		for _, needle := range removed {
			if strings.Contains(line, needle) {
				t.Fatalf("ze_container build: binary retained symbol %q matching %q", line, needle)
			}
		}
	}
}
