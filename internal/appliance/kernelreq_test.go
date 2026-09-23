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

// completeRuntimeConfig renders a kernel config that satisfies the whole runtime
// floor, plus the extra symbols a test's manifest asks for.
//
// It is DERIVED from runtimeKernelRequirements rather than typed out beside it.
// A hand-written copy is a second declaration of the floor: it went stale the
// first time a symbol was added, and the failure read as the new symbol being
// wrong rather than as the fixture being incomplete.
func completeRuntimeConfig(extra ...string) string {
	symbols := append(append([]string(nil), extra...), runtimeKernelRequirements...)
	var b strings.Builder
	for _, symbol := range symbols {
		b.WriteString(symbol)
		b.WriteString("=y\n")
	}
	return b.String()
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
	full := completeRuntimeConfig("CONFIG_VETH")
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

// TestRuntimeFloorRequiresESP covers the IPsec half of the runtime floor.
//
// VALIDATES: AC-11. A runtime kernel config that omits CONFIG_INET_ESP fails
// enforcement, and the error names the missing symbol.
// PREVENTS: an appliance image whose IKE negotiates a Child SA the kernel then
// cannot carry. The image ships no iproute2 and no busybox, so an operator has no
// second tool to find that out with: the build is the only place to catch it.
func TestRuntimeFloorRequiresESP(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "runtime.require")
	if err := os.WriteFile(manifest, []byte("CONFIG_VETH\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(dir, "config")
	full := completeRuntimeConfig("CONFIG_VETH")
	if err := os.WriteFile(config, []byte(full), 0o644); err != nil {
		t.Fatal(err)
	}
	profile := kernelProfileResolution{Name: "runtime", Manifests: []string{manifest}}
	if err := enforceKernelRequirements(profile, config, runtimeKernelRequirements); err != nil {
		t.Fatalf("runtime floor on a complete IPsec config: %v", err)
	}

	// Each symbol is dropped on its own, so a floor that carries only one of them
	// cannot pass by accident.
	for _, symbol := range []string{"CONFIG_INET_ESP", "CONFIG_INET6_ESP", "CONFIG_XFRM_STATISTICS"} {
		missing := strings.ReplaceAll(full, symbol+"=y\n", "# "+symbol+" is not set\n")
		if err := os.WriteFile(config, []byte(missing), 0o644); err != nil {
			t.Fatal(err)
		}
		err := enforceKernelRequirements(profile, config, runtimeKernelRequirements)
		if err == nil || !strings.Contains(err.Error(), symbol) {
			t.Errorf("runtime floor error = %v, want %s", err, symbol)
		}
	}
}

// VALIDATES: R-6. The registered runtime profile requests each compiled floor
// symbol as built in. The emitted kernel config is checked after Kconfig resolves it.
// PREVENTS: A new floor requirement without a matching request in any fragment.
func TestRuntimeProfileRequestsFloorSymbols(t *testing.T) {
	t.Chdir(filepath.Join("..", ".."))
	profile, err := resolveKernelProfile(filepath.Join("gokrazy", "kernel"), "runtime")
	if err != nil {
		t.Fatal(err)
	}
	requested := make(map[string]bool)
	for _, fragment := range profile.Fragments {
		enabled, _, err := readKernelConfig(fragment)
		if err != nil {
			t.Fatal(err)
		}
		for symbol := range enabled {
			requested[symbol] = true
		}
	}
	for _, symbol := range runtimeKernelRequirements {
		if !requested[symbol] {
			t.Errorf("runtime profile does not request %s=y", symbol)
		}
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
