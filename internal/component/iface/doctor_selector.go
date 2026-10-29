// Design: docs/architecture/iface/logical-name-resolution.md -- the config-only binding a doctor check reads
// Related: doctor_linux.go -- checkEthernetInterfaces and netDevicesWithAddress, the sysfs side
// Related: doctor_selector_other.go -- netDevicesWithAddress where there is no sysfs
// Related: config_apply.go -- bindDevices, the apply-path binding this mirrors

package iface

import (
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// SelectedNetDevice answers the kernel device the ethernet entry called name
// in tree configures, the way the config apply binds it (bindDevices), or a
// diagnostic when its hardware selector names no device or more than one. An
// unmatched selector is a warning, because the apply defers the binding; an
// ambiguous one is an error, because the apply refuses it.
//
// It reads the config tree and sysfs only. A doctor check runs in the CLI
// process, where no iface backend runs and the resolver's selector table
// (ResolveDevice) is empty, so the tree is the only place the selector can be
// read there. A name with no ethernet entry, or an entry with neither
// selector, IS its own kernel device.
//
// tree MUST be non-nil.
func SelectedNetDevice(tree *config.Tree, name string) (string, *diagnostic.Diagnostic) {
	var entry *config.Tree
	if ifaceBlock := tree.GetContainer(configRootInterface); ifaceBlock != nil {
		entry = ifaceBlock.GetList("ethernet")[name]
	}
	return selectedNetDevice(name, entry)
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
				if !macSelectorsReadable {
					return "", &diagnostic.Diagnostic{
						Code:     diagnostic.CodeDoctorIfaceSelectorUnmatched,
						Severity: diagnostic.SeverityWarning,
						Message: tb.Reset().Str("ethernet ").Str(name).Str(": MAC ").Str(match).
							Str(" was not checked, because MAC selectors are only checked on Linux").String(),
					}
				}
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
