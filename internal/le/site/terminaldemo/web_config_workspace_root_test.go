// Design: docs/contributing/terminal-demos.md -- private journey store ownership.
package siteterminaldemo

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/paths"
)

// TestWebConfigWorkspaceRootRejectsInheritedAliases launches an actual Go child
// through the workspace's command owner. The child consumes DefaultConfigDir
// and writes into that store, so alias poisoning cannot hide behind an argv or
// environment-only assertion. A separate foreign-store sentinel must survive.
// Absolute and relative state overrides must preserve that same isolation.
func TestWebConfigWorkspaceRootRejectsInheritedAliases(t *testing.T) {
	for _, stateRelative := range []bool{false, true} {
		for _, alias := range []string{"ze.config.dir", "zE_CoNfIg.DiR", "Ze.CoNfIg_DiR", "ZE_CONFIG_DIR"} {
			name := alias
			if stateRelative {
				name += "/relative-state"
			}
			t.Run(name, func(t *testing.T) {
				root := t.TempDir()
				foreign := filepath.Join(root, "foreign-store")
				const original = "foreign store remains owned by another invocation\n"
				mustWrite(t, filepath.Join(foreign, "store-sentinel"), original)
				t.Setenv(alias, foreign)
				t.Setenv("ZE_DEMO_ROOT", root)
				stateRoot := filepath.Join(root, "state")
				if stateRelative {
					t.Chdir(root)
					stateRoot = "state"
				}
				t.Setenv("ZE_DEMO_STATE_ROOT", stateRoot)
				observed := filepath.Join(root, "resolved-root")
				t.Setenv("ZE_JOURNEY_RESOLVED_ROOT", observed)
				executable, err := os.Executable()
				if err != nil {
					t.Fatal(err)
				}
				bin := filepath.Join(root, "bin")
				if err := os.Mkdir(bin, 0o700); err != nil {
					t.Fatal(err)
				}
				// A direct executable link, not a shell wrapper: shells may discard
				// dotted environment names before the consuming Go code sees them.
				if err := os.Symlink(executable, filepath.Join(bin, "ze")); err != nil {
					t.Fatal(err)
				}
				t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
				ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
				defer cancel()
				workspace, err := newWebConfigWorkspace(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := workspace.Close(); err != nil {
						t.Error(err)
					}
				}()
				if _, err := workspace.run(ctx, []string{"-test.run=^TestWebConfigWorkspaceRootChild$"}, nil, "root.log"); err != nil {
					t.Fatal(err)
				}
				resolved, err := os.ReadFile(observed)
				if err != nil {
					t.Fatal(err)
				}
				private := filepath.Join(workspace.directory, "config")
				if string(resolved) != private {
					t.Errorf("actual child resolved inherited %s instead of its private store: got %q want %q", alias, resolved, private)
				}
				if data, err := os.ReadFile(filepath.Join(private, "store-sentinel")); err != nil || string(data) != "written by actual workspace child\n" {
					t.Errorf("actual child did not write its private store: %q, %v", data, err)
				}
				if data, err := os.ReadFile(filepath.Join(foreign, "store-sentinel")); err != nil || string(data) != original {
					t.Errorf("workspace child modified foreign store through %s: %q, %v", alias, data, err)
				}
			})
		}
	}
}

// TestWebConfigWorkspaceRootChild deliberately uses the real consumer rather
// than reading one chosen environment spelling. Its store write makes a wrong
// root observable independently of the separately retained resolved path.
func TestWebConfigWorkspaceRootChild(t *testing.T) {
	observed := os.Getenv("ZE_JOURNEY_RESOLVED_ROOT")
	if observed == "" {
		return
	}
	root := paths.DefaultConfigDir()
	if root == "" {
		t.Fatal("child could not resolve its config store")
	}
	if err := os.WriteFile(observed, []byte(root), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "store-sentinel"), []byte("written by actual workspace child\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}
