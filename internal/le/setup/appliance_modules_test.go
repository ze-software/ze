package setup

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/appliance/instance"
)

// VALIDATES: the module-cache fill walks every gokrazy/<instance>/builddir,
// the docker host's as well as the appliance's, and skips each instance's
// reserved kernel module.
// PREVENTS: the docker-host image build failing under GOPROXY=off with
// "module lookup disabled" because only gokrazy/ze/builddir was downloaded.
// Method: a checkout with two instances, a kernel module in each, and a Shell
// whose Exec records the directory of every `go mod download all`.
func TestApplianceModulesFillEveryInstance(t *testing.T) {
	root := t.TempDir()
	write := func(relative string) string {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", relative, err)
		}
		if err := os.WriteFile(path, []byte("module x\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", relative, err)
		}
		return filepath.Dir(path)
	}
	ze := write("gokrazy/ze/builddir/github.com/ze-software/ze/go.mod")
	host := write("gokrazy/docker-host/builddir/github.com/docker/cli/go.mod")
	write("gokrazy/ze/builddir/" + instance.KernelModule + "/go.mod")
	write("gokrazy/docker-host/builddir/" + instance.KernelModule + "/go.mod")

	var dirs []string
	setup := &Setup{Root: root, Shell: &Shell{Exec: func(_ context.Context, cmd Cmd) Result {
		dirs = append(dirs, cmd.Dir)
		return Result{Argv: cmd.Argv}
	}}}
	report, code := setup.FillApplianceModules()
	if code != 0 {
		t.Fatalf("FillApplianceModules answered %d: %v", code, report)
	}
	slices.Sort(dirs)
	want := []string{host, ze}
	slices.Sort(want)
	if !slices.Equal(dirs, want) {
		t.Fatalf("downloaded in %v, want exactly %v", dirs, want)
	}
}
