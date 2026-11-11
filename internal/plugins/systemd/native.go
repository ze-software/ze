// Design: docs/architecture/cli/plugin-modes.md -- native service lifecycle.

package systemd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/cmd/ze/install"
	"github.com/ze-software/ze/internal/core/opererr"
)

type nativeService struct{ rt *serviceRuntime }

type nativeStoppedService struct {
	service   nativeService
	binary    string
	configDir string
	start     bool
	resume    bool
}

func (service nativeService) Available() (bool, error) {
	if !service.rt.ops.isLinux() {
		return false, nil
	}
	if _, err := service.rt.ops.lookPath("systemctl"); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("locate systemctl: %w; repair PATH or select --no-service for foreground operation", err)
	}
	if _, err := service.rt.ops.stat("/run/systemd/system"); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("inspect running systemd: %w; use ze install --no-service for foreground operation", err)
	}
	return true, nil
}

func (service nativeService) Preflight(binary, configDir string) error {
	if !service.rt.ops.isRoot() {
		return errors.New("service installation requires root; run sudo ze install or use --no-service with a writable --prefix")
	}
	for _, path := range []string{binary, configDir} {
		if !filepath.IsAbs(path) {
			return errors.New("service paths must be absolute; select an absolute installation prefix")
		}
		if containsUnitPathUnsafeChar(path) {
			return errors.New("service path contains unsafe characters; select a path without shell characters or whitespace")
		}
	}
	if err := service.trustedExecutable(binary); err != nil {
		return err
	}
	return service.compatibleUnit(binary, configDir)
}

func (service nativeService) Stop(binary, configDir string) (install.StoppedService, error) {
	if err := service.Preflight(binary, configDir); err != nil {
		return nil, err
	}
	if _, err := service.rt.ops.lstat(service.rt.unitPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &nativeStoppedService{service: service, binary: binary, configDir: configDir, start: true}, nil
		} // No service exists to stop on a fresh installation.
		return nil, err
	}
	state, err := service.rt.ops.output("systemctl", "show", "--property=ActiveState", "--value", serviceName)
	if err != nil {
		return nil, fmt.Errorf("read ze.service state: %w; run systemctl status ze.service before retrying installation", err)
	}
	plan := &nativeStoppedService{service: service, binary: binary, configDir: configDir}
	switch strings.TrimSpace(string(state)) {
	case "active", "activating", "reloading":
		if err := service.rt.ops.run("systemctl", "stop", serviceName); err != nil {
			return nil, err
		}
		plan.start, plan.resume = true, true
	case "inactive":
		// Preserve an administrator's inactive service during replacement.
	case "failed":
		plan.start = true
	default:
		return nil, fmt.Errorf("ze.service state %q cannot be replaced safely; wait for a settled state and inspect systemctl status ze.service", strings.TrimSpace(string(state)))
	}
	return plan, nil
}

func (service nativeService) Install(binary, configDir string) int {
	return service.installSelected(binary, configDir, true)
}

func (stopped *nativeStoppedService) Install() int {
	return stopped.service.installSelected(stopped.binary, stopped.configDir, stopped.start)
}

func (stopped *nativeStoppedService) Restore() error {
	if !stopped.resume {
		return nil
	}
	return stopped.service.rt.ops.run("systemctl", "start", serviceName)
}

func (service nativeService) installSelected(binary, configDir string, start bool) int {
	if err := service.Preflight(binary, configDir); err != nil {
		return service.refuse(err)
	}
	if _, err := service.rt.ops.lstat(service.rt.unitPath); err == nil {
		// A compatible unit is administrator-owned. Preserve it and every
		// drop-in byte, and do not change its enablement on reinstall.
		if !start {
			return exitOK
		}
		if err := service.rt.ops.run("systemctl", "start", serviceName); err != nil {
			return service.refuse(err)
		}
		return exitOK
	} else if !errors.Is(err, os.ErrNotExist) {
		return service.refuse(err)
	}
	service.rt.binaryPath = binary
	args := []string{"--config", configDir}
	if start {
		args = append(args, "--start")
	}
	return service.rt.cmdInstall(args)
}

func (service nativeService) Uninstall(binary, configDir string) int {
	if err := service.Preflight(binary, configDir); err != nil {
		return service.refuse(err)
	}
	return service.rt.cmdUninstall(nil)
}

func (service nativeService) refuse(err error) int {
	writeln(service.rt.stderr, opererr.Wrap(err, "The Ze service operation did not finish", "The existing unit and its policy require inspection before retrying.", "sudo systemctl status ze.service"))
	return exitError
}

// compatibleUnit admits only this selected standalone installation. Vendor
// units, masks, links and a different binary/config owner are never replaced.
func (service nativeService) compatibleUnit(binary, configDir string) error {
	for _, vendor := range []string{"/usr/lib/systemd/system/ze.service", "/lib/systemd/system/ze.service", "/run/systemd/system/ze.service"} {
		if _, err := service.rt.ops.lstat(vendor); err == nil {
			return fmt.Errorf("another service definition %s already supplies Ze; preserve its policy and inspect systemctl cat ze.service", vendor)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	identity, err := service.effectiveUnit()
	if err != nil {
		return err
	}
	staticDropins, err := service.dropins()
	if err != nil {
		return err
	}
	info, err := service.rt.ops.lstat(service.rt.unitPath)
	var data []byte
	if errors.Is(err, os.ErrNotExist) {
		if identity.state != nativeUnitAbsent {
			return errors.New("systemd still owns a missing Ze fragment; inspect systemctl show ze.service before installation")
		}
		if len(staticDropins) != 0 {
			return errors.New("orphan Ze service drop-ins would change a fresh installation; preserve them and inspect systemctl cat ze.service")
		}
		data = []byte(buildUnitFile(unitSpec{BinaryPath: binary, ConfigDir: configDir}))
	} else {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("service %s is masked or is not regular; preserve its policy and inspect systemctl cat ze.service", service.rt.unitPath)
		}
		if identity.state != nativeUnitLoaded {
			return errors.New("Ze service file differs from systemd's loaded policy; inspect it, then run sudo systemctl daemon-reload")
		}
		data, err = service.readNativePolicy(service.rt.unitPath)
		if err != nil {
			return err
		}
		for _, path := range staticDropins {
			if !slices.Contains(identity.dropins, path) {
				return errors.New("Ze service drop-ins differ from systemd's loaded policy; inspect them, then run sudo systemctl daemon-reload")
			}
		}
	}
	// The manager's resolved list also includes global service.d tuning and
	// supplies the actual ordering, rather than a second copy of that policy.
	combined := string(data)
	for _, path := range identity.dropins {
		fragment, err := service.readNativePolicy(path)
		if err != nil {
			return err
		}
		combined += "\n" + string(fragment)
	}
	return validateNativePolicy([]byte(combined), binary, configDir)
}

func matchNativeUnit(unit serviceUnitInfo, binary, configDir string) error {
	if unit.execStart != binary {
		return fmt.Errorf("ze.service starts %s instead of %s; preserve that installation or select its --prefix", unit.execStart, binary)
	}
	if unit.user != serviceUser {
		return errors.New("ze.service uses another account; preserve the administrator unit or select --no-service")
	}
	if unit.group != serviceGroup {
		return errors.New("ze.service uses another group; preserve the administrator unit or select --no-service")
	}
	if unit.workingDir != configDir {
		return errors.New("ze.service uses another working directory; preserve the administrator unit or select its installation prefix")
	}
	if unit.configDir != configDir {
		return errors.New("ze.service uses another config directory; preserve the administrator unit or select its installation prefix")
	}
	return nil
}

func (service nativeService) dropins() ([]string, error) {
	byName := make(map[string]string)
	for _, dir := range []string{"/usr/lib/systemd/system/ze.service.d", "/run/systemd/system/ze.service.d", service.rt.unitPath + ".d"} {
		entries, err := service.rt.ops.readDir(dir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".conf") {
				continue
			}
			if !entry.Type().IsRegular() {
				return nil, fmt.Errorf("service drop-in %s is not regular; inspect it before standalone installation", filepath.Join(dir, entry.Name()))
			}
			byName[entry.Name()] = filepath.Join(dir, entry.Name())
		}
	}
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	slices.Sort(names)
	paths := make([]string, 0, len(names))
	for _, name := range names {
		paths = append(paths, byName[name])
	}
	return paths, nil
}
