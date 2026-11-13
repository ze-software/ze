// Design: docs/architecture/appliance/builder.md -- helper completion is not an image.

package appliance

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gokrazy/tools/gok"
)

type gokHelperCompletionFixture struct{}

func (*gokHelperCompletionFixture) Error() string     { return "partitioning helper completed" }
func (*gokHelperCompletionFixture) HelperCompletion() {}

// Exercise the existing embedded boundary seam: helper completion is a distinct
// successful command result, not a completed image. No image injection, next
// batch member, or manifest may run after it; prepared secrets must be removed.
func TestGokHelperCompletionStopsImagePostwork(t *testing.T) {
	if !gok.IsHelperCompletion(&gokHelperCompletionFixture{}) {
		t.Fatal("fixture does not implement the public embedded helper-completion contract")
	}
	for _, all := range []bool{false, true} {
		t.Run(map[bool]string{false: "single", true: "batch"}[all], func(t *testing.T) {
			root := writeGokrazyFixture(t)
			if err := os.MkdirAll(filepath.Join(root, "gokrazy", "modcache"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/ze-software/ze\n\ngo 1.26\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "gokrazy", "ze", "ze.conf"), []byte("set system host fixture\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			underPatchedTree(t, patchedTreeFixture(t, true))
			dir := initTestAppliance(t, "helper-a", nil)
			cfg := DefaultConfig("helper-b")
			encoded, err := json.Marshal(cfg)
			if err != nil {
				t.Fatal(err)
			}
			input := filepath.Join(dir, "helper-b.json")
			if err := os.WriteFile(input, encoded, 0o600); err != nil {
				t.Fatal(err)
			}
			if code := runInit([]string{"--config", input, "helper-b"}); code != exitOK {
				t.Fatalf("second appliance initialization=%d", code)
			}
			for _, name := range []string{"helper-a", "helper-b"} {
				pinTestApplianceHostArch(t, dir, name)
			}
			oldMkfs, oldDebugfs := e2fsMkfs, e2fsDebugfs
			e2fsMkfs, e2fsDebugfs = "fixture-mkfs", "fixture-debugfs"
			t.Cleanup(func() { e2fsMkfs, e2fsDebugfs = oldMkfs, oldDebugfs })
			oldExecute, oldExternal := gokExecuteFn, runExternalFn
			calls := 0
			var prepared string
			gokExecuteFn = func(_ context.Context, args []string) error {
				calls++
				prepared = args[1]
				if _, err := os.Stat(filepath.Join(prepared, "ze", "config.json")); err != nil {
					t.Fatalf("helper reached without private prepared configuration: %v", err)
				}
				return &gokHelperCompletionFixture{}
			}
			runExternalFn = func(context.Context, string, ...string) ([]byte, error) {
				t.Error("image formatting/injection ran after helper completion")
				return nil, errors.New("forbidden image postwork")
			}
			t.Cleanup(func() { gokExecuteFn, runExternalFn = oldExecute, oldExternal })
			args := []string{"helper-a"}
			if all {
				args = []string{"--all"}
			}
			if code := runBuild(args); code != exitOK {
				t.Errorf("successful partition helper command=%d, want 0", code)
			}
			if calls != 1 {
				t.Errorf("helper continued into %d packing calls, want exactly one", calls)
			}
			if _, err := os.Stat(prepared); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("helper completion left prepared credentials: %v", err)
			}
			for _, name := range []string{"helper-a", "helper-b"} {
				if _, err := os.Stat(filepath.Join(dir, name, "build.json")); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("helper published image manifest for %s: %v", name, err)
				}
			}
		})
	}
}

func TestGokHelperCompletionPublicContract(t *testing.T) {
	completion := &gok.HelperCompletion{}
	if !gok.IsHelperCompletion(completion) {
		t.Error("upstream helper completion is not recognized")
	}
	if !IsGokHelperCompletion(completion) {
		t.Error("appliance helper classifier lost the upstream completion")
	}
	if gok.IsHelperCompletion(errors.New("ordinary packing failure")) {
		t.Error("ordinary failure misclassified as successful helper completion")
	}
	if gok.IsHelperCompletion(errors.Join(completion, errors.New("working directory restoration failed"))) {
		t.Error("joined cleanup failure misclassified as successful helper completion")
	}
	if gok.IsHelperCompletion(nil) {
		t.Error("ordinary image success misclassified as helper completion")
	}
}
