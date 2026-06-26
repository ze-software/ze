package appliance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnforceRequire(t *testing.T) {
	// VALIDATES: AC-5 expects manifest symbols missing from build/config to FATAL with the symbol name.
	// PREVENTS: a kernel build that silently downgrades required drivers to modules or disables them.
	dir := t.TempDir()
	manifest := filepath.Join(dir, "qemu.require")
	if err := os.WriteFile(manifest, []byte("CONFIG_VIRTIO_NET\nCONFIG_VIRTIO_BLK\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(dir, "config")
	if err := os.WriteFile(config, []byte(strings.Join([]string{
		"CONFIG_IP_PNP_DHCP=y",
		"CONFIG_EXT4_FS=y",
		"CONFIG_BLK_DEV_INITRD=y",
		"CONFIG_DEVTMPFS_MOUNT=y",
		"CONFIG_VIRTIO_NET=y",
		"CONFIG_VIRTIO_BLK=y",
		"CONFIG_NOT_REQUIRED=m",
	}, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	profile := kernelProfileResolution{Name: "qemu", Manifests: []string{manifest}}
	if err := enforceKernelRequirements(profile, config, universalKernelRequirements); err != nil {
		t.Fatalf("enforceKernelRequirements valid config: %v", err)
	}

	if err := os.WriteFile(config, []byte(strings.Join([]string{
		"CONFIG_IP_PNP_DHCP=y",
		"CONFIG_EXT4_FS=y",
		"CONFIG_BLK_DEV_INITRD=y",
		"CONFIG_DEVTMPFS_MOUNT=y",
		"CONFIG_VIRTIO_NET=m",
		"# CONFIG_VIRTIO_BLK is not set",
	}, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := enforceKernelRequirements(profile, config, universalKernelRequirements)
	if err == nil || !strings.Contains(err.Error(), "CONFIG_VIRTIO_NET") {
		t.Fatalf("missing symbol error = %v, want CONFIG_VIRTIO_NET", err)
	}
}

func TestEnforceUniversalFloor(t *testing.T) {
	// VALIDATES: AC-6 expects the hardcoded universal floor to catch stripped floor symbols even when manifests omit them.
	// PREVENTS: manifest typos removing IP DHCP, ext4, initrd, or devtmpfs boot requirements.
	dir := t.TempDir()
	manifest := filepath.Join(dir, "qemu.require")
	if err := os.WriteFile(manifest, []byte("CONFIG_VIRTIO_NET\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(dir, "config")
	if err := os.WriteFile(config, []byte(strings.Join([]string{
		"CONFIG_EXT4_FS=y",
		"CONFIG_BLK_DEV_INITRD=y",
		"CONFIG_DEVTMPFS_MOUNT=y",
		"CONFIG_VIRTIO_NET=y",
	}, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := enforceKernelRequirements(kernelProfileResolution{Name: "qemu", Manifests: []string{manifest}}, config, universalKernelRequirements)
	if err == nil || !strings.Contains(err.Error(), "CONFIG_IP_PNP_DHCP") {
		t.Fatalf("floor error = %v, want CONFIG_IP_PNP_DHCP", err)
	}
}

func TestRuntimeFloorEnforced(t *testing.T) {
	// VALIDATES: AC-8 expects the runtime floor (CONFIG_MODULES + L2TP/PPP set) to be
	// enforced for the runtime target, giving it the same verified guarantee as the installer.
	// PREVENTS: a runtime kernel shipping without modules or the L2TP/PPP/PPPoE drivers the
	// ze-qemu evidence tests boot on.
	dir := t.TempDir()
	manifest := filepath.Join(dir, "runtime.require")
	if err := os.WriteFile(manifest, []byte("CONFIG_VETH\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(dir, "config")
	full := strings.Join([]string{
		"CONFIG_VETH=y",
		"CONFIG_MODULES=y",
		"CONFIG_PPP=y",
		"CONFIG_PPPOE=y",
		"CONFIG_L2TP=y",
		"CONFIG_PPPOL2TP=y",
		"CONFIG_L2TP_V3=y",
	}, "\n") + "\n"
	if err := os.WriteFile(config, []byte(full), 0o644); err != nil {
		t.Fatal(err)
	}
	profile := kernelProfileResolution{Name: "runtime", Manifests: []string{manifest}}
	if err := enforceKernelRequirements(profile, config, runtimeKernelRequirements); err != nil {
		t.Fatalf("runtime floor on complete config: %v", err)
	}

	// Drop CONFIG_PPPOE: the runtime floor must reject it even though the manifest omits it.
	missing := strings.ReplaceAll(full, "CONFIG_PPPOE=y\n", "# CONFIG_PPPOE is not set\n")
	if err := os.WriteFile(config, []byte(missing), 0o644); err != nil {
		t.Fatal(err)
	}
	err := enforceKernelRequirements(profile, config, runtimeKernelRequirements)
	if err == nil || !strings.Contains(err.Error(), "CONFIG_PPPOE") {
		t.Fatalf("runtime floor error = %v, want CONFIG_PPPOE", err)
	}
}

func TestEnforceHardwareKMSFirmware(t *testing.T) {
	// VALIDATES: A-3 expects hardware-kms to preserve the CONFIG_EXTRA_FIRMWARE check alongside manifests.
	// PREVENTS: building an i915 KMS installer kernel without embedded display firmware.
	dir := t.TempDir()
	manifest := filepath.Join(dir, "hardware-kms.require")
	if err := os.WriteFile(manifest, []byte("CONFIG_DRM_I915\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(dir, "config")
	base := strings.Join([]string{
		"CONFIG_IP_PNP_DHCP=y",
		"CONFIG_EXT4_FS=y",
		"CONFIG_BLK_DEV_INITRD=y",
		"CONFIG_DEVTMPFS_MOUNT=y",
		"CONFIG_DRM_I915=y",
	}, "\n") + "\n"
	if err := os.WriteFile(config, []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}

	profile := kernelProfileResolution{Name: "hardware-kms", Manifests: []string{manifest}}
	err := enforceKernelRequirements(profile, config, universalKernelRequirements)
	if err == nil || !strings.Contains(err.Error(), "CONFIG_EXTRA_FIRMWARE") {
		t.Fatalf("firmware error = %v, want CONFIG_EXTRA_FIRMWARE", err)
	}
	if err := os.WriteFile(config, []byte(base+"CONFIG_EXTRA_FIRMWARE=\"i915/adlp_dmc.bin\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := enforceKernelRequirements(profile, config, universalKernelRequirements); err != nil {
		t.Fatalf("enforceKernelRequirements with firmware: %v", err)
	}
}
