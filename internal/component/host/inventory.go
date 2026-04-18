// Design: plan/spec-host-0-inventory.md — hardware inventory detection

package host

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrUnsupported is returned by detectors on platforms that cannot
// provide the information (e.g. darwin for all Linux-specific sections).
var ErrUnsupported = errors.New("host inventory: unsupported on this platform")

// strUnknown is the canonical JSON string for any enum value that
// cannot be mapped. Shared by all enum String() fallbacks.
const strUnknown = "unknown"

// Inventory is the full hardware inventory for the host. Every section is
// a pointer so "unknown" or "unsupported on this platform" is expressible
// as nil rather than a zero value that could be confused with real data.
//
// Every field carries explicit units in its name (*-bytes, *-mhz, *-mc for
// millicelsius, *-mbps, *-seconds). Enum-valued fields are typed identities
// internally; the String forms appear only in JSON output.
type Inventory struct {
	CPU     *CPUInfo     `json:"cpu,omitempty"`
	NICs    []NICInfo    `json:"nics,omitempty"`
	DMI     *DMIInfo     `json:"dmi,omitempty"`
	Memory  *MemoryInfo  `json:"memory,omitempty"`
	Thermal *ThermalInfo `json:"thermal,omitempty"`
	Storage *StorageInfo `json:"storage,omitempty"`
	Kernel  *KernelInfo  `json:"kernel,omitempty"`
	Host    *HostInfo    `json:"host,omitempty"`

	// Errors collects non-fatal read/parse errors encountered during
	// detection. A populated Errors slice with a non-nil section means
	// "section is partial"; callers should surface the errors.
	Errors []DetectError `json:"errors,omitempty"`
}

// DetectError records a single non-fatal failure during detection.
// Path identifies the sysfs/procfs node; Err is the wrapped OS error.
type DetectError struct {
	Path string `json:"path"`
	Err  string `json:"error"`
}

// CPUVendor is a typed enum for the CPU vendor string. Zero value is
// CPUVendorUnknown so uninitialised values surface immediately.
type CPUVendor uint8

const (
	CPUVendorUnknown CPUVendor = 0
	CPUVendorIntel   CPUVendor = 1
	CPUVendorAMD     CPUVendor = 2
	CPUVendorOther   CPUVendor = 255
)

var cpuVendorNames = map[CPUVendor]string{
	CPUVendorUnknown: "unknown",
	CPUVendorIntel:   "intel",
	CPUVendorAMD:     "amd",
	CPUVendorOther:   "other",
}

// String returns the canonical lowercase string form of the vendor.
// Used only for JSON output; comparisons use the typed constants.
func (v CPUVendor) String() string {
	if s, ok := cpuVendorNames[v]; ok {
		return s
	}
	return strUnknown
}

// MarshalJSON emits the string form so JSON consumers see "intel" not 1.
func (v CPUVendor) MarshalJSON() ([]byte, error) {
	return []byte(`"` + v.String() + `"`), nil
}

// CoreRole classifies a core in a hybrid topology. CoreRoleUniform means
// the CPU is not hybrid (all cores equivalent).
type CoreRole uint8

const (
	CoreRoleUnknown     CoreRole = 0
	CoreRoleUniform     CoreRole = 1
	CoreRolePerformance CoreRole = 2
	CoreRoleEfficient   CoreRole = 3
)

var coreRoleNames = map[CoreRole]string{
	CoreRoleUnknown:     "unknown",
	CoreRoleUniform:     "uniform",
	CoreRolePerformance: "performance",
	CoreRoleEfficient:   "efficient",
}

// String returns the lowercase name of the role.
func (r CoreRole) String() string {
	if s, ok := coreRoleNames[r]; ok {
		return s
	}
	return strUnknown
}

// MarshalJSON emits the string form.
func (r CoreRole) MarshalJSON() ([]byte, error) {
	return []byte(`"` + r.String() + `"`), nil
}

// ScalingDriver identifies the cpufreq scaling driver in use.
type ScalingDriver uint8

const (
	ScalingDriverUnknown     ScalingDriver = 0
	ScalingDriverIntelPState ScalingDriver = 1
	ScalingDriverAMDPState   ScalingDriver = 2
	ScalingDriverACPICpufreq ScalingDriver = 3
	ScalingDriverOther       ScalingDriver = 255
)

var scalingDriverNames = map[ScalingDriver]string{
	ScalingDriverUnknown:     "unknown",
	ScalingDriverIntelPState: "intel_pstate",
	ScalingDriverAMDPState:   "amd_pstate",
	ScalingDriverACPICpufreq: "acpi_cpufreq",
	ScalingDriverOther:       "other",
}

// String returns the kernel name of the driver.
func (d ScalingDriver) String() string {
	if s, ok := scalingDriverNames[d]; ok {
		return s
	}
	return strUnknown
}

// MarshalJSON emits the string form.
func (d ScalingDriver) MarshalJSON() ([]byte, error) {
	return []byte(`"` + d.String() + `"`), nil
}

// NICTransport names the PCI/USB/other bus the NIC sits on.
type NICTransport uint8

const (
	NICTransportUnknown  NICTransport = 0
	NICTransportPCI      NICTransport = 1
	NICTransportUSB      NICTransport = 2
	NICTransportPlatform NICTransport = 3
)

var nicTransportNames = map[NICTransport]string{
	NICTransportUnknown:  "unknown",
	NICTransportPCI:      "pci",
	NICTransportUSB:      "usb",
	NICTransportPlatform: "platform",
}

// String returns the lowercase name of the transport.
func (t NICTransport) String() string {
	if s, ok := nicTransportNames[t]; ok {
		return s
	}
	return strUnknown
}

// MarshalJSON emits the string form.
func (t NICTransport) MarshalJSON() ([]byte, error) {
	return []byte(`"` + t.String() + `"`), nil
}

// CPUInfo describes the CPU package(s) and individual cores.
type CPUInfo struct {
	Vendor            CPUVendor     `json:"vendor"`
	ModelName         string        `json:"model-name,omitempty"`
	Family            int           `json:"family,omitempty"`
	Model             int           `json:"model,omitempty"`
	Stepping          int           `json:"stepping,omitempty"`
	Microcode         string        `json:"microcode,omitempty"`
	LogicalCPUs       int           `json:"logical-cpus"`
	PhysicalCores     int           `json:"physical-cores,omitempty"`
	ThreadsPerCore    int           `json:"threads-per-core,omitempty"`
	Hybrid            bool          `json:"hybrid"`
	ScalingDriver     ScalingDriver `json:"scaling-driver"`
	HWPAvailable      bool          `json:"hwp-available"`
	BaseFreqMHz       int           `json:"base-freq-mhz,omitempty"`
	MaxFreqMHz        int           `json:"max-freq-mhz,omitempty"`
	CurrentFreqAvgMHz int           `json:"current-freq-mhz-avg,omitempty"`
	Cores             []CoreInfo    `json:"cores,omitempty"`
	Flags             []string      `json:"flags,omitempty"`
}

// CoreInfo describes one logical CPU (one entry per `processor` in
// /proc/cpuinfo).
type CoreInfo struct {
	CPU                  int      `json:"cpu"`
	CoreID               int      `json:"core-id"`
	PhysicalPackage      int      `json:"physical-package-id"`
	Role                 CoreRole `json:"role"`
	Capacity             int      `json:"capacity,omitempty"`
	CurrentFreqMHz       int      `json:"current-freq-mhz,omitempty"`
	CoreThrottleCount    uint64   `json:"core-throttle-count"`
	PackageThrottleCount uint64   `json:"package-throttle-count"`
}

// NICInfo describes one physical network interface.
type NICInfo struct {
	Name            string       `json:"name"`
	Driver          string       `json:"driver,omitempty"`
	FirmwareVersion string       `json:"firmware-version,omitempty"`
	Transport       NICTransport `json:"transport"`
	PCIVendor       string       `json:"pci-vendor,omitempty"`
	PCIDevice       string       `json:"pci-device,omitempty"`
	MAC             string       `json:"mac,omitempty"`
	LinkSpeedMbps   int          `json:"link-speed-mbps"`
	Duplex          string       `json:"duplex,omitempty"`
	Carrier         bool         `json:"carrier"`
	RxQueues        int          `json:"rx-queues,omitempty"`
	TxQueues        int          `json:"tx-queues,omitempty"`
	RingRx          int          `json:"ring-rx,omitempty"`
	RingTx          int          `json:"ring-tx,omitempty"`
}

// DMIInfo mirrors the fields exposed under /sys/class/dmi/id/. Unreadable
// fields (file genuinely absent) are left empty (JSON omitempty).
// Permission-denied and other non-absence errors surface in Errors.
type DMIInfo struct {
	SystemVendor  string        `json:"system-vendor,omitempty"`
	SystemProduct string        `json:"system-product,omitempty"`
	SystemVersion string        `json:"system-version,omitempty"`
	SystemSerial  string        `json:"system-serial,omitempty"`
	BoardVendor   string        `json:"board-vendor,omitempty"`
	BoardProduct  string        `json:"board-product,omitempty"`
	BoardVersion  string        `json:"board-version,omitempty"`
	BoardSerial   string        `json:"board-serial,omitempty"`
	BIOSVendor    string        `json:"bios-vendor,omitempty"`
	BIOSVersion   string        `json:"bios-version,omitempty"`
	BIOSDate      string        `json:"bios-date,omitempty"`
	BIOSRevision  string        `json:"bios-revision,omitempty"`
	ChassisVendor string        `json:"chassis-vendor,omitempty"`
	ChassisType   string        `json:"chassis-type,omitempty"`
	ChassisSerial string        `json:"chassis-serial,omitempty"`
	Errors        []DetectError `json:"errors,omitempty"`
}

// MemoryInfo mirrors /proc/meminfo plus edac counters when present.
type MemoryInfo struct {
	TotalBytes             uint64 `json:"total-bytes"`
	FreeBytes              uint64 `json:"free-bytes"`
	AvailableBytes         uint64 `json:"available-bytes"`
	BuffersBytes           uint64 `json:"buffers-bytes"`
	CachedBytes            uint64 `json:"cached-bytes"`
	SwapTotalBytes         uint64 `json:"swap-total-bytes"`
	SwapFreeBytes          uint64 `json:"swap-free-bytes"`
	ECCCorrectableErrors   uint64 `json:"ecc-correctable-errors"`
	ECCUncorrectableErrors uint64 `json:"ecc-uncorrectable-errors"`
	ECCPresent             bool   `json:"ecc-present"`
}

// ThermalInfo aggregates hwmon sensor readings and per-core throttle
// counters.
type ThermalInfo struct {
	Sensors  []SensorReading `json:"sensors,omitempty"`
	Throttle []ThrottleEntry `json:"throttle,omitempty"`
}

// SensorReading is one hwmon temperature reading in millicelsius.
type SensorReading struct {
	Name   string `json:"name"`
	Device string `json:"device,omitempty"`
	TempMC int64  `json:"temp-mc"`
	Alarm  bool   `json:"alarm"`
}

// ThrottleEntry collapses per-CPU thermal_throttle counters.
type ThrottleEntry struct {
	CPU                  int    `json:"cpu"`
	CoreThrottleCount    uint64 `json:"core-throttle-count"`
	PackageThrottleCount uint64 `json:"package-throttle-count"`
}

// StorageInfo lists block devices.
type StorageInfo struct {
	Devices []StorageDevice `json:"devices,omitempty"`
}

// StorageDevice describes one block device.
type StorageDevice struct {
	Name         string `json:"name"`
	SizeBytes    uint64 `json:"size-bytes"`
	Model        string `json:"model,omitempty"`
	Serial       string `json:"serial,omitempty"`
	Transport    string `json:"transport,omitempty"`
	Rotational   bool   `json:"rotational"`
	NVMeFirmware string `json:"nvme-firmware-version,omitempty"`
}

// KernelInfo reports /proc/version, /proc/cmdline, and selected CPU
// security flags.
type KernelInfo struct {
	Release           string   `json:"release"`
	Version           string   `json:"version,omitempty"`
	Architecture      string   `json:"architecture,omitempty"`
	Cmdline           string   `json:"cmdline,omitempty"`
	BootTime          string   `json:"boot-time,omitempty"`
	BootTimeUnix      int64    `json:"boot-time-unix,omitempty"`
	MicrocodeRevision string   `json:"microcode-revision,omitempty"`
	ArchFlags         []string `json:"arch-flags,omitempty"`
}

// HostInfo is metadata about the host process's environment.
type HostInfo struct {
	Hostname      string `json:"hostname,omitempty"`
	UptimeSeconds uint64 `json:"uptime-seconds"`
	Timezone      string `json:"timezone,omitempty"`
}

// Detector runs inventory detection with configurable filesystem root.
// The default Detector (zero value) reads from "/" and is safe for
// production use. Tests construct their own Detector with Root pointed
// at a testdata tree.
//
// Detector is safe for concurrent use; methods are stateless.
type Detector struct {
	// Root is the filesystem root used when joining sysfs/procfs paths.
	// Empty means "/" (production).
	Root string
}

// defaultDetector reads from the real root.
var defaultDetector = &Detector{}

// Path helpers (sysfsPath, procPath, root) live in fsroot_linux.go
// because they are only used by Linux-specific detectors. Non-Linux
// platforms return ErrUnsupported before reaching sysfs/procfs.

// Detect returns the full Inventory. Section-level failures populate
// Errors but do NOT return an error from Detect; the top-level error
// is only returned for setup failures.
func Detect() (*Inventory, error) {
	return defaultDetector.Detect()
}

// ErrUnknownSection is returned by DetectSection when the caller
// asks for a name that is not registered. Consumers may compare via
// errors.Is to surface a consistent error message.
var ErrUnknownSection = errors.New("host: unknown section")

// sectionDetectors is the SINGLE SOURCE OF TRUTH for the set of
// sections exposed by `ze host show` (offline) and the online
// `show host *` RPCs. Adding a new section means one entry here; all
// consumers — the online dispatcher, the offline CLI, the usage
// banners, the error messages — DERIVE from this map via
// SectionNames() and DetectSection().
//
// See rules/derive-not-hardcode.md: the anti-pattern this map
// replaces was a second parallel `validSections` table in
// cmd/ze/host/host.go that had to be kept in sync by hand.
var sectionDetectors = map[string]func(*Detector) (any, error){
	"cpu":     func(d *Detector) (any, error) { return d.DetectCPU() },
	"nic":     func(d *Detector) (any, error) { return d.DetectNICs() },
	"dmi":     func(d *Detector) (any, error) { return d.DetectDMI() },
	"memory":  func(d *Detector) (any, error) { return d.DetectMemory() },
	"thermal": func(d *Detector) (any, error) { return d.DetectThermal() },
	"storage": func(d *Detector) (any, error) { return d.DetectStorage() },
	"kernel":  func(d *Detector) (any, error) { return d.DetectKernel() },
	"all":     func(d *Detector) (any, error) { return d.Detect() },
}

// SectionNames returns the sorted list of valid section names. The
// slice is freshly allocated on every call so callers can mutate it
// safely (e.g. for display formatting) without affecting the registry.
func SectionNames() []string {
	names := make([]string, 0, len(sectionDetectors))
	for k := range sectionDetectors {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// SectionList returns the sorted section names joined with ", ". Used
// by help text, error messages, and Meta.Subs strings — one function
// means one canonical string shape across the codebase.
func SectionList() string {
	return strings.Join(SectionNames(), ", ")
}

// maxNameInError bounds the length of the `name` echoed in an
// ErrUnknownSection message. Without a cap, a 1 MiB argv or a
// misbehaving caller would produce a 1 MiB error string that lands
// in logs and responses. 256 bytes is enough to surface the typo an
// operator actually typed while keeping the error string small.
const maxNameInError = 256

// DetectSection dispatches to the detector for the named section and
// returns its result. Unknown names produce an ErrUnknownSection wrap
// with the valid-list hint appended; the caller's name is truncated
// to maxNameInError bytes to keep the error string bounded.
func (d *Detector) DetectSection(name string) (any, error) {
	fn, ok := sectionDetectors[name]
	if !ok {
		echoed := name
		if len(echoed) > maxNameInError {
			echoed = echoed[:maxNameInError] + "...(truncated)"
		}
		return nil, fmt.Errorf("%w: %s (valid: %s)", ErrUnknownSection, echoed, SectionList())
	}
	return fn(d)
}

// DetectSection on the default Detector — convenience for callers that
// do not need to inject a Root.
func DetectSection(name string) (any, error) {
	return defaultDetector.DetectSection(name)
}

// DetectCPU returns the CPU section using the default Detector.
func DetectCPU() (*CPUInfo, error) { return defaultDetector.DetectCPU() }

// DetectNICs returns the NIC section using the default Detector.
func DetectNICs() ([]NICInfo, error) { return defaultDetector.DetectNICs() }

// DetectDMI returns the DMI section using the default Detector.
func DetectDMI() (*DMIInfo, error) { return defaultDetector.DetectDMI() }

// DetectMemory returns the memory section using the default Detector.
func DetectMemory() (*MemoryInfo, error) { return defaultDetector.DetectMemory() }

// DetectThermal returns the thermal section using the default Detector.
func DetectThermal() (*ThermalInfo, error) { return defaultDetector.DetectThermal() }

// DetectStorage returns the storage section using the default Detector.
func DetectStorage() (*StorageInfo, error) { return defaultDetector.DetectStorage() }

// DetectKernel returns the kernel section using the default Detector.
func DetectKernel() (*KernelInfo, error) { return defaultDetector.DetectKernel() }

// DetectHost returns the host section using the default Detector.
func DetectHost() (*HostInfo, error) { return defaultDetector.DetectHost() }

// Detect assembles the full Inventory.
func (d *Detector) Detect() (*Inventory, error) {
	inv := &Inventory{}

	if cpu, err := d.DetectCPU(); err == nil {
		inv.CPU = cpu
	} else if !errors.Is(err, ErrUnsupported) {
		inv.Errors = append(inv.Errors, DetectError{Path: "cpu", Err: err.Error()})
	}

	if nics, err := d.DetectNICs(); err == nil {
		inv.NICs = nics
	} else if !errors.Is(err, ErrUnsupported) {
		inv.Errors = append(inv.Errors, DetectError{Path: "nics", Err: err.Error()})
	}

	if dmi, err := d.DetectDMI(); err == nil {
		inv.DMI = dmi
	} else if !errors.Is(err, ErrUnsupported) {
		inv.Errors = append(inv.Errors, DetectError{Path: "dmi", Err: err.Error()})
	}

	if mem, err := d.DetectMemory(); err == nil {
		inv.Memory = mem
	} else if !errors.Is(err, ErrUnsupported) {
		inv.Errors = append(inv.Errors, DetectError{Path: "memory", Err: err.Error()})
	}

	if t, err := d.DetectThermal(); err == nil {
		inv.Thermal = t
	} else if !errors.Is(err, ErrUnsupported) {
		inv.Errors = append(inv.Errors, DetectError{Path: "thermal", Err: err.Error()})
	}

	if s, err := d.DetectStorage(); err == nil {
		inv.Storage = s
	} else if !errors.Is(err, ErrUnsupported) {
		inv.Errors = append(inv.Errors, DetectError{Path: "storage", Err: err.Error()})
	}

	if k, err := d.DetectKernel(); err == nil {
		inv.Kernel = k
	} else if !errors.Is(err, ErrUnsupported) {
		inv.Errors = append(inv.Errors, DetectError{Path: "kernel", Err: err.Error()})
	}

	if h, err := d.DetectHost(); err == nil {
		inv.Host = h
	} else if !errors.Is(err, ErrUnsupported) {
		inv.Errors = append(inv.Errors, DetectError{Path: "host", Err: err.Error()})
	}

	return inv, nil
}
