package vendorpatch

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/mod/modfile"
)

const gokrazyRuntimeRebootPatch = "gokrazy/patches/reboot-ack.patch"
const gokrazyRuntimeModule = "github.com/gokrazy/gokrazy"

func TestRuntimeSourceComparisonChecksTheWholeFileSet(t *testing.T) {
	for _, tc := range []struct {
		name          string
		local         map[string]string
		upstreamExtra map[string]string
		wantError     string
	}{
		{name: "same", local: map[string]string{"first.go": "first", "second.go": "second"}},
		{name: "dropped upstream file", local: map[string]string{"first.go": "first"}, wantError: "upstream file second.go is missing locally"},
		{name: "unexpected local file", local: map[string]string{"first.go": "first", "second.go": "second", "extra.go": "extra"}, wantError: "local file extra.go is absent upstream"},
		{name: "changed contents", local: map[string]string{"first.go": "changed", "second.go": "second"}, wantError: "file first.go differs from upstream"},
		{name: "omitted documentation trees", local: map[string]string{"first.go": "first", "second.go": "second"}, upstreamExtra: map[string]string{"docs/guide.html": "documentation", "website/config.toml": "website"}},
		{name: "dropped docs named source", local: map[string]string{"first.go": "first", "second.go": "second"}, upstreamExtra: map[string]string{"docs.go": "package gokrazy"}, wantError: "upstream file docs.go is missing locally"},
		{name: "dropped nested docs asset", local: map[string]string{"first.go": "first", "second.go": "second"}, upstreamExtra: map[string]string{"internal/assets/docs/asset.txt": "embedded asset"}, wantError: "upstream file internal/assets/docs/asset.txt is missing locally"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := t.TempDir()
			local := t.TempDir()
			for name, content := range map[string]string{"first.go": "first", "second.go": "second"} {
				if err := os.WriteFile(filepath.Join(upstream, name), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			for name, content := range tc.upstreamExtra {
				path := filepath.Join(upstream, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			for name, content := range tc.local {
				if err := os.WriteFile(filepath.Join(local, name), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			err := compareRuntimeSources(local, upstream)
			if tc.wantError == "" {
				if err != nil {
					t.Fatalf("equal sources failed comparison: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("source comparison accepted %s", tc.name)
			}
			if err.Error() != tc.wantError {
				t.Fatalf("source comparison error = %v, want %s", err, tc.wantError)
			}
		})
	}
}

func TestGokrazyRuntimeRebootPatchApplied(t *testing.T) {
	assertVendorPatchApplied(t, gokrazyRuntimeRebootPatch, "reapply gokrazy/patches/reboot-ack.patch to the pinned tracked runtime")
}

// TestPinnedGokrazyRuntimeMatchesUpstream extends the existing normalization
// proof to the pinned runtime source tracked outside root vendor/. It reverses
// its one canonical diff and compares the maintained runtime source and assets
// with upstream, excluding only the untracked top-level documentation trees.
func TestPinnedGokrazyRuntimeMatchesUpstream(t *testing.T) {
	root := repoRoot(t)
	body, err := os.ReadFile(filepath.Join(root, gokrazyRuntimeRebootPatch))
	if err != nil {
		t.Fatal(err)
	}
	module, version, err := Stamp(body)
	if err != nil {
		t.Fatal(err)
	}
	if module != gokrazyRuntimeModule {
		t.Fatalf("runtime patch module = %q, want %s", module, gokrazyRuntimeModule)
	}
	manifests, err := filepath.Glob(filepath.Join(root, "gokrazy", "*", "builddir", "github.com", "gokrazy", "gokrazy", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if len(manifests) == 0 {
		t.Fatal("no appliance runtime pin was checked")
	}
	for _, path := range manifests {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		manifest, err := modfile.Parse(path, data, nil)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, requirement := range manifest.Require {
			if requirement.Mod.Path == module {
				found = true
				if requirement.Mod.Version != version {
					t.Fatalf("%s pins %s, but runtime patch was reviewed against %s", path, requirement.Mod.Version, version)
				}
			}
		}
		if !found {
			t.Fatalf("%s has no runtime module requirement", path)
		}
	}
	relative := "gokrazy/modcache/" + module + "@" + version
	targets := patchTargets(string(body))
	if len(targets) == 0 {
		t.Fatal("runtime patch has no source target")
	}
	for _, target := range targets {
		if !strings.HasPrefix(target, relative+"/") {
			t.Fatalf("runtime patch changes %s outside the selected pin %s", target, relative)
		}
	}
	upstream := upstreamDir(t, root, module, version)
	work := t.TempDir()
	copied := copyModuleTree(t, filepath.Join(root, filepath.FromSlash(relative)), filepath.Join(work, filepath.FromSlash(relative)), module, nil)
	gitApplyReverse(t, work, filepath.Join(root, gokrazyRuntimeRebootPatch), gokrazyRuntimeRebootPatch)
	for _, path := range copied {
		_, err := os.Stat(filepath.Join(work, filepath.FromSlash(relative), path))
		if os.IsNotExist(err) {
			switch path {
			case "reboot_ack_test.go", "testboot_cmdline_test.go":
			default:
				t.Fatalf("reversing the runtime patch removed unexpected source %s", path)
			}
			if _, err := os.Stat(filepath.Join(upstream, path)); !os.IsNotExist(err) {
				t.Fatalf("removed runtime test %s is not a patch-only addition: %v", path, err)
			}
			continue // Only the canonical patch's two new regressions disappear.
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := compareRuntimeSources(filepath.Join(work, filepath.FromSlash(relative)), upstream); err != nil {
		t.Fatalf("runtime differs from %s@%s plus its canonical patch: %v", module, version, err)
	}
}

// compareRuntimeSources compares both complete maintained regular-file sets.
// Walking only the local copy would miss an upstream file dropped from it.
func compareRuntimeSources(local, upstream string) error {
	got, err := runtimeRegularFiles(local)
	if err != nil {
		return err
	}
	want, err := runtimeRegularFiles(upstream)
	if err != nil {
		return err
	}
	for path, content := range want {
		localContent, found := got[path]
		if !found {
			return fmt.Errorf("upstream file %s is missing locally", path)
		}
		if !bytes.Equal(localContent, content) {
			return fmt.Errorf("file %s differs from upstream", path)
		}
	}
	for path := range got {
		if _, found := want[path]; !found {
			return fmt.Errorf("local file %s is absent upstream", path)
		}
	}
	return nil
}

func runtimeRegularFiles(root string) (map[string][]byte, error) {
	files := make(map[string][]byte)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			// These exact top-level non-source trees are intentionally omitted
			// by gokrazy/modcache/.gitignore; nested assets stay in the proof.
			if path == filepath.Join(root, "docs") {
				return filepath.SkipDir
			}
			if path == filepath.Join(root, "website") {
				return filepath.SkipDir
			}
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = content
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}
