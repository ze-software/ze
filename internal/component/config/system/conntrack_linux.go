// Design: docs/architecture/config/syntax.md -- conntrack module loading (Linux)

//go:build linux

package system

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// LoadConntrackModules loads kernel conntrack helper modules via modprobe.
// Load-only: modules are never unloaded (unloading breaks active connections).
// On gokrazy, modules are built-in so modprobe is not available; this is
// detected and skipped gracefully.
func LoadConntrackModules(modules []string) (loaded []string, errs []error) {
	if len(modules) == 0 {
		return nil, nil
	}

	modprobePath, err := exec.LookPath("modprobe")
	if err != nil {
		return nil, nil
	}

	loadedSet := readLoadedModules()

	for _, mod := range modules {
		if !ValidConntrackModule(mod) {
			errs = append(errs, fmt.Errorf("conntrack: refusing to load unknown module %q", mod))
			continue
		}
		kernelMod := toKernelModName(mod)
		if loadedSet[kernelMod] {
			loaded = append(loaded, mod)
			continue
		}
		cmd := exec.Command(modprobePath, kernelMod)
		if err := cmd.Run(); err != nil {
			errs = append(errs, fmt.Errorf("conntrack: modprobe %s: %w", kernelMod, err))
			continue
		}
		loaded = append(loaded, mod)
	}
	return loaded, errs
}

// toKernelModName converts a config module name to its kernel module name.
// The kernel uses underscores where config uses hyphens (e.g., netbios-ns
// becomes nf_conntrack_netbios_ns).
func toKernelModName(configName string) string {
	return "nf_conntrack_" + strings.ReplaceAll(configName, "-", "_")
}

// toConfigModName converts a kernel module suffix back to config-style
// (underscores to hyphens).
func toConfigModName(kernelSuffix string) string {
	return strings.ReplaceAll(kernelSuffix, "_", "-")
}

// readLoadedModules reads /proc/modules once and returns a set of loaded
// module names. Returns an empty map on error.
func readLoadedModules() map[string]bool {
	data, err := os.ReadFile("/proc/modules")
	if err != nil {
		return nil
	}
	set := make(map[string]bool)
	for _, line := range splitLines(data) {
		if sp := strings.IndexByte(line, ' '); sp > 0 {
			set[line[:sp]] = true
		}
	}
	return set
}

func splitLines(data []byte) []string {
	var lines []string
	start := 0
	for i, b := range data {
		if b == '\n' {
			lines = append(lines, string(data[start:i]))
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, string(data[start:]))
	}
	return lines
}

// LoadedConntrackModules returns the list of currently loaded nf_conntrack_*
// modules by reading /proc/modules. Names are returned in config-style
// (hyphens, not underscores).
func LoadedConntrackModules() []string {
	data, err := os.ReadFile("/proc/modules")
	if err != nil {
		return nil
	}
	const prefix = "nf_conntrack_"
	var modules []string
	for _, line := range splitLines(data) {
		sp := strings.IndexByte(line, ' ')
		if sp <= 0 {
			continue
		}
		name := line[:sp]
		if len(name) > len(prefix) && name[:len(prefix)] == prefix {
			modules = append(modules, toConfigModName(name[len(prefix):]))
		}
	}
	return modules
}
