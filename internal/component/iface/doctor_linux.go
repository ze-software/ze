//go:build linux

// Design: docs/architecture/iface/logical-name-resolution.md -- the readiness check this component owns
// Related: register_linux.go -- the init() that installs the registration below
// Related: config_apply.go -- bindDevices and devicesWithMAC, the apply-path resolution this check mirrors
// Related: resolve.go -- deviceMatchMAC, the resolver that prefers the permanent address
//
// The check lived in internal/component/doctor and the runner reached it by
// writing its name out. It judges every configured ethernet entry the way the
// apply path binds it, by the device its selector names, so the question is
// this component's (ai/patterns/registration.md, "Doctor Check Registry"): it
// is owned here now and dropping this component drops its check with it.
// Linux-only because the devices it reads are sysfs entries.

package iface

import (
	"os"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The codes the check emits. internal/core/diagnostic/codes.go declares each
// one, so `ze explain <code>` answers. Each is its own declaration: one code
// is one fact, and a block of them reads as a second list of the code
// registry (./le arch enumeration check).

// codeIfaceMissing names a configured ethernet device sysfs does not hold.
const codeIfaceMissing = "doctor-iface-missing"

// codeIfaceDown names a configured ethernet device whose operstate is down.
const codeIfaceDown = "doctor-iface-down"

// codeIfaceMACOverrideByName names an entry that overrides a MAC on a device it
// reaches by name.
const codeIfaceMACOverrideByName = "doctor-iface-mac-override-by-name"

// The two operstate values sysfs reports for a device that carries traffic:
// "up", and "unknown" for a kind the kernel does not track a carrier for.
const (
	operstateUp      = "up"
	operstateUnknown = "unknown"
)

// doctorSysClassNet is the sysfs directory the check reads. It is a variable,
// initialized from the same constant the offload code reads, so a test points
// the check at a fixture tree; nothing else assigns it.
var doctorSysClassNet = sysClassNetDir

// ethernetDoctorCheck is the registration register_linux.go installs.
//
// Order 130 reproduces the sequence the doctor runner printed before the check
// moved onto the registry: it ran before the runner's registry dispatch, after
// the VPP API socket probe (120, internal/plugins/iface/vpp) and before the
// DHCP listen-interface check (140, internal/plugins/dhcpserver).
var ethernetDoctorCheck = diagnostic.DoctorCheck{
	Name:         "iface-ethernet",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        130,
	Component:    "iface",
	Dependencies: []string{"sysfs"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes: []string{
		codeIfaceMissing,
		codeIfaceDown,
		codeIfaceMACOverrideByName,
		diagnostic.CodeDoctorIfaceSelectorUnmatched,
		diagnostic.CodeDoctorIfaceSelectorAmbiguous,
	},
	Check: checkEthernetInterfaces,
}

// checkEthernetInterfaces reports on every configured ethernet interface:
// absent, administratively down, or carrying a hardware selector that names no
// device or more than one.
//
// The entry NAME is not the device when the entry carries a selector. `os-name`
// aliases a kernel device and `mac/match` binds to the device carrying a
// hardware address, and the config apply path keys its work by whichever one
// answers (bindDevices, config_apply.go). A check that looked the entry name
// up in sysfs therefore called a perfectly good mac/match config a missing
// interface, and said nothing at all about a selector that resolves to
// nothing.
//
// The two selector verdicts differ in severity because the daemon treats them
// differently. A selector no device answers to is a DEFERRED binding, which the
// YANG promises and the apply accepts, so it is a warning. A selector several
// devices answer to is refused by the apply, so it is an error.
//
// The VPP backend keeps its devices in VPP rather than in sysfs, so the check
// answers nothing under it. A nil tree is the missing-config phase, which this
// check does not run in, and a context carrying anything else is a runner
// defect the runner's own type assertion reports (doctorTree,
// internal/component/doctor/registry.go).
func checkEthernetInterfaces(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	ifaceBlock := tree.GetContainer(configRootInterface)
	if ifaceBlock == nil {
		return nil
	}
	backend, _ := ifaceBlock.Get("backend")
	if backend == vppBackendName {
		return nil
	}
	ethList := ifaceBlock.GetList("ethernet")
	if len(ethList) == 0 {
		return nil
	}

	var diags []diagnostic.Diagnostic
	var tb textbuf.Buffer
	for name, entry := range ethList {
		if diag := macOverrideBoundByName(name, entry); diag != nil {
			diags = append(diags, *diag)
		}
		device, diag := selectedNetDevice(name, entry)
		if diag != nil {
			diags = append(diags, *diag)
			continue
		}
		if strings.Contains(device, "..") || strings.ContainsAny(device, "/\x00") {
			continue
		}
		statePath := tb.Reset().Str(doctorSysClassNet).Byte('/').Str(device).String()
		info, err := os.Stat(statePath)
		if err != nil || !info.IsDir() {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     codeIfaceMissing,
				Severity: diagnostic.SeverityError,
				Message:  tb.Reset().Str("ethernet interface not found: ").Str(device).String(),
			})
			continue
		}
		operstate, err := os.ReadFile(tb.Reset().Str(statePath).Str("/operstate").String()) //nolint:gosec // path traversal guarded above
		if err != nil {
			continue
		}
		state := strings.TrimSpace(string(operstate))
		if state != operstateUp && state != operstateUnknown {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     codeIfaceDown,
				Severity: diagnostic.SeverityWarning,
				Message:  tb.Reset().Str("ethernet interface ").Str(device).Str(" operstate: ").Str(state).String(),
			})
		}
	}
	return diags
}

// macOverrideBoundByName returns a diagnostic for an ethernet entry that
// IMPOSES a hardware address on a device it reaches by NAME.
//
// `mac/address` is written to whichever device the entry resolves to, on every
// apply (applyConfig, config_apply.go). An entry that resolves by name
// therefore hands THIS NIC's address to a DIFFERENT NIC the first time the
// kernel gives the name to another port, and the wrong port then answers to
// the address. `mac/match` removes the exposure: the override follows the NIC
// it was written for. An `os-name` alias is still a name, so it earns the same
// report.
//
// The check reads the config alone. Comparing the configured address against
// the device's current address answers nothing, because the apply has already
// written one onto the other; the permanent address is what the entry should
// have been bound by in the first place, which is what the message says.
//
// It is a warning, not an error, because an override on a named interface is
// a valid config an operator can mean. The advice holds either way.
func macOverrideBoundByName(name string, entry *config.Tree) *diagnostic.Diagnostic {
	if entry == nil {
		return nil
	}
	mac := entry.GetContainer("mac")
	if mac == nil {
		return nil
	}
	address, ok := mac.Get("address")
	if !ok || address == "" {
		return nil
	}
	if match, matched := mac.Get("match"); matched && match != "" {
		return nil
	}
	var tb textbuf.Buffer
	return &diagnostic.Diagnostic{
		Code:     codeIfaceMACOverrideByName,
		Severity: diagnostic.SeverityWarning,
		Message: tb.Str("ethernet ").Str(name).Str(": mac address ").Str(address).
			Str(" is written to whichever device this entry's name reaches; bind the entry with mac match <permanent address> so the override stays on one NIC").String(),
	}
}

// selectedNetDevice returns the kernel device an ethernet entry configures, or
// a diagnostic when its hardware selector names no device or more than one.
// mac/match wins over os-name, as the YANG leaf states, and an entry with
// neither selector IS its own kernel device.
func selectedNetDevice(name string, entry *config.Tree) (string, *diagnostic.Diagnostic) {
	if entry == nil {
		return name, nil
	}
	var tb textbuf.Buffer
	if mac := entry.GetContainer("mac"); mac != nil {
		if match, ok := mac.Get("match"); ok && match != "" {
			devices := netDevicesWithAddress(match)
			switch len(devices) {
			case 1:
				return devices[0], nil
			case 0:
				return "", &diagnostic.Diagnostic{
					Code:     diagnostic.CodeDoctorIfaceSelectorUnmatched,
					Severity: diagnostic.SeverityWarning,
					Message: tb.Reset().Str("ethernet ").Str(name).Str(": no device carries MAC ").Str(match).
						Str("; the binding stays deferred until one appears").String(),
				}
			default:
				return "", &diagnostic.Diagnostic{
					Code:     diagnostic.CodeDoctorIfaceSelectorAmbiguous,
					Severity: diagnostic.SeverityError,
					Message: tb.Reset().Str("ethernet ").Str(name).Str(": MAC ").Str(match).Str(" is carried by ").
						Str(strings.Join(devices, ", ")).Str("; a hardware MAC selects at most one device").String(),
				}
			}
		}
	}
	if osName, ok := entry.Get("os-name"); ok && osName != "" {
		return osName, nil
	}
	return name, nil
}

// netDevicesWithAddress returns the names of the kernel devices that carry mac
// as their OWN hardware address, read from sysfs and sorted for a reproducible
// message.
//
// A device standing on another one is not a candidate, and that exclusion is
// load-bearing rather than tidy. A vlan and a macvlan hang off a parent, and a
// bridge or a bond wears the address of a port it holds, so a second device
// reports the selector's address the moment ze builds anything on the selected
// port -- which ze does, on the operator's own config
// (test/plugin/iface-bridge-mac-match-apply.ci). The config apply path skips
// the same two kinds before it matches (devicesWithMAC, config_apply.go), so a
// doctor that counted them called that config ambiguous, at SeverityError,
// while the daemon bound to the right port.
//
// sysfs exposes the CURRENT address; the daemon's resolver prefers the
// permanent (factory) one and falls back to the current address only for the
// virtual kinds that report none (deviceMatchMAC, resolve.go). The two agree
// unless something outside ze overrode a NIC's operational address, and on
// such a box this check reports "no device carries MAC" where the daemon
// binds. That is why the unmatched verdict is a warning naming the address it
// compared, rather than an error.
func netDevicesWithAddress(mac string) []string {
	target := strings.ToLower(strings.TrimSpace(mac))
	entries, err := os.ReadDir(doctorSysClassNet)
	if err != nil {
		return nil
	}
	var tb textbuf.Buffer
	var found []string
	for _, e := range entries {
		device := e.Name()
		if strings.Contains(device, "..") || strings.ContainsAny(device, "/\x00") {
			continue
		}
		deviceDir := tb.Reset().Str(doctorSysClassNet).Byte('/').Str(device).String()
		addr, readErr := os.ReadFile(tb.Reset().Str(deviceDir).Str("/address").String()) //nolint:gosec // path traversal guarded above
		if readErr != nil {
			continue
		}
		if strings.ToLower(strings.TrimSpace(string(addr))) != target {
			continue
		}
		if hasLowerDevice(deviceDir) {
			continue
		}
		found = append(found, device)
	}
	slices.Sort(found)
	return found
}

// hasLowerDevice reports whether the kernel lists a device standing under the
// one in deviceDir, which is what makes the address that device reports
// somebody else's. sysfs writes the relation on the UPPER device, as a
// lower_<name> symlink, and it writes it for both kinds a hardware selector
// must skip: a stacked device gets one for its parent, and an aggregator gets
// one for each member it holds.
//
// Measured against a live kernel on 2026-08-19, in a network namespace holding
// one dummy port: a vlan and a macvlan on that port each carry lower_<port>, a
// bridge carries lower_<port> once the port is enslaved and carries nothing
// before that, and a veth carries none at all -- its peer is IFLA_LINK, which
// sysfs reports as iflink rather than as a link. The port itself carries
// master and upper_<bridge>, never a lower_ link, so it stays a candidate here
// exactly as it stays one in the apply path.
//
// A directory that cannot be read reports no lower device. The caller has just
// read this device's address file, so the only reader that arrives here on an
// unreadable directory is one racing a device that is being removed, and the
// device it would judge no longer exists.
func hasLowerDevice(deviceDir string) bool {
	entries, err := os.ReadDir(deviceDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "lower_") {
			return true
		}
	}
	return false
}
