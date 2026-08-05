// Design: plan/learned/982-install-11-hw-kernel-profiles.md - installer kernel requirement enforcement

package appliance

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// universalKernelRequirements is the installer floor: the busybox initrd has no
// modules, so kernel-level DHCP autoconfig, ext4, and the initrd/devtmpfs mount
// must be built in regardless of profile.
var universalKernelRequirements = []string{
	"CONFIG_IP_PNP_DHCP",
	"CONFIG_EXT4_FS",
	"CONFIG_BLK_DEV_INITRD",
	"CONFIG_DEVTMPFS_MOUNT",
}

// runtimeKernelRequirements is the runtime floor, giving the runtime kernel the
// same Go-verified guarantee 982 gave the installer. It is intentionally partly
// redundant with gokrazy/kernel/runtime.require so the verified path has a floor
// independent of the editable manifest: modules support plus the L2TP/PPP/PPPoE
// set the ze-qemu evidence tests boot on.
//
// The ESP entries carry the IPsec dataplane. gokrazy/kernel/kernel.config sets
// CONFIG_XFRM_USER, which gives the kernel the netlink interface ze installs
// through; without CONFIG_INET_ESP the same kernel accepts the install and drops
// every ESP packet. The image ships two ze binaries and gokrazy randomd and
// heartbeat, so there is no iproute2 and no busybox to diagnose that with: the
// build is the only place to catch it. CONFIG_XFRM_STATISTICS sources the SAD byte
// counters `show vpn ipsec sa` reports.
var runtimeKernelRequirements = []string{
	"CONFIG_MODULES",
	"CONFIG_PPP",
	"CONFIG_PPPOE",
	"CONFIG_L2TP",
	"CONFIG_PPPOL2TP",
	"CONFIG_L2TP_V3",
	"CONFIG_INET_ESP",
	"CONFIG_INET6_ESP",
	"CONFIG_XFRM_STATISTICS",
}

func enforceKernelRequirements(profile kernelProfileResolution, configPath string, floor []string) error {
	enabled, set, err := readKernelConfig(configPath)
	if err != nil {
		return err
	}
	required, err := readKernelRequireManifests(profile.Manifests)
	if err != nil {
		return err
	}
	required = append(required, floor...)

	seen := make(map[string]bool, len(required))
	for _, symbol := range required {
		if seen[symbol] {
			continue
		}
		seen[symbol] = true
		if !enabled[symbol] {
			return fmt.Errorf("kernel profile %q: %s did not resolve to =y in %s; add it to the config fragments or remove it from the require manifest only if it is no longer required", profile.Name, symbol, configPath)
		}
	}
	if profile.Name == hardwareKMSProfile && !set["CONFIG_EXTRA_FIRMWARE"] {
		return fmt.Errorf("kernel profile %q: CONFIG_EXTRA_FIRMWARE is not set in %s; provide i915 firmware so KMS display output works", profile.Name, configPath)
	}
	return nil
}

func readKernelRequireManifests(paths []string) ([]string, error) {
	var required []string
	for _, path := range paths {
		symbols, err := readKernelRequireManifest(path)
		if err != nil {
			return nil, err
		}
		required = append(required, symbols...)
	}
	return required, nil
}

func readKernelRequireManifest(path string) ([]string, error) {
	f, err := os.Open(path) //nolint:gosec // manifest paths are built from validated profile tokens
	if err != nil {
		return nil, fmt.Errorf("read kernel require manifest %s: %w", path, err)
	}
	defer f.Close() //nolint:errcheck // read-only close

	var symbols []string
	scanner := bufio.NewScanner(f)
	for lineNo := 1; scanner.Scan(); lineNo++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if trimmed, ok := strings.CutSuffix(line, "=y"); ok {
			line = trimmed
		} else if strings.Contains(line, "=") {
			return nil, fmt.Errorf("kernel require manifest %s:%d has %q; require entries must be CONFIG_SYMBOL or CONFIG_SYMBOL=y", path, lineNo, line)
		}
		if !strings.HasPrefix(line, "CONFIG_") || strings.ContainsAny(line, " \t/") {
			return nil, fmt.Errorf("kernel require manifest %s:%d has invalid symbol %q; expected CONFIG_SYMBOL", path, lineNo, line)
		}
		symbols = append(symbols, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read kernel require manifest %s: %w", path, err)
	}
	return symbols, nil
}

func readKernelConfig(path string) (enabled, set map[string]bool, err error) {
	f, err := os.Open(path) //nolint:gosec // config path is controlled by the builder output
	if err != nil {
		return nil, nil, fmt.Errorf("read resolved kernel config %s: %w", path, err)
	}
	defer f.Close() //nolint:errcheck // read-only close

	enabled = make(map[string]bool)
	set = make(map[string]bool)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "CONFIG_") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		set[key] = true
		if value == "y" {
			enabled[key] = true
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("read resolved kernel config %s: %w", path, err)
	}
	return enabled, set, nil
}
