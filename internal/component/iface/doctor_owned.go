// Design: docs/features/interfaces.md -- plugin-owned devices (bridges and links)
// Related: owned_bridge_link.go -- the owned passes that raise these warnings
// Related: config_apply.go -- reconcileOwnedDevices, the clean pass that clears them
// Related: register.go -- registers this check and raises the stop-time findings
//
// The owned-device check answers for the kernel state of the bridges and links
// plugins asked for. Each failure that can leave that state wrong raises a
// warning on the report bus under OwnedDeviceReportSource, and a clean owned
// pass clears what it proves resolved. The report bus is the record the check
// reads, so no second copy of the outcome is kept here.

package iface

import (
	"strings"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/health"
	"github.com/ze-software/ze/internal/core/report"
)

// OwnedDeviceReportSource is the report-bus source of every owned-device
// warning. The netlink backend raises under it too, for a half-made device it
// could not delete, so the one check reads them all.
const OwnedDeviceReportSource = "iface-owned-device"

// OwnedDeviceSubjectDevice prefixes the subject of a warning about one device
// by name. A clean pass clears such a warning only once the device is gone.
const OwnedDeviceSubjectDevice = "device "

// ownedSubjectPlugin prefixes the subject of a warning about one owner's
// devices. A clean pass proves every owner's set made and every orphan deleted,
// so it clears these.
const ownedSubjectPlugin = "plugin "

// ownedSubjectStop is the subject of the warning the ordered stop raises when
// it could not delete every owned device.
const ownedSubjectStop = "interface stop"

// ownedDeviceInspect is the command that lists the marked devices, the fix of
// a finding whose record the report bus evicted.
const ownedDeviceInspect = "ip -d link show"

// ownedDeviceDoctorCheck is the doctor-check registration, installed from
// register.go's init().
var ownedDeviceDoctorCheck = diagnostic.DoctorCheck{
	Name:         "iface-owned-devices",
	Phase:        diagnostic.DoctorPhaseRuntime,
	Order:        743,
	Component:    "iface",
	Dependencies: []string{"plugin-owned-devices"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes: []string{
		CodeDoctorIfaceOwnedDeviceCleanup,
		CodeDoctorIfaceOwnedDeviceReconcile,
	},
	Check: checkOwnedDevices,
}

// checkOwnedDevices reports every owned-device warning the daemon holds. It
// reads no configuration, because an owner's devices outlive its config. Safe
// for concurrent use; it changes nothing.
func checkOwnedDevices(diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	var findings []diagnostic.Diagnostic
	for _, issue := range report.Warnings() {
		if issue.Source != OwnedDeviceReportSource {
			continue
		}
		findings = append(findings, diagnostic.Diagnostic{
			Code: issue.Code, Severity: diagnostic.SeverityWarning,
			Message: issue.Message, Fix: issue.Fix,
		})
	}
	if len(findings) != 0 {
		return findings
	}
	// An evicted warning is unknown state, never evidence the devices are
	// right.
	status, reason := report.HealthProbeDegraded(
		CodeDoctorIfaceOwnedDeviceCleanup,
		CodeDoctorIfaceOwnedDeviceReconcile,
	)()
	if status != health.StatusHealthy {
		return []diagnostic.Diagnostic{{
			Code: CodeDoctorIfaceOwnedDeviceCleanup, Severity: diagnostic.SeverityWarning,
			Message: "the state of the plugin-owned interface devices is unknown: " + reason,
			Fix:     ownedDeviceInspect,
		}}
	}
	return findings
}

// raiseOwnedCleanupFailure records that owner's devices were not all deleted
// after the plugin ended. Restarting the plugin declares its set again, and
// that pass deletes what it no longer names.
func raiseOwnedCleanupFailure(owner string, err error) {
	report.RaiseWarningFix(OwnedDeviceReportSource, CodeDoctorIfaceOwnedDeviceCleanup,
		ownedSubjectPlugin+owner,
		report.FitMessage("owned interface devices of ended plugin "+owner+" were not all deleted: "+err.Error()),
		"ze request plugin restart "+owner)
}

// raiseOwnedStopFailure records that the ordered stop could not delete every
// owned device. The next start deletes every marked device no registration
// names.
func raiseOwnedStopFailure(err error) {
	report.RaiseWarningFix(OwnedDeviceReportSource, CodeDoctorIfaceOwnedDeviceCleanup,
		ownedSubjectStop,
		report.FitMessage("interface stop: owned devices not all deleted: "+err.Error()),
		"ze start")
}

// raiseOwnedReconcileFailure records that owner's previous bridge set could
// not be made again after a refused replacement, so the kernel no longer holds
// what the plugin counts on.
func raiseOwnedReconcileFailure(owner string, err error) {
	report.RaiseWarningFix(OwnedDeviceReportSource, CodeDoctorIfaceOwnedDeviceReconcile,
		ownedSubjectPlugin+owner,
		report.FitMessage("owned bridges of plugin "+owner+" do not match its last accepted set: "+err.Error()),
		"ze request plugin restart "+owner)
}

// clearOwnedDeviceWarnings clears what a clean owned pass proves resolved:
// every warning about an owner or the stop, and every warning about a device
// the pass's listing does not hold. A device the listing still holds keeps its
// warning, even when the pass deleted it as an orphan, because the listing is
// the only evidence read. The caller MUST hold reconcileMu.
func clearOwnedDeviceWarnings(currentByName map[string]InterfaceInfo) {
	for _, issue := range report.Warnings() {
		if issue.Source != OwnedDeviceReportSource {
			continue
		}
		if name, isDevice := strings.CutPrefix(issue.Subject, OwnedDeviceSubjectDevice); isDevice {
			if _, present := currentByName[name]; present {
				continue
			}
		}
		report.ClearWarning(issue.Source, issue.Code, issue.Subject)
	}
}
