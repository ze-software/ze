// Design: docs/architecture/provisioning/pxe-staging.md -- staging unit tests

package provision

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/rescueauth"
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

// test-relax: shellAuthHash was deleted, not weakened. It hashed the ADMIN
// PASSWORD with an unsalted sha256 and published the digest on an unauthenticated
// PXE cmdline. Its replacement is a dedicated random rescue token behind salted
// argon2id, covered by internal/core/rescueauth tests; the config-emission half
// is retested below against the new leaf.
func TestProvisionConfigRescueAuth(t *testing.T) {
	t.Parallel()

	token, want, err := rescueauth.NewValue()
	if err != nil {
		t.Fatalf("NewValue: %v", err)
	}
	cfg := generateConfig(configParams{
		iface:         "eth0",
		network:       "198.19.255.0/24",
		image:         "/images/ze.img",
		serverIP:      "198.19.255.1",
		sshUsername:   "admin",
		sshPassHash:   "$2a$10$hash",
		rescueAuth:    want,
		bootScriptURL: "http://198.19.255.1/install/boot/boot.ipxe",
	})

	if !strings.Contains(cfg, "rescue-auth \""+want+"\"") {
		t.Errorf("generated config missing rescue-auth:\n%s", cfg)
	}
	// The generated config must never carry the token itself, only the digest.
	if strings.Contains(cfg, token) {
		t.Errorf("generated config leaks the rescue token:\n%s", cfg)
	}
	if strings.Contains(cfg, "shell-auth") {
		t.Errorf("generated config still emits the removed shell-auth leaf:\n%s", cfg)
	}
}

// VALIDATES: AC-1 -- the provisioner shows the operator the token exactly once,
// and the printed text says it cannot be recovered.
// PREVENTS: Minting a credential the operator never sees, which turns every
// failed PXE install into an unrecoverable machine.
func TestPrintRescueToken(t *testing.T) {
	t.Parallel()

	var out strings.Builder
	printRescueToken(&out, "dead-beef-cafe-1234-5678")

	got := out.String()
	if !strings.Contains(got, "dead-beef-cafe-1234-5678") {
		t.Errorf("token not shown to the operator: %q", got)
	}
	if !strings.Contains(got, "rescue") {
		t.Errorf("output does not say what the token is for: %q", got)
	}
	if !strings.Contains(got, "not recoverable") {
		t.Errorf("output does not warn the token cannot be recovered: %q", got)
	}
}

func TestPXEDirs(t *testing.T) {
	t.Parallel()

	bootDir, tftpDir := pxeDirs("build/pxe")
	if !filepath.IsAbs(bootDir) || !filepath.IsAbs(tftpDir) {
		t.Fatalf("serve dirs must be absolute: boot=%q tftp=%q", bootDir, tftpDir)
	}
	if filepath.Base(bootDir) != "boot" {
		t.Errorf("bootDir base = %q, want boot", filepath.Base(bootDir))
	}
	if filepath.Base(tftpDir) != "tftp" {
		t.Errorf("tftpDir base = %q, want tftp", filepath.Base(tftpDir))
	}
	if filepath.Base(filepath.Dir(bootDir)) != "pxe" {
		t.Errorf("bootDir parent = %q, want pxe", filepath.Base(filepath.Dir(bootDir)))
	}

	// An empty --pxe-dir resolves to the same paths as the explicit default.
	b2, t2 := pxeDirs("")
	if b2 != bootDir || t2 != tftpDir {
		t.Errorf("empty pxe-dir = (%q,%q), want (%q,%q)", b2, t2, bootDir, tftpDir)
	}
}

func TestProvisionConfigServeDirs(t *testing.T) {
	t.Parallel()

	cfg := generateConfig(configParams{
		iface:       "eth0",
		network:     "198.19.255.0/24",
		image:       "/images/ze.img",
		serverIP:    "198.19.255.1",
		sshUsername: "admin",
		sshPassHash: "$2a$10$hash",
		bootDir:     "/srv/pxe/boot",
		tftpDir:     "/srv/pxe/tftp",
	})
	if !strings.Contains(cfg, "boot-directory /srv/pxe/boot;") {
		t.Errorf("config missing custom boot-directory:\n%s", cfg)
	}
	if !strings.Contains(cfg, "root-directory /srv/pxe/tftp;") {
		t.Errorf("config missing custom root-directory:\n%s", cfg)
	}

	// Empty dirs fall back to the consolidated build/pxe defaults.
	def := generateConfig(configParams{
		iface:       "eth0",
		network:     "198.19.255.0/24",
		image:       "/images/ze.img",
		serverIP:    "198.19.255.1",
		sshUsername: "admin",
		sshPassHash: "$2a$10$hash",
	})
	if !strings.Contains(def, "boot-directory build/pxe/boot;") {
		t.Errorf("default config missing build/pxe/boot:\n%s", def)
	}
	if !strings.Contains(def, "root-directory build/pxe/tftp;") {
		t.Errorf("default config missing build/pxe/tftp:\n%s", def)
	}
}

func TestValidateFlagsPXEDir(t *testing.T) {
	t.Parallel()

	// A --pxe-dir resolving to a path with a space would corrupt the generated
	// config and must be rejected. Other flags are left invalid on purpose; we
	// only assert the pxe-dir error is among those reported.
	found := false
	for _, e := range validateFlags("", "", "", "", "", "/has space/pxe") {
		msg := e.Error()
		if strings.Contains(msg, "--pxe-dir") {
			found = true
			// The error must name the RESOLVED path, not just the flag, so a
			// failure from the cwd-derived default is attributable to the
			// checkout path rather than a flag the operator never set.
			if !strings.Contains(msg, "resolves to") || !strings.Contains(msg, "/has space/pxe/boot") {
				t.Errorf("pxe-dir error should name the resolved path, got: %q", msg)
			}
		}
	}
	if !found {
		t.Error("expected a --pxe-dir validation error for a path containing a space")
	}

	// A clean root produces no pxe-dir error.
	for _, e := range validateFlags("", "", "", "", "", "build/pxe") {
		if strings.Contains(e.Error(), "--pxe-dir") {
			t.Errorf("unexpected --pxe-dir error for a clean root: %v", e)
		}
	}
}
