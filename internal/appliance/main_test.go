package appliance

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// TestDispatchTableWired guards against the regression where the dispatch map
// captured the stub handlers before the per-command init() functions installed
// the real implementations. Every subcommand must resolve to a real handler.
func TestDispatchTableWired(t *testing.T) {
	stubPtr := reflect.ValueOf(stub).Pointer()
	table := dispatchTable()
	if len(table) == 0 {
		t.Fatal("dispatchTable is empty")
	}
	for name, h := range table {
		if h == nil {
			t.Errorf("subcommand %q has a nil handler", name)
			continue
		}
		if reflect.ValueOf(h).Pointer() == stubPtr {
			t.Errorf("subcommand %q is not wired (still points at stub)", name)
		}
	}
}

func setApplianceDir(t *testing.T, value string) {
	t.Helper()
	t.Setenv("ZE_APPLIANCE_DIR", value)
	env.ResetCache()
}

func TestDirResolution(t *testing.T) {
	t.Run("flag takes priority", func(t *testing.T) {
		setApplianceDir(t, "/from-env")
		got := ResolveDir("/from-flag")
		if got != "/from-flag" {
			t.Errorf("ResolveDir = %q, want /from-flag", got)
		}
	})

	t.Run("env takes priority over default", func(t *testing.T) {
		setApplianceDir(t, "/from-env")
		got := ResolveDir("")
		if got != "/from-env" {
			t.Errorf("ResolveDir = %q, want /from-env", got)
		}
	})

	t.Run("default uses XDG_CONFIG_HOME", func(t *testing.T) {
		setApplianceDir(t, "")
		xdgDir := filepath.Join(t.TempDir(), "xdg-config")
		t.Setenv("XDG_CONFIG_HOME", xdgDir)
		env.ResetCache()
		got := ResolveDir("")
		want := filepath.Join(xdgDir, defaultSubdir)
		if got != want {
			t.Errorf("ResolveDir = %q, want %q", got, want)
		}
	})

	t.Run("default falls back to HOME/.config", func(t *testing.T) {
		setApplianceDir(t, "")
		t.Setenv("XDG_CONFIG_HOME", "")
		env.ResetCache()
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skip("cannot determine home dir")
		}
		got := ResolveDir("")
		want := filepath.Join(home, ".config", defaultSubdir)
		if got != want {
			t.Errorf("ResolveDir = %q, want %q", got, want)
		}
	})
}
