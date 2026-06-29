// Design: plan/learned/851-install-10-pxe-staging.md -- staging unit tests

package provision

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProvisionStaging(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	kernelPath := filepath.Join(srcDir, "Image")
	initrdPath := filepath.Join(srcDir, "initrd.img.gz")

	if err := os.WriteFile(kernelPath, []byte("KERNEL"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(initrdPath, []byte("INITRD"), 0o644); err != nil {
		t.Fatal(err)
	}

	stageDir := t.TempDir()
	td := filepath.Join(stageDir, "tftp")
	bd := filepath.Join(stageDir, "boot")

	ipxeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(ipxeDir, "ipxe.pxe"), []byte("BIOS"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ipxeDir, "ipxe.efi"), []byte("UEFI"), 0o644); err != nil {
		t.Fatal(err)
	}

	sc := stagingConfig{
		KernelPath: kernelPath,
		InitrdPath: initrdPath,
		IPXEDir:    ipxeDir,
		TFTPDir:    td,
		BootDir:    bd,
	}
	if err := stageArtifacts(sc); err != nil {
		t.Fatalf("stageArtifacts: %v", err)
	}

	checks := []struct {
		path string
		want string
	}{
		{filepath.Join(bd, "vmlinuz"), "KERNEL"},
		{filepath.Join(bd, "initrd.img.gz"), "INITRD"},
		{filepath.Join(td, "ipxe.pxe"), "BIOS"},
		{filepath.Join(td, "ipxe.efi"), "UEFI"},
	}
	for _, c := range checks {
		data, err := os.ReadFile(c.path)
		if err != nil {
			t.Errorf("missing staged file %s: %v", c.path, err)
			continue
		}
		if string(data) != c.want {
			t.Errorf("%s: content = %q, want %q", c.path, string(data), c.want)
		}
	}
}

func TestStagedKernelNameConstant(t *testing.T) {
	t.Parallel()
	// VALIDATES: AC-12 expects the staged kernel filename to be one package-local
	// constant, wired into both the staging copy and the bootArtifactNames slice.
	// PREVENTS: the bare "vmlinuz" literal drifting between the copy and the validator.
	if stagedKernelName != "vmlinuz" {
		t.Fatalf("stagedKernelName = %q, want vmlinuz (gok/staging boot-artifact name must not change)", stagedKernelName)
	}
	if bootArtifactNames[0] != stagedKernelName {
		t.Errorf("bootArtifactNames[0] = %q, want it sourced from stagedKernelName %q", bootArtifactNames[0], stagedKernelName)
	}
	if bootArtifactNames[1] != stagedInitrdName {
		t.Errorf("bootArtifactNames[1] = %q, want it sourced from stagedInitrdName %q", bootArtifactNames[1], stagedInitrdName)
	}

	// The staged file lands under the constant name.
	srcDir := t.TempDir()
	kernelPath := filepath.Join(srcDir, "Image")
	if err := os.WriteFile(kernelPath, []byte("K"), 0o644); err != nil {
		t.Fatal(err)
	}
	bd := filepath.Join(t.TempDir(), "boot")
	if err := stageArtifacts(stagingConfig{KernelPath: kernelPath, BootDir: bd, TFTPDir: filepath.Join(t.TempDir(), "tftp")}); err != nil {
		t.Fatalf("stageArtifacts: %v", err)
	}
	if _, err := os.Stat(filepath.Join(bd, stagedKernelName)); err != nil {
		t.Errorf("kernel not staged as %q: %v", stagedKernelName, err)
	}
}

func TestProvisionStagingMissing(t *testing.T) {
	t.Parallel()

	stageDir := t.TempDir()
	sc := stagingConfig{
		TFTPDir: filepath.Join(stageDir, "tftp"),
		BootDir: filepath.Join(stageDir, "boot"),
	}

	if err := stageArtifacts(sc); err != nil {
		t.Fatalf("stageArtifacts with no files: %v", err)
	}

	err := validateStaging(sc)
	if err == nil {
		t.Fatal("expected validation error for missing artifacts")
	}

	msg := err.Error()
	for _, name := range []string{"vmlinuz", "initrd.img.gz", "ipxe.pxe", "ipxe.efi"} {
		if !strings.Contains(msg, name) {
			t.Errorf("error should mention %s: %s", name, msg)
		}
	}
}

func TestProvisionPreStaged(t *testing.T) {
	t.Parallel()

	stageDir := t.TempDir()
	td := filepath.Join(stageDir, "tftp")
	bd := filepath.Join(stageDir, "boot")

	if err := os.MkdirAll(td, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(bd, 0o750); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ipxe.pxe", "ipxe.efi"} {
		if err := os.WriteFile(filepath.Join(td, name), []byte("bin"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"vmlinuz", "initrd.img.gz"} {
		if err := os.WriteFile(filepath.Join(bd, name), []byte("boot"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	sc := stagingConfig{TFTPDir: td, BootDir: bd}
	if err := stageArtifacts(sc); err != nil {
		t.Fatalf("stageArtifacts: %v", err)
	}
	if err := validateStaging(sc); err != nil {
		t.Fatalf("validateStaging: %v (files were pre-staged)", err)
	}
}

func TestProvisionMissingKernelFile(t *testing.T) {
	t.Parallel()

	stageDir := t.TempDir()
	sc := stagingConfig{
		KernelPath: "/nonexistent/kernel",
		TFTPDir:    filepath.Join(stageDir, "tftp"),
		BootDir:    filepath.Join(stageDir, "boot"),
	}

	err := stageArtifacts(sc)
	if err == nil {
		t.Fatal("expected error for nonexistent kernel file")
	}
}

func TestProvisionConfigBootScriptURL(t *testing.T) {
	t.Parallel()

	cfg := generateConfig(configParams{
		iface:         "eth0",
		network:       "198.19.255.0/24",
		image:         "/images/ze.img",
		serverIP:      "198.19.255.1",
		sshUsername:   "admin",
		sshPassHash:   "$2a$10$hash",
		bootScriptURL: "http://198.19.255.1/install/boot/boot.ipxe",
	})

	if !strings.Contains(cfg, "boot-script-url http://198.19.255.1/install/boot/boot.ipxe") {
		t.Errorf("generated config missing boot-script-url:\n%s", cfg)
	}
}

func TestProvisionConfigShellAuth(t *testing.T) {
	t.Parallel()

	want := shellAuthHash("hunter2")
	cfg := generateConfig(configParams{
		iface:           "eth0",
		network:         "198.19.255.0/24",
		image:           "/images/ze.img",
		serverIP:        "198.19.255.1",
		sshUsername:     "admin",
		sshPassHash:     "$2a$10$hash",
		shellAuthSHA256: want,
		bootScriptURL:   "http://198.19.255.1/install/boot/boot.ipxe",
	})

	if !strings.Contains(cfg, "shell-auth-sha256 \""+want+"\"") {
		t.Errorf("generated config missing shell-auth-sha256:\n%s", cfg)
	}
}

func TestShellAuthHash(t *testing.T) {
	t.Parallel()

	// sha256 of the empty string -- a well-known vector -- validates the helper
	// produces a lowercase hex sha256 digest.
	if got := shellAuthHash(""); got != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Errorf("shellAuthHash(%q) = %q", "", got)
	}
	h := shellAuthHash("hunter2")
	if len(h) != 64 {
		t.Fatalf("shellAuthHash length = %d, want 64", len(h))
	}
	if again := shellAuthHash("hunter2"); again != h {
		t.Errorf("shellAuthHash not deterministic: %q vs %q", again, h)
	}
}
