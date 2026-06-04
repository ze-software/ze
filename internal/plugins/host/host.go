// Design: plan/spec-host-0-inventory.md — offline `ze host show` CLI

// Package host is the offline home for `ze host show [section] [--text]`.
// It reads the hardware inventory directly from sysfs/procfs (no daemon
// required) and writes JSON to stdout by default. ISPs pipe the JSON
// into monitoring/alerting pipelines; `--text` renders a human-readable
// summary for interactive use.
//
// Sibling consumers of the same inventory library:
//
//	internal/component/cmd/show/host.go   online `show host *` RPCs
//	internal/component/host/inventory.go  detection library
package host

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	hostinv "codeberg.org/thomas-mangin/ze/internal/component/host"
)

// All section names and their detector bodies live in the host
// package (`internal/component/host/inventory.go`). This CLI derives
// from that single source of truth — see rules/derive-not-hardcode.md.
// An earlier draft of this file carried a parallel `validSections`
// map; it was deleted when the canonical registry landed in the
// host package.

// RunHint is the handler for bare `ze host` (no subcommand). Prints
// one usage line to stdout and returns 0. The registry routes
// subcommand-less invocations here so the operator sees a purpose-
// ful hint rather than the generic dispatcher's "unknown command"
// response.
func RunHint(_ []string) int {
	usage := "Usage: ze host show [" + strings.ReplaceAll(sectionList(), ", ", "|") + "] [--text]\n"
	if _, err := os.Stdout.WriteString(usage); err != nil {
		fmt.Fprintf(os.Stderr, "error: write: %v\n", err)
		return 1
	}
	return 0
}

// RunShow implements `ze host show [section] [--text]`. Default output
// is JSON (machine-parseable — ISP pipelines); `--text` produces a
// human-readable summary. Returns 0 on success, 1 on argument / IO
// error.
func RunShow(args []string) int {
	fs := flag.NewFlagSet("host show", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	text := fs.Bool("text", false, "human-readable output (default: JSON)")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ze host show [section] [--text]\n")
		fmt.Fprintf(os.Stderr, "\nSections: %s\n", sectionList())
		fmt.Fprintf(os.Stderr, "\nDefault section is 'all'. Output defaults to JSON.\n")
	}
	if err := fs.Parse(args); err != nil {
		return 1
	}

	positional := fs.Args()
	section := "all"
	if len(positional) > 0 {
		section = strings.ToLower(positional[0])
	}

	data, err := hostinv.DetectSection(section)
	if err != nil {
		if errors.Is(err, hostinv.ErrUnknownSection) {
			fmt.Fprintf(os.Stderr, "error: unknown section %q; valid: %s\n", section, sectionList())
			return 1
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if *text {
		return renderText(section, data)
	}
	return renderJSON(data)
}

// renderJSON writes the inventory as indented JSON to stdout.
func renderJSON(data any) int {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: marshal: %v\n", err)
		return 1
	}
	if _, err := os.Stdout.Write(append(b, '\n')); err != nil {
		fmt.Fprintf(os.Stderr, "error: write: %v\n", err)
		return 1
	}
	return 0
}

// renderText writes a minimal human-readable summary. The format is
// stable for interactive use but NOT a machine contract — scripts
// must use JSON. Every section type has its own printer so
// `--text` works uniformly; the final default only fires for
// types the library grows in the future but this CLI hasn't
// learned yet (fall back to JSON rather than panic).
func renderText(section string, data any) int {
	switch v := data.(type) {
	case *hostinv.CPUInfo:
		return renderCPUText(v)
	case []hostinv.NICInfo:
		return renderNICText(v)
	case *hostinv.DMIInfo:
		return renderDMIText(v)
	case *hostinv.MemoryInfo:
		return renderMemoryText(v)
	case *hostinv.ThermalInfo:
		return renderThermalText(v)
	case *hostinv.StorageInfo:
		return renderStorageText(v)
	case *hostinv.KernelInfo:
		return renderKernelText(v)
	case *hostinv.HostInfo:
		return renderHostText(v)
	case *hostinv.Inventory:
		return renderInventoryText(v)
	}
	// Unknown type for a future section: fall back to JSON so the
	// operator still sees something.
	_ = section
	return renderJSON(data)
}

func renderCPUText(v *hostinv.CPUInfo) int {
	if v == nil {
		fmt.Println("(no CPU inventory available)")
		return 0
	}
	fmt.Printf("CPU:      %s\n", v.ModelName)
	fmt.Printf("Vendor:   %s\n", v.Vendor)
	fmt.Printf("Cores:    %d logical / %d physical (hybrid=%v)\n",
		v.LogicalCPUs, v.PhysicalCores, v.Hybrid)
	fmt.Printf("Scaling:  %s (hwp=%v)\n", v.ScalingDriver, v.HWPAvailable)
	if v.MaxFreqMHz > 0 {
		fmt.Printf("Freq:     base %d MHz, max %d MHz, current avg %d MHz\n",
			v.BaseFreqMHz, v.MaxFreqMHz, v.CurrentFreqAvgMHz)
	}
	return 0
}

func renderNICText(v []hostinv.NICInfo) int {
	if len(v) == 0 {
		fmt.Println("(no physical NICs detected)")
		return 0
	}
	fmt.Printf("%-12s %-10s %-10s %-18s %s\n", "NAME", "DRIVER", "SPEED", "MAC", "PCI")
	for i := range v {
		n := &v[i]
		speed := "-"
		if n.Carrier && n.LinkSpeedMbps > 0 {
			speed = fmt.Sprintf("%dMbps", n.LinkSpeedMbps)
		}
		fmt.Printf("%-12s %-10s %-10s %-18s %s:%s\n",
			n.Name, n.Driver, speed, n.MAC, n.PCIVendor, n.PCIDevice)
	}
	return 0
}

func renderDMIText(v *hostinv.DMIInfo) int {
	if v == nil {
		fmt.Println("(no DMI data available)")
		return 0
	}
	printKV := func(label, value string) {
		if value != "" {
			fmt.Printf("%-18s %s\n", label, value)
		}
	}
	printKV("System vendor:", v.SystemVendor)
	printKV("System product:", v.SystemProduct)
	printKV("Board vendor:", v.BoardVendor)
	printKV("Board product:", v.BoardProduct)
	printKV("BIOS vendor:", v.BIOSVendor)
	printKV("BIOS version:", v.BIOSVersion)
	printKV("BIOS date:", v.BIOSDate)
	printKV("Chassis vendor:", v.ChassisVendor)
	printKV("Chassis type:", v.ChassisType)
	for _, e := range v.Errors {
		fmt.Printf("(unreadable) %s: %s\n", e.Path, e.Err)
	}
	return 0
}

func renderMemoryText(v *hostinv.MemoryInfo) int {
	if v == nil {
		fmt.Println("(no memory data available)")
		return 0
	}
	fmt.Printf("Total:     %d bytes (%.2f GiB)\n", v.TotalBytes, float64(v.TotalBytes)/(1<<30))
	fmt.Printf("Available: %d bytes\n", v.AvailableBytes)
	fmt.Printf("Free:      %d bytes\n", v.FreeBytes)
	fmt.Printf("Swap:      %d total / %d free\n", v.SwapTotalBytes, v.SwapFreeBytes)
	if v.ECCPresent {
		fmt.Printf("ECC:       present (correctable=%d, uncorrectable=%d)\n",
			v.ECCCorrectableErrors, v.ECCUncorrectableErrors)
	} else {
		fmt.Println("ECC:       not present")
	}
	return 0
}

func renderThermalText(v *hostinv.ThermalInfo) int {
	if v == nil || (len(v.Sensors) == 0 && len(v.Throttle) == 0) {
		fmt.Println("(no thermal data available)")
		return 0
	}
	if len(v.Sensors) > 0 {
		fmt.Printf("%-12s %-16s %s\n", "HWMON", "DEVICE", "TEMP")
		for i := range v.Sensors {
			s := &v.Sensors[i]
			fmt.Printf("%-12s %-16s %.1f C%s\n",
				s.Name, s.Device, float64(s.TempMC)/1000.0, alarmSuffix(s.Alarm))
		}
	}
	if len(v.Throttle) > 0 {
		fmt.Println()
		fmt.Printf("%-6s %-22s %s\n", "CPU", "CORE THROTTLE COUNT", "PACKAGE THROTTLE COUNT")
		for i := range v.Throttle {
			t := &v.Throttle[i]
			fmt.Printf("cpu%-3d %-22d %d\n",
				t.CPU, t.CoreThrottleCount, t.PackageThrottleCount)
		}
	}
	return 0
}

func alarmSuffix(alarm bool) string {
	if alarm {
		return " !ALARM"
	}
	return ""
}

func renderStorageText(v *hostinv.StorageInfo) int {
	if v == nil || len(v.Devices) == 0 {
		fmt.Println("(no block devices detected)")
		return 0
	}
	fmt.Printf("%-12s %-10s %-8s %-24s %s\n", "NAME", "TRANSPORT", "ROTATE", "MODEL", "SIZE")
	for i := range v.Devices {
		dev := &v.Devices[i]
		rot := "no"
		if dev.Rotational {
			rot = "yes"
		}
		fmt.Printf("%-12s %-10s %-8s %-24s %.2f GiB\n",
			dev.Name, dev.Transport, rot, dev.Model, float64(dev.SizeBytes)/(1<<30))
	}
	return 0
}

func renderKernelText(v *hostinv.KernelInfo) int {
	if v == nil {
		fmt.Println("(no kernel data available)")
		return 0
	}
	fmt.Printf("Release:      %s\n", v.Release)
	fmt.Printf("Architecture: %s\n", v.Architecture)
	if v.BootTime != "" {
		fmt.Printf("Boot time:    %s\n", v.BootTime)
	}
	if v.MicrocodeRevision != "" {
		fmt.Printf("Microcode:    %s\n", v.MicrocodeRevision)
	}
	if len(v.ArchFlags) > 0 {
		fmt.Printf("Arch flags:   %s\n", strings.Join(v.ArchFlags, " "))
	}
	if v.Cmdline != "" {
		fmt.Printf("Cmdline:      %s\n", v.Cmdline)
	}
	return 0
}

func renderHostText(v *hostinv.HostInfo) int {
	if v == nil {
		fmt.Println("(no host data available)")
		return 0
	}
	fmt.Printf("Hostname: %s\n", v.Hostname)
	fmt.Printf("Uptime:   %d seconds\n", v.UptimeSeconds)
	fmt.Printf("Timezone: %s\n", v.Timezone)
	return 0
}

func renderInventoryText(v *hostinv.Inventory) int {
	if v == nil {
		fmt.Println("(no inventory data available)")
		return 0
	}
	// Empty-inventory case (e.g. darwin where every section stub
	// returns ErrUnsupported, or a container where /sys is hidden).
	// Surface a single explanatory line rather than silent zero-byte
	// output so the operator knows detection ran but found nothing.
	empty := v.CPU == nil && len(v.NICs) == 0 && v.DMI == nil &&
		v.Memory == nil && v.Thermal == nil && v.Storage == nil &&
		v.Kernel == nil && v.Host == nil && len(v.Errors) == 0
	if empty {
		fmt.Println("(nothing to show — host inventory is not available on this platform)")
		return 0
	}
	if v.CPU != nil {
		fmt.Println("--- CPU ---")
		renderCPUText(v.CPU)
	}
	if len(v.NICs) > 0 {
		fmt.Println("\n--- NICs ---")
		renderNICText(v.NICs)
	}
	if v.DMI != nil {
		fmt.Println("\n--- DMI ---")
		renderDMIText(v.DMI)
	}
	if v.Memory != nil {
		fmt.Println("\n--- Memory ---")
		renderMemoryText(v.Memory)
	}
	if v.Thermal != nil {
		fmt.Println("\n--- Thermal ---")
		renderThermalText(v.Thermal)
	}
	if v.Storage != nil {
		fmt.Println("\n--- Storage ---")
		renderStorageText(v.Storage)
	}
	if v.Kernel != nil {
		fmt.Println("\n--- Kernel ---")
		renderKernelText(v.Kernel)
	}
	if v.Host != nil {
		fmt.Println("\n--- Host ---")
		renderHostText(v.Host)
	}
	for _, e := range v.Errors {
		fmt.Printf("\n(error) %s: %s\n", e.Path, e.Err)
	}
	return 0
}

// sectionList returns the sorted, comma-separated list of valid
// section names for error messages and help output. Thin alias over
// host.SectionList() — kept so the local call sites stay readable
// and the canonical source remains in the host package.
func sectionList() string {
	return hostinv.SectionList()
}
