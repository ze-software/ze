package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// writeInstanceFixture lays out a checked-in gokrazy tree
// (<root>/gokrazy/ze/{config.json,builddir/...}) and returns the root and the
// parent dir, mirroring the shape of the repository's own gokrazy/.
func writeInstanceFixture(t *testing.T) (root, parent string) {
	t.Helper()
	root = t.TempDir()
	parent = filepath.Join(root, "gokrazy")
	mod := filepath.Join(parent, "ze", "builddir", "github.com", "ze-software", "ze")
	if err := os.MkdirAll(mod, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(path, data string) {
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(mod, "go.mod"), "module gokrazy/build/github.com/ze-software/ze\n\ngo 1.26\n")
	write(filepath.Join(parent, "ze", "config.json"), `{"Hostname":"ze"}`)
	return root, parent
}

// TestZeGokPreparesParentDir verifies ze-gok rewrites --parent_dir to a prepared
// copy under the project tmp/ before gok sees it, in both pflag spellings, and
// that the copy carries the builddir pins.
//
// VALIDATES: AC-13 and D-1c -- `make ze-gokrazy` gets a prepared instance with no
// change to mk/gokrazy.mk.
// PREVENTS: an image build running from, or writing to, the tracked gokrazy dir.
func TestZeGokPreparesParentDir(t *testing.T) {
	for _, tc := range []struct {
		name string
		args func(parent string) []string
		at   int // index of the parent_dir VALUE, or -1 when it is inline
	}{
		{
			name: "separate value",
			args: func(p string) []string {
				return []string{"--parent_dir", p, "-i", "ze", "overwrite", "--full", "x.img"}
			},
			at: 1,
		},
		{
			name: "inline value",
			args: func(p string) []string {
				return []string{"--parent_dir=" + p, "-i", "ze", "overwrite", "--full", "x.img"}
			},
			at: -1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, parent := writeInstanceFixture(t)

			got, cleanup, err := prepareArgs(tc.args(parent))
			if err != nil {
				t.Fatalf("prepareArgs: %v", err)
			}
			defer cleanup()

			var prepared string
			if tc.at >= 0 {
				prepared = got[tc.at]
			} else {
				prepared = strings.TrimPrefix(got[0], "--parent_dir=")
			}

			if prepared == parent {
				t.Fatalf("gok would still build from the tracked dir %s", parent)
			}
			wantTmp := filepath.Join(root, "tmp")
			if !strings.HasPrefix(prepared, wantTmp+string(filepath.Separator)) {
				t.Errorf("prepared parent %q is not under project tmp %q", prepared, wantTmp)
			}
			if _, err := os.Stat(filepath.Join(prepared, "ze", "builddir", "github.com", "ze-software", "ze", "go.mod")); err != nil {
				t.Errorf("prepared instance lost the builddir pins: %v", err)
			}

			// Every other argument is passed through untouched.
			if got[len(got)-1] != "x.img" || got[len(got)-2] != "--full" {
				t.Errorf("trailing args were altered: %v", got)
			}

			cleanup()
			if _, err := os.Stat(prepared); !os.IsNotExist(err) {
				t.Errorf("cleanup did not remove the prepared dir: %v", err)
			}
		})
	}
}

// TestZeGokLeavesMutatingSubcommandsAlone verifies a subcommand that EDITS the
// instance is never redirected to a throwaway copy. Preparing `gok edit` or
// `gok add` would write the operator's change into a temp dir and then delete
// it, losing the edit silently.
//
// VALIDATES: preparation is scoped to image-building subcommands.
// PREVENTS: silently discarding an operator's instance edit.
func TestZeGokLeavesMutatingSubcommandsAlone(t *testing.T) {
	for _, verb := range []string{"edit", "add", "new", "get"} {
		t.Run(verb, func(t *testing.T) {
			_, parent := writeInstanceFixture(t)
			args := []string{"--parent_dir", parent, "-i", "ze", verb}

			got, cleanup, err := prepareArgs(args)
			if err != nil {
				t.Fatalf("prepareArgs: %v", err)
			}
			defer cleanup()

			if got[1] != parent {
				t.Errorf("%s was redirected to %q; it must act on the real instance %q", verb, got[1], parent)
			}
		})
	}
}

// TestZeGokIdentifiesTheSubcommandNotAnyToken verifies the build/mutate decision
// is made from gok's actual subcommand, not from any token that happens to spell
// one.
//
// Found in review: scanning every argument meant `gok edit --instance overwrite`
// looked like a build, and preparing an `edit` writes the operator's change into
// a temp copy that is deleted moments later. The value of a global value-taking
// flag is never the subcommand.
//
// VALIDATES: preparation is keyed on the subcommand.
// PREVENTS: silently discarding an instance edit whose flag value spells a build
// verb.
func TestZeGokIdentifiesTheSubcommandNotAnyToken(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"plain", []string{"overwrite"}, "overwrite"},
		{"after global flags", []string{"--parent_dir", "p", "-i", "ze", "overwrite"}, "overwrite"},
		{"inline flag value", []string{"--parent_dir=p", "-i", "ze", "overwrite"}, "overwrite"},
		{"instance named like a verb", []string{"-i", "overwrite", "edit"}, "edit"},
		{"parent dir named like a verb", []string{"--parent_dir", "overwrite", "add"}, "add"},
		{"no subcommand", []string{"--parent_dir", "p"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := subcommandOf(tc.args); got != tc.want {
				t.Errorf("subcommandOf(%v) = %q, want %q", tc.args, got, tc.want)
			}
		})
	}

	t.Run("instance named like a build verb is not prepared", func(t *testing.T) {
		_, parent := writeInstanceFixture(t)
		got, cleanup, err := prepareArgs([]string{"--parent_dir", parent, "-i", "overwrite", "edit"})
		if err != nil {
			t.Fatalf("prepareArgs: %v", err)
		}
		defer cleanup()
		if got[1] != parent {
			t.Errorf("an edit was redirected to %q; the operator's change would be lost", got[1])
		}
	})
}

// TestZeGokFailsClosedOnUnpreparableParentDir verifies a --parent_dir that cannot
// be prepared is an error naming the path, never a silent fallthrough that would
// build from the tracked dir with the pins discarded.
//
// VALIDATES: fail-closed guard (ai/rules/fail-closed-guards.md).
// PREVENTS: a preparation failure degrading into an unpinned network build.
func TestZeGokFailsClosedOnUnpreparableParentDir(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nonexistent")
	args := []string{"--parent_dir", missing, "-i", "ze", "overwrite"}

	_, _, err := prepareArgs(args)
	if err == nil {
		t.Fatal("prepareArgs accepted a parent dir it could not prepare")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("error does not name the offending path %q: %v", missing, err)
	}
}

// TestZeGokPassesKernelPackage verifies the out-of-tree kernel selected by
// `make ze-gokrazy KERNEL_PKG=...` reaches the prepared instance, and that
// leaving it unset builds the pinned kernel.
//
// This is the wiring proof for AC-7/AC-8 at the entry point an operator actually
// uses. It replaces `make ze-kernel` writing a replace into the tracked
// gokrazy/ze/builddir/github.com/rtr7/kernel/go.mod, which dirtied the working
// tree and persisted until ze-kernel-clean was remembered.
//
// VALIDATES: AC-7 and AC-8 end to end through ze-gok.
// PREVENTS: the kernel selection silently falling back to the pin, handing an
// operator a pinned-kernel image while they believe they are testing their own.
func TestZeGokPassesKernelPackage(t *testing.T) {
	kernelModDir := func(parent string) string {
		return filepath.Join(parent, "ze", "builddir", "github.com", "rtr7", "kernel")
	}
	writeKernelMod := func(t *testing.T, parent string) {
		t.Helper()
		dir := kernelModDir(parent)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		const mod = "module gokrazy/build/ze\n\ngo 1.26.2\n\nrequire github.com/rtr7/kernel v0.0.0-20260403073601-5a996da3a37b // indirect\n"
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("selected package reaches the prepared copy", func(t *testing.T) {
		_, parent := writeInstanceFixture(t)
		writeKernelMod(t, parent)
		pkg := t.TempDir()
		t.Setenv("ze.gok.kernel-package", pkg)
		env.ResetCache()
		t.Cleanup(env.ResetCache)

		got, cleanup, err := prepareArgs([]string{"--parent_dir", parent, "-i", "ze", "overwrite"})
		if err != nil {
			t.Fatalf("prepareArgs: %v", err)
		}
		defer cleanup()

		data, err := os.ReadFile(filepath.Join(kernelModDir(got[1]), "go.mod"))
		if err != nil {
			t.Fatalf("read prepared kernel go.mod: %v", err)
		}
		if !strings.Contains(string(data), pkg) {
			t.Errorf("the selected kernel package %q did not reach the build:\n%s", pkg, data)
		}
		// And the tracked module is untouched.
		srcData, err := os.ReadFile(filepath.Join(kernelModDir(parent), "go.mod"))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(srcData), pkg) {
			t.Errorf("the tracked kernel go.mod was modified by a build:\n%s", srcData)
		}
	})

	t.Run("unset builds the pin", func(t *testing.T) {
		_, parent := writeInstanceFixture(t)
		writeKernelMod(t, parent)
		t.Setenv("ze.gok.kernel-package", "")
		env.ResetCache()
		t.Cleanup(env.ResetCache)

		got, cleanup, err := prepareArgs([]string{"--parent_dir", parent, "-i", "ze", "overwrite"})
		if err != nil {
			t.Fatalf("prepareArgs: %v", err)
		}
		defer cleanup()

		data, err := os.ReadFile(filepath.Join(kernelModDir(got[1]), "go.mod"))
		if err != nil {
			t.Fatalf("read prepared kernel go.mod: %v", err)
		}
		if strings.Contains(string(data), "replace github.com/rtr7/kernel") {
			t.Errorf("a kernel replace appeared though none was requested:\n%s", data)
		}
	})
}

// repoRoot walks up to the module root (the directory holding gokrazy/ze). It
// returns "" when the layout is absent.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "gokrazy", "ze", "config.json")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// snapshotTracked maps every file under dir to its size and modification time.
func snapshotTracked(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := make(map[string]string)
	if err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, statErr := d.Info()
		if statErr != nil {
			return statErr
		}
		out[path] = info.ModTime().UTC().Format("20060102150405.000000000") + "/" + strconv.FormatInt(info.Size(), 10)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestZeGokLeavesTrackedTreeClean verifies that preparing the REAL checked-in
// instance -- including with an out-of-tree kernel selected, the case that used
// to write into the tracked go.mod -- leaves every file under gokrazy/ze
// byte-identical, and removes the prepared copy afterwards.
//
// This is the acceptance criterion the whole spec exists for: a build step must
// not write to a tracked path, because in a checkout shared by concurrent
// sessions that is a cross-commit hazard.
//
// test-relax: the single t.Skip is an ENVIRONMENT guard on a NEW test, not a
// relaxation of existing coverage. It fires only outside a full checkout, where
// there is no checked-in instance to prepare and nothing to compare.
//
// VALIDATES: AC-2 through the same code path the bin/gok binary runs.
// PREVENTS: a build dirtying the working tree, and a prepared directory leaking.
func TestZeGokLeavesTrackedTreeClean(t *testing.T) {
	root := repoRoot(t)
	if root == "" {
		t.Skip("checked-in gokrazy instance not found; not a full checkout")
	}
	tracked := filepath.Join(root, "gokrazy", "ze")
	parent := filepath.Join(root, "gokrazy")

	for _, tc := range []struct {
		name       string
		kernelPkg  string
		wantInCopy bool
	}{
		{name: "pinned kernel"},
		{name: "out-of-tree kernel", kernelPkg: t.TempDir(), wantInCopy: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ze.gok.kernel-package", tc.kernelPkg)
			env.ResetCache()
			t.Cleanup(env.ResetCache)

			before := snapshotTracked(t, tracked)

			got, cleanup, err := prepareArgs([]string{"--parent_dir", parent, "-i", "ze", "overwrite"})
			if err != nil {
				t.Fatalf("prepareArgs: %v", err)
			}
			prepared := got[1]

			// When a kernel package was selected it must be in the COPY.
			if tc.wantInCopy {
				data, readErr := os.ReadFile(filepath.Join(prepared, "ze", "builddir", "github.com", "rtr7", "kernel", "go.mod"))
				if readErr != nil {
					t.Fatalf("read prepared kernel go.mod: %v", readErr)
				}
				if !strings.Contains(string(data), tc.kernelPkg) {
					t.Errorf("the selected kernel package did not reach the prepared copy:\n%s", data)
				}
			}

			cleanup()

			after := snapshotTracked(t, tracked)
			if len(before) != len(after) {
				t.Fatalf("gokrazy/ze changed size: %d files before, %d after", len(before), len(after))
			}
			for path, stamp := range before {
				if after[path] != stamp {
					t.Errorf("a build modified the tracked file %s", path)
				}
			}
			if _, err := os.Stat(prepared); !os.IsNotExist(err) {
				t.Errorf("the prepared directory %s was left behind: %v", prepared, err)
			}
		})
	}
}

// TestZeGokWithoutParentDirIsUntouched verifies that with no --parent_dir there
// is nothing repo-local to prepare, so the arguments pass through and gok uses
// its own default instance directory.
func TestZeGokWithoutParentDirIsUntouched(t *testing.T) {
	args := []string{"-i", "ze", "overwrite", "--full", "x.img"}

	got, cleanup, err := prepareArgs(args)
	if err != nil {
		t.Fatalf("prepareArgs: %v", err)
	}
	defer cleanup()

	if len(got) != len(args) {
		t.Fatalf("argument count changed: got %v, want %v", got, args)
	}
	for i := range args {
		if got[i] != args[i] {
			t.Errorf("arg[%d] = %q, want %q", i, got[i], args[i])
		}
	}
}
