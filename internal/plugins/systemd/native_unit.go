// Design: docs/architecture/cli/plugin-modes.md -- effective service policy.

package systemd

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

type nativeLoadState uint8

const (
	nativeUnitAbsent nativeLoadState = iota + 1
	nativeUnitLoaded
)

type nativeUnitIdentity struct {
	state    nativeLoadState
	fragment string
	dropins  []string
}

func (service nativeService) effectiveUnit() (nativeUnitIdentity, error) {
	data, err := service.rt.ops.output("systemctl", "show", "--property=LoadState", "--property=FragmentPath", "--property=DropInPaths", serviceName)
	if err != nil {
		return nativeUnitIdentity{}, fmt.Errorf("read effective ze.service policy: %w; inspect systemctl show ze.service before retrying", err)
	}
	var load, fragment, dropins string
	seen := make(map[string]bool)
	for line := range strings.SplitSeq(string(data), "\n") {
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		switch key {
		case "LoadState":
			load = value
			seen[key] = true
		case "FragmentPath":
			fragment = value
			seen[key] = true
		case "DropInPaths":
			dropins = value
			seen[key] = true
		}
	}
	for _, property := range []string{"LoadState", "FragmentPath", "DropInPaths"} {
		if !seen[property] {
			return nativeUnitIdentity{}, fmt.Errorf("systemd did not report %s for ze.service; inspect systemctl show ze.service before retrying", property)
		}
	}
	identity := nativeUnitIdentity{fragment: fragment, dropins: strings.Fields(dropins)}
	switch load {
	case "not-found":
		identity.state = nativeUnitAbsent
		if fragment != "" {
			return nativeUnitIdentity{}, errors.New("systemd reports an absent Ze unit with an existing fragment; inspect systemctl show ze.service before retrying")
		}
	case "loaded":
		identity.state = nativeUnitLoaded
		if fragment != service.rt.unitPath {
			return nativeUnitIdentity{}, fmt.Errorf("effective Ze service comes from %s; preserve that owner and inspect systemctl cat ze.service", fragment)
		}
	default:
		return nativeUnitIdentity{}, fmt.Errorf("effective Ze service is %s; preserve its policy and inspect systemctl show ze.service", load)
	}
	for _, path := range identity.dropins {
		if !filepath.IsAbs(path) {
			return nativeUnitIdentity{}, errors.New("systemd reported an unsupported drop-in path; inspect systemctl cat ze.service")
		}
		if containsUnitPathUnsafeChar(path) {
			return nativeUnitIdentity{}, errors.New("systemd drop-in paths contain unsupported escapes; inspect systemctl cat ze.service")
		}
	}
	return identity, nil
}

func (service nativeService) readNativePolicy(path string) ([]byte, error) {
	if err := service.trustedExecutable(path); err != nil {
		return nil, err
	}
	info, err := service.rt.ops.lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("service policy %s is not regular; inspect systemctl cat ze.service", path)
	}
	if err := service.trustedNode(path, info); err != nil {
		return nil, err
	}
	return service.rt.ops.readFile(path)
}

func validateNativePolicy(data []byte, binary, configDir string) error {
	unit := parseServiceUnit(data)
	if unit.unsupported != "" {
		return fmt.Errorf("Ze service directive %s can change execution or config ownership; preserve the unit and inspect systemctl cat ze.service", unit.unsupported)
	}
	if unit.execStartValue != binary+" start" {
		if unit.execStartValue != execStartShell+" -c 'trap \"\" HUP; exec "+binary+" start'" {
			return errors.New("Ze service start command has unsupported arguments; preserve the unit and inspect systemctl cat ze.service")
		}
	}
	return matchNativeUnit(unit, binary, configDir)
}
