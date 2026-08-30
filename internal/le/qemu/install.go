// Design: docs/architecture/testing/qemu-integration.md -- installer QEMU evidence
// Overview: actions.go -- the qemu actions that reach this runner
// Detail: install_build.go -- host and target artifact construction
// Detail: install_boot.go -- QEMU, serial, HTTP, and SSH lifecycles
// Detail: install_iso.go -- ISO primitives shared with Ventoy
// Detail: install_scenarios.go -- fault, pin, and rescue scenarios
// Detail: install_ventoy.go -- Ventoy FAT-disk proof
package qemu

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/dockerhost"
)

const (
	InstallImageName     = "ze-test.img"
	InstallApplianceName = "ze-install-qemu"
	InstallGuestServerIP = "10.0.2.2"
	InstallMarkWritten   = "image written, partition table re-read"
	InstallMarkDone      = "installation complete, rebooting"
	InstallMarkISODone   = "ISO installation complete, powering off"
	InstallMarkVentoy    = "found installer ISO via Ventoy"
	InstallPinnedMAC     = "52:54:00:ab:cd:01"
	InstallForeignMAC    = "52:54:00:ab:cd:02"
	// #nosec G101 -- this is a public serial marker and rescue-console test input, not a credential.
	InstallRescueToken         = "ze-rescue-evidence"
	InstallRescueAuth          = "5a65726573637565536f6c74303031ff:fed7b65bb317bc34097440c9bbd0a2ab3749edb8d88d3d37c94abe6cf62e399b"
	InstallDummyMediaID        = "00112233445566778899aabbccddeeff"
	InstallAArch64BIOSFallback = "/opt/homebrew/share/qemu/edk2-aarch64-code.fd"
	installTarget              = "/dev/vda"
	installDefaultSSHUser      = "admin"
	installDefaultSSHPass      = "secret"
	installDefaultNIC          = "virtio-net-pci"
	installSerialMax           = 1 << 20
	installSerialTailBytes     = 8000
)

// The kernel console the guest is told to write its boot to. The device name
// follows the architecture's serial port: 8250 on x86-64, PL011 on arm64.
const (
	installConsoleAMD64 = "ttyS0"
	installConsoleARM64 = "ttyAMA0"
)

const (
	installArchKey      = "ze.install.arch"
	installKernelKey    = "ze.install.kernel"
	installImageKey     = "ze.install.image"
	installZeFSKey      = "ze.install.zefs"
	installKeepKey      = "ze.install.keep"
	installImageSizeKey = "ze.install.image.size"
	installSSHUserKey   = "ze.install.ssh.user"
	// #nosec G101 -- this public environment key names a password setting; it is not the password.
	installSSHPassKey           = "ze.install.ssh.pass"
	installSSHPortKey           = "ze.install.ssh.port"
	installAccelKey             = "ze.install.qemu.accel"
	installNICKey               = "ze.install.nic"
	installAArch64BIOSKey       = "ze.install.aarch64.bios"
	installX86UEFIBIOSKey       = "ze.install.x86.uefi.bios"
	installBootTimeoutKey       = "ze.install.boot.timeout"
	installFaultTimeoutKey      = "ze.install.fault.timeout"
	installRescueStepTimeoutKey = "ze.install.rescue.step.timeout"
	installRescueTimeoutKey     = "ze.install.rescue.timeout"
)

var _ = []env.EnvEntry{
	installSetting(installArchKey, "", "the Linux target architecture for installer and appliance artifacts"),
	installSetting(installKernelKey, "", "the operator-supplied installer kernel"),
	installSetting(installImageKey, "", "a pre-built appliance image"),
	installSetting(installZeFSKey, "", "the ZeFS database served with a pre-built image"),
	installSetting(installKeepKey, "", "whether to retain installer QEMU artifacts"),
	installSetting(installImageSizeKey, "", "the appliance image size in bytes"),
	installSetting(installSSHUserKey, installDefaultSSHUser, "the installed appliance SSH user"),
	installSetting(installSSHPassKey, installDefaultSSHPass, "the installed appliance SSH password"),
	installSetting(installSSHPortKey, "", "the host port forwarded to appliance SSH"),
	installSetting(installAccelKey, "", "the QEMU accelerator"),
	installSetting(installNICKey, installDefaultNIC, "the installer QEMU network device"),
	installSetting(installAArch64BIOSKey, "", "the aarch64 UEFI firmware path"),
	installSetting(installX86UEFIBIOSKey, "", "the x86_64 UEFI firmware path"),
	installSetting(installBootTimeoutKey, "300s", "the installer boot deadline"),
	installSetting(installFaultTimeoutKey, "90s", "the fault scenario deadline"),
	installSetting(installRescueStepTimeoutKey, "120s", "the rescue prompt deadline"),
	installSetting(installRescueTimeoutKey, "120s", "the unattended rescue deadline"),
}

func installSetting(key, fallback, description string) env.EnvEntry {
	return env.MustRegister(env.EnvEntry{Key: key, Type: envTypeString, Default: fallback, Description: description, Private: true})
}

// InstallKind identifies one of the four import-linked installer proofs.
type InstallKind uint8

const (
	InstallKindUnspecified InstallKind = iota
	InstallKindHTTP
	InstallKindISO
	InstallKindScenarios
	InstallKindVentoy
)

func (kind InstallKind) String() string {
	switch kind {
	case InstallKindHTTP:
		return "install-test"
	case InstallKindISO:
		return "install-iso-test"
	case InstallKindScenarios:
		return "install-scenarios-test"
	case InstallKindVentoy:
		return "install-ventoy-test"
	default:
		return verdictWordUnspecified
	}
}

// InstallVerdict is zero-invalid so an unexecuted report cannot look successful.
type InstallVerdict uint8

const (
	InstallVerdictUnspecified InstallVerdict = iota
	InstallVerdictPass
	InstallVerdictSkip
	InstallVerdictFail
)

func (verdict InstallVerdict) String() string {
	switch verdict {
	case InstallVerdictPass:
		return verdictWordPass
	case InstallVerdictSkip:
		return verdictWordSkip
	case InstallVerdictFail:
		return verdictWordFail
	default:
		return verdictWordUnspecified
	}
}

func (verdict InstallVerdict) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(verdict.String())), nil
}

// InstallCheck is one observable assertion made by an installer proof.
type InstallCheck struct {
	Name    string         `json:"name"`
	Verdict InstallVerdict `json:"verdict"`
	Detail  string         `json:"detail,omitempty"`
}

// InstallScenarioReport is one scenario branch and its explicit verdict.
type InstallScenarioReport struct {
	Name    string         `json:"name"`
	Verdict InstallVerdict `json:"verdict"`
	Detail  string         `json:"detail,omitempty"`
}

// InstallArtifact records a byte-bearing artifact and its retained path.
type InstallArtifact struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
}

// InstallReport is the structured answer shared by all four actions.
type InstallReport struct {
	Action      string                  `json:"action"`
	Verdict     InstallVerdict          `json:"verdict"`
	Reason      string                  `json:"reason,omitempty"`
	Arch        string                  `json:"arch"`
	Accelerator string                  `json:"accelerator"`
	Kernel      string                  `json:"kernel,omitempty"`
	Checks      []InstallCheck          `json:"checks,omitempty"`
	Scenarios   []InstallScenarioReport `json:"scenarios,omitempty"`
	Artifacts   []InstallArtifact       `json:"artifacts,omitempty"`
	Retained    string                  `json:"retained,omitempty"`
	lines       []string
}

// Text replays the producer's operator-facing transcript for the bare action.
func (report InstallReport) Text() string {
	if len(report.lines) == 0 {
		return ""
	}
	var tb textbuf.Buffer
	return tb.Join(report.lines, "\n").Byte('\n').String()
}

func (report *InstallReport) line(prefix, text string) {
	report.lines = append(report.lines, prefix+text)
}
func (report *InstallReport) check(name string, verdict InstallVerdict, detail string) {
	report.Checks = append(report.Checks, InstallCheck{Name: name, Verdict: verdict, Detail: detail})
}
func (report *InstallReport) artifact(name, path string, bytes int64) {
	report.Artifacts = append(report.Artifacts, InstallArtifact{Name: name, Path: path, Bytes: bytes})
}

// InstallOptions holds every environment-derived decision for one run.
type InstallOptions struct {
	Kind              InstallKind
	Arch              string
	Accelerator       string
	Kernel            string
	Image             string
	ZeFS              string
	ImageSize         int64
	Keep              bool
	SSHUser           string
	SSHPassword       string
	SSHPort           int
	NIC               string
	AArch64BIOS       string
	X86UEFIBIOS       string
	BootTimeout       time.Duration
	FaultTimeout      time.Duration
	RescueStepTimeout time.Duration
	RescueTimeout     time.Duration
}

// Installer is one host-side install proof. It is not safe for concurrent use.
type Installer struct {
	Tree    string
	Options InstallOptions
	ops     installOps
}

type installOps struct {
	runOps
	Access func(string, uint32) bool
	// Socket reports whether a path is a usable Docker socket. It is a seam so a
	// test can drive the Colima selection on any platform.
	Socket func(string) bool
}

func productionInstallOps() installOps {
	return installOps{
		runOps: productionRunOps(),
		Access: func(path string, mode uint32) bool { return unix.Access(path, mode) == nil },
		Socket: dockerhost.IsSocket,
	}
}

// NewInstaller creates one proof over tree.
func NewInstaller(tree string, options InstallOptions) *Installer {
	return &Installer{Tree: tree, Options: options, ops: productionInstallOps()}
}

// DefaultInstallOptions resolves every environment default before resources are acquired.
func DefaultInstallOptions(kind InstallKind) (InstallOptions, error) {
	options := InstallOptions{
		Kind: kind, Arch: env.Get(installArchKey), Kernel: env.Get(installKernelKey), Image: env.Get(installImageKey),
		ZeFS: env.Get(installZeFSKey), Keep: env.Get(installKeepKey) == "1",
		SSHUser: settingOr(installSSHUserKey, installDefaultSSHUser), SSHPassword: settingOr(installSSHPassKey, installDefaultSSHPass),
		NIC: settingOr(installNICKey, installDefaultNIC), AArch64BIOS: env.Get(installAArch64BIOSKey),
		X86UEFIBIOS: env.Get(installX86UEFIBIOSKey), Accelerator: env.Get(installAccelKey),
	}
	if options.Arch == "" {
		if kind == InstallKindISO || kind == InstallKindVentoy {
			options.Arch = ArchAMD64
		} else {
			options.Arch = installHostArch(runtime.GOARCH)
		}
	}
	if options.Arch != ArchAMD64 && options.Arch != ArchARM64 {
		return options, fmt.Errorf("%s must be amd64 or arm64, got %q", installArchKey, options.Arch)
	}
	var err error
	if options.ImageSize, err = installOptionalBytes(env.Get(installImageSizeKey)); err != nil {
		return options, err
	}
	if options.SSHPort, err = installOptionalPort(env.Get(installSSHPortKey)); err != nil {
		return options, err
	}
	if options.BootTimeout, err = installDuration(installBootTimeoutKey, 300*time.Second); err != nil {
		return options, err
	}
	if options.FaultTimeout, err = installDuration(installFaultTimeoutKey, 90*time.Second); err != nil {
		return options, err
	}
	if options.RescueStepTimeout, err = installDuration(installRescueStepTimeoutKey, 120*time.Second); err != nil {
		return options, err
	}
	if options.RescueTimeout, err = installDuration(installRescueTimeoutKey, 120*time.Second); err != nil {
		return options, err
	}
	return options, nil
}

func installHostArch(goarch string) string {
	if goarch == ArchARM64 {
		return ArchARM64
	}
	return ArchAMD64
}

func installOptionalBytes(value string) (int64, error) {
	if value == "" {
		return 0, nil
	}
	bytes, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer byte count, got %q: %w", installImageSizeKey, value, err)
	}
	if bytes <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero, got %d", installImageSizeKey, bytes)
	}
	return bytes, nil
}

func installOptionalPort(value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	port, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s %q is not a port: %w", installSSHPortKey, value, err)
	}
	if port < 1 {
		return 0, fmt.Errorf("%s must be in 1..65535, got %d", installSSHPortKey, port)
	}
	if port > 65535 {
		return 0, fmt.Errorf("%s must be in 1..65535, got %d", installSSHPortKey, port)
	}
	return port, nil
}

func installDuration(key string, fallback time.Duration) (time.Duration, error) {
	value := env.Get(key)
	if value == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err == nil {
		if duration <= 0 {
			return 0, fmt.Errorf("%s must be greater than zero, got %q", key, value)
		}
		return duration, nil
	}
	seconds, secondsErr := strconv.Atoi(value)
	if secondsErr != nil {
		return 0, fmt.Errorf("%s %q is not a duration: %w", key, value, err)
	}
	if seconds <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero, got %q", key, value)
	}
	return time.Duration(seconds) * time.Second, nil
}

func (installer *Installer) prefix() string {
	switch installer.Options.Kind {
	case InstallKindHTTP:
		return "INSTALL-QEMU: "
	case InstallKindISO:
		return "INSTALL-ISO-QEMU: "
	case InstallKindScenarios:
		return "INSTALL-SCENARIOS-QEMU: "
	case InstallKindVentoy:
		return "INSTALL-VENTOY-QEMU: "
	default:
		return "INSTALL-QEMU: "
	}
}

func (installer *Installer) qemuBinary() string {
	if installer.Options.Arch == ArchARM64 {
		return qemuSystemARM64
	}
	return qemuSystemAMD64
}

func (installer *Installer) accelerator() string {
	if installer.Options.Accelerator != "" {
		return installer.Options.Accelerator
	}
	if installer.ops.GOOS == goosDarwin {
		return acceleratorHVF
	}
	if installer.ops.Access("/dev/kvm", unix.R_OK|unix.W_OK) {
		return acceleratorKVM
	}
	return acceleratorTCG
}

func (installer *Installer) skip(report InstallReport, reason string) (InstallReport, error) {
	report.Verdict, report.Reason = InstallVerdictSkip, reason
	var tb textbuf.Buffer
	report.line(installer.prefix(), tb.Str("SKIP ").Str(reason).String())
	return report, nil
}
func (installer *Installer) fail(report InstallReport, reason string) (InstallReport, error) {
	report.Verdict, report.Reason = InstallVerdictFail, reason
	var tb textbuf.Buffer
	report.line(installer.prefix(), tb.Str("FAIL ").Str(reason).String())
	return report, nil
}

// Execute runs the selected proof and pairs every temporary resource with cleanup.
func (installer *Installer) Execute(ctx context.Context) (report InstallReport, err error) {
	report = InstallReport{
		Action:      installer.Options.Kind.String(),
		Arch:        installer.Options.Arch,
		Accelerator: installer.accelerator(),
		Kernel:      installer.Options.Kernel,
	}
	if installer.Options.Kind == InstallKindUnspecified {
		return report, errors.New("installer QEMU action is unspecified")
	}
	qemu := installer.qemuBinary()
	if _, err := installer.ops.Look(qemu); err != nil {
		var tb textbuf.Buffer
		return installer.skip(report, tb.Str(qemu).Str(" not found").String())
	}
	if installer.Options.Kernel == "" {
		return installer.skip(report, installer.kernelSkipReason())
	}
	kernel := installer.Options.Kernel
	if !filepath.IsAbs(kernel) {
		kernel = filepath.Join(installer.Tree, kernel)
	}
	info, err := installer.ops.FS.Stat(kernel)
	if err != nil {
		return installer.skip(report, installer.kernelSkipReason())
	}
	if !info.Mode().IsRegular() {
		return installer.skip(report, installer.kernelSkipReason())
	}
	installer.Options.Kernel, report.Kernel = kernel, kernel
	if _, err := installer.ops.Look("go"); err != nil {
		return installer.skip(report, "go not found (needed to build Go initrd)")
	}
	if reason := installer.toolSkip(); reason != "" {
		return installer.skip(report, reason)
	}

	work, err := installer.ops.FS.MkdirTemp("", installer.workPrefix())
	if err != nil {
		return report, fmt.Errorf("create installer QEMU work directory: %w", err)
	}
	if installer.Options.Keep {
		report.Retained = work
	} else {
		defer func() {
			err = joinInstallCleanup(err, func() error {
				return installer.ops.FS.RemoveAll(work)
			}, "remove installer QEMU work directory")
		}()
	}

	switch installer.Options.Kind {
	case InstallKindHTTP:
		return installer.executeHTTP(ctx, work, report)
	case InstallKindISO:
		return installer.executeISO(ctx, work, report)
	case InstallKindScenarios:
		return installer.executeScenarios(ctx, work, report)
	case InstallKindVentoy:
		return installer.executeVentoy(ctx, work, report)
	default:
		return report, errors.New("installer QEMU action is unspecified")
	}
}

func joinInstallCleanup(result error, cleanup func() error, operation string) error {
	if err := cleanup(); err != nil {
		return errors.Join(result, fmt.Errorf("%s: %w", operation, err))
	}
	return result
}

func (installer *Installer) workPrefix() string {
	var b textbuf.Buffer
	return b.Str("ze-").Str(installer.Options.Kind.String()).Byte('-').String()
}

func (installer *Installer) kernelSkipReason() string {
	required := "IP_PNP_DHCP/VIRTIO_NET/VIRTIO_BLK/EXT4"
	if installer.Options.Kind == InstallKindISO {
		required += "/ISO9660/SR"
	}
	if installer.Options.Kind == InstallKindVentoy {
		required += "/ISO9660/VFAT/BLK_DEV_LOOP"
	}
	var b textbuf.Buffer
	return b.Str("no installer kernel — set ZE_INSTALL_KERNEL to a vmlinuz with ").Str(required).Str(" built in (=y)").String()
}

func (installer *Installer) toolSkip() string {
	needsISO := installer.Options.Kind == InstallKindISO
	if installer.Options.Kind == InstallKindVentoy {
		needsISO = true
	}
	if needsISO {
		if _, err := installer.ops.Look("grub-mkstandalone"); err != nil {
			if _, err := installer.ops.Look("grub2-mkstandalone"); err != nil {
				if installer.Options.Kind == InstallKindVentoy {
					return "grub-mkstandalone not found (needed to build the appliance ISO)"
				}
				return "grub-mkstandalone not found"
			}
		}
		if _, err := installer.ops.Look("xorriso"); err != nil {
			if installer.Options.Kind == InstallKindVentoy {
				return "xorriso not found (needed to build/inspect the appliance ISO)"
			}
			return "xorriso not found"
		}
	}
	if installer.Options.Kind == InstallKindVentoy {
		for _, tool := range []string{"mformat", "mcopy"} {
			if _, err := installer.ops.Look(tool); err != nil {
				var tb textbuf.Buffer
				return tb.Str(tool).Str(" not found (install mtools) — needed to build the FAT data disk").String()
			}
		}
	}
	needsImageTool := installer.Options.Kind != InstallKindScenarios
	if needsImageTool {
		if installer.Options.Image == "" {
			if _, err := installer.ops.Look("debugfs"); err != nil {
				if installer.brewDebugfs() == "" {
					return "debugfs missing (install e2fsprogs) — needed to inject zefs into /perm"
				}
			}
		}
	}
	if needsISO {
		firmware := installer.findUEFIFirmware()
		if firmware == "" {
			if installer.Options.Arch == ArchARM64 {
				return "aarch64 UEFI firmware not found (set ZE_INSTALL_AARCH64_BIOS)"
			}
			return "x86_64 UEFI firmware not found (set ZE_INSTALL_X86_UEFI_BIOS)"
		}
	}
	return ""
}

func (installer *Installer) run(ctx context.Context, spec commandSpec) (commandResult, error) {
	deadline, cancel := context.WithTimeout(ctx, installBuildTimeout)
	defer cancel()
	return installer.ops.Run(deadline, spec)
}

func installEnvWithoutTarget(environ []string) []string {
	result := make([]string, 0, len(environ)+1)
	for _, entry := range environ {
		name, _, _ := strings.Cut(entry, "=")
		if name == "GOOS" || name == "GOARCH" || name == "CGO_ENABLED" {
			continue
		}
		result = append(result, entry)
	}
	return append(result, "CGO_ENABLED=0")
}

func installEnvSet(environ []string, values ...string) []string {
	replacements := make(map[string]string, len(values))
	for _, entry := range values {
		name, _, _ := strings.Cut(entry, "=")
		replacements[name] = entry
	}
	result := make([]string, 0, len(environ)+len(values))
	for _, entry := range environ {
		name, _, _ := strings.Cut(entry, "=")
		if _, ok := replacements[name]; !ok {
			result = append(result, entry)
		}
	}
	return append(result, values...)
}
