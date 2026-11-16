// Design: docs/architecture/appliance/kernel-profiles.md -- Loading driver modules on the device: when the pass runs
// Overview: coldplug.go -- the pass this step runs

//go:build linux

package coldplug

import (
	"errors"
	"path/filepath"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/cmd/ze/startprep"
	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// stepName is the start step's name in the startprep registry.
const stepName = "kernel-module-coldplug"

// applianceKey is the switch the gokrazy image sets on `ze start`
// (gokrazy/ze/config.json, Environment); internal/component/config registers
// it. A ze_appliance binary started anywhere else (a test host, a container)
// lacks it and loads nothing, so it never loads modules into a host it does
// not own.
const applianceKey = "ze.gokrazy.enabled"

// modulesDir is where the kernel's module trees live, one directory per
// kernel release.
const modulesDir = "/lib/modules"

// The appliance flavor links this package (cmd/ze/setup_features_appliance.go),
// and the step runs on `ze start` before the store is opened and before any
// interface is enumerated.
func init() {
	startprep.Register(stepName, coldplugOnAppliance)
}

// coldplugOnAppliance runs the pass over the running kernel's own module tree,
// on the appliance only.
func coldplugOnAppliance() {
	if !env.IsEnabled(applianceKey) {
		return
	}
	log := slogutil.Logger("coldplug")
	release, err := kernelRelease()
	if err != nil {
		log.Warn("the kernel release cannot be read, so no driver module is loaded for the devices present",
			"code", diagnostic.CodeDoctorModuleMissing, "error", err, "fix", fixKernelLog)
		return
	}
	// A nil set is a module list that could not be read, never an empty one.
	// The pass still runs: finit_module answers EEXIST for a module the
	// kernel already holds, and the pass counts that as loaded.
	loaded := kernelcap.LoadedModules()
	if loaded == nil {
		log.Debug("the loaded-module list cannot be read; the kernel will say which modules it already holds")
	}
	coldplug("/sys", filepath.Join(modulesDir, release), loaded, finitModule, log)
}

// kernelRelease answers the running kernel's release, the name of its
// /lib/modules directory.
func kernelRelease() (string, error) {
	var uts unix.Utsname
	if err := unix.Uname(&uts); err != nil {
		return "", err
	}
	release := unix.ByteSliceToString(uts.Release[:])
	if release == "" {
		return "", errors.New("uname answered an empty kernel release")
	}
	return release, nil
}
