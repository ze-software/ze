package appliance

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

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
