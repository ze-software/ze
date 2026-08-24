// Design: docs/architecture/appliance/installer-initrd.md -- R-6 forced-panic fault injection (evidence-only)

//go:build linux && ze_installer && ze_installer_fault

package disk

import (
	"log/slog"
	"os"
	"strings"
)

// maybeInjectFault is R-6's fault-injection evidence hook. It is compiled ONLY
// under the ze_installer_fault build tag, so the shipping installer initrd
// never contains it (the production build uses just ze_installer; the QEMU
// evidence harness sets ZE_INITRD_FAULT=1 to add the tag).
func maybeInjectFault(cfg installConfig) {
	injectFault(cfg, cmdlineFault())
}

// injectFault dispatches on the ze.fault value. For ze.fault=panic-goroutine it
// spawns a goroutine that hits a real runtime fault (a nil-map write, the kind
// R-6 worries about: an unexpected nil deref deep in a netlink/dhcp/net/http
// library goroutine), guarded by the same defer/recover->fatalInitrd pattern
// every installer worker goroutine must use. The main goroutine then blocks, so
// the ONLY way forward is through the spawned goroutine's recover: a working
// R-6 mitigation prints "recovered goroutine panic" and reaches the
// FATAL/reboot path; a broken one lets the runtime kill PID 1 and the kernel's
// panic=-1 reboots without that marker. Either way the box reboots instead of
// hanging, which is the property R-6 demands.
func injectFault(cfg installConfig, fault string) {
	switch fault {
	case "":
		return
	case "panic-goroutine":
		slog.Warn("fault injection armed", "fault", fault)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("recovered goroutine panic", "panic", r)
					fatalInitrd(cfg, "fault-injection: panic in goroutine")
				}
			}()
			triggerRuntimeFault()
		}()
		// Halt the main goroutine: the recovered fault goroutine now drives the
		// FATAL/reboot path. Without this the main flow would race a normal
		// install to completion and mask the recovery being exercised.
		select {}
	default:
		slog.Warn("unknown ze.fault value, ignoring", "fault", fault)
	}
}

// triggerRuntimeFault forces a real runtime fault (assignment to a nil map),
// modeling an unexpected nil deref deep in a library goroutine rather than a
// hand-rolled fault string.
func triggerRuntimeFault() {
	var m map[string]int
	m["fault"] = 1 //nolint:staticcheck // SA5000: the write to a nil map IS the fault this function injects
}

// cmdlineFault returns the ze.fault value from /proc/cmdline, or "" if absent.
// The fault hook parses the cmdline itself so the production installConfig and
// its parser carry no evidence-only field.
func cmdlineFault() string {
	data, err := os.ReadFile("/proc/cmdline")
	if err != nil {
		return ""
	}
	return parseFaultParam(strings.TrimSpace(string(data)))
}

// parseFaultParam extracts the first ze.fault=<value> from a kernel cmdline.
func parseFaultParam(line string) string {
	for param := range strings.FieldsSeq(line) {
		if v, ok := strings.CutPrefix(param, "ze.fault="); ok {
			return v
		}
	}
	return ""
}
