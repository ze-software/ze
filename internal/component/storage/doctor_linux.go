//go:build linux

// Design: docs/architecture/storage/smart-health.md -- the readiness check this component owns
// Related: register_linux.go -- the init() that installs the registration below
// Related: discover_linux.go -- discoverBlockDevices, the same sysfs walk the manager runs
// Related: manager.go -- Manager.checkDevice, the smart.Detect this check rehearses
//
// The check lived in internal/component/doctor and the runner reached it by
// writing its name out. It rehearses what the manager does on its first poll,
// walk /sys/class/block and ask each whole device for SMART, so the answer is
// this component's (ai/patterns/registration.md, "Doctor Check Registry"): it
// is owned here now and dropping this component drops its check with it.
// Linux-only because sysfs and the SMART ioctls are.

package storage

import (
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/smart"
)

// codeSmartSysfs names a /sys/class/block the check cannot enumerate.
// internal/core/diagnostic/codes.go declares it, so `ze explain` answers.
const codeSmartSysfs = "doctor-smart-sysfs"

// codeSmartAccess names a host whose whole devices all refuse SMART, declared
// beside codeSmartSysfs in the same registry.
const codeSmartAccess = "doctor-smart-access"

// The two probes checkSmartEnabled runs: the sysfs walk and the SMART query.
// Each is a variable so a test stands in a host without devices or without
// the privilege; nothing else assigns them.
var (
	smartReadBlockDir = os.ReadDir
	smartDetect       = smart.Detect
)

// smartDoctorCheck is the registration register_linux.go installs.
//
// Order 2300 reproduces the sequence the doctor runner printed before the check
// moved onto the registry: after the resolv.conf path check (2290,
// internal/component/config/system) and before the config-claims audit that
// followed it.
var smartDoctorCheck = diagnostic.DoctorCheck{
	Name:         "storage-smart",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        2300,
	Component:    "storage",
	Dependencies: []string{"sysfs", "smart-ioctl"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{codeSmartSysfs, codeSmartAccess},
	Check:        checkSmartEnabled,
}

// checkSmartEnabled warns, for a config that enables SMART management, when
// sysfs cannot be enumerated, or when the host holds whole block devices and
// none of them answers a SMART query, which is what an unprivileged process
// sees.
//
// A nil tree is the missing-config phase, which this check does not run in,
// and a context carrying anything else is a runner defect the runner's own
// type assertion reports (doctorTree, internal/component/doctor/registry.go).
func checkSmartEnabled(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	smartCfg := tree.GetContainerPath("storage/smart")
	if smartCfg == nil {
		return nil
	}
	if enabled, _ := smartCfg.Get("enabled"); enabled != "true" {
		return nil
	}

	entries, err := smartReadBlockDir(sysClassBlockDir)
	if err != nil {
		return []diagnostic.Diagnostic{{
			Code:     codeSmartSysfs,
			Severity: diagnostic.SeverityWarning,
			Message:  "cannot enumerate block devices: " + err.Error(),
		}}
	}

	checked := 0
	accessible := 0
	for _, e := range entries {
		name := e.Name()
		if isPartition(filepath.Join(sysClassBlockDir, name)) {
			continue
		}
		checked++
		info := smartDetect(name, "")
		if info == nil {
			continue
		}
		if !info.Unavailable {
			accessible++
		}
	}
	if checked > 0 && accessible == 0 {
		return []diagnostic.Diagnostic{{
			Code:     codeSmartAccess,
			Severity: diagnostic.SeverityWarning,
			Message:  "SMART enabled in config but no devices are accessible (check privileges)",
		}}
	}
	return nil
}
