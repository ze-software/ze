// VALIDATES: `le build gokrazy` prepares the gokrazy instance for an image
// build and leaves every other gok subcommand and the tracked tree alone.
// PREVENTS: an image built from, or written into, the tracked gokrazy dir.

package buildgokrazy

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/appliance/instance"
	"github.com/ze-software/ze/internal/core/env"
)

// fakeRuntimeKernel makes runtimeKernelTreeFn answer a fixture runtime kernel
// tree, and answers the arches it was asked for.
func fakeRuntimeKernel(t *testing.T) *[]string {
	t.Helper()
	asked := &[]string{}
	old := runtimeKernelTreeFn
	runtimeKernelTreeFn = func(arch string) (string, error) {
		*asked = append(*asked, arch)
		tree := filepath.Join(t.TempDir(), "runtime-"+arch)
		if err := os.MkdirAll(filepath.Join(tree, "lib", "modules", "7.2.9-ze"), 0o755); err != nil {
			t.Fatal(err)
		}
		offset, magic := 0x202, "HdrS"
		if arch == "arm64" {
			offset, magic = 0x38, "ARMd"
		}
		image := make([]byte, 0x400)
		copy(image[offset:], magic)
		if err := os.WriteFile(filepath.Join(tree, "vmlinuz"), image, 0o644); err != nil {
			t.Fatal(err)
		}
		// The provenance a 7.2.9 build writes; Prepare derives the image's GPLv2
		// notice from it and refuses a tree without one.
		provenance := "version=7.2.9\ntarget=runtime\nprofile=runtime\narch=" + arch + "\nmodules=yes\nbuilder=docker\n" +
			"source-url=https://cdn.kernel.org/pub/linux/kernel/v7.x/linux-7.2.9.tar.xz\n" +
			"source-sha256=b4c5dfbe51a364a6c7f03869200f88c8e1f77403539005f14b7fc6bc91b8d8ba\n"
		if err := os.WriteFile(filepath.Join(tree, "kernel.version"), []byte(provenance), 0o644); err != nil {
			t.Fatal(err)
		}
		return tree, nil
	}
	t.Cleanup(func() { runtimeKernelTreeFn = old })
	return asked
}

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
	kernelMod := filepath.Join(parent, "ze", "builddir", filepath.FromSlash(instance.KernelModule))
	if err := os.MkdirAll(kernelMod, 0o755); err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(kernelMod, "go.mod"), "module gokrazy/build/ze\n\ngo 1.26\n\nrequire "+instance.KernelModule+" v0.0.0\n")
	write(filepath.Join(parent, "ze", "config.json"), `{"Hostname":"ze"}`)
	fakeRuntimeKernel(t)
	return root, parent
}

// TestBuildGokrazyPreparesTheInstance verifies `le build gokrazy` rewrites --parent_dir to a prepared
// copy under the project tmp/ before gok sees it, in both pflag spellings, and
// that the copy carries the builddir pins.
//
// VALIDATES: AC-13 and D-1c -- `./ze appliance build` gets a prepared instance with no
// change to internal/appliance/cmd_build.go.
// PREVENTS: an image build running from, or writing to, the tracked gokrazy dir.
func TestBuildGokrazyPreparesTheInstance(t *testing.T) {
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

// TestBuildGokrazyLeavesMutatingSubcommandsAlone verifies a subcommand that EDITS the
// instance is never redirected to a throwaway copy. Preparing `gok edit` or
// `gok add` would write the operator's change into a temp dir and then delete
// it, losing the edit silently.
//
// VALIDATES: preparation is scoped to image-building subcommands.
// PREVENTS: silently discarding an operator's instance edit.
func TestBuildGokrazyLeavesMutatingSubcommandsAlone(t *testing.T) {
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

// TestBuildGokrazyIdentifiesTheSubcommandNotAnyToken verifies the build/mutate decision
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
func TestBuildGokrazyIdentifiesTheSubcommandNotAnyToken(t *testing.T) {
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

// TestBuildGokrazyFailsClosedOnUnpreparableParentDir verifies a --parent_dir that cannot
// be prepared is an error naming the path, never a silent fallthrough that would
// build from the tracked dir with the pins discarded.
//
// VALIDATES: fail-closed guard (ai/rules/evidence.md).
// PREVENTS: a preparation failure degrading into an unpinned network build.
func TestBuildGokrazyFailsClosedOnUnpreparableParentDir(t *testing.T) {
	fakeRuntimeKernel(t)
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

// TestRunResolvesRuntimeKernelByDefault verifies `le build gokrazy` resolves
// ze's runtime kernel for the image's GOARCH and the prepared copy's kernel
// module points at the package assembled from it, while the tracked module is
// untouched.
//
// VALIDATES: the `./le build gokrazy` wiring row: same resolver, then Prepare.
// PREVENTS: an image whose kernel gok resolves on its own.
func TestRunResolvesRuntimeKernelByDefault(t *testing.T) {
	_, parent := writeInstanceFixture(t)
	asked := fakeRuntimeKernel(t)
	t.Setenv("GOARCH", "arm64")

	got, cleanup, err := prepareArgs([]string{"--parent_dir", parent, "-i", "ze", "overwrite"})
	if err != nil {
		t.Fatalf("prepareArgs: %v", err)
	}
	defer cleanup()

	if len(*asked) != 1 || (*asked)[0] != "arm64" {
		t.Fatalf("runtime kernel resolved for %v, want exactly [arm64] from GOARCH", *asked)
	}
	kernelMod := filepath.FromSlash(instance.KernelModule)
	data, err := os.ReadFile(filepath.Join(got[1], "ze", "builddir", kernelMod, "go.mod"))
	if err != nil {
		t.Fatalf("read prepared kernel go.mod: %v", err)
	}
	if !strings.Contains(string(data), "=> "+filepath.Join(got[1], "kernel")) {
		t.Errorf("the prepared kernel module does not point at the assembled package:\n%s", data)
	}
	srcData, err := os.ReadFile(filepath.Join(parent, "ze", "builddir", kernelMod, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(srcData), "replace") {
		t.Errorf("the tracked kernel go.mod was modified by a build:\n%s", srcData)
	}
}

// TestRunHasNoKernelOverride verifies no setting selects another kernel package:
// ze.gok.kernel-package is not registered, and setting it in the environment
// changes nothing about the kernel the build resolves.
//
// VALIDATES: AC-6 as the owner rewrote it (2026-10-09): ze's own kernel only.
// PREVENTS: a route that lets an image carry a different kernel.
func TestRunHasNoKernelOverride(t *testing.T) {
	const knob = "ze.gok.kernel-package"
	if env.IsRegistered(knob) {
		t.Fatalf("%s is registered; an image must carry ze's runtime kernel and no other", knob)
	}
	_, parent := writeInstanceFixture(t)
	asked := fakeRuntimeKernel(t)
	t.Setenv(knob, t.TempDir())
	t.Setenv("GOARCH", "amd64")

	_, cleanup, err := prepareArgs([]string{"--parent_dir", parent, "-i", "ze", "overwrite"})
	if err != nil {
		t.Fatalf("prepareArgs: %v", err)
	}
	defer cleanup()
	if len(*asked) != 1 {
		t.Errorf("runtime kernel resolved %d times with %s set, want once", len(*asked), knob)
	}
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

// TestBuildGokrazyLeavesTrackedTreeClean verifies that preparing the REAL checked-in
// instance -- including with an out-of-tree kernel selected, the case that used
// to write into the tracked go.mod -- leaves every file under gokrazy/ze
// byte-identical, and removes the prepared copy afterwards.
//
// This is the acceptance criterion the whole spec exists for: a build step must
// not write to a tracked path, because in a checkout shared by concurrent
// sessions that is a cross-commit hazard.
//
// the single t.Skip is an ENVIRONMENT guard on a NEW test, not a
// relaxation of existing coverage. It fires only outside a full checkout, where
// there is no checked-in instance to prepare and nothing to compare.
//
// VALIDATES: AC-2 through the same code path the bin/gok binary runs.
// PREVENTS: a build dirtying the working tree, and a prepared directory leaking.
func TestBuildGokrazyLeavesTrackedTreeClean(t *testing.T) {
	root := repoRoot(t)
	if root == "" {
		t.Skip("checked-in gokrazy instance not found; not a full checkout")
	}
	tracked := filepath.Join(root, "gokrazy", "ze")
	parent := filepath.Join(root, "gokrazy")

	fakeRuntimeKernel(t)
	t.Setenv("GOARCH", runtime.GOARCH)
	for _, tc := range []struct {
		name string
	}{
		{name: "runtime kernel"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := snapshotTracked(t, tracked)

			got, cleanup, err := prepareArgs([]string{"--parent_dir", parent, "-i", "ze", "overwrite"})
			if err != nil {
				t.Fatalf("prepareArgs: %v", err)
			}
			prepared := got[1]

			// The kernel package is assembled in the COPY.
			if _, statErr := os.Stat(filepath.Join(prepared, "kernel", "vmlinuz")); statErr != nil {
				t.Errorf("the prepared copy carries no kernel package: %v", statErr)
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

// TestBuildGokrazyWithoutParentDirIsUntouched verifies that with no --parent_dir there
// is nothing repo-local to prepare, so the arguments pass through and gok uses
// its own default instance directory.
func TestBuildGokrazyWithoutParentDirIsUntouched(t *testing.T) {
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
