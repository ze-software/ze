// Design: plan/learned/982-install-11-hw-kernel-profiles.md - installer kernel profile registry

package appliance

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	defaultKernelProfile = "qemu"
	hardwareKMSProfile   = "hardware-kms"
	kernelConfigName     = "kernel.config"
	kernelRequireName    = "kernel.require"
)

var validKernelProfileRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

type kernelProfileResolution struct {
	Name      string
	SourceDir string
	Fragments []string
	Manifests []string
}

func validateKernelProfileName(profile string) error {
	if !validKernelProfileRe.MatchString(profile) {
		return fmt.Errorf("kernel profile %q: must match %s; use lowercase letters, digits, and dashes", profile, validKernelProfileRe.String())
	}
	return nil
}

func resolveKernelProfile(srcDir, profile string) (kernelProfileResolution, error) {
	if err := validateKernelProfileName(profile); err != nil {
		return kernelProfileResolution{}, err
	}

	kernelConfig := filepath.Join(srcDir, kernelConfigName)
	kernelRequire := filepath.Join(srcDir, kernelRequireName)
	if err := requireFile(kernelConfig, "base config fragment"); err != nil {
		return kernelProfileResolution{}, err
	}
	if err := requireFile(kernelRequire, "base require manifest"); err != nil {
		return kernelProfileResolution{}, err
	}

	profileConfig := filepath.Join(srcDir, profile+".config")
	profileRequire := filepath.Join(srcDir, profile+".require")
	if err := requireFile(profileConfig, "profile config fragment"); err != nil {
		return kernelProfileResolution{}, fmt.Errorf("kernel profile %q: missing config fragment %s; add %s.config and %s.require: %w", profile, profileConfig, profile, profile, err)
	}
	if err := requireFile(profileRequire, "profile require manifest"); err != nil {
		return kernelProfileResolution{}, fmt.Errorf("kernel profile %q: no require manifest at %s; add %s.require before building this profile: %w", profile, profileRequire, profile, err)
	}

	fragments := []string{kernelConfig}
	manifests := []string{kernelRequire}
	base, err := readKernelProfileBase(profileConfig)
	if err != nil {
		return kernelProfileResolution{}, fmt.Errorf("kernel profile %q: %w", profile, err)
	}
	if base != "" {
		if err := validateKernelProfileName(base); err != nil {
			return kernelProfileResolution{}, fmt.Errorf("kernel profile %q base %q: %w", profile, base, err)
		}
		baseConfig := filepath.Join(srcDir, base+".config")
		baseRequire := filepath.Join(srcDir, base+".require")
		if err := requireFile(baseConfig, "base profile config fragment"); err != nil {
			return kernelProfileResolution{}, fmt.Errorf("kernel profile %q base %q: missing config fragment %s: %w", profile, base, baseConfig, err)
		}
		if err := requireFile(baseRequire, "base profile require manifest"); err != nil {
			return kernelProfileResolution{}, fmt.Errorf("kernel profile %q base %q: no require manifest at %s: %w", profile, base, baseRequire, err)
		}
		nestedBase, err := readKernelProfileBase(baseConfig)
		if err != nil {
			return kernelProfileResolution{}, fmt.Errorf("kernel profile %q base %q: %w", profile, base, err)
		}
		if nestedBase != "" {
			return kernelProfileResolution{}, fmt.Errorf("kernel profile %q base %q declares ze-base %q; only one level of ze-base is supported", profile, base, nestedBase)
		}
		fragments = append(fragments, baseConfig)
		manifests = append(manifests, baseRequire)
	}
	fragments = append(fragments, profileConfig)
	manifests = append(manifests, profileRequire)

	return kernelProfileResolution{
		Name:      profile,
		SourceDir: srcDir,
		Fragments: fragments,
		Manifests: manifests,
	}, nil
}

func registeredKernelProfiles(srcDir string) ([]string, error) {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return nil, fmt.Errorf("scan kernel profile registry %s: %w", srcDir, err)
	}
	profiles := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".config") || name == kernelConfigName {
			continue
		}
		profile := strings.TrimSuffix(name, ".config")
		if !validKernelProfileRe.MatchString(profile) {
			continue
		}
		if _, err := os.Stat(filepath.Join(srcDir, profile+".require")); err != nil {
			continue
		}
		profiles = append(profiles, profile)
	}
	sort.Strings(profiles)
	return profiles, nil
}

func requireFile(path, label string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s %s not found", label, path)
	}
	if info.IsDir() {
		return fmt.Errorf("%s %s is a directory", label, path)
	}
	return nil
}

func readKernelProfileBase(path string) (string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // profile paths are built from validated tokens
	if err != nil {
		return "", fmt.Errorf("read config fragment %s: %w", path, err)
	}
	var base string
	for lineNo, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "# ze-base:") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, "# ze-base:"))
		if value == "" {
			return "", fmt.Errorf("%s:%d empty ze-base value", path, lineNo+1)
		}
		if base != "" {
			return "", fmt.Errorf("%s:%d duplicate ze-base value", path, lineNo+1)
		}
		base = value
	}
	return base, nil
}
