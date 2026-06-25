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
	if err := enforceKernelRequirements(profile, config); err != nil {
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
	err := enforceKernelRequirements(profile, config)
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

	err := enforceKernelRequirements(kernelProfileResolution{Name: "qemu", Manifests: []string{manifest}}, config)
	if err == nil || !strings.Contains(err.Error(), "CONFIG_IP_PNP_DHCP") {
		t.Fatalf("floor error = %v, want CONFIG_IP_PNP_DHCP", err)
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
	err := enforceKernelRequirements(profile, config)
	if err == nil || !strings.Contains(err.Error(), "CONFIG_EXTRA_FIRMWARE") {
		t.Fatalf("firmware error = %v, want CONFIG_EXTRA_FIRMWARE", err)
	}
	if err := os.WriteFile(config, []byte(base+"CONFIG_EXTRA_FIRMWARE=\"i915/adlp_dmc.bin\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := enforceKernelRequirements(profile, config); err != nil {
		t.Fatalf("enforceKernelRequirements with firmware: %v", err)
	}
}
