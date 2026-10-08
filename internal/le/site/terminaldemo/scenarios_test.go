package siteterminaldemo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
	_ "github.com/ze-software/ze/internal/component/config/cli" // registers the config root
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/pkg/zefs"
)

// VALIDATES: the ze command a scenario runs to install its merged config
// replaces the ze.conf that `ze init` already wrote, through ze's own root
// handler, against a real store.
// PREVENTS: a scenario argv that ze refuses once the config exists (`config
// import` without confirmation), which failed every demo that adds fragments.
func TestScenarioConfigReplacesTheInitializedConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZE_CONFIG_DIR", dir)
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	store, err := storage.Create(dir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	discovered := []byte("interface { ethernet eth0 { } }\n")
	if err := store.WriteKey(zefs.KeyFileActive.Key(zeConfigFile), discovered); err != nil {
		t.Fatalf("seed ze.conf the way ze init does: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	merged := "interface { ethernet eth0 { } }\nbgp { }\n"
	path := filepath.Join(t.TempDir(), "active.conf")
	if err := os.WriteFile(path, []byte(merged), 0o600); err != nil {
		t.Fatalf("write merged config: %v", err)
	}

	args := importScenarioConfigArgs(path)
	handler := registry.LookupRoot(args[0])
	if handler == nil {
		t.Fatalf("ze has no %q root command", args[0])
	}
	if code := handler(nil, args[1:]); code != 0 {
		t.Fatalf("ze %v exited %d over an existing ze.conf", args, code)
	}

	store, err = storage.Open(dir)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer store.Close() //nolint:errcheck // test read handle

	got, err := storage.ReadActiveConfig(store, zeConfigFile)
	if err != nil {
		t.Fatalf("read active ze.conf: %v", err)
	}
	if string(got) != merged {
		t.Fatalf("active ze.conf = %q, want the merged config %q", got, merged)
	}
}

// VALIDATES: the daemon a scenario starts reads the config the scenario
// installed. The argv names no file, so ze opens the store under ZE_CONFIG_DIR
// and reads the stored config its instance name selects, and the instance name
// the harness answers `ze init` with selects ze.conf, the config `ze init`
// writes and importScenarioConfigArgs replaces.
// PREVENTS: `ze start ze.conf`, which is explicit-file mode: ze opened a store
// beside ze.conf in the working directory, ignored ZE_CONFIG_DIR, and never
// came up. Also an init answer whose instance name selects a config no step
// wrote (`ze-demo` selects ze-demo.conf).
func TestScenarioDaemonReadsTheInstalledConfig(t *testing.T) {
	args := daemonArgs()
	if len(args) == 0 {
		t.Fatal("daemon argv is empty")
	}
	if args[0] != commandStart {
		t.Fatalf("daemon argv %v does not start ze", args)
	}
	for _, arg := range args[1:] {
		if !strings.HasPrefix(arg, "-") {
			t.Fatalf("daemon argv %v names config %q: ze would run in explicit-file mode and ignore ZE_CONFIG_DIR", args, arg)
		}
	}

	answers := strings.Split(initText(demoPassword), "\n")
	// Ze init asks for the username, password, host, port and name, in that order.
	const nameAnswer = 4
	if len(answers) <= nameAnswer {
		t.Fatalf("init answers %q carry no instance name", answers)
	}

	store, err := storage.Create(t.TempDir())
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close() //nolint:errcheck // test handle

	if err := store.WriteKey(zefs.KeyInstanceName.Pattern, []byte(answers[nameAnswer])); err != nil {
		t.Fatalf("write instance name the way ze init does: %v", err)
	}
	if got := resolve.DefaultConfig(store); got != zeConfigFile {
		t.Fatalf("instance name %q makes bare `ze start` read %q, but the scenario installs %q", answers[nameAnswer], got, zeConfigFile)
	}
}
