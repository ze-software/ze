package appliance

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// shippedKernelProfiles is the exact profile set tools/installer-kernel/ ships.
// tools/installer-kernel/PROFILES.md documents the FORM of a profile and names
// n100, which ships no files, so this list is the only manifest of the set.
// kernel.config is the base fragment and registeredKernelProfiles excludes it.
var shippedKernelProfiles = []string{"hardware", "hardware-kms", "qemu"}

func writeKernelProfileFile(t *testing.T, dir, name, config, require string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name+".config"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	if require != "" {
		if err := os.WriteFile(filepath.Join(dir, name+".require"), []byte(require), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestProfileNameToken(t *testing.T) {
	// VALIDATES: AC-8 expects profile names "../x", empty, or with illegal chars to be rejected.
	// PREVENTS: path traversal through profile names used as filenames, URLs, environment values, or argv.
	tests := []struct {
		name    string
		profile string
		wantErr bool
	}{
		{name: "qemu", profile: "qemu"},
		{name: "hardware-kms", profile: "hardware-kms"},
		{name: "digit", profile: "7edge"},
		{name: "empty", profile: "", wantErr: true},
		{name: "uppercase", profile: "Hardware", wantErr: true},
		{name: "underscore", profile: "hardware_kms", wantErr: true},
		{name: "dot", profile: "hardware.kms", wantErr: true},
		{name: "slash", profile: "hardware/kms", wantErr: true},
		{name: "traversal", profile: "../x", wantErr: true},
		{name: "leading dash", profile: "-bad", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateKernelProfileName(tt.profile)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("validateKernelProfileName(%q) succeeded, want error", tt.profile)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateKernelProfileName(%q): %v", tt.profile, err)
			}
		})
	}
}

func TestResolveProfileFragments(t *testing.T) {
	// VALIDATES: AC-7 expects # ze-base: hardware to layer hardware before the selected profile.
	// PREVENTS: hardware-kms losing the hardware fragment or accepting nested base chains.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "kernel.config"), []byte("CONFIG_BASE=y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "kernel.require"), []byte("CONFIG_BASE\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeKernelProfileFile(t, dir, "hardware", "CONFIG_HW=y\n", "CONFIG_HW\n")
	writeKernelProfileFile(t, dir, "hardware-kms", "# ze-base: hardware\nCONFIG_KMS=y\n", "CONFIG_KMS\n")

	got, err := resolveKernelProfile(dir, "hardware-kms")
	if err != nil {
		t.Fatalf("resolveKernelProfile: %v", err)
	}
	wantFragments := []string{
		filepath.Join(dir, "kernel.config"),
		filepath.Join(dir, "hardware.config"),
		filepath.Join(dir, "hardware-kms.config"),
	}
	if !reflect.DeepEqual(got.Fragments, wantFragments) {
		t.Fatalf("fragments = %#v, want %#v", got.Fragments, wantFragments)
	}
	wantManifests := []string{
		filepath.Join(dir, "kernel.require"),
		filepath.Join(dir, "hardware.require"),
		filepath.Join(dir, "hardware-kms.require"),
	}
	if !reflect.DeepEqual(got.Manifests, wantManifests) {
		t.Fatalf("manifests = %#v, want %#v", got.Manifests, wantManifests)
	}

	writeKernelProfileFile(t, dir, "nested-base", "# ze-base: hardware-kms\nCONFIG_NESTED=y\n", "CONFIG_NESTED\n")
	_, err = resolveKernelProfile(dir, "nested-base")
	if err == nil || !strings.Contains(err.Error(), "only one level") {
		t.Fatalf("nested base error = %v, want only one level", err)
	}
}

func TestResolveSharedInclude(t *testing.T) {
	// VALIDATES: AC-3/AC-4/AC-5 expect `# ze-include: <name>` to pull a shared fragment
	// from the common dir exactly once, and resolution to FAIL when the shared fragment is
	// missing; a profile without the directive must not get it.
	// PREVENTS: the shared efi-console subset silently dropping out of a kernel.
	dir := t.TempDir()
	t.Chdir(dir)
	// srcDir and kernelCommonDir are both repo-relative in production; keep them
	// relative here so the resolved fragment paths are consistent.
	regAbs := filepath.Join(dir, "reg")
	commonAbs := filepath.Join(dir, kernelCommonDir)
	if err := os.MkdirAll(regAbs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(commonAbs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeKernelProfileFile(t, regAbs, "kernel", "CONFIG_BASE=y\n", "CONFIG_BASE\n")
	writeKernelProfileFile(t, regAbs, "runtime", "# ze-include: efi-console\nCONFIG_RT=y\n", "CONFIG_RT\n")
	writeKernelProfileFile(t, regAbs, "qemu", "CONFIG_Q=y\n", "CONFIG_Q\n")
	writeKernelProfileFile(t, commonAbs, "efi-console", "CONFIG_SHARED=y\n", "# no required symbols\n")

	got, err := resolveKernelProfile("reg", "runtime")
	if err != nil {
		t.Fatalf("resolveKernelProfile(runtime): %v", err)
	}
	incConfig := filepath.Join(kernelCommonDir, "efi-console.config")
	if got.Fragments[len(got.Fragments)-1] != incConfig {
		t.Fatalf("fragments = %#v, want %s appended last", got.Fragments, incConfig)
	}
	incRequire := filepath.Join(kernelCommonDir, "efi-console.require")
	if got.Manifests[len(got.Manifests)-1] != incRequire {
		t.Fatalf("manifests = %#v, want %s appended last", got.Manifests, incRequire)
	}

	// AC-5: a profile without the directive must NOT get the shared fragment.
	qemu, err := resolveKernelProfile("reg", "qemu")
	if err != nil {
		t.Fatalf("resolveKernelProfile(qemu): %v", err)
	}
	for _, f := range qemu.Fragments {
		if strings.Contains(f, "efi-console") {
			t.Fatalf("qemu unexpectedly includes shared fragment: %#v", qemu.Fragments)
		}
	}

	// Deduplication: an include in both base and profile is added once.
	writeKernelProfileFile(t, regAbs, "kernel", "# ze-include: efi-console\nCONFIG_BASE=y\n", "CONFIG_BASE\n")
	dedup, err := resolveKernelProfile("reg", "runtime")
	if err != nil {
		t.Fatalf("resolveKernelProfile(runtime) after dup include: %v", err)
	}
	count := 0
	for _, f := range dedup.Fragments {
		if f == incConfig {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("shared fragment appears %d times, want 1: %#v", count, dedup.Fragments)
	}

	// AC-4: deleting the shared fragment makes resolution FAIL at include-resolution.
	if err := os.Remove(incConfig); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveKernelProfile("reg", "runtime"); err == nil {
		t.Fatal("expected resolution to fail when shared fragment is missing")
	}
}

func TestResolveNestedIncludeRejected(t *testing.T) {
	// VALIDATES: a shared fragment that itself declares # ze-include is rejected
	// loudly (one level only), instead of the nested include being silently ignored.
	dir := t.TempDir()
	t.Chdir(dir)
	regAbs := filepath.Join(dir, "reg")
	commonAbs := filepath.Join(dir, kernelCommonDir)
	if err := os.MkdirAll(regAbs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(commonAbs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeKernelProfileFile(t, regAbs, "kernel", "CONFIG_BASE=y\n", "CONFIG_BASE\n")
	writeKernelProfileFile(t, regAbs, "runtime", "# ze-include: efi-console\nCONFIG_RT=y\n", "CONFIG_RT\n")
	// The shared fragment itself declares a (nested) include.
	writeKernelProfileFile(t, commonAbs, "efi-console", "# ze-include: more\nCONFIG_SHARED=y\n", "# none\n")
	writeKernelProfileFile(t, commonAbs, "more", "CONFIG_MORE=y\n", "# none\n")

	_, err := resolveKernelProfile("reg", "runtime")
	if err == nil || !strings.Contains(err.Error(), "nested includes are not supported") {
		t.Fatalf("nested include error = %v, want 'nested includes are not supported'", err)
	}
}

func TestResolveProfileRequiresManifest(t *testing.T) {
	// VALIDATES: AC-4 expects a .config without matching .require to fail resolution.
	// PREVENTS: unverified custom profiles entering the kernel build path.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "kernel.config"), []byte("CONFIG_BASE=y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "kernel.require"), []byte("CONFIG_BASE\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "foo.config"), []byte("CONFIG_FOO=y\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := resolveKernelProfile(dir, "foo")
	if err == nil || !strings.Contains(err.Error(), "no require manifest") {
		t.Fatalf("resolveKernelProfile missing require error = %v, want no require manifest", err)
	}
}

func TestEnumerateRegistry(t *testing.T) {
	// VALIDATES: AC-10 expects profile discovery to scan .config+.require pairs instead of using old constants.
	// PREVENTS: iso --check and ze doctor missing newly added valid profiles.
	dir := t.TempDir()
	for _, name := range []string{"kernel", "qemu", "hardware", "custom"} {
		if err := os.WriteFile(filepath.Join(dir, name+".config"), []byte("CONFIG_X=y\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+".require"), []byte("CONFIG_X\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "draft.config"), []byte("CONFIG_DRAFT=y\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := registeredKernelProfiles(dir)
	if err != nil {
		t.Fatalf("registeredKernelProfiles: %v", err)
	}
	want := []string{"custom", "hardware", "qemu"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("profiles = %#v, want %#v", got, want)
	}
}

func TestRegisteredKernelProfilesShippedSet(t *testing.T) {
	// VALIDATES: AC-2 and AC-3 expect tools/installer-kernel/ to register the shipped profiles and nothing else.
	// PREVENTS: a test fixture left in the tracked source directory becoming a buildable kernel profile.
	t.Run("real directory", func(t *testing.T) {
		// A Go test runs with its package directory as the working directory.
		const srcDir = "../../tools/installer-kernel"

		got, err := registeredKernelProfiles(srcDir)
		if err != nil {
			t.Fatalf("registeredKernelProfiles(%s): %v", srcDir, err)
		}
		shipped := make(map[string]bool, len(shippedKernelProfiles))
		for _, name := range shippedKernelProfiles {
			shipped[name] = true
		}
		registered := make(map[string]bool, len(got))
		for _, name := range got {
			registered[name] = true
			if shipped[name] {
				continue
			}
			t.Errorf("tools/installer-kernel/ registers profile %q, which this repository does not ship.\n"+
				"Leaked test fixture: delete tools/installer-kernel/%s.config and tools/installer-kernel/%s.require, and make the test that wrote them build its profile in a scratch directory.\n"+
				"New shipped profile: add %q to shippedKernelProfiles in this file and document it in tools/installer-kernel/PROFILES.md",
				name, name, name, name)
		}
		for _, name := range shippedKernelProfiles {
			if registered[name] {
				continue
			}
			t.Errorf("tools/installer-kernel/ no longer registers the shipped profile %q.\n"+
				"Restore tools/installer-kernel/%s.config and tools/installer-kernel/%s.require, or drop %q from shippedKernelProfiles in this file and from tools/installer-kernel/PROFILES.md",
				name, name, name, name)
		}
	})

	t.Run("stray pair is registered", func(t *testing.T) {
		// The discrimination proof for the case above. registeredKernelProfiles
		// registers every <name>.config plus <name>.require pair it finds, so a
		// fixture pair left in the real directory turns that case red.
		dir := t.TempDir()
		writeKernelProfileFile(t, dir, "kernel", "CONFIG_BASE=y\n", "CONFIG_BASE\n")
		for _, name := range shippedKernelProfiles {
			writeKernelProfileFile(t, dir, name, "CONFIG_X=y\n", "CONFIG_X\n")
		}
		writeKernelProfileFile(t, dir, "fixture", "CONFIG_FIXTURE=y\n", "CONFIG_FIXTURE\n")

		got, err := registeredKernelProfiles(dir)
		if err != nil {
			t.Fatalf("registeredKernelProfiles: %v", err)
		}
		want := append([]string{"fixture"}, shippedKernelProfiles...)
		sort.Strings(want)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("profiles = %#v, want %#v", got, want)
		}
	})
}
